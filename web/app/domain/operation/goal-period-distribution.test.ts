import { describe, expect, it } from 'vitest'

import {
  distributeAmount,
  distributeMonthlyGoal,
  suggestRemainingWeeklyGoals,
} from './goal-period-distribution'

describe('goal period distribution', () => {
  it('divides the monthly financial target and preserves commercial indicators', () => {
    const weeks = distributeMonthlyGoal({
      monthlyGoal: 120000,
      avgTicketGoal: 850,
      conversionGoal: 30,
      paGoal: 1.9,
    })

    expect(weeks).toHaveLength(4)
    expect(weeks.every((week) => week.monthlyGoal === 30000)).toBe(true)
    expect(weeks.every((week) => week.avgTicketGoal === 850)).toBe(true)
    expect(weeks.every((week) => week.paGoal === 1.9)).toBe(true)
  })

  it('preserves cents exactly when splitting a value', () => {
    expect(distributeAmount(100.01, 4)).toEqual([25.01, 25, 25, 25])
  })

  it('suggests the remaining target after realized sales', () => {
    expect(suggestRemainingWeeklyGoals(100_000, 46_000, 2)).toEqual([27_000, 27_000])
    expect(suggestRemainingWeeklyGoals(100_000, 120_000, 2)).toEqual([0, 0])
  })
})
