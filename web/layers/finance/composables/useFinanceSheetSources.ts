import { computed } from 'vue'
import type { Ref } from 'vue'
import type {
  FinanceCategoryConfig,
  FinanceFixedAccountConfig,
  FinanceRecurringClientEntry,
  FinanceRecurringEntryConfig,
  FinanceLineItem,
} from '../types/finances'
import {
  financeRecurringRowId,
  financeRecurringStoreRowId,
  isFinanceRecurringRowId,
  isFinanceRecurringStoreRowId,
} from '../utils/finance-ids'
import {
  makeLine,
  ensureLineAdjustments,
  recalcLineAdjustmentTotal,
  normalizeAdjustmentDate,
  normalizeText,
  lineTotal,
  formatRecurringStoreBreakdown,
  buildRecurringDetails,
} from '../utils/finance-helpers'
export interface SheetEditorDeps {
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

interface SourcesDeps extends SheetEditorDeps {
  draft: { entradas: FinanceLineItem[]; saidas: FinanceLineItem[] }
  selectedSheetId: Ref<string | null>
  queuePersist: () => void
}
export function useFinanceSheetSources({
  configDraft,
  clientRecurringEntries,
  draft,
  selectedSheetId,
  queuePersist,
}: SourcesDeps) {
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

  function recurringRowId(clientId: string, storeName?: string) {
    const oldId = storeName
      ? financeRecurringStoreRowId(clientId, storeName)
      : financeRecurringRowId(clientId)
    if (draft.entradas.some((row) => row.id === oldId)) return oldId
    const sheetId = selectedSheetId.value || ''
    return storeName
      ? financeRecurringStoreRowId(clientId, storeName, sheetId)
      : financeRecurringRowId(clientId, sheetId)
  }

  function buildRecurringRows(entry: FinanceRecurringClientEntry, defaultCategory: string) {
    const recurringConfig = recurringEntryForTenant(entry.coreTenantId)
    const adjustment = Number(recurringConfig?.adjustmentAmount || 0)
    const notes = recurringConfig?.notes || ''
    const storeBreakdown = formatRecurringStoreBreakdown(entry)

    if (entry.billingMode === 'per_store' && entry.stores.length > 0) {
      return entry.stores.map((store, index) => ({
        id: recurringRowId(entry.coreTenantId, store.name),
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
        id: recurringRowId(entry.coreTenantId),
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
        const rowId = recurringRowId(entry.coreTenantId, store.name)
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
      rows.reduce((sum, item) => sum + Number(item.row.amount || 0), 0).toFixed(2),
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

  return {
    categoryOptions,
    resolveFixedById,
    recurringEntryForTenant,
    entradaDisplayItems,
    recurringGroupByRowId,
    ensureRows,
    syncAllFixedRows,
  }
}
