import type { FinanceLineItem } from '../types/finances'
import { isFinanceUuid, normalizeFinanceEntityId, normalizeFinanceLinkedUuid } from './finance-ids'
export function normalizeFinanceApiText(value: unknown, max = 12000) {
  return String(value ?? '')
    .replace(/\s+/g, ' ')
    .trim()
    .slice(0, max)
}

export function normalizePeriod(value: unknown) {
  const raw = String(value ?? '').trim()
  return /^\d{4}-\d{2}$/.test(raw) ? raw : ''
}

export function normalizeCoreTenantId(value: unknown) {
  return String(value ?? '')
    .trim()
    .toLowerCase()
}

export function normalizeDate(value: unknown) {
  const raw = String(value ?? '').trim()
  if (!raw) return ''
  if (/^\d{4}-\d{2}-\d{2}$/.test(raw)) return raw
  return ''
}

export function normalizeSheetId(value: unknown) {
  const raw = String(value ?? '')
    .trim()
    .toLowerCase()
  return isFinanceUuid(raw) ? raw : ''
}

export function normalizeAmount(value: unknown, allowNegative = false) {
  if (typeof value === 'number' && Number.isFinite(value)) {
    const rounded = Number(value.toFixed(2))
    return allowNegative ? rounded : Math.max(0, rounded)
  }

  const raw = String(value ?? '').trim()
  if (!raw) return 0

  let normalized = raw
    .replace(/\s+/g, '')
    .replace(/^R\$/i, '')
    .replace(/[^\d,.-]/g, '')

  const hasComma = normalized.includes(',')
  const hasDot = normalized.includes('.')
  if (hasComma && hasDot) {
    if (normalized.lastIndexOf(',') > normalized.lastIndexOf('.')) {
      normalized = normalized.replace(/\./g, '').replace(',', '.')
    } else {
      normalized = normalized.replace(/,/g, '')
    }
  } else if (hasComma) {
    normalized = normalized.replace(/\./g, '').replace(',', '.')
  }

  const parsed = Number(normalized)
  if (!Number.isFinite(parsed)) return 0
  const rounded = Number(parsed.toFixed(2))
  return allowNegative ? rounded : Math.max(0, rounded)
}

export function normalizeAdjustments(value: unknown) {
  if (!Array.isArray(value)) return []

  return value.map((item) => {
    const source =
      item && typeof item === 'object'
        ? (item as {
            id?: unknown
            amount?: unknown
            note?: unknown
            date?: unknown
          })
        : {}

    return {
      id: normalizeFinanceEntityId(source.id),
      amount: normalizeAmount(source.amount, true),
      note: normalizeFinanceApiText(source.note, 240),
      date: normalizeDate(source.date),
    }
  })
}

export function normalizeRows(value: FinanceLineItem[] | undefined, _kind: 'entrada' | 'saida') {
  if (!Array.isArray(value)) return undefined

  return value.map((item) => {
    const adjustments = normalizeAdjustments(item?.adjustments)
    const adjustmentAmount =
      adjustments.length > 0
        ? Number(adjustments.reduce((sum, adjustment) => sum + adjustment.amount, 0).toFixed(2))
        : normalizeAmount(item?.adjustmentAmount, true)

    return {
      adjustments,
      id: normalizeFinanceEntityId(item?.id),
      description: normalizeFinanceApiText(item?.description, 260),
      category: normalizeFinanceApiText(item?.category, 120),
      effective: Boolean(item?.effective),
      effectiveDate: normalizeDate(item?.effectiveDate),
      amount: normalizeAmount(item?.amount, false),
      adjustmentAmount,
      fixedAccountId: normalizeFinanceLinkedUuid(item?.fixedAccountId),
      details: normalizeFinanceApiText(item?.details, 600),
    }
  })
}
