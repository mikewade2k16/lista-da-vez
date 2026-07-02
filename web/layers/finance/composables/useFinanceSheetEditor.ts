// Editor da planilha financeira ativa: draft + autosave + linhas + efetivacao +
// grupos de recorrencia. Recebe o estado de configuracao (configDraft +
// clientRecurringEntries) para sincronizar linhas fixas/recorrentes.
import type { Ref } from 'vue'
import type {
  FinanceCategoryConfig,
  FinanceFixedAccountConfig,
  FinanceLineAdjustment,
  FinanceLineItem,
  FinanceRecurringClientEntry,
  FinanceRecurringEntryConfig,
  FinanceSheetItem,
} from '../types/finances'
import {
  createFinanceUuid,
  financeRecurringRowId,
  financeRecurringStoreRowId,
  isFinanceRecurringRowId,
  isFinanceRecurringStoreRowId,
  normalizeFinanceLinkedUuid,
} from '../utils/finance-ids'
import {
  applyLineEffectiveState,
  applySnapshotLine,
  buildRecurringDetails,
  ensureLineAdjustments,
  formatRecurringStoreBreakdown,
  lineTotal,
  makeLine,
  normalizeAdjustmentDate,
  normalizeText,
  parseDraftAdjustmentAmount,
  recalcLineAdjustmentTotal,
  snapshotLine,
  todayIsoDate,
  currentPeriod,
  LINE_CARD_INTERACTIVE_SELECTOR,
  SHEET_AUTOSAVE_DEBOUNCE_MS,
  SHEET_SAVE_INDICATOR_DELAY_MS,
} from '../utils/finance-helpers'
import { useCoreAccountStore } from '../../core/stores/account'

interface SheetEditorDeps {
  configDraft: {
    categories: FinanceCategoryConfig[]
    fixedAccounts: FinanceFixedAccountConfig[]
    recurringEntries: FinanceRecurringEntryConfig[]
  }
  clientRecurringEntries: Ref<FinanceRecurringClientEntry[]>
}

interface FinanceRecurringGroupStoreLine {
  key: string
  rowId: string
  row: FinanceLineItem
  name: string
  amount: number
}

export interface FinanceRecurringGroupView {
  key: string
  entryId: string
  title: string
  category: string
  baseAmount: number
  totalAmount: number
  adjustmentAmount: number
  effective: boolean
  effectiveDate: string
  rows: FinanceRecurringGroupStoreLine[]
}

type FinanceEntradaDisplayItem =
  | { kind: 'line'; key: string; row: FinanceLineItem; index: number }
  | { kind: 'group'; key: string; group: FinanceRecurringGroupView }

export function useFinanceSheetEditor(deps: SheetEditorDeps) {
  const coreAccount = useCoreAccountStore()
  const { configDraft, clientRecurringEntries } = deps
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
    updateSheetLine,
    deleteSheet,
  } = useFinancesManager()

  const targetCoreTenantId = computed(() => String(coreAccount.activeAccountId || '').trim())
  const selectedSheetId = ref<string | null>(null)
  const activeSheetSaveIndicator = ref(false)

  const lineDetailsOpen = reactive<Record<string, boolean>>({})
  const lineAdjustmentHistoryOpen = reactive<Record<string, boolean>>({})
  const effectiveDateModalOpen = reactive<Record<string, boolean>>({})
  const adjustmentModalOpen = reactive<Record<string, boolean>>({})
  const adjustmentDraftByKey = reactive<
    Record<string, { amountInput: string; note: string; date: string }>
  >({})

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

  let saveTimer: ReturnType<typeof setTimeout> | null = null
  let saveIndicatorTimer: ReturnType<typeof setTimeout> | null = null
  let sheetPersistInFlight = false
  let sheetPersistQueued = false

  const filteredSheets = computed(() => sheets.value)

  // ---- category / fixed helpers (leem configDraft) ----
  function resolveCategoryNameById(categoryId: string) {
    if (!categoryId) return ''
    return configDraft.categories.find((category) => category.id === categoryId)?.name || ''
  }
  function fixedAccountsByKind(kind: 'entrada' | 'saida') {
    return configDraft.fixedAccounts.filter(
      (account) => account.kind === 'ambas' || account.kind === kind,
    )
  }
  function categoryOptions(kind: 'entrada' | 'saida') {
    return configDraft.categories
      .filter((category) => category.kind === 'ambas' || category.kind === kind)
      .map((category) => ({ label: category.name, value: category.name }))
  }
  function resolveFixedById(id: string) {
    return configDraft.fixedAccounts.find((account) => account.id === id) || null
  }
  function recurringEntryForTenant(sourceCoreTenantId: string) {
    const normalized = String(sourceCoreTenantId || '')
      .trim()
      .toLowerCase()
    if (!normalized) return undefined
    return configDraft.recurringEntries.find(
      (item) =>
        String(item.sourceCoreTenantId || '')
          .trim()
          .toLowerCase() === normalized,
    )
  }

  // ---- recurring row/group helpers ----
  function isRecurringRowId(value: string) {
    return isFinanceRecurringRowId(value) || isFinanceRecurringStoreRowId(value)
  }

  function buildRecurringRows(entry: FinanceRecurringClientEntry, defaultCategory: string) {
    const recurringConfig = recurringEntryForTenant(entry.coreTenantId)
    const adjustment = Number(recurringConfig?.adjustmentAmount || 0)
    const notes = recurringConfig?.notes || ''
    const storeBreakdown = formatRecurringStoreBreakdown(entry)

    if (entry.billingMode === 'per_store' && entry.stores.length > 0) {
      return entry.stores.map((store, index) => ({
        id: financeRecurringStoreRowId(entry.coreTenantId, store.name),
        description: `Mensalidade ${entry.name} - ${store.name}`,
        amount: Number((Number(store.amount || 0) + (index === 0 ? adjustment : 0)).toFixed(2)),
        category: defaultCategory,
        details: buildRecurringDetails(entry, {
          storeName: store.name,
          storeBreakdown,
          notes: index === 0 ? notes : '',
        }),
      }))
    }

    return [
      {
        id: financeRecurringRowId(entry.coreTenantId),
        description: `Mensalidade ${entry.name}`,
        amount: Number((entry.amount + adjustment).toFixed(2)),
        category: defaultCategory,
        details: buildRecurringDetails(entry, { storeBreakdown, notes }),
      },
    ]
  }

  function buildRecurringGroup(
    entry: FinanceRecurringClientEntry,
  ): FinanceRecurringGroupView | null {
    if (entry.billingMode !== 'per_store' || entry.stores.length === 0) return null

    const rows = entry.stores
      .map((store) => {
        const rowId = financeRecurringStoreRowId(entry.coreTenantId, store.name)
        const row = draft.entradas.find((item) => item.id === rowId)
        if (!row) return null
        return {
          key: `${entry.id}:${rowId}`,
          rowId,
          row,
          name: store.name,
          amount: Number(lineTotal(row).toFixed(2)),
        }
      })
      .filter((item): item is FinanceRecurringGroupStoreLine => Boolean(item))

    if (rows.length === 0) return null

    const totalAmount = Number(rows.reduce((sum, item) => sum + item.amount, 0).toFixed(2))
    const baseAmount = Number(
      entry.stores.reduce((sum, store) => sum + Number(store.amount || 0), 0).toFixed(2),
    )
    const effective = rows.length > 0 && rows.every((item) => item.row.effective)
    const effectiveDates = rows
      .map((item) => normalizeAdjustmentDate(item.row.effectiveDate))
      .filter(Boolean)
      .sort()

    return {
      key: `recurring-group:${entry.id}`,
      entryId: entry.id,
      title: `Mensalidade ${entry.name}`,
      category:
        rows.find((item) => normalizeText(item.row.category, 120))?.row.category ||
        'Receita mensalidade',
      baseAmount,
      totalAmount,
      adjustmentAmount: Number((totalAmount - baseAmount).toFixed(2)),
      effective,
      effectiveDate:
        effective && effectiveDates.length === rows.length
          ? effectiveDates[effectiveDates.length - 1] || ''
          : '',
      rows,
    }
  }

  const recurringGroupByRowId = computed(() => {
    const groups = new Map<string, FinanceRecurringGroupView>()
    clientRecurringEntries.value.forEach((entry) => {
      const group = buildRecurringGroup(entry)
      if (!group) return
      group.rows.forEach((item) => groups.set(item.rowId, group))
    })
    return groups
  })

  const entradaDisplayItems = computed<FinanceEntradaDisplayItem[]>(() => {
    const consumed = new Set<string>()
    const output: FinanceEntradaDisplayItem[] = []
    draft.entradas.forEach((row, index) => {
      if (consumed.has(row.id)) return
      const recurringGroup = recurringGroupByRowId.value.get(row.id)
      if (recurringGroup) {
        output.push({ kind: 'group', key: recurringGroup.key, group: recurringGroup })
        recurringGroup.rows.forEach((item) => consumed.add(item.rowId))
        return
      }
      output.push({ kind: 'line', key: row.id || `entrada:${index}`, row, index })
    })
    return output
  })

  // ---- sync fixed / recurring rows with draft ----
  function ensureRows() {
    if (draft.entradas.length === 0) draft.entradas = [makeLine()]
    if (draft.saidas.length === 0) draft.saidas = [makeLine()]
    draft.entradas.forEach((row) => {
      ensureLineAdjustments(row)
      recalcLineAdjustmentTotal(row)
    })
    draft.saidas.forEach((row) => {
      ensureLineAdjustments(row)
      recalcLineAdjustmentTotal(row)
    })
  }

  function syncFixedRowsWithDraft(kind: 'entrada' | 'saida', shouldPersist = false) {
    const list = kind === 'entrada' ? draft.entradas : draft.saidas
    const fixedAccounts = fixedAccountsByKind(kind)
    const fixedById = new Map(fixedAccounts.map((account) => [account.id, account] as const))

    for (let index = list.length - 1; index >= 0; index -= 1) {
      const row = list[index]
      if (!row?.fixedAccountId) continue
      if (isRecurringRowId(row.id)) continue
      if (!fixedById.has(row.fixedAccountId)) list.splice(index, 1)
    }

    fixedAccounts.forEach((account) => {
      let row = list.find((item) => item.fixedAccountId === account.id)
      const categoryName = resolveCategoryNameById(account.categoryId)
      if (!row) {
        row = makeLine()
        row.fixedAccountId = account.id
        row.description = account.name
        row.amount = Number(account.defaultAmount || 0)
        row.category = categoryName
        row.details = account.notes || ''
        list.unshift(row)
        return
      }
      row.description = account.name
      row.amount = Number(account.defaultAmount || 0)
      if (categoryName) row.category = categoryName
      if (!normalizeText(row.details, 600)) row.details = account.notes || ''
    })

    const fixedRows = fixedAccounts
      .map((account) => list.find((item) => item.fixedAccountId === account.id))
      .filter((item): item is FinanceLineItem => Boolean(item))
    const customRows = list.filter((row) => !fixedById.has(row.fixedAccountId))
    list.splice(0, list.length, ...fixedRows, ...customRows)

    ensureRows()
    if (shouldPersist) queuePersist()
  }

  function syncRecurringRowsWithDraft(shouldPersist = false) {
    const list = draft.entradas
    const defaultCategory =
      configDraft.categories.find(
        (category) => category.kind === 'entrada' || category.kind === 'ambas',
      )?.name || 'Receita mensalidade'
    const recurringRowsInput = clientRecurringEntries.value.flatMap((entry) =>
      buildRecurringRows(entry, defaultCategory),
    )
    const recurringRowIds = new Set(recurringRowsInput.map((row) => row.id))

    for (let index = list.length - 1; index >= 0; index -= 1) {
      const row = list[index]
      if (!row || !isRecurringRowId(row.id)) continue
      if (!recurringRowIds.has(row.id)) list.splice(index, 1)
    }

    recurringRowsInput.forEach((sourceRow) => {
      let row = list.find((item) => item.id === sourceRow.id)
      if (!row) {
        row = makeLine()
        row.id = sourceRow.id
        row.effective = false
        list.unshift(row)
      }
      row.description = sourceRow.description
      row.category = sourceRow.category
      row.amount = sourceRow.amount
      row.details = sourceRow.details
      row.fixedAccountId = ''
    })

    const recurringRows = recurringRowsInput
      .map((sourceRow) => list.find((item) => item.id === sourceRow.id))
      .filter((item): item is FinanceLineItem => Boolean(item))
    const otherRows = list.filter((row) => !isRecurringRowId(row.id))
    list.splice(0, list.length, ...recurringRows, ...otherRows)

    ensureRows()
    if (shouldPersist) queuePersist()
  }

  function syncAllFixedRows(shouldPersist = false) {
    syncFixedRowsWithDraft('entrada', shouldPersist)
    syncFixedRowsWithDraft('saida', shouldPersist)
    syncRecurringRowsWithDraft(shouldPersist)
  }

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
    syncAllFixedRows(false)
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

  function scheduleSheetSaveIndicator() {
    if (saveIndicatorTimer) clearTimeout(saveIndicatorTimer)
    saveIndicatorTimer = setTimeout(() => {
      if (sheetPersistInFlight) activeSheetSaveIndicator.value = true
    }, SHEET_SAVE_INDICATOR_DELAY_MS)
  }

  function clearSheetSaveIndicator() {
    if (saveIndicatorTimer) {
      clearTimeout(saveIndicatorTimer)
      saveIndicatorTimer = null
    }
    activeSheetSaveIndicator.value = false
  }

  async function persist() {
    const id = String(selectedSheetId.value || '')
      .trim()
      .toLowerCase()
    if (!id) return
    if (sheetPersistInFlight) {
      sheetPersistQueued = true
      return
    }
    sheetPersistInFlight = true
    sheetPersistQueued = false
    scheduleSheetSaveIndicator()
    try {
      await updateSheet(id, buildSheetPersistPayload())
    } finally {
      sheetPersistInFlight = false
      clearSheetSaveIndicator()
      if (sheetPersistQueued) {
        sheetPersistQueued = false
        void persist()
      }
    }
  }

  function queuePersist(delayMs = SHEET_AUTOSAVE_DEBOUNCE_MS) {
    if (!selectedSheetId.value) return
    if (saveTimer) clearTimeout(saveTimer)
    saveTimer = setTimeout(
      () => {
        saveTimer = null
        void persist()
      },
      Math.max(0, delayMs),
    )
  }

  // ---- line effective ----
  async function persistLineEffective(row: FinanceLineItem, previous: FinanceLineItem) {
    const id = String(selectedSheetId.value || '')
      .trim()
      .toLowerCase()
    if (!id || !row.id) return false
    const response = await updateSheetLine(id, row.id, {
      effective: row.effective,
      effectiveDate: row.effectiveDate,
    })
    if (!response) {
      applySnapshotLine(row, previous)
      return false
    }
    applySnapshotLine(row, response.line)
    return true
  }

  // ---- keys ----
  function lineKey(kind: 'entrada' | 'saida', row: FinanceLineItem, index: number) {
    return `${kind}:${row.id || index}`
  }
  function lineScopedKey(
    prefix: string,
    kind: 'entrada' | 'saida',
    row: FinanceLineItem,
    index: number,
  ) {
    return `${prefix}:${lineKey(kind, row, index)}`
  }

  // ---- line details ----
  function toggleLineDetails(kind: 'entrada' | 'saida', row: FinanceLineItem, index: number) {
    const key = lineKey(kind, row, index)
    lineDetailsOpen[key] = !lineDetailsOpen[key]
  }
  function isLineDetailsOpen(kind: 'entrada' | 'saida', row: FinanceLineItem, index: number) {
    return Boolean(lineDetailsOpen[lineKey(kind, row, index)])
  }
  function isInteractiveLineTarget(eventTarget: EventTarget | null) {
    if (!(eventTarget instanceof Element)) return false
    return Boolean(eventTarget.closest(LINE_CARD_INTERACTIVE_SELECTOR))
  }
  function onLineCardClick(
    kind: 'entrada' | 'saida',
    row: FinanceLineItem,
    index: number,
    event: MouseEvent,
  ) {
    if (isInteractiveLineTarget(event.target)) return
    toggleLineDetails(kind, row, index)
  }

  function onLineTotalInput(row: FinanceLineItem, value: unknown) {
    const parsed = Number(value || 0)
    const safeTotal = Number.isFinite(parsed) ? parsed : 0
    const nextBase = safeTotal - Number(row.adjustmentAmount || 0)
    row.amount = Number(Math.max(0, nextBase).toFixed(2))
    queuePersist()
  }

  // ---- adjustment history / modal ----
  function isAdjustmentHistoryOpen(kind: 'entrada' | 'saida', row: FinanceLineItem, index: number) {
    return Boolean(lineAdjustmentHistoryOpen[lineScopedKey('line-history', kind, row, index)])
  }
  function toggleAdjustmentHistory(kind: 'entrada' | 'saida', row: FinanceLineItem, index: number) {
    const key = lineScopedKey('line-history', kind, row, index)
    lineAdjustmentHistoryOpen[key] = !lineAdjustmentHistoryOpen[key]
  }
  function ensureAdjustmentDraft(kind: 'entrada' | 'saida', row: FinanceLineItem, index: number) {
    const key = lineScopedKey('line-adjustment', kind, row, index)
    if (!adjustmentDraftByKey[key]) {
      adjustmentDraftByKey[key] = { amountInput: '', note: '', date: todayIsoDate() }
    }
    return adjustmentDraftByKey[key]!
  }
  function isAdjustmentModalOpen(kind: 'entrada' | 'saida', row: FinanceLineItem, index: number) {
    return Boolean(adjustmentModalOpen[lineScopedKey('line-adjustment', kind, row, index)])
  }
  function setAdjustmentModalOpen(
    kind: 'entrada' | 'saida',
    row: FinanceLineItem,
    index: number,
    open: boolean,
  ) {
    adjustmentModalOpen[lineScopedKey('line-adjustment', kind, row, index)] = Boolean(open)
    if (!open) return
    const draftEntry = ensureAdjustmentDraft(kind, row, index)
    draftEntry.amountInput = ''
    draftEntry.date = normalizeAdjustmentDate(draftEntry.date) || todayIsoDate()
  }
  function closeAdjustmentModal(kind: 'entrada' | 'saida', row: FinanceLineItem, index: number) {
    setAdjustmentModalOpen(kind, row, index, false)
  }
  function addLineAdjustment(kind: 'entrada' | 'saida', row: FinanceLineItem, index: number) {
    const draftEntry = ensureAdjustmentDraft(kind, row, index)
    const amount = parseDraftAdjustmentAmount(draftEntry.amountInput)
    if (!Number.isFinite(amount) || amount === 0) return false
    ensureLineAdjustments(row)
    row.adjustments.push({
      id: createFinanceUuid(),
      amount,
      note: normalizeText(draftEntry.note, 240),
      date: normalizeAdjustmentDate(draftEntry.date) || todayIsoDate(),
    })
    recalcLineAdjustmentTotal(row)
    draftEntry.amountInput = ''
    draftEntry.note = ''
    draftEntry.date = todayIsoDate()
    closeAdjustmentModal(kind, row, index)
    queuePersist()
    return true
  }
  function onAdjustmentSubmitShortcut(
    kind: 'entrada' | 'saida',
    row: FinanceLineItem,
    index: number,
    event: KeyboardEvent,
  ) {
    event.preventDefault()
    addLineAdjustment(kind, row, index)
  }
  function onAdjustmentCancelShortcut(
    kind: 'entrada' | 'saida',
    row: FinanceLineItem,
    index: number,
  ) {
    closeAdjustmentModal(kind, row, index)
  }
  function removeLineAdjustment(row: FinanceLineItem, adjustmentId: string) {
    ensureLineAdjustments(row)
    row.adjustments = row.adjustments.filter((adjustment) => adjustment.id !== adjustmentId)
    recalcLineAdjustmentTotal(row)
    queuePersist()
  }
  function onAdjustmentHistoryChanged(row: FinanceLineItem) {
    ensureLineAdjustments(row)
    recalcLineAdjustmentTotal(row)
    queuePersist()
  }
  function setAdjustmentSign(
    row: FinanceLineItem,
    adjustment: FinanceLineAdjustment,
    sign: '+' | '-',
  ) {
    const absolute = Math.abs(Number(adjustment.amount || 0))
    adjustment.amount = Number((sign === '-' ? -absolute : absolute).toFixed(2))
    onAdjustmentHistoryChanged(row)
  }
  function setAdjustmentAbsoluteAmount(
    row: FinanceLineItem,
    adjustment: FinanceLineAdjustment,
    value: number,
  ) {
    const absolute = Math.abs(Number(value || 0))
    const sign = Number(adjustment.amount || 0) < 0 ? -1 : 1
    adjustment.amount = Number((absolute * sign).toFixed(2))
    onAdjustmentHistoryChanged(row)
  }

  // ---- effective date modal ----
  function isEffectiveDateModalOpen(
    kind: 'entrada' | 'saida',
    row: FinanceLineItem,
    index: number,
  ) {
    return Boolean(effectiveDateModalOpen[lineScopedKey('effective-date', kind, row, index)])
  }
  function setEffectiveDateModalOpen(
    kind: 'entrada' | 'saida',
    row: FinanceLineItem,
    index: number,
    open: boolean,
  ) {
    effectiveDateModalOpen[lineScopedKey('effective-date', kind, row, index)] = Boolean(open)
  }
  function closeEffectiveDateModal(kind: 'entrada' | 'saida', row: FinanceLineItem, index: number) {
    setEffectiveDateModalOpen(kind, row, index, false)
  }
  async function setEffectiveToday(row: FinanceLineItem) {
    const previous = snapshotLine(row)
    row.effectiveDate = todayIsoDate()
    await persistLineEffective(row, previous)
  }
  async function clearEffectiveDate(row: FinanceLineItem) {
    const previous = snapshotLine(row)
    row.effectiveDate = ''
    await persistLineEffective(row, previous)
  }
  async function onEffectiveDateSubmitShortcut(
    kind: 'entrada' | 'saida',
    row: FinanceLineItem,
    index: number,
    event: KeyboardEvent,
  ) {
    event.preventDefault()
    const previous = snapshotLine(row)
    row.effectiveDate = normalizeAdjustmentDate(row.effectiveDate) || todayIsoDate()
    await persistLineEffective(row, previous)
    closeEffectiveDateModal(kind, row, index)
  }
  function onEffectiveDateCancelShortcut(
    kind: 'entrada' | 'saida',
    row: FinanceLineItem,
    index: number,
  ) {
    closeEffectiveDateModal(kind, row, index)
  }
  async function onEffectiveDateChanged(row: FinanceLineItem) {
    const previous = snapshotLine(row)
    row.effectiveDate = normalizeAdjustmentDate(row.effectiveDate) || todayIsoDate()
    await persistLineEffective(row, previous)
  }
  async function onEffectiveToggle(
    kind: 'entrada' | 'saida',
    row: FinanceLineItem,
    index: number,
    next: boolean,
  ) {
    const previous = snapshotLine(row)
    row.effective = Boolean(next)
    if (!row.effective) {
      row.effectiveDate = ''
      closeEffectiveDateModal(kind, row, index)
      await persistLineEffective(row, previous)
      return
    }
    if (!normalizeAdjustmentDate(row.effectiveDate)) row.effectiveDate = todayIsoDate()
    setEffectiveDateModalOpen(kind, row, index, true)
    const saved = await persistLineEffective(row, previous)
    if (!saved) closeEffectiveDateModal(kind, row, index)
  }

  // ---- recurring group effective ----
  async function persistRecurringStoreRowEffective(
    row: FinanceLineItem,
    options: { effective: boolean; effectiveDate?: string },
  ) {
    const previous = snapshotLine(row)
    applyLineEffectiveState(row, options.effective, options.effectiveDate || row.effectiveDate)
    return persistLineEffective(row, previous)
  }
  async function persistRecurringGroupEffective(
    group: FinanceRecurringGroupView,
    options: { effective: boolean; effectiveDate?: string },
  ) {
    const nextEffectiveDate = options.effective
      ? normalizeAdjustmentDate(options.effectiveDate) || group.effectiveDate || todayIsoDate()
      : ''
    const results = await Promise.all(
      group.rows.map((item) =>
        persistRecurringStoreRowEffective(item.row, {
          effective: options.effective,
          effectiveDate: nextEffectiveDate,
        }),
      ),
    )
    if (results.some((result) => !result)) await refreshActiveSheetDraft()
  }
  async function onRecurringGroupEffectiveToggle(group: FinanceRecurringGroupView, next: boolean) {
    await persistRecurringGroupEffective(group, {
      effective: next,
      effectiveDate: next ? group.effectiveDate || todayIsoDate() : '',
    })
  }
  async function onRecurringGroupEffectiveDateChange(
    group: FinanceRecurringGroupView,
    value: string,
  ) {
    await persistRecurringGroupEffective(group, {
      effective: true,
      effectiveDate: normalizeAdjustmentDate(value) || todayIsoDate(),
    })
  }
  async function onRecurringStoreEffectiveToggle(
    group: FinanceRecurringGroupView,
    rowId: string,
    next: boolean,
  ) {
    const storeRow = group.rows.find((item) => item.rowId === rowId)
    if (!storeRow) return
    const saved = await persistRecurringStoreRowEffective(storeRow.row, {
      effective: next,
      effectiveDate: next
        ? group.effectiveDate || storeRow.row.effectiveDate || todayIsoDate()
        : '',
    })
    if (!saved) await refreshActiveSheetDraft()
  }
  async function onRecurringStoreEffectiveDateChange(
    group: FinanceRecurringGroupView,
    rowId: string,
    value: string,
  ) {
    const storeRow = group.rows.find((item) => item.rowId === rowId)
    if (!storeRow) return
    const saved = await persistRecurringStoreRowEffective(storeRow.row, {
      effective: true,
      effectiveDate: normalizeAdjustmentDate(value) || todayIsoDate(),
    })
    if (!saved) await refreshActiveSheetDraft()
  }

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
    const created = await createSheet({
      title: `Finance ${currentPeriod()}`,
      period: currentPeriod(),
      status: 'aberta',
      coreTenantId: targetCoreTenantId.value,
    })
    if (!created) return
    selectedSheetId.value = created.id
    hydrate(created)
  }
  async function onDeleteSheet(id: string) {
    if (!import.meta.client || !window.confirm('Excluir esta planilha?')) return
    const deleted = await deleteSheet(id)
    if (!deleted) return
    if (selectedSheetId.value === id) selectedSheetId.value = filteredSheets.value[0]?.id ?? null
  }
  function selectSheet(id: string) {
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
  const activeSheetSaving = computed(() => activeSheetSaveIndicator.value)

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

  onScopeDispose(() => {
    if (saveTimer) clearTimeout(saveTimer)
    clearSheetSaveIndicator()
  })

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
    // line details
    isLineDetailsOpen,
    toggleLineDetails,
    onLineCardClick,
    onLineTotalInput,
    // adjustments
    ensureAdjustmentDraft,
    isAdjustmentHistoryOpen,
    toggleAdjustmentHistory,
    isAdjustmentModalOpen,
    setAdjustmentModalOpen,
    closeAdjustmentModal,
    addLineAdjustment,
    onAdjustmentSubmitShortcut,
    onAdjustmentCancelShortcut,
    removeLineAdjustment,
    onAdjustmentHistoryChanged,
    setAdjustmentSign,
    setAdjustmentAbsoluteAmount,
    // effective
    isEffectiveDateModalOpen,
    setEffectiveDateModalOpen,
    closeEffectiveDateModal,
    setEffectiveToday,
    clearEffectiveDate,
    onEffectiveDateSubmitShortcut,
    onEffectiveDateCancelShortcut,
    onEffectiveDateChanged,
    onEffectiveToggle,
    // recurring group
    onRecurringGroupEffectiveToggle,
    onRecurringGroupEffectiveDateChange,
    onRecurringStoreEffectiveToggle,
    onRecurringStoreEffectiveDateChange,
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
  }
}

export type FinanceSheetEditor = ReturnType<typeof useFinanceSheetEditor>
