import { computed, onBeforeUnmount, ref, watch } from 'vue'
import { useScriptsRealtime } from '~/composables/useScriptsRealtime'
import { scriptCellKey, useScriptPresence } from '~/composables/useScriptPresence'
import {
  newScriptRow,
  useScriptsManager,
  type ScriptCellField,
  type ScriptDocument,
  type ScriptDraft,
  type ScriptRow,
} from '~/composables/useScriptsManager'

// Edição POR CÉLULA (nível intermediário, ver tools/AGENT.md "Implementações futuras"):
// - cada célula (título, observações, áudio/vídeo de uma linha) salva sozinha via PATCH,
//   com debounce de 400 ms, e é transmitida pronta às outras telas (sem refetch);
// - estrutura (ordem/linhas novas/removidas e cliente) salva à parte via PUT, que nunca
//   sobrescreve conteúdo de célula;
// - travas de presença: quem foca uma célula a trava; os outros veem "Fulano editando" e
//   não conseguem editar, remover a linha ou excluir o roteiro (o servidor confere também).

const CELL_SAVE_MS = 400
const STRUCTURE_SAVE_MS = 300

export function useScriptWorkspace() {
  const manager = useScriptsManager()
  const ui = useUiStore()
  const selectedId = ref('')
  const editing = ref(false)
  const draft = ref<ScriptDraft>({
    title: 'Novo roteiro',
    rows: [newScriptRow()],
    description: '',
    clientAccountId: '',
    clientVisible: true,
  })
  /** Shared scripts (owned by the agency, visible to this client) are read-only. */
  const shared = ref(false)
  const canEdit = computed(() => manager.canManage.value && !shared.value)
  const presence = useScriptPresence({
    enabled: () => manager.canView.value && editing.value,
    accountId: () => manager.accountId.value,
    scriptId: () => selectedId.value,
  })
  const lockedBy = (field: ScriptCellField, rowId = '') =>
    presence.lockers.value[scriptCellKey(field, rowId)] || ''

  // ---------- per-cell saves ----------
  const pendingCells = new Map<
    string,
    { field: ScriptCellField; rowId: string; timer: ReturnType<typeof setTimeout> }
  >()
  let cellsInFlight = 0
  const savingCells = ref(false)
  function readCell(field: ScriptCellField, rowId: string) {
    if (field === 'title' || field === 'description') return draft.value[field]
    const row = draft.value.rows.find((item) => item.id === rowId)
    return row ? row[field] : undefined
  }
  function writeCell(field: ScriptCellField, rowId: string, value: string) {
    if (field === 'title' || field === 'description') draft.value[field] = value
    else {
      const row = draft.value.rows.find((item) => item.id === rowId)
      if (row) row[field] = value
    }
  }
  /** Called by the editor for every local change of one cell. */
  function editCell(field: ScriptCellField, rowId: string, value: string) {
    if (!canEdit.value || readCell(field, rowId) === value) return
    writeCell(field, rowId, value)
    const key = scriptCellKey(field, rowId)
    clearTimeout(pendingCells.get(key)?.timer)
    pendingCells.set(key, {
      field,
      rowId,
      timer: setTimeout(() => void flushCell(key), CELL_SAVE_MS),
    })
    savingCells.value = true
  }
  async function flushCell(key: string) {
    const pending = pendingCells.get(key)
    if (!pending) return true
    clearTimeout(pending.timer)
    pendingCells.delete(key)
    const id = selectedId.value
    const value = readCell(pending.field, pending.rowId)
    if (!id || value === undefined) return true
    cellsInFlight++
    try {
      const updatedAt = await manager.patchCell(id, pending.field, pending.rowId, value)
      if (!updatedAt) return false
      if (id === selectedId.value) draft.value.updatedAt = updatedAt
      patchListCard(id, pending.field, pending.rowId, value, updatedAt)
      return true
    } finally {
      cellsInFlight--
      savingCells.value = pendingCells.size > 0 || cellsInFlight > 0
    }
  }
  async function flushCells() {
    const results = await Promise.all([...pendingCells.keys()].map((key) => flushCell(key)))
    return results.every(Boolean)
  }

  // ---------- structure saves (rows order/add/remove, client) ----------
  let structureTimer: ReturnType<typeof setTimeout> | undefined
  let applyingRemote = false
  const structureKey = computed(() =>
    JSON.stringify([
      draft.value.rows.map((row) => row.id),
      draft.value.clientAccountId,
      draft.value.clientVisible,
    ]),
  )
  watch(structureKey, (_next, previous) => {
    if (!editing.value || !canEdit.value || applyingRemote || previous === undefined) return
    clearTimeout(structureTimer)
    structureTimer = setTimeout(() => void saveStructure(), STRUCTURE_SAVE_MS)
  })
  async function saveStructure() {
    clearTimeout(structureTimer)
    structureTimer = undefined
    const id = selectedId.value
    if (!id || !canEdit.value) return true
    if (manager.saving.value) {
      // Another save is running (manager.save would skip): retry instead of reverting.
      structureTimer = setTimeout(() => void saveStructure(), STRUCTURE_SAVE_MS)
      return true
    }
    const result = await manager.save(id, JSON.parse(JSON.stringify(draft.value)) as ScriptDraft)
    if (!result) {
      // Refused (e.g. removing a row someone is editing): go back to the stored structure.
      const stored = await manager.refreshDocument(id)
      if (stored && id === selectedId.value) applyStructure(stored)
      return false
    }
    if (id === selectedId.value) draft.value.updatedAt = result.updatedAt
    return true
  }

  const dirty = computed(() => savingCells.value || manager.saving.value)
  async function saveNow() {
    const cells = await flushCells()
    const structure = structureTimer ? await saveStructure() : true
    return cells && structure
  }
  async function maySwitch() {
    presence.blur()
    if (await saveNow()) return true
    const result = await ui.confirm({
      title: 'Alterações não salvas',
      message: 'Não foi possível salvar. Descartar as alterações deste roteiro?',
      confirmLabel: 'Descartar',
    })
    const discard = Boolean(
      result && typeof result === 'object' && 'confirmed' in result && result.confirmed,
    )
    if (discard) dropPendingCells()
    return discard
  }
  function dropPendingCells() {
    pendingCells.forEach((pending) => clearTimeout(pending.timer))
    pendingCells.clear()
    savingCells.value = cellsInFlight > 0
  }

  // ---------- open / create / delete ----------
  function hydrate(document: ScriptDocument) {
    applyingRemote = true
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
    editing.value = true
    void Promise.resolve().then(() => (applyingRemote = false))
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
      rows: [newScriptRow()],
      description: '',
      clientAccountId: filtered ? manager.clientFilter.value : '',
      clientVisible: true,
    })
    if (result) hydrate(result)
  }
  async function remove(id: string) {
    if (!id || manager.saving.value) return
    if (id === selectedId.value && presence.editors.value.length) {
      ui.error(`${presence.editors.value[0]} está editando este roteiro. Exclua quando terminar.`)
      return
    }
    const result = await ui.confirm({
      title: 'Excluir documento',
      message: 'Excluir este documento? Esta ação não pode ser desfeita.',
      confirmLabel: 'Excluir',
    })
    if (!result || typeof result !== 'object' || !('confirmed' in result) || !result.confirmed)
      return
    localDeletes.add(id)
    if (!(await manager.remove(id))) {
      localDeletes.delete(id)
      return
    }
    if (selectedId.value === id) {
      dropPendingCells()
      editing.value = false
      selectedId.value = ''
      const first = manager.documents.value[0]
      if (first) {
        const loaded = await manager.open(first.id)
        if (loaded) hydrate(loaded)
      }
    }
  }

  // ---------- rows ----------
  function addRow() {
    if (canEdit.value && draft.value.rows.length < 200) draft.value.rows.push(newScriptRow())
  }
  function removeRow(row: ScriptRow) {
    const holder = presence.rowLocker(row.id)
    if (holder) {
      ui.error(`${holder} está editando esta linha.`)
      return
    }
    draft.value.rows = draft.value.rows.filter((item) => item !== row)
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
    async ([scope]) => {
      await flushCells()
      editing.value = false
      selectedId.value = ''
      shared.value = false
      manager.search.value = ''
      manager.clientFilter.value = ''
      void manager.loadClients()
      await manager.load()
      if (scope !== manager.accountId.value) return
      const first = manager.documents.value[0]
      if (first) {
        const result = await manager.open(first.id)
        if (result) hydrate(result)
      }
    },
    { immediate: true },
  )

  // ---------- realtime ----------
  const remoteNotice = ref<'' | 'deleted'>('')
  const localDeletes = new Set<string>()
  let localReorderAt = 0
  let filteredReload: ReturnType<typeof setTimeout> | undefined
  const sameTime = (a?: string, b?: string) => Boolean(a && b && Date.parse(a) === Date.parse(b))
  /** Keeps the sidebar card in sync with one cell, without fetching. */
  function patchListCard(
    id: string,
    field: ScriptCellField,
    rowId: string,
    value: string,
    updatedAt: string,
  ) {
    const card = manager.documents.value.find((item) => item.id === id)
    if (!card) return
    card.updatedAt = updatedAt
    if (field === 'title') card.title = value
    if (field !== 'audio' && field !== 'video') return
    const rows = card.preview?.length ? card.preview : card.rows
    const row = rows?.find((item) => item.id === rowId)
    if (row) row[field] = value
  }
  /** Remote structure (row order/add/remove, client): reuse row objects by id, no rebuild. */
  function applyStructure(document: ScriptDocument) {
    applyingRemote = true
    const byId = new Map(draft.value.rows.map((row) => [row.id, row]))
    draft.value.rows = document.rows.map((row) => {
      const current = byId.get(row.id)
      if (!current) return { ...row }
      for (const field of ['audio', 'video'] as const)
        if (!pendingCells.has(scriptCellKey(field, row.id)) && current[field] !== row[field])
          current[field] = row[field]
      return current
    })
    draft.value.clientAccountId = document.clientAccountId || ''
    draft.value.clientVisible = document.clientVisible ?? true
    draft.value.updatedAt = document.updatedAt
    shared.value = Boolean(document.shared)
    void Promise.resolve().then(() => (applyingRemote = false))
  }
  function remoteCell(
    id: string,
    field: ScriptCellField,
    rowId: string,
    value: string,
    updatedAt: string,
  ) {
    patchListCard(id, field, rowId, value, updatedAt)
    if (id !== selectedId.value) return
    const key = scriptCellKey(field, rowId)
    // My own pending edit or the cell I am typing in wins locally (my save comes next).
    if (pendingCells.has(key) || presence.activeFieldKey.value === key) return
    writeCell(field, rowId, value)
    draft.value.updatedAt = updatedAt
  }
  async function remoteSaved(id: string, updatedAt: string) {
    if (manager.search.value.trim() || manager.clientFilter.value) {
      // Com filtro, a API decide se o roteiro ainda entra na lista.
      clearTimeout(filteredReload)
      filteredReload = setTimeout(() => void manager.load(), 500)
      if (id !== selectedId.value) return
    }
    const known = manager.documents.value.find((item) => item.id === id)
    const opened = id === selectedId.value
    if (sameTime(opened ? draft.value.updatedAt : known?.updatedAt, updatedAt)) return
    const document = await manager.refreshDocument(id)
    if (document && opened && id === selectedId.value) applyStructure(document)
  }
  function remoteDeleted(id: string) {
    if (localDeletes.delete(id)) return
    manager.dropDocument(id)
    if (id !== selectedId.value) return
    dropPendingCells()
    editing.value = false
    selectedId.value = ''
    remoteNotice.value = 'deleted'
  }
  useScriptsRealtime({
    enabled: () => manager.canView.value,
    accountId: () => manager.accountId.value,
    onCell: remoteCell,
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
    void flushCells()
    clearTimeout(structureTimer)
    clearTimeout(filteredReload)
  })
  return {
    ...manager,
    remoteNotice,
    presence,
    lockedBy,
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
    editCell,
    addRow,
    removeRow,
    removeDocument: remove,
    moveRow,
    moveDocument,
  }
}
