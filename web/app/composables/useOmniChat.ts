import { createApiRequest, getApiErrorMessage } from '~/utils/api-client'

// Composable do "Omni Chat" interno do painel de Operacao (MVP M0).
// Liga o bloco visual de OperationSidePanel.vue ao endpoint Go
// POST /v1/omni-chat/ask. Contrato congelado (docs/automation/OMNI_CHAT_PLAN.md
// secao 3.1): request { question, topic? } -> response { answer, topic? }.
// storeId/accountId NAO vao no body: o header X-Account-Id e injetado
// automaticamente pelo api-client a partir do account ativo.

export interface OmniChatProduct {
  name: string
  code?: string
  price?: number
  brand?: string
  image?: string
}

export interface ChatMessage {
  id: string
  role: 'user' | 'assistant'
  text: string
  topic?: string
  products?: OmniChatProduct[]
}

interface OmniChatAskResponse {
  answer?: string
  topic?: string
  products?: OmniChatProduct[]
}

const QUESTION_MAX_LENGTH = 2000

function newMessageId(): string {
  return crypto.randomUUID()
}

function isAbortError(error: unknown): boolean {
  return error instanceof DOMException && error.name === 'AbortError'
}

const PRICE_FORMATTER = new Intl.NumberFormat('pt-BR', { style: 'currency', currency: 'BRL' })

// Formata o preco (reais) como moeda pt-BR. Vazio quando 0/invalido — o catalogo da
// Perola tem itens sem preco no ERP; nesse caso o card simplesmente nao mostra preco.
function formatPrice(value?: number): string {
  if (typeof value !== 'number' || !Number.isFinite(value) || value <= 0) {
    return ''
  }
  return PRICE_FORMATTER.format(value)
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
  // ID da conversa atual: vai no /ask e escopa a memoria do n8n (o back combina
  // com account+user). "Nova conversa" gera outro id => contexto zerado.
  const conversationId = ref(newMessageId())

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
          ? {
              question: trimmedQuestion,
              topic: resolvedTopic,
              conversationId: conversationId.value,
            }
          : { question: trimmedQuestion, conversationId: conversationId.value },
        signal: controller.signal,
      })) as OmniChatAskResponse

      const answer = String(response?.answer || '').trim()
      const products = Array.isArray(response?.products) ? response.products : []
      messages.value.push({
        id: newMessageId(),
        role: 'assistant',
        text: answer || 'O Omni nao retornou uma resposta. Tente reformular a pergunta.',
        topic: String(response?.topic || resolvedTopic || '').trim() || undefined,
        products: products.length ? products : undefined,
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

  // Inicia uma conversa nova: limpa o histórico visível e gera um conversationId
  // novo, zerando a memória do n8n (que é keyed por esse id).
  function newConversation() {
    inflightController?.abort()
    inflightController = null
    messages.value = []
    draft.value = ''
    errorMessage.value = ''
    activeTopic.value = ''
    sending.value = false
    conversationId.value = newMessageId()
  }

  // Resolve a URL da imagem do produto. O catalogo devolve um path relativo
  // (/uploads/...); o front prefixa com o apiBase para o <img>. URL absoluta passa
  // direto. A api serve /uploads/* (mesma base das chamadas de API).
  function mediaUrl(path?: string): string {
    const raw = String(path || '').trim()
    if (!raw) {
      return ''
    }
    if (/^https?:\/\//i.test(raw)) {
      return raw
    }
    const base = String(runtimeConfig.public.apiBase || '').replace(/\/+$/, '')
    return base + (raw.startsWith('/') ? raw : `/${raw}`)
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
    conversationId,
    sendQuestion,
    send,
    newConversation,
    selectTopic,
    mediaUrl,
    formatPrice,
  }
}
