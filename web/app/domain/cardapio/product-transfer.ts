import type { Addon, ProductPairing, Variation } from '~/domain/cardapio/types'
import { slugify } from '~/domain/utils/slugify'

export type ProductTransferFormat = 'json' | 'csv'

export interface ProductTransferItem {
  categorySlug: string
  categoryName: string
  slug: string
  name: string
  shortDesc: string
  description: string
  body: string
  priceCents: number
  compareAtPriceCents: number
  imageUrl: string
  gallery: string[]
  weight: string
  cookTime: string
  diet: string[]
  allergens: string[]
  pairing: ProductPairing | null
  tags: string[]
  isAvailable: boolean
  isFeatured: boolean
  sortOrder: number
  variations: Array<Omit<Variation, 'id' | 'productId'>>
  addons: Array<Omit<Addon, 'id' | 'productId'>>
}

export interface ProductTransferDocument {
  version: number
  exportedAt: string
  products: ProductTransferItem[]
}

export interface ProductImportInput {
  updateExisting: boolean
  acceptedCategorySlugs: string[]
  products: ProductTransferItem[]
}

export interface ProductImportPreviewCategory {
  slug: string
  name: string
  productCount: number
}

export interface ProductImportPreview {
  newCategories: ProductImportPreviewCategory[]
}

export interface ProductImportCategoryDecision {
  originalSlug: string
  originalName: string
  name: string
  removed: boolean
}

export interface ProductImportError {
  row: number
  slug?: string
  message: string
}

export interface ProductImportResult {
  created: number
  updated: number
  skipped: number
  failed: number
  errors: ProductImportError[]
}

export function applyProductImportCategoryDecisions(
  products: ProductTransferItem[],
  decisions: ProductImportCategoryDecision[],
): Pick<ProductImportInput, 'acceptedCategorySlugs' | 'products'> {
  const decisionBySlug = new Map(
    decisions.map((decision) => [slugify(decision.originalSlug), decision]),
  )
  const acceptedCategorySlugs = new Set<string>()
  const mappedProducts = products.map((product) => {
    const decision = decisionBySlug.get(slugify(product.categorySlug))
    if (!decision) return product
    if (decision.removed) {
      return { ...product, categorySlug: '', categoryName: '' }
    }
    const name = decision.name.trim()
    const categorySlug =
      name === decision.originalName.trim() ? slugify(decision.originalSlug) : slugify(name)
    if (categorySlug) acceptedCategorySlugs.add(categorySlug)
    return { ...product, categorySlug, categoryName: name }
  })
  return { acceptedCategorySlugs: [...acceptedCategorySlugs], products: mappedProducts }
}

const CSV_HEADERS = [
  'categorySlug',
  'categoryName',
  'slug',
  'name',
  'shortDesc',
  'description',
  'body',
  'priceCents',
  'compareAtPriceCents',
  'imageUrl',
  'gallery',
  'weight',
  'cookTime',
  'diet',
  'allergens',
  'pairing',
  'tags',
  'isAvailable',
  'isFeatured',
  'sortOrder',
  'variations',
  'addons',
] as const

type TransferRecord = Record<string, unknown>

function asRecord(value: unknown): TransferRecord {
  return value && typeof value === 'object' && !Array.isArray(value)
    ? (value as TransferRecord)
    : {}
}

function asString(value: unknown): string {
  return String(value ?? '').trim()
}

function asNumber(value: unknown): number {
  const parsed = Number(value)
  return Number.isFinite(parsed) ? Math.round(parsed) : 0
}

function asBoolean(value: unknown, fallback = false): boolean {
  if (typeof value === 'boolean') return value
  const normalized = asString(value).toLowerCase()
  if (['true', '1', 'sim', 'yes'].includes(normalized)) return true
  if (['false', '0', 'nao', 'não', 'no'].includes(normalized)) return false
  return fallback
}

function parseJSONCell(value: unknown): unknown {
  if (typeof value !== 'string') return value
  const trimmed = value.trim()
  if (!trimmed) return null
  try {
    return JSON.parse(trimmed) as unknown
  } catch {
    return null
  }
}

function asStringArray(value: unknown): string[] {
  const parsed = parseJSONCell(value)
  if (Array.isArray(parsed)) {
    return parsed.map(asString).filter(Boolean)
  }
  return asString(value)
    .split('|')
    .map((item) => item.trim())
    .filter(Boolean)
}

function asPairing(value: unknown): ProductPairing | null {
  const record = asRecord(parseJSONCell(value))
  const name = asString(record.name)
  if (!name) return null
  return {
    name,
    type: asString(record.type),
    priceCents: asNumber(record.priceCents),
    ...(record.halfCents === undefined ? {} : { halfCents: asNumber(record.halfCents) }),
  }
}

function asVariations(value: unknown): Array<Omit<Variation, 'id' | 'productId'>> {
  const parsed = parseJSONCell(value)
  if (!Array.isArray(parsed)) return []
  return parsed.map((entry, index) => {
    const record = asRecord(entry)
    return {
      name: asString(record.name),
      priceDeltaCents: asNumber(record.priceDeltaCents),
      sortOrder: record.sortOrder === undefined ? index : asNumber(record.sortOrder),
    }
  })
}

function asAddons(value: unknown): Array<Omit<Addon, 'id' | 'productId'>> {
  const parsed = parseJSONCell(value)
  if (!Array.isArray(parsed)) return []
  return parsed.map((entry, index) => {
    const record = asRecord(entry)
    return {
      name: asString(record.name),
      priceCents: asNumber(record.priceCents),
      sortOrder: record.sortOrder === undefined ? index : asNumber(record.sortOrder),
    }
  })
}

function normalizeItem(value: unknown, index: number): ProductTransferItem {
  const record = asRecord(value)
  return {
    categorySlug: asString(record.categorySlug),
    categoryName: asString(record.categoryName),
    slug: asString(record.slug),
    name: asString(record.name),
    shortDesc: asString(record.shortDesc),
    description: asString(record.description),
    body: asString(record.body),
    priceCents: asNumber(record.priceCents),
    compareAtPriceCents: asNumber(record.compareAtPriceCents),
    imageUrl: asString(record.imageUrl),
    gallery: asStringArray(record.gallery),
    weight: asString(record.weight),
    cookTime: asString(record.cookTime),
    diet: asStringArray(record.diet),
    allergens: asStringArray(record.allergens),
    pairing: asPairing(record.pairing),
    tags: asStringArray(record.tags),
    isAvailable: asBoolean(record.isAvailable, true),
    isFeatured: asBoolean(record.isFeatured),
    sortOrder: record.sortOrder === undefined ? index : asNumber(record.sortOrder),
    variations: asVariations(record.variations),
    addons: asAddons(record.addons),
  }
}

function normalizeDocument(value: unknown): ProductTransferDocument {
  const record = asRecord(value)
  const rawProducts = Array.isArray(value)
    ? value
    : Array.isArray(record.products)
      ? record.products
      : []
  if (rawProducts.length === 0) {
    throw new Error('O arquivo nao contem produtos.')
  }
  return {
    version: asNumber(record.version) || 1,
    exportedAt: asString(record.exportedAt) || new Date().toISOString(),
    products: rawProducts.map(normalizeItem),
  }
}

function parseCSVRows(input: string): string[][] {
  const rows: string[][] = []
  let row: string[] = []
  let cell = ''
  let quoted = false

  for (let index = 0; index < input.length; index += 1) {
    const char = input[index]
    const next = input[index + 1]
    if (quoted && char === '"' && next === '"') {
      cell += '"'
      index += 1
      continue
    }
    if (char === '"') {
      quoted = !quoted
      continue
    }
    if (!quoted && char === ',') {
      row.push(cell)
      cell = ''
      continue
    }
    if (!quoted && (char === '\n' || char === '\r')) {
      if (char === '\r' && next === '\n') index += 1
      row.push(cell)
      if (row.some((value) => value.trim())) rows.push(row)
      row = []
      cell = ''
      continue
    }
    cell += char
  }
  row.push(cell)
  if (row.some((value) => value.trim())) rows.push(row)
  return rows
}

function parseCSV(input: string): ProductTransferDocument {
  const rows = parseCSVRows(input.replace(/^\uFEFF/, ''))
  if (rows.length < 2) {
    throw new Error('O CSV precisa ter cabecalho e pelo menos um produto.')
  }
  const headers = rows[0].map((header) => header.trim())
  const records = rows.slice(1).map((row) =>
    headers.reduce<TransferRecord>((record, header, index) => {
      if (header) record[header] = row[index] ?? ''
      return record
    }, {}),
  )
  return normalizeDocument(records)
}

export function detectProductTransferFormat(fileName: string): ProductTransferFormat {
  const extension = fileName.toLowerCase().split('.').pop()
  if (extension === 'json') return 'json'
  if (extension === 'csv') return 'csv'
  throw new Error('Formato nao suportado. Use um arquivo .json ou .csv.')
}

export function parseProductTransfer(
  input: string,
  format: ProductTransferFormat,
): ProductTransferDocument {
  if (format === 'csv') return parseCSV(input)
  try {
    return normalizeDocument(JSON.parse(input) as unknown)
  } catch (caught) {
    if (caught instanceof SyntaxError) {
      throw new Error('JSON invalido.')
    }
    if (caught instanceof Error) throw caught
    throw new Error('JSON invalido.')
  }
}

function csvCell(value: unknown): string {
  const text =
    typeof value === 'object' && value !== null ? JSON.stringify(value) : String(value ?? '')
  return `"${text.replace(/"/g, '""')}"`
}

export function serializeProductTransfer(
  document: ProductTransferDocument,
  format: ProductTransferFormat,
): string {
  if (format === 'json') return JSON.stringify(document, null, 2)
  const rows = document.products.map((product) =>
    CSV_HEADERS.map((header) => csvCell(product[header])).join(','),
  )
  return [CSV_HEADERS.join(','), ...rows].join('\r\n')
}
