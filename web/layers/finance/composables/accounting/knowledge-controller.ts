import { reactive } from 'vue'
import type {
  KnowledgeBase,
  KnowledgeDocument,
  KnowledgeMetadata,
  KnowledgeResult,
} from '../../types/knowledge'
export interface KnowledgeContext {
  accountId: string
  actorId: string
  issuerId: string
  modules: readonly string[]
  permissions: readonly string[]
}
export type KnowledgeTransport = <T>(
  account: string,
  path: string,
  options: { method?: 'GET' | 'POST' | 'PUT' | 'PATCH'; body?: unknown; signal: AbortSignal },
) => Promise<T>
export function createKnowledgeController(
  context: () => KnowledgeContext,
  transport: KnowledgeTransport,
) {
  const state = reactive({
    bases: [] as KnowledgeBase[],
    baseId: '',
    documents: [] as KnowledgeDocument[],
    result: null as KnowledgeResult | null,
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
      c.permissions.includes(`accounting.knowledge.${write ? 'manage' : 'view'}`)
    )
  }
  function reset() {
    generation++
    abort.abort()
    abort = new AbortController()
    Object.assign(state, {
      bases: [],
      baseId: '',
      documents: [],
      result: null,
      busy: false,
      error: '',
      page: 1,
      more: false,
    })
  }
  async function run<T>(
    work: (
      request: <V>(
        path: string,
        method?: 'GET' | 'POST' | 'PUT' | 'PATCH',
        body?: unknown,
      ) => Promise<V>,
    ) => Promise<T>,
    apply: (value: T) => void,
    write = false,
  ) {
    if (state.busy || !allowed(write)) return false
    const ticket = generation,
      origin = signature(),
      c = context(),
      signal = abort.signal
    const request = <V>(
      path: string,
      method: 'GET' | 'POST' | 'PUT' | 'PATCH' = 'GET',
      body?: unknown,
    ) => {
      if (ticket !== generation || signature() !== origin || signal.aborted)
        throw new Error('context_changed')
      return transport<V>(
        c.accountId,
        `/v1/accounting/issuers/${encodeURIComponent(c.issuerId)}/knowledge${path}`,
        { method, body, signal },
      )
    }
    state.busy = true
    state.error = ''
    try {
      const value = await work(request)
      if (ticket !== generation || signature() !== origin) return false
      apply(value)
      return true
    } catch {
      if (ticket === generation && signature() === origin)
        state.error =
          'Operação não concluída. Confira permissões, origem, armazenamento e versão; recarregue antes de repetir a publicação.'
      return false
    } finally {
      if (ticket === generation) state.busy = false
    }
  }
  async function load() {
    return run(
      (r) => r<KnowledgeBase[]>('/bases'),
      (value) => {
        state.bases = value
      },
    )
  }
  async function choose(base: string) {
    state.baseId = base
    state.documents = []
    state.page = 1
    state.more = false
    if (!base) return false
    return run(
      (r) => r<KnowledgeDocument[]>(`/bases/${encodeURIComponent(base)}/documents?page=1&limit=20`),
      (value) => {
        if (state.baseId === base) {
          state.documents = value
          state.more = value.length === 20
        }
      },
    )
  }
  async function next() {
    if (!state.more || !state.baseId) return false
    const base = state.baseId,
      page = state.page + 1
    return run(
      (r) =>
        r<KnowledgeDocument[]>(
          `/bases/${encodeURIComponent(base)}/documents?page=${page}&limit=20`,
        ),
      (value) => {
        if (state.baseId === base) {
          state.documents.push(...value)
          state.page = page
          state.more = value.length === 20
        }
      },
    )
  }
  async function createBase(name: string) {
    return run(
      (r) => r<KnowledgeBase>('/bases', 'POST', { name }),
      (value) => {
        state.bases.push(value)
      },
      true,
    )
  }
  async function toggle(base: KnowledgeBase) {
    return run(
      (r) =>
        r<KnowledgeBase>(`/bases/${base.id}`, 'PATCH', {
          expectedVersion: base.version,
          enabled: !base.enabled,
        }),
      (value) => {
        const index = state.bases.findIndex((v) => v.id === value.id)
        if (index >= 0) state.bases[index] = value
      },
      true,
    )
  }
  const save = (value: KnowledgeDocument) => {
    const index = state.documents.findIndex((v) => v.id === value.id)
    if (index >= 0) state.documents[index] = value
    else state.documents.unshift(value)
  }
  async function ingest(input: {
    title: string
    sourceRef: string
    fileName: string
    contentType: string
    content: string
    metadata: KnowledgeMetadata
  }) {
    if (!state.baseId) return false
    return run(
      (r) => r<KnowledgeDocument>('/imports', 'POST', { ...input, baseId: state.baseId }),
      save,
      true,
    )
  }
  async function review(doc: KnowledgeDocument, metadata: KnowledgeMetadata) {
    return run(
      (r) =>
        r<KnowledgeDocument>(`/documents/${doc.id}/review`, 'PUT', {
          expectedVersion: doc.lockVersion,
          metadata,
          confirmed: true,
        }),
      save,
      true,
    )
  }
  async function publish(doc: KnowledgeDocument) {
    return run(
      (r) =>
        r<KnowledgeDocument>(`/documents/${doc.id}/publish`, 'POST', {
          expectedVersion: doc.lockVersion,
          confirmed: true,
        }),
      save,
      true,
    )
  }
  async function deactivate(doc: KnowledgeDocument) {
    return run(
      (r) =>
        r<KnowledgeDocument>(`/documents/${doc.id}/deactivate`, 'POST', {
          expectedVersion: doc.lockVersion,
        }),
      save,
      true,
    )
  }
  async function search(
    query: string,
    competenceDate: string,
    jurisdiction: string,
    regime: string,
  ) {
    return run(
      (r) =>
        r<KnowledgeResult>('/search', 'POST', {
          query,
          competenceDate,
          jurisdiction,
          regime,
          topK: 5,
        }),
      (value) => {
        state.result = value
      },
    )
  }
  return {
    state,
    allowed,
    reset,
    load,
    choose,
    next,
    createBase,
    toggle,
    ingest,
    review,
    publish,
    deactivate,
    search,
  }
}
