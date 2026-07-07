import { createApiRequest, getApiErrorMessage } from '~/utils/api-client'
import { useAuthStore } from '~/stores/auth'
import { useCalendarStore } from '~/stores/calendar'
import type { CalendarEventInput } from '~/utils/calendar'
// Store de Tasks vive em outra layer; import cross-layer (precedente: ConfigTasks.vue). So
// e usado ao CONFIRMAR uma proposta de task (WAVE 5, E7); a Pinia instancia sob demanda.
import { useTasksStore } from '../../layers/tasks/stores/tasks'
import {
  deleteConversation as apiDeleteConversation,
  fetchChatScope,
  fetchConversations,
  getConversation,
  type CalendarChatConversation,
  type CalendarChatScope,
  type CalendarChatScopeMode,
} from '~/domain/calendar/calendar-chat-api'

// Chat flutuante do Calendario (SPEC-F7/F10, contrato C7/D3/D4). Liga o painel
// CalendarChatPanel.vue aos endpoints Go de chat do calendario. A partir da wave 4 as
// conversas e mensagens sao PERSISTIDAS no banco (calendar.chat_conversations/
// chat_messages): a lista de conversas e o historico vem do banco (nao somem no reload)
// e a IA tem MEMORIA (o back carrega as ultimas N mensagens da conversa). O escopo
// (client|all) viaja no /ask e fica salvo na conversa; account_id NAO viaja (o back
// resolve pelo Principal) e o ACESSO e resolvido server-side pela permissao (agencia ve
// todas as conversas; cliente-side so as suas; fora do visivel => 404).
//
// Estado SINGLETON via useState: o FAB flutuante, o painel e o botao "Abrir chat" da
// aba IA do drawer de config (F6) mexem no MESMO estado (mesma conversa, historico,
// lista e escopo), sem prop drilling nem provide/inject.

export interface CalendarChatMessage {
  id: string
  role: 'user' | 'assistant'
  text: string
}

// WAVE 5 (E7): proposta de criacao devolvida pela IA. A IA NAO cria nada; o painel mostra um
// cartao de confirmacao e, se o usuario confirmar, cria pela API autenticada dele (store).
export interface CalendarChatProposal {
  kind: 'event' | 'task'
  fields: {
    title?: string
    date?: string
    time?: string
    type?: string
    status?: string
    dueDate?: string
    columnId?: string
    clientId?: string
  }
}

interface CalendarChatAskResponse {
  answer?: string
  conversationId?: string
  title?: string
  proposal?: CalendarChatProposal | null
  // aiError (WAVE 5) = a IA nao respondeu (503/cota/chave/vazio). O front mostra o estado
  // "IA fora do ar" (visual distinto), nao um balao normal, e nao persiste a mensagem.
  aiError?: boolean
}

const QUESTION_MAX_LENGTH = 4000

// Escopo default antes de o back responder o /chat/scope (cliente-side por seguranca:
// so libera o select quando o back confirma canSelect=true).
const DEFAULT_SCOPE: CalendarChatScope = { canSelect: false, lockedClientId: '', clients: [] }

// AbortController fica fora do estado reativo (singleton do modulo): cancela a pergunta
// anterior ainda em voo quando o usuario dispara outra. NAO abortamos no unmount de
// componente porque o estado e compartilhado por varios (FAB/painel/drawer).
let inflightController: AbortController | null = null

function newId(): string {
  return crypto.randomUUID()
}

function isAbortError(error: unknown): boolean {
  return error instanceof DOMException && error.name === 'AbortError'
}

function httpStatus(error: unknown): number {
  const e = error as { status?: number; statusCode?: number; response?: { status?: number } }
  return Number(e?.status ?? e?.statusCode ?? e?.response?.status ?? 0)
}

function errorCode(error: unknown): string {
  const e = error as { data?: { error?: { code?: string } } }
  return String(e?.data?.error?.code || '')
}

export function useCalendarChat() {
  const runtimeConfig = useRuntimeConfig()
  const auth = useAuthStore()
  const store = useCalendarStore()
  const apiRequest = createApiRequest(runtimeConfig, () => auth.accessToken)

  // Chaves unicas do useState = o singleton. Qualquer componente que chamar
  // useCalendarChat() recebe as MESMAS refs.
  const messages = useState<CalendarChatMessage[]>('calendar-chat:messages', () => [])
  const draft = useState<string>('calendar-chat:draft', () => '')
  const sending = useState<boolean>('calendar-chat:sending', () => false)
  const errorMessage = useState<string>('calendar-chat:error', () => '')
  const panelOpen = useState<boolean>('calendar-chat:open', () => false)
  // Minimizado: a janela colapsa numa pill re-expansivel SEM perder o estado.
  const minimized = useState<boolean>('calendar-chat:minimized', () => false)

  // Conversa ativa. Vazio = conversa NOVA ainda nao persistida: o back a cria no
  // primeiro /ask e devolve o id/titulo reais, que adotamos aqui.
  const conversationId = useState<string>('calendar-chat:conversation', () => '')
  const conversationTitle = useState<string>('calendar-chat:title', () => '')

  // WAVE 5 (E7): proposta pendente de criacao (evento/task) sugerida pela IA. O painel
  // renderiza um cartao de confirmacao; confirmar cria pela API do proprio usuario.
  const pendingProposal = useState<CalendarChatProposal | null>(
    'calendar-chat:proposal',
    () => null,
  )
  const creatingProposal = useState<boolean>('calendar-chat:proposal-busy', () => false)

  // WAVE 5: estado "IA fora do ar" — true quando a ULTIMA tentativa falhou (503/cota/chave/
  // timeout/kill switch). O painel mostra um indicador no cabecalho + um bloco distinto no
  // chat (nunca um balao normal), para o usuario SEMPRE saber que a IA nao esta funcionando.
  const aiOffline = useState<boolean>('calendar-chat:ai-offline', () => false)
  const aiOfflineReason = useState<string>('calendar-chat:ai-offline-reason', () => '')

  // Lista de conversas persistidas (menu "Conversas") + estados de carga.
  const conversations = useState<CalendarChatConversation[]>('calendar-chat:list', () => [])
  const loadingConversations = useState<boolean>('calendar-chat:list-loading', () => false)
  const loadingConversation = useState<boolean>('calendar-chat:conv-loading', () => false)

  // Escopo do contexto (D3/D4). scopeMode/scopeClientId viajam no /ask e ficam salvos
  // na conversa; chatScope (do back) diz se o select aparece e trava o cliente unico.
  const chatScope = useState<CalendarChatScope>('calendar-chat:scope', () => ({ ...DEFAULT_SCOPE }))
  const scopeMode = useState<CalendarChatScopeMode>('calendar-chat:scope-mode', () => 'all')
  const scopeClientId = useState<string>('calendar-chat:scope-client', () => '')

  // Define o escopo default de uma conversa NOVA (nao mexe numa conversa ja aberta, que
  // carrega o proprio escopo salvo). Cliente-side (canSelect=false) trava no unico
  // cliente visivel; agencia usa o cliente filtrado na tela (se visivel) ou "todos".
  function applyScopeDefault(scope: CalendarChatScope): void {
    if (conversationId.value) return
    if (!scope.canSelect && scope.lockedClientId) {
      scopeMode.value = 'client'
      scopeClientId.value = scope.lockedClientId
      return
    }
    const filtered = store.selectedClientId
    if (filtered && scope.clients.some((c) => c.id === filtered)) {
      scopeMode.value = 'client'
      scopeClientId.value = filtered
    } else {
      scopeMode.value = 'all'
      scopeClientId.value = ''
    }
  }

  // Troca o escopo escolhido (usado pelo SELECT do painel, SPEC-F11).
  function setScope(mode: CalendarChatScopeMode, clientId: string): void {
    scopeMode.value = mode === 'client' ? 'client' : 'all'
    scopeClientId.value = mode === 'client' ? clientId : ''
  }

  async function loadConversations(): Promise<void> {
    loadingConversations.value = true
    try {
      conversations.value = await fetchConversations(apiRequest)
    } catch {
      // silencioso: mantem a lista anterior (nao trava o chat)
    } finally {
      loadingConversations.value = false
    }
  }

  async function loadScope(): Promise<void> {
    try {
      const scope = await fetchChatScope(apiRequest)
      chatScope.value = scope
      applyScopeDefault(scope)
    } catch {
      // sem escopo do back: mantem o atual
    }
  }

  // Chamado ao abrir o chat: busca a lista de conversas + o escopo do banco.
  async function ensureChatLoaded(): Promise<void> {
    await auth.ensureSession()
    if (!auth.isAuthenticated) return
    await Promise.all([loadConversations(), loadScope()])
  }

  function openPanel(): void {
    // Abrir/reabrir sempre traz a janela cheia (nunca a pill) e recarrega a lista +
    // escopo do banco (reflete conversas novas e troca de conta).
    panelOpen.value = true
    minimized.value = false
    void ensureChatLoaded()
  }

  function closePanel(): void {
    panelOpen.value = false
    minimized.value = false
  }

  function togglePanel(): void {
    if (panelOpen.value) closePanel()
    else openPanel()
  }

  function minimize(): void {
    minimized.value = true
  }

  function restore(): void {
    minimized.value = false
  }

  // Traduz o erro do back em mensagem ACIONAVEL (o que fazer, nao so o que falhou).
  function actionableError(error: unknown): string {
    const status = httpStatus(error)
    const code = errorCode(error)
    if (status === 503 || code === 'chat_not_configured') {
      return 'O chat do calendario ainda nao esta configurado. Defina o env CALENDAR_CHAT_WEBHOOK_URL e importe o workflow "Calendar Chat" no n8n.'
    }
    if (status === 502 || status === 504) {
      return 'A IA do calendario nao respondeu agora (servico indisponivel). Tente novamente em instantes.'
    }
    if (status === 400) {
      return getApiErrorMessage(error, 'Pergunta invalida. Revise e tente de novo.')
    }
    return getApiErrorMessage(
      error,
      'Nao foi possivel falar com a IA do calendario agora. Tente novamente em instantes.',
    )
  }

  // Atualiza o resumo da conversa na lista apos uma resposta. Conversa ja na lista:
  // atualiza titulo/updatedAt e sobe pro topo (mais recente). Conversa nova (recem
  // criada pelo /ask): recarrega a lista do banco (para pegar autor/data corretos).
  function upsertConversationSummary(id: string, title: string): void {
    if (!id) return
    const idx = conversations.value.findIndex((c) => c.id === id)
    if (idx < 0) {
      void loadConversations()
      return
    }
    const existing = conversations.value[idx]!
    const updated: CalendarChatConversation = {
      ...existing,
      title: title || existing.title,
      updatedAt: new Date().toISOString(),
    }
    conversations.value = [updated, ...conversations.value.filter((c) => c.id !== id)]
  }

  // Envia a pergunta com o escopo escolhido + a conversa ativa. O back persiste as duas
  // mensagens (user/assistant), carrega a memoria e devolve { answer, conversationId,
  // title } — adotamos o id/titulo que ele resolveu.
  async function ask(question: string): Promise<void> {
    const trimmed = String(question || '').trim()
    if (!trimmed || sending.value) {
      return
    }
    if (trimmed.length > QUESTION_MAX_LENGTH) {
      errorMessage.value = `A pergunta passou de ${QUESTION_MAX_LENGTH} caracteres. Resuma e tente de novo.`
      return
    }

    inflightController?.abort()
    const controller = new AbortController()
    inflightController = controller

    errorMessage.value = ''
    sending.value = true
    pendingProposal.value = null // nova pergunta descarta uma proposta anterior nao confirmada
    messages.value = [...messages.value, { id: newId(), role: 'user', text: trimmed }]

    try {
      const response = (await apiRequest('/v1/calendar/chat/ask', {
        method: 'POST',
        body: {
          question: trimmed,
          conversationId: conversationId.value,
          // Escopo do contexto (D4): 'client' manda o cliente; 'all' = todos os
          // visiveis. O back valida contra a permissao (nunca confia no body).
          scopeMode: scopeMode.value,
          scopeClientId: scopeMode.value === 'client' ? scopeClientId.value : '',
          month: store.focusMonthKey || '',
        },
        signal: controller.signal,
      })) as CalendarChatAskResponse

      const newConvId = String(response?.conversationId || '').trim()
      const newTitle = String(response?.title || '').trim()
      if (newConvId) conversationId.value = newConvId
      if (newTitle) conversationTitle.value = newTitle

      // WAVE 5: a IA falhou (n8n sinalizou aiError). NAO adiciona balao normal — marca o
      // estado "IA fora do ar" (visual distinto) com o motivo. A pergunta do usuario fica.
      if (response?.aiError) {
        aiOffline.value = true
        aiOfflineReason.value =
          String(response?.answer || '').trim() || 'A IA não respondeu agora. Tente de novo.'
        pendingProposal.value = null
        return
      }

      aiOffline.value = false
      aiOfflineReason.value = ''
      const answer = String(response?.answer || '').trim()
      messages.value = [
        ...messages.value,
        {
          id: newId(),
          role: 'assistant',
          text: answer || 'A IA nao retornou uma resposta. Tente reformular a pergunta.',
        },
      ]
      // WAVE 5 (E7): a IA pode devolver uma proposta de criacao junto da resposta.
      pendingProposal.value = response?.proposal || null
      upsertConversationSummary(conversationId.value, conversationTitle.value)
    } catch (error) {
      // Pergunta cancelada de proposito (o usuario enviou outra) nao vira erro.
      if (isAbortError(error)) {
        return
      }
      // WAVE 5: qualquer falha ao falar com a IA (rede/503/timeout/back) = estado "IA fora
      // do ar" com visual distinto (nao um balao nem so a barra de erro generica).
      aiOffline.value = true
      aiOfflineReason.value = actionableError(error)
    } finally {
      // So libera o estado se este controller ainda for o vigente.
      if (inflightController === controller) {
        inflightController = null
        sending.value = false
      }
    }
  }

  // Envia o rascunho do input (Enter ou botao). Limpa o input na hora.
  function send(): void {
    const question = draft.value.trim()
    if (!question) {
      return
    }
    draft.value = ''
    void ask(question)
  }

  // Abre uma conversa persistida: carrega as mensagens do banco (SUBSTITUI as locais) e
  // adota o escopo salvo da conversa. Acesso resolvido no back (fora do visivel => 404,
  // tratado como erro acionavel).
  async function openConversation(id: string): Promise<void> {
    if (!id || loadingConversation.value) return
    inflightController?.abort()
    inflightController = null
    loadingConversation.value = true
    errorMessage.value = ''
    aiOffline.value = false
    aiOfflineReason.value = ''
    try {
      const detail = await getConversation(apiRequest, id)
      conversationId.value = detail.id
      conversationTitle.value = detail.title
      scopeMode.value = detail.scopeMode
      scopeClientId.value = detail.scopeClientId
      messages.value = detail.messages.map((m) => ({ id: m.id, role: m.role, text: m.content }))
      draft.value = ''
      sending.value = false
    } catch (error) {
      errorMessage.value = actionableError(error)
    } finally {
      loadingConversation.value = false
    }
  }

  // Inicia uma conversa nova: limpa o historico visivel e zera o id (o back cria a
  // conversa no primeiro /ask). Reaplica o escopo default (cliente-side trava; agencia
  // usa o filtro/todos).
  function newConversation(): void {
    inflightController?.abort()
    inflightController = null
    messages.value = []
    draft.value = ''
    errorMessage.value = ''
    sending.value = false
    loadingConversation.value = false
    conversationId.value = ''
    conversationTitle.value = ''
    pendingProposal.value = null
    aiOffline.value = false
    aiOfflineReason.value = ''
    applyScopeDefault(chatScope.value)
  }

  // WAVE 5 (E7): descarta a proposta sem criar nada.
  function dismissProposal(): void {
    pendingProposal.value = null
  }

  // noteAssistant empurra uma nota da IA no historico visivel (ex.: confirmacao de criacao).
  function noteAssistant(text: string): void {
    messages.value = [...messages.value, { id: newId(), role: 'assistant', text }]
  }

  // confirmProposal CRIA o evento/task proposto pela API autenticada do usuario (permissao e
  // escopo normais — o back valida). Evento -> calendar store (createTask default se ha board);
  // task -> tasks store (board = config.tasks.boardId). Sucesso => nota no chat + limpa a proposta.
  async function confirmProposal(): Promise<void> {
    const proposal = pendingProposal.value
    if (!proposal || creatingProposal.value) return
    creatingProposal.value = true
    errorMessage.value = ''
    try {
      const f = proposal.fields || {}
      const clientId = String(
        f.clientId || (scopeMode.value === 'client' ? scopeClientId.value : '') || '',
      )
      if (proposal.kind === 'task') {
        const boardId = String(store.config.tasks?.boardId || '')
        if (!boardId) {
          errorMessage.value = 'Configure um board na aba Integrações da config para criar tasks.'
          return
        }
        const tasksStore = useTasksStore()
        await tasksStore.initialize({ allowAutoCreate: false }).catch(() => undefined)
        const created = await tasksStore.createTask({
          projectId: boardId,
          title: String(f.title || ''),
          dueDate: String(f.dueDate || f.date || ''),
          clientId,
        })
        if (!created) {
          errorMessage.value = 'Não consegui criar a task agora.'
          return
        }
        noteAssistant('Task criada no board ✅')
      } else {
        // createEvent tipa os enums (type/status/priority); a proposta vem validada e o back
        // re-valida, entao usamos um cast controlado no payload.
        const ok = await store.createEvent({
          date: String(f.date || f.dueDate || ''),
          time: String(f.time || ''),
          clientId,
          type: String(f.type || 'post'),
          title: String(f.title || ''),
          status: String(f.status || 'planejado'),
          priority: 'media',
          responsibleId: '',
          involvedIds: [],
          media: [],
          description: '',
          createTask: Boolean(store.config.tasks?.boardId),
        } as unknown as CalendarEventInput)
        if (!ok) {
          errorMessage.value = 'Não consegui criar o evento (confira a data proposta).'
          return
        }
        noteAssistant('Evento criado no calendário ✅')
      }
      pendingProposal.value = null
    } catch (error) {
      errorMessage.value = actionableError(error)
    } finally {
      creatingProposal.value = false
    }
  }

  // Apaga (soft-delete) uma conversa. Se era a ativa, comeca uma nova.
  async function removeConversation(id: string): Promise<void> {
    if (!id) return
    try {
      await apiDeleteConversation(apiRequest, id)
    } catch {
      return
    }
    conversations.value = conversations.value.filter((c) => c.id !== id)
    if (conversationId.value === id) newConversation()
  }

  return {
    messages,
    draft,
    sending,
    errorMessage,
    panelOpen,
    minimized,
    conversationId,
    conversationTitle,
    conversations,
    loadingConversations,
    loadingConversation,
    chatScope,
    scopeMode,
    scopeClientId,
    pendingProposal,
    creatingProposal,
    confirmProposal,
    dismissProposal,
    aiOffline,
    aiOfflineReason,
    ask,
    send,
    setScope,
    openConversation,
    newConversation,
    removeConversation,
    ensureChatLoaded,
    openPanel,
    closePanel,
    togglePanel,
    minimize,
    restore,
  }
}
