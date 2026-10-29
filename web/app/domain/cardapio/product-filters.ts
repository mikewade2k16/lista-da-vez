import { normalizeProductSearch } from './product-search'

export interface CardapioProductFilters {
  search?: unknown
  categoryId?: unknown
  availability?: unknown
  image?: unknown
  createdPeriod?: unknown
  createdFrom?: unknown
  createdTo?: unknown
}

export function productDateRange(filters: CardapioProductFilters, now = new Date()) {
  const start = new Date(now.getFullYear(), now.getMonth(), now.getDate())
  const end = new Date(start)
  end.setDate(end.getDate() + 1)
  switch (filters.createdPeriod) {
    case 'today':
      break
    case 'yesterday':
      start.setDate(start.getDate() - 1)
      end.setDate(end.getDate() - 1)
      break
    case 'this_week':
      start.setDate(start.getDate() - ((start.getDay() + 6) % 7))
      break
    case 'last_7_days':
      start.setDate(start.getDate() - 6)
      break
    case 'this_month':
      start.setDate(1)
      break
    case 'custom': {
      const from = parseDate(filters.createdFrom)
      const to = parseDate(filters.createdTo)
      if (to) to.setDate(to.getDate() + 1)
      return { from, to }
    }
    default:
      return { from: null, to: null }
  }
  return { from: start, to: end }
}

function parseDate(value: unknown): Date | null {
  if (typeof value !== 'string' || !/^\d{4}-\d{2}-\d{2}$/.test(value)) return null
  const [year, month, day] = value.split('-').map(Number)
  return new Date(year!, month! - 1, day!)
}

export function productMatchesFilters(
  product: {
    name: string
    categoryId: string | null
    isAvailable: boolean
    imageUrl: string
    createdAt?: string
  },
  filters: CardapioProductFilters,
  range = productDateRange(filters),
) {
  const query = normalizeProductSearch(filters.search)
  const category = String(filters.categoryId ?? '').trim()
  if (query && !normalizeProductSearch(product.name).includes(query)) return false
  if (category && product.categoryId !== category) return false
  if (typeof filters.availability === 'boolean' && product.isAvailable !== filters.availability)
    return false
  const hasImage = Boolean(product.imageUrl?.trim())
  if (filters.image === 'without' && hasImage) return false
  if (filters.image === 'with' && !hasImage) return false
  if (range.from || range.to) {
    const created = new Date(product.createdAt ?? '').getTime()
    if (!Number.isFinite(created)) return false
    if (range.from && created < range.from.getTime()) return false
    if (range.to && created >= range.to.getTime()) return false
  }
  return true
}

export const productExtraFilters = [
  {
    key: 'image',
    label: 'Foto',
    type: 'select' as const,
    placeholder: 'Foto do produto',
    options: [
      { label: 'Sem foto', value: 'without' },
      { label: 'Com foto', value: 'with' },
    ],
  },
  {
    key: 'createdPeriod',
    label: 'Data de cadastro',
    type: 'select' as const,
    placeholder: 'Data de cadastro',
    options: [
      { label: 'Hoje', value: 'today' },
      { label: 'Ontem', value: 'yesterday' },
      { label: 'Esta semana', value: 'this_week' },
      { label: 'Últimos 7 dias', value: 'last_7_days' },
      { label: 'Este mês', value: 'this_month' },
      { label: 'Escolher datas', value: 'custom' },
    ],
  },
]
