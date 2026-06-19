import { createApiRequest, getApiErrorMessage } from '~/utils/api-client'

// Composable da PERSONA (prompt/sistema) do Omni Chat. Separado de useOmniChat
// (que cuida da conversa) porque a edicao do prompt e uma responsabilidade
// distinta: ler/gravar o systemPrompt efetivo no banco via o modulo Go.
//
// Contrato congelado do backend:
//   GET  /v1/omni-chat/persona -> { systemPrompt: string; isDefault: boolean; historyWindow: number }
//   PUT  /v1/omni-chat/persona  body { systemPrompt: string; historyWindow: number }
//        -> { systemPrompt: string; isDefault: false; historyWindow: number }
//        erros: 400 empty_prompt (vazio), 400 prompt_too_long (>20000 chars)
// historyWindow = quantas interacoes (pergunta+resposta) o n8n mantem na memoria
// da conversa. O header X-Account-Id e injetado automaticamente pelo api-client
// (igual ao /v1/omni-chat/ask); accountId NUNCA vai no body.

export interface OmniChatPersona {
  systemPrompt: string
  isDefault: boolean
  historyWindow: number
}

interface OmniChatPersonaResponse {
  systemPrompt?: string
  isDefault?: boolean
  historyWindow?: number
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

function normalizePersona(response: OmniChatPersonaResponse | null): OmniChatPersona {
  return {
    systemPrompt: String(response?.systemPrompt || ''),
    isDefault: Boolean(response?.isDefault),
    historyWindow: clampWindow(Number(response?.historyWindow)),
  }
}

export function useOmniChatPersona() {
  const runtimeConfig = useRuntimeConfig()
  const auth = useAuthStore()
  const apiRequest = createApiRequest(runtimeConfig, () => auth.accessToken)

  // Texto em edicao no <textarea>. Re-hidratado da resposta do back a cada
  // fetchPersona; so persiste enquanto ha edicao do usuario.
  const draft = ref('')
  // Reflete o estado de fabrica do prompt vigente (mostra o aviso "usando o
  // texto padrao"). Vem do banco; nunca cravado no front.
  const isDefault = ref(true)
  // Janela de memoria em edicao (interacoes que o n8n mantem). Re-hidratada do back.
  const historyWindow = ref(HISTORY_WINDOW_DEFAULT)
  const loading = ref(false)
  const saving = ref(false)
  const errorMessage = ref('')
  // Mensagem de sucesso efemera ("Persona salva.") apos um PUT bem-sucedido.
  const successMessage = ref('')

  // Cancela o GET anterior ainda em voo quando o editor reabre antes de carregar.
  let inflightController: AbortController | null = null

  function isAbortError(error: unknown): boolean {
    return error instanceof DOMException && error.name === 'AbortError'
  }

  async function fetchPersona(): Promise<OmniChatPersona> {
    inflightController?.abort()
    const controller = new AbortController()
    inflightController = controller

    loading.value = true
    errorMessage.value = ''
    successMessage.value = ''

    try {
      const response = (await apiRequest('/v1/omni-chat/persona', {
        method: 'GET',
        signal: controller.signal,
        dedupe: false,
      })) as OmniChatPersonaResponse

      const persona = normalizePersona(response)
      draft.value = persona.systemPrompt
      isDefault.value = persona.isDefault
      historyWindow.value = persona.historyWindow
      return persona
    } catch (error) {
      if (isAbortError(error)) {
        return {
          systemPrompt: draft.value,
          isDefault: isDefault.value,
          historyWindow: historyWindow.value,
        }
      }
      errorMessage.value = getApiErrorMessage(
        error,
        'Nao foi possivel carregar a persona do Omni. Tente novamente.',
      )
      throw error
    } finally {
      if (inflightController === controller) {
        inflightController = null
        loading.value = false
      }
    }
  }

  async function savePersona(systemPrompt: string): Promise<void> {
    const trimmed = String(systemPrompt || '').trim()
    if (!trimmed) {
      errorMessage.value = 'O prompt nao pode ficar vazio.'
      return
    }
    if (trimmed.length > PERSONA_MAX_LENGTH) {
      errorMessage.value = `O prompt passou de ${PERSONA_MAX_LENGTH} caracteres. Resuma e tente de novo.`
      return
    }
    if (saving.value) {
      return
    }

    saving.value = true
    errorMessage.value = ''
    successMessage.value = ''

    try {
      const response = (await apiRequest('/v1/omni-chat/persona', {
        method: 'PUT',
        body: { systemPrompt: trimmed, historyWindow: clampWindow(historyWindow.value) },
      })) as OmniChatPersonaResponse

      // Re-hidrata do que o back devolveu (fonte de verdade), nao do que
      // mandamos: o servidor pode normalizar o texto e a janela (clamp).
      const persona = normalizePersona(response)
      draft.value = persona.systemPrompt
      isDefault.value = persona.isDefault
      historyWindow.value = persona.historyWindow
      successMessage.value = 'Persona salva.'
    } catch (error) {
      errorMessage.value = getApiErrorMessage(
        error,
        'Nao foi possivel salvar a persona do Omni. Tente novamente.',
      )
    } finally {
      saving.value = false
    }
  }

  function resetFeedback() {
    errorMessage.value = ''
    successMessage.value = ''
  }

  onBeforeUnmount(() => {
    inflightController?.abort()
    inflightController = null
  })

  return {
    draft,
    isDefault,
    historyWindow,
    loading,
    saving,
    errorMessage,
    successMessage,
    fetchPersona,
    savePersona,
    resetFeedback,
  }
}
