import { computed, type ComputedRef } from 'vue'

import {
  isShiftTemplateValid,
  shiftHours,
  shiftTemplatesForStore,
} from '~/domain/planning/scheduler'
import type {
  PlanningIssue,
  PlanningShift,
  PlanningStaffMember,
  PlanningStore,
} from '~/domain/planning/types'

export function usePlanningMetrics(
  activeStore: ComputedRef<PlanningStore | undefined>,
  activeStaff: ComputedRef<PlanningStaffMember[]>,
  activeShifts: ComputedRef<PlanningShift[]>,
  issues: ComputedRef<PlanningIssue[]>,
) {
  const scheduledHours = computed(() =>
    activeShifts.value.reduce((total, shift) => total + shiftHours(shift), 0),
  )
  const contractedHours = computed(() =>
    activeStaff.value.reduce((total, member) => total + member.weeklyHours, 0),
  )
  const hardIssueCount = computed(
    () => issues.value.filter((issue) => issue.severity === 'hard').length,
  )
  const warningCount = computed(
    () => issues.value.filter((issue) => issue.severity === 'warning').length,
  )
  const openDayCount = computed(
    () =>
      activeStore.value?.operatingHoursByLocationType[activeStore.value.locationType].filter(
        (day) => day.isOpen,
      ).length || 0,
  )
  const coveredDayCount = computed(
    () =>
      new Set(
        activeShifts.value.filter((shift) => shiftHours(shift) > 0).map((shift) => shift.isoDate),
      ).size,
  )
  const invalidShiftTemplateCount = computed(() =>
    activeStore.value
      ? shiftTemplatesForStore(activeStore.value).filter(
          (template) => !isShiftTemplateValid(template),
        ).length
      : 0,
  )

  return {
    scheduledHours,
    contractedHours,
    hardIssueCount,
    warningCount,
    openDayCount,
    coveredDayCount,
    invalidShiftTemplateCount,
  }
}
