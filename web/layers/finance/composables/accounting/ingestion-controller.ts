import { reactive } from 'vue'
export interface IngestionContext {
  accountId: string
  actorId: string
  issuerId: string
  baseId: string
  modules: readonly string[]
  permissions: readonly string[]
}
export interface SourceSpec {
  id: string
  title: string
  url: string
  revision: string
}
export interface SourceSubscription {
  id: string
  baseId: string
  sourceId: string
  version: number
  enabled: boolean
  intervalHours: number
  nextCheckAt: string
}
export interface IngestionRun {
  id: string
  baseId: string
  sourceId: string
  state: string
  documentId: string | null
  errorCode: string | null
  createdAt: string
}
export type IngestionTransport = <T>(
  account: string,
  path: string,
  options: { method: 'GET' | 'PUT'; body?: unknown; key?: string; signal: AbortSignal },
) => Promise<T>
export function createIngestionController(
  context: () => IngestionContext,
  api: IngestionTransport,
  key = () => crypto.randomUUID(),
) {
  const state = reactive({
    catalog: [] as SourceSpec[],
    subscriptions: [] as SourceSubscription[],
    runs: [] as IngestionRun[],
    sourceId: '',
    enabled: false,
    intervalHours: 24,
    busy: false,
    error: '',
    page: 1,
    more: false,
  })
  let generation = 0,
    abort = new AbortController(),
    pending: { body: string; key: string } | null = null
  const signature = () => JSON.stringify(context())
  const allowed = (write = false) => {
    const c = context()
    return !!(
      c.accountId &&
      c.actorId &&
      c.issuerId &&
      c.baseId &&
      c.modules.includes('accounting') &&
      c.permissions.includes(`accounting.knowledge.${write ? 'manage' : 'view'}`)
    )
  }
  function reset() {
    generation++
    abort.abort()
    abort = new AbortController()
    pending = null
    Object.assign(state, {
      catalog: [],
      subscriptions: [],
      runs: [],
      sourceId: '',
      enabled: false,
      intervalHours: 24,
      busy: false,
      error: '',
      page: 1,
      more: false,
    })
  }
  function select(id: string) {
    state.sourceId = id
    const current = state.subscriptions.find(
      (s) => s.baseId === context().baseId && s.sourceId === id,
    )
    state.enabled = current?.enabled || false
    state.intervalHours = current?.intervalHours || 24
  }
  async function execute(save = false, more = false) {
    if (!allowed(save) || state.busy || (save && !state.sourceId)) return
    const ctx = { ...context() },
      token = signature(),
      current = generation,
      root = `/v1/accounting/issuers/${encodeURIComponent(ctx.issuerId)}/knowledge/ingestion`
    state.busy = true
    state.error = ''
    try {
      if (save) {
        const sub = state.subscriptions.find(
          (s) => s.baseId === ctx.baseId && s.sourceId === state.sourceId,
        )
        const body = {
          baseId: ctx.baseId,
          sourceId: state.sourceId,
          expectedVersion: sub?.version || 0,
          enabled: state.enabled,
          intervalHours: state.intervalHours,
        }
        const json = JSON.stringify(body)
        if (!pending || pending.body !== json) pending = { body: json, key: key() }
        await api(ctx.accountId, `${root}/subscriptions`, {
          method: 'PUT',
          body,
          key: pending.key,
          signal: abort.signal,
        })
        if (generation !== current || signature() !== token) return
        pending = null
      }
      const page = more ? state.page + 1 : 1
      const [catalog, subscriptions, runs] = await Promise.all([
        api<SourceSpec[]>(ctx.accountId, `${root}/catalog`, {
          method: 'GET',
          signal: abort.signal,
        }),
        api<SourceSubscription[]>(ctx.accountId, `${root}/subscriptions`, {
          method: 'GET',
          signal: abort.signal,
        }),
        api<{ items: IngestionRun[] }>(ctx.accountId, `${root}/runs?page=${page}&limit=20`, {
          method: 'GET',
          signal: abort.signal,
        }),
      ])
      if (generation !== current || signature() !== token) return
      state.catalog = catalog
      state.subscriptions = subscriptions
      state.runs = more ? [...state.runs, ...runs.items] : runs.items
      state.page = page
      state.more = runs.items.length === 20
    } catch {
      if (generation === current && signature() === token)
        state.error =
          'Não foi possível atualizar. Confira permissões, base ativa e versão dos dados; tente novamente ou atualize a lista.'
    } finally {
      if (generation === current && signature() === token) state.busy = false
    }
  }
  return {
    state,
    allowed,
    reset,
    select,
    load: (more = false) => execute(false, more),
    save: () => execute(true),
  }
}
