import { reactive } from 'vue'
export interface CrowPlusContext {
  accountId: string
  actorId: string
  modules: readonly string[]
  permissions: readonly string[]
  sheetId: string
  month: string
}
export interface CrowPlusEnvelope {
  contractVersion: string
  requestId: string
  conversationId: string
  context: { surface: string; mode: string; issuerId: string }
  status: string
  message: string
  intent: string
  data: Record<string, unknown>
  evidence: unknown[]
  proposals: unknown[]
}
export interface CrowPlusMessage {
  id: string
  role: string
  content: string
  plusEnvelope?: CrowPlusEnvelope
}
export type CrowPlusTransport = <T>(
  account: string,
  path: string,
  options: { method?: 'GET' | 'POST'; body?: unknown; key?: string; signal: AbortSignal },
) => Promise<T>
export function createCrowPlusController(
  context: () => CrowPlusContext,
  transport: CrowPlusTransport,
  key: () => string,
) {
  const state = reactive({
    enabled: false,
    statusKnown: false,
    busy: false,
    error: '',
    issuerId: '',
    conversationId: '',
    draft: '',
    messages: [] as CrowPlusMessage[],
    conversations: [] as Array<{ id: string; title: string }>,
    blocked: false,
  })
  let generation = 0,
    abort = new AbortController()
  let pending: { body: Record<string, unknown>; key: string } | null = null
  const scope = () => `${context().accountId}:${context().actorId}:${state.issuerId}`
  const permitted = () =>
    Boolean(context().accountId && context().actorId) &&
    context().modules.includes('finance') &&
    context().permissions.includes('finance.sheets.view') &&
    (!state.issuerId ||
      (context().modules.includes('accounting') &&
        context().permissions.includes('accounting.issuers.view')))
  function reset(issuerId = '') {
    generation++
    abort.abort()
    abort = new AbortController()
    pending = null
    Object.assign(state, {
      enabled: false,
      statusKnown: false,
      busy: false,
      error: '',
      issuerId,
      conversationId: '',
      draft: '',
      messages: [],
      conversations: [],
      blocked: false,
    })
  }
  async function run<T>(
    work: (
      request: <V>(
        path: string,
        options?: { method?: 'GET' | 'POST'; body?: unknown; key?: string },
      ) => Promise<V>,
    ) => Promise<T>,
    apply: (data: T) => void,
  ) {
    if (state.busy || !permitted()) return false
    const ticket = generation,
      origin = scope(),
      account = context().accountId,
      signal = abort.signal
    state.busy = true
    state.error = ''
    try {
      const value = await work(<V>(path: string, options = {}) => {
        if (ticket !== generation || origin !== scope() || signal.aborted)
          throw new Error('context_changed')
        return transport<V>(account, path, { ...options, signal })
      })
      if (ticket !== generation || origin !== scope()) return false
      apply(value)
      return true
    } catch (cause) {
      if (ticket === generation && origin === scope()) {
        const code = (cause as { data?: { error?: { code?: string } } }).data?.error?.code
        state.blocked = code === 'request_in_progress'
        state.error = state.blocked
          ? 'A operação está em andamento ou tem resultado incerto. Consulte o histórico; não reenvie uma preparação automaticamente.'
          : code === 'idempotency_conflict'
            ? 'A chave desta tentativa já foi usada com outros dados. Consulte o histórico.'
            : 'Não foi possível concluir. Confira a configuração, o contexto e as permissões.'
      }
      return false
    } finally {
      if (ticket === generation) state.busy = false
    }
  }
  const query = () => `mode=plus&issuerId=${encodeURIComponent(state.issuerId)}`
  async function load() {
    return run(
      async (request) => {
        const status = await request<{ data: { enabled: boolean } }>(
          `/v1/finance/assistant/plus/status?issuerId=${encodeURIComponent(state.issuerId)}`,
        )
        if (!status.data.enabled) return { enabled: false, conversations: [] }
        const history = await request<{ conversations: Array<{ id: string; title: string }> }>(
          `/v1/assistant/chat/conversations?${query()}`,
        )
        return { enabled: true, conversations: history.conversations }
      },
      (value) => {
        state.statusKnown = true
        state.enabled = value.enabled
        state.conversations = value.conversations
      },
    )
  }
  async function open(id: string) {
    if (!state.enabled || state.busy) return false
    return run(
      (request) =>
        request<{ id: string; issuerId: string; messages: CrowPlusMessage[] }>(
          `/v1/assistant/chat/conversations/${encodeURIComponent(id)}?${query()}`,
        ),
      (value) => {
        if ((value.issuerId || '') !== state.issuerId) return
        state.conversationId = value.id
        state.messages = value.messages
        state.draft = ''
        pending = null
        state.blocked = false
      },
    )
  }
  async function send(
    options: {
      action?: { toolId: string; input: unknown }
      draftId?: string
      diagnosisId?: string
      documentId?: string
    } = {},
  ) {
    if (!state.enabled || state.busy || state.blocked || !state.draft.trim()) return false
    if (/\b(senha|password|token|cookies?|api[_ -]?key)\b/i.test(state.draft)) {
      state.error = 'Use o formulário protegido de acesso; não envie segredos como mensagem.'
      return false
    }
    const c = context()
    const body = {
      surface: 'finance',
      mode: 'plus',
      contractVersion: '1.0',
      question: state.draft.trim(),
      conversationId: state.conversationId,
      issuerId: state.issuerId,
      ...(c.sheetId ? { financeSheetId: c.sheetId } : {}),
      ...(c.month ? { month: c.month } : {}),
      ...(options.draftId ? { financeDraftId: options.draftId } : {}),
      ...(options.diagnosisId ? { diagnosisId: options.diagnosisId } : {}),
      ...(options.documentId ? { documentId: options.documentId } : {}),
      ...(options.action ? { action: options.action } : {}),
    }
    if (!pending || JSON.stringify(pending.body) !== JSON.stringify(body))
      pending = { body, key: key() }
    const attempt = pending
    return run(
      (request) =>
        request<CrowPlusEnvelope>('/v1/assistant/chat/ask', {
          method: 'POST',
          body: attempt.body,
          key: attempt.key,
        }),
      (value) => {
        if (
          value.context.mode !== 'plus' ||
          value.context.surface !== 'finance' ||
          (value.context.issuerId || '') !== state.issuerId
        ) {
          state.error = 'Resposta incompatível com o contexto atual.'
          return
        }
        state.conversationId = value.conversationId
        state.messages.push(
          { id: `${value.requestId}:user`, role: 'user', content: body.question },
          {
            id: `${value.requestId}:assistant`,
            role: 'assistant',
            content: value.message,
            plusEnvelope: value,
          },
        )
        state.draft = ''
        pending = null
      },
    )
  }
  async function chooseIssuer(id: string) {
    reset(id)
    await load()
  }
  async function fresh() {
    const issuer = state.issuerId
    reset(issuer)
    await load()
  }
  return { state, permitted, reset, load, open, send, chooseIssuer, fresh }
}
