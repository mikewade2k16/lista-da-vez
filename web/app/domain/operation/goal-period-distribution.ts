export type GoalPeriodMetrics = {
  monthlyGoal: number
  avgTicketGoal: number
  conversionGoal: number
  paGoal: number
}

function positive(value: number): number {
  return Math.max(0, Number(value) || 0)
}

function normalized(metrics: GoalPeriodMetrics): GoalPeriodMetrics {
  return {
    monthlyGoal: positive(metrics.monthlyGoal),
    avgTicketGoal: positive(metrics.avgTicketGoal),
    conversionGoal: Math.min(100, positive(metrics.conversionGoal)),
    paGoal: positive(metrics.paGoal),
  }
}

export function distributeAmount(total: number, parts: number): number[] {
  const normalizedTotal = Math.round(positive(total) * 100)
  const normalizedParts = Math.max(0, Math.trunc(parts))
  if (!normalizedParts) return []

  const base = Math.floor(normalizedTotal / normalizedParts)
  const remainder = normalizedTotal - base * normalizedParts
  return Array.from(
    { length: normalizedParts },
    (_, index) => (base + (index < remainder ? 1 : 0)) / 100,
  )
}

export function distributeMonthlyGoal(metrics: GoalPeriodMetrics): GoalPeriodMetrics[] {
  const source = normalized(metrics)
  return distributeAmount(source.monthlyGoal, 4).map((monthlyGoal) => ({
    ...source,
    monthlyGoal,
  }))
}

export function suggestRemainingWeeklyGoals(
  monthlyGoal: number,
  realizedToDate: number,
  remainingWeeks: number,
): number[] {
  return distributeAmount(
    Math.max(0, positive(monthlyGoal) - positive(realizedToDate)),
    remainingWeeks,
  )
}
