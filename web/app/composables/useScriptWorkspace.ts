import { computed, onBeforeUnmount, ref, watch } from 'vue'
import { useScriptsRealtime } from '~/composables/useScriptsRealtime'
import {
  useScriptsManager,
  type ScriptDocument,
  type ScriptDraft,
} from '~/composables/useScriptsManager'

export function useScriptWorkspace() {
  const manager = useScriptsManager()
  const ui = useUiStore()
  const selectedId = ref('')
  const editing = ref(false)
  const draft = ref<ScriptDraft>({
    title: 'Novo roteiro',
    rows: [{ audio: '', video: '' }],
    description: '',
    clientAccountId: '',
    clientVisible: true,
  })
  /** Shared scripts (owned by the agency, visible to this client) are read-only. */
  const shared = ref(false)
  const canEdit = computed(() => manager.canManage.value && !shared.value)
  const snapshot = ref(JSON.stringify(draft.value))
  const dirty = computed(() => editing.value && JSON.stringify(draft.value) !== snapshot.value)
  const drafts = new Map<string, { id: string; draft: ScriptDraft; snapshot: string }>()
  let timer: ReturnType<typeof setTimeout> | undefined
  let inFlight: Promise<boolean> | undefined
  function hydrate(document: ScriptDocument) {
    selectedId.value = document.id
    draft.value = {
      title: document.title,
      rows: document.rows.map((row) => ({ ...row })),
      description: document.description,
      updatedAt: document.updatedAt,
      clientAccountId: document.clientAccountId || '',
      clientVisible: document.clientVisible ?? true,
    }
    shared.value = Boolean(document.shared)
    snapshot.value = JSON.stringify(draft.value)
    editing.value = true
  }
  async function persist() {
    clearTimeout(timer)
    if (inFlight) {
      await inFlight
      if (dirty.value) return saveNow()
      return true
    }
    const scope = manager.accountId.value
    const id = selectedId.value
    while (dirty.value && scope === manager.accountId.value && id === selectedId.value) {
      const before = JSON.stringify(draft.value)
      const result = await manager.save(id, JSON.parse(before) as ScriptDraft)
      if (!result || scope !== manager.accountId.value) return false
      const unchanged = JSON.stringify(draft.value) === before
      if (!id) selectedId.value = result.id
      const authoritative = {
        title: result.title,
        rows: result.rows,
        description: result.description,
        updatedAt: result.updatedAt,
        clientAccountId: result.clientAccountId || '',
        clientVisible: result.clientVisible ?? true,
      }
      snapshot.value = JSON.stringify(authoritative)
      if (unchanged) {
        // Preserve row/editor identity and focus across autosaves.
        draft.value.title = result.title
        draft.value.description = result.description
        draft.value.clientAccountId = authoritative.clientAccountId
        draft.value.clientVisible = authoritative.clientVisible
        if (JSON.stringify(draft.value.rows) !== JSON.stringify(result.rows))
          draft.value.rows = result.rows
        draft.value.updatedAt = result.updatedAt
      } else draft.value.updatedAt = result.updatedAt
      if (!id) break
    }
    return !dirty.value
  }
  async function saveNow(): Promise<boolean> {
    if (!canEdit.value || !dirty.value) return !dirty.value
    if (inFlight) return inFlight
    inFlight = persist()
    let success = false
    try {
      success = await inFlight
      return success
    } finally {
      inFlight = undefined
      if (success && dirty.value) scheduleSave()
    }
  }
  function scheduleSave() {
    clearTimeout(timer)
    if (dirty.value && canEdit.value)
      timer = setTimeout(() => {
        void saveNow()
      }, 400)
  }
  watch(draft, scheduleSave, { deep: true })
  async function maySwitch() {
    await saveNow()
    if (!dirty.value) return true
    const result = await ui.confirm({
      title: 'Alterações não salvas',
      message: 'Não foi possível salvar. Descartar as alterações deste roteiro?',
      confirmLabel: 'Descartar',
    })
    return Boolean(
      result && typeof result === 'object' && 'confirmed' in result && result.confirmed,
    )
  }
  async function select(document: ScriptDocument) {
    if (manager.opening.value || !(await maySwitch())) return
    const result = await manager.open(document.id)
    if (result) hydrate(result)
  }
  async function create() {
    if (!manager.canManage.value || manager.saving.value || !(await maySwitch())) return
    // Com um cliente filtrado, o roteiro novo já nasce marcado com ele.
    const filtered = manager.clients.value.some((c) => c.id === manager.clientFilter.value)
    const result = await manager.save('', {
      title: 'Novo roteiro',
      rows: [{ audio: '', video: '' }],
      description: '',
      clientAccountId: filtered ? manager.clientFilter.value : '',
      clientVisible: true,
    })
    if (result) hydrate(result)
  }
  async function remove(id: string) {
    if (!id || manager.saving.value) return
    const result = await ui.confirm({
      title: 'Excluir documento',
      message: 'Excluir este documento? Esta ação não pode ser desfeita.',
      confirmLabel: 'Excluir',
    })
    if (!result || typeof result !== 'object' || !('confirmed' in result) || !result.confirmed)
      return
    clearTimeout(timer)
    localDeletes.add(id)
    if (!(await manager.remove(id))) {
      localDeletes.delete(id)
      return
    }
    if (selectedId.value === id) {
      editing.value = false
      selectedId.value = ''
      const first = manager.documents.value[0]
      if (first) {
        const loaded = await manager.open(first.id)
        if (loaded) hydrate(loaded)
      }
    }
  }
  function moveRow(from: number, to: number) {
    const rows = [...draft.value.rows]
    const [row] = rows.splice(from, 1)
    if (row) rows.splice(to, 0, row)
    draft.value.rows = rows
  }
  async function moveDocument(from: number, to: number) {
    if (!(await maySwitch())) return
    const list = [...manager.documents.value]
    const [item] = list.splice(from, 1)
    if (item) list.splice(to, 0, item)
    // Roteiros compartilhados pela agência não entram na ordem desta conta.
    localReorderAt = Date.now()
    await manager.reorder(
      list.filter((document) => !document.shared).map((document) => document.id),
    )
  }
  watch(
    [manager.accountId, manager.canView],
    async ([scope], previous) => {
      clearTimeout(timer)
      if (previous?.[0] && dirty.value)
        drafts.set(previous[0], {
          id: selectedId.value,
          draft: JSON.parse(JSON.stringify(draft.value)),
          snapshot: snapshot.value,
        })
      editing.value = false
      selectedId.value = ''
      shared.value = false
      manager.search.value = ''
      manager.clientFilter.value = ''
      void manager.loadClients()
      await manager.load()
      if (scope !== manager.accountId.value) return
      const pending = drafts.get(scope)
      if (pending && manager.canView.value) {
        selectedId.value = pending.id
        draft.value = pending.draft
        snapshot.value = pending.snapshot
        editing.value = true
        drafts.delete(scope)
        return
      }
      const first = manager.documents.value[0]
      if (first) {
        const result = await manager.open(first.id)
        if (result) hydrate(result)
      }
    },
    { immediate: true },
  )
  // Realtime (versão simples): o evento invalida, a API é a fonte. Ecos da própria aba são
  // ignorados comparando updatedAt; com edição local pendente, só avisa (sem sobrescrever).
  const remoteNotice = ref<'' | 'changed' | 'deleted'>('')
  const localDeletes = new Set<string>()
  let localReorderAt = 0
  let filteredReload: ReturnType<typeof setTimeout> | undefined
  const sameTime = (a?: string, b?: string) => Boolean(a && b && Date.parse(a) === Date.parse(b))
  async function reloadOpened() {
    const result = await manager.open(selectedId.value)
    if (result) hydrate(result)
    remoteNotice.value = ''
  }
  async function remoteSaved(id: string, updatedAt: string) {
    if (manager.search.value.trim() || manager.clientFilter.value) {
      // Com filtro, a API decide se o roteiro ainda entra na lista.
      clearTimeout(filteredReload)
      filteredReload = setTimeout(() => void manager.load(), 500)
    } else {
      const known = manager.documents.value.find((item) => item.id === id)
      if (!sameTime(known?.updatedAt, updatedAt)) await manager.refreshDocument(id)
    }
    if (id !== selectedId.value || manager.saving.value || inFlight) return
    if (sameTime(draft.value.updatedAt, updatedAt)) return
    if (dirty.value) remoteNotice.value = 'changed'
    else await reloadOpened()
  }
  function remoteDeleted(id: string) {
    if (localDeletes.delete(id)) return
    manager.dropDocument(id)
    if (id !== selectedId.value) return
    clearTimeout(timer)
    editing.value = false
    selectedId.value = ''
    remoteNotice.value = 'deleted'
  }
  useScriptsRealtime({
    enabled: () => manager.canView.value,
    accountId: () => manager.accountId.value,
    onSaved: (id, updatedAt) => void remoteSaved(id, updatedAt),
    onDeleted: remoteDeleted,
    onReordered: () => {
      if (Date.now() - localReorderAt > 3000) void manager.load()
    },
  })
  watch(selectedId, (id) => {
    if (id) remoteNotice.value = ''
  })
  onBeforeUnmount(() => {
    clearTimeout(timer)
    clearTimeout(filteredReload)
  })
  return {
    remoteNotice,
    reloadOpened,
    ...manager,
    selectedId,
    shared,
    canEdit,
    editing,
    draft,
    dirty,
    saveNow,
    maySwitch,
    select,
    create,
    removeDocument: remove,
    moveRow,
    moveDocument,
    hasOtherDrafts: () => drafts.size > 0,
  }
}
