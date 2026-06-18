import { defineQuickEditField } from '~/domain/quick-edit/defineQuickEditField'
import { upsertGoalScope, type GoalQuickEditContext } from '~/domain/quick-edit/fields/goalContext'

// Meta de ticket médio da LOJA. Sem ela, a penalidade de qualidade fica desligada.
// Salva na linha de meta da loja (consultantId vazio) via /v1/operations/goals.
export const storeTicketGoal = defineQuickEditField<GoalQuickEditContext>({
  id: 'storeTicketGoal',
  label: 'Meta de ticket médio (loja)',
  type: 'currency',
  hint: 'Penalidade de qualidade usa esta meta.',
  isMissing: (ctx) => ctx.store.missingTicketGoal,
  warning: () => 'sem TM',
  canEdit: (permission) => permission.canManageGoalTargets,
  current: (ctx) => (ctx.store.avgTicketGoal > 0 ? ctx.store.avgTicketGoal : null),
  save: (value, ctx) => upsertGoalScope(ctx, 'store', { avgTicketGoal: value }),
  afterSave: (ctx) => ctx.refreshCrm(),
})
