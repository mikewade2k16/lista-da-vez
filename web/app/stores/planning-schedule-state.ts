import { computed, ref, type ComputedRef, type Ref } from 'vue'

import { replacePlanningShifts } from '~/domain/planning/schedule-mutations'
import type {
  PlanningPersistedSchedule,
  PlanningShift,
  ScheduleStatus,
} from '~/domain/planning/types'

export function usePlanningScheduleState(
  shifts: Ref<PlanningShift[]>,
  activeStaffIDs: ComputedRef<Set<string>>,
  activeDateKeys: ComputedRef<Set<string>>,
) {
  const scheduleStatus = ref<ScheduleStatus>('loading')
  const scheduleVersion = ref<number>()
  const persistedGoalAllocations = ref<PlanningPersistedSchedule['goalAllocations']>([])
  const persistedIssues = ref<PlanningPersistedSchedule['issues']>([])
  const issues = computed(() => persistedIssues.value)
  const allocations = computed(() => persistedGoalAllocations.value)

  function resetScheduleLoading() {
    scheduleStatus.value = 'loading'
    scheduleVersion.value = undefined
    persistedGoalAllocations.value = []
    persistedIssues.value = []
  }

  function applyPersistedSchedule(schedule?: PlanningPersistedSchedule | null) {
    shifts.value = shifts.value.filter(
      (shift) =>
        !activeStaffIDs.value.has(shift.staffId) || !activeDateKeys.value.has(shift.isoDate),
    )
    if (schedule) shifts.value.push(...structuredClone(schedule.shifts))
    scheduleStatus.value = schedule?.status || 'unsaved'
    scheduleVersion.value = schedule?.version
    persistedGoalAllocations.value = structuredClone(schedule?.goalAllocations || [])
    persistedIssues.value = structuredClone(schedule?.issues || [])
  }

  function replaceActiveShifts(nextShifts: PlanningShift[]) {
    shifts.value = replacePlanningShifts(
      shifts.value,
      nextShifts,
      activeStaffIDs.value,
      activeDateKeys.value,
    )
    markScheduleUnsaved()
  }

  function markScheduleSaving() {
    scheduleStatus.value = 'saving'
  }

  function markScheduleSaved(schedule: PlanningPersistedSchedule) {
    scheduleStatus.value = schedule.status
    scheduleVersion.value = schedule.version
    persistedGoalAllocations.value = structuredClone(schedule.goalAllocations || [])
    persistedIssues.value = structuredClone(schedule.issues || [])
  }

  function markScheduleUnsaved() {
    scheduleStatus.value = 'unsaved'
    persistedGoalAllocations.value = []
    persistedIssues.value = []
  }

  return {
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
  }
}
