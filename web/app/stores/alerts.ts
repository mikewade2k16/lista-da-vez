import { computed, ref, watch } from "vue"
import { defineStore } from "pinia"

import { useAuthStore } from "~/stores/auth"
import { createApiRequest, getApiErrorMessage } from "~/utils/api-client"
import { normalizeAlertHexColor } from "~/utils/alert-colors"

function normalizeText(value: unknown) {
  return String(value || "").trim()
}

function normalizeBoolean(value: unknown, fallback = false) {
  if (typeof value === "boolean") {
    return value
  }

  if (typeof value === "string") {
    const normalized = value.trim().toLowerCase()
    if (["true", "1", "yes"].includes(normalized)) {
      return true
    }
    if (["false", "0", "no"].includes(normalized)) {
      return false
    }
  }

  return fallback
}

function normalizeDate(value: unknown) {
  const normalized = normalizeText(value)
  return normalized || ""
}

function normalizeMetadata(value: unknown) {
  return value && typeof value === "object" && !Array.isArray(value) ? { ...value as Record<string, unknown> } : {}
}

function normalizeAlert(alert: Record<string, unknown> = {}) {
  return {
    id: normalizeText(alert.id),
    tenantId: normalizeText(alert.tenantId),
    storeId: normalizeText(alert.storeId),
    serviceId: normalizeText(alert.serviceId),
    consultantId: normalizeText(alert.consultantId),
    type: normalizeText(alert.type),
    category: normalizeText(alert.category),
    severity: normalizeText(alert.severity),
    status: normalizeText(alert.status),
    sourceModule: normalizeText(alert.sourceModule),
    headline: normalizeText(alert.headline),
    body: normalizeText(alert.body),
    openedAt: normalizeDate(alert.openedAt),
    lastTriggeredAt: normalizeDate(alert.lastTriggeredAt),
    acknowledgedAt: normalizeDate(alert.acknowledgedAt),
    resolvedAt: normalizeDate(alert.resolvedAt),
    interactionKind: normalizeText(alert.interactionKind) || "none",
    interactionResponse: normalizeText(alert.interactionResponse),
    respondedAt: normalizeDate(alert.respondedAt),
    externalNotifiedAt: normalizeDate(alert.externalNotifiedAt),
    createdAt: normalizeDate(alert.createdAt),
    updatedAt: normalizeDate(alert.updatedAt),
    metadata: normalizeMetadata(alert.metadata),
    ruleDefinitionId: normalizeText(alert.ruleDefinitionId),
    displayKind: normalizeText(alert.displayKind) || "banner",
    colorTheme: normalizeAlertHexColor(alert.colorTheme || "amber"),
    titleTemplate: normalizeText(alert.titleTemplate) || normalizeText(alert.headline),
    bodyTemplate: normalizeText(alert.bodyTemplate) || normalizeText(alert.body),
    responseOptions: Array.isArray(alert.responseOptions)
      ? (alert.responseOptions as Array<{ value: string; label: string }>)
      : [],
    isMandatory: normalizeBoolean(alert.isMandatory, false),
    consultantName: normalizeText(alert.consultantName)
  }
}

function normalizeOverview(overview: Record<string, unknown> = {}) {
  return {
    tenantId: normalizeText(overview.tenantId),
    storeId: normalizeText(overview.storeId),
    totalActive: Math.max(0, Number(overview.totalActive || 0) || 0),
    criticalActive: Math.max(0, Number(overview.criticalActive || 0) || 0),
    acknowledged: Math.max(0, Number(overview.acknowledged || 0) || 0),
    resolvedToday: Math.max(0, Number(overview.resolvedToday || 0) || 0)
  }
}

function normalizeRules(rules: Record<string, unknown> = {}) {
  return {
    tenantId: normalizeText(rules.tenantId),
    longOpenServiceMinutes: Math.max(1, Number(rules.longOpenServiceMinutes || 25) || 25),
    idleStoreMinutes: Math.max(1, Number(rules.idleStoreMinutes || 20) || 20),
    afterClosingGraceMinutes: Math.max(0, Number(rules.afterClosingGraceMinutes || 15) || 15),
    notifyDashboard: normalizeBoolean(rules.notifyDashboard, true),
    notifyOperationContext: normalizeBoolean(rules.notifyOperationContext, true),
    notifyExternal: normalizeBoolean(rules.notifyExternal, false),
    source: normalizeText(rules.source) || "module-defaults",
    updatedAt: normalizeDate(rules.updatedAt)
  }
}

const rulePayloadKeys = [
  "name",
  "description",
  "isActive",
  "triggerType",
  "thresholdMinutes",
  "severity",
  "displayKind",
  "colorTheme",
  "titleTemplate",
  "bodyTemplate",
  "interactionKind",
  "responseOptions",
  "isMandatory",
  "notifyDashboard",
  "notifyOperationContext",
  "notifyExternal",
  "externalChannel"
]

function createScopeKey(scopeType: string, scopeId: string, filters: { status: string; type: string }) {
  return JSON.stringify({
    scopeType,
    scopeId,
    filters
  })
}

export const useAlertsStore = defineStore("alerts", () => {
  const runtimeConfig = useRuntimeConfig()
  const auth = useAuthStore()
  const apiRequest = createApiRequest(runtimeConfig, () => auth.accessToken)

  const items = ref<Array<ReturnType<typeof normalizeAlert>>>([])
  const overview = ref<ReturnType<typeof normalizeOverview> | null>(null)
  const rules = ref<ReturnType<typeof normalizeRules> | null>(null)
  const ruleDefinitions = ref<Array<Record<string, any>>>([])
  const filters = ref({
    status: "active",
    type: ""
  })
  const pending = ref(false)
  const rulesPending = ref(false)
  const ready = ref(false)
  const rulesLoaded = ref(false)
  const errorMessage = ref("")
  const lastLoadedKey = ref("")
  const pendingFinishForServiceId = ref<string | null>(null)

  const integratedScope = computed(() => Boolean(auth.isAllStoresScope))
  const activeStoreId = computed(() => normalizeText(auth.activeStoreId))
  const activeTenantId = computed(() => normalizeText(auth.activeTenantId || auth.tenantContext?.[0]?.id))

  function hasValidScope() {
    if (!auth.isAuthenticated) {
      return false
    }

    if (integratedScope.value) {
      return Boolean(activeTenantId.value)
    }

    return Boolean(activeStoreId.value || activeTenantId.value)
  }

  function buildScopeParams({ includeFilters = true } = {}) {
    const params = new URLSearchParams()

    if (activeTenantId.value) {
      params.set("tenantId", activeTenantId.value)
    }

    if (!integratedScope.value && activeStoreId.value) {
      params.set("storeId", activeStoreId.value)
    }

    if (includeFilters) {
      params.set("category", "operational")

      if (filters.value.status) {
        params.set("status", filters.value.status)
      }
      if (filters.value.type) {
        params.set("type", filters.value.type)
      }
    }

    return params
  }

  function sanitizeRulePayload(input: Record<string, unknown> = {}, { includeTenant = false } = {}) {
    const payload: Record<string, unknown> = {}

    if (includeTenant && activeTenantId.value) {
      payload.tenantId = activeTenantId.value
    }

    for (const key of rulePayloadKeys) {
      if (!Object.prototype.hasOwnProperty.call(input, key)) {
        continue
      }

      if (key === "responseOptions") {
        payload.responseOptions = Array.isArray(input.responseOptions)
          ? input.responseOptions
              .map((option: any) => ({
                value: normalizeText(option?.value),
                label: normalizeText(option?.label)
              }))
              .filter((option) => option.value || option.label)
          : []
        continue
      }

      payload[key] = key === "colorTheme"
        ? normalizeAlertHexColor(input[key]).toUpperCase()
        : input[key]
    }

    return payload
  }

  function currentScopeKey() {
    if (integratedScope.value) {
      return createScopeKey("tenant", activeTenantId.value, filters.value)
    }

    return createScopeKey("store", activeStoreId.value, filters.value)
  }

  function clearState() {
    items.value = []
    overview.value = null
    rules.value = null
    ready.value = false
    rulesLoaded.value = false
    errorMessage.value = ""
    lastLoadedKey.value = ""
  }

  function updateLocalAlert(nextAlert: Record<string, unknown>) {
    const normalized = normalizeAlert(nextAlert)
    const index = items.value.findIndex((item) => item.id === normalized.id)
    if (index === -1) {
      items.value = [normalized, ...items.value]
      return normalized
    }

    items.value[index] = normalized
    return normalized
  }

  async function refreshOverview() {
    if (!hasValidScope()) {
      overview.value = null
      return null
    }

    const response = await apiRequest(`/v1/alerts/overview?${buildScopeParams({ includeFilters: false }).toString()}`)
    overview.value = normalizeOverview(response?.overview || {})
    return overview.value
  }

  async function refreshAlerts() {
    if (!hasValidScope()) {
      clearState()
      return []
    }

    pending.value = true
    errorMessage.value = ""

    try {
      const [alertsResponse, overviewResponse] = await Promise.all([
        apiRequest(`/v1/alerts?${buildScopeParams().toString()}`),
        apiRequest(`/v1/alerts/overview?${buildScopeParams({ includeFilters: false }).toString()}`)
      ])

      items.value = Array.isArray(alertsResponse?.alerts)
        ? alertsResponse.alerts.map((alert: Record<string, unknown>) => normalizeAlert(alert))
        : []
      overview.value = normalizeOverview(overviewResponse?.overview || {})
      ready.value = true
      lastLoadedKey.value = currentScopeKey()

      return items.value
    } catch (error) {
      errorMessage.value = getApiErrorMessage(error, "Nao foi possivel carregar os alertas.")
      throw error
    } finally {
      pending.value = false
    }
  }

  async function fetchRules() {
    if (!activeTenantId.value || !auth.isAuthenticated) {
      rules.value = null
      rulesLoaded.value = false
      return null
    }

    rulesPending.value = true

    try {
      const params = new URLSearchParams()
      params.set("tenantId", activeTenantId.value)
      const response = await apiRequest(`/v1/alerts/rules?${params.toString()}`)
      rules.value = normalizeRules(response?.rules || {})
      rulesLoaded.value = true
      return rules.value
    } catch (error) {
      errorMessage.value = getApiErrorMessage(error, "Nao foi possivel carregar as regras de alertas.")
      throw error
    } finally {
      rulesPending.value = false
    }
  }

  async function ensureLoaded() {
    if (!hasValidScope()) {
      clearState()
      return false
    }

    if (ready.value && lastLoadedKey.value === currentScopeKey()) {
      if (!rulesLoaded.value && activeTenantId.value) {
        try {
          await fetchRules()
          await fetchRuleDefinitions()
        } catch {
          return true
        }
      }

      return true
    }

    try {
      await refreshAlerts()
      await fetchRules()
      await fetchRuleDefinitions()
      return true
    } catch {
      return false
    }
  }

  async function applyFilters(nextFilters: Partial<typeof filters.value> = {}) {
    filters.value = {
      ...filters.value,
      ...nextFilters,
      status: normalizeText(nextFilters.status ?? filters.value.status),
      type: normalizeText(nextFilters.type ?? filters.value.type)
    }

    return refreshAlerts()
  }

  async function acknowledgeAlert(alertId: string, note = "") {
    const response = await apiRequest(`/v1/alerts/${encodeURIComponent(normalizeText(alertId))}/acknowledge`, {
      method: "POST",
      body: {
        note: normalizeText(note)
      }
    })

    const updated = updateLocalAlert(response?.alert || {})
    await refreshOverview()
    return updated
  }

  async function resolveAlert(alertId: string, note = "") {
    const response = await apiRequest(`/v1/alerts/${encodeURIComponent(normalizeText(alertId))}/resolve`, {
      method: "POST",
      body: {
        note: normalizeText(note)
      }
    })

    const updated = updateLocalAlert(response?.alert || {})
    await refreshOverview()
    return updated
  }

  async function updateRules(payload: Record<string, unknown> = {}) {
    if (!activeTenantId.value) {
      throw new Error("Tenant invalido para regras de alertas.")
    }

    rulesPending.value = true

    try {
      const response = await apiRequest(`/v1/alerts/rules?tenantId=${encodeURIComponent(activeTenantId.value)}`, {
        method: "PATCH",
        body: {
          longOpenServiceMinutes: Math.max(1, Number(payload.longOpenServiceMinutes || 0) || defaultLongOpenServiceMinutes),
          idleStoreMinutes: Math.max(1, Number(payload.idleStoreMinutes || 0) || defaultIdleStoreMinutes),
          afterClosingGraceMinutes: Math.max(0, Number(payload.afterClosingGraceMinutes || 0) || defaultAfterClosingGraceMinutes),
          notifyDashboard: normalizeBoolean(payload.notifyDashboard, true),
          notifyOperationContext: normalizeBoolean(payload.notifyOperationContext, true),
          notifyExternal: normalizeBoolean(payload.notifyExternal, false)
        }
      })

      rules.value = normalizeRules(response?.rules || {})
      rulesLoaded.value = true
      return rules.value
    } finally {
      rulesPending.value = false
    }
  }

  async function respondToAlert(alertId: string, response: string) {
    const result = await apiRequest(`/v1/alerts/${encodeURIComponent(normalizeText(alertId))}/respond`, {
      method: "POST",
      body: { response }
    })

    updateLocalAlert(result?.alert || {})
    await refreshOverview()

    if (result?.openFinishModal && result?.serviceId) {
      pendingFinishForServiceId.value = normalizeText(result.serviceId)
    }

    return result
  }

  async function fetchRuleDefinitions(filters: { triggerType?: string; onlyActive?: boolean } = {}) {
    if (!activeTenantId.value) {
      ruleDefinitions.value = []
      return []
    }

    rulesPending.value = true

    try {
      const params = new URLSearchParams({
        tenantId: activeTenantId.value,
        format: "definitions"
      })
      if (filters.triggerType) {
        params.append("triggerType", normalizeText(filters.triggerType))
      }
      if (filters.onlyActive) {
        params.append("onlyActive", "true")
      }

      const response = await apiRequest(`/v1/alerts/rules?${params.toString()}`)
      const rules = response?.rules || []
      ruleDefinitions.value = rules
      return rules
    } catch (err) {
      errorMessage.value = getApiErrorMessage(err)
      return []
    } finally {
      rulesPending.value = false
    }
  }

  async function createRule(input: Record<string, unknown>) {
    if (!activeTenantId.value) {
      throw new Error("Tenant invalido para criar regra.")
    }

    rulesPending.value = true

    try {
      const response = await apiRequest("/v1/alerts/rules", {
        method: "POST",
        body: sanitizeRulePayload(input, { includeTenant: true })
      })
      const rule = response?.rule || null
      if (rule) {
        ruleDefinitions.value = [rule, ...ruleDefinitions.value]
      }
      return rule
    } catch (err) {
      errorMessage.value = getApiErrorMessage(err)
      throw err
    } finally {
      rulesPending.value = false
    }
  }

  async function updateRule(ruleId: string, input: Record<string, unknown>) {
    rulesPending.value = true

    try {
      const response = await apiRequest(`/v1/alerts/rules/${encodeURIComponent(normalizeText(ruleId))}`, {
        method: "PATCH",
        body: sanitizeRulePayload(input)
      })
      const rule = response?.rule || null
      if (rule) {
        const index = ruleDefinitions.value.findIndex((r) => r.id === rule.id)
        if (index !== -1) {
          ruleDefinitions.value[index] = rule
        }
      }
      return rule
    } catch (err) {
      errorMessage.value = getApiErrorMessage(err)
      throw err
    } finally {
      rulesPending.value = false
    }
  }

  async function deleteRule(ruleId: string) {
    rulesPending.value = true

    try {
      await apiRequest(`/v1/alerts/rules/${encodeURIComponent(normalizeText(ruleId))}`, {
        method: "DELETE"
      })
      ruleDefinitions.value = ruleDefinitions.value.filter((r) => r.id !== ruleId)
    } catch (err) {
      errorMessage.value = getApiErrorMessage(err)
      throw err
    } finally {
      rulesPending.value = false
    }
  }

  async function applyRuleNow(ruleId: string) {
    rulesPending.value = true

    try {
      const response = await apiRequest(`/v1/alerts/rules/${encodeURIComponent(normalizeText(ruleId))}/apply-now`, {
        method: "POST"
      })
      const appliedCount = Math.max(0, Number(response?.appliedCount || 0) || 0)
      await refreshAlerts()
      return { appliedCount }
    } catch (err) {
      errorMessage.value = getApiErrorMessage(err)
      throw err
    } finally {
      rulesPending.value = false
    }
  }

  function activeAlertsForStore(storeId: string) {
    const normalizedStoreId = normalizeText(storeId)
    return items.value.filter(
      (alert) => alert.status === "active" && alert.storeId === normalizedStoreId
    )
  }

  function alertForService(serviceId: string) {
    const normalizedServiceId = normalizeText(serviceId)
    return items.value.find(
      (alert) => alert.serviceId === normalizedServiceId && alert.status === "active"
    )
  }

  async function refreshRealtimeState() {
    if (!ready.value) {
      return
    }

    await Promise.allSettled([
      refreshAlerts(),
      rulesLoaded.value ? fetchRules() : Promise.resolve(null),
      activeTenantId.value ? fetchRuleDefinitions() : Promise.resolve([])
    ])
  }

  if (import.meta.client) {
    watch(
      () => [auth.isAuthenticated, activeStoreId.value, activeTenantId.value, integratedScope.value],
      ([isAuthenticated, storeId, tenantId, isIntegrated], [previousAuthenticated, previousStoreId, previousTenantId, previousIntegrated]) => {
        if (!isAuthenticated || (isIntegrated ? !tenantId : !storeId && !tenantId)) {
          clearState()
          return
        }

        if (
          !previousAuthenticated ||
          previousStoreId !== storeId ||
          previousTenantId !== tenantId ||
          previousIntegrated !== isIntegrated
        ) {
          void ensureLoaded()
        }
      }
    )
  }

  return {
    items,
    overview,
    rules,
    ruleDefinitions,
    filters,
    pending,
    rulesPending,
    ready,
    errorMessage,
    integratedScope,
    activeStoreId,
    activeTenantId,
    pendingFinishForServiceId,
    ensureLoaded,
    refreshAlerts,
    refreshOverview,
    fetchRules,
    applyFilters,
    acknowledgeAlert,
    resolveAlert,
    respondToAlert,
    fetchRuleDefinitions,
    createRule,
    updateRule,
    deleteRule,
    applyRuleNow,
    activeAlertsForStore,
    alertForService,
    updateRules,
    refreshRealtimeState
  }
})

const defaultLongOpenServiceMinutes = 25
const defaultIdleStoreMinutes = 20
const defaultAfterClosingGraceMinutes = 15
