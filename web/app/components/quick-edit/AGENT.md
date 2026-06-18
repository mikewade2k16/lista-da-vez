# AGENTS

## Escopo

`web/app/components/quick-edit` + descriptors em `web/app/domain/quick-edit`.

## Responsabilidade

Motor PLUGÁVEL de "aviso acionável inline + quick-edit" (AGENT_RULES > Frontend >
"Config/dado faltando = aviso ACIONAVEL inline"). Onde um dado que pode faltar muda o
resultado exibido, mostra um aviso honesto e — se o usuário pode editar — abre um popover
ancorado que grava pela MESMA API canônica do recurso e re-hidrata do back.

## Como funciona (declarativo, drop-in)

Adicionar um caso novo = **escrever 1 descriptor + soltar `<InlineFieldGuard>`**. Zero
lógica bespoke por tela, zero fork de componente.

1. **`~/domain/quick-edit/defineQuickEditField.ts`** — tipo `QuickEditFieldDescriptor<TContext>`
   - helper `defineQuickEditField`. Um descriptor é objeto PURO: `id`, `label`, `type`
     (`number`|`percent`|`currency`), `hint?`, `isMissing(ctx)`, `warning(ctx)`,
     `canEdit(permission)`, `current(ctx)`, `save(value, ctx)`, `afterSave(ctx)`.
2. **`QuickEditPopover.vue`** — popover ANCORADO genérico (um input number/percent/currency,
   salvar/cancelar, loading/erro). Fecha no clique-fora + Esc + ao salvar (regra obrigatória).
   Tokens do design system, classes BEM.
3. **`InlineFieldGuard.vue`** — `props: { descriptor, context }`. Se `descriptor.isMissing(context)`,
   renderiza um chip de aviso com `descriptor.warning`. Se `descriptor.canEdit(context.permission)`,
   o chip é clicável e abre o `QuickEditPopover` ancorado nele; ao salvar chama `descriptor.save`
   e depois `descriptor.afterSave` (re-hidrata do back). Sem permissão = só o aviso informativo.
   Genérico via `<script setup lang="ts" generic="TContext extends QuickEditContextBase">`.

## Descriptors atuais (`~/domain/quick-edit/fields/`)

- **`goalContext.ts`** — tipo `GoalQuickEditContext` (escopo tenant/store/consultant/month +
  flags de gap de loja/consultor + `goals` store + `refreshCrm`) e helper `upsertGoalScope`:
  resolve a linha de meta existente do escopo (loja = `consultantId` vazio; consultor =
  preenchido) via `loadGoals` e faz PATCH se existir, senão POST. API canônica:
  `/v1/operations/goals`. SEM endpoint novo.
- **`storeTicketGoal`** / **`storePaGoal`** — salvam na linha de meta da LOJA. Aviso: "Sem meta
  de ticket/P.A. — penalidade desligada".
- **`consultantMonthlyGoal`** — salva na linha do CONSULTOR. Aviso: "Sem meta individual —
  R$ X da loja ÷ N".

## Montagem do contexto

`~/composables/useGoalQuickEditContext.ts` centraliza auth (`canManageGoalTargets`,
espelhando o back) + store de metas + `refreshCrm` (`consultants.refreshIntegratedView`).
Os componentes "burros" só informam escopo + flags e recebem o `GoalQuickEditContext` pronto.

## Contrato congelado consumido (do `/v1/erp/crm`)

- por consultor: `goalSource` (`own`|`store-split`|`none`), `missingMonthlyGoal`,
  `missingTicketGoal`, `missingPaGoal`.
- por loja: `storeGoalSource` (`own`|`consultant-sum`|`none`), `missingStoreGoal`,
  `missingTicketGoal`, `missingPaGoal`, `splitConsultantCount`.

Lidos defensivamente em `~/domain/utils/consultant-integrated-view.ts` (`ErpMetric`,
`ErpStorePayout`) e propagados por `useConsultantIntegratedRows` → grid/workspace → card.
Antes do rebuild do back, os flags vêm ausentes (default seguro = sem aviso).
