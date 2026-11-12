import { computed, ref, watch } from 'vue'
import { useCoreAccountStore } from '../../layers/core/stores/account'
import { createApiRequest, getApiErrorMessage } from '~/utils/api-client'

export interface ScriptRow {
  audio: string
  video: string
}
export interface ScriptDocument {
  id: string
  title: string
  rows: ScriptRow[]
  description: string
  updatedAt: string
  /** Client account the script is about (same organization); empty when none. */
  clientAccountId: string
  clientName: string
  /** Whether users of the client account can read it. */
  clientVisible: boolean
  /** Script owned by another account (agency) and shared with this one: read-only. */
  shared: boolean
  /** Plain-text first rows, only present in list responses (sidebar thumbnail). */
  preview?: ScriptRow[]
}
export interface ScriptDraft {
  title: string
  rows: ScriptRow[]
  description: string
  updatedAt?: string
  clientAccountId: string
  clientVisible: boolean
}
export interface ScriptClient {
  id: string
  name: string
  /** Scripts of this account tagged with the client; the sidebar filter only lists > 0. */
  scriptCount: number
}

export function useScriptsManager() {
  const auth = useAuthStore()
  const account = useCoreAccountStore()
  const api = createApiRequest(useRuntimeConfig(), () => auth.accessToken)
  const documents = ref<ScriptDocument[]>([])
  const loading = ref(false)
  const hasMore = ref(false)
  const opening = ref(false)
  const saving = ref(false)
  const error = ref('')
  // Busca no servidor (título + texto das linhas): a lista é paginada, filtro local perderia itens.
  const search = ref('')
  /** '' = todos, 'none' = sem cliente, id = aquele cliente. */
  const clientFilter = ref('')
  const clients = ref<ScriptClient[]>([])
  const canManage = computed(
    () =>
      auth.role === 'platform_admin' ||
      auth.effectivePermissionKeys.includes('tools.scripts.manage'),
  )
  const canView = computed(
    () => canManage.value || auth.effectivePermissionKeys.includes('tools.scripts.view'),
  )
  const accountId = computed(() => account.activeAccountId || '')
  let loadGeneration = 0
  let requestedSearch = ''
  let requestedClient = ''

  async function load(more = false) {
    const generation = ++loadGeneration
    requestedSearch = search.value.trim()
    requestedClient = clientFilter.value
    if (!more) documents.value = []
    hasMore.value = false
    error.value = ''
    loading.value = false
    if (!accountId.value || !canView.value) return
    loading.value = true
    try {
      const result = (await api('/v1/tools/scripts', {
        headers: { 'X-Account-Id': accountId.value },
        query: {
          offset: more ? documents.value.length : 0,
          q: requestedSearch || undefined,
          client: requestedClient || undefined,
        },
      })) as { data: ScriptDocument[]; hasMore: boolean }
      if (generation === loadGeneration) {
        documents.value = more ? [...documents.value, ...result.data] : result.data
        hasMore.value = result.hasMore
      }
    } catch (err) {
      if (generation === loadGeneration)
        error.value = getApiErrorMessage(err, 'Não foi possível carregar os roteiros.')
    } finally {
      if (generation === loadGeneration) loading.value = false
    }
  }
  // Recarrega só quando o termo efetivo muda; a troca de conta já zera o termo e chama load.
  watch([search, clientFilter], ([value, client]) => {
    if (value.trim() !== requestedSearch || client !== requestedClient) void load()
  })
  // Only an agency account gets options (the API returns [] for client accounts).
  async function loadClients() {
    const scope = accountId.value
    clients.value = []
    if (!scope || !canManage.value) return
    try {
      const result = (await api('/v1/tools/scripts/clients', {
        headers: { 'X-Account-Id': scope },
      })) as { data: ScriptClient[] }
      if (scope !== accountId.value) return
      clients.value = result.data
      const current = result.data.find((client) => client.id === clientFilter.value)
      if (current && !current.scriptCount) clientFilter.value = ''
    } catch {
      // Sem a lista o roteiro só não pode ser marcado; a listagem segue funcionando.
    }
  }
  /**
   * Realtime: refetch one script after a remote change and upsert it in the list (summary
   * from the full document; the thumbnail reads `rows` when there is no `preview`). 404 means
   * it left this account's read scope (deleted, client changed or hidden): drop it.
   */
  async function refreshDocument(id: string) {
    const scope = accountId.value
    if (!scope || !canView.value) return null
    try {
      const result = (await api(`/v1/tools/scripts/${encodeURIComponent(id)}`, {
        headers: { 'X-Account-Id': scope },
        skipLoadingIndicator: true,
      })) as { data: ScriptDocument }
      if (scope !== accountId.value) return null
      const index = documents.value.findIndex((item) => item.id === id)
      if (index >= 0)
        documents.value = documents.value.map((item) => (item.id === id ? result.data : item))
      else documents.value = [result.data, ...documents.value]
      return result.data
    } catch {
      if (scope === accountId.value) dropDocument(id)
      return null
    }
  }
  function dropDocument(id: string) {
    documents.value = documents.value.filter((item) => item.id !== id)
  }
  async function open(id: string) {
    const scope = accountId.value
    if (!scope || !canView.value || opening.value) return null
    opening.value = true
    error.value = ''
    try {
      const result = (await api(`/v1/tools/scripts/${encodeURIComponent(id)}`, {
        headers: { 'X-Account-Id': scope },
      })) as { data: ScriptDocument }
      return scope === accountId.value ? result.data : null
    } catch (err) {
      if (scope === accountId.value)
        error.value = getApiErrorMessage(err, 'Não foi possível abrir o roteiro.')
      return null
    } finally {
      opening.value = false
    }
  }
  async function save(id: string, draft: ScriptDraft) {
    const scope = accountId.value
    if (!scope || !canManage.value || saving.value) return null
    saving.value = true
    error.value = ''
    try {
      const result = (await api(
        id ? `/v1/tools/scripts/${encodeURIComponent(id)}` : '/v1/tools/scripts',
        {
          method: id ? 'PUT' : 'POST',
          body: draft,
          headers: { 'X-Account-Id': scope },
        },
      )) as { data: ScriptDocument }
      if (scope !== accountId.value) return null
      const before = documents.value.find((item) => item.id === result.data.id)
      if ((before?.clientAccountId || '') !== (result.data.clientAccountId || ''))
        void loadClients()
      const index = documents.value.findIndex((item) => item.id === result.data.id)
      if (index < 0) documents.value = [result.data, ...documents.value]
      else
        documents.value = documents.value.map((item) =>
          item.id === result.data.id ? result.data : item,
        )
      return result.data
    } catch (err) {
      if (scope === accountId.value)
        error.value = getApiErrorMessage(err, 'Não foi possível salvar o roteiro.')
      return null
    } finally {
      saving.value = false
    }
  }
  async function remove(id: string) {
    const scope = accountId.value
    if (!scope || !canManage.value || saving.value) return false
    saving.value = true
    error.value = ''
    try {
      await api(`/v1/tools/scripts/${encodeURIComponent(id)}`, {
        method: 'DELETE',
        headers: { 'X-Account-Id': scope },
      })
      if (scope === accountId.value) {
        if (documents.value.find((item) => item.id === id)?.clientAccountId) void loadClients()
        documents.value = documents.value.filter((item) => item.id !== id)
      }
      return scope === accountId.value
    } catch (err) {
      if (scope === accountId.value)
        error.value = getApiErrorMessage(err, 'Não foi possível excluir o roteiro.')
      return false
    } finally {
      saving.value = false
    }
  }
  async function reorder(ids: string[]) {
    const scope = accountId.value
    if (!scope || !canManage.value || saving.value) return false
    saving.value = true
    error.value = ''
    try {
      await api('/v1/tools/scripts/order', {
        method: 'PUT',
        body: { ids },
        headers: { 'X-Account-Id': scope },
      })
      if (scope !== accountId.value) return false
      const rank = new Map(ids.map((id, index) => [id, index]))
      documents.value = [...documents.value].sort(
        (a, b) => (rank.get(a.id) ?? ids.length) - (rank.get(b.id) ?? ids.length),
      )
      return true
    } catch (err) {
      if (scope === accountId.value)
        error.value = getApiErrorMessage(err, 'Não foi possível reordenar os documentos.')
      return false
    } finally {
      saving.value = false
    }
  }
  return {
    refreshDocument,
    dropDocument,
    search,
    clientFilter,
    clients,
    loadClients,
    reorder,
    hasMore,
    opening,
    open,
    canManage,
    canView,
    documents,
    accountId,
    loading,
    saving,
    error,
    load,
    save,
    remove,
  }
}
