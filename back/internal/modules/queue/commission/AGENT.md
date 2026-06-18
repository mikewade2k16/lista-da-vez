# AGENT

## Escopo

Estas instrucoes valem para `back/internal/modules/queue/commission`.

## Responsabilidade do modulo

Pacote FOLHA (so stdlib; sem DB, sem HTTP, sem dependencia de outro modulo).
E a FONTE UNICA do calculo de comissao ("Recebimento por atingimento de meta").
O cálculo vivia so no front (`web/app/domain/utils/crm-performance-policy.ts`);
agora vive aqui e e embutido no payload de `GET /v1/erp/crm` (modulo `crm/erp`).

100% testavel (`calculate_test.go`). Nenhum outro modulo deve reimplementar
esta logica — deve chamar `commission.Calculate`.

## Contrato (politica v2)

Politica armazenada como JSONB em
`queue.tenant_operation_core_settings.crm_goal_payout_policy`. Shape v2:

```jsonc
{
  "consultant":      [{ "threshold": 50, "value": 1.5, "mode": "percent" }],
  "managerShopping": [{ "threshold": 80, "value": 0.8, "mode": "percent" }],
  "managerBairro":   [{ "threshold": 80, "value": 1,   "mode": "percent" }],
  "support":         [{ "threshold": 80, "value": 80,  "mode": "amount"  }],
  "consultantRules": { "base": "self", "minOwnGoalPercent": 100, "qualityPenaltyPercent": 0.1 }
}
```

- `mode` = `percent` | `amount`.
- `base` (consultantRules) = `self` (propria venda) | `store` (total da loja).

### Retrocompat (`NormalizePolicy`)

- Linha antiga tem so `consultant`/`manager`/`support`. `managerShopping` e
  `managerBairro` ausentes sao semeados a partir de `manager` (legado); so
  quando o campo novo FALTA (nil). Faixa vazia EXPLICITA (`[]`) e preservada.
- `consultantRules` ausente => default `{base:self, minOwnGoalPercent:100, qualityPenaltyPercent:0.1}`.

### Defaults v2

- `consultant` = `[{50, 1.5, percent}]`
- `managerShopping` = `[{80,0.8},{90,0.9},{100,1},{120,1.2}]` percent
- `managerBairro` = `[{80,1},{100,1.7},{120,2}]` percent
- `support` = `[{80,80},{90,90},{100,100},{120,120}]` amount

## Calculo (`Calculate(Input) Result`)

Faixa = regra de MAIOR `threshold <= progresso` (se nenhuma, payout 0,
`RuleMatched=false`).

- `storeProgress = storeGoal>0 ? storeSold/storeGoal*100 : 0`
- `consultantProgress = monthlyGoal>0 ? consultantSold/monthlyGoal*100 : 0`
- `hitPa = paGoal>0 ? paScore>=paGoal : true`
- `hitTicket = avgTicketGoal>0 ? ticketAverage>=avgTicketGoal : true`

Por grupo (mapeado de `role` por `MapRoleToGroup`):

- **manager**: faixas = `storeType=="shopping" ? managerShopping : managerBairro`;
  faixa por `storeProgress`; `percent` -> `amount = storeSold*value/100`;
  `amount` -> valor fixo; base = `storeSold`.
- **support**: faixas `support`; faixa por `storeProgress`; `amount` fixo.
- **consultant**: se `consultantProgress < minOwnGoalPercent` => payout 0.
  Senao faixa por `storeProgress` em `consultant`;
  `rateEfetiva = rule.value - qualityPenaltyPercent*((hitPa?0:1)+(hitTicket?0:1))`
  com clamp em 0; base = `base=="self" ? consultantSold : storeSold`;
  `percent` -> `amount = base*rateEfetiva/100`; `amount` -> valor fixo.

### Mapeamento papel -> grupo (`MapRoleToGroup`)

Normaliza minusculas + remove acentos, depois casa por token:

- manager = `[manager, gerente, gerencia, subgerente, lider, leader]`
- support = `[support, caixa, cashier, auxiliar, assistant, estoquista, estoque, financeiro, recepcao]`
- senao => consultant

## `Result`

`{ Amount, RatePercent, Base, Group, RuleLabel, PenaltyApplied, RuleMatched }`.
`RatePercent` = rate efetiva (ja com penalidade). `Base` = valor sobre o qual
incidiu. `RuleLabel` curto (ex.: "1,5% da propria venda", "R$ 90 fixo",
"Sem faixa"). `PenaltyApplied` = penalidade total em pontos percentuais.

## Regras do modulo

- So stdlib. Nao importar pgx/auth/http nem outro modulo.
- Paridade EXATA com o contrato e com `crm-performance-policy.ts` (`NormalizePolicy`).
- Qualquer mudanca de regra deve refletir nos dois lados (Go + TS normalize) e
  num teste novo aqui.
