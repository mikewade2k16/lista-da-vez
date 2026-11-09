import { reactive } from 'vue'
import type { FinanceLineItem, FinanceLineAdjustment } from '../types/finances'
import { createFinanceUuid } from '../utils/finance-ids'
import {
  LINE_CARD_INTERACTIVE_SELECTOR,
  todayIsoDate,
  normalizeText,
  normalizeAdjustmentDate,
  ensureLineAdjustments,
  recalcLineAdjustmentTotal,
  parseDraftAdjustmentAmount,
} from '../utils/finance-helpers'
export function useFinanceLineEditor(queuePersist: () => void) {
  const lineDetailsOpen = reactive<Record<string, boolean>>({})
  const lineAdjustmentHistoryOpen = reactive<Record<string, boolean>>({})
  const adjustmentModalOpen = reactive<Record<string, boolean>>({})
  const adjustmentDraftByKey = reactive<
    Record<string, { amountInput: string; note: string; date: string }>
  >({})

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
    // A lista já foi normalizada; não promover o total antigo a ajuste legado.
    row.adjustmentAmount = 0
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

  return {
    lineScopedKey,
    toggleLineDetails,
    isLineDetailsOpen,
    onLineCardClick,
    onLineTotalInput,
    isAdjustmentHistoryOpen,
    toggleAdjustmentHistory,
    ensureAdjustmentDraft,
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
  }
}
