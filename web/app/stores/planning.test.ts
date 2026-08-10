import { beforeEach, describe, expect, it } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'

import { shiftHours } from '~/domain/planning/scheduler'

import { usePlanningStore } from './planning'

describe('planning store', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
  })

  it('keeps independent operating hours for shopping and street profiles', () => {
    const planning = usePlanningStore()

    expect(planning.activeStore?.locationType).toBe('shopping')
    expect(
      planning.activeStore?.operatingHoursByLocationType.shopping.find(
        (day) => day.weekday === 'mon',
      )?.opensAt,
    ).toBe('10:00')

    planning.updateOperatingDay('mon', { opensAt: '10:30' })
    planning.updateLocationType('street')

    expect(
      planning.activeStore?.operatingHoursByLocationType.street.find((day) => day.weekday === 'mon')
        ?.opensAt,
    ).toBe('09:00')

    planning.updateOperatingDay('mon', { opensAt: '08:30' })
    planning.updateLocationType('shopping')

    expect(
      planning.activeStore?.operatingHoursByLocationType.shopping.find(
        (day) => day.weekday === 'mon',
      )?.opensAt,
    ).toBe('10:30')

    planning.updateLocationType('street')
    expect(
      planning.activeStore?.operatingHoursByLocationType.street.find((day) => day.weekday === 'mon')
        ?.opensAt,
    ).toBe('08:30')
  })

  it('keeps independent shift templates for shopping and street profiles', () => {
    const planning = usePlanningStore()
    const shoppingClosing = planning.activeStore?.shiftTemplatesByLocationType.shopping.find(
      (template) => template.id === 'closing',
    )
    const streetClosing = planning.activeStore?.shiftTemplatesByLocationType.street.find(
      (template) => template.id === 'closing',
    )

    expect(shoppingClosing?.endsAt).toBe('22:00')
    expect(streetClosing?.endsAt).toBe('19:00')

    planning.updateShiftTemplate('shopping', 'closing', { name: 'Fechamento shopping' })
    planning.updateLocationType('street')

    expect(
      planning.activeStore?.shiftTemplatesByLocationType.street.find(
        (template) => template.id === 'closing',
      )?.name,
    ).toBe('Fechamento')

    planning.updateShiftTemplate('street', 'closing', { name: 'Fechamento rua' })
    planning.updateLocationType('shopping')

    expect(
      planning.activeStore?.shiftTemplatesByLocationType.shopping.find(
        (template) => template.id === 'closing',
      )?.name,
    ).toBe('Fechamento shopping')
  })

  it('recalculates shift templates from the registered operating hours', () => {
    const planning = usePlanningStore()

    for (const weekday of ['mon', 'tue', 'wed', 'thu', 'fri', 'sat'] as const) {
      planning.updateOperatingDay(weekday, { opensAt: '11:00', closesAt: '21:00' })
    }

    const shoppingTemplates = planning.activeStore?.shiftTemplatesByLocationType.shopping
    expect(shoppingTemplates?.find((template) => template.id === 'opening')).toMatchObject({
      startsAt: '11:00',
      endsAt: '20:00',
    })
    expect(shoppingTemplates?.find((template) => template.id === 'middle')).toMatchObject({
      startsAt: '11:30',
      endsAt: '20:30',
    })
    expect(shoppingTemplates?.find((template) => template.id === 'closing')).toMatchObject({
      startsAt: '12:00',
      endsAt: '21:00',
    })
    expect(
      planning.activeStore?.shiftTemplatesByLocationType.street.find(
        (template) => template.id === 'closing',
      )?.endsAt,
    ).toBe('19:00')
  })

  it('uses real weekly goals and recalculates employee targets from the active schedule', () => {
    const planning = usePlanningStore()
    planning.syncStoreReferences([
      {
        id: 'store-real',
        name: 'Loja Real',
        city: 'Aracaju',
        storeType: 'shopping',
      },
    ])
    planning.syncStaffReferences('store-real', [
      {
        id: 'staff-one',
        storeId: 'store-real',
        name: 'Pessoa Um',
        employeeCode: 'C-1',
        role: 'Consultor',
      },
      {
        id: 'staff-two',
        storeId: 'store-real',
        name: 'Pessoa Dois',
        employeeCode: 'C-2',
        role: 'Consultor',
      },
    ])
    planning.syncGoalReferences('store-real', '2026-07', [
      {
        scope: 'store',
        storeId: 'store-real',
        month: '2026-07',
        week: 0,
        monthlyGoal: 120_000,
      },
      {
        scope: 'store',
        storeId: 'store-real',
        month: '2026-07',
        week: 2,
        monthlyGoal: 30_000,
      },
    ])

    planning.setGoalReference('2026-07', 'p2')
    expect(planning.selectedTarget).toBe(30_000)
    planning.applyPersistedSchedule({
      id: 'schedule-1',
      storeId: 'store-real',
      weekStart: planning.weekStart,
      targetMonth: '2026-07',
      goalWeek: 2,
      status: 'saved',
      shifts: [],
      goalAllocations: [
        { staffId: 'staff-one', scheduledHours: 44, weightedHours: 44, share: 0.5, target: 15_000 },
        { staffId: 'staff-two', scheduledHours: 44, weightedHours: 44, share: 0.5, target: 15_000 },
      ],
      version: 1,
      updatedAt: '2026-07-06T12:00:00Z',
      issues: [],
    })

    const allocated = planning.allocations.reduce((total, row) => total + row.target, 0)
    expect(allocated).toBeCloseTo(30_000, 2)
    expect(planning.allocations.every((row) => row.scheduledHours > 0)).toBe(true)
  })

  it('keeps the authoritative consultant nickname updated in planning', () => {
    const planning = usePlanningStore()
    planning.syncStoreReferences([
      { id: 'store-real', name: 'Loja Real', city: 'Aracaju', storeType: 'shopping' },
    ])
    planning.syncStaffReferences('store-real', [
      {
        id: 'staff-one',
        storeId: 'store-real',
        name: 'Daiane Caroline dos Santos',
        nick: 'Daiane C.',
        role: 'Consultora',
      },
    ])
    planning.syncStaffReferences('store-real', [
      {
        id: 'staff-one',
        storeId: 'store-real',
        name: 'Daiane Caroline dos Santos',
        nick: 'Dai C.',
        role: 'Consultora',
      },
    ])

    expect(planning.activeStaff[0]?.nick).toBe('Dai C.')
  })

  it('derives missing weekly goals from the authoritative monthly goal', () => {
    const planning = usePlanningStore()
    planning.syncStoreReferences([
      { id: 'store-monthly', name: 'Loja Mensal', city: 'Aracaju', storeType: 'shopping' },
    ])
    planning.syncGoalReferences('store-monthly', '2026-07', [
      {
        scope: 'store',
        storeId: 'store-monthly',
        month: '2026-07',
        week: 0,
        monthlyGoal: 125_000,
      },
    ])

    for (const period of ['p1', 'p2', 'p3', 'p4', 'p5'] as const) {
      planning.setGoalReference('2026-07', period)
      expect(planning.selectedTarget).toBe(25_000)
    }
  })

  it('removes shifts from a day when operating hours close that day', () => {
    const planning = usePlanningStore()
    planning.setGoalReference('2026-07', 'p4')

    const monday = planning.weekDates.find((date) => date.weekday === 'mon')
    expect(monday).toBeDefined()
    expect(planning.assignStaffToDay(planning.activeStaff[0]!.id, monday!.isoDate)).toBe(true)
    expect(planning.activeShifts.some((shift) => shift.isoDate === monday?.isoDate)).toBe(true)

    planning.updateOperatingDay('mon', { isOpen: false })

    expect(planning.activeShifts.some((shift) => shift.isoDate === monday?.isoDate)).toBe(false)
  })

  it('fits an existing shift to changed operating hours', () => {
    const planning = usePlanningStore()
    planning.setGoalReference('2026-07', 'p4')
    const monday = planning.weekDates.find((date) => date.weekday === 'mon')!
    const staff = planning.activeStaff[0]!

    planning.setShift(staff.id, monday.isoDate, 'closing')
    planning.updateOperatingDay('mon', { closesAt: '20:00' })

    const shift = planning.activeShifts.find(
      (item) => item.staffId === staff.id && item.isoDate === monday.isoDate,
    )!
    expect(shift.endsAt).toBe('20:00')
    expect(shiftHours(shift)).toBeLessThanOrEqual(staff.maxDailyHours)
  })

  it('places a shift on another day and lane in one mutation', () => {
    const planning = usePlanningStore()
    planning.setGoalReference('2026-07', 'p4')
    const monday = planning.weekDates.find((date) => date.weekday === 'mon')!
    const tuesday = planning.weekDates.find((date) => date.weekday === 'tue')!
    const staff = planning.activeStaff[0]!

    planning.setShift(staff.id, monday.isoDate, 'opening')

    expect(planning.placeShift(staff.id, monday.isoDate, tuesday.isoDate, 'closing')).toBe(true)
    expect(planning.shiftFor(staff.id, monday.isoDate)).toBeUndefined()
    expect(planning.shiftFor(staff.id, tuesday.isoDate)?.templateId).toBe('closing')
  })

  it('keeps previously loaded weeks available to the monthly calendar preview', () => {
    const planning = usePlanningStore()
    const staff = planning.activeStaff[0]!

    planning.setGoalReference('2026-07', 'p1')
    const firstWeekDate = planning.weekDates[0]!.isoDate
    planning.assignStaffToDay(staff.id, firstWeekDate)

    planning.setGoalReference('2026-07', 'p2')
    const secondWeekDate = planning.weekDates[0]!.isoDate
    planning.assignStaffToDay(staff.id, secondWeekDate)

    expect(planning.activeShifts.map((shift) => shift.isoDate)).toEqual([secondWeekDate])
    expect(planning.storeShifts.map((shift) => shift.isoDate).sort()).toEqual(
      [firstWeekDate, secondWeekDate].sort(),
    )
  })

  it('rebuilds existing shifts when the employee daily limit changes', () => {
    const planning = usePlanningStore()
    planning.setGoalReference('2026-07', 'p4')
    const monday = planning.weekDates.find((date) => date.weekday === 'mon')!
    const staff = planning.activeStaff[0]!

    expect(planning.assignStaffToDay(staff.id, monday.isoDate)).toBe(true)
    planning.updateStaffMember(staff.id, { maxDailyHours: 6 })

    const shift = planning.activeShifts.find(
      (item) => item.staffId === staff.id && item.isoDate === monday.isoDate,
    )!
    expect(shiftHours(shift)).toBe(6)
  })

  it('rebuilds existing shifts when the active labor policy changes', () => {
    const planning = usePlanningStore()
    planning.setGoalReference('2026-07', 'p4')
    const monday = planning.weekDates.find((date) => date.weekday === 'mon')!
    const staff = planning.activeStaff[0]!

    expect(planning.assignStaffToDay(staff.id, monday.isoDate)).toBe(true)
    planning.updatePolicy({ maxDailyHours: 5 })

    const shift = planning.activeShifts.find(
      (item) => item.staffId === staff.id && item.isoDate === monday.isoDate,
    )!
    expect(shiftHours(shift)).toBe(5)
  })

  it('allows manual Sunday shifts when shopping is open', () => {
    const planning = usePlanningStore()
    planning.syncStoreReferences([
      { id: 'shopping-real', name: 'Shopping Real', city: 'Aracaju', storeType: 'shopping' },
    ])
    planning.syncStaffReferences('shopping-real', [
      {
        id: 'staff-one',
        storeId: 'shopping-real',
        name: 'Pessoa Um',
        role: 'Consultor',
      },
      {
        id: 'staff-two',
        storeId: 'shopping-real',
        name: 'Pessoa Dois',
        role: 'Consultor',
      },
    ])
    planning.setGoalReference('2026-07', 'p1')

    const sunday = planning.weekDates.find((date) => date.weekday === 'sun')
    expect(sunday).toBeDefined()
    expect(planning.assignStaffToDay('staff-one', sunday!.isoDate)).toBe(true)
    expect(planning.activeShifts.some((shift) => shift.isoDate === sunday?.isoDate)).toBe(true)
  })

  it('moves an existing shift between lanes on the same day', () => {
    const planning = usePlanningStore()
    planning.setGoalReference('2026-07', 'p4')
    const monday = planning.weekDates.find((date) => date.weekday === 'mon')!
    const staff = planning.activeStaff[0]!

    planning.setShift(staff.id, monday.isoDate, 'opening')

    expect(planning.placeShift(staff.id, monday.isoDate, monday.isoDate, 'closing')).toBe(true)
    expect(planning.shiftFor(staff.id, monday.isoDate)?.templateId).toBe('closing')
    expect(
      planning.activeShifts.filter(
        (shift) => shift.staffId === staff.id && shift.isoDate === monday.isoDate,
      ),
    ).toHaveLength(1)
  })

  it('persists coverage, rotation, holidays and dated absences in the planning configuration', () => {
    const planning = usePlanningStore()
    const staffId = planning.activeStaff[0]!.id
    planning.updateCoverageRule('shopping', { openingMinimum: 3, enabled: true })
    planning.updateStaffMember(staffId, { alternateSundays: true, sundayRotationOffset: 1 })
    planning.addHoliday({
      isoDate: '2026-08-07',
      name: 'Feriado local',
      isOpen: false,
      opensAt: '10:00',
      closesAt: '18:00',
    })
    planning.addStaffException({
      id: 'absence-1',
      staffId,
      isoDate: '2026-08-03',
      kind: 'vacation',
      allDay: true,
      startsAt: '09:00',
      endsAt: '18:00',
      notes: '',
    })
    planning.setShowWorkspaceHero(false)

    const configuration = planning.persistedConfiguration()
    expect(configuration?.coverageByLocationType.shopping.openingMinimum).toBe(3)
    expect(configuration?.staff.find((member) => member.id === staffId)).toMatchObject({
      alternateSundays: true,
      sundayRotationOffset: 1,
    })
    expect(configuration?.holidays).toHaveLength(1)
    expect(configuration?.exceptions).toHaveLength(1)
    expect(configuration?.uiPreferences?.showWorkspaceHero).toBe(false)
  })
})
