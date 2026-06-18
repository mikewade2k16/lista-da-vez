import { defineQuickEditField } from '~/domain/quick-edit/defineQuickEditField'
import { upsertGoalScope, type GoalQuickEditContext } from '~/domain/quick-edit/fields/goalContext'

// Meta individual do CONSULTOR. Sem ela, o back divide a meta da loja igualmente
// entre N consultores (goalSource = store-split). Salva na linha do CONSULTOR.
export const consultantMonthlyGoal = defineQuickEditField<GoalQuickEditContext>({
  id: 'consultantMonthlyGoal',
  label: 'Meta individual (consultor)',
  type: 'currency',
  hint: 'Substitui a divisão automática da meta da loja entre os consultores.',
  isMissing: (ctx) => ctx.consultant.missingMonthlyGoal && ctx.consultant.goalSource !== 'own',
  warning: () => 'sem Meta',
  canEdit: (permission) => permission.canManageGoalTargets,
  current: (ctx) => (ctx.consultant.monthlyGoal > 0 ? ctx.consultant.monthlyGoal : null),
  save: (value, ctx) => upsertGoalScope(ctx, 'consultant', { monthlyGoal: value }),
  afterSave: (ctx) => ctx.refreshCrm(),
})
