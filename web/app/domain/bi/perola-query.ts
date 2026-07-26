import { BI_API_ENTITIES, biApiFieldLabel } from './api-catalog'
import type { BiApiEntity } from './api-catalog'

export type PerolaFilterValueType = 'string' | 'integer' | 'date' | 'boolean'

export interface PerolaDatasetOrder {
  field: string
  direction: 'ASC' | 'DESC'
}

export interface PerolaDatasetFilterRule {
  field: string
  valueType: PerolaFilterValueType
  operators: string[]
}

export interface PerolaDatasetFilterSelector {
  field: string
  operator: string
}

export interface PerolaDatasetDateRange {
  field: string
  maxDays: number
}

export interface PerolaDatasetCatalogItem {
  id: string
  label: string
  description: string
  defaultLimit: number
  maxLimit: number
  defaultOrderBy: PerolaDatasetOrder
  allowedOrderFields: string[]
  filters: PerolaDatasetFilterRule[]
  requiredFilterRule: string
  requiredFilterAlternatives: PerolaDatasetFilterSelector[][]
  dateRange?: PerolaDatasetDateRange
}

export interface PerolaDatasetQueryFilter {
  field: string
  operator: string
  value: string | number | boolean
}

export interface PerolaDatasetQueryInput {
  pageNumber: number
  limit: number
  orderBy: PerolaDatasetOrder
  filters: PerolaDatasetQueryFilter[]
}

export interface PerolaDatasetQueryResponse {
  datasetId: string
  datasetLabel: string
  pageNumber: number
  limit: number
  totalRecords: number
  totalPages: number
  returned: number
  hasMore: boolean
  orderBy: PerolaDatasetOrder
  filterCount: number
  durationMs: number
  records: Array<Record<string, unknown>>
}

export interface PerolaFilterDraft {
  id: string
  field: string
  operator: string
  value: string
}

export interface PerolaQueryColumn {
  id: string
  label: string
  width: string
  locked?: boolean
  defaultVisible: boolean
}

export const PEROLA_DATASET_ENTITY_IDS: Record<string, string> = {
  item: 'item',
  'imagem-item': 'image-item',
  'item-saldo-preco-compra': 'purchase-price',
  nota: 'invoice',
  'nota-item': 'invoice-item',
  inventario: 'inventory',
}

export const PEROLA_STATIC_DATASETS = Object.entries(PEROLA_DATASET_ENTITY_IDS)
  .map(([datasetId, entityId]) => {
    const entity = BI_API_ENTITIES.find((candidate) => candidate.id === entityId)
    return entity ? { id: datasetId, label: entity.label, description: entity.description } : null
  })
  .filter(
    (
      dataset,
    ): dataset is {
      id: string
      label: string
      description: string
    } => Boolean(dataset),
  )

export const PEROLA_OPERATOR_LABELS: Record<string, string> = {
  eq: 'Igual a',
  neq: 'Diferente de',
  gt: 'Maior que',
  gte: 'A partir de',
  lt: 'Menor que',
  lte: 'Até',
  contains: 'Contém',
  startsWith: 'Começa com',
  endsWith: 'Termina com',
}

function asRecord(value: unknown): Record<string, unknown> | null {
  if (!value || typeof value !== 'object' || Array.isArray(value)) return null
  return value as Record<string, unknown>
}

function asString(value: unknown) {
  return typeof value === 'string' ? value.trim() : ''
}

function asPositiveInteger(value: unknown) {
  const parsed = typeof value === 'number' ? value : Number(value)
  return Number.isInteger(parsed) && parsed > 0 ? parsed : 0
}

function asNonNegativeInteger(value: unknown) {
  const parsed = typeof value === 'number' ? value : Number(value)
  return Number.isInteger(parsed) && parsed >= 0 ? parsed : 0
}

function asStringArray(value: unknown) {
  return Array.isArray(value) ? value.map(asString).filter(Boolean) : []
}

function normalizeOrder(value: unknown): PerolaDatasetOrder | null {
  const record = asRecord(value)
  const field = asString(record?.field)
  const direction = asString(record?.direction).toUpperCase()
  if (!field || (direction !== 'ASC' && direction !== 'DESC')) return null
  return { field, direction }
}

function normalizeFilterRule(value: unknown): PerolaDatasetFilterRule | null {
  const record = asRecord(value)
  const field = asString(record?.field)
  const valueType = asString(record?.valueType)
  const operators = asStringArray(record?.operators)
  if (
    !field ||
    !operators.length ||
    !['string', 'integer', 'date', 'boolean'].includes(valueType)
  ) {
    return null
  }
  return { field, valueType: valueType as PerolaFilterValueType, operators }
}

function normalizeFilterSelector(value: unknown): PerolaDatasetFilterSelector | null {
  const record = asRecord(value)
  const field = asString(record?.field)
  const operator = asString(record?.operator)
  return field && operator ? { field, operator } : null
}

function normalizeRequiredAlternatives(value: unknown) {
  if (!Array.isArray(value)) return []
  return value
    .map((alternative) =>
      Array.isArray(alternative)
        ? alternative
            .map(normalizeFilterSelector)
            .filter((selector): selector is PerolaDatasetFilterSelector => Boolean(selector))
        : [],
    )
    .filter((alternative) => alternative.length > 0)
}

function normalizeDateRange(value: unknown): PerolaDatasetDateRange | undefined {
  const record = asRecord(value)
  const field = asString(record?.field)
  const maxDays = asPositiveInteger(record?.maxDays)
  return field && maxDays ? { field, maxDays } : undefined
}

export function normalizePerolaDatasetCatalog(payload: unknown): PerolaDatasetCatalogItem[] {
  const root = asRecord(payload)
  if (!Array.isArray(root?.datasets)) return []

  return root.datasets
    .map((candidate): PerolaDatasetCatalogItem | null => {
      const record = asRecord(candidate)
      const id = asString(record?.id)
      const label = asString(record?.label)
      const description = asString(record?.description)
      const defaultLimit = asPositiveInteger(record?.defaultLimit)
      const maxLimit = asPositiveInteger(record?.maxLimit)
      const defaultOrderBy = normalizeOrder(record?.defaultOrderBy)
      const allowedOrderFields = asStringArray(record?.allowedOrderFields)
      const filters = Array.isArray(record?.filters)
        ? record.filters
            .map(normalizeFilterRule)
            .filter((filter): filter is PerolaDatasetFilterRule => Boolean(filter))
        : []
      const requiredFilterRule = asString(record?.requiredFilterRule)
      const requiredFilterAlternatives = normalizeRequiredAlternatives(
        record?.requiredFilterAlternatives,
      )

      if (
        !id ||
        !label ||
        !defaultLimit ||
        !maxLimit ||
        !defaultOrderBy ||
        !allowedOrderFields.length ||
        !filters.length ||
        !requiredFilterRule ||
        !requiredFilterAlternatives.length
      ) {
        return null
      }

      return {
        id,
        label,
        description,
        defaultLimit,
        maxLimit,
        defaultOrderBy,
        allowedOrderFields,
        filters,
        requiredFilterRule,
        requiredFilterAlternatives,
        dateRange: normalizeDateRange(record?.dateRange),
      }
    })
    .filter((dataset): dataset is PerolaDatasetCatalogItem => Boolean(dataset))
}

export function normalizePerolaDatasetQueryResponse(
  payload: unknown,
): PerolaDatasetQueryResponse | null {
  const record = asRecord(payload)
  const datasetId = asString(record?.datasetId)
  const datasetLabel = asString(record?.datasetLabel)
  const pageNumber = asPositiveInteger(record?.pageNumber)
  const limit = asPositiveInteger(record?.limit)
  const orderBy = normalizeOrder(record?.orderBy)
  if (!datasetId || !datasetLabel || !pageNumber || !limit || !orderBy) return null

  const records = Array.isArray(record?.records)
    ? record.records.map(asRecord).filter((row): row is Record<string, unknown> => Boolean(row))
    : []

  return {
    datasetId,
    datasetLabel,
    pageNumber,
    limit,
    totalRecords: asNonNegativeInteger(record?.totalRecords),
    totalPages: asNonNegativeInteger(record?.totalPages),
    returned: asNonNegativeInteger(record?.returned),
    hasMore: record?.hasMore === true,
    orderBy,
    filterCount: asNonNegativeInteger(record?.filterCount),
    durationMs: asNonNegativeInteger(record?.durationMs),
    records,
  }
}

export function createInitialPerolaFilterDrafts(
  dataset: PerolaDatasetCatalogItem,
): PerolaFilterDraft[] {
  const preferred = dataset.requiredFilterAlternatives[0] || []
  return preferred.map((selector, index) => ({
    id: `${dataset.id}-${selector.field}-${selector.operator}-${index}`,
    field: selector.field,
    operator: selector.operator,
    value: '',
  }))
}

export function createEmptyPerolaFilterDraft(
  dataset: PerolaDatasetCatalogItem,
  index: number,
): PerolaFilterDraft {
  const rule = dataset.filters[0]
  return {
    id: `${dataset.id}-filter-${Date.now()}-${index}`,
    field: rule?.field || '',
    operator: rule?.operators[0] || 'eq',
    value: '',
  }
}

function isValidDateOnly(value: string) {
  if (!/^\d{4}-\d{2}-\d{2}$/.test(value)) return false
  const parsed = new Date(`${value}T00:00:00.000Z`)
  return !Number.isNaN(parsed.getTime()) && parsed.toISOString().startsWith(value)
}

function matchesRequiredAlternative(
  dataset: PerolaDatasetCatalogItem,
  filters: PerolaDatasetQueryFilter[],
) {
  const selectors = new Set(filters.map((filter) => `${filter.field}:${filter.operator}`))
  return dataset.requiredFilterAlternatives.some((alternative) =>
    alternative.every((selector) => selectors.has(`${selector.field}:${selector.operator}`)),
  )
}

function validateDateRange(dataset: PerolaDatasetCatalogItem, filters: PerolaDatasetQueryFilter[]) {
  if (!dataset.dateRange) return ''
  const from = filters.find(
    (filter) => filter.field === dataset.dateRange?.field && filter.operator === 'gte',
  )
  const to = filters.find(
    (filter) => filter.field === dataset.dateRange?.field && filter.operator === 'lte',
  )
  if (!from && !to) return ''
  if (!from || !to) return 'O período precisa ter data inicial e final.'

  const fromTime = new Date(`${from.value}T00:00:00.000Z`).getTime()
  const toTime = new Date(`${to.value}T00:00:00.000Z`).getTime()
  const inclusiveDays = Math.floor((toTime - fromTime) / 86_400_000) + 1
  if (inclusiveDays < 1 || inclusiveDays > dataset.dateRange.maxDays) {
    return `O período deve ter no máximo ${dataset.dateRange.maxDays} dias.`
  }
  return ''
}

export function buildPerolaQueryFilters(
  dataset: PerolaDatasetCatalogItem,
  drafts: PerolaFilterDraft[],
): { filters: PerolaDatasetQueryFilter[]; error: string } {
  if (!drafts.length) return { filters: [], error: dataset.requiredFilterRule }

  const filters: PerolaDatasetQueryFilter[] = []
  const selectors = new Set<string>()
  for (const draft of drafts) {
    const rule = dataset.filters.find((filter) => filter.field === draft.field)
    if (!rule || !rule.operators.includes(draft.operator)) {
      return { filters: [], error: 'Há um filtro ou operador inválido.' }
    }

    const rawValue = draft.value.trim()
    if (!rawValue) {
      return { filters: [], error: `Preencha o valor de ${biApiFieldLabel(draft.field)}.` }
    }

    const selector = `${draft.field}:${draft.operator}`
    if (selectors.has(selector)) {
      return { filters: [], error: 'O mesmo campo e operador não podem ser repetidos.' }
    }
    selectors.add(selector)

    let value: string | number | boolean = rawValue
    if (rule.valueType === 'integer') {
      const parsed = Number(rawValue)
      if (!Number.isInteger(parsed) || parsed <= 0) {
        return {
          filters: [],
          error: `${biApiFieldLabel(draft.field)} deve ser um inteiro positivo.`,
        }
      }
      value = parsed
    } else if (rule.valueType === 'boolean') {
      if (rawValue !== 'true' && rawValue !== 'false') {
        return { filters: [], error: `${biApiFieldLabel(draft.field)} deve ser Sim ou Não.` }
      }
      value = rawValue === 'true'
    } else if (rule.valueType === 'date' && !isValidDateOnly(rawValue)) {
      return { filters: [], error: `${biApiFieldLabel(draft.field)} deve ser uma data válida.` }
    }

    filters.push({ field: draft.field, operator: draft.operator, value })
  }

  if (!matchesRequiredAlternative(dataset, filters)) {
    return { filters: [], error: dataset.requiredFilterRule }
  }
  const dateError = validateDateRange(dataset, filters)
  return dateError ? { filters: [], error: dateError } : { filters, error: '' }
}

export function perolaDatasetEntity(datasetId: string): BiApiEntity | null {
  const entityId = PEROLA_DATASET_ENTITY_IDS[datasetId]
  return BI_API_ENTITIES.find((entity) => entity.id === entityId) || null
}

export function perolaQueryResultColumns(
  datasetId: string,
  records: Array<Record<string, unknown>>,
): PerolaQueryColumn[] {
  const entity = perolaDatasetEntity(datasetId)
  const schemaKeys =
    entity?.fieldGroups.flatMap((fieldGroup) => fieldGroup.fields.map((field) => field.key)) || []
  const returnedKeys = records.flatMap((record) => Object.keys(record))
  const knownKeys = new Set(schemaKeys)
  const extraKeys = Array.from(new Set(returnedKeys.filter((key) => !knownKeys.has(key)))).sort(
    (left, right) => left.localeCompare(right, 'pt-BR'),
  )
  const keys = Array.from(new Set([...schemaKeys, ...extraKeys]))

  return keys.map((key) => ({
    id: key,
    label: biApiFieldLabel(key),
    width: key === 'id' ? '100px' : 'minmax(150px, 1fr)',
    locked: key === 'id',
    defaultVisible: true,
  }))
}
