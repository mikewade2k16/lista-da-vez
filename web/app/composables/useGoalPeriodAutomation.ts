import { ref } from 'vue'
import type { Ref } from 'vue'

import {
  distributeMonthlyGoal,
  suggestRemainingWeeklyGoals,
  type GoalPeriodMetrics,
} from '~/domain/operation/goal-period-distribution'
import type { OperationGoalTarget } from '~/stores/operation-goals'
import { useOperationGoalsStore } from '~/stores/operation-goals'

type GoalPeriodAutomationOptions = {
  goals: Ref<OperationGoalTarget[]>
}

function metricsOf(goal: OperationGoalTarget): GoalPeriodMetrics {
  return {
    monthlyGoal: goal.monthlyGoal,
    avgTicketGoal: goal.avgTicketGoal,
    conversionGoal: goal.conversionGoal,
    paGoal: goal.paGoal,
  }
}

function sameGoalOwner(left: OperationGoalTarget, right: OperationGoalTarget): boolean {
  return (
    left.scope === right.scope &&
    left.storeId === right.storeId &&
    left.consultantId === right.consultantId &&
    left.month === right.month
  )
}

export function useGoalPeriodAutomation(options: GoalPeriodAutomationOptions) {
  const operationGoals = useOperationGoalsStore()
  const syncing = ref(false)

  function findPeriodGoal(source: OperationGoalTarget, week: number) {
    return options.goals.value.find(
      (goal) => sameGoalOwner(goal, source) && Number(goal.week || 0) === week,
    )
  }

  async function upsertPeriodGoal(
    source: OperationGoalTarget,
    week: number,
    metrics: GoalPeriodMetrics,
  ): Promise<boolean> {
    const existing = findPeriodGoal(source, week)
    const result = existing
      ? await operationGoals.updateGoal(existing.id, metrics, {
          reload: false,
          skipLoadingIndicator: true,
        })
      : await operationGoals.createGoal(
          {
            storeId: source.storeId,
            consultantId: source.consultantId || undefined,
            month: source.month,
            week,
            ...metrics,
          },
          { reload: false, skipLoadingIndicator: true },
        )
    return result?.ok !== false
  }

  async function syncMonthlyGoal(
    source: OperationGoalTarget,
    redistributeFinancial = true,
  ): Promise<boolean> {
    if (Number(source.week || 0) !== 0) return true

    syncing.value = true
    try {
      const weeks = distributeMonthlyGoal(metricsOf(source))
      const results = await Promise.all(
        weeks.map((metrics, index) => {
          const existing = findPeriodGoal(source, index + 1)
          return upsertPeriodGoal(source, index + 1, {
            ...metrics,
            monthlyGoal:
              redistributeFinancial || !existing ? metrics.monthlyGoal : existing.monthlyGoal,
          })
        }),
      )
      return results.every(Boolean)
    } finally {
      syncing.value = false
    }
  }

  async function applyRemainingSuggestion(
    monthlyGoal: OperationGoalTarget,
    completedWeek: number,
    realizedToDate: number,
  ): Promise<boolean> {
    const remainingWeeks = Math.max(0, 4 - completedWeek)
    if (!remainingWeeks || syncing.value) return false

    syncing.value = true
    try {
      const suggestions = suggestRemainingWeeklyGoals(
        monthlyGoal.monthlyGoal,
        realizedToDate,
        remainingWeeks,
      )
      const baseMetrics = metricsOf(monthlyGoal)
      const results = await Promise.all(
        suggestions.map((monthlyGoalValue, index) => {
          const week = completedWeek + index + 1
          const existing = findPeriodGoal(monthlyGoal, week)
          return upsertPeriodGoal(monthlyGoal, week, {
            ...(existing ? metricsOf(existing) : baseMetrics),
            monthlyGoal: monthlyGoalValue,
          })
        }),
      )
      return results.every(Boolean)
    } finally {
      syncing.value = false
    }
  }

  return { syncing, syncMonthlyGoal, applyRemainingSuggestion }
}
