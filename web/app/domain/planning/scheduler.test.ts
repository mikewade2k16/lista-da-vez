import { describe, expect, it } from 'vitest'

import { createPlanningFixtures } from './fixtures'
import {
  buildShiftFromTemplate,
  buildWeekDates,
  deriveShiftTemplatesFromOperatingHours,
  shiftHours,
  weekStartForGoalPeriod,
} from './scheduler'

describe('planning schedule helpers', () => {
  it('maps goal periods 1 to 4 to their calendar weeks', () => {
    expect(weekStartForGoalPeriod('2026-07', 'p1')).toBe('2026-06-29')
    expect(weekStartForGoalPeriod('2026-07', 'p2')).toBe('2026-07-06')
    expect(weekStartForGoalPeriod('2026-07', 'p3')).toBe('2026-07-13')
    expect(weekStartForGoalPeriod('2026-07', 'p4')).toBe('2026-07-20')
    expect(weekStartForGoalPeriod('2026-07', 'p5')).toBe('2026-07-27')
    expect(weekStartForGoalPeriod('2026-08', 'p1')).toBe('2026-07-27')
  })

  it('builds a manual shift within the registered store hours', () => {
    const fixtures = createPlanningFixtures()
    const store = fixtures.stores[0]
    const policy = fixtures.policies[0]
    const staff = fixtures.staff.find((member) => member.storeId === store.id)!
    const monday = buildWeekDates('2026-07-27')[0]!

    const shift = buildShiftFromTemplate(store, staff, policy, monday, 'opening')

    expect(shift).not.toBeNull()
    expect(shiftHours(shift!)).toBeLessThanOrEqual(staff.maxDailyHours)
  })

  it('derives separate templates from each operating profile', () => {
    const fixtures = createPlanningFixtures()
    const store = fixtures.stores[0]
    const shopping = deriveShiftTemplatesFromOperatingHours(
      store.operatingHoursByLocationType.shopping,
      store.shiftTemplatesByLocationType.shopping,
    )
    const street = deriveShiftTemplatesFromOperatingHours(
      store.operatingHoursByLocationType.street,
      store.shiftTemplatesByLocationType.street,
    )

    expect(shopping.find((template) => template.id === 'closing')?.endsAt).toBe('22:00')
    expect(street.find((template) => template.id === 'closing')?.endsAt).toBe('19:00')
  })
})
