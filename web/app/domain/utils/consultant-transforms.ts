/**
 * Funções puras de transformação e normalização para consultores.
 * Sem efeitos colaterais; sem dependência de store ou composable.
 */

export interface NormalizedConsultant {
  id: string
  storeId: string
  storeName: string
  storeCode: string
  storeCity: string
  name: string
  role: string
  initials: string
  color: string
  monthlyGoal: number
  commissionRate: number
  conversionGoal: number
  avgTicketGoal: number
  paGoal: number
  active: boolean
  access: {
    userId: string
    email: string
    active: boolean
  } | null
}

export interface StoreContext {
  id?: string
  name?: string
  code?: string
  city?: string
}

export interface DateRange {
  dateFrom: string
  dateTo: string
}

export function normalizeText(value: unknown): string {
  return String(value || '').trim()
}

export function formatDateInput(date: Date): string {
  return [
    date.getUTCFullYear(),
    String(date.getUTCMonth() + 1).padStart(2, '0'),
    String(date.getUTCDate()).padStart(2, '0'),
  ].join('-')
}

export function buildCurrentMonthRange(): DateRange {
  const now = new Date()
  return {
    dateFrom: formatDateInput(new Date(Date.UTC(now.getUTCFullYear(), now.getUTCMonth(), 1))),
    dateTo: formatDateInput(new Date(Date.UTC(now.getUTCFullYear(), now.getUTCMonth() + 1, 0))),
  }
}

export function parseDateStartMs(value: unknown): number | null {
  const normalized = normalizeText(value)
  if (!normalized) return null
  const parsed = Date.parse(`${normalized}T00:00:00.000Z`)
  return Number.isFinite(parsed) ? parsed : null
}

export function parseDateEndExclusiveMs(value: unknown): number | null {
  const normalized = normalizeText(value)
  if (!normalized) return null
  const parsed = Date.parse(`${normalized}T00:00:00.000Z`)
  if (!Number.isFinite(parsed)) return null
  return parsed + 24 * 60 * 60 * 1000
}

export function resolveHistoryFinishedAt(
  entry: Record<string, unknown> | null | undefined,
): number {
  return Math.max(
    0,
    Number(
      entry?.finishedAt || entry?.effectiveFinishedAt || entry?.stoppedAt || entry?.startedAt || 0,
    ) || 0,
  )
}

export function filterHistoryByDateRange(
  history: Record<string, unknown>[] = [],
  dateFrom = '',
  dateTo = '',
): Record<string, unknown>[] {
  const rangeStart = parseDateStartMs(dateFrom)
  const rangeEndExclusive = parseDateEndExclusiveMs(dateTo)
  const hasExplicitRange = rangeStart !== null || rangeEndExclusive !== null

  if (!hasExplicitRange) {
    return Array.isArray(history) ? history : []
  }

  return (Array.isArray(history) ? history : []).filter((entry) => {
    const finishedAt = resolveHistoryFinishedAt(entry)
    if (rangeStart !== null && finishedAt < rangeStart) {
      return false
    }
    if (rangeEndExclusive !== null && finishedAt >= rangeEndExclusive) {
      return false
    }
    return true
  })
}

export function buildRowKey(storeId: unknown, consultantId: unknown): string {
  return `${normalizeText(storeId)}:${normalizeText(consultantId)}`
}

export function normalizeConsultantList(
  consultants: Record<string, unknown>[] = [],
  fallbackStore: StoreContext = {},
): NormalizedConsultant[] {
  return (Array.isArray(consultants) ? consultants : [])
    .map((consultant) => ({
      id: normalizeText(consultant?.id),
      storeId: normalizeText(consultant?.storeId) || normalizeText(fallbackStore?.id),
      storeName: normalizeText(fallbackStore?.name),
      storeCode: normalizeText(fallbackStore?.code),
      storeCity: normalizeText(fallbackStore?.city),
      name: normalizeText(consultant?.name),
      role: normalizeText(consultant?.role) || 'Atendimento',
      initials: normalizeText(consultant?.initials),
      color: normalizeText(consultant?.color) || '#168aad',
      monthlyGoal: Math.max(0, Number(consultant?.monthlyGoal || 0) || 0),
      commissionRate: Math.max(0, Number(consultant?.commissionRate || 0) || 0),
      conversionGoal: Math.max(0, Number(consultant?.conversionGoal || 0) || 0),
      avgTicketGoal: Math.max(0, Number(consultant?.avgTicketGoal || 0) || 0),
      paGoal: Math.max(0, Number(consultant?.paGoal || 0) || 0),
      active: Boolean(consultant?.active ?? true),
      access:
        consultant?.access && typeof consultant.access === 'object'
          ? {
              userId: normalizeText((consultant.access as Record<string, unknown>)?.userId),
              email: normalizeText(
                (consultant.access as Record<string, unknown>)?.email,
              ).toLowerCase(),
              active: Boolean((consultant.access as Record<string, unknown>)?.active ?? false),
            }
          : null,
    }))
    .filter((consultant) => consultant.id && consultant.name)
}
