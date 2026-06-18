import { defineQuickEditField } from '~/domain/quick-edit/defineQuickEditField'
import { upsertGoalScope, type GoalQuickEditContext } from '~/domain/quick-edit/fields/goalContext'

// Meta de P.A. (peças por atendimento) da LOJA. Sem ela, a penalidade de qualidade
// fica desligada. Salva na linha de meta da loja (consultantId vazio).
export const storePaGoal = defineQuickEditField<GoalQuickEditContext>({
  id: 'storePaGoal',
  label: 'Meta de P.A. (loja)',
  type: 'number',
  hint: 'Peças por atendimento. Penalidade de qualidade usa esta meta.',
  isMissing: (ctx) => ctx.store.missingPaGoal,
  warning: () => 'sem PA',
  canEdit: (permission) => permission.canManageGoalTargets,
  current: (ctx) => (ctx.store.paGoal > 0 ? ctx.store.paGoal : null),
  save: (value, ctx) => upsertGoalScope(ctx, 'store', { paGoal: value }),
  afterSave: (ctx) => ctx.refreshCrm(),
})
