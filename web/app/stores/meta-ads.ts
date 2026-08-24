import { computed, ref } from 'vue'
import { defineStore } from 'pinia'

import { useCoreAccountStore } from '../../layers/core/stores/account'
import { useAuthStore } from '~/stores/auth'
import { fetchChatScope, type CalendarChatScope } from '~/domain/calendar/calendar-chat-api'
import { createApiRequest, getApiErrorMessage } from '~/utils/api-client'
import type {
  MetaAdsAdAccount,
  MetaAdsCampaign,
  MetaAdsConnection,
  MetaAdsInsightPoint,
  MetaAdsInstagramIdentity,
  MetaAdsOAuthStart,
  MetaAdsOverview,
} from '~/types/meta-ads'

// Store do modulo meta-ads (MVP). API publica congelada pelo plano
// docs/meta-ads/PLANO_INTEGRACAO_META_ADS.md — os componentes em
// web/app/components/meta-ads/* dependem EXATAMENTE destes nomes (estados,
// computeds e actions). Nao renomear sem alinhar com o agente de componentes.
// X-Account-Id e injetado automaticamente pelo createApiRequest (account ativa).
const DEFAULT_RANGE = 'last_30d'
const DEFAULT_CLIENT_SCOPE: CalendarChatScope = {
  canSelect: false,
  lockedClientId: '',
  clients: [],
}

interface MetaAdsDataScope {
  accountId: string
  adAccountId: string
  range: string
  contextGeneration: number
}

interface MetaAdsSelectionRequest extends MetaAdsDataScope {
  generation: number
  signal: AbortSignal
}

interface MetaAdsReportRequest extends MetaAdsDataScope {
  generation: number
  signal: AbortSignal
}

interface MetaAdsDataRequest {
  selection: MetaAdsSelectionRequest
  report: MetaAdsReportRequest
}

export const useMetaAdsStore = defineStore('meta-ads', () => {
  const runtimeConfig = useRuntimeConfig()
  const auth = useAuthStore()
  const accountStore = useCoreAccountStore()
  const apiRequest = createApiRequest(runtimeConfig, () => auth.accessToken)

  const connection = ref<MetaAdsConnection | null>(null)
  const adAccounts = ref<MetaAdsAdAccount[]>([])
  const selectedAdAccountId = ref('')
  const overview = ref<MetaAdsOverview | null>(null)
  const campaigns = ref<MetaAdsCampaign[]>([])
  const insights = ref<MetaAdsInsightPoint[]>([])
  const range = ref(DEFAULT_RANGE)
  const pending = ref(false)
  const connecting = ref(false)
  const syncing = ref(false)
  const error = ref('')
  const clientScope = ref<CalendarChatScope>({ ...DEFAULT_CLIENT_SCOPE })
  const clientScopeLoading = ref(false)
  const clientMappingBusyId = ref('')
  const clientMappingError = ref('')
  const instagramIdentities = ref<MetaAdsInstagramIdentity[]>([])
  const instagramIdentitiesLoading = ref(false)
  const instagramIdentityMappingBusyId = ref('')
  const instagramIdentityMappingError = ref('')

  // Toda leitura/mutacao pertence a geracao da account ativa. Ao trocar account, o
  // workspace chama resetState(): requests antigas sao abortadas e respostas tardias
  // nao podem repopular a UI com dados do tenant anterior.
  let contextGeneration = 0
  let contextAbort = new AbortController()
  // A account ativa, a conta de anuncio e o periodo podem mudar sem trocar a
  // geracao global. Selecao e relatorio usam cancelamento independente para que
  // respostas fora de ordem nunca sobrescrevam o snapshot atual.
  let selectionGeneration = 0
  let selectionAbort: AbortController | null = null
  let reportGeneration = 0
  let reportAbort: AbortController | null = null

  const connected = computed(() => connection.value?.connected ?? false)
  const kpis = computed(() => overview.value?.kpis ?? null)
  const selectedAdAccount = computed(
    () => adAccounts.value.find((adAccount) => adAccount.id === selectedAdAccountId.value) ?? null,
  )
  const hasPrivilegedRole = computed(() => auth.role === 'platform_admin' || auth.role === 'owner')
  const canManageMetaAds = computed(
    () =>
      hasPrivilegedRole.value ||
      (auth.effectivePermissionsResolved &&
        auth.effectivePermissionKeys.includes('meta_ads.manage')),
  )
  const canConnectMetaAds = computed(
    () =>
      hasPrivilegedRole.value ||
      (auth.effectivePermissionsResolved &&
        auth.effectivePermissionKeys.includes('meta_ads.connect')),
  )
  const canManageClientMapping = computed(() => canManageMetaAds.value)

  function requestContext(): { generation: number; signal: AbortSignal } {
    return { generation: contextGeneration, signal: contextAbort.signal }
  }

  function isCurrentContext(generation: number): boolean {
    return generation === contextGeneration
  }

  function captureDataScope(): MetaAdsDataScope {
    return {
      accountId: String(accountStore.activeAccountId || '').trim(),
      adAccountId: String(selectedAdAccountId.value || '').trim(),
      range: String(range.value || '').trim() || DEFAULT_RANGE,
      contextGeneration,
    }
  }

  function dataScopeHeaders(scope: MetaAdsDataScope): Record<string, string> | undefined {
    return scope.accountId ? { 'X-Account-Id': scope.accountId } : undefined
  }

  function isCurrentDataScope(scope: MetaAdsDataScope): boolean {
    return (
      isCurrentContext(scope.contextGeneration) &&
      scope.accountId === String(accountStore.activeAccountId || '').trim() &&
      scope.adAccountId === String(selectedAdAccountId.value || '').trim() &&
      scope.range === (String(range.value || '').trim() || DEFAULT_RANGE)
    )
  }

  function abortSelectionLoad(): void {
    selectionGeneration += 1
    selectionAbort?.abort()
    selectionAbort = null
  }

  function abortReportLoad(): void {
    reportGeneration += 1
    reportAbort?.abort()
    reportAbort = null
  }

  function beginSelectionRequest(scope = captureDataScope()): MetaAdsSelectionRequest {
    abortSelectionLoad()
    selectionAbort = new AbortController()
    return {
      ...scope,
      generation: selectionGeneration,
      signal: selectionAbort.signal,
    }
  }

  function beginReportRequest(scope = captureDataScope()): MetaAdsReportRequest {
    abortReportLoad()
    reportAbort = new AbortController()
    return {
      ...scope,
      generation: reportGeneration,
      signal: reportAbort.signal,
    }
  }

  function beginDataRequest(): MetaAdsDataRequest {
    const scope = captureDataScope()
    return {
      selection: beginSelectionRequest(scope),
      report: beginReportRequest(scope),
    }
  }

  function isCurrentSelectionRequest(request: MetaAdsSelectionRequest): boolean {
    return (
      request.generation === selectionGeneration &&
      selectionAbort?.signal === request.signal &&
      isCurrentDataScope(request)
    )
  }

  function isCurrentReportRequest(request: MetaAdsReportRequest): boolean {
    return (
      request.generation === reportGeneration &&
      reportAbort?.signal === request.signal &&
      isCurrentDataScope(request)
    )
  }

  function isCurrentDataRequest(request: MetaAdsDataRequest): boolean {
    return isCurrentSelectionRequest(request.selection) && isCurrentReportRequest(request.report)
  }

  function cancelDataLoads(): void {
    abortSelectionLoad()
    abortReportLoad()
    pending.value = false
  }

  function resetState() {
    contextGeneration += 1
    contextAbort.abort()
    contextAbort = new AbortController()
    cancelDataLoads()
    connection.value = null
    adAccounts.value = []
    selectedAdAccountId.value = ''
    overview.value = null
    campaigns.value = []
    insights.value = []
    range.value = DEFAULT_RANGE
    pending.value = false
    connecting.value = false
    syncing.value = false
    error.value = ''
    clientScope.value = { ...DEFAULT_CLIENT_SCOPE }
    clientScopeLoading.value = false
    clientMappingBusyId.value = ''
    clientMappingError.value = ''
    instagramIdentities.value = []
    instagramIdentitiesLoading.value = false
    instagramIdentityMappingBusyId.value = ''
    instagramIdentityMappingError.value = ''
  }

  async function loadOverviewForRequest(request: MetaAdsSelectionRequest): Promise<void> {
    const query = request.adAccountId
      ? `?adAccountId=${encodeURIComponent(request.adAccountId)}`
      : ''
    const response = (await apiRequest(`/v1/meta-ads/overview${query}`, {
      method: 'GET',
      signal: request.signal,
      headers: dataScopeHeaders(request),
    })) as MetaAdsOverview
    if (!isCurrentSelectionRequest(request)) return
    overview.value = response
    if (response?.connection) {
      connection.value = response.connection
    }
  }

  async function loadOverview(): Promise<void> {
    const request = beginSelectionRequest()
    try {
      await loadOverviewForRequest(request)
    } catch (caught) {
      if (isCurrentSelectionRequest(request)) throw caught
    }
  }

  async function loadAdAccounts() {
    const request = requestContext()
    const response = (await apiRequest('/v1/meta-ads/ad-accounts', {
      method: 'GET',
      signal: request.signal,
    })) as { adAccounts?: MetaAdsAdAccount[] } | MetaAdsAdAccount[]
    if (!isCurrentContext(request.generation)) return
    adAccounts.value = Array.isArray(response) ? response : (response?.adAccounts ?? [])
  }

  async function loadClientScope(): Promise<void> {
    const request = requestContext()
    clientScopeLoading.value = true
    clientMappingError.value = ''
    try {
      const scope = await fetchChatScope(apiRequest, request.signal)
      if (!isCurrentContext(request.generation)) return
      clientScope.value = scope
    } catch (caught) {
      if (isCurrentContext(request.generation)) {
        clientScope.value = { ...DEFAULT_CLIENT_SCOPE }
        clientMappingError.value = getApiErrorMessage(
          caught,
          'Nao foi possivel carregar os clientes visiveis.',
        )
      }
    } finally {
      if (isCurrentContext(request.generation)) clientScopeLoading.value = false
    }
  }

  async function setAdAccountClient(adAccountId: string, clientAccountId: string): Promise<void> {
    const id = String(adAccountId || '').trim()
    if (!id || !canManageClientMapping.value) return
    const request = requestContext()
    clientMappingBusyId.value = id
    clientMappingError.value = ''
    try {
      const updated = (await apiRequest(
        `/v1/meta-ads/ad-accounts/${encodeURIComponent(id)}/client`,
        {
          method: 'PATCH',
          body: { clientAccountId: String(clientAccountId || '').trim() },
          signal: request.signal,
        },
      )) as MetaAdsAdAccount
      if (!isCurrentContext(request.generation)) return
      adAccounts.value = adAccounts.value.map((adAccount) =>
        adAccount.id === id ? updated : adAccount,
      )
    } catch (caught) {
      if (isCurrentContext(request.generation)) {
        clientMappingError.value = getApiErrorMessage(
          caught,
          'Nao foi possivel vincular a conta de anuncio ao cliente.',
        )
      }
    } finally {
      if (isCurrentContext(request.generation) && clientMappingBusyId.value === id) {
        clientMappingBusyId.value = ''
      }
    }
  }

  async function loadInstagramIdentities(): Promise<void> {
    const request = requestContext()
    instagramIdentitiesLoading.value = true
    instagramIdentityMappingError.value = ''
    try {
      const response = (await apiRequest('/v1/meta-ads/instagram-identities', {
        method: 'GET',
        signal: request.signal,
      })) as MetaAdsInstagramIdentity[] | { identities?: MetaAdsInstagramIdentity[] }
      if (!isCurrentContext(request.generation)) return
      instagramIdentities.value = Array.isArray(response) ? response : (response.identities ?? [])
    } catch (caught) {
      if (isCurrentContext(request.generation)) {
        instagramIdentities.value = []
        instagramIdentityMappingError.value = getApiErrorMessage(
          caught,
          'Nao foi possivel carregar as Paginas e contas do Instagram.',
        )
      }
    } finally {
      if (isCurrentContext(request.generation)) instagramIdentitiesLoading.value = false
    }
  }

  async function setInstagramIdentityClient(
    igUserId: string,
    clientAccountId: string,
  ): Promise<void> {
    const id = String(igUserId || '').trim()
    if (!id || !canManageClientMapping.value) return
    const request = requestContext()
    instagramIdentityMappingBusyId.value = id
    instagramIdentityMappingError.value = ''
    try {
      const updated = (await apiRequest(
        `/v1/meta-ads/instagram-identities/${encodeURIComponent(id)}/client`,
        {
          method: 'PATCH',
          body: { clientAccountId: String(clientAccountId || '').trim() },
          signal: request.signal,
        },
      )) as MetaAdsInstagramIdentity
      if (!isCurrentContext(request.generation)) return
      instagramIdentities.value = instagramIdentities.value.map((identity) =>
        identity.igUserId === id ? updated : identity,
      )
    } catch (caught) {
      if (isCurrentContext(request.generation)) {
        instagramIdentityMappingError.value = getApiErrorMessage(
          caught,
          'Nao foi possivel vincular a identidade do Instagram ao cliente.',
        )
      }
    } finally {
      if (isCurrentContext(request.generation) && instagramIdentityMappingBusyId.value === id) {
        instagramIdentityMappingBusyId.value = ''
      }
    }
  }

  async function loadCampaignsForRequest(request: MetaAdsSelectionRequest): Promise<void> {
    const response = (await apiRequest(
      `/v1/meta-ads/campaigns?adAccountId=${encodeURIComponent(request.adAccountId)}`,
      {
        method: 'GET',
        signal: request.signal,
        headers: dataScopeHeaders(request),
      },
    )) as { campaigns?: MetaAdsCampaign[] } | MetaAdsCampaign[]
    if (!isCurrentSelectionRequest(request)) return
    campaigns.value = Array.isArray(response) ? response : (response?.campaigns ?? [])
  }

  async function loadCampaigns(): Promise<void> {
    const request = beginSelectionRequest()
    try {
      await loadCampaignsForRequest(request)
    } catch (caught) {
      if (isCurrentSelectionRequest(request)) throw caught
    }
  }

  async function loadInsightsForRequest(request: MetaAdsReportRequest): Promise<void> {
    const response = (await apiRequest(
      `/v1/meta-ads/insights?adAccountId=${encodeURIComponent(
        request.adAccountId,
      )}&range=${encodeURIComponent(request.range)}&level=account`,
      {
        method: 'GET',
        signal: request.signal,
        headers: dataScopeHeaders(request),
      },
    )) as { insights?: MetaAdsInsightPoint[] } | MetaAdsInsightPoint[]
    if (!isCurrentReportRequest(request)) return
    insights.value = Array.isArray(response) ? response : (response?.insights ?? [])
  }

  async function loadInsights(): Promise<void> {
    const request = beginReportRequest()
    try {
      await loadInsightsForRequest(request)
    } catch (caught) {
      if (isCurrentReportRequest(request)) throw caught
    }
  }

  async function loadSelectedData(request: MetaAdsDataRequest): Promise<void> {
    try {
      await Promise.all([
        loadOverviewForRequest(request.selection),
        loadCampaignsForRequest(request.selection),
        loadInsightsForRequest(request.report),
      ])
    } catch (caught) {
      if (isCurrentDataRequest(request)) throw caught
    }
  }

  async function selectAdAccount(id: string) {
    selectedAdAccountId.value = String(id || '').trim()
    cancelDataLoads()
    overview.value = null
    campaigns.value = []
    insights.value = []

    if (!selectedAdAccountId.value) {
      return
    }
    const request = beginDataRequest()

    pending.value = true
    error.value = ''
    try {
      await loadSelectedData(request)
    } catch (caught) {
      if (isCurrentDataRequest(request)) {
        error.value = getApiErrorMessage(caught, 'Nao foi possivel carregar a conta de anuncio.')
      }
    } finally {
      if (isCurrentDataRequest(request)) pending.value = false
    }
  }

  async function init() {
    const request = requestContext()
    pending.value = true
    error.value = ''
    try {
      const response = (await apiRequest('/v1/meta-ads/overview', {
        method: 'GET',
        signal: request.signal,
      })) as MetaAdsOverview
      if (!isCurrentContext(request.generation)) return
      overview.value = response
      connection.value = response?.connection ?? null

      if (connected.value) {
        await loadAdAccounts()
        if (!isCurrentContext(request.generation)) return
        const first = adAccounts.value[0]
        if (first) {
          await selectAdAccount(first.id)
        }
      }
    } catch (caught) {
      if (isCurrentContext(request.generation)) {
        error.value = getApiErrorMessage(caught, 'Nao foi possivel carregar o Meta Ads.')
      }
    } finally {
      if (isCurrentContext(request.generation)) pending.value = false
    }
  }

  async function saveConnection(token: string) {
    if (!canConnectMetaAds.value) return
    const request = requestContext()
    connecting.value = true
    error.value = ''
    try {
      await apiRequest('/v1/meta-ads/connection', {
        method: 'POST',
        body: { token },
        signal: request.signal,
      })
      if (!isCurrentContext(request.generation)) return
      await init()
    } catch (caught) {
      if (isCurrentContext(request.generation)) {
        error.value = getApiErrorMessage(caught, 'Nao foi possivel conectar na Meta.')
      }
    } finally {
      if (isCurrentContext(request.generation)) connecting.value = false
    }
  }

  async function startConnectionOAuth(): Promise<MetaAdsOAuthStart | null> {
    if (!canConnectMetaAds.value) return null
    const request = requestContext()
    connecting.value = true
    error.value = ''
    try {
      const response = (await apiRequest('/v1/meta-ads/oauth/start', {
        method: 'POST',
        signal: request.signal,
      })) as MetaAdsOAuthStart
      if (!isCurrentContext(request.generation)) return null
      return response
    } catch (caught) {
      if (isCurrentContext(request.generation)) {
        error.value = getApiErrorMessage(
          caught,
          'Nao foi possivel iniciar o Facebook Login. Use a conexao manual se necessario.',
        )
      }
      return null
    } finally {
      if (isCurrentContext(request.generation)) connecting.value = false
    }
  }

  async function deleteConnection() {
    if (!canConnectMetaAds.value) return
    const request = requestContext()
    error.value = ''
    try {
      await apiRequest('/v1/meta-ads/connection', {
        method: 'DELETE',
        signal: request.signal,
      })
      if (!isCurrentContext(request.generation)) return
      resetState()
    } catch (caught) {
      if (isCurrentContext(request.generation)) {
        error.value = getApiErrorMessage(caught, 'Nao foi possivel desconectar.')
      }
    }
  }

  async function sync() {
    if (!canManageMetaAds.value || !selectedAdAccountId.value) return
    const request = requestContext()
    syncing.value = true
    error.value = ''
    try {
      await apiRequest('/v1/meta-ads/sync', {
        method: 'POST',
        body: { adAccountId: selectedAdAccountId.value },
        signal: request.signal,
      })
      if (!isCurrentContext(request.generation)) return
      await loadSelectedData(beginDataRequest())
    } catch (caught) {
      if (isCurrentContext(request.generation)) {
        error.value = getApiErrorMessage(caught, 'Nao foi possivel sincronizar com a Meta.')
      }
    } finally {
      if (isCurrentContext(request.generation)) syncing.value = false
    }
  }

  async function setRange(nextRange: string) {
    range.value = String(nextRange || '').trim() || DEFAULT_RANGE
    insights.value = []

    if (!selectedAdAccountId.value) {
      abortReportLoad()
      pending.value = false
      return
    }
    const request = beginReportRequest()

    pending.value = true
    error.value = ''
    try {
      await loadInsightsForRequest(request)
    } catch (caught) {
      if (isCurrentReportRequest(request)) {
        error.value = getApiErrorMessage(caught, 'Nao foi possivel carregar as metricas.')
      }
    } finally {
      if (isCurrentReportRequest(request)) pending.value = false
    }
  }

  return {
    connection,
    adAccounts,
    selectedAdAccountId,
    overview,
    campaigns,
    insights,
    range,
    pending,
    connecting,
    syncing,
    error,
    clientScope,
    clientScopeLoading,
    clientMappingBusyId,
    clientMappingError,
    instagramIdentities,
    instagramIdentitiesLoading,
    instagramIdentityMappingBusyId,
    instagramIdentityMappingError,
    connected,
    kpis,
    selectedAdAccount,
    canManageMetaAds,
    canConnectMetaAds,
    canManageClientMapping,
    resetState,
    cancelDataLoads,
    init,
    loadAdAccounts,
    loadClientScope,
    setAdAccountClient,
    loadInstagramIdentities,
    setInstagramIdentityClient,
    selectAdAccount,
    loadOverview,
    loadCampaigns,
    loadInsights,
    startConnectionOAuth,
    saveConnection,
    deleteConnection,
    sync,
    setRange,
  }
})
