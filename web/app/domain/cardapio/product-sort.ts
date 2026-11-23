import { normalizeProductSearch } from './product-search'
import type { OmniTableSort } from '~/types/omni/collection'

// Ordenacao client-side da lista de produtos (a lista lean ja vem completa).
// Sem ordenacao ativa a tabela mantem a ordem do site (categoria -> produto).
export type ProductSort = OmniTableSort

export interface SortableProductRow {
  name: string
  categoryId: string | null
  priceReais: number
  compareAtPriceReais: number
  isAvailable: boolean
  isFeatured: boolean
  imageUrl: string
  createdAt?: string
}

// Opcoes do seletor mobile (sem cabecalho de tabela no celular).
export const PRODUCT_SORT_OPTIONS: Array<ProductSort & { label: string }> = [
  { key: 'name', dir: 'asc', label: 'Nome (A-Z)' },
  { key: 'name', dir: 'desc', label: 'Nome (Z-A)' },
  { key: 'categoryId', dir: 'asc', label: 'Categoria (A-Z)' },
  { key: 'priceReais', dir: 'asc', label: 'Menor preço' },
  { key: 'priceReais', dir: 'desc', label: 'Maior preço' },
  { key: 'isAvailable', dir: 'asc', label: 'Disponíveis primeiro' },
  { key: 'isAvailable', dir: 'desc', label: 'Indisponíveis primeiro' },
  { key: 'imageSrc', dir: 'asc', label: 'Com foto primeiro' },
  { key: 'imageSrc', dir: 'desc', label: 'Sem foto primeiro' },
  { key: 'createdAt', dir: 'desc', label: 'Mais recentes' },
  { key: 'createdAt', dir: 'asc', label: 'Mais antigos' },
]

const collator = new Intl.Collator('pt-BR', { sensitivity: 'base', numeric: true })

function compareText(a: string, b: string) {
  return collator.compare(normalizeProductSearch(a), normalizeProductSearch(b))
}

// Booleanos: "sim" vem antes no sentido crescente (disponivel, com foto, destaque).
function flag(value: boolean) {
  return value ? 0 : 1
}

function createdTime(value?: string) {
  const time = new Date(value ?? '').getTime()
  return Number.isFinite(time) ? time : 0
}

function comparator<Row extends SortableProductRow>(
  key: string,
  categoryName: (id: string) => string,
): ((a: Row, b: Row) => number) | null {
  switch (key) {
    case 'name':
      return (a, b) => compareText(a.name, b.name)
    case 'categoryId':
      return (a, b) =>
        compareText(categoryName(a.categoryId ?? ''), categoryName(b.categoryId ?? '')) ||
        compareText(a.name, b.name)
    case 'priceReais':
      return (a, b) => a.priceReais - b.priceReais
    case 'compareAtPriceReais':
      return (a, b) => a.compareAtPriceReais - b.compareAtPriceReais
    case 'isAvailable':
      return (a, b) => flag(a.isAvailable) - flag(b.isAvailable)
    case 'isFeatured':
      return (a, b) => flag(a.isFeatured) - flag(b.isFeatured)
    case 'imageSrc':
      return (a, b) => flag(Boolean(a.imageUrl?.trim())) - flag(Boolean(b.imageUrl?.trim()))
    case 'createdAt':
      return (a, b) => createdTime(a.createdAt) - createdTime(b.createdAt)
    default:
      return null
  }
}

// Ordenacao estavel: empates mantem a ordem do site (categoria -> produto).
export function sortProductRows<Row extends SortableProductRow>(
  rows: Row[],
  sort: ProductSort | null,
  categoryName: (id: string) => string,
): Row[] {
  const compare = sort ? comparator<Row>(sort.key, categoryName) : null
  if (!sort || !compare) return rows
  const direction = sort.dir === 'desc' ? -1 : 1
  return rows.slice().sort((a, b) => compare(a, b) * direction)
}
