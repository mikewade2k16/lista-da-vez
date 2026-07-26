export interface BiRecentSalesResponse {
  generatedAt: string
  periodStart: string
  periodEnd: string
  limit: number
  returned: number
  durationMs: number
  records: Array<Record<string, unknown>>
}

export interface BiRecentSalesColumn {
  id: string
  label: string
}

const preferredFields = [
  'dataVenda',
  'dtVenda',
  'data',
  'dataEmissao',
  'numDocumento',
  'documento',
  'vendaId',
  'id',
  'colaboradorNome',
  'colaborador',
  'vendedor',
  'cliente',
  'pessoaNomeRazaoSocial',
  'quantidade',
  'valorTotal',
  'total',
]

function asRecord(value: unknown): Record<string, unknown> | null {
  return value && typeof value === 'object' && !Array.isArray(value)
    ? (value as Record<string, unknown>)
    : null
}

function asNonNegativeInteger(value: unknown) {
  const parsed = Number(value)
  return Number.isInteger(parsed) && parsed >= 0 ? parsed : 0
}

export function normalizeBiRecentSales(payload: unknown): BiRecentSalesResponse | null {
  const record = asRecord(payload)
  if (!record || !Array.isArray(record.records)) return null

  const records = record.records
    .map(asRecord)
    .filter((item): item is Record<string, unknown> => Boolean(item))

  return {
    generatedAt: String(record.generatedAt || ''),
    periodStart: String(record.periodStart || ''),
    periodEnd: String(record.periodEnd || ''),
    limit: asNonNegativeInteger(record.limit),
    returned: asNonNegativeInteger(record.returned),
    durationMs: asNonNegativeInteger(record.durationMs),
    records,
  }
}

function fieldLabel(field: string) {
  const spaced = field
    .replace(/([a-z0-9])([A-Z])/g, '$1 $2')
    .replace(/[_-]+/g, ' ')
    .trim()
  return spaced ? spaced.charAt(0).toUpperCase() + spaced.slice(1) : field
}

export function biRecentSalesColumns(
  records: Array<Record<string, unknown>>,
): BiRecentSalesColumn[] {
  const available = Array.from(new Set(records.flatMap((record) => Object.keys(record))))
  const rank = new Map(preferredFields.map((field, index) => [field, index]))

  return available
    .sort((left, right) => {
      const leftRank = rank.get(left) ?? preferredFields.length
      const rightRank = rank.get(right) ?? preferredFields.length
      return leftRank - rightRank || left.localeCompare(right, 'pt-BR')
    })
    .slice(0, 8)
    .map((id) => ({ id, label: fieldLabel(id) }))
}

export function formatBiRecentSaleValue(value: unknown) {
  if (value === null || value === undefined || value === '') return '—'
  if (typeof value === 'boolean') return value ? 'Sim' : 'Não'
  if (typeof value === 'number') {
    return new Intl.NumberFormat('pt-BR', { maximumFractionDigits: 2 }).format(value)
  }
  return String(value)
}
