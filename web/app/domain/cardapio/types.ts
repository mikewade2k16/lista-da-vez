// Tipos do modulo Cardapio Online (painel /cardapio).
// Port EXATO do contrato da API publica do front cardapio (camelCase).
// Dinheiro SEMPRE em centavos inteiros (`...Cents`). Nunca float de reais.
// Fonte da verdade do shape: docs/cardapio/PLANO_MODULO_CARDAPIO.md (§4/§5).

export interface RestaurantAddress {
  street: string
  neighborhood: string
  city: string
  state: string
  zip: string
}

export interface RestaurantHour {
  days: string
  hours: string
}

export interface RestaurantSettings {
  deliveryFeeCents: number
  deliveryEnabled: boolean
  pickupEnabled: boolean
  dineInEnabled: boolean
  minOrderCents: number
  freeDeliveryAboveCents: number
}

export interface Restaurant {
  id: string
  slug: string
  name: string
  tagline: string
  description: string
  logoUrl: string
  bannerUrl: string
  whatsapp: string
  phone: string
  email: string
  instagram: string
  address: RestaurantAddress
  hours: RestaurantHour[]
  settings: RestaurantSettings
  theme: Record<string, unknown>
  isActive: boolean
  createdAt: string
  updatedAt: string
}

// Projecao lean da listagem (GET /v1/cardapio/restaurants).
export interface RestaurantListItem {
  id: string
  accountId: string
  accountName: string
  slug: string
  name: string
  isActive: boolean
  primaryDomain: string
  updatedAt: string
}

export interface Category {
  id: string
  restaurantId: string
  slug: string
  name: string
  description: string
  sortOrder: number
  isActive: boolean
  createdAt: string
}

export interface Variation {
  id: string
  productId: string
  name: string
  priceDeltaCents: number
  sortOrder: number
}

export interface Addon {
  id: string
  productId: string
  name: string
  priceCents: number
  sortOrder: number
}

export interface ProductPairing {
  name: string
  type: string
  priceCents: number
  halfCents?: number
}

export interface Product {
  id: string
  restaurantId: string
  categoryId: string | null
  slug: string
  name: string
  shortDesc: string
  description: string
  body: string
  priceCents: number
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
  rating: number | null
  reviewCount: number
  soldCount: number
  createdAt: string
  updatedAt: string
  variations: Variation[]
  addons: Addon[]
}

// Projecao lean da listagem de produtos (GET .../products).
export interface ProductListItem {
  id: string
  restaurantId: string
  categoryId: string | null
  slug: string
  name: string
  priceCents: number
  imageUrl: string
  isAvailable: boolean
  isFeatured: boolean
  sortOrder: number
}

export interface Review {
  id: string
  restaurantId: string
  productId: string
  authorName: string
  authorLevel: string
  rating: number
  body: string
  isHighlight: boolean
  dateLabel: string
  sortOrder: number
  createdAt: string
}

export type OrderStatus =
  | 'recebido'
  | 'em_preparo'
  | 'pronto'
  | 'saiu_entrega'
  | 'entregue'
  | 'cancelado'

export type OrderType = 'retirada' | 'entrega' | 'local'

export interface OrderItemAddon {
  name: string
  priceCents: number
}

export interface OrderItem {
  id: string
  orderId: string
  productId: string | null
  productName: string
  variationName: string
  addons: OrderItemAddon[]
  quantity: number
  unitPriceCents: number
  totalCents: number
  notes: string
}

export interface Order {
  id: string
  restaurantId: string
  customerId: string | null
  orderNumber: number
  status: OrderStatus
  type: OrderType
  customerName: string
  customerPhone: string
  deliveryAddress: string
  notes: string
  subtotalCents: number
  deliveryFeeCents: number
  discountCents: number
  totalCents: number
  createdAt: string
  updatedAt: string
  items: OrderItem[]
}

export interface RestaurantDomain {
  host: string
  restaurantId: string
  isPrimary: boolean
  createdAt: string
}

export interface CardapioEvent {
  id: string
  restaurantId: string
  name: string
  sessionId: string
  context: Record<string, unknown>
  createdAt: string
}

// --- Labels de status/tipo (UI em pt-BR) ---

export const ORDER_STATUS_LABELS: Record<OrderStatus, string> = {
  recebido: 'Recebido',
  em_preparo: 'Em preparo',
  pronto: 'Pronto',
  saiu_entrega: 'Saiu para entrega',
  entregue: 'Entregue',
  cancelado: 'Cancelado',
}

export const ORDER_STATUS_ORDER: OrderStatus[] = [
  'recebido',
  'em_preparo',
  'pronto',
  'saiu_entrega',
  'entregue',
  'cancelado',
]

export const ORDER_TYPE_LABELS: Record<OrderType, string> = {
  retirada: 'Retirada',
  entrega: 'Entrega',
  local: 'Consumo no local',
}

// --- Helpers de dinheiro (modelo guarda centavos; UI exibe R$) ---

// Converte centavos inteiros para string "1.234,56" (sem prefixo R$).
export function formatCents(cents: number | null | undefined): string {
  const value = Number.isFinite(cents) ? Number(cents) : 0
  return (value / 100).toLocaleString('pt-BR', {
    minimumFractionDigits: 2,
    maximumFractionDigits: 2,
  })
}

// Exibe "R$ 1.234,56".
export function formatCurrency(cents: number | null | undefined): string {
  return `R$ ${formatCents(cents)}`
}

// Converte input do usuario ("12,90", "R$ 12.90", "12") em centavos inteiros.
// Aceita virgula ou ponto como separador decimal; ignora separador de milhar.
export function parseCents(input: string | number | null | undefined): number {
  if (typeof input === 'number') {
    return Math.round(input * 100)
  }

  const raw = String(input ?? '').trim()
  if (!raw) {
    return 0
  }

  // Remove tudo exceto digitos, virgula, ponto e sinal.
  const cleaned = raw.replace(/[^\d,.-]/g, '')
  if (!cleaned) {
    return 0
  }

  const negative = cleaned.startsWith('-')
  const unsigned = cleaned.replace(/-/g, '')

  // Ultimo separador (virgula ou ponto) define a parte decimal.
  const lastComma = unsigned.lastIndexOf(',')
  const lastDot = unsigned.lastIndexOf('.')
  const decimalPos = Math.max(lastComma, lastDot)

  let integerPart = unsigned
  let decimalPart = ''

  if (decimalPos >= 0) {
    integerPart = unsigned.slice(0, decimalPos)
    decimalPart = unsigned.slice(decimalPos + 1)
  }

  const digitsInteger = integerPart.replace(/[^\d]/g, '') || '0'
  const digitsDecimal = (decimalPart.replace(/[^\d]/g, '') + '00').slice(0, 2)
  const cents = Number(digitsInteger) * 100 + Number(digitsDecimal)

  return negative ? -cents : cents
}

// Normaliza um texto livre em slug (lowercase, hifens). Espelha o backend.
export function slugify(value: string): string {
  return String(value || '')
    .normalize('NFD')
    .replace(/[̀-ͯ]/g, '')
    .trim()
    .toLowerCase()
    .replace(/[_\s]+/g, '-')
    .replace(/[^a-z0-9-]+/g, '-')
    .replace(/-+/g, '-')
    .replace(/^-|-$/g, '')
}
