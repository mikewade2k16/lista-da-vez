// Editor da planilha financeira ativa: draft + autosave + linhas + efetivacao +
// grupos de recorrencia. Recebe o estado de configuracao (configDraft +
// clientRecurringEntries) para sincronizar linhas fixas/recorrentes.
import type { FinanceLineItem, FinanceSheetItem } from '../types/finances'
import { normalizeFinanceLinkedUuid } from '../utils/finance-ids'
import {
  applySnapshotLine,
  makeLine,
  snapshotLine,
  currentPeriod,
  SHEET_AUTOSAVE_DEBOUNCE_MS,
} from '../utils/finance-helpers'
import { useFinanceSheetSources, type SheetEditorDeps } from './useFinanceSheetSources'
import { useFinanceLineEditor } from './useFinanceLineEditor'
import { useFinanceEffectiveEditor } from './useFinanceEffectiveEditor'
import { useFinanceAutosave } from './useFinanceAutosave'
import { useAuthStore } from '~/stores/auth'
import { useCoreAccountStore } from '../../core/stores/account'

export function useFinanceSheetEditor(deps: SheetEditorDeps) {
  const coreAccount = useCoreAccountStore()
  const {
    sheets,
    activeSheet,
    detailLoading,
    creating,
    deletingId,
    errorMessage,
    fetchSheets,
    fetchSheetDetail,
    createSheet,
    updateSheet,
    changeSheetStatus,
    updateSheetLine,
    deleteSheet,
  } = useFinancesManager()

  const targetCoreTenantId = computed(() => String(coreAccount.activeAccountId || '').trim())
  const selectedSheetId = ref<string | null>(null)
  const auth = useAuthStore()
  const canManageSheets = computed(
    () =>
      auth.role === 'platform_admin' ||
      (auth.role === 'owner' && coreAccount.permissions.includes('core.account.manage')) ||
      coreAccount.permissions.includes('finance.sheets.manage'),
  )
  const isClosed = computed(() => activeSheet.value?.status === 'fechada')
  const autosave = useFinanceAutosave(SHEET_AUTOSAVE_DEBOUNCE_MS)
  const savedRevisions = new Map<string, string>()

  const draft = reactive<{
    title: string
    period: string
    status: string
    notes: string
    entradas: FinanceLineItem[]
    saidas: FinanceLineItem[]
  }>({
    title: '',
    period: '',
    status: 'aberta',
    notes: '',
    entradas: [],
    saidas: [],
  })

  const filteredSheets = computed(() => sheets.value)

  const sources = useFinanceSheetSources({ ...deps, draft, selectedSheetId, queuePersist })
  const {
    categoryOptions,
    resolveFixedById,
    recurringEntryForTenant,
    entradaDisplayItems,
    recurringGroupByRowId,
    ensureRows,
    syncAllFixedRows,
  } = sources

  function hydrate(sheet: FinanceSheetItem | null) {
    if (!sheet) {
      draft.title = ''
      draft.period = currentPeriod()
      draft.status = 'aberta'
      draft.notes = ''
      draft.entradas = [makeLine()]
      draft.saidas = [makeLine()]
      return
    }
    draft.title = sheet.title || ''
    draft.period = sheet.period || currentPeriod()
    draft.status = sheet.status || 'aberta'
    draft.notes = sheet.notes || ''
    const mapRow = (row: FinanceLineItem) => ({
      ...row,
      adjustments: Array.isArray(row.adjustments) ? row.adjustments.map((a) => ({ ...a })) : [],
      fixedAccountId: normalizeFinanceLinkedUuid(row.fixedAccountId),
      details: row.details || '',
    })
    draft.entradas = (sheet.entradas || []).map(mapRow)
    draft.saidas = (sheet.saidas || []).map(mapRow)
    ensureRows()
  }

  async function refreshActiveSheetDraft() {
    const id = String(selectedSheetId.value || '')
      .trim()
      .toLowerCase()
    if (!id) return
    const detail = await fetchSheetDetail(id)
    if (selectedSheetId.value !== id || !detail) return
    hydrate(detail)
  }

  // ---- persist (autosave) ----
  function buildSheetPersistPayload() {
    return {
      title: draft.title,
      period: draft.period,
      status: draft.status,
      notes: draft.notes,
      coreTenantId: targetCoreTenantId.value || activeSheet.value?.coreTenantId,
      entradas: draft.entradas.map(snapshotLine),
      saidas: draft.saidas.map(snapshotLine),
    }
  }

  function queuePersist(delayMs = SHEET_AUTOSAVE_DEBOUNCE_MS) {
    const id = selectedSheetId.value
    const accountId = coreAccount.activeAccountId
    if (!id || !accountId || !canManageSheets.value || isClosed.value) return
    const key = `${accountId}:${id}`
    const payload = structuredClone(buildSheetPersistPayload())
    const revision = activeSheet.value?.updatedAt
    autosave.schedule(
      key,
      async () => {
        const saved = await updateSheet(
          id,
          { ...payload, expectedUpdatedAt: savedRevisions.get(key) || revision },
          accountId,
        )
        if (!saved) throw new Error('finance_save_failed')
        savedRevisions.set(key, saved.updatedAt)
        if (
          selectedSheetId.value === id &&
          coreAccount.activeAccountId === accountId &&
          !autosave.has(key)
        )
          hydrate(saved)
      },
      typeof delayMs === 'number' ? delayMs : SHEET_AUTOSAVE_DEBOUNCE_MS,
    )
  }

  async function onStatusChange(value: unknown) {
    const id = selectedSheetId.value
    const accountId = coreAccount.activeAccountId
    if (!id || !canManageSheets.value || !(await autosave.flush())) return
    if (id !== selectedSheetId.value || accountId !== coreAccount.activeAccountId) return
    try {
      const saved = await changeSheetStatus(id, String(value))
      savedRevisions.set(`${coreAccount.activeAccountId}:${id}`, saved.updatedAt)
      hydrate(saved)
    } catch {
      errorMessage.value =
        'Não foi possível alterar o status. Recarregue a planilha e tente novamente.'
    }
  }

  // ---- line effective ----
  async function persistLineEffective(row: FinanceLineItem, previous: FinanceLineItem) {
    const id = selectedSheetId.value
    const accountId = coreAccount.activeAccountId
    if (!id || !row.id || !canManageSheets.value || isClosed.value) return false
    const stored = [
      ...(activeSheet.value?.entradas || []),
      ...(activeSheet.value?.saidas || []),
    ].find((item) => item.id === row.id)
    const rollback = stored ? snapshotLine(stored) : previous
    const desired = { effective: row.effective, effectiveDate: row.effectiveDate }
    if (!(await autosave.flush())) {
      applySnapshotLine(row, rollback)
      return false
    }
    if (selectedSheetId.value !== id || accountId !== coreAccount.activeAccountId) return false
    const key = `${accountId}:${id}`
    let saved = false
    autosave.schedule(
      `${key}:line:${row.id}`,
      async () => {
        const response = await updateSheetLine(id, row.id, desired, accountId)
        if (!response) throw new Error('finance_line_save_failed')
        savedRevisions.set(key, response.updatedAt)
        if (selectedSheetId.value === id && accountId === coreAccount.activeAccountId) {
          const current = [...draft.entradas, ...draft.saidas].find((item) => item.id === row.id)
          if (current) applySnapshotLine(current, response.line)
        }
        saved = true
      },
      0,
    )
    await autosave.flush()
    if (!saved) applySnapshotLine(row, rollback)
    return saved
  }

  const lineEditor = useFinanceLineEditor(queuePersist)
  const { lineScopedKey } = lineEditor

  const effectiveEditor = useFinanceEffectiveEditor({
    lineScopedKey,
    persistLineEffective,
    refreshActiveSheetDraft,
  })

  // ---- sheet CRUD (UI) ----
  function addLine(kind: 'entrada' | 'saida') {
    const list = kind === 'entrada' ? draft.entradas : draft.saidas
    list.push(makeLine())
    queuePersist()
  }
  function removeRow(kind: 'entrada' | 'saida', index: number) {
    const list = kind === 'entrada' ? draft.entradas : draft.saidas
    if (index < 0 || index >= list.length) return
    list.splice(index, 1)
    if (list.length === 0) list.push(makeLine())
    queuePersist()
  }
  async function onCreateSheet() {
    if (!canManageSheets.value || !(await autosave.flush())) return
    const created = await createSheet({
      title: `Finance ${currentPeriod()}`,
      period: currentPeriod(),
      status: 'aberta',
      coreTenantId: targetCoreTenantId.value,
    })
    if (!created) return
    selectedSheetId.value = created.id
    // A seleção deve terminar de hidratar antes de inicializar contas/recorrências.
    await nextTick()
    hydrate(created)
    syncAllFixedRows(true)
  }
  async function onDeleteSheet(id: string) {
    if (!canManageSheets.value || !(await autosave.flush())) return
    if (!import.meta.client || !window.confirm('Excluir esta planilha?')) return
    const deleted = await deleteSheet(id)
    if (!deleted) return
    if (selectedSheetId.value === id) selectedSheetId.value = filteredSheets.value[0]?.id ?? null
  }
  async function selectSheet(id: string) {
    const accountId = coreAccount.activeAccountId
    if (!(await autosave.flush()) || accountId !== coreAccount.activeAccountId) return
    selectedSheetId.value = id
  }

  // ---- summary ----
  const entriesExpected = computed(() =>
    draft.entradas.reduce(
      (sum, row) => sum + Number(row.amount || 0) + Number(row.adjustmentAmount || 0),
      0,
    ),
  )
  const entriesEffective = computed(() =>
    draft.entradas.reduce(
      (sum, row) =>
        row.effective ? sum + Number(row.amount || 0) + Number(row.adjustmentAmount || 0) : sum,
      0,
    ),
  )
  const exitsExpected = computed(() =>
    draft.saidas.reduce(
      (sum, row) => sum + Number(row.amount || 0) + Number(row.adjustmentAmount || 0),
      0,
    ),
  )
  const exitsEffective = computed(() =>
    draft.saidas.reduce(
      (sum, row) =>
        row.effective ? sum + Number(row.amount || 0) + Number(row.adjustmentAmount || 0) : sum,
      0,
    ),
  )
  const balanceExpected = computed(() => entriesExpected.value - exitsExpected.value)
  const balanceEffective = computed(() => entriesEffective.value - exitsEffective.value)
  const activeSheetSaving = computed(() => autosave.state.saving)

  // ---- selection watchers ----
  watch(
    filteredSheets,
    () => {
      if (filteredSheets.value.length === 0) {
        selectedSheetId.value = null
        hydrate(null)
        return
      }
      if (!filteredSheets.value.some((sheet) => sheet.id === selectedSheetId.value)) {
        selectedSheetId.value = filteredSheets.value[0]?.id ?? null
      }
    },
    { immediate: true, deep: true },
  )

  watch(
    selectedSheetId,
    async (id) => {
      if (!id) {
        hydrate(null)
        return
      }
      if (activeSheet.value?.id === id) {
        hydrate(activeSheet.value)
        return
      }
      hydrate(null)
      const detail = await fetchSheetDetail(id)
      if (selectedSheetId.value !== id) return
      hydrate(detail)
    },
    { immediate: true },
  )

  watch(
    () => coreAccount.activeAccountId,
    () => {
      void autosave.flush()
    },
    { flush: 'sync' },
  )

  return {
    // state
    sheets,
    activeSheet,
    detailLoading,
    creating,
    deletingId,
    errorMessage,
    selectedSheetId,
    draft,
    filteredSheets,
    // data ops
    fetchSheets,
    refreshActiveSheetDraft,
    hydrate,
    syncAllFixedRows,
    queuePersist,
    // category/fixed helpers
    categoryOptions,
    resolveFixedById,
    recurringEntryForTenant,
    // display
    entradaDisplayItems,
    recurringGroupByRowId,
    ...lineEditor,
    ...effectiveEditor,
    // sheet CRUD
    addLine,
    removeRow,
    onCreateSheet,
    onDeleteSheet,
    selectSheet,
    // summary
    entriesExpected,
    entriesEffective,
    exitsExpected,
    exitsEffective,
    balanceExpected,
    balanceEffective,
    activeSheetSaving,
    canManageSheets,
    isClosed,
    onStatusChange,
    saveLabel: autosave.label,
    retrySave: autosave.flush,
  }
}

export type FinanceSheetEditor = ReturnType<typeof useFinanceSheetEditor>
