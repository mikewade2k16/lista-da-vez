import { createApiRequest, getApiErrorMessage } from '~/utils/api-client'
import { useAuthStore } from '~/stores/auth'
import { useCalendarStore } from '~/stores/calendar'
import { useCoreAccountStore } from '../../layers/core/stores/account'
import {
  calendarChatProposalConfirmationKey,
  confirmCalendarChatProposal,
  deleteConversation as apiDeleteConversation,
  fetchChatScope,
  fetchConversations,
  getConversation,
  updateProposalStatus,
  type CalendarChatConversation,
  type CalendarChatCalendarItem,
  type CalendarChatProposalFields,
  type CalendarChatStoredProposal,
  type CalendarChatStoredMessage,
  type AssistantResource,
  type CalendarChatScope,
  type CalendarChatScopeMode,
  type AssistantChatSurface,
  normalizeAssistantChatSurface,
  normalizeAssistantResources,
  normalizeCalendarChatStoredMessage,
  assistantResourceInstruction,
} from '~/domain/calendar/calendar-chat-api'
import {
  cancelMetaActionProposal,
  confirmMetaActionProposal,
  getMetaActionProposal,
  metaActionConfirmationKey,
  metaActionCancellationKey,
  reconcileMetaActionProposal,
  type MetaAdsActionProposalView,
} from '~/domain/meta-ads/meta-ads-actions-api'

// Controller transitorio do chat compartilhado (SPEC-F7/F10, contrato C7/D3/D4).
// Mantem o nome useCalendarChat para preservar os consumidores enquanto o host passa
// a atender Calendar, Meta Ads e as demais rotas pelos endpoints /v1/assistant/chat.
// A partir da wave 4 as
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
  // proposals (multi-tarefa, WAVE 5.1): lista de propostas, cada uma com status proprio.
  proposals: CalendarChatStoredProposal[]
  calendarItems: CalendarChatCalendarItem[]
  resources: AssistantResource[]
}

interface CalendarChatAskResponse {
  answer?: string
  conversationId?: string
  title?: string
  surface?: AssistantChatSurface
  entrySurface?: AssistantChatSurface
  message?: unknown
  // aiError (WAVE 5) = a IA nao respondeu (503/cota/chave/vazio). O front mostra o estado
  // "IA fora do ar" (visual distinto), nao um balao normal, e nao persiste a mensagem.
  aiError?: boolean
}

interface PendingChatAskAttempt {
  fingerprint: string
  key: string
  userMessageId: string
  viaVoice: boolean
}

const QUESTION_MAX_LENGTH = 4000

// Escopo default antes de o back responder o /chat/scope (cliente-side por seguranca:
// so libera o select quando o back confirma canSelect=true).
const DEFAULT_SCOPE: CalendarChatScope = { canSelect: false, lockedClientId: '', clients: [] }

export function createAssistantConversationLoadFence() {
  let conversationLoadGeneration = 0
  let controller: AbortController | null = null
  return {
    begin() {
      controller?.abort()
      controller = new AbortController()
      const generation = ++conversationLoadGeneration
      return {
        signal: controller.signal,
        isCurrent: () => generation === conversationLoadGeneration,
      }
    },
    invalidate() {
      conversationLoadGeneration += 1
      controller?.abort()
      controller = null
    },
  }
}

export function createAssistantChatRuntime() {
  return {
    inflightController: null as AbortController | null,
    identityAbortController: null as AbortController | null,
    conversationLoadFence: createAssistantConversationLoadFence(),
  }
}

type AssistantChatRuntime = ReturnType<typeof createAssistantChatRuntime>
const assistantChatRuntimes = new WeakMap<object, AssistantChatRuntime>()

function useAssistantChatRuntime(): AssistantChatRuntime {
  // Cada Nuxt app/SSR request recebe controladores proprios. O WeakMap preserva o
  // singleton no browser sem permitir que uma renderizacao aborte outro tenant no servidor.
  const nuxtApp = useNuxtApp() as object
  let runtime = assistantChatRuntimes.get(nuxtApp)
  if (!runtime) {
    runtime = createAssistantChatRuntime()
    assistantChatRuntimes.set(nuxtApp, runtime)
  }
  return runtime
}

function newId(): string {
  return crypto.randomUUID()
}

function localMessage(role: 'user' | 'assistant', text: string): CalendarChatMessage {
  return { id: newId(), role, text, proposals: [], calendarItems: [], resources: [] }
}

function storedMessage(raw: unknown): CalendarChatMessage {
  const message = normalizeCalendarChatStoredMessage(raw)
  return {
    id: message.id,
    role: message.role,
    text: message.content,
    proposals: message.proposals,
    calendarItems: message.calendarItems,
    resources: message.resources,
  }
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
  const runtime = useAssistantChatRuntime()
  const runtimeConfig = useRuntimeConfig()
  const auth = useAuthStore()
  const store = useCalendarStore()
  const accountStore = useCoreAccountStore()
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
  // Surface atual vem da rota; a surface da conversa fica congelada ate o usuario
  // iniciar outra conversa. Assim navegar Calendar -> Meta nao troca silenciosamente
  // o contexto de um historico ja aberto.
  const surface = useState<AssistantChatSurface>('assistant-chat:surface', () => 'global')
  const conversationSurface = useState<AssistantChatSurface>(
    'assistant-chat:conversation-surface',
    () => 'global',
  )

  // draftFromVoice (WAVE 15): o rascunho atual veio de TRANSCRICAO DE AUDIO (Whisper/ditado).
  // Vai como `viaVoice` no /ask — o prompt trata erros foneticos como provaveis. O painel marca
  // ao preencher por voz e limpa quando o usuario digita manualmente.
  const draftFromVoice = useState<boolean>('calendar-chat:draft-voice', () => false)

  // WAVE 5 (E7): proposta pendente de criacao (evento/task) sugerida pela IA. O painel
  // renderiza um cartao de confirmacao; confirmar cria pela API do proprio usuario.
  const proposalBusyId = useState<string>('calendar-chat:proposal-busy-id', () => '')

  // WAVE 5: estado "IA fora do ar" — true quando a ULTIMA tentativa falhou (503/cota/chave/
  // timeout/kill switch). O painel mostra um indicador no cabecalho + um bloco distinto no
  // chat (nunca um balao normal), para o usuario SEMPRE saber que a IA nao esta funcionando.
  const aiOffline = useState<boolean>('calendar-chat:ai-offline', () => false)
  const aiOfflineReason = useState<string>('calendar-chat:ai-offline-reason', () => '')
  const checkingAvailability = useState<boolean>('calendar-chat:ai-checking', () => false)
  // Mantido apos falha de transporte: repetir a mesma intencao reaproveita a
  // chave e o backend devolve as mesmas mensagens, sem gerar outro turno.
  const pendingAskAttempt = useState<PendingChatAskAttempt | null>(
    'assistant-chat:pending-ask-attempt',
    () => null,
  )

  // Lista de conversas persistidas (menu "Conversas") + estados de carga.
  const conversations = useState<CalendarChatConversation[]>('calendar-chat:list', () => [])
  const loadingConversations = useState<boolean>('calendar-chat:list-loading', () => false)
  const loadingConversation = useState<boolean>('calendar-chat:conv-loading', () => false)
  // Sinal de rolagem (UX tipo WhatsApp): ao ABRIR/CARREGAR uma conversa o painel posiciona na
  // 1a mensagem (le de cima pra baixo), em vez de cair na ultima. Mensagem NOVA (enviar/receber)
  // continua rolando pro fim. openConversation marca true; o painel consome e zera.
  const pendingTopScroll = useState<boolean>('calendar-chat:scroll-top', () => false)

  // Escopo do contexto (D3/D4). scopeMode/scopeClientId viajam no /ask e ficam salvos
  // na conversa; chatScope (do back) diz se o select aparece e trava o cliente unico.
  const chatScope = useState<CalendarChatScope>('calendar-chat:scope', () => ({ ...DEFAULT_SCOPE }))
  const scopeMode = useState<CalendarChatScopeMode>('calendar-chat:scope-mode', () => 'all')
  const scopeClientId = useState<string>('calendar-chat:scope-client', () => '')

  // Chave e geracoes sao compartilhadas entre todas as chamadas do composable. Isso
  // evita que painel, config e pagina instalem resets concorrentes e impede respostas
  // da account anterior de repovoarem o estado depois da troca de contexto.
  const boundIdentity = useState<string>('assistant-chat:identity', () => '')
  const identityGeneration = useState<number>('assistant-chat:identity-generation', () => 0)
  const availabilityGeneration = useState<number>('assistant-chat:availability-generation', () => 0)

  const identityKey = computed(() => {
    const accountId = String(accountStore.activeAccountId || '').trim()
    const userId = String(
      auth.principal?.userId || auth.principal?.userID || auth.user?.id || '',
    ).trim()
    return accountId && userId ? `${accountId}:${userId}` : ''
  })

  function activeSurface(): AssistantChatSurface {
    return conversationId.value || messages.value.length ? conversationSurface.value : surface.value
  }

  function identitySignal(): AbortSignal {
    if (!runtime.identityAbortController || runtime.identityAbortController.signal.aborted) {
      runtime.identityAbortController = new AbortController()
    }
    return runtime.identityAbortController.signal
  }

  function resetRuntimeState(): void {
    runtime.inflightController?.abort()
    runtime.inflightController = null
    runtime.identityAbortController?.abort()
    runtime.identityAbortController = new AbortController()
    runtime.conversationLoadFence.invalidate()
    identityGeneration.value += 1
    availabilityGeneration.value += 1
    messages.value = []
    draft.value = ''
    draftFromVoice.value = false
    sending.value = false
    errorMessage.value = ''
    panelOpen.value = false
    minimized.value = false
    conversationId.value = ''
    conversationTitle.value = ''
    conversationSurface.value = surface.value
    proposalBusyId.value = ''
    aiOffline.value = false
    aiOfflineReason.value = ''
    checkingAvailability.value = false
    pendingAskAttempt.value = null
    conversations.value = []
    loadingConversations.value = false
    loadingConversation.value = false
    pendingTopScroll.value = false
    chatScope.value = { ...DEFAULT_SCOPE }
    scopeMode.value = 'all'
    scopeClientId.value = ''
  }

  // Fail-closed na troca de account/usuario: apaga o estado visivel antes de qualquer
  // nova leitura. A geracao faz requests antigos serem descartados mesmo quando o
  // transporte ja estava avancado demais para o AbortController interromper.
  watch(
    identityKey,
    (nextIdentity) => {
      if (nextIdentity === boundIdentity.value) return
      boundIdentity.value = nextIdentity
      resetRuntimeState()
    },
    { immediate: true },
  )

  function setSurface(nextSurface: AssistantChatSurface): void {
    const normalized = normalizeAssistantChatSurface(nextSurface)
    if (surface.value === normalized) return
    surface.value = normalized
    if (!conversationId.value && !messages.value.length) {
      conversationSurface.value = normalized
    }
    if (panelOpen.value) void checkAvailability()
  }

  // Cards read-only nunca executam uma acao. O clique apenas prepara uma nova
  // instrucao editavel no draft; o usuario ainda precisa revisar e enviar, e uma
  // eventual escrita continua sujeita ao fluxo separado de proposta/confirmacao.
  function prepareResourceInstruction(resource: AssistantResource): void {
    const clean = normalizeAssistantResources([resource])[0]
    if (!clean) return
    draft.value = assistantResourceInstruction(clean)
    draftFromVoice.value = false
    panelOpen.value = true
    minimized.value = false
  }

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
    // O filtro do calendario so e contexto implicito na propria surface. Reusar um
    // cliente deixado no calendario ao abrir Meta/global misturaria modulos sem o
    // usuario ter escolhido esse escopo no assistente.
    const filtered = surface.value === 'calendar' ? store.selectedClientId : ''
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
    // O backend congela o escopo no nascimento da conversa. Impede que chamadas
    // programaticas ou um select tardio deixem a UI apontando para outro cliente
    // enquanto a conversa continua autoritativamente no escopo salvo.
    if (sending.value || conversationId.value || messages.value.length) return
    scopeMode.value = mode === 'client' ? 'client' : 'all'
    scopeClientId.value = mode === 'client' ? clientId : ''
    void checkAvailability()
  }

  async function loadConversations(): Promise<void> {
    const generation = identityGeneration.value
    loadingConversations.value = true
    try {
      const loaded = await fetchConversations(apiRequest, identitySignal())
      if (generation === identityGeneration.value) conversations.value = loaded
    } catch {
      // silencioso: mantem a lista anterior (nao trava o chat)
    } finally {
      if (generation === identityGeneration.value) loadingConversations.value = false
    }
  }

  async function loadScope(): Promise<void> {
    const generation = identityGeneration.value
    try {
      const scope = await fetchChatScope(apiRequest, identitySignal())
      if (generation !== identityGeneration.value) return
      chatScope.value = scope
      applyScopeDefault(scope)
    } catch {
      // sem escopo do back: mantem o atual
    }
  }

  // Preflight barato: valida config/chave/kill switch e o /healthz do n8n. A rota nao
  // executa prompt, nao cria mensagem e nao consome tokens.
  async function checkAvailability(): Promise<boolean> {
    const sequence = availabilityGeneration.value + 1
    availabilityGeneration.value = sequence
    checkingAvailability.value = true
    try {
      await apiRequest('/v1/assistant/chat/status', {
        query: {
          surface: activeSurface(),
          scopeMode: scopeMode.value,
          scopeClientId: scopeMode.value === 'client' ? scopeClientId.value : '',
        },
        dedupe: false,
        skipLoadingIndicator: true,
        signal: identitySignal(),
      })
      if (sequence === availabilityGeneration.value) {
        aiOffline.value = false
        aiOfflineReason.value = ''
      }
      return sequence === availabilityGeneration.value
    } catch (error) {
      if (sequence === availabilityGeneration.value) {
        aiOffline.value = true
        aiOfflineReason.value = actionableError(error)
      }
      return false
    } finally {
      if (sequence === availabilityGeneration.value) {
        checkingAvailability.value = false
      }
    }
  }

  // Chamado ao abrir o chat: busca lista/escopo e so entao verifica a IA sem tokens.
  async function ensureChatLoaded(): Promise<void> {
    await auth.ensureSession()
    if (!auth.isAuthenticated) return
    await Promise.all([loadConversations(), loadScope()])
    await checkAvailability()
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
      return 'O assistente ainda nao esta configurado para esta area. Revise a configuracao da IA e tente novamente.'
    }
    if (code === 'ai_disabled') {
      return getApiErrorMessage(error, 'A IA esta desligada na configuracao do assistente.')
    }
    if (code === 'ai_key_missing') {
      return getApiErrorMessage(error, 'A chave do provedor de IA nao esta configurada na aba IA.')
    }
    if (status === 504 || code === 'upstream_timeout') {
      return 'O servico de IA demorou para responder. Tente novamente em instantes.'
    }
    if (status === 502 || code === 'upstream_error') {
      return 'O servico de IA nao respondeu ao assistente. Tente novamente em instantes.'
    }
    if (status === 400) {
      return getApiErrorMessage(error, 'Pergunta invalida. Revise e tente de novo.')
    }
    return getApiErrorMessage(
      error,
      'Nao foi possivel falar com o assistente agora. Tente novamente em instantes.',
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
  async function ask(question: string, availabilityChecked = false): Promise<void> {
    const trimmed = String(question || '').trim()
    if (!trimmed || sending.value) {
      return
    }
    if (trimmed.length > QUESTION_MAX_LENGTH) {
      errorMessage.value = `A pergunta passou de ${QUESTION_MAX_LENGTH} caracteres. Resuma e tente de novo.`
      return
    }
    // O preflight acontece ANTES de adicionar a pergunta ao historico e antes de chamar /ask.
    if (!availabilityChecked && !(await checkAvailability())) {
      return
    }

    runtime.inflightController?.abort()
    const controller = new AbortController()
    runtime.inflightController = controller
    const generation = identityGeneration.value

    const requestSurface = activeSurface()
    const month = requestSurface === 'calendar' ? store.focusMonthKey || '' : ''
    const fingerprint = JSON.stringify({
      question: trimmed,
      conversationId: conversationId.value,
      surface: requestSurface,
      scopeMode: scopeMode.value,
      scopeClientId: scopeMode.value === 'client' ? scopeClientId.value : '',
      month,
    })
    const previousAttempt = pendingAskAttempt.value
    const retryAttempt = previousAttempt?.fingerprint === fingerprint ? previousAttempt : null
    const viaVoice = retryAttempt ? retryAttempt.viaVoice : draftFromVoice.value
    const userMessage = retryAttempt
      ? messages.value.find((message) => message.id === retryAttempt.userMessageId)
      : undefined
    const attempt: PendingChatAskAttempt = retryAttempt
      ? retryAttempt
      : {
          fingerprint,
          key: `assistant-ask:${newId()}`,
          userMessageId: userMessage?.id || newId(),
          viaVoice,
        }
    pendingAskAttempt.value = attempt
    draftFromVoice.value = false
    errorMessage.value = ''
    sending.value = true
    if (!userMessage) {
      messages.value = [
        ...messages.value,
        {
          id: attempt.userMessageId,
          role: 'user',
          text: trimmed,
          proposals: [],
          calendarItems: [],
          resources: [],
        },
      ]
    }

    try {
      const body: Record<string, unknown> = {
        question: trimmed,
        conversationId: conversationId.value,
        surface: requestSurface,
        // Escopo do contexto (D4): 'client' manda o cliente; 'all' = todos os
        // visiveis. O back valida contra a permissao (nunca confia no body).
        scopeMode: scopeMode.value,
        scopeClientId: scopeMode.value === 'client' ? scopeClientId.value : '',
        // WAVE 15: veio de transcricao de audio => o prompt considera erros foneticos.
        viaVoice,
      }
      if (month) body.month = month

      const response = (await apiRequest('/v1/assistant/chat/ask', {
        method: 'POST',
        body,
        headers: { 'Idempotency-Key': attempt.key },
        signal: controller.signal,
      })) as CalendarChatAskResponse
      if (generation !== identityGeneration.value) return
      pendingAskAttempt.value = null

      const newConvId = String(response?.conversationId || '').trim()
      const newTitle = String(response?.title || '').trim()
      if (newConvId) conversationId.value = newConvId
      if (newTitle) conversationTitle.value = newTitle
      if (response?.surface || response?.entrySurface) {
        conversationSurface.value = normalizeAssistantChatSurface(
          response.surface ?? response.entrySurface,
        )
      }

      // WAVE 5: a IA falhou (n8n sinalizou aiError). NAO adiciona balao normal — marca o
      // estado "IA fora do ar" (visual distinto) com o motivo. A pergunta do usuario fica.
      if (response?.aiError) {
        aiOffline.value = true
        aiOfflineReason.value =
          String(response?.answer || '').trim() || 'A IA não respondeu agora. Tente de novo.'
        return
      }

      aiOffline.value = false
      aiOfflineReason.value = ''
      const answer = String(response?.answer || '').trim()
      messages.value = [
        ...messages.value,
        response?.message
          ? storedMessage(response.message)
          : localMessage(
              'assistant',
              answer || 'A IA nao retornou uma resposta. Tente reformular a pergunta.',
            ),
      ]
      void hydratePendingMetaActions(generation)
      upsertConversationSummary(conversationId.value, conversationTitle.value)
    } catch (error) {
      if (generation !== identityGeneration.value) return
      // Pergunta cancelada de proposito (o usuario enviou outra) nao vira erro.
      if (isAbortError(error)) {
        return
      }
      // WAVE 5: qualquer falha ao falar com a IA (rede/503/timeout/back) = estado "IA fora
      // do ar" com visual distinto (nao um balao nem so a barra de erro generica).
      aiOffline.value = true
      aiOfflineReason.value = actionableError(error)
      if (!draft.value.trim()) draft.value = trimmed
      const status = httpStatus(error)
      const code = errorCode(error)
      if (
        status > 0 &&
        ![409, 502, 503, 504].includes(status) &&
        code !== 'idempotency_in_progress'
      ) {
        pendingAskAttempt.value = null
      }
    } finally {
      // So libera o estado se este controller ainda for o vigente.
      if (runtime.inflightController === controller) {
        runtime.inflightController = null
        sending.value = false
      }
    }
  }

  // Envia o rascunho somente depois do preflight. Se a IA estiver indisponivel, preserva
  // o texto no campo para o usuario nao perder o que escreveu.
  async function send(): Promise<void> {
    const question = draft.value.trim()
    if (!question || sending.value || checkingAvailability.value) {
      return
    }
    if (!(await checkAvailability())) return
    if (draft.value.trim() === question) draft.value = ''
    await ask(question, true)
  }

  // Abre uma conversa persistida: carrega as mensagens do banco (SUBSTITUI as locais) e
  // adota o escopo salvo da conversa. Acesso resolvido no back (fora do visivel => 404,
  // tratado como erro acionavel).
  async function openConversation(id: string): Promise<void> {
    if (!id) return
    pendingAskAttempt.value = null
    runtime.inflightController?.abort()
    runtime.inflightController = null
    const load = runtime.conversationLoadFence.begin()
    const generation = identityGeneration.value
    loadingConversation.value = true
    errorMessage.value = ''
    try {
      const detail = await getConversation(apiRequest, id, load.signal)
      if (generation !== identityGeneration.value || !load.isCurrent()) return
      conversationId.value = detail.id
      conversationTitle.value = detail.title
      conversationSurface.value = detail.surface
      scopeMode.value = detail.scopeMode
      scopeClientId.value = detail.scopeClientId
      // Marca ANTES de trocar as mensagens: o painel posiciona no topo (1a msg) em vez de
      // rolar pro fim como faz numa mensagem nova.
      pendingTopScroll.value = true
      messages.value = detail.messages.map(storedMessage)
      void hydratePendingMetaActions(generation)
      draft.value = ''
      sending.value = false
      void checkAvailability()
    } catch (error) {
      if (generation === identityGeneration.value && load.isCurrent() && !isAbortError(error)) {
        errorMessage.value = actionableError(error)
      }
    } finally {
      if (generation === identityGeneration.value && load.isCurrent()) {
        loadingConversation.value = false
      }
    }
  }

  // Inicia uma conversa nova: limpa o historico visivel e zera o id (o back cria a
  // conversa no primeiro /ask). Reaplica o escopo default (cliente-side trava; agencia
  // usa o filtro/todos).
  function newConversation(): void {
    runtime.inflightController?.abort()
    runtime.inflightController = null
    runtime.conversationLoadFence.invalidate()
    messages.value = []
    draft.value = ''
    errorMessage.value = ''
    sending.value = false
    loadingConversation.value = false
    conversationId.value = ''
    conversationTitle.value = ''
    conversationSurface.value = surface.value
    proposalBusyId.value = ''
    pendingAskAttempt.value = null
    applyScopeDefault(chatScope.value)
    void checkAvailability()
  }

  // WAVE 5 (E7): descarta a proposta sem criar nada.
  function replaceMessage(updated: CalendarChatStoredMessage): void {
    messages.value = messages.value.map((message) =>
      message.id === updated.id ? storedMessage(updated) : message,
    )
  }

  // O status operacional da acao vive no agregado Meta Ads. O chat guarda apenas o card
  // e seu aceite/recusa; esta projecao atualiza a copia visual sem transformar o JSONB do
  // chat em fonte de verdade da execucao.
  function patchMetaActionView(view: MetaAdsActionProposalView): void {
    if (!view.id) return
    messages.value = messages.value.map((message) => ({
      ...message,
      proposals: message.proposals.map((proposal) => {
        const meta = proposal.fields.metaAction
        if (proposal.kind !== 'metaAction' || meta?.actionProposalId !== view.id) return proposal
        return {
          ...proposal,
          fields: {
            metaAction: {
              ...meta,
              action: view.action,
              actionProposalId: view.id,
              adAccountId: view.adAccountId,
              adAccountName: view.adAccountName,
              campaignId: view.targetCampaignId || meta.campaignId,
              currency: view.currency,
              summary: view.summary || meta.summary,
              actionStatus: view.status,
              executionAvailable: view.executionAvailable,
              canConfirm: view.canConfirm,
              requiresSpendAcknowledgement: view.requiresSpendAcknowledgement,
              expiresAt: view.expiresAt,
              errorCode: view.errorCode,
              errorMessage: view.errorMessage,
            },
          },
        }
      }),
    }))
  }

  async function hydratePendingMetaActions(generation: number): Promise<void> {
    const actionIds = Array.from(
      new Set(
        messages.value.flatMap((message) =>
          message.proposals
            .filter((proposal) => proposal.status === 'pending' && proposal.kind === 'metaAction')
            .map((proposal) => proposal.fields.metaAction?.actionProposalId || '')
            .filter(Boolean),
        ),
      ),
    ).slice(-20)
    await Promise.all(
      actionIds.map(async (actionId) => {
        try {
          const view = await getMetaActionProposal(apiRequest, actionId, identitySignal())
          if (generation === identityGeneration.value) patchMetaActionView(view)
        } catch (error) {
          if (!isAbortError(error)) {
            // O snapshot persistido continua visivel; uma falha de hidratacao nunca apaga o card.
          }
        }
      }),
    )
  }

  // pendingTargets resolve as propostas AINDA pendentes de uma mensagem para os ids pedidos
  // (ignora ids ja resolvidos ou inexistentes) — base comum de criar/recusar em lote.
  function pendingTargets(messageId: string, proposalIds: string[]): CalendarChatStoredProposal[] {
    const message = messages.value.find((m) => m.id === messageId)
    if (!message) return []
    return message.proposals.filter((p) => proposalIds.includes(p.id) && p.status === 'pending')
  }

  function metaActionOutcomeMessage(view: MetaAdsActionProposalView): string {
    if (view.errorMessage) return view.errorMessage
    if (view.status === 'unknown') {
      return 'O Meta pode ter aplicado esta acao. Use Reconciliar antes de qualquer nova tentativa.'
    }
    if (view.status === 'executing') {
      return 'A acao ainda esta em processamento. Aguarde e use Reconciliar.'
    }
    if (view.status === 'failed') {
      return 'A acao falhou sem uma nova tentativa automatica. Revise o card ou recuse e reformule.'
    }
    if (view.status === 'cancelled')
      return 'A proposta Meta foi cancelada e nao pode mais ser executada.'
    if (view.status === 'expired')
      return 'A proposta Meta expirou. Prepare uma nova acao pelo chat.'
    return 'A acao Meta ainda nao foi concluida.'
  }

  async function confirmDurableMetaAction(
    proposal: CalendarChatStoredProposal,
    messageId: string,
    acknowledgeSpend: boolean,
    generation: number,
    activeConversationId: string,
    signal: AbortSignal,
  ): Promise<string> {
    const meta = proposal.fields.metaAction
    if (!meta?.actionProposalId) {
      return meta?.errorMessage || 'Esta proposta Meta nao esta disponivel para confirmacao.'
    }
    if (meta.requiresSpendAcknowledgement && !acknowledgeSpend) {
      return 'Marque a confirmacao reforcada de gasto antes de continuar.'
    }
    if (
      meta.actionStatus !== 'succeeded' &&
      (!meta.executionAvailable || !meta.canConfirm || meta.actionStatus !== 'pending')
    ) {
      return meta.errorMessage || 'Esta acao Meta nao pode ser executada neste estado.'
    }

    const view = await confirmMetaActionProposal(
      apiRequest,
      meta.actionProposalId,
      metaActionConfirmationKey(messageId, meta.actionProposalId),
      meta.requiresSpendAcknowledgement && acknowledgeSpend,
      signal,
    )
    if (generation !== identityGeneration.value) return ''
    patchMetaActionView(view)
    if (view.status !== 'succeeded') return metaActionOutcomeMessage(view)

    // Somente o sucesso duravel autoriza espelhar accepted no JSONB do chat. Se este
    // PATCH falhar, o retry usa a mesma chave e o executor devolve o mesmo resultado.
    const updated = await updateProposalStatus(
      apiRequest,
      activeConversationId,
      messageId,
      proposal.id,
      'accepted',
      signal,
    )
    if (generation !== identityGeneration.value) return ''
    replaceMessage(updated)
    patchMetaActionView(view)
    return ''
  }

  // Confirma em lote, mas cada efeito local e executado exclusivamente pelo endpoint
  // transacional. O front apenas envia os campos editaveis e substitui a mensagem
  // autoritativa devolvida; nao chama stores de Calendar/Tasks.
  async function confirmSelectedProposals(
    messageId: string,
    items: {
      id: string
      clientId: string
      fields?: CalendarChatProposalFields
      acknowledgeSpend?: boolean
    }[],
  ): Promise<void> {
    if (proposalBusyId.value) return
    const targets = pendingTargets(
      messageId,
      items.map((i) => i.id),
    )
    if (!targets.length) return
    const clientById = new Map(items.map((i) => [i.id, i.clientId]))
    const itemById = new Map(items.map((item) => [item.id, item]))
    // Edit inline (WAVE 9): campos ajustados pelo dono no cartao antes de aprovar; senao usa os da IA.
    const fieldsById = new Map(items.filter((i) => i.fields).map((i) => [i.id, i.fields!]))
    const generation = identityGeneration.value
    const activeConversationId = conversationId.value
    const signal = identitySignal()
    proposalBusyId.value = messageId
    errorMessage.value = ''
    let failed = 0
    let lastError = ''
    try {
      for (const proposal of targets) {
        if (generation !== identityGeneration.value) return
        try {
          if (proposal.kind === 'metaAction') {
            const item = itemById.get(proposal.id)
            const err = await confirmDurableMetaAction(
              proposal,
              messageId,
              item?.acknowledgeSpend === true,
              generation,
              activeConversationId,
              signal,
            )
            if (generation !== identityGeneration.value) return
            if (err) {
              failed++
              lastError = err
            }
            continue
          }
          if (proposal.execution?.canConfirm === false) {
            failed++
            lastError =
              proposal.execution.message || 'Este card nao possui um executor seguro no backend.'
            continue
          }
          const updated = await confirmCalendarChatProposal(
            apiRequest,
            activeConversationId,
            messageId,
            proposal.id,
            calendarChatProposalConfirmationKey(messageId, proposal.id),
            fieldsById.get(proposal.id),
            clientById.get(proposal.id) || '',
            signal,
          )
          if (generation !== identityGeneration.value) return
          replaceMessage(updated)
        } catch (error) {
          if (generation !== identityGeneration.value || isAbortError(error)) return
          failed++
          lastError = actionableError(error)
        }
      }
      if (generation === identityGeneration.value && failed > 0) {
        const okCount = targets.length - failed
        const reason = lastError ? ` Motivo: ${lastError}` : ''
        errorMessage.value =
          okCount > 0
            ? `Apliquei ${okCount} de ${targets.length}. ${failed} falhou.${reason}`
            : `Não consegui aplicar.${reason}`
      }
    } finally {
      if (generation === identityGeneration.value) proposalBusyId.value = ''
    }
  }

  async function reconcileMetaAction(messageId: string, proposalId: string): Promise<void> {
    if (proposalBusyId.value) return
    const proposal = pendingTargets(messageId, [proposalId])[0]
    const actionId = proposal?.fields.metaAction?.actionProposalId || ''
    if (proposal?.kind !== 'metaAction' || !actionId) return
    const generation = identityGeneration.value
    const activeConversationId = conversationId.value
    const signal = identitySignal()
    proposalBusyId.value = messageId
    errorMessage.value = ''
    try {
      const view = await reconcileMetaActionProposal(apiRequest, actionId, signal)
      if (generation !== identityGeneration.value) return
      patchMetaActionView(view)
      if (view.status !== 'succeeded') {
        errorMessage.value = metaActionOutcomeMessage(view)
        return
      }
      const updated = await updateProposalStatus(
        apiRequest,
        activeConversationId,
        messageId,
        proposal.id,
        'accepted',
        signal,
      )
      if (generation !== identityGeneration.value) return
      replaceMessage(updated)
      patchMetaActionView(view)
    } catch (error) {
      if (generation !== identityGeneration.value || isAbortError(error)) return
      errorMessage.value = actionableError(error)
    } finally {
      if (generation === identityGeneration.value) proposalBusyId.value = ''
    }
  }

  // rejectSelectedProposals recusa em lote (persiste 'rejected'); serve tanto o "×" de um item
  // quanto o "Recusar todas". Nao cria nada.
  async function rejectSelectedProposals(messageId: string, proposalIds: string[]): Promise<void> {
    if (proposalBusyId.value) return
    const targets = pendingTargets(messageId, proposalIds)
    if (!targets.length) return
    const generation = identityGeneration.value
    const activeConversationId = conversationId.value
    const signal = identitySignal()
    proposalBusyId.value = messageId
    errorMessage.value = ''
    try {
      for (const proposal of targets) {
        if (generation !== identityGeneration.value) return
        try {
          if (proposal.kind === 'metaAction') {
            const meta = proposal.fields.metaAction
            const actionId = meta?.actionProposalId || ''
            if (
              !actionId ||
              (meta?.actionStatus !== 'pending' && meta?.actionStatus !== 'cancelled')
            ) {
              errorMessage.value =
                'Esta acao Meta ja foi iniciada ou concluida e nao pode ser recusada.'
              continue
            }
            if (meta.actionStatus === 'pending') {
              const view = await cancelMetaActionProposal(
                apiRequest,
                actionId,
                metaActionCancellationKey(messageId, actionId),
                signal,
              )
              if (generation !== identityGeneration.value) return
              patchMetaActionView(view)
              if (view.status !== 'cancelled') {
                errorMessage.value = metaActionOutcomeMessage(view)
                continue
              }
            }
          }
          const updated = await updateProposalStatus(
            apiRequest,
            activeConversationId,
            messageId,
            proposal.id,
            'rejected',
            signal,
          )
          if (generation !== identityGeneration.value) return
          replaceMessage(updated)
        } catch (error) {
          if (generation !== identityGeneration.value || isAbortError(error)) return
          errorMessage.value = actionableError(error)
        }
      }
    } finally {
      if (generation === identityGeneration.value) proposalBusyId.value = ''
    }
  }

  // Apaga (soft-delete) uma conversa. Se era a ativa, comeca uma nova.
  async function removeConversation(id: string): Promise<void> {
    if (!id) return
    const generation = identityGeneration.value
    try {
      await apiDeleteConversation(apiRequest, id, identitySignal())
    } catch {
      return
    }
    if (generation !== identityGeneration.value) return
    conversations.value = conversations.value.filter((c) => c.id !== id)
    if (conversationId.value === id) newConversation()
  }

  return {
    messages,
    draft,
    draftFromVoice,
    sending,
    errorMessage,
    panelOpen,
    minimized,
    conversationId,
    conversationTitle,
    surface,
    conversationSurface,
    conversations,
    loadingConversations,
    loadingConversation,
    pendingTopScroll,
    chatScope,
    scopeMode,
    scopeClientId,
    proposalBusyId,
    confirmSelectedProposals,
    rejectSelectedProposals,
    reconcileMetaAction,
    aiOffline,
    aiOfflineReason,
    checkingAvailability,
    checkAvailability,
    ask,
    send,
    setScope,
    setSurface,
    prepareResourceInstruction,
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
