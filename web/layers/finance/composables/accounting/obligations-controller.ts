import { reactive } from 'vue'
export interface ObligationView {
  id: string
  code: string
  period: string
  version: number
  state: string
  amount: string | null
  dueDate: string | null
  pending: string[]
  memory: Record<string, unknown>
}
export interface ObligationContext {
  accountId: string
  actorId: string
  issuerId: string
  modules: readonly string[]
  permissions: readonly string[]
}
export type ObligationTransport = <T>(
  account: string,
  path: string,
  options: {
    method: 'GET' | 'POST'
    body?: unknown
    key?: string
    signal: AbortSignal
  },
) => Promise<T>
export function createObligationsController(
  context: () => ObligationContext,
  api: ObligationTransport,
  key = () => crypto.randomUUID(),
) {
  const state = reactive({
    items: [] as ObligationView[],
    period: '',
    code: '',
    busy: false,
    error: '',
    page: 1,
    more: false,
  })
  let generation = 0,
    abort = new AbortController()
  const signature = () => JSON.stringify(context())
  const allowed = (write = false) => {
    const c = context()
    return !!(
      c.accountId &&
      c.actorId &&
      c.issuerId &&
      c.modules.includes('accounting') &&
      c.permissions.includes(`accounting.obligations.${write ? 'manage' : 'view'}`)
    )
  }
  function reset() {
    generation++
    abort.abort()
    abort = new AbortController()
    Object.assign(state, {
      items: [],
      period: '',
      code: '',
      busy: false,
      error: '',
      page: 1,
      more: false,
    })
  }
  async function run(create: boolean, more = false) {
    if (!allowed(create) || state.busy) return
    const c = { ...context() },
      tag = signature(),
      current = generation
    state.busy = true
    state.error = ''
    try {
      const root = `/v1/accounting/issuers/${encodeURIComponent(c.issuerId)}/obligations`
      if (create) {
        await api(c.accountId, root, {
          method: 'POST',
          signal: abort.signal,
          key: key(),
          body: { code: state.code, period: state.period, expectedVersion: 0, base: null },
        })
        if (current !== generation || signature() !== tag) return
      }
      const page = more ? state.page + 1 : 1
      const out = await api<{ items: ObligationView[] }>(
        c.accountId,
        `${root}?page=${page}&limit=20`,
        { method: 'GET', signal: abort.signal },
      )
      if (current !== generation || signature() !== tag) return
      state.items = more ? [...state.items, ...out.items] : out.items
      state.page = page
      state.more = out.items.length === 20
    } catch {
      if (current === generation && signature() === tag)
        state.error =
          'Não foi possível atualizar. Confira o período; uma obrigação já existente deve ser consultada, sem criar outra.'
    } finally {
      if (current === generation && signature() === tag) state.busy = false
    }
  }
  return {
    state,
    allowed,
    reset,
    load: (more = false) => run(false, more),
    create: () => run(true),
  }
}
