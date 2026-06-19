import { createApiRequest, getApiErrorMessage } from '~/utils/api-client'

// Composable do "Omni Chat" interno do painel de Operacao (MVP M0).
// Liga o bloco visual de OperationSidePanel.vue ao endpoint Go
// POST /v1/omni-chat/ask. Contrato congelado (docs/automation/OMNI_CHAT_PLAN.md
// secao 3.1): request { question, topic? } -> response { answer, topic? }.
// storeId/accountId NAO vao no body: o header X-Account-Id e injetado
// automaticamente pelo api-client a partir do account ativo.

export interface ChatMessage {
  id: string
  role: 'user' | 'assistant'
  text: string
  topic?: string
}

interface OmniChatAskResponse {
  answer?: string
  topic?: string
}

const QUESTION_MAX_LENGTH = 2000

function newMessageId(): string {
  return crypto.randomUUID()
}

function isAbortError(error: unknown): boolean {
  return error instanceof DOMException && error.name === 'AbortError'
}

export function useOmniChat() {
  const runtimeConfig = useRuntimeConfig()
  const auth = useAuthStore()
  const apiRequest = createApiRequest(runtimeConfig, () => auth.accessToken)

  const messages = ref<ChatMessage[]>([])
  const draft = ref('')
  const activeTopic = ref('')
  const sending = ref(false)
  const errorMessage = ref('')

  // Cancela a pergunta anterior ainda em voo quando o usuario dispara outra.
  let inflightController: AbortController | null = null

  function selectTopic(topic: string) {
    const normalized = String(topic || '').trim()
    // Clicar de novo no chip ativo desmarca o filtro de topico.
    activeTopic.value = activeTopic.value === normalized ? '' : normalized
  }

  async function sendQuestion(question: string, topic?: string) {
    const trimmedQuestion = String(question || '').trim()
    if (!trimmedQuestion || sending.value) {
      return
    }
    if (trimmedQuestion.length > QUESTION_MAX_LENGTH) {
      errorMessage.value = `A pergunta passou de ${QUESTION_MAX_LENGTH} caracteres. Resuma e tente de novo.`
      return
    }

    const resolvedTopic = String(topic ?? activeTopic.value ?? '').trim()

    // Cancela qualquer pergunta anterior ainda pendente.
    inflightController?.abort()
    const controller = new AbortController()
    inflightController = controller

    errorMessage.value = ''
    sending.value = true
    messages.value.push({
      id: newMessageId(),
      role: 'user',
      text: trimmedQuestion,
      topic: resolvedTopic || undefined,
    })

    try {
      const response = (await apiRequest('/v1/omni-chat/ask', {
        method: 'POST',
        body: resolvedTopic
          ? { question: trimmedQuestion, topic: resolvedTopic }
          : { question: trimmedQuestion },
        signal: controller.signal,
      })) as OmniChatAskResponse

      const answer = String(response?.answer || '').trim()
      messages.value.push({
        id: newMessageId(),
        role: 'assistant',
        text: answer || 'O Omni nao retornou uma resposta. Tente reformular a pergunta.',
        topic: String(response?.topic || resolvedTopic || '').trim() || undefined,
      })
    } catch (error) {
      // Pergunta cancelada de proposito (usuario enviou outra) nao vira erro.
      if (isAbortError(error)) {
        return
      }
      errorMessage.value = getApiErrorMessage(
        error,
        'Nao foi possivel falar com o Omni agora. Tente novamente em instantes.',
      )
    } finally {
      // So libera o estado se este controller ainda for o vigente: uma pergunta
      // mais nova ja pode ter assumido o sending.
      if (inflightController === controller) {
        inflightController = null
        sending.value = false
      }
    }
  }

  function send() {
    const question = draft.value.trim()
    if (!question) {
      return
    }
    draft.value = ''
    void sendQuestion(question, activeTopic.value)
  }

  onBeforeUnmount(() => {
    inflightController?.abort()
    inflightController = null
  })

  return {
    messages,
    draft,
    activeTopic,
    sending,
    errorMessage,
    sendQuestion,
    send,
    selectTopic,
  }
}
