import { createApiRequest, getApiErrorMessage } from '~/utils/api-client'
import { useAuthStore } from '~/stores/auth'
import { useCalendarStore } from '~/stores/calendar'
import type { CalendarEventInput } from '~/utils/calendar'
import { applyClientProfileProposal, applyNoteProposal } from '~/utils/calendar-chat-crud'
// Store de Tasks vive em outra layer; import cross-layer (precedente: ConfigTasks.vue). So
// e usado ao CONFIRMAR uma proposta de task (WAVE 5, E7); a Pinia instancia sob demanda.
import { useTasksStore } from '../../layers/tasks/stores/tasks'
import type { TaskPriority } from '../../layers/tasks/types/tasks'
import {
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
  // proposals (multi-tarefa, WAVE 5.1): lista de propostas, cada uma com status proprio.
  proposals: CalendarChatStoredProposal[]
  calendarItems: CalendarChatCalendarItem[]
}

interface CalendarChatAskResponse {
  answer?: string
  conversationId?: string
  title?: string
  message?: CalendarChatStoredMessage
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
let availabilitySequence = 0

function newId(): string {
  return crypto.randomUUID()
}

function localMessage(role: 'user' | 'assistant', text: string): CalendarChatMessage {
  return { id: newId(), role, text, proposals: [], calendarItems: [] }
}

function storedMessage(message: CalendarChatStoredMessage): CalendarChatMessage {
  return {
    id: message.id,
    role: message.role,
    text: message.content,
    proposals: message.proposals,
    calendarItems: message.calendarItems,
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

function hasProposalField(
  fields: CalendarChatProposalFields,
  key: keyof CalendarChatProposalFields,
) {
  return Object.prototype.hasOwnProperty.call(fields, key)
}

function proposalText(
  fields: CalendarChatProposalFields,
  key: keyof CalendarChatProposalFields,
): string {
  const value = fields[key]
  return typeof value === 'string' ? value.trim() : ''
}

function proposalArray(
  fields: CalendarChatProposalFields,
  key: keyof CalendarChatProposalFields,
): string[] {
  const value = fields[key]
  return Array.isArray(value) ? value.map((item) => String(item || '').trim()).filter(Boolean) : []
}

function proposalBody(fields: CalendarChatProposalFields): string {
  return proposalText(fields, 'description') || proposalText(fields, 'contentHtml')
}

function firstProposalText(
  fields: CalendarChatProposalFields,
  keys: Array<keyof CalendarChatProposalFields>,
): string {
  for (const key of keys) {
    const value = proposalText(fields, key)
    if (value) return value
  }
  return ''
}

function taskPriority(value: string): TaskPriority {
  return value === 'alta' || value === 'baixa' || value === 'media' ? value : 'media'
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
  const proposalBusyId = useState<string>('calendar-chat:proposal-busy-id', () => '')

  // WAVE 5: estado "IA fora do ar" — true quando a ULTIMA tentativa falhou (503/cota/chave/
  // timeout/kill switch). O painel mostra um indicador no cabecalho + um bloco distinto no
  // chat (nunca um balao normal), para o usuario SEMPRE saber que a IA nao esta funcionando.
  const aiOffline = useState<boolean>('calendar-chat:ai-offline', () => false)
  const aiOfflineReason = useState<string>('calendar-chat:ai-offline-reason', () => '')
  const checkingAvailability = useState<boolean>('calendar-chat:ai-checking', () => false)

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
    void checkAvailability()
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

  // Preflight barato: valida config/chave/kill switch e o /healthz do n8n. A rota nao
  // executa prompt, nao cria mensagem e nao consome tokens.
  async function checkAvailability(): Promise<boolean> {
    const sequence = ++availabilitySequence
    checkingAvailability.value = true
    try {
      await apiRequest('/v1/calendar/chat/status', {
        query: {
          scopeMode: scopeMode.value,
          scopeClientId: scopeMode.value === 'client' ? scopeClientId.value : '',
        },
        dedupe: false,
        skipLoadingIndicator: true,
      })
      if (sequence === availabilitySequence) {
        aiOffline.value = false
        aiOfflineReason.value = ''
      }
      return true
    } catch (error) {
      if (sequence === availabilitySequence) {
        aiOffline.value = true
        aiOfflineReason.value = actionableError(error)
      }
      return false
    } finally {
      if (sequence === availabilitySequence) {
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
      return 'O chat do calendario ainda nao esta configurado. Defina o env CALENDAR_CHAT_WEBHOOK_URL e importe o workflow "Calendar Chat" no n8n.'
    }
    if (code === 'ai_disabled') {
      return getApiErrorMessage(error, 'A IA do calendario esta desligada na aba IA.')
    }
    if (code === 'ai_key_missing') {
      return getApiErrorMessage(error, 'A chave do provedor de IA nao esta configurada na aba IA.')
    }
    if (status === 504 || code === 'upstream_timeout') {
      return 'O n8n demorou para responder ao chat do calendario. Tente novamente; se repetir, confira se o container n8n esta reiniciando ou sem memoria.'
    }
    if (status === 502 || code === 'upstream_error') {
      return 'O n8n nao respondeu ao chat do calendario. Confira se o workflow "Calendar Chat" esta ativo e se o container n8n esta healthy.'
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

    inflightController?.abort()
    const controller = new AbortController()
    inflightController = controller

    errorMessage.value = ''
    sending.value = true
    messages.value = [...messages.value, localMessage('user', trimmed)]

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
    if (!id || loadingConversation.value) return
    inflightController?.abort()
    inflightController = null
    loadingConversation.value = true
    errorMessage.value = ''
    try {
      const detail = await getConversation(apiRequest, id)
      conversationId.value = detail.id
      conversationTitle.value = detail.title
      scopeMode.value = detail.scopeMode
      scopeClientId.value = detail.scopeClientId
      messages.value = detail.messages.map(storedMessage)
      draft.value = ''
      sending.value = false
      void checkAvailability()
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
    proposalBusyId.value = ''
    applyScopeDefault(chatScope.value)
    void checkAvailability()
  }

  // WAVE 5 (E7): descarta a proposta sem criar nada.
  function replaceMessage(updated: CalendarChatStoredMessage): void {
    messages.value = messages.value.map((message) =>
      message.id === updated.id ? storedMessage(updated) : message,
    )
  }

  function normalizePersonLabel(value: string): string {
    return String(value || '')
      .trim()
      .toLowerCase()
      .normalize('NFD')
      .replace(/[\u0300-\u036f]/g, '')
  }

  function resolveResponsibleId(value: string): string {
    const raw = String(value || '').trim()
    if (!raw) return ''
    const people = store.people || []
    if (people.some((person) => person.id === raw)) return raw
    const needle = normalizePersonLabel(raw)
    const matches = people.filter((person) => {
      const name = normalizePersonLabel(person.name)
      return name === needle || name.startsWith(`${needle} `)
    })
    return matches.length === 1 ? matches[0]!.id : raw
  }

  // matchByTitle acha o UNICO item cujo titulo casa com `name` (igual ou contido em qualquer
  // direcao) — rede de seguranca quando a IA manda o NOME em vez do id. Qualquer modelo escorrega
  // nisso (visto no gpt-4o-mini); a resolucao robusta e aqui, nao no prompt.
  function matchByTitle<T extends { title: string }>(items: T[], name: string): T | undefined {
    const needle = normalizePersonLabel(name)
    if (!needle) return undefined
    const cands = items.filter((it) => {
      const t = normalizePersonLabel(it.title)
      return t === needle || t.includes(needle) || needle.includes(t)
    })
    return cands.length === 1 ? cands[0] : undefined
  }

  // applyProposal EXECUTA a proposta pela API autenticada do usuario conforme a `action`:
  // create (item novo), update (edita item existente por targetId, full-replace mesclando os
  // campos nao-vazios) ou delete (exclui/arquiva). clientId ja vem resolvido pelo componente.
  // Devolve '' no sucesso ou uma mensagem de erro acionavel.
  async function applyProposal(
    proposal: CalendarChatStoredProposal,
    clientId: string,
  ): Promise<string> {
    const f = proposal.fields || {}
    const action = proposal.action || 'create'
    // WAVE 7: anotacao do mes e perfil do cliente tem execucao propria (kinds note/clientProfile,
    // em calendar-chat-crud.ts). Deps: apiRequest + os 3 pontos de nota da store + tradutor de erro.
    const crudDeps = { apiRequest, store, actionableError }
    if (proposal.kind === 'note') return applyNoteProposal(crudDeps, action, f.note || {})
    if (proposal.kind === 'clientProfile') {
      return applyClientProfileProposal(crudDeps, action, f, clientId)
    }
    const targetId = String(f.targetId || '')
    const proposedResponsibleId = resolveResponsibleId(proposalText(f, 'responsibleId'))

    async function loadConfiguredTasksBoard(includeArchived = false): Promise<string> {
      const boardId = String(store.config.tasks?.boardId || '')
      const tasksStore = useTasksStore()
      await tasksStore.initialize({ allowAutoCreate: false }).catch(() => undefined)
      if (boardId) {
        await tasksStore
          .ensureBoardTasksLoaded(boardId, { includeArchived, force: true })
          .catch(() => undefined)
      }
      return boardId
    }

    async function applyTaskTarget(): Promise<string> {
      const boardId = await loadConfiguredTasksBoard(action !== 'create')
      if (!boardId)
        return 'Configure um board na aba Integrações da config para usar tasks pelo chat.'
      const tasksStore = useTasksStore()
      const eventTaskId = store.getEventById(targetId)?.taskId || ''
      const taskId = eventTaskId || targetId
      let existingTask = tasksStore.tasks.find((task) => task.id === taskId)
      if (!existingTask) {
        // Rede de seguranca: a IA manda o NOME da task no targetId em vez do UUID, as vezes parcial.
        // Casa pelo titulo (igual/contido) se UNICO — cobre "Brasil GMS" -> "Brasil GMS Tooop".
        existingTask = matchByTitle(tasksStore.tasks, taskId)
      }
      if ((action === 'update' || action === 'delete') && !existingTask) {
        return 'Não encontrei essa task no board configurado. Abra/recarregue o board e tente de novo.'
      }
      if (action === 'delete') {
        const ok = await tasksStore.removeTask(existingTask!.id)
        return ok ? '' : 'Não consegui excluir a task.'
      }
      if (action === 'update') {
        const patch: Record<string, unknown> = {}
        const title = proposalText(f, 'title')
        const body = proposalBody(f)
        const dueDate = firstProposalText(f, ['dueDate', 'date'])
        const dueTime = proposalText(f, 'time')
        const involvedIds = proposalArray(f, 'involvedIds')
        if (title) patch.title = title
        if (body) patch.description = body
        if (proposalText(f, 'status')) patch.status = proposalText(f, 'status')
        if (proposalText(f, 'priority')) patch.priority = proposalText(f, 'priority')
        // HORARIO (WAVE 11): a task guarda o prazo com hora (datetime). Quando a IA manda time
        // junto da data, compomos 'YYYY-MM-DDTHH:MM' (hora local; toOptionalDateTime converte).
        // Time SEM data nova: reusa a data atual da task para so trocar a hora.
        if (dueDate) patch.dueDate = dueTime ? `${dueDate}T${dueTime}` : dueDate
        else if (dueTime && existingTask!.dueDate) {
          patch.dueDate = `${String(existingTask!.dueDate).slice(0, 10)}T${dueTime}`
        }
        if (proposalText(f, 'dueEndDate')) patch.dueEndDate = proposalText(f, 'dueEndDate')
        if (proposalText(f, 'type')) patch.type = proposalText(f, 'type')
        if (proposedResponsibleId) patch.responsible = proposedResponsibleId
        if (involvedIds.length) patch.involved = involvedIds
        if (clientId || proposalText(f, 'clientId'))
          patch.clientId = clientId || proposalText(f, 'clientId')
        if (proposalText(f, 'clientName')) patch.clientName = proposalText(f, 'clientName')
        if (hasProposalField(f, 'archived')) patch.archived = Boolean(f.archived)
        if (!Object.keys(patch).length) return 'Não encontrei campos para alterar nessa task.'
        const updated = await tasksStore.updateTask(existingTask!.id, patch)
        return updated ? '' : 'Não consegui editar a task.'
      }
      const createDate = firstProposalText(f, ['dueDate', 'date'])
      const createTime = proposalText(f, 'time')
      const created = await tasksStore.createTask({
        projectId: boardId,
        title: proposalText(f, 'title'),
        description: proposalBody(f),
        status: proposalText(f, 'status'),
        priority: taskPriority(proposalText(f, 'priority')),
        // HORARIO (WAVE 11): data+hora viram datetime local (toOptionalDateTime converte).
        dueDate: createDate && createTime ? `${createDate}T${createTime}` : createDate,
        dueEndDate: proposalText(f, 'dueEndDate'),
        responsible: proposedResponsibleId,
        involved: proposalArray(f, 'involvedIds'),
        clientId: clientId || proposalText(f, 'clientId'),
        clientName: proposalText(f, 'clientName'),
        type: proposalText(f, 'type'),
      })
      return created ? '' : 'Não consegui criar a task.'
    }

    if (action === 'update' || action === 'delete') {
      // Roteia primeiro pelo targetId real: a IA as vezes rotula um evento do calendario como
      // kind:'task'. Se o id e um evento carregado, edita/exclui o evento; se nao, task usa o
      // board configurado (context.tasks/taskId).
      let existing = store.getEventById(targetId)
      if (!existing && proposal.kind !== 'task') {
        // kind=event mas targetId pode ser um NOME (a IA escorrega). Casa o evento pelo titulo (a
        // store expoe eventsByDate: Record<data, eventos[]>, entao achatamos).
        const allEvents = Object.values(store.eventsByDate || {}).flat()
        existing = matchByTitle(allEvents, targetId) ?? null
      }
      if (!existing) {
        return proposal.kind === 'task'
          ? applyTaskTarget()
          : 'Não encontrei esse item no calendário (abra o mês dele e tente de novo).'
      }
      if (action === 'delete') {
        const ok = await store.deleteEvent(existing.id)
        return ok ? '' : 'Não consegui excluir o item.'
      }
      // update = full-replace: campos NAO-VAZIOS da proposta vencem; o resto mantem o existente.
      const proposedDate = firstProposalText(f, ['date', 'dueDate'])
      const proposedBody = proposalBody(f)
      const proposedInvolved = proposalArray(f, 'involvedIds')
      const buildPatch = (base: NonNullable<ReturnType<typeof store.getEventById>>) =>
        ({
          date: proposedDate || base.date || '',
          time: proposalText(f, 'time') || base.time || '',
          clientId: clientId || proposalText(f, 'clientId') || base.clientId || '',
          type: proposalText(f, 'type') || base.type || 'post',
          title: proposalText(f, 'title') || base.title || '',
          status: proposalText(f, 'status') || base.status || 'planejado',
          priority: proposalText(f, 'priority') || base.priority || 'media',
          responsibleId: proposedResponsibleId || base.responsibleId || '',
          involvedIds: proposedInvolved.length ? proposedInvolved : base.involvedIds || [],
          media: base.media || [],
          description: proposedBody || String(base.description || ''),
        }) as unknown as CalendarEventInput
      let outcome = await store.updateEvent(existing.id, buildPatch(existing), existing.version)
      if (outcome === 'conflict') {
        // Versao defasada (ex.: o evento mudou por sync de task no back). Refetch + tenta 1x com a
        // versao fresca — evita pedir "recarregue" quando o proprio sistema ja atualizou o item.
        await store.refetchWindow()
        const fresh = store.getEventById(existing.id)
        if (fresh) outcome = await store.updateEvent(fresh.id, buildPatch(fresh), fresh.version)
      }
      if (outcome === 'conflict') {
        return 'Esse item mudou enquanto isso. Recarregue o calendário e tente de novo.'
      }
      return outcome === 'ok' ? '' : 'Não consegui editar o item.'
    }

    // CREATE. Evento -> calendar store; task -> tasks store.
    if (proposal.kind === 'task') {
      return applyTaskTarget()
    }
    // createEvent tipa os enums (type/status/priority); a proposta vem validada e o back
    // re-valida, entao usamos um cast controlado no payload.
    const ok = await store.createEvent({
      date: firstProposalText(f, ['date', 'dueDate']),
      time: proposalText(f, 'time'),
      clientId: clientId || proposalText(f, 'clientId'),
      type: proposalText(f, 'type') || 'post',
      title: proposalText(f, 'title'),
      status: proposalText(f, 'status') || 'planejado',
      priority: proposalText(f, 'priority') || 'media',
      responsibleId: proposedResponsibleId,
      involvedIds: proposalArray(f, 'involvedIds'),
      media: [],
      description: proposalBody(f),
      createTask: Boolean(store.config.tasks?.boardId),
    } as unknown as CalendarEventInput)
    return ok ? '' : 'Não consegui criar o evento (confira a data proposta).'
  }

  // pendingTargets resolve as propostas AINDA pendentes de uma mensagem para os ids pedidos
  // (ignora ids ja resolvidos ou inexistentes) — base comum de criar/recusar em lote.
  function pendingTargets(messageId: string, proposalIds: string[]): CalendarChatStoredProposal[] {
    const message = messages.value.find((m) => m.id === messageId)
    if (!message) return []
    return message.proposals.filter((p) => proposalIds.includes(p.id) && p.status === 'pending')
  }

  // confirmSelectedProposals cria em LOTE as propostas selecionadas (multi-tarefa). `items`
  // traz, por proposta, o clientId JA resolvido pelo componente ({id, clientId}). Para cada uma
  // cria o item e, no sucesso, marca 'accepted' no back (replaceMessage atualiza o card item a
  // item). Falha parcial NAO aborta o lote — conta as que falharam e avisa no fim.
  async function confirmSelectedProposals(
    messageId: string,
    items: { id: string; clientId: string; fields?: CalendarChatProposalFields }[],
  ): Promise<void> {
    if (proposalBusyId.value) return
    const targets = pendingTargets(
      messageId,
      items.map((i) => i.id),
    )
    if (!targets.length) return
    const clientById = new Map(items.map((i) => [i.id, i.clientId]))
    // Edit inline (WAVE 9): campos ajustados pelo dono no cartao antes de aprovar; senao usa os da IA.
    const fieldsById = new Map(items.filter((i) => i.fields).map((i) => [i.id, i.fields!]))
    proposalBusyId.value = messageId
    errorMessage.value = ''
    let failed = 0
    let lastError = ''
    try {
      for (const proposal of targets) {
        try {
          const edited = fieldsById.get(proposal.id)
          const err = await applyProposal(
            edited ? { ...proposal, fields: edited } : proposal,
            clientById.get(proposal.id) || '',
          )
          if (err) {
            failed++
            lastError = err
            continue
          }
          const updated = await updateProposalStatus(
            apiRequest,
            conversationId.value,
            messageId,
            proposal.id,
            'accepted',
          )
          replaceMessage(updated)
        } catch (error) {
          failed++
          lastError = actionableError(error)
        }
      }
      if (failed > 0) {
        const okCount = targets.length - failed
        const reason = lastError ? ` Motivo: ${lastError}` : ''
        errorMessage.value =
          okCount > 0
            ? `Apliquei ${okCount} de ${targets.length}. ${failed} falhou.${reason}`
            : `Não consegui aplicar.${reason}`
      }
    } finally {
      proposalBusyId.value = ''
    }
  }

  // rejectSelectedProposals recusa em lote (persiste 'rejected'); serve tanto o "×" de um item
  // quanto o "Recusar todas". Nao cria nada.
  async function rejectSelectedProposals(messageId: string, proposalIds: string[]): Promise<void> {
    if (proposalBusyId.value) return
    const targets = pendingTargets(messageId, proposalIds)
    if (!targets.length) return
    proposalBusyId.value = messageId
    errorMessage.value = ''
    try {
      for (const proposal of targets) {
        try {
          const updated = await updateProposalStatus(
            apiRequest,
            conversationId.value,
            messageId,
            proposal.id,
            'rejected',
          )
          replaceMessage(updated)
        } catch (error) {
          errorMessage.value = actionableError(error)
        }
      }
    } finally {
      proposalBusyId.value = ''
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
    proposalBusyId,
    confirmSelectedProposals,
    rejectSelectedProposals,
    aiOffline,
    aiOfflineReason,
    checkingAvailability,
    checkAvailability,
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
