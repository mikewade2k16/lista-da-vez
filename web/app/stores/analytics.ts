import { computed, ref, watch } from 'vue'
import { defineStore } from 'pinia'

import { useAuthStore } from '~/stores/auth'
import { createApiRequest, getApiErrorMessage } from '~/utils/api-client'

function normalizeText(value) {
  return String(value || '').trim()
}

function formatDateInput(date: Date) {
  return [
    date.getUTCFullYear(),
    String(date.getUTCMonth() + 1).padStart(2, '0'),
    String(date.getUTCDate()).padStart(2, '0'),
  ].join('-')
}

function buildCurrentMonthRange() {
  const now = new Date()
  return {
    dateFrom: formatDateInput(new Date(Date.UTC(now.getUTCFullYear(), now.getUTCMonth(), 1))),
    dateTo: formatDateInput(new Date(Date.UTC(now.getUTCFullYear(), now.getUTCMonth() + 1, 0))),
  }
}

export const useAnalyticsStore = defineStore('analytics', () => {
  const runtimeConfig = useRuntimeConfig()
  const auth = useAuthStore()
  const apiRequest = createApiRequest(runtimeConfig, () => auth.accessToken)
  const defaultRange = buildCurrentMonthRange()

  const ranking = ref(null)
  const data = ref(null)
  const intelligence = ref(null)
  const pending = ref(false)
  const ready = ref(false)
  const errorMessage = ref('')
  const integratedScope = ref(false)
  const rankingScopeKey = ref('')
  const dataScopeKey = ref('')
  const intelligenceScopeKey = ref('')
  const dateFrom = ref(defaultRange.dateFrom)
  const dateTo = ref(defaultRange.dateTo)

  const currentScopeKey = computed(() => {
    const dateScope = `:${normalizeText(dateFrom.value)}:${normalizeText(dateTo.value)}`
    if (integratedScope.value) {
      return `tenant:${normalizeText(auth.activeTenantId)}${dateScope}`
    }

    return `store:${normalizeText(auth.activeStoreId)}${dateScope}`
  })

  function clearState() {
    ranking.value = null
    data.value = null
    intelligence.value = null
    ready.value = false
    errorMessage.value = ''
    rankingScopeKey.value = ''
    dataScopeKey.value = ''
    intelligenceScopeKey.value = ''
  }

  function buildScopeQuery() {
    const params = new URLSearchParams()
    if (integratedScope.value) {
      const tenantId = normalizeText(auth.activeTenantId)
      if (tenantId) params.set('tenantId', tenantId)
    } else {
      const storeId = normalizeText(auth.activeStoreId)
      if (storeId) params.set('storeId', storeId)
    }

    if (dateFrom.value) params.set('dateFrom', dateFrom.value)
    if (dateTo.value) params.set('dateTo', dateTo.value)

    const query = params.toString()
    return query ? `?${query}` : ''
  }

  function resetCurrentMonth() {
    const nextRange = buildCurrentMonthRange()
    dateFrom.value = nextRange.dateFrom
    dateTo.value = nextRange.dateTo
  }

  async function ensureBase() {
    await auth.ensureSession()

    const hasScope = integratedScope.value
      ? normalizeText(auth.activeTenantId)
      : normalizeText(auth.activeStoreId)

    if (!auth.isAuthenticated || !hasScope) {
      clearState()
      return false
    }

    return true
  }

  async function fetchRanking() {
    if (!(await ensureBase())) {
      return null
    }

    pending.value = true
    errorMessage.value = ''

    try {
      ranking.value = await apiRequest(`/v1/analytics/ranking${buildScopeQuery()}`)
      rankingScopeKey.value = currentScopeKey.value
      ready.value = true
      return ranking.value
    } catch (error) {
      errorMessage.value = getApiErrorMessage(error, 'Nao foi possivel carregar o ranking.')
      throw error
    } finally {
      pending.value = false
    }
  }

  async function applyRankingFilters() {
    return fetchRanking()
  }

  async function fetchData() {
    if (!(await ensureBase())) {
      return null
    }

    pending.value = true
    errorMessage.value = ''

    try {
      data.value = await apiRequest(`/v1/analytics/data${buildScopeQuery()}`)
      dataScopeKey.value = currentScopeKey.value
      ready.value = true
      return data.value
    } catch (error) {
      errorMessage.value = getApiErrorMessage(
        error,
        'Nao foi possivel carregar os dados operacionais.',
      )
      throw error
    } finally {
      pending.value = false
    }
  }

  async function fetchIntelligence() {
    if (!(await ensureBase())) {
      return null
    }

    pending.value = true
    errorMessage.value = ''

    try {
      intelligence.value = await apiRequest(`/v1/analytics/intelligence${buildScopeQuery()}`)
      intelligenceScopeKey.value = currentScopeKey.value
      ready.value = true
      return intelligence.value
    } catch (error) {
      errorMessage.value = getApiErrorMessage(
        error,
        'Nao foi possivel carregar a inteligencia operacional.',
      )
      throw error
    } finally {
      pending.value = false
    }
  }

  async function ensureRanking() {
    if (ranking.value && rankingScopeKey.value === currentScopeKey.value) {
      return ranking.value
    }

    return fetchRanking()
  }

  async function ensureData() {
    if (data.value && dataScopeKey.value === currentScopeKey.value) {
      return data.value
    }

    return fetchData()
  }

  async function ensureIntelligence() {
    if (intelligence.value && intelligenceScopeKey.value === currentScopeKey.value) {
      return intelligence.value
    }

    return fetchIntelligence()
  }

  function setIntegratedScope(value) {
    const nextValue = Boolean(value)

    if (integratedScope.value === nextValue) {
      return
    }

    integratedScope.value = nextValue
    clearState()
  }

  if (import.meta.client) {
    watch(
      () => [auth.isAuthenticated, auth.activeStoreId, auth.activeTenantId, integratedScope.value],
      (
        [isAuthenticated, activeStoreId, activeTenantId, isIntegrated],
        [previousAuthenticated, previousStoreId, previousTenantId, previousIntegrated],
      ) => {
        const normalizedStoreId = normalizeText(activeStoreId)
        const normalizedTenantId = normalizeText(activeTenantId)

        if (!isAuthenticated || (isIntegrated ? !normalizedTenantId : !normalizedStoreId)) {
          clearState()
          return
        }

        if (
          !previousAuthenticated ||
          previousStoreId !== activeStoreId ||
          previousTenantId !== activeTenantId ||
          previousIntegrated !== isIntegrated
        ) {
          clearState()
        }
      },
    )
  }

  return {
    ranking,
    data,
    intelligence,
    pending,
    ready,
    errorMessage,
    integratedScope,
    dateFrom,
    dateTo,
    clearState,
    fetchRanking,
    applyRankingFilters,
    fetchData,
    fetchIntelligence,
    ensureRanking,
    ensureData,
    ensureIntelligence,
    setIntegratedScope,
    resetCurrentMonth,
  }
})
