export const WEEKDAY_DEFINITIONS = [
  { id: 'mon', shortLabel: 'Seg', label: 'Segunda' },
  { id: 'tue', shortLabel: 'Ter', label: 'Terça' },
  { id: 'wed', shortLabel: 'Qua', label: 'Quarta' },
  { id: 'thu', shortLabel: 'Qui', label: 'Quinta' },
  { id: 'fri', shortLabel: 'Sex', label: 'Sexta' },
  { id: 'sat', shortLabel: 'Sáb', label: 'Sábado' },
  { id: 'sun', shortLabel: 'Dom', label: 'Domingo' },
] as const

export type WeekdayId = (typeof WEEKDAY_DEFINITIONS)[number]['id']
export type StoreLocationType = 'shopping' | 'street'
export type ShiftTemplateId = 'off' | 'opening' | 'middle' | 'closing'
export type WorkShiftTemplateId = Exclude<ShiftTemplateId, 'off'>
export type ScheduleStatus = 'loading' | 'unsaved' | 'saving' | 'saved' | 'published'
export type PlanningIssueSeverity = 'hard' | 'warning'
export type PlanningSectionId = 'operation' | 'schedule' | 'goals'
export type PlanningGoalPeriod = 'month' | 'p1' | 'p2' | 'p3' | 'p4' | 'p5'
export type PlanningExceptionKind =
  | 'vacation'
  | 'medical_leave'
  | 'training'
  | 'meeting'
  | 'time_bank'
  | 'exceptional_day_off'

export type PlanningGoalTargets = Record<PlanningGoalPeriod, number>

export interface PlanningOperatingDay {
  weekday: WeekdayId
  isOpen: boolean
  opensAt: string
  closesAt: string
}

export type PlanningOperatingHoursByLocationType = Record<StoreLocationType, PlanningOperatingDay[]>

export interface PlanningShiftTemplate {
  id: WorkShiftTemplateId
  name: string
  startsAt: string
  endsAt: string
}

export type PlanningShiftTemplatesByLocationType = Record<
  StoreLocationType,
  PlanningShiftTemplate[]
>

export interface PlanningCoverageRule {
  enabled: boolean
  openingMinimum: number
  peakMinimum: number
  closingMinimum: number
  peakStartsAt: string
  peakEndsAt: string
}

export type PlanningCoverageByLocationType = Record<StoreLocationType, PlanningCoverageRule>

export interface PlanningHoliday {
  isoDate: string
  name: string
  isOpen: boolean
  opensAt: string
  closesAt: string
}

export interface PlanningStaffException {
  id: string
  staffId: string
  isoDate: string
  kind: PlanningExceptionKind
  allDay: boolean
  startsAt: string
  endsAt: string
  notes: string
}

export interface PlanningStore {
  id: string
  name: string
  city: string
  timezone: string
  locationType: StoreLocationType
  goalsByMonth: Record<string, PlanningGoalTargets>
  operatingHoursByLocationType: PlanningOperatingHoursByLocationType
  shiftTemplatesByLocationType: PlanningShiftTemplatesByLocationType
  coverageByLocationType: PlanningCoverageByLocationType
  holidays: PlanningHoliday[]
  exceptions: PlanningStaffException[]
}

export interface PlanningStoreReference {
  id: string
  name: string
  city: string
  storeType: 'shopping' | 'bairro'
}

export interface PlanningStaffMember {
  id: string
  storeId: string
  name: string
  nick?: string
  employeeCode: string
  jobRole: string
  weeklyHours: number
  maxDailyHours: number
  availableDays: WeekdayId[]
  targetWeight: number
  active: boolean
  worksSundays: boolean
  alternateSundays: boolean
  sundayRotationOffset: 0 | 1
  worksHolidays: boolean
}

export interface PlanningStaffReference {
  id: string
  storeId: string
  name: string
  nick?: string
  employeeCode?: string
  role: string
}

export interface PlanningGoalReference {
  scope: 'store' | 'consultant'
  storeId: string
  month: string
  week: number
  monthlyGoal: number
}

export interface PlanningLaborPolicy {
  id: string
  label: string
  maxDailyHours: number
  maxConsecutiveDays: number
  minDaysOff: number
  breakAfterHours: number
  minBreakMinutes: number
}

export interface PlanningWeekDate {
  isoDate: string
  weekday: WeekdayId
  shortLabel: string
  dayLabel: string
}

export interface PlanningShift {
  staffId: string
  isoDate: string
  templateId: WorkShiftTemplateId
  startsAt: string
  endsAt: string
  breakMinutes: number
}

export interface PlanningPersistedConfiguration {
  uiPreferences?: {
    showWorkspaceHero: boolean
  }
  activePolicyId: string
  operatingHoursByLocationType: PlanningOperatingHoursByLocationType
  shiftTemplatesByLocationType: PlanningShiftTemplatesByLocationType
  policies: PlanningLaborPolicy[]
  staff: PlanningStaffMember[]
  coverageByLocationType: PlanningCoverageByLocationType
  holidays: PlanningHoliday[]
  exceptions: PlanningStaffException[]
}

export interface PlanningPersistedSchedule {
  id: string
  storeId: string
  weekStart: string
  targetMonth: string
  goalWeek: number
  status: 'saved' | 'published'
  shifts: PlanningShift[]
  goalAllocations: PlanningGoalAllocation[]
  version: number
  publishedAt?: string
  updatedAt: string
  issues: PlanningIssue[]
}

export interface PlanningStaffContract {
  consultantId: string
  weeklyHours: number
  maxDailyHours: number
  targetWeight: number
  availableWeekdays: WeekdayId[]
  version: number
}

export interface PlanningIssue {
  id: string
  severity: PlanningIssueSeverity
  message: string
  staffId?: string
  isoDate?: string
}

export interface PlanningScheduleRevision {
  version: number
  status: 'saved' | 'published'
  changedByName: string
  createdAt: string
}

export interface PlanningGoalAllocation {
  staffId: string
  scheduledHours: number
  weightedHours: number
  share: number
  target: number
}

export interface PlanningFixtures {
  stores: PlanningStore[]
  staff: PlanningStaffMember[]
  policies: PlanningLaborPolicy[]
}
