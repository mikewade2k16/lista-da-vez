import { computed, ref } from 'vue'
import { defineStore } from 'pinia'

import { useAuthStore } from '~/stores/auth'
import { createApiRequest, getApiErrorMessage } from '~/utils/api-client'
import type {
  MetaAdsAdAccount,
  MetaAdsAssistantHealth,
  MetaAdsAssistantMessage,
  MetaAdsAssistantSettings,
  MetaAdsCampaign,
  MetaAdsConnection,
  MetaAdsInsightPoint,
  MetaAdsOverview,
} from '~/types/meta-ads'

// Store do modulo meta-ads (MVP). API publica congelada pelo plano
// docs/meta-ads/PLANO_INTEGRACAO_META_ADS.md — os componentes em
// web/app/components/meta-ads/* dependem EXATAMENTE destes nomes (estados,
// computeds e actions). Nao renomear sem alinhar com o agente de componentes.
// X-Account-Id e injetado automaticamente pelo createApiRequest (account ativa).
const DEFAULT_RANGE = 'last_30d'

// Shape minimo dos erros do $fetch/ofetch para mapear os codigos do assistente
// (503 assistant_not_configured / 502 assistant_error) sem usar `any`.
interface AssistantApiError {
  statusCode?: number
}

function assistantSendErrorMessage(caught: unknown): string {
  const statusCode = (caught as AssistantApiError | null)?.statusCode
  if (statusCode === 503) {
    return 'Assistente nao configurado no servidor. Suba o runner (meta-ads-assistant/README) e tente de novo.'
  }
  if (statusCode === 502) {
    return 'O assistente falhou ao executar o comando. Tente novamente em instantes.'
  }
  return getApiErrorMessage(caught, 'Nao foi possivel falar com o assistente.')
}

// Backend persiste actions como jsonb — defensivo contra null no lugar de [].
function normalizeAssistantMessage(message: MetaAdsAssistantMessage): MetaAdsAssistantMessage {
  return { ...message, actions: Array.isArray(message.actions) ? message.actions : [] }
}

export const useMetaAdsStore = defineStore('meta-ads', () => {
  const runtimeConfig = useRuntimeConfig()
  const auth = useAuthStore()
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

  // --- Assistente MCP (chat) ---
  const assistantMessages = ref<MetaAdsAssistantMessage[]>([])
  const assistantSending = ref(false)
  const assistantError = ref('')
  const assistantHealth = ref<MetaAdsAssistantHealth | null>(null)

  // --- Login do assistente no MCP da Meta (OAuth dirigido pelo painel) ---
  const assistantAuthUrl = ref('')
  const assistantAuthBusy = ref(false)
  const assistantAuthError = ref('')
  const assistantAuthDone = ref(false)

  // AbortController do envio em andamento (botao Cancelar).
  let assistantSendAbort: AbortController | null = null

  // --- Configuracoes do assistente (modelo + system prompt editaveis) ---
  const assistantSettings = ref<MetaAdsAssistantSettings | null>(null)
  const assistantSettingsSaving = ref(false)
  const assistantSettingsError = ref('')

  const connected = computed(() => connection.value?.connected ?? false)
  const kpis = computed(() => overview.value?.kpis ?? null)
  const selectedAdAccount = computed(
    () => adAccounts.value.find((adAccount) => adAccount.id === selectedAdAccountId.value) ?? null,
  )

  function resetState() {
    connection.value = null
    adAccounts.value = []
    selectedAdAccountId.value = ''
    overview.value = null
    campaigns.value = []
    insights.value = []
    range.value = DEFAULT_RANGE
    assistantMessages.value = []
    assistantSending.value = false
    assistantError.value = ''
    assistantHealth.value = null
    assistantAuthUrl.value = ''
    assistantAuthBusy.value = false
    assistantAuthError.value = ''
    assistantAuthDone.value = false
    assistantSettings.value = null
    assistantSettingsSaving.value = false
    assistantSettingsError.value = ''
  }

  async function loadOverview() {
    const query = selectedAdAccountId.value
      ? `?adAccountId=${encodeURIComponent(selectedAdAccountId.value)}`
      : ''
    const response = (await apiRequest(`/v1/meta-ads/overview${query}`, {
      method: 'GET',
    })) as MetaAdsOverview
    overview.value = response
    if (response?.connection) {
      connection.value = response.connection
    }
  }

  async function loadAdAccounts() {
    const response = (await apiRequest('/v1/meta-ads/ad-accounts', {
      method: 'GET',
    })) as { adAccounts?: MetaAdsAdAccount[] } | MetaAdsAdAccount[]
    adAccounts.value = Array.isArray(response) ? response : (response?.adAccounts ?? [])
  }

  async function loadCampaigns() {
    const response = (await apiRequest(
      `/v1/meta-ads/campaigns?adAccountId=${encodeURIComponent(selectedAdAccountId.value)}`,
      { method: 'GET' },
    )) as { campaigns?: MetaAdsCampaign[] } | MetaAdsCampaign[]
    campaigns.value = Array.isArray(response) ? response : (response?.campaigns ?? [])
  }

  async function loadInsights() {
    const response = (await apiRequest(
      `/v1/meta-ads/insights?adAccountId=${encodeURIComponent(
        selectedAdAccountId.value,
      )}&range=${encodeURIComponent(range.value)}&level=account`,
      { method: 'GET' },
    )) as { insights?: MetaAdsInsightPoint[] } | MetaAdsInsightPoint[]
    insights.value = Array.isArray(response) ? response : (response?.insights ?? [])
  }

  async function selectAdAccount(id: string) {
    selectedAdAccountId.value = String(id || '').trim()

    if (!selectedAdAccountId.value) {
      return
    }

    pending.value = true
    error.value = ''
    try {
      await Promise.all([loadOverview(), loadCampaigns(), loadInsights()])
    } catch (caught) {
      error.value = getApiErrorMessage(caught, 'Nao foi possivel carregar a conta de anuncio.')
    } finally {
      pending.value = false
    }
  }

  async function init() {
    pending.value = true
    error.value = ''
    try {
      const response = (await apiRequest('/v1/meta-ads/overview', {
        method: 'GET',
      })) as MetaAdsOverview
      overview.value = response
      connection.value = response?.connection ?? null

      if (connected.value) {
        await loadAdAccounts()
        const first = adAccounts.value[0]
        if (first) {
          await selectAdAccount(first.id)
        }
      }
    } catch (caught) {
      error.value = getApiErrorMessage(caught, 'Nao foi possivel carregar o Meta Ads.')
    } finally {
      pending.value = false
    }
  }

  async function saveConnection(token: string) {
    connecting.value = true
    error.value = ''
    try {
      await apiRequest('/v1/meta-ads/connection', {
        method: 'POST',
        body: { token },
      })
      await init()
    } catch (caught) {
      error.value = getApiErrorMessage(caught, 'Nao foi possivel conectar na Meta.')
    } finally {
      connecting.value = false
    }
  }

  async function deleteConnection() {
    error.value = ''
    try {
      await apiRequest('/v1/meta-ads/connection', { method: 'DELETE' })
      resetState()
    } catch (caught) {
      error.value = getApiErrorMessage(caught, 'Nao foi possivel desconectar.')
    }
  }

  async function sync() {
    syncing.value = true
    error.value = ''
    try {
      await apiRequest('/v1/meta-ads/sync', {
        method: 'POST',
        body: { adAccountId: selectedAdAccountId.value },
      })
      await Promise.all([loadOverview(), loadCampaigns(), loadInsights()])
    } catch (caught) {
      error.value = getApiErrorMessage(caught, 'Nao foi possivel sincronizar com a Meta.')
    } finally {
      syncing.value = false
    }
  }

  async function setRange(nextRange: string) {
    range.value = String(nextRange || '').trim() || DEFAULT_RANGE

    if (!selectedAdAccountId.value) {
      return
    }

    pending.value = true
    error.value = ''
    try {
      await loadInsights()
    } catch (caught) {
      error.value = getApiErrorMessage(caught, 'Nao foi possivel carregar as metricas.')
    } finally {
      pending.value = false
    }
  }

  // --- Assistente MCP (chat texto → acoes na Meta via runner headless) ---

  async function loadAssistantHealth() {
    try {
      const response = (await apiRequest('/v1/meta-ads/assistant/health', {
        method: 'GET',
      })) as MetaAdsAssistantHealth
      // Normaliza defensivamente (detail nulo nao pode quebrar os computeds da UI).
      assistantHealth.value = {
        ok: Boolean(response?.ok),
        claudeAuth: Boolean(response?.claudeAuth),
        detail: String(response?.detail ?? ''),
      }
    } catch {
      // Health responde 200 sempre; falha aqui = backend/rede fora.
      assistantHealth.value = {
        ok: false,
        claudeAuth: false,
        detail: 'Nao foi possivel consultar a saude do assistente.',
      }
    }
  }

  async function loadAssistant() {
    assistantError.value = ''
    try {
      const response = (await apiRequest('/v1/meta-ads/assistant/messages?limit=50', {
        method: 'GET',
      })) as MetaAdsAssistantMessage[]
      assistantMessages.value = (Array.isArray(response) ? response : []).map(
        normalizeAssistantMessage,
      )
    } catch (caught) {
      assistantError.value = getApiErrorMessage(
        caught,
        'Nao foi possivel carregar o historico do assistente.',
      )
    }
    await loadAssistantHealth()
  }

  // Envia um comando. A resposta pode levar 30-120s (Claude + MCP da Meta):
  // skipLoadingIndicator evita a barra global e o estado fica em assistantSending.
  // Retorna true em sucesso para o componente decidir manter/restaurar o rascunho.
  async function sendAssistant(text: string): Promise<boolean> {
    const trimmed = String(text || '').trim()
    if (!trimmed || assistantSending.value) {
      return false
    }
    if (!selectedAdAccountId.value) {
      assistantError.value = 'Selecione uma conta de anuncio antes de usar o assistente.'
      return false
    }

    assistantSending.value = true
    assistantError.value = ''

    // Eco local imediato (resposta imediata ao clique); o backend devolve a
    // mensagem do usuario persistida + a resposta — o eco e substituido por elas.
    const localId = `local-${Date.now()}`
    assistantMessages.value = [
      ...assistantMessages.value,
      {
        id: localId,
        role: 'user',
        content: trimmed,
        actions: [],
        createdAt: new Date().toISOString(),
      },
    ]

    const abort = new AbortController()
    assistantSendAbort = abort
    try {
      const response = (await apiRequest('/v1/meta-ads/assistant/messages', {
        method: 'POST',
        body: { message: trimmed, adAccountId: selectedAdAccountId.value },
        skipLoadingIndicator: true,
        signal: abort.signal,
      })) as { messages?: MetaAdsAssistantMessage[]; syncTriggered?: boolean }

      const returned = (response?.messages ?? []).map(normalizeAssistantMessage)
      assistantMessages.value = [
        ...assistantMessages.value.filter((message) => message.id !== localId),
        ...returned,
      ]

      if (response?.syncTriggered) {
        try {
          await Promise.all([loadOverview(), loadCampaigns(), loadInsights()])
        } catch (caught) {
          error.value = getApiErrorMessage(
            caught,
            'Acao concluida, mas nao foi possivel atualizar os dados. Sincronize manualmente.',
          )
        }
      }
      return true
    } catch (caught) {
      assistantMessages.value = assistantMessages.value.filter((message) => message.id !== localId)
      // Cancelado pelo usuario: sem erro vermelho.
      assistantError.value = abort.signal.aborted ? '' : assistantSendErrorMessage(caught)
      return false
    } finally {
      assistantSendAbort = null
      assistantSending.value = false
    }
  }

  // Limpa todo o historico do chat (botao Limpar conversa).
  async function clearAssistant() {
    if (assistantSending.value) return
    assistantError.value = ''
    try {
      await apiRequest('/v1/meta-ads/assistant/messages', { method: 'DELETE' })
      assistantMessages.value = []
    } catch (caught) {
      assistantError.value = getApiErrorMessage(caught, 'Nao foi possivel limpar a conversa.')
    }
  }

  // Cancela o envio em andamento (aborta a requisicao; o eco local sai no catch).
  function cancelAssistant() {
    if (assistantSendAbort) {
      assistantSendAbort.abort()
    }
  }

  // Carrega as configuracoes do assistente (modelo + system prompt).
  async function loadAssistantSettings() {
    assistantSettingsError.value = ''
    try {
      const response = (await apiRequest('/v1/meta-ads/assistant/settings', {
        method: 'GET',
      })) as MetaAdsAssistantSettings
      assistantSettings.value = {
        model: String(response?.model ?? ''),
        systemPrompt: String(response?.systemPrompt ?? ''),
      }
    } catch (caught) {
      assistantSettingsError.value = getApiErrorMessage(
        caught,
        'Nao foi possivel carregar as configuracoes.',
      )
    }
  }

  // Salva modelo + system prompt; devolve true em sucesso.
  async function saveAssistantSettings(model: string, systemPrompt: string): Promise<boolean> {
    assistantSettingsSaving.value = true
    assistantSettingsError.value = ''
    try {
      const response = (await apiRequest('/v1/meta-ads/assistant/settings', {
        method: 'PUT',
        body: { model, systemPrompt },
      })) as MetaAdsAssistantSettings
      assistantSettings.value = {
        model: String(response?.model ?? ''),
        systemPrompt: String(response?.systemPrompt ?? ''),
      }
      return true
    } catch (caught) {
      assistantSettingsError.value = getApiErrorMessage(
        caught,
        'Nao foi possivel salvar as configuracoes.',
      )
      return false
    } finally {
      assistantSettingsSaving.value = false
    }
  }

  // Inicia o login do MCP da Meta: pede a URL de autorizacao ao runner. O
  // usuario abre a URL, autoriza, e cola a URL de callback em completeAssistantAuth.
  async function startAssistantAuth() {
    assistantAuthBusy.value = true
    assistantAuthError.value = ''
    assistantAuthUrl.value = ''
    assistantAuthDone.value = false
    try {
      const response = (await apiRequest('/v1/meta-ads/assistant/auth/start', {
        method: 'POST',
        skipLoadingIndicator: true,
      })) as { url?: string }
      assistantAuthUrl.value = String(response?.url ?? '')
      if (!assistantAuthUrl.value) {
        // Sem URL = o assistente ja esta autenticado nesta sessao.
        assistantAuthDone.value = true
        await loadAssistantHealth()
      }
    } catch (caught) {
      assistantAuthError.value = assistantSendErrorMessage(caught)
    } finally {
      assistantAuthBusy.value = false
    }
  }

  // Conclui o login com a URL de callback (localhost/callback?code=...) colada.
  async function completeAssistantAuth(callbackUrl: string): Promise<boolean> {
    // callbackUrl e opcional: com a sessao persistente o login pode concluir
    // sozinho (redirect localhost capturado com a conexao viva).
    const trimmed = String(callbackUrl || '').trim()
    if (assistantAuthBusy.value) {
      return false
    }
    assistantAuthBusy.value = true
    assistantAuthError.value = ''
    try {
      const response = (await apiRequest('/v1/meta-ads/assistant/auth/complete', {
        method: 'POST',
        body: { callbackUrl: trimmed },
        skipLoadingIndicator: true,
      })) as { ok?: boolean; detail?: string }
      if (response?.ok) {
        assistantAuthDone.value = true
        assistantAuthUrl.value = ''
        await loadAssistantHealth()
        return true
      }
      assistantAuthError.value =
        String(response?.detail ?? '') ||
        'Nao foi possivel concluir o login. Confira a URL de callback e tente de novo.'
      return false
    } catch (caught) {
      assistantAuthError.value = assistantSendErrorMessage(caught)
      return false
    } finally {
      assistantAuthBusy.value = false
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
    connected,
    kpis,
    selectedAdAccount,
    init,
    loadAdAccounts,
    selectAdAccount,
    loadOverview,
    loadCampaigns,
    loadInsights,
    saveConnection,
    deleteConnection,
    sync,
    setRange,
    assistantMessages,
    assistantSending,
    assistantError,
    assistantHealth,
    loadAssistant,
    loadAssistantHealth,
    sendAssistant,
    clearAssistant,
    cancelAssistant,
    assistantSettings,
    assistantSettingsSaving,
    assistantSettingsError,
    loadAssistantSettings,
    saveAssistantSettings,
    assistantAuthUrl,
    assistantAuthBusy,
    assistantAuthError,
    assistantAuthDone,
    startAssistantAuth,
    completeAssistantAuth,
  }
})
