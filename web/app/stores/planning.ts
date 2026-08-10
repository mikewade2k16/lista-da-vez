import { computed, ref } from 'vue'
import { defineStore } from 'pinia'

import { createPlanningFixtures } from '~/domain/planning/fixtures'
import {
  buildWeekDates,
  currentPlanningMonth,
  deriveShiftTemplatesFromOperatingHours,
  isShiftTemplateValid,
  shiftTemplatesForStore,
  weekStartForGoalPeriod,
} from '~/domain/planning/scheduler'
import {
  movePlanningShift,
  rebuildPlanningShifts,
  setPlanningShift,
  swapPlanningShifts,
} from '~/domain/planning/schedule-mutations'
import type {
  PlanningGoalPeriod,
  PlanningLaborPolicy,
  PlanningOperatingDay,
  PlanningShift,
  PlanningShiftTemplate,
  PlanningStaffMember,
  ShiftTemplateId,
  StoreLocationType,
  WeekdayId,
  WorkShiftTemplateId,
} from '~/domain/planning/types'
import { usePlanningScheduleState } from '~/stores/planning-schedule-state'
import { usePlanningMetrics } from '~/stores/planning-metrics'
import { usePlanningReferenceState } from '~/stores/planning-reference-state'
import { usePlanningRuleActions } from '~/stores/planning-rule-actions'

export const usePlanningStore = defineStore('planning', () => {
  const fixtures = createPlanningFixtures()
  const stores = ref(fixtures.stores)
  const staff = ref(fixtures.staff)
  const policies = ref(fixtures.policies)
  const activeStoreId = ref(stores.value[0]?.id || '')
  const activePolicyId = ref(policies.value[0]?.id || '')
  const selectedMonth = ref(currentPlanningMonth())
  const selectedPeriod = ref<PlanningGoalPeriod>('p1')
  const weekStart = ref(weekStartForGoalPeriod(selectedMonth.value, selectedPeriod.value))
  const shifts = ref<PlanningShift[]>([])
  const showWorkspaceHero = ref(true)

  const activeStore = computed(() => stores.value.find((store) => store.id === activeStoreId.value))
  const activePolicy = computed(() =>
    policies.value.find((policy) => policy.id === activePolicyId.value),
  )
  const activeStaff = computed(() =>
    staff.value.filter((member) => member.storeId === activeStoreId.value && member.active),
  )
  const weekDates = computed(() => buildWeekDates(weekStart.value))
  const activeDateKeys = computed(() => new Set(weekDates.value.map((date) => date.isoDate)))
  const activeStaffIDs = computed(() => new Set(activeStaff.value.map((member) => member.id)))
  const activeShifts = computed(() =>
    shifts.value.filter(
      (shift) => activeStaffIDs.value.has(shift.staffId) && activeDateKeys.value.has(shift.isoDate),
    ),
  )
  const storeShifts = computed(() =>
    shifts.value.filter((shift) => activeStaffIDs.value.has(shift.staffId)),
  )
  const {
    scheduleStatus,
    scheduleVersion,
    issues,
    allocations,
    resetScheduleLoading,
    applyPersistedSchedule,
    replaceActiveShifts,
    markScheduleSaving,
    markScheduleSaved,
    markScheduleUnsaved,
  } = usePlanningScheduleState(shifts, activeStaffIDs, activeDateKeys)
  const {
    syncStoreReferences,
    syncStaffReferences,
    applyStaffContracts,
    syncGoalReferences,
    persistedConfiguration,
    resetActiveConfiguration,
    applyPersistedConfiguration,
  } = usePlanningReferenceState({
    stores,
    staff,
    policies,
    shifts,
    activeStoreId,
    activePolicyId,
    activeStore,
    activeStaff,
    showWorkspaceHero,
    resetScheduleLoading,
  })
  const { updateCoverageRule, addHoliday, removeHoliday, addStaffException, removeStaffException } =
    usePlanningRuleActions(activeStore, scheduleStatus)
  const selectedTarget = computed(
    () => activeStore.value?.goalsByMonth[selectedMonth.value]?.[selectedPeriod.value] || 0,
  )
  const {
    scheduledHours,
    contractedHours,
    hardIssueCount,
    warningCount,
    openDayCount,
    coveredDayCount,
    invalidShiftTemplateCount,
  } = usePlanningMetrics(activeStore, activeStaff, activeShifts, issues)

  function setActiveStore(storeId: string) {
    if (stores.value.some((store) => store.id === storeId)) {
      activeStoreId.value = storeId
      resetScheduleLoading()
    }
  }

  function setActivePolicy(policyId: string) {
    if (policies.value.some((policy) => policy.id === policyId)) {
      activePolicyId.value = policyId
      rebuildActiveShifts()
      scheduleStatus.value = 'unsaved'
    }
  }

  function rebuildActiveShifts(templateId?: WorkShiftTemplateId) {
    const store = activeStore.value
    const policy = activePolicy.value
    if (!store || !policy) return

    shifts.value = rebuildPlanningShifts(
      shifts.value,
      { store, policy, staff: activeStaff.value, dates: weekDates.value },
      activeStaffIDs.value,
      activeDateKeys.value,
      templateId,
    )
  }

  function updateLocationType(value: StoreLocationType) {
    if (activeStore.value) {
      activeStore.value.locationType = value
      rebuildActiveShifts()
      scheduleStatus.value = 'unsaved'
    }
  }

  function setGoalReference(month: string, period: PlanningGoalPeriod) {
    if (/^\d{4}-\d{2}$/.test(month)) {
      selectedMonth.value = month
    }
    selectedPeriod.value = period
    weekStart.value = weekStartForGoalPeriod(selectedMonth.value, period)
    scheduleStatus.value = 'loading'
  }

  function updateOperatingDay(weekday: WeekdayId, patch: Partial<PlanningOperatingDay>) {
    const store = activeStore.value
    const day = store?.operatingHoursByLocationType[store.locationType].find(
      (item) => item.weekday === weekday,
    )
    if (!day) {
      return
    }
    Object.assign(day, patch)
    const locationType = store.locationType
    store.shiftTemplatesByLocationType[locationType] = deriveShiftTemplatesFromOperatingHours(
      store.operatingHoursByLocationType[locationType],
      store.shiftTemplatesByLocationType[locationType],
    )
    rebuildActiveShifts()
    scheduleStatus.value = 'unsaved'
  }

  function updateShiftTemplate(
    locationType: StoreLocationType,
    templateId: WorkShiftTemplateId,
    patch: Pick<PlanningShiftTemplate, 'name'>,
  ) {
    const template = activeStore.value?.shiftTemplatesByLocationType[locationType].find(
      (item) => item.id === templateId,
    )
    if (!template) {
      return
    }
    Object.assign(template, patch, { id: templateId })

    if (activeStore.value?.locationType === locationType) {
      rebuildActiveShifts(templateId)
    }
    scheduleStatus.value = 'unsaved'
  }

  function updatePolicy(patch: Partial<PlanningLaborPolicy>) {
    if (!activePolicy.value) {
      return
    }
    Object.assign(activePolicy.value, patch)
    rebuildActiveShifts()
    scheduleStatus.value = 'unsaved'
  }

  function updateStaffMember(staffId: string, patch: Partial<PlanningStaffMember>) {
    const member = staff.value.find((item) => item.id === staffId)
    if (!member || member.storeId !== activeStoreId.value) {
      return
    }
    Object.assign(member, patch)
    if (patch.maxDailyHours !== undefined) rebuildActiveShifts()
    scheduleStatus.value = 'unsaved'
  }

  function toggleStaffAvailability(staffId: string, weekday: WeekdayId) {
    const member = staff.value.find((item) => item.id === staffId)
    if (!member) {
      return
    }
    member.availableDays = member.availableDays.includes(weekday)
      ? member.availableDays.filter((day) => day !== weekday)
      : [...member.availableDays, weekday]
    scheduleStatus.value = 'unsaved'
  }

  function setShift(staffId: string, isoDate: string, templateId: ShiftTemplateId) {
    if (!activeStore.value || !activePolicy.value) return
    const next = setPlanningShift(
      shifts.value,
      {
        store: activeStore.value,
        policy: activePolicy.value,
        staff: activeStaff.value,
        dates: weekDates.value,
      },
      staffId,
      isoDate,
      templateId,
    )
    if (!next) return
    shifts.value = next
    scheduleStatus.value = 'unsaved'
  }

  function assignStaffToDay(staffId: string, isoDate: string): boolean {
    if (shiftFor(staffId, isoDate)) return false
    const firstTemplate = activeStore.value
      ? shiftTemplatesForStore(activeStore.value).find(isShiftTemplateValid)
      : undefined
    if (!firstTemplate) return false
    setShift(staffId, isoDate, firstTemplate.id)
    return Boolean(shiftFor(staffId, isoDate))
  }

  function moveShift(staffId: string, fromDate: string, toDate: string): boolean {
    if (!activeStore.value || !activePolicy.value) return false
    const next = movePlanningShift(
      activeShifts.value,
      {
        store: activeStore.value,
        policy: activePolicy.value,
        staff: activeStaff.value,
        dates: weekDates.value,
      },
      staffId,
      fromDate,
      toDate,
    )
    if (!next) return false
    shifts.value = [
      ...shifts.value.filter(
        (shift) =>
          !activeStaffIDs.value.has(shift.staffId) || !activeDateKeys.value.has(shift.isoDate),
      ),
      ...next,
    ]
    scheduleStatus.value = 'unsaved'
    return true
  }

  function placeShift(
    staffId: string,
    fromDate: string,
    toDate: string,
    templateId: ShiftTemplateId,
  ): boolean {
    const previousShifts = shifts.value.map((shift) => ({ ...shift }))
    const previousStatus = scheduleStatus.value
    if (fromDate && fromDate !== toDate && !moveShift(staffId, fromDate, toDate)) return false
    setShift(staffId, toDate, templateId)
    const placed = shiftFor(staffId, toDate)?.templateId === templateId
    if (!placed) {
      shifts.value = previousShifts
      scheduleStatus.value = previousStatus
    }
    return placed
  }

  function swapShifts(
    sourceStaffId: string,
    sourceDate: string,
    targetStaffId: string,
    targetDate: string,
  ): boolean {
    if (!activeStore.value || !activePolicy.value) return false
    const next = swapPlanningShifts(
      activeShifts.value,
      {
        store: activeStore.value,
        policy: activePolicy.value,
        staff: activeStaff.value,
        dates: weekDates.value,
      },
      sourceStaffId,
      sourceDate,
      targetStaffId,
      targetDate,
    )
    if (!next) return false
    shifts.value = [
      ...shifts.value.filter(
        (shift) =>
          !activeStaffIDs.value.has(shift.staffId) || !activeDateKeys.value.has(shift.isoDate),
      ),
      ...next,
    ]
    scheduleStatus.value = 'unsaved'
    return true
  }

  function shiftFor(staffId: string, isoDate: string): PlanningShift | undefined {
    return activeShifts.value.find(
      (shift) => shift.staffId === staffId && shift.isoDate === isoDate,
    )
  }

  return {
    stores,
    policies,
    activeStoreId,
    activePolicyId,
    weekStart,
    selectedMonth,
    selectedPeriod,
    scheduleStatus,
    scheduleVersion,
    activeStore,
    activePolicy,
    activeStaff,
    weekDates,
    activeShifts,
    storeShifts,
    issues,
    allocations,
    showWorkspaceHero,
    selectedTarget,
    scheduledHours,
    contractedHours,
    hardIssueCount,
    warningCount,
    openDayCount,
    coveredDayCount,
    invalidShiftTemplateCount,
    setActiveStore,
    setActivePolicy,
    setShowWorkspaceHero(value: boolean) {
      showWorkspaceHero.value = value
      scheduleStatus.value = 'unsaved'
    },
    updateLocationType,
    setGoalReference,
    updateOperatingDay,
    updateShiftTemplate,
    syncStoreReferences,
    syncStaffReferences,
    applyStaffContracts,
    syncGoalReferences,
    updatePolicy,
    updateCoverageRule,
    addHoliday,
    removeHoliday,
    addStaffException,
    removeStaffException,
    updateStaffMember,
    toggleStaffAvailability,
    setShift,
    assignStaffToDay,
    moveShift,
    placeShift,
    swapShifts,
    shiftFor,
    persistedConfiguration,
    resetActiveConfiguration,
    applyPersistedConfiguration,
    applyPersistedSchedule,
    replaceActiveShifts,
    markScheduleSaving,
    markScheduleSaved,
    markScheduleUnsaved,
  }
})
