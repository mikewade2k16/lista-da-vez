import type { ComputedRef, Ref } from 'vue'

import type {
  PlanningCoverageRule,
  PlanningHoliday,
  PlanningStaffException,
  PlanningStore,
  ScheduleStatus,
  StoreLocationType,
} from '~/domain/planning/types'

export function usePlanningRuleActions(
  activeStore: ComputedRef<PlanningStore | undefined>,
  scheduleStatus: Ref<ScheduleStatus>,
) {
  function markChanged() {
    scheduleStatus.value = 'unsaved'
  }

  function updateCoverageRule(
    locationType: StoreLocationType,
    patch: Partial<PlanningCoverageRule>,
  ) {
    const store = activeStore.value
    if (!store) return
    Object.assign(store.coverageByLocationType[locationType], patch)
    markChanged()
  }

  function addHoliday(holiday: PlanningHoliday) {
    const store = activeStore.value
    if (!store || store.holidays.some((item) => item.isoDate === holiday.isoDate)) return
    store.holidays.push(structuredClone(holiday))
    markChanged()
  }

  function removeHoliday(isoDate: string) {
    const store = activeStore.value
    if (!store) return
    store.holidays = store.holidays.filter((holiday) => holiday.isoDate !== isoDate)
    markChanged()
  }

  function addStaffException(exception: PlanningStaffException) {
    const store = activeStore.value
    if (!store || store.exceptions.some((item) => item.id === exception.id)) return
    store.exceptions.push(structuredClone(exception))
    markChanged()
  }

  function removeStaffException(exceptionId: string) {
    const store = activeStore.value
    if (!store) return
    store.exceptions = store.exceptions.filter((exception) => exception.id !== exceptionId)
    markChanged()
  }

  return {
    updateCoverageRule,
    addHoliday,
    removeHoliday,
    addStaffException,
    removeStaffException,
  }
}
