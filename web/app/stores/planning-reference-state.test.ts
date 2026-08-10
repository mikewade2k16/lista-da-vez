import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it } from 'vitest'

import { usePlanningStore } from './planning'

describe('planning reference state', () => {
  beforeEach(() => setActivePinia(createPinia()))

  it('clears fixtures and prior context when the authoritative store list is empty', () => {
    const planning = usePlanningStore()
    expect(planning.stores.length).toBeGreaterThan(0)

    planning.syncStoreReferences([])

    expect(planning.stores).toEqual([])
    expect(planning.activeStore).toBeUndefined()
    expect(planning.activeStaff).toEqual([])
    expect(planning.storeShifts).toEqual([])
    expect(planning.persistedConfiguration()).toBeUndefined()
    expect(planning.scheduleStatus).toBe('loading')
  })

  it('drops configuration and shifts when references move to another account', () => {
    const planning = usePlanningStore()
    planning.syncStoreReferences([
      { id: 'account-a-store', name: 'Conta A', city: 'Aracaju', storeType: 'shopping' },
    ])
    planning.syncStaffReferences('account-a-store', [
      { id: 'account-a-staff', storeId: 'account-a-store', name: 'Pessoa A', role: 'Consultora' },
    ])
    planning.updatePolicy({ maxDailyHours: 5 })
    planning.setGoalReference('2026-07', 'p1')
    planning.assignStaffToDay('account-a-staff', planning.weekDates[0]!.isoDate)

    planning.syncStoreReferences([
      { id: 'account-b-store', name: 'Conta B', city: 'Maceio', storeType: 'bairro' },
    ])

    expect(planning.activeStore?.id).toBe('account-b-store')
    expect(planning.activeStaff).toEqual([])
    expect(planning.storeShifts).toEqual([])
    expect(planning.activePolicy?.maxDailyHours).toBe(8)
    expect(planning.scheduleStatus).toBe('loading')
  })

  it('resets configuration before applying an authoritative snapshot', () => {
    const planning = usePlanningStore()
    const staffId = planning.activeStaff[0]!.id
    planning.updatePolicy({ maxDailyHours: 5 })
    planning.updateStaffMember(staffId, { worksHolidays: false })
    planning.addHoliday({
      isoDate: '2026-08-07',
      name: 'Local',
      isOpen: false,
      opensAt: '09:00',
      closesAt: '18:00',
    })
    planning.setShowWorkspaceHero(false)

    planning.resetActiveConfiguration()

    expect(planning.activePolicy?.maxDailyHours).toBe(8)
    expect(planning.activeStaff[0]?.worksHolidays).toBe(true)
    expect(planning.activeStore?.holidays).toEqual([])
    expect(planning.showWorkspaceHero).toBe(true)
  })
})
