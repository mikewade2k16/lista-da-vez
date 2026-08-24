import { computed, onScopeDispose, ref, watch } from 'vue'

import { useAuthStore } from '~/stores/auth'
import { createApiRequest, getApiErrorMessage } from '~/utils/api-client'

// Configuracao compartilhada do Assistente Omni. Separada de useOmniChat
// (que cuida da conversa) porque editar prompt, modelo e capacidades por
// superficie e uma responsabilidade distinta da troca de mensagens.
//
// Contrato congelado do backend:
//   GET/PUT /v1/omni-chat/config -> configuracao efetiva da conta, incluindo
//   surfaceModules. Respostas antigas ou parciais recebem defaults seguros.
// historyWindow = quantas interacoes (pergunta+resposta) o n8n mantem na memoria
// da conversa. O header X-Account-Id e injetado automaticamente pelo api-client
// (igual ao /v1/omni-chat/ask); accountId NUNCA vai no body.

export const OMNI_ASSISTANT_SURFACES = ['calendar', 'meta_ads', 'global'] as const
export type OmniAssistantSurface = (typeof OMNI_ASSISTANT_SURFACES)[number]

export const OMNI_ASSISTANT_MODULES = ['calendar', 'tasks', 'meta_ads', 'users'] as const
export type OmniAssistantModule = (typeof OMNI_ASSISTANT_MODULES)[number]

export const OMNI_ASSISTANT_ACCESS_MODES = ['off', 'read', 'write'] as const
export type OmniAssistantAccessMode = (typeof OMNI_ASSISTANT_ACCESS_MODES)[number]

export type OmniAssistantSurfaceModules = Record<
  OmniAssistantSurface,
  Record<OmniAssistantModule, OmniAssistantAccessMode>
>

export const OMNI_ASSISTANT_SURFACE_MODULE_DEFAULTS = {
  calendar: { calendar: 'write', tasks: 'write', meta_ads: 'off', users: 'read' },
  meta_ads: { calendar: 'off', tasks: 'off', meta_ads: 'write', users: 'off' },
  global: { calendar: 'read', tasks: 'read', meta_ads: 'read', users: 'read' },
} as const satisfies OmniAssistantSurfaceModules

export interface OmniChatPersona {
  enabled: boolean
  systemPrompt: string
  isDefault: boolean
  inherited: boolean
  inheritedFrom: string
  credentialId: string
  provider: 'openai' | 'anthropic' | 'gemini' | 'glm'
  model: string
  temperature: number
  historyWindow: number
  surfaceModules: OmniAssistantSurfaceModules
}

interface OmniChatPersonaResponse {
  enabled?: boolean
  systemPrompt?: string
  isDefault?: boolean
  inherited?: boolean
  inheritedFrom?: string
  credentialId?: string
  provider?: string
  model?: string
  temperature?: number
  historyWindow?: number
  surfaceModules?: unknown
}

// Limite alinhado ao backend (prompt_too_long acima de 20000 chars). Validar no
// front evita uma ida ao servidor so para descobrir que passou do limite.
export const PERSONA_MAX_LENGTH = 20000

// Janela de memoria: default e faixa alinhados ao backend (clamp 1..20).
export const HISTORY_WINDOW_DEFAULT = 5
export const HISTORY_WINDOW_MIN = 1
export const HISTORY_WINDOW_MAX = 20

function clampWindow(n: number): number {
  if (!Number.isFinite(n) || n <= 0) {
    return HISTORY_WINDOW_DEFAULT
  }
  return Math.min(HISTORY_WINDOW_MAX, Math.max(HISTORY_WINDOW_MIN, Math.round(n)))
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null && !Array.isArray(value)
}

/**
 * Mantem a matriz completa mesmo ao ler configuracoes legadas, parciais ou
 * adulteradas. Chaves desconhecidas sao descartadas e modos invalidos voltam
 * ao default da respectiva superficie/modulo.
 */
export function normalizeOmniAssistantSurfaceModules(value: unknown): OmniAssistantSurfaceModules {
  const source = isRecord(value) ? value : {}
  const normalized = {} as OmniAssistantSurfaceModules

  for (const surface of OMNI_ASSISTANT_SURFACES) {
    const configuredModules = isRecord(source[surface]) ? source[surface] : {}
    const modules = {} as Record<OmniAssistantModule, OmniAssistantAccessMode>

    for (const module of OMNI_ASSISTANT_MODULES) {
      const requestedMode = String(configuredModules[module] || '')
        .trim()
        .toLowerCase()
      modules[module] = OMNI_ASSISTANT_ACCESS_MODES.includes(
        requestedMode as OmniAssistantAccessMode,
      )
        ? (requestedMode as OmniAssistantAccessMode)
        : OMNI_ASSISTANT_SURFACE_MODULE_DEFAULTS[surface][module]
    }

    normalized[surface] = modules
  }

  return normalized
}

function normalizePersona(response: OmniChatPersonaResponse | null): OmniChatPersona {
  const provider = ['openai', 'anthropic', 'gemini', 'glm'].includes(String(response?.provider))
    ? (String(response?.provider) as OmniChatPersona['provider'])
    : 'openai'
  return {
    enabled: response?.enabled !== false,
    systemPrompt: String(response?.systemPrompt || ''),
    isDefault: Boolean(response?.isDefault),
    inherited: response?.inherited === true,
    inheritedFrom: String(response?.inheritedFrom || '').trim(),
    credentialId: String(response?.credentialId || ''),
    provider,
    model: String(response?.model || 'gpt-4.1-mini'),
    temperature: Math.min(1, Math.max(0, Number(response?.temperature) || 0.2)),
    historyWindow: clampWindow(Number(response?.historyWindow)),
    surfaceModules: normalizeOmniAssistantSurfaceModules(response?.surfaceModules),
  }
}

export function useOmniChatPersona(accountId?: () => string) {
  const runtimeConfig = useRuntimeConfig()
  const auth = useAuthStore()
  const apiRequest = createApiRequest(runtimeConfig, () => auth.accessToken)

  function currentAccountId(): string {
    return String(accountId?.() || '').trim()
  }

  function accountHeaders(value: string): Record<string, string> {
    return { 'X-Account-Id': value }
  }

  // Texto em edicao no <textarea>. Re-hidratado da resposta do back a cada
  // fetchPersona; so persiste enquanto ha edicao do usuario.
  const draft = ref('')
  const enabled = ref(false)
  // Reflete o estado de fabrica do prompt vigente (mostra o aviso "usando o
  // texto padrao"). Vem do banco; nunca cravado no front.
  const isDefault = ref(true)
  const inherited = ref(false)
  const inheritedFrom = ref('')
  const credentialId = ref('')
  const provider = ref<OmniChatPersona['provider']>('openai')
  const model = ref('')
  const temperature = ref(0.2)
  // Janela de memoria em edicao (interacoes que o n8n mantem). Re-hidratada do back.
  const historyWindow = ref(HISTORY_WINDOW_DEFAULT)
  const surfaceModules = ref<OmniAssistantSurfaceModules>(
    normalizeOmniAssistantSurfaceModules(null),
  )
  const loading = ref(false)
  const saving = ref(false)
  const errorMessage = ref('')
  // Mensagem de sucesso efemera ("Persona salva.") apos um PUT bem-sucedido.
  const successMessage = ref('')
  const loadedAccountId = ref('')
  const ready = computed(
    () => Boolean(loadedAccountId.value) && loadedAccountId.value === currentAccountId(),
  )

  // Controllers + geracao pertencem ao accountId capturado no inicio da operacao.
  // Trocar de conta aborta GET/PUT, limpa os drafts no mesmo tick e invalida
  // qualquer resposta que o transporte nao tenha conseguido interromper.
  let fetchController: AbortController | null = null
  let saveController: AbortController | null = null
  let contextGeneration = 0
  let contextAccountId = '\u0000'

  function isAbortError(error: unknown): boolean {
    return (
      (error instanceof DOMException && error.name === 'AbortError') ||
      (error as { name?: string } | null)?.name === 'AbortError'
    )
  }

  function snapshot(): OmniChatPersona {
    return {
      enabled: enabled.value,
      systemPrompt: draft.value,
      isDefault: isDefault.value,
      inherited: inherited.value,
      inheritedFrom: inheritedFrom.value,
      credentialId: credentialId.value,
      provider: provider.value,
      model: model.value,
      temperature: temperature.value,
      historyWindow: historyWindow.value,
      surfaceModules: normalizeOmniAssistantSurfaceModules(surfaceModules.value),
    }
  }

  function applyPersona(persona: OmniChatPersona): void {
    enabled.value = persona.enabled
    draft.value = persona.systemPrompt
    isDefault.value = persona.isDefault
    inherited.value = persona.inherited
    inheritedFrom.value = persona.inheritedFrom
    credentialId.value = persona.credentialId
    provider.value = persona.provider
    model.value = persona.model
    temperature.value = persona.temperature
    historyWindow.value = persona.historyWindow
    surfaceModules.value = persona.surfaceModules
  }

  function clearAccountState(): void {
    draft.value = ''
    enabled.value = false
    isDefault.value = true
    inherited.value = false
    inheritedFrom.value = ''
    credentialId.value = ''
    provider.value = 'openai'
    model.value = ''
    temperature.value = 0.2
    historyWindow.value = HISTORY_WINDOW_DEFAULT
    surfaceModules.value = normalizeOmniAssistantSurfaceModules(null)
    loading.value = false
    saving.value = false
    errorMessage.value = ''
    successMessage.value = ''
    loadedAccountId.value = ''
  }

  function isCurrentContext(account: string, generation: number): boolean {
    return (
      Boolean(account) &&
      generation === contextGeneration &&
      account === contextAccountId &&
      account === currentAccountId()
    )
  }

  function switchAccount(nextAccountId: string): void {
    if (nextAccountId === contextAccountId) return
    contextAccountId = nextAccountId
    contextGeneration += 1
    fetchController?.abort()
    saveController?.abort()
    fetchController = null
    saveController = null
    clearAccountState()
  }

  watch(currentAccountId, switchAccount, { immediate: true, flush: 'sync' })

  async function fetchPersona(): Promise<OmniChatPersona> {
    const requestAccountId = currentAccountId()
    if (!requestAccountId) return snapshot()

    fetchController?.abort()
    const controller = new AbortController()
    fetchController = controller
    const generation = contextGeneration

    loading.value = true
    errorMessage.value = ''
    successMessage.value = ''

    try {
      const response = (await apiRequest('/v1/omni-chat/config', {
        method: 'GET',
        headers: accountHeaders(requestAccountId),
        signal: controller.signal,
        dedupe: false,
      })) as OmniChatPersonaResponse

      if (fetchController !== controller || !isCurrentContext(requestAccountId, generation)) {
        return snapshot()
      }
      const persona = normalizePersona(response)
      applyPersona(persona)
      loadedAccountId.value = requestAccountId
      return persona
    } catch (error) {
      if (
        isAbortError(error) ||
        fetchController !== controller ||
        !isCurrentContext(requestAccountId, generation)
      ) {
        return snapshot()
      }
      errorMessage.value = getApiErrorMessage(
        error,
        'Nao foi possivel carregar a persona do Omni. Tente novamente.',
      )
      throw error
    } finally {
      if (fetchController === controller) {
        fetchController = null
        if (isCurrentContext(requestAccountId, generation)) loading.value = false
      }
    }
  }

  async function savePersona(systemPrompt: string): Promise<void> {
    const requestAccountId = currentAccountId()
    const trimmed = String(systemPrompt || '').trim()
    if (trimmed.length > PERSONA_MAX_LENGTH) {
      errorMessage.value = `O prompt passou de ${PERSONA_MAX_LENGTH} caracteres. Resuma e tente de novo.`
      return
    }
    if (!ready.value || !requestAccountId || saving.value) return

    const generation = contextGeneration
    const controller = new AbortController()
    saveController = controller
    saving.value = true
    errorMessage.value = ''
    successMessage.value = ''

    try {
      const response = (await apiRequest('/v1/omni-chat/config', {
        method: 'PUT',
        headers: accountHeaders(requestAccountId),
        signal: controller.signal,
        body: {
          enabled: enabled.value,
          systemPrompt: trimmed,
          credentialId: credentialId.value,
          model: model.value.trim(),
          temperature: Math.min(1, Math.max(0, Number(temperature.value) || 0)),
          historyWindow: clampWindow(historyWindow.value),
          surfaceModules: normalizeOmniAssistantSurfaceModules(surfaceModules.value),
        },
      })) as OmniChatPersonaResponse

      if (!isCurrentContext(requestAccountId, generation)) return
      // Re-hidrata do que o back devolveu (fonte de verdade), nao do que
      // mandamos: o servidor pode normalizar o texto e a janela (clamp).
      const persona = normalizePersona(response)
      applyPersona(persona)
      loadedAccountId.value = requestAccountId
      successMessage.value = 'Configuração salva para toda a conta.'
    } catch (error) {
      if (isAbortError(error) || !isCurrentContext(requestAccountId, generation)) return
      errorMessage.value = getApiErrorMessage(
        error,
        'Nao foi possivel salvar a persona do Omni. Tente novamente.',
      )
    } finally {
      if (saveController === controller) {
        saveController = null
        if (isCurrentContext(requestAccountId, generation)) saving.value = false
      }
    }
  }

  function resetFeedback() {
    errorMessage.value = ''
    successMessage.value = ''
  }

  onScopeDispose(() => {
    fetchController?.abort()
    saveController?.abort()
    fetchController = null
    saveController = null
  })

  return {
    draft,
    enabled,
    isDefault,
    inherited,
    inheritedFrom,
    credentialId,
    provider,
    model,
    temperature,
    historyWindow,
    surfaceModules,
    loading,
    saving,
    errorMessage,
    successMessage,
    loadedAccountId,
    ready,
    fetchPersona,
    savePersona,
    resetFeedback,
  }
}
