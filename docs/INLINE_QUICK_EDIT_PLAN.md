# Plano — Aviso acionável inline + quick-edit de metas via API (de qualquer tela)

> Status: **plano (pending)**. Não implementar sem ok explícito. Espelhado em
> `roadmap-data.ts` (fase `crm-c10`) e na regra do [AGENT_RULES.md](../AGENT_RULES.md)
> "Config/dado faltando = aviso ACIONAVEL inline".

## Problema (caso real — Perola Jardins, 2026-06-17)

Dados que o cálculo de comissão usa estavam faltando e **mudavam o resultado em silêncio**:
- Loja **sem meta de ticket/PA** → a penalidade de qualidade (−0,1%/métrica) fica desligada
  (consultor com PA baixo recebe a taxa cheia, sem aviso).
- Loja **sem meta por consultor** → o back divide a meta da loja igualmente entre N consultores
  (`goalSource = store-split`); o número "de R$ 83.333,33" no card não diz que veio de uma divisão.
- A única forma de corrigir era achar a tela de config certa (Configurações > Metas CRM / Multi-loja).

Em produção isso **vai** acontecer até o pessoal se acostumar a preencher. A tela não pode mascarar
nem obrigar navegação: tem que mostrar o gap onde ele importa e deixar corrigir na hora.

## Princípio (já anotado no AGENT_RULES)

Quando uma tela consome config/dado que pode faltar e cuja ausência altera o resultado exibido:
1. **Transparência** — aviso honesto onde o dado faz diferença (o número nunca mente sobre a origem).
2. **Aviso clicável = editor inline** que grava pela **mesma API canônica** do recurso (sem "vá em X").
3. **Mesma ação em qualquer tela** via componente/composable compartilhado (sem reimplementar por tela).
4. **Gate por permissão espelhando o back + re-hidrata** após salvar. Fonte única = API; muda só o
   ponto de entrada da edição.

## Decisões de arquitetura

- **Reusar a infra de metas que JÁ existe** — NÃO criar endpoint novo:
  - Back: módulo `operationgoals` → `GET/POST/PATCH/DELETE /v1/operations/goals`.
  - Front: `useOperationGoalsStore` (`loadGoals`/`createGoal`/`updateGoal`).
  - Banco: `queue.operation_goal_targets` (unique index por escopo loja/consultor — migration 0121). **Sem migration nova.**
- O cálculo (`queue/commission`, embutido em `GET /v1/erp/crm`) já lê dessas metas. Editar a meta →
  próximo `/v1/erp/crm` reflete. Round-trip fechado (regra "Nada hardcoded").
- O back só ganha **flags de gap** no payload — o front não deve adivinhar o que falta.
- **Plugável e simples (declarativo, drop-in).** O mecanismo é UMA peça genérica: um componente
  `<InlineFieldGuard :descriptor :context />` + **descriptors** (objetos puros). Adicionar um caso novo
  (qualquer dado faltante editável, em qualquer tela) = **escrever 1 descriptor + soltar o componente**
  onde o dado aparece. Zero lógica bespoke por tela, zero fork de componente. Esse é o requisito central:
  ser plugável de forma bem simples.

### Descriptor (o "plugin") — formato

```ts
// web/app/domain/quick-edit/fields/*.ts
defineQuickEditField({
  id: 'storeTicketGoal',
  label: 'Meta de ticket médio',
  type: 'currency',                              // number | percent | currency | select
  isMissing: (ctx) => ctx.store?.missingTicketGoal,        // quando mostrar o aviso (lê os flags do payload)
  warning:   (ctx) => 'Sem meta de ticket — penalidade de qualidade desligada',
  canEdit:   (perm) => perm.canManageGoalTargets,          // espelha o back
  current:   (ctx) => ctx.store?.avgTicketGoal ?? null,    // valor atual (do payload)
  save:      (value, ctx) => goals.upsert({ storeId: ctx.storeId, month: ctx.month, avgTicketGoal: value }), // API canônica
  afterSave: (ctx) => crm.refresh(),                       // re-hidrata do back
})
```

O `<InlineFieldGuard>` recebe `descriptor` + `context` (tenant/store/consultant/month + os flags daquele
recurso) e cuida de TUDO: mostra o aviso se `isMissing`, deixa clicável se `canEdit`, abre o popover,
salva via `save`, re-hidrata via `afterSave`, fecha no clique-fora/Esc. A tela só faz:
`<InlineFieldGuard :descriptor="storeTicketGoal" :context="ctx" />`.

## Backend (Trilha A) — pequeno

**A1. Flags de gap no `GET /v1/erp/crm`** (DTO `crm/erp/model.go` + preenchimento em
`repository_crm_payout.go`, que já sabe quando caiu no fallback):
- Por consultor: `goalSource` (`own` | `store-split` | `none`), `missingMonthlyGoal`,
  `missingTicketGoal`, `missingPaGoal` (bool).
- Por loja: `missingStoreGoal`, `missingTicketGoal`, `missingPaGoal`, `goalSource`, e `splitCount`
  (nº de consultores na divisão, p/ a mensagem "meta da loja ÷ N").
- Muda `back/` → rebuild `docker compose up -d --build api`.

**A2. (Opcional, simplifica o QuickEdit)** `PUT /v1/operations/goals` upsert-by-scope
(tenant/store/consultant/month → monthly/ticket/pa, `ON CONFLICT` nos unique index da 0121), pra o front
não precisar saber se a linha já existe. MVP pode dispensar (load + POST/PATCH já resolve).

## Frontend (Trilha B) — grosso

**B1. O motor genérico (a peça plugável — escreve 1x, reusa em tudo):**
- `InlineFieldGuard.vue`: dado `descriptor` + `context`, renderiza o aviso (`isMissing`/`warning`),
  torna clicável se `canEdit`, abre o `QuickEditPopover`, salva via `descriptor.save`, re-hidrata via
  `descriptor.afterSave`, fecha no clique-fora/Esc. Tokens do design system; estados loading/erro.
- `QuickEditPopover.vue`: o popover genérico de input (number/percent/currency/select) usado pelo guard.
- `defineQuickEditField` + registry em `web/app/domain/quick-edit/`: os descriptors vivem aqui, um por
  arquivo. **Adicionar caso novo = 1 descriptor + soltar `<InlineFieldGuard>`.**

**B2. Primeiros descriptors + plug no /consultor** (`ConsultantPlayerCard`/`ConsultantPlayerGrid`):
- Descriptors: `storeTicketGoal`, `storePaGoal`, `consultantMonthlyGoal` (salvam via `/v1/operations/goals`).
- Plugar `<InlineFieldGuard>` nos cards. Aviso informativo p/ todos; editar gated por `canManageGoalTargets`.
- Mensagens vêm do descriptor: "Sem meta de ticket/PA — penalidade desligada"; "Sem meta individual —
  meta da loja R$ X ÷ N consultores".

**B3. Estender (2ª onda) = só plugar o MESMO componente** em `/operação`, `/ranking`, `multiloja`, e
escrever descriptors novos quando surgir outro dado (ex.: `storeType` via `PATCH /v1/stores/{id}`). Nada
reimplementado por tela.

## Permissões

Edição gated por `canManageGoalTargets` (já existe no front), espelhando o RBAC do back
(`operationgoals` valida escopo de tenant/store contra o Principal). Quem não pode editar vê só o aviso.

## Faseamento

- **Fase 1 (MVP):** A1 + B1 + B2 (só /consultor; metas de ticket/PA da loja e meta por consultor).
- **Fase 2:** B3 (demais telas) + A2 (upsert) + estender o padrão a outros dados (store_type via
  `PATCH /v1/stores/{id}`, política via `PATCH /v1/settings/crm-policy`).

## Verificação (quando pronto — testar no browser, papel-alvo)

Na Perola Jardins: /consultor mostra os avisos ("sem ticket/PA", "meta da loja ÷ 6"); usuário com
permissão clica → popover → cadastra a meta → o valor recalcula (vem do back) e persiste após refresh;
PA baixo passa a derrubar a taxa quando a meta de PA é cadastrada; quem não tem permissão vê só o aviso.

## Notas de Deploy

- Muda `back/` (flags no `/v1/erp/crm`) → rebuild `api`. **Sem migration** (reusa 0121).
- Atualizar 3 docs ao finalizar: este doc + AGENT.md (`crm/erp`, `consultant` front) + `roadmap-data.ts`.
