import { buildShiftFromTemplate } from './scheduler'
import type {
  PlanningLaborPolicy,
  PlanningShift,
  PlanningStaffMember,
  PlanningStore,
  PlanningWeekDate,
  ShiftTemplateId,
  WorkShiftTemplateId,
} from './types'

interface ScheduleContext {
  store: PlanningStore
  policy: PlanningLaborPolicy
  staff: PlanningStaffMember[]
  dates: PlanningWeekDate[]
}

export function replacePlanningShifts(
  shifts: PlanningShift[],
  next: PlanningShift[],
  activeStaffIDs: Set<string>,
  activeDateKeys: Set<string>,
): PlanningShift[] {
  return [
    ...shifts.filter(
      (shift) => !activeStaffIDs.has(shift.staffId) || !activeDateKeys.has(shift.isoDate),
    ),
    ...structuredClone(next),
  ]
}

export function rebuildPlanningShifts(
  shifts: PlanningShift[],
  context: ScheduleContext,
  activeStaffIDs: Set<string>,
  activeDateKeys: Set<string>,
  templateId?: WorkShiftTemplateId,
): PlanningShift[] {
  return shifts.flatMap((shift) => {
    if (
      !activeStaffIDs.has(shift.staffId) ||
      !activeDateKeys.has(shift.isoDate) ||
      (templateId && shift.templateId !== templateId)
    )
      return [shift]
    const member = context.staff.find((item) => item.id === shift.staffId)
    const date = context.dates.find((item) => item.isoDate === shift.isoDate)
    if (!member || !date) return []
    const rebuilt = buildShiftFromTemplate(
      context.store,
      member,
      context.policy,
      date,
      shift.templateId,
    )
    return rebuilt ? [rebuilt] : []
  })
}

export function setPlanningShift(
  shifts: PlanningShift[],
  context: ScheduleContext,
  staffId: string,
  isoDate: string,
  templateId: ShiftTemplateId,
): PlanningShift[] | undefined {
  const member = context.staff.find((item) => item.id === staffId)
  const date = context.dates.find((item) => item.isoDate === isoDate)
  if (!member || !date) return undefined
  const next = shifts.filter((shift) => !(shift.staffId === staffId && shift.isoDate === isoDate))
  const built = buildShiftFromTemplate(context.store, member, context.policy, date, templateId)
  if (built) next.push(built)
  return next
}

export function movePlanningShift(
  shifts: PlanningShift[],
  context: ScheduleContext,
  staffId: string,
  fromDate: string,
  toDate: string,
): PlanningShift[] | undefined {
  const source = shifts.find((shift) => shift.staffId === staffId && shift.isoDate === fromDate)
  if (
    !source ||
    fromDate === toDate ||
    shifts.some((shift) => shift.staffId === staffId && shift.isoDate === toDate)
  )
    return undefined
  const member = context.staff.find((item) => item.id === staffId)
  const date = context.dates.find((item) => item.isoDate === toDate)
  if (!member || !date) return undefined
  const moved = buildShiftFromTemplate(
    context.store,
    member,
    context.policy,
    date,
    source.templateId,
  )
  if (!moved) return undefined
  return [
    ...shifts.filter((shift) => !(shift.staffId === staffId && shift.isoDate === fromDate)),
    moved,
  ]
}

export function swapPlanningShifts(
  shifts: PlanningShift[],
  context: ScheduleContext,
  sourceStaffId: string,
  sourceDate: string,
  targetStaffId: string,
  targetDate: string,
): PlanningShift[] | undefined {
  const source = shifts.find(
    (shift) => shift.staffId === sourceStaffId && shift.isoDate === sourceDate,
  )
  const target = shifts.find(
    (shift) => shift.staffId === targetStaffId && shift.isoDate === targetDate,
  )
  if (!source || !target || sourceDate === targetDate) return undefined
  const sourceMember = context.staff.find((item) => item.id === sourceStaffId)
  const targetMember = context.staff.find((item) => item.id === targetStaffId)
  const sourceTargetDate = context.dates.find((item) => item.isoDate === targetDate)
  const targetSourceDate = context.dates.find((item) => item.isoDate === sourceDate)
  if (!sourceMember || !targetMember || !sourceTargetDate || !targetSourceDate) return undefined
  const movedSource = buildShiftFromTemplate(
    context.store,
    sourceMember,
    context.policy,
    sourceTargetDate,
    source.templateId,
  )
  const movedTarget = buildShiftFromTemplate(
    context.store,
    targetMember,
    context.policy,
    targetSourceDate,
    target.templateId,
  )
  if (!movedSource || !movedTarget) return undefined
  return [
    ...shifts.filter(
      (shift) =>
        !(shift.staffId === sourceStaffId && shift.isoDate === sourceDate) &&
        !(shift.staffId === targetStaffId && shift.isoDate === targetDate),
    ),
    movedSource,
    movedTarget,
  ]
}
