// Rótulos puros das telas de comprovantes, retenção e clientes (testáveis sem DOM).
import type {
  FinanceDocumentOriginal,
  FinanceFiscalCustomerVersion,
  FinanceReceiptItem,
} from '../types/records'

export const RETENTION_PRESETS = [
  { days: 30, label: '1 mês' },
  { days: 90, label: '3 meses' },
  { days: 180, label: '6 meses' },
  { days: 365, label: '1 ano' },
  { days: 1825, label: '5 anos' },
] as const

const dateFormat = new Intl.DateTimeFormat('pt-BR', {
  day: '2-digit',
  month: '2-digit',
  year: 'numeric',
})

export function formatRecordDate(value?: string): string {
  if (!value) return ''
  const day = /^(\d{4})-(\d{2})-(\d{2})$/.exec(value)
  if (day) return `${day[3]}/${day[2]}/${day[1]}`
  const date = new Date(value)
  return Number.isNaN(date.getTime()) ? '' : dateFormat.format(date)
}

export function formatBytes(bytes: number): string {
  if (!Number.isFinite(bytes) || bytes <= 0) return '0 KB'
  if (bytes < 1024 * 1024) return `${Math.max(1, Math.round(bytes / 1024))} KB`
  return `${(bytes / (1024 * 1024)).toFixed(1).replace('.', ',')} MB`
}

export function retentionDaysLabel(days: number): string {
  const preset = RETENTION_PRESETS.find((item) => item.days === days)
  return preset ? `${preset.label} (${days} dias)` : `${days} dias`
}

export type RecordTone = 'ok' | 'warning' | 'muted'

export function receiptLinkSummary(item: FinanceReceiptItem): { text: string; tone: RecordTone } {
  const link = item.link
  if (!link) return { text: 'Sem lançamento na planilha', tone: 'muted' }
  const where = item.sheetTitle ? ` em “${item.sheetTitle}”` : ''
  const kind = `${link.kind === 'saida' ? 'saída' : 'entrada'} ${link.scope}`
  switch (link.status) {
    case 'line_removed':
      return { text: `O lançamento${where} foi apagado da planilha`, tone: 'warning' }
    case 'amount_changed':
      return {
        text: `O valor do lançamento${where} foi alterado (comprovante: R$ ${link.amount})`,
        tone: 'warning',
      }
    default:
      return { text: `Lançado como ${kind}${where}`, tone: 'ok' }
  }
}

export function originalSummary(original?: FinanceDocumentOriginal): {
  text: string
  tone: RecordTone
} {
  if (!original) return { text: 'Original não guardado (enviado antes da retenção)', tone: 'muted' }
  switch (original.status) {
    case 'stored':
      return {
        text: `Original guardado até ${formatRecordDate(original.expiresAt)} · ${formatBytes(original.sizeBytes)}`,
        tone: 'ok',
      }
    case 'purged':
      return {
        text: `Original apagado pela retenção em ${formatRecordDate(original.purgedAt)}`,
        tone: 'muted',
      }
    case 'removed':
      return {
        text: `Original apagado manualmente em ${formatRecordDate(original.purgedAt)}`,
        tone: 'muted',
      }
    default:
      return { text: 'Original não guardado: armazenamento indisponível no envio', tone: 'warning' }
  }
}

export function customerVersionSummary(version: FinanceFiscalCustomerVersion): string {
  const data = version.data
  const amount =
    typeof data.amount === 'number'
      ? ` · R$ ${data.amount.toLocaleString('pt-BR', { minimumFractionDigits: 2 })}`
      : ''
  const origin = data.editedManually
    ? 'editado na tela'
    : version.sourceDraftId
      ? 'salvo pelo chat'
      : 'salvo'
  return `v${version.version} · ${formatRecordDate(version.createdAt)} · ${origin} · ${data.name ?? ''}${amount}`
}

export function maskDocument(document: string): string {
  if (document.length <= 4) return document
  return `${'•'.repeat(Math.max(0, document.length - 4))}${document.slice(-4)}`
}
