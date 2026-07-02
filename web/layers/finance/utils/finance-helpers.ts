// Helpers puros do modulo Finance (sem estado reativo). Extraidos da pagina
// web-reference/app/pages/admin/finance.vue para respeitar o limite de arquivo.
import type {
  FinanceLineAdjustment,
  FinanceLineItem,
  FinanceRecurringClientEntry,
} from '../types/finances'
import { createFinanceUuid, normalizeFinanceEntityId } from './finance-ids'

export const BRAZIL_TIMEZONE = 'America/Sao_Paulo'
export const SHEET_AUTOSAVE_DEBOUNCE_MS = 1200
export const CONFIG_AUTOSAVE_DEBOUNCE_MS = 900
export const SHEET_SAVE_INDICATOR_DELAY_MS = 220

export const STATUS_OPTIONS = [
  { label: 'Aberta', value: 'aberta' },
  { label: 'Conferencia', value: 'conferencia' },
  { label: 'Fechada', value: 'fechada' },
]

export const KIND_OPTIONS = [
  { label: 'Entrada', value: 'entrada' },
  { label: 'Saida', value: 'saida' },
  { label: 'Ambas', value: 'ambas' },
]

export const LINE_CARD_INTERACTIVE_SELECTOR = [
  'button',
  'input',
  'textarea',
  'select',
  'a',
  '[role="button"]',
  '[role="switch"]',
].join(', ')

const moneyFormatter = new Intl.NumberFormat('pt-BR', { style: 'currency', currency: 'BRL' })

export function formatMoney(value: unknown) {
  const parsed = Number(value || 0)
  return moneyFormatter.format(Number.isFinite(parsed) ? parsed : 0)
}

export function formatSignedMoney(value: unknown) {
  const parsed = Number(value || 0)
  if (!Number.isFinite(parsed) || parsed === 0) return formatMoney(0)
  return `${parsed > 0 ? '+' : '-'} ${formatMoney(Math.abs(parsed))}`
}

export function financeBrazilDateParts(value = new Date()) {
  const parts = new Intl.DateTimeFormat('en-US', {
    timeZone: BRAZIL_TIMEZONE,
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
  }).formatToParts(value)

  const mapped = parts.reduce<Record<string, string>>((acc, part) => {
    if (part.type === 'year' || part.type === 'month' || part.type === 'day') {
      acc[part.type] = part.value
    }
    return acc
  }, {})

  return {
    year: mapped.year || '0000',
    month: mapped.month || '01',
    day: mapped.day || '01',
  }
}

export function currentPeriod() {
  const parts = financeBrazilDateParts()
  return `${parts.year}-${parts.month}`
}

export function todayIsoDate() {
  const parts = financeBrazilDateParts()
  return `${parts.year}-${parts.month}-${parts.day}`
}

export function normalizeText(value: unknown, max = 240) {
  return String(value ?? '')
    .replace(/\s+/g, ' ')
    .trim()
    .slice(0, max)
}

export function normalizeAdjustmentDate(value: unknown) {
  const raw = String(value ?? '').trim()
  if (/^\d{4}-\d{2}-\d{2}$/.test(raw)) return raw
  return ''
}

export function normalizeAdjustmentEntry(value: unknown): FinanceLineAdjustment {
  const source = value && typeof value === 'object' ? (value as Partial<FinanceLineAdjustment>) : {}

  return {
    id: normalizeFinanceEntityId(source.id),
    amount: Number(Number(source.amount || 0).toFixed(2)),
    note: normalizeText(source.note, 240),
    date: normalizeAdjustmentDate(source.date),
  }
}

export function makeLine(): FinanceLineItem {
  return {
    id: createFinanceUuid(),
    description: '',
    category: '',
    effective: false,
    effectiveDate: '',
    amount: 0,
    adjustmentAmount: 0,
    adjustments: [],
    fixedAccountId: '',
    details: '',
  }
}

export function ensureLineAdjustments(row: FinanceLineItem) {
  if (!Array.isArray(row.adjustments)) {
    row.adjustments = []
  }

  if (row.adjustments.length === 0 && Number(row.adjustmentAmount || 0) !== 0) {
    row.adjustments.push({
      id: createFinanceUuid(),
      amount: Number(Number(row.adjustmentAmount || 0).toFixed(2)),
      note: 'Ajuste legado',
      date: todayIsoDate(),
    })
  }
}

export function recalcLineAdjustmentTotal(row: FinanceLineItem) {
  ensureLineAdjustments(row)
  row.adjustmentAmount = Number(
    row.adjustments.reduce((sum, adjustment) => sum + Number(adjustment.amount || 0), 0).toFixed(2),
  )
}

export function lineTotal(row: FinanceLineItem) {
  return Number((Number(row.amount || 0) + Number(row.adjustmentAmount || 0)).toFixed(2))
}

export function parseDraftAdjustmentAmount(rawValue: string) {
  const raw = String(rawValue ?? '').trim()
  if (!raw) return 0

  const clean = raw.replace(/\s+/g, '').replace(/^R\$/i, '')
  const explicitMinus = clean.startsWith('-')

  let normalized = clean.replace(/[^\d,.-]/g, '')
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
  if (!Number.isFinite(parsed) || parsed === 0) return 0

  const absolute = Math.abs(parsed)
  const sign = explicitMinus ? -1 : 1
  return Number((absolute * sign).toFixed(2))
}

export function formatAdjustmentInputHint() {
  return '100 = +100 | -100 para subtrair'
}

export function snapshotAdjustment(adjustment: FinanceLineAdjustment): FinanceLineAdjustment {
  return {
    id: adjustment.id,
    amount: Number(adjustment.amount || 0),
    note: adjustment.note || '',
    date: adjustment.date || '',
  }
}

export function snapshotLine(row: FinanceLineItem): FinanceLineItem {
  return {
    id: row.id,
    kind: row.kind,
    description: row.description || '',
    category: row.category || '',
    effective: Boolean(row.effective),
    effectiveDate: row.effectiveDate || '',
    amount: Number(row.amount || 0),
    adjustmentAmount: Number(row.adjustmentAmount || 0),
    adjustments: Array.isArray(row.adjustments) ? row.adjustments.map(snapshotAdjustment) : [],
    fixedAccountId: row.fixedAccountId || '',
    details: row.details || '',
  }
}

export function applySnapshotLine(target: FinanceLineItem, source: FinanceLineItem) {
  target.id = source.id
  target.kind = source.kind
  target.description = source.description || ''
  target.category = source.category || ''
  target.effective = Boolean(source.effective)
  target.effectiveDate = source.effectiveDate || ''
  target.amount = Number(source.amount || 0)
  target.adjustmentAmount = Number(source.adjustmentAmount || 0)
  target.adjustments = Array.isArray(source.adjustments)
    ? source.adjustments.map(snapshotAdjustment)
    : []
  target.fixedAccountId = source.fixedAccountId || ''
  target.details = source.details || ''
}

export function applyLineEffectiveState(
  row: FinanceLineItem,
  effective: boolean,
  effectiveDate: string,
) {
  row.effective = effective
  if (!effective) {
    row.effectiveDate = ''
    return
  }
  row.effectiveDate = normalizeAdjustmentDate(effectiveDate) || todayIsoDate()
}

export function formatRecurringStoreBreakdown(entry: FinanceRecurringClientEntry) {
  return entry.stores
    .filter((store) => store.name)
    .map((store) => `${store.name} (${formatMoney(store.amount)})`)
    .join(' | ')
}

export function buildRecurringDetails(
  entry: FinanceRecurringClientEntry,
  options: { storeName?: string; storeBreakdown?: string; notes?: string },
) {
  return [
    options.notes ? `Ajuste: ${options.notes}` : '',
    entry.dueDay ? `Vencimento: ${entry.dueDay}` : '',
    options.storeName ? `Loja: ${options.storeName}` : '',
    !options.storeName && entry.billingMode === 'per_store' && options.storeBreakdown
      ? `Lojas: ${options.storeBreakdown}`
      : '',
  ]
    .filter(Boolean)
    .join(' | ')
}
