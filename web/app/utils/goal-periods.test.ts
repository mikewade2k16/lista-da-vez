import { describe, expect, it } from 'vitest'

import { goalWeekCount, goalWeekPeriods } from './goal-periods'

describe('goal periods', () => {
  it('detecta automaticamente meses com quatro ou cinco periodos', () => {
    expect(goalWeekCount('2026-02')).toBe(4)
    expect(goalWeekCount('2028-02')).toBe(5)
    expect(goalWeekCount('2026-07')).toBe(5)
    expect(goalWeekPeriods('2026-07')).toEqual(['p1', 'p2', 'p3', 'p4', 'p5'])
  })
})
