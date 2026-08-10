export type GoalPeriodValue = 'month' | 'p1' | 'p2' | 'p3' | 'p4' | 'p5'

export function goalWeekCount(month: string): 4 | 5 {
  const match = /^(\d{4})-(\d{2})$/.exec(month)
  if (!match) return 4
  const days = new Date(Date.UTC(Number(match[1]), Number(match[2]), 0)).getUTCDate()
  return days > 28 ? 5 : 4
}

export function goalWeekPeriods(month: string): GoalPeriodValue[] {
  return Array.from(
    { length: goalWeekCount(month) },
    (_, index) => `p${index + 1}` as GoalPeriodValue,
  )
}

export function isGoalPeriodForMonth(value: string, month: string): value is GoalPeriodValue {
  return value === 'month' || goalWeekPeriods(month).includes(value as GoalPeriodValue)
}
