import { createApiRequest, getApiErrorMessage } from '~/utils/api-client'

export interface AutomationView {
  id: string
  name: string
  slug: string
  type: string
  status: string
  enabled: boolean
}

export interface WhatsAppView {
  provider: string
  sessionName: string
  status: string
  connected: boolean
  connectedPhone?: string
}

export interface AutomationOverview {
  automation: AutomationView
  whatsapp: WhatsAppView
}

interface ConnectResponse {
  status: string
  qr?: string
  connectedPhone?: string
}

export interface PersonaView {
  id: string
  name: string
  systemPrompt: string
}

const POLL_INTERVAL_MS = 3000

export function useAutomation() {
  const runtimeConfig = useRuntimeConfig()
  const auth = useAuthStore()
  const apiRequest = createApiRequest(runtimeConfig, () => auth.accessToken)

  const overview = ref<AutomationOverview | null>(null)
  const qr = ref('')
  const loading = ref(false)
  const connecting = ref(false)
  const savingEnabled = ref(false)
  const errorMessage = ref('')

  const personaName = ref('')
  const personaPrompt = ref('')
  const personaLoading = ref(false)
  const savingPersona = ref(false)
  const personaSavedAt = ref(0)

  let pollTimer: ReturnType<typeof setInterval> | null = null

  const connected = computed(() => overview.value?.whatsapp.connected ?? false)
  const whatsappStatus = computed(() => overview.value?.whatsapp.status ?? 'STOPPED')
  const connectedPhone = computed(() => overview.value?.whatsapp.connectedPhone ?? '')
  const enabled = computed(() => overview.value?.automation.enabled ?? false)

  async function load() {
    loading.value = true
    errorMessage.value = ''
    try {
      overview.value = (await apiRequest('/v1/automation', { method: 'GET' })) as AutomationOverview
      if (connected.value) qr.value = ''
    } catch (error) {
      errorMessage.value = getApiErrorMessage(error, 'Falha ao carregar a automacao.')
    } finally {
      loading.value = false
    }
  }

  async function connect() {
    connecting.value = true
    errorMessage.value = ''
    try {
      const res = (await apiRequest('/v1/automation/whatsapp/connect', {
        method: 'POST',
      })) as ConnectResponse
      if (res.status === 'WORKING') {
        qr.value = ''
        await load()
      } else {
        qr.value = res.qr ?? ''
        startPolling()
      }
    } catch (error) {
      errorMessage.value = getApiErrorMessage(error, 'Nao foi possivel conectar.')
    } finally {
      connecting.value = false
    }
  }

  async function disconnect() {
    errorMessage.value = ''
    try {
      await apiRequest('/v1/automation/whatsapp/disconnect', { method: 'POST' })
      qr.value = ''
      stopPolling()
      await load()
    } catch (error) {
      errorMessage.value = getApiErrorMessage(error, 'Nao foi possivel desconectar.')
    }
  }

  async function setEnabled(value: boolean) {
    savingEnabled.value = true
    errorMessage.value = ''
    try {
      const view = (await apiRequest('/v1/automation/settings', {
        method: 'PUT',
        body: { enabled: value },
      })) as AutomationView
      if (overview.value) overview.value.automation = view
    } catch (error) {
      errorMessage.value = getApiErrorMessage(error, 'Nao foi possivel salvar.')
    } finally {
      savingEnabled.value = false
    }
  }

  async function loadPersona() {
    personaLoading.value = true
    errorMessage.value = ''
    try {
      const p = (await apiRequest('/v1/automation/persona', { method: 'GET' })) as PersonaView
      personaName.value = p.name
      personaPrompt.value = p.systemPrompt
    } catch (error) {
      errorMessage.value = getApiErrorMessage(error, 'Falha ao carregar a persona.')
    } finally {
      personaLoading.value = false
    }
  }

  async function savePersona() {
    if (!personaPrompt.value.trim()) {
      errorMessage.value = 'O comportamento não pode ficar vazio.'
      return
    }
    savingPersona.value = true
    errorMessage.value = ''
    try {
      const p = (await apiRequest('/v1/automation/persona', {
        method: 'PUT',
        body: { name: personaName.value, systemPrompt: personaPrompt.value },
      })) as PersonaView
      personaName.value = p.name
      personaPrompt.value = p.systemPrompt
      personaSavedAt.value = Date.now()
    } catch (error) {
      errorMessage.value = getApiErrorMessage(error, 'Não foi possível salvar.')
    } finally {
      savingPersona.value = false
    }
  }

  function startPolling() {
    stopPolling()
    pollTimer = setInterval(() => {
      void load().then(() => {
        if (connected.value) {
          qr.value = ''
          stopPolling()
        }
      })
    }, POLL_INTERVAL_MS)
  }

  function stopPolling() {
    if (pollTimer) {
      clearInterval(pollTimer)
      pollTimer = null
    }
  }

  onBeforeUnmount(stopPolling)

  return {
    overview,
    qr,
    loading,
    connecting,
    savingEnabled,
    errorMessage,
    connected,
    whatsappStatus,
    connectedPhone,
    enabled,
    load,
    connect,
    disconnect,
    setEnabled,
    personaName,
    personaPrompt,
    personaLoading,
    savingPersona,
    personaSavedAt,
    loadPersona,
    savePersona,
  }
}
