import type {
  PlanningGoalPeriod,
  PlanningLaborPolicy,
  PlanningOperatingDay,
  PlanningShift,
  PlanningShiftTemplate,
  PlanningStaffMember,
  PlanningStore,
  PlanningWeekDate,
  ShiftTemplateId,
  WeekdayId,
  WorkShiftTemplateId,
} from './types'
import { WEEKDAY_DEFINITIONS } from './types'
import { goalWeekCount } from '~/utils/goal-periods'

const weekdayByJSIndex: WeekdayId[] = ['sun', 'mon', 'tue', 'wed', 'thu', 'fri', 'sat']

export function currentPlanningMonth(): string {
  const now = new Date()
  return `${now.getUTCFullYear()}-${String(now.getUTCMonth() + 1).padStart(2, '0')}`
}

function parseISODate(value: string): Date {
  return new Date(`${value}T12:00:00.000Z`)
}

function toISODate(value: Date): string {
  return value.toISOString().slice(0, 10)
}

export function normalizeWeekStart(value: string): string {
  const parsed = parseISODate(value)
  if (Number.isNaN(parsed.getTime())) {
    return '2026-07-27'
  }

  const day = parsed.getUTCDay()
  const distanceFromMonday = day === 0 ? 6 : day - 1
  parsed.setUTCDate(parsed.getUTCDate() - distanceFromMonday)
  return toISODate(parsed)
}

export function weekStartForGoalPeriod(month: string, period: PlanningGoalPeriod): string {
  const normalizedMonth = /^\d{4}-\d{2}$/.test(month) ? month : '2026-07'
  const periodIndex = period === 'month' ? 1 : Math.max(1, Number(period.slice(1)) || 1)
  const startDay = (Math.min(periodIndex, goalWeekCount(normalizedMonth)) - 1) * 7 + 1
  return normalizeWeekStart(`${normalizedMonth}-${String(startDay).padStart(2, '0')}`)
}

export function buildWeekDates(weekStart: string): PlanningWeekDate[] {
  const start = parseISODate(normalizeWeekStart(weekStart))
  return WEEKDAY_DEFINITIONS.map((definition, index) => {
    const date = new Date(start)
    date.setUTCDate(start.getUTCDate() + index)
    return {
      isoDate: toISODate(date),
      weekday: definition.id,
      shortLabel: definition.shortLabel,
      dayLabel: new Intl.DateTimeFormat('pt-BR', {
        day: '2-digit',
        month: '2-digit',
        timeZone: 'UTC',
      }).format(date),
    }
  })
}

function timeToMinutes(value: string): number {
  const [hour = 0, minute = 0] = value.split(':').map(Number)
  return hour * 60 + minute
}

function minutesToTime(value: number): string {
  const normalized = Math.max(0, Math.min(24 * 60 - 1, Math.round(value)))
  return `${String(Math.floor(normalized / 60)).padStart(2, '0')}:${String(normalized % 60).padStart(2, '0')}`
}

function operatingDay(store: PlanningStore, weekday: WeekdayId) {
  return store.operatingHoursByLocationType[store.locationType].find(
    (day) => day.weekday === weekday,
  )
}

export function isShiftTemplateValid(template: PlanningShiftTemplate): boolean {
  return (
    /^\d{2}:\d{2}$/.test(template.startsAt) &&
    /^\d{2}:\d{2}$/.test(template.endsAt) &&
    timeToMinutes(template.endsAt) > timeToMinutes(template.startsAt)
  )
}

export function deriveShiftTemplatesFromOperatingHours(
  days: PlanningOperatingDay[],
  existingTemplates: PlanningShiftTemplate[],
): PlanningShiftTemplate[] {
  const frequency = new Map<string, { count: number; opensAt: string; closesAt: string }>()

  days.forEach((day) => {
    if (
      !day.isOpen ||
      !/^\d{2}:\d{2}$/.test(day.opensAt) ||
      !/^\d{2}:\d{2}$/.test(day.closesAt) ||
      timeToMinutes(day.closesAt) <= timeToMinutes(day.opensAt)
    ) {
      return
    }
    const key = `${day.opensAt}-${day.closesAt}`
    const current = frequency.get(key)
    frequency.set(key, {
      count: (current?.count || 0) + 1,
      opensAt: day.opensAt,
      closesAt: day.closesAt,
    })
  })

  const predominant = [...frequency.values()].reduce<
    { count: number; opensAt: string; closesAt: string } | undefined
  >(
    (selected, candidate) => (!selected || candidate.count > selected.count ? candidate : selected),
    undefined,
  )
  if (!predominant) return existingTemplates

  const open = timeToMinutes(predominant.opensAt)
  const close = timeToMinutes(predominant.closesAt)
  const operatingSpan = close - open
  const shiftSpan = Math.min(9 * 60, operatingSpan)
  const middleStart = open + Math.floor((operatingSpan - shiftSpan) / 2)
  const ids: WorkShiftTemplateId[] = ['opening', 'middle', 'closing']
  const defaultNames: Record<WorkShiftTemplateId, string> = {
    opening: 'Abertura',
    middle: 'Intermediário',
    closing: 'Fechamento',
  }

  return ids.map((id) => {
    const existing = existingTemplates.find((template) => template.id === id)
    const startsAt = id === 'opening' ? open : id === 'middle' ? middleStart : close - shiftSpan
    const endsAt =
      id === 'opening' ? open + shiftSpan : id === 'middle' ? middleStart + shiftSpan : close
    return {
      id,
      name: existing?.name || defaultNames[id],
      startsAt: minutesToTime(startsAt),
      endsAt: minutesToTime(endsAt),
    }
  })
}

export function shiftTemplatesForStore(store: PlanningStore): PlanningShiftTemplate[] {
  return store.shiftTemplatesByLocationType[store.locationType]
}

export function shiftHours(shift: PlanningShift): number {
  const grossMinutes = timeToMinutes(shift.endsAt) - timeToMinutes(shift.startsAt)
  return Math.max(0, (grossMinutes - shift.breakMinutes) / 60)
}

function shiftSpan(
  store: PlanningStore,
  weekday: WeekdayId,
  template: PlanningShiftTemplate,
): { startsAt: string; endsAt: string } | null {
  const day = operatingDay(store, weekday)
  if (!day?.isOpen || !isShiftTemplateValid(template)) {
    return null
  }

  const open = timeToMinutes(day.opensAt)
  const close = timeToMinutes(day.closesAt)
  const start = Math.max(open, timeToMinutes(template.startsAt))
  const end = Math.min(close, timeToMinutes(template.endsAt))
  if (end <= start) {
    return null
  }

  return { startsAt: minutesToTime(start), endsAt: minutesToTime(end) }
}

function fitShiftToDailyLimit(
  span: { startsAt: string; endsAt: string },
  templateId: ShiftTemplateId,
  staff: PlanningStaffMember,
  policy: PlanningLaborPolicy,
): { startsAt: string; endsAt: string; breakMinutes: number } | null {
  let start = timeToMinutes(span.startsAt)
  let end = timeToMinutes(span.endsAt)
  const dailyLimitMinutes = Math.max(0, Math.min(staff.maxDailyHours, policy.maxDailyHours) * 60)
  if (dailyLimitMinutes <= 0 || end <= start) return null

  const initialGrossMinutes = end - start
  const initialBreakMinutes =
    initialGrossMinutes >= policy.breakAfterHours * 60 ? policy.minBreakMinutes : 0
  const maximumSpanMinutes = dailyLimitMinutes + initialBreakMinutes

  if (initialGrossMinutes > maximumSpanMinutes) {
    if (templateId === 'closing') start = end - maximumSpanMinutes
    else end = start + maximumSpanMinutes
  }

  const grossMinutes = end - start
  const breakMinutes = grossMinutes >= policy.breakAfterHours * 60 ? policy.minBreakMinutes : 0
  if (grossMinutes - breakMinutes > dailyLimitMinutes + 0.01) return null

  return {
    startsAt: minutesToTime(start),
    endsAt: minutesToTime(end),
    breakMinutes,
  }
}

export function buildShiftFromTemplate(
  store: PlanningStore,
  staff: PlanningStaffMember,
  policy: PlanningLaborPolicy,
  date: PlanningWeekDate,
  templateId: ShiftTemplateId,
): PlanningShift | null {
  if (templateId === 'off') {
    return null
  }

  const template = shiftTemplatesForStore(store).find((item) => item.id === templateId)
  if (!template) {
    return null
  }
  const span = shiftSpan(store, date.weekday, template)
  if (!span) {
    return null
  }
  const fitted = fitShiftToDailyLimit(span, templateId, staff, policy)
  if (!fitted) return null

  return {
    staffId: staff.id,
    isoDate: date.isoDate,
    templateId,
    startsAt: fitted.startsAt,
    endsAt: fitted.endsAt,
    breakMinutes: fitted.breakMinutes,
  }
}

export function weekdayForISODate(isoDate: string): WeekdayId {
  return weekdayByJSIndex[parseISODate(isoDate).getUTCDay()] || 'mon'
}
