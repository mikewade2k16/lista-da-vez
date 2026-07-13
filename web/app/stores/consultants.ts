import { computed, ref, watch } from 'vue'
import { defineStore, storeToRefs } from 'pinia'

import { useAuthStore } from '~/stores/auth'
import { useAppRuntimeStore } from '~/stores/app-runtime'
import { normalizeServiceHistoryList } from '~/stores/dashboard/runtime/state'
import { createApiRequest, getApiErrorMessage } from '~/utils/api-client'
import { hydrateRuntimeStoreContext } from '~/utils/runtime-remote'
import {
  normalizeText,
  buildCurrentMonthRange,
  buildPreviousMonthRange,
  buildMonthWeekRange,
  filterHistoryByDateRange,
  normalizeConsultantList,
} from '~/domain/utils/consultant-transforms'
import { buildIntegratedRankingResponse } from '~/domain/utils/consultant-integrated-view'

export const useConsultantsStore = defineStore('consultants', () => {
  const runtimeConfig = useRuntimeConfig()
  const runtime = useAppRuntimeStore()
  const auth = useAuthStore()
  const { state } = storeToRefs(runtime)
  const apiRequest = createApiRequest(runtimeConfig, () => auth.accessToken)
  const integratedDefaultRange = buildCurrentMonthRange()
  const integratedRoster = ref([])
  const integratedRanking = ref(null)
  const integratedOverview = ref(null)
  const integratedHistory = ref([])
  const integratedStaff = ref<
    Array<{
      id: string
      name: string
      role: string
      roleLabel: string
      storeId: string
      storeName: string
    }>
  >([])
  const integratedPending = ref(false)
  const integratedReady = ref(false)
  const integratedError = ref('')
  const integratedScopeKey = ref('')
  const integratedDateFrom = ref(integratedDefaultRange.dateFrom)
  const integratedDateTo = ref(integratedDefaultRange.dateTo)

  const roster = computed(() => state.value.roster || [])
  const selectedConsultantId = computed(() => state.value.selectedConsultantId || null)
  const accessibleStores = computed(() => {
    const allowedStoreIds = new Set(
      Array.isArray(auth.accessibleStoreIds)
        ? auth.accessibleStoreIds.map((storeId) => normalizeText(storeId)).filter(Boolean)
        : [],
    )

    return (auth.storeContext || []).filter((store) => {
      const storeId = normalizeText(store?.id)
      return !allowedStoreIds.size || allowedStoreIds.has(storeId)
    })
  })
  const activeTenantId = computed(() =>
    normalizeText(auth.activeTenantId || auth.tenantContext?.[0]?.id),
  )

  async function resolveActiveStoreId() {
    await runtime.ensure()

    if (auth.isAuthenticated) {
      await auth.ensureSession()
    }

    return String(auth.activeStoreId || runtime.state.activeStoreId || '').trim()
  }

  function canArchiveConsultantLocally(consultantId: string) {
    const currentState = runtime.state
    const isInQueue = (currentState.waitingList || []).some((item) => item.id === consultantId)
    const isInService = (currentState.activeServices || []).some((item) => item.id === consultantId)
    const isPaused = (currentState.pausedEmployees || []).some(
      (item) => item.personId === consultantId,
    )

    if (isInQueue || isInService || isPaused) {
      return {
        ok: false,
        message: 'Retire o consultor de fila, atendimento ou pausa antes de arquivar.',
      }
    }

    return { ok: true }
  }

  function normalizeConsultantPayload(payload: Record<string, unknown> = {}) {
    return {
      name: String(payload?.name || '').trim(),
      role: String(payload?.role || '').trim(),
      color: String(payload?.color || '').trim(),
      monthlyGoal: Math.max(0, Number(payload?.monthlyGoal || 0) || 0),
      commissionRate: Math.max(0, Number(payload?.commissionRate || 0) || 0),
      conversionGoal: Math.max(0, Number(payload?.conversionGoal || 0) || 0),
      avgTicketGoal: Math.max(0, Number(payload?.avgTicketGoal || 0) || 0),
      paGoal: Math.max(0, Number(payload?.paGoal || 0) || 0),
    }
  }

  async function refreshActiveStore() {
    const storeId = await resolveActiveStoreId()

    if (!storeId || !auth.isAuthenticated) {
      return null
    }

    const runtimeContext = await hydrateRuntimeStoreContext(
      runtime,
      apiRequest,
      storeId,
      activeTenantId.value,
    )
    auth.applyRuntimeSettingsStatus(runtimeContext)
    return runtimeContext
  }

  function clearIntegratedView() {
    integratedRoster.value = []
    integratedRanking.value = null
    integratedOverview.value = null
    integratedHistory.value = []
    integratedStaff.value = []
    integratedPending.value = false
    integratedReady.value = false
    integratedError.value = ''
    integratedScopeKey.value = ''
  }

  function buildIntegratedScopeKey() {
    return JSON.stringify({
      tenantId: activeTenantId.value,
      dateFrom: integratedDateFrom.value,
      dateTo: integratedDateTo.value,
      storeIds: accessibleStores.value
        .map((store) => normalizeText(store?.id))
        .filter(Boolean)
        .sort(),
    })
  }

  function resetIntegratedCurrentMonth() {
    const nextRange = buildCurrentMonthRange()
    integratedDateFrom.value = nextRange.dateFrom
    integratedDateTo.value = nextRange.dateTo
  }

  function resetIntegratedPreviousMonth() {
    const nextRange = buildPreviousMonthRange()
    integratedDateFrom.value = nextRange.dateFrom
    integratedDateTo.value = nextRange.dateTo
  }

  // Recorte por semana (metas semanais): fatia fixa do mês do período ATIVO, então
  // compõe com "Mês anterior" (a semana usa o mês que estiver selecionado).
  function setIntegratedWeek(week: number) {
    const nextRange = buildMonthWeekRange(integratedDateFrom.value, week)
    integratedDateFrom.value = nextRange.dateFrom
    integratedDateTo.value = nextRange.dateTo
  }

  async function refreshIntegratedView() {
    await runtime.ensure()

    if (auth.isAuthenticated) {
      await auth.ensureSession()
    }

    const tenantId = activeTenantId.value
    const stores = accessibleStores.value

    if (!tenantId || !auth.isAuthenticated || !stores.length) {
      clearIntegratedView()
      return null
    }

    integratedPending.value = true
    integratedError.value = ''

    try {
      const scopeKey = buildIntegratedScopeKey()
      const crmParams = new URLSearchParams({ tenantId })
      if (integratedDateFrom.value) {
        crmParams.set('dateFrom', integratedDateFrom.value)
      }
      if (integratedDateTo.value) {
        crmParams.set('dateTo', integratedDateTo.value)
      }
      const [overviewResponse, storeResponses, erpCrmResponse] = await Promise.all([
        apiRequest('/v1/operations/overview'),
        Promise.all(
          stores.map(async (store) => {
            // store-staff é buscado POR LOJA (com storeId), igual /v1/consultants:
            // ensureStoreAccess valida a loja contra o Principal (platform_admin
            // libera qualquer loja; demais papéis pela própria StoreIDs). O fetch
            // global (/v1/store-staff sem storeId) dependia de resolveAccessibleStoreIDs
            // (lojas do AccountID), que diverge do escopo que o front enxerga -> staff
            // vinha vazio e os gerentes não apareciam.
            const [consultantsResponse, snapshotResponse, staffResponse] = await Promise.all([
              apiRequest(`/v1/consultants?storeId=${encodeURIComponent(store.id)}`),
              apiRequest(`/v1/operations/snapshot?storeId=${encodeURIComponent(store.id)}`),
              apiRequest(`/v1/store-staff?storeId=${encodeURIComponent(store.id)}`).catch(() => ({
                items: [],
              })),
            ])

            return {
              store,
              consultants: Array.isArray(consultantsResponse?.consultants)
                ? consultantsResponse.consultants
                : [],
              snapshot: snapshotResponse || {},
              staff: Array.isArray(staffResponse?.items) ? staffResponse.items : [],
            }
          }),
        ),
        apiRequest(`/v1/erp/crm?${crmParams.toString()}`).catch(() => null),
      ])

      integratedStaff.value = storeResponses.flatMap(({ staff }) =>
        (staff as Record<string, unknown>[]).map((item) => ({
          id: normalizeText(item?.id),
          name: normalizeText(item?.name),
          role: normalizeText(item?.role),
          roleLabel: normalizeText(item?.roleLabel),
          storeId: normalizeText(item?.storeId),
          storeName: normalizeText(item?.storeName),
        })),
      )

      integratedRoster.value = storeResponses.flatMap(({ store, consultants }) =>
        normalizeConsultantList(consultants, store),
      )
      integratedHistory.value = filterHistoryByDateRange(
        storeResponses.flatMap(({ store, snapshot }) =>
          normalizeServiceHistoryList(snapshot?.serviceHistory, store.id, store.name, Date.now()),
        ),
        integratedDateFrom.value,
        integratedDateTo.value,
      )
      integratedRanking.value = buildIntegratedRankingResponse(
        tenantId,
        integratedRoster.value,
        integratedHistory.value,
        erpCrmResponse,
        integratedDateFrom.value,
        integratedDateTo.value,
      )
      integratedOverview.value = overviewResponse
      integratedReady.value = true
      integratedScopeKey.value = scopeKey

      return {
        roster: integratedRoster.value,
        ranking: integratedRanking.value,
        overview: integratedOverview.value,
        history: integratedHistory.value,
      }
    } catch (error) {
      integratedError.value = getApiErrorMessage(
        error,
        'Nao foi possivel carregar o comparativo dos consultores.',
      )
      throw error
    } finally {
      integratedPending.value = false
    }
  }

  async function ensureIntegratedView() {
    const scopeKey = buildIntegratedScopeKey()

    if (integratedReady.value && integratedScopeKey.value === scopeKey) {
      return {
        roster: integratedRoster.value,
        ranking: integratedRanking.value,
        overview: integratedOverview.value,
        history: integratedHistory.value,
      }
    }

    try {
      return await refreshIntegratedView()
    } catch {
      return null
    }
  }

  async function applyIntegratedFilters() {
    return refreshIntegratedView()
  }

  async function createConsultantProfile(payload: Record<string, unknown>) {
    const storeId = await resolveActiveStoreId()

    if (!storeId || !auth.isAuthenticated) {
      return { ok: false, message: 'Sessao ou loja ativa indisponivel.' }
    }

    try {
      const response = await apiRequest('/v1/consultants', {
        method: 'POST',
        body: {
          storeId,
          ...normalizeConsultantPayload(payload),
        },
      })

      const runtimeContext = await hydrateRuntimeStoreContext(
        runtime,
        apiRequest,
        storeId,
        activeTenantId.value,
      )
      auth.applyRuntimeSettingsStatus(runtimeContext)
      return {
        ok: true,
        consultant: response?.consultant || null,
        access: response?.access || null,
      }
    } catch (error) {
      return {
        ok: false,
        message: getApiErrorMessage(error, 'Nao foi possivel criar consultor.'),
      }
    }
  }

  async function updateConsultantProfile(consultantId: string, payload: Record<string, unknown>) {
    const storeId = await resolveActiveStoreId()

    if (!storeId || !auth.isAuthenticated) {
      return { ok: false, message: 'Sessao ou loja ativa indisponivel.' }
    }

    try {
      await apiRequest(`/v1/consultants/${consultantId}`, {
        method: 'PATCH',
        body: normalizeConsultantPayload(payload),
      })

      const runtimeContext = await hydrateRuntimeStoreContext(
        runtime,
        apiRequest,
        storeId,
        activeTenantId.value,
      )
      auth.applyRuntimeSettingsStatus(runtimeContext)
      return { ok: true }
    } catch (error) {
      return {
        ok: false,
        message: getApiErrorMessage(error, 'Nao foi possivel atualizar consultor.'),
      }
    }
  }

  async function archiveConsultantProfile(consultantId: string) {
    const storeId = await resolveActiveStoreId()

    if (!storeId || !auth.isAuthenticated) {
      return { ok: false, message: 'Sessao ou loja ativa indisponivel.' }
    }

    const localValidation = canArchiveConsultantLocally(consultantId)
    if (localValidation.ok === false) {
      return localValidation
    }

    try {
      await apiRequest(`/v1/consultants/${consultantId}/archive`, {
        method: 'POST',
      })

      const runtimeContext = await hydrateRuntimeStoreContext(
        runtime,
        apiRequest,
        storeId,
        activeTenantId.value,
      )
      auth.applyRuntimeSettingsStatus(runtimeContext)
      return { ok: true }
    } catch (error) {
      return {
        ok: false,
        message: getApiErrorMessage(error, 'Nao foi possivel arquivar consultor.'),
      }
    }
  }

  if (import.meta.client) {
    watch(
      () => [auth.isAuthenticated, activeTenantId.value, accessibleStores.value.length],
      (
        [isAuthenticated, tenantId, storeCount],
        [previousAuthenticated, previousTenantId, previousStoreCount],
      ) => {
        if (!isAuthenticated || !tenantId || storeCount < 1) {
          clearIntegratedView()
          return
        }

        if (
          !previousAuthenticated ||
          previousTenantId !== tenantId ||
          previousStoreCount !== storeCount
        ) {
          clearIntegratedView()
        }
      },
    )
  }

  return {
    state,
    roster,
    selectedConsultantId,
    integratedRoster,
    integratedRanking,
    integratedOverview,
    integratedHistory,
    integratedStaff,
    integratedPending,
    integratedReady,
    integratedError,
    integratedDateFrom,
    integratedDateTo,
    ensure: runtime.ensure,
    refreshActiveStore,
    refreshIntegratedView,
    ensureIntegratedView,
    applyIntegratedFilters,
    clearIntegratedView,
    resetIntegratedCurrentMonth,
    resetIntegratedPreviousMonth,
    setIntegratedWeek,
    setSelectedConsultant(personId: string) {
      return runtime.run('setSelectedConsultant', personId)
    },
    setConsultantSimulationAdditionalSales(amount: number) {
      return runtime.run('setConsultantSimulationAdditionalSales', amount)
    },
    createConsultantProfile,
    updateConsultantProfile,
    archiveConsultantProfile,
  }
})
