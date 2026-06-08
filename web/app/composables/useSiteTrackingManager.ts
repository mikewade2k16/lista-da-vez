import type { SiteTrackingEventItem, SiteTrackingEventsListResponse } from '~/types/tracking'
import { createApiRequest, getApiErrorMessage } from '~/utils/api-client'

function normalizeText(value: unknown) {
  return String(value ?? '')
}

function normalizeNullableNumber(value: unknown): number | null {
  if (value === null || value === undefined || value === '') return null
  const parsed = Number(value)
  return Number.isFinite(parsed) ? parsed : null
}

function normalizeTrackingEvent(raw: Record<string, unknown>): SiteTrackingEventItem {
  return {
    id: normalizeText(raw.id),
    accountId: normalizeText(raw.accountId),
    sourceId: normalizeText(raw.sourceId),
    sourceLabel: normalizeText(raw.sourceLabel),
    source: normalizeText(raw.source),
    batchId: normalizeText(raw.batchId),
    sourceEventId: normalizeText(raw.sourceEventId),
    visitorId: normalizeText(raw.visitorId),
    sessionId: normalizeText(raw.sessionId),
    eventType: normalizeText(raw.eventType),
    eventName: normalizeText(raw.eventName),
    pageUrl: normalizeText(raw.pageUrl),
    pagePath: normalizeText(raw.pagePath),
    pageTitle: normalizeText(raw.pageTitle),
    pageGroup: normalizeText(raw.pageGroup),
    pageName: normalizeText(raw.pageName),
    referrer: normalizeText(raw.referrer),
    elementTag: normalizeText(raw.elementTag),
    elementText: normalizeText(raw.elementText),
    elementHref: normalizeText(raw.elementHref),
    elementId: normalizeText(raw.elementId),
    elementClasses: normalizeText(raw.elementClasses),
    elementRole: normalizeText(raw.elementRole),
    productCode: normalizeText(raw.productCode),
    activeSeconds: normalizeNullableNumber(raw.activeSeconds),
    scrollDepth: normalizeNullableNumber(raw.scrollDepth),
    screenWidth: normalizeNullableNumber(raw.screenWidth),
    screenHeight: normalizeNullableNumber(raw.screenHeight),
    viewportWidth: normalizeNullableNumber(raw.viewportWidth),
    viewportHeight: normalizeNullableNumber(raw.viewportHeight),
    deviceType: normalizeText(raw.deviceType),
    browserLang: normalizeText(raw.browserLang),
    timezone: normalizeText(raw.timezone),
    utmSource: normalizeText(raw.utmSource),
    utmMedium: normalizeText(raw.utmMedium),
    utmCampaign: normalizeText(raw.utmCampaign),
    utmTerm: normalizeText(raw.utmTerm),
    utmContent: normalizeText(raw.utmContent),
    eventData: normalizeText(raw.eventData),
    rawPayload: normalizeText(raw.rawPayload),
    ip: normalizeText(raw.ip),
    userAgent: normalizeText(raw.userAgent),
    sentAt: normalizeText(raw.sentAt),
    receivedAt: normalizeText(raw.receivedAt),
  }
}

export function useSiteTrackingManager() {
  const runtimeConfig = useRuntimeConfig()
  const auth = useAuthStore()
  const apiRequest = createApiRequest(runtimeConfig, () => auth.accessToken)

  function accountHeaders() {
    return auth.activeTenantId ? { 'X-Account-Id': auth.activeTenantId } : {}
  }

  const events = ref<SiteTrackingEventItem[]>([])
  const filters = reactive({
    q: '',
    source: '',
    eventType: '',
    pagePath: '',
  })
  const page = ref(1)
  const perPage = ref(50)
  const total = ref(0)
  const loading = ref(false)
  const errorMessage = ref('')
  const canResetFilters = computed(() =>
    Boolean(filters.q || filters.source || filters.eventType || filters.pagePath),
  )

  async function fetchEvents() {
    loading.value = true
    errorMessage.value = ''
    try {
      const query = new URLSearchParams()
      if (filters.q) query.set('q', filters.q)
      if (filters.source) query.set('source', filters.source)
      if (filters.eventType) query.set('eventType', filters.eventType)
      if (filters.pagePath) query.set('pagePath', filters.pagePath)
      query.set('page', String(page.value))
      query.set('perPage', String(perPage.value))

      const response = await apiRequest(`/v1/admin/tracking-events?${query.toString()}`, {
        headers: accountHeaders(),
      })
      const payload = response as Partial<SiteTrackingEventsListResponse>
      events.value = Array.isArray(payload.events)
        ? payload.events.map((item) => normalizeTrackingEvent(item as Record<string, unknown>))
        : []
      total.value = Number(payload.total ?? events.value.length) || 0
      page.value = Number(payload.page ?? page.value) || 1
      perPage.value = Number(payload.perPage ?? perPage.value) || 50
    } catch (error) {
      errorMessage.value = getApiErrorMessage(error, 'Falha ao carregar tracking do site.')
    } finally {
      loading.value = false
    }
  }

  function setPage(value: number) {
    page.value = Number.isFinite(value) && value > 0 ? Math.floor(value) : 1
  }

  function setPerPage(value: number) {
    const normalized = Number.isFinite(value) && value > 0 ? Math.floor(value) : 50
    perPage.value = Math.max(1, Math.min(200, normalized))
  }

  function resetFilters() {
    filters.q = ''
    filters.source = ''
    filters.eventType = ''
    filters.pagePath = ''
  }

  return {
    events,
    filters,
    page,
    perPage,
    total,
    loading,
    errorMessage,
    canResetFilters,
    fetchEvents,
    setPage,
    setPerPage,
    resetFilters,
  }
}
