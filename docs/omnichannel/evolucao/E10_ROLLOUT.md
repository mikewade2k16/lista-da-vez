# E10 — rollout e operação

**Status:** `DRAFT` até gates E1–E9

**Resultado:** IA e canais são ativados por conta/número/coorte em etapas mensuráveis, com shadow,
canário, pausa imediata e rollback ensaiado.

## 1. Modos de operação

Config autoritativa fica em `core.account_modules.config` ou estrutura de feature flag já existente,
validada pelo Go:

| Modo | Ingest/CRM | IA | Envio IA | Uso |
|---|---:|---:|---:|---|
| `off` | opcional/provider desligado | não | não | antes do onboarding |
| `observe` | sim | não | não | provar canal/inbox |
| `shadow` | sim | sim | não | medir decisão/custo sem risco externo |
| `assist` | sim | sim | draft humano | calibrar qualidade |
| `auto_pilot` | sim | sim | sim, policy/coorte | piloto controlado |
| `active` | sim | sim | sim | operação expandida |
| `paused` | sim | não/não envia | não | kill switch mantendo inbox humano |

O gate é server-side no dispatch e no claim da outbox. Ocultar botão no front não desativa IA.

Config de rollout proposta:

```json
{
  "mode": "shadow",
  "allowedInstanceIds": ["uuid"],
  "allowedInstagramAccountIds": [],
  "autoReplyPercent": 0,
  "allowedQueues": ["uuid"],
  "allowedHours": {"timezone": "America/Sao_Paulo", "windows": []},
  "excludedTags": ["vip", "legal"],
  "maxDailyAutoReplies": 100,
  "killSwitchReason": null
}
```

Não criar migration se config existente suportar validação/versionamento. Mudança de modo é
auditada com ator, before/after, reason e timestamp.

## 2. Gates antes de qualquer cliente real

### Gate técnico

- E0 ownership verificado;
- smoke E1 completo;
- E2 shadow idempotente e corrida humano×IA aprovada;
- CRM E4 sem duplicata crítica;
- handoff E5 e fallback sem n8n aprovados;
- provider do número e rollback E7 provados;
- SLO/alerta/backup/restore E9 ativos;
- nenhum segredo no export/log;
- runbook e on-call definidos.

### Gate de produto

- setores, filas, membros, horários e responsáveis configurados;
- agente publicado com modelo/chave/custo/limite;
- campos obrigatórios e regras de roteamento revisados;
- tom, FAQ, ferramentas e palavras bloqueadas aprovados;
- templates oficiais necessários aprovados;
- mensagem de transparência/opt-out e política de dados definidas;
- conjunto de conversas de avaliação com resultado esperado.

## 3. Etapas de rollout

### R0 — demonstração interna

Conta teste, número teste, fixtures + celular real. Mostrar na tela: inbound, mídia, reply, CRM,
classificação, shadow decision, handoff, humano assume e auditoria/custo. Nenhuma resposta IA real
sem aprovação.

### R1 — observe

24–48h recebendo tráfego do número piloto sem IA. Medir dedupe, ingest, latência, mídia, inbox e
capacidade dos atendentes. Zero perda/duplicata é gate.

### R2 — shadow

IA processa 100% do coorte, envia 0%. Revisão humana amostra decisões por intenção e calcula:
schema-valid, intent accuracy, route accuracy, field completion, false handoff, unsafe reply,
latência e custo.

### R3 — assist

Draft aparece ao atendente. Medir taxa de uso, edição e rejeição; coletar reason codes. Ajustar
prompt/policy por nova versão publicada, nunca editar versão ativa.

### R4 — auto piloto

Começar em 5–10% de conversas elegíveis, horário comercial, intenções allowlisted, sem tags/casos
sensíveis. Subir 10→25→50→100% apenas após janela de observação e gates.

### R5 — expansão

Adicionar filas, horários, números e contas um de cada vez. WhatsApp oficial por número; Instagram
DM antes de comentário automático; comentário público começa em aprovação.

## 4. Critérios quantitativos de avanço

Baseline inicial sugerida, ajustável somente por decisão registrada:

- 100% outputs no schema ou fallback seguro;
- 0 envio duplicado e 0 vazamento cross-tenant;
- >= 90% rota correta no conjunto rotulado antes do auto piloto;
- <= 5% respostas consideradas inseguras/incorretas críticas; crítico individual pausa rollout;
- >= 95% handoffs com resumo/campos utilizáveis;
- SLOs E9 atendidos por 24h no coorte;
- custo/conversa dentro do budget da conta;
- backlog/outbox/DLQ sem crescimento sustentado;
- opt-out e janela do canal respeitados em 100% dos casos testados.

## 5. Kill switch e rollback

`paused` interrompe novos dispatches e claims de mensagens automáticas, mas mantém ingest, CRM e
inbox humano. Jobs em processamento checam mode/generation antes do side effect. Motivos mínimos:
cross-tenant, duplicata, resposta insegura crítica, assinatura/webhook comprometido, custo fora do
limite, backlog crescente ou provider ban/quality risk.

Rollback por camada:

- prompt/modelo: repontar versão publicada anterior;
- IA: `assist|shadow|paused`, sem trocar provider;
- workflow: desativar somente workflow próprio, preservar export/versão;
- provider: seguir runbook E7 por número, drenando outbox;
- front: capability esconde ação nova após backend já estar seguro;
- migration: não dropar; código anterior deve tolerar colunas/tabelas aditivas.

Nunca reprocessar em massa eventos deduplicados durante rollback.

## 6. Operação e painel

Painel de rollout mostra modo, coorte, percentagem, versão do agente/workflow, provider, última
mudança/ator, KPIs, custo, incidentes e kill switch. Toda alteração exige permission, motivo e
confirmação. O botão de aumento de percentagem fica indisponível se gate técnico falhar.

Runbook diário:

1. health/provider/webhook;
2. backlog/idade outbox e dispatch;
3. handoff/SLA;
4. erros schema/tool/media;
5. custo/limite;
6. amostra de qualidade e opt-out;
7. incidentes/ações/decisão de manter ou avançar.

## 7. Pacotes atômicos

| Pacote | Entrega |
|---|---|
| `E10-CONTRACT-01` | modos, config, gates e permissions |
| `E10-BE-02` | gate server-side, audit e kill switch |
| `E10-FE-03` | painel de rollout e bloqueios explicados |
| `E10-EVAL-04` | dataset/métricas/revisão shadow-assist |
| `E10-RUNBOOK-05` | demonstração, piloto, expansão e incidentes |
| `E10-QA-06` | ensaio de kill switch e rollback por camada |

## 8. Aceite final do programa

- demonstração R0 executável do zero com roteiro e evidência;
- shadow/assist/auto respeitam percentagem/coorte/horário/tags;
- kill switch impede side effect atrasado e humano continua atendendo;
- rollback de modelo, IA, workflow e provider foi ensaiado;
- avanço de etapa deixa decisão/ator/métricas auditados;
- WhatsApp/Instagram usam o mesmo contato, domínio, outbox e inbox;
- Automação/WAHA/Calendário/Operação permanecem preservados;
- owner aceita qualidade, custo, SLA, segurança e operação antes de `active`.
