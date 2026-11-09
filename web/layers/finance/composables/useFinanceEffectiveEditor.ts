import { reactive } from 'vue'
import type { FinanceLineItem } from '../types/finances'
import type { FinanceRecurringGroupView } from './useFinanceSheetSources'
import {
  snapshotLine,
  todayIsoDate,
  normalizeAdjustmentDate,
  applyLineEffectiveState,
} from '../utils/finance-helpers'
interface EffectiveDeps {
  lineScopedKey: (
    prefix: string,
    kind: 'entrada' | 'saida',
    row: FinanceLineItem,
    index: number,
  ) => string
  persistLineEffective: (row: FinanceLineItem, previous: FinanceLineItem) => Promise<boolean>
  refreshActiveSheetDraft: () => Promise<void>
}
export function useFinanceEffectiveEditor({
  lineScopedKey,
  persistLineEffective,
  refreshActiveSheetDraft,
}: EffectiveDeps) {
  const effectiveDateModalOpen = reactive<Record<string, boolean>>({})
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

  return {
    isEffectiveDateModalOpen,
    setEffectiveDateModalOpen,
    closeEffectiveDateModal,
    setEffectiveToday,
    clearEffectiveDate,
    onEffectiveDateSubmitShortcut,
    onEffectiveDateCancelShortcut,
    onEffectiveDateChanged,
    onEffectiveToggle,
    onRecurringGroupEffectiveToggle,
    onRecurringGroupEffectiveDateChange,
    onRecurringStoreEffectiveToggle,
    onRecurringStoreEffectiveDateChange,
  }
}
