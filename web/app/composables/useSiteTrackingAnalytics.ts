import type { SiteTrackingAnalytics } from '~/types/tracking'
import { createApiRequest, getApiErrorMessage } from '~/utils/api-client'

function emptyAnalytics(): SiteTrackingAnalytics {
  return {
    rangeDays: 14,
    totals: {
      totalEvents: 0,
      totalSessions: 0,
      totalVisitors: 0,
      pageViews: 0,
      today: 0,
      last7Days: 0,
    },
    devices: [],
    eventsByType: [],
    conversions: [],
    accessByDay: [],
    topReferrers: [],
    recentVisits: [],
  }
}

function toNumber(value: unknown): number {
  const parsed = Number(value)
  return Number.isFinite(parsed) ? parsed : 0
}

function toText(value: unknown): string {
  return String(value ?? '')
}

function normalizeCountItems(value: unknown): SiteTrackingAnalytics['devices'] {
  if (!Array.isArray(value)) return []
  return value.map((raw) => {
    const item = raw as Record<string, unknown>
    return { label: toText(item.label), count: toNumber(item.count) }
  })
}

function normalizeAnalytics(raw: Record<string, unknown>): SiteTrackingAnalytics {
  const totals = (raw.totals ?? {}) as Record<string, unknown>
  return {
    rangeDays: toNumber(raw.rangeDays) || 14,
    totals: {
      totalEvents: toNumber(totals.totalEvents),
      totalSessions: toNumber(totals.totalSessions),
      totalVisitors: toNumber(totals.totalVisitors),
      pageViews: toNumber(totals.pageViews),
      today: toNumber(totals.today),
      last7Days: toNumber(totals.last7Days),
    },
    devices: normalizeCountItems(raw.devices),
    eventsByType: normalizeCountItems(raw.eventsByType),
    conversions: Array.isArray(raw.conversions)
      ? raw.conversions.map((entry) => {
          const item = entry as Record<string, unknown>
          return {
            key: toText(item.key),
            label: toText(item.label),
            count: toNumber(item.count),
            percentOfVisitors: toNumber(item.percentOfVisitors),
          }
        })
      : [],
    accessByDay: Array.isArray(raw.accessByDay)
      ? raw.accessByDay.map((entry) => {
          const item = entry as Record<string, unknown>
          return { date: toText(item.date), count: toNumber(item.count) }
        })
      : [],
    topReferrers: normalizeCountItems(raw.topReferrers),
    recentVisits: Array.isArray(raw.recentVisits)
      ? raw.recentVisits.map((entry) => {
          const item = entry as Record<string, unknown>
          return {
            receivedAt: toText(item.receivedAt),
            deviceType: toText(item.deviceType),
            ip: toText(item.ip),
            referrer: toText(item.referrer),
            pagePath: toText(item.pagePath),
          }
        })
      : [],
  }
}

export function useSiteTrackingAnalytics() {
  const runtimeConfig = useRuntimeConfig()
  const auth = useAuthStore()
  const apiRequest = createApiRequest(runtimeConfig, () => auth.accessToken)

  function accountHeaders() {
    return auth.activeTenantId ? { 'X-Account-Id': auth.activeTenantId } : {}
  }

  const analytics = ref<SiteTrackingAnalytics>(emptyAnalytics())
  const days = ref(14)
  const source = ref('')
  const loading = ref(false)
  const errorMessage = ref('')

  async function fetchAnalytics() {
    loading.value = true
    errorMessage.value = ''
    try {
      const query = new URLSearchParams()
      query.set('days', String(days.value))
      if (source.value) query.set('source', source.value)

      const response = await apiRequest(`/v1/admin/tracking-analytics?${query.toString()}`, {
        headers: accountHeaders(),
      })
      analytics.value = normalizeAnalytics((response ?? {}) as Record<string, unknown>)
    } catch (error) {
      errorMessage.value = getApiErrorMessage(error, 'Falha ao carregar analytics do tracking.')
      analytics.value = emptyAnalytics()
    } finally {
      loading.value = false
    }
  }

  function setDays(value: number) {
    days.value = Number.isFinite(value) && value > 0 ? Math.floor(value) : 14
  }

  function setSource(value: string) {
    source.value = String(value ?? '').trim()
  }

  return {
    analytics,
    days,
    source,
    loading,
    errorMessage,
    fetchAnalytics,
    setDays,
    setSource,
  }
}
