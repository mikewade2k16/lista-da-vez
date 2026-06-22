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
  // Campos extras (Fase 2) — opcionais; gravados dentro do address jsonb.
  number?: string
  complement?: string
  reference?: string
}

export interface RestaurantHour {
  days: string
  hours: string
}

// Pagamento informativo (Fase 2 — WS-B). Salvo em settings.payment (jsonb);
// NAO entra no checkout, so e exibido no site publico. Bandeiras = lista livre.
export interface PaymentCard {
  accepted: boolean
  brands: string[]
}

export interface RestaurantPayment {
  cash: boolean
  debit: PaymentCard
  credit: PaymentCard
  pix: boolean
  ticket: boolean
  other: string
}

export interface RestaurantSettings {
  deliveryFeeCents: number
  deliveryEnabled: boolean
  pickupEnabled: boolean
  dineInEnabled: boolean
  minOrderCents: number
  freeDeliveryAboveCents: number
  payment?: RestaurantPayment
}

// Tema RICO (Fase 2 — WS-D). Salvo em restaurant.theme (jsonb livre) e editado
// pela secao Aparencia. Superset do shape curado antigo ({base,accent,font,mode}):
// agora carrega uma PALETA semantica de 5 cores + 2 familias de fonte + raio dos
// cantos. As cores sao HEX escolhidos pelo usuario (DADO, nao token do design
// system do painel). `base` vira apenas um preset/ponto de partida. O front
// publico (TAVOLA) aplica este mapa — ver THEME_SEMANTIC_MAP.
export type ThemeMode = 'light' | 'dark'
export type ThemeRadius = 'reto' | 'suave' | 'arredondado'

export interface ThemeColors {
  background: string
  surface: string
  text: string
  accent: string
  border: string
}

export interface ThemeFonts {
  display: string
  body: string
}

export interface RestaurantTheme {
  base: string
  mode: ThemeMode
  colors: ThemeColors
  fonts: ThemeFonts
  radius: ThemeRadius
}

// --- Site Builder (Studio do TAVOLA embutido por iframe — desenho B4) ---
// O layout do site e um documento livre editado no Studio do TAVOLA (iframe) e
// SALVO/PUBLICADO pelo painel (que detem o JWT; o iframe nunca recebe token).
// Contrato espelhado dos dois lados; protocolo postMessage no canal
// 'omni-studio'. Endpoints: GET/PUT .../layout (rascunho, com versao p/
// concorrencia via If-Match) e POST .../layout/publish.

// Overrides de tema do site (livre). Espelha o RestaurantTheme rico, porem todos
// os campos sao opcionais — o Studio so envia o que sobrescreve.
export interface ThemeOverrides {
  base?: string
  mode?: ThemeMode
  colors?: Partial<ThemeColors>
  fonts?: Partial<ThemeFonts>
  radius?: ThemeRadius
}

// Bloco de uma pagina do site. `props` e livre (o Studio define o shape de cada
// `type`); `visible` controla render no site publico.
export interface LayoutBlock {
  id: string
  type: string
  props: Record<string, unknown>
  visible: boolean
}

// Uma pagina do site (ex.: "home", "cardapio"): identificador + blocos ordenados.
export interface PageLayout {
  page: string
  blocks: LayoutBlock[]
}

// Documento de layout do site inteiro. Vazio = `{ pages: {} }`.
export interface SiteLayout {
  pages: Record<string, PageLayout>
  theme?: ThemeOverrides
  updatedAt?: string
}

export interface Restaurant {
  id: string
  slug: string
  name: string
  tagline: string
  description: string
  segment: string
  logoUrl: string
  bannerUrl: string
  whatsapp: string
  phone: string
  email: string
  instagram: string
  facebook: string
  youtube: string
  googleAnalyticsId: string
  facebookPixelId: string
  customHeadHtml: string
  address: RestaurantAddress
  hours: RestaurantHour[]
  settings: RestaurantSettings
  theme: Record<string, unknown>
  isActive: boolean
  createdAt: string
  updatedAt: string
}

// Zona de entrega (Fase 2 — WS-A): bairro + valor do frete. Centavos inteiros.
export interface DeliveryZone {
  id: string
  restaurantId: string
  name: string
  feeCents: number
  isActive: boolean
  sortOrder: number
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
  // WS-F: foto da categoria (opcional no contrato). productCount e derivado no
  // menu publico (omitempty no back; ausente => o front deriva).
  imageUrl: string
  sortOrder: number
  isActive: boolean
  createdAt: string
  productCount?: number
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
  // WS-F: preco "cheio" para exibicao riscada (promocao). Ausente = sem risco.
  compareAtPriceCents?: number
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
  // Código curto voltado ao cliente (WS-G); o atendente usa para casar o pedido.
  code: string
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

// --- Aparencia RICA: listas curadas + presets + normalize (WS-D) ---

// Cada chave de cor SEMANTICA -> o que ela pinta no site publico (TAVOLA). Este
// e o CONTRATO que o front publico aplica; documentado tambem em THEME_SEMANTIC_MAP.
export const CARDAPIO_THEME_COLOR_FIELDS: {
  key: keyof ThemeColors
  label: string
  hint: string
}[] = [
  { key: 'background', label: 'Fundo', hint: 'Fundo da pagina' },
  { key: 'surface', label: 'Superficie', hint: 'Cards e blocos' },
  { key: 'text', label: 'Texto', hint: 'Cor do texto' },
  { key: 'accent', label: 'Destaque', hint: 'Botoes, precos, links' },
  { key: 'border', label: 'Bordas', hint: 'Linhas e contornos' },
]

// Familias de fonte registradas no front publico (TAVOLA). Lista CURADA — precisa
// estar carregada no TAVOLA. `display` = titulos; `body` = corpo. value = familia.
export const CARDAPIO_THEME_FONTS: { value: string; label: string }[] = [
  { value: 'Cormorant Garamond', label: 'Cormorant Garamond (serifada)' },
  { value: 'Fraunces', label: 'Fraunces (serifada)' },
  { value: 'DM Sans', label: 'DM Sans (sem serifa)' },
  { value: 'Space Grotesk', label: 'Space Grotesk (sem serifa)' },
  { value: 'JetBrains Mono', label: 'JetBrains Mono (monoespacada)' },
]

export const CARDAPIO_THEME_MODES: { value: ThemeMode; label: string }[] = [
  { value: 'light', label: 'Claro' },
  { value: 'dark', label: 'Escuro' },
]

export const CARDAPIO_THEME_RADII: { value: ThemeRadius; label: string; px: string }[] = [
  { value: 'reto', label: 'Reto', px: '0px' },
  { value: 'suave', label: 'Suave', px: '10px' },
  { value: 'arredondado', label: 'Arredondado', px: '20px' },
]

// Presets: pontos de partida que preenchem cores/fontes/modo/cantos de uma vez.
// `value` vai em theme.base (identidade escolhida). As cores sao DADO (hex livre).
export interface CardapioThemePreset {
  value: string
  label: string
  description: string
  theme: Omit<RestaurantTheme, 'base'>
}

export const CARDAPIO_THEME_PRESETS: CardapioThemePreset[] = [
  {
    value: 'tavola',
    label: 'Tavola',
    description: 'Escuro, serifado e dourado — alta gastronomia.',
    theme: {
      mode: 'dark',
      colors: {
        background: '#11100c',
        surface: '#1c1a14',
        text: '#f4efe6',
        accent: '#c9a227',
        border: '#3a352a',
      },
      fonts: { display: 'Cormorant Garamond', body: 'DM Sans' },
      radius: 'suave',
    },
  },
  {
    value: 'brasa',
    label: 'Brasa',
    description: 'Claro e quente — churrascaria e brasa.',
    theme: {
      mode: 'light',
      colors: {
        background: '#fbf6f1',
        surface: '#ffffff',
        text: '#2b1b14',
        accent: '#c0392b',
        border: '#e7d6c9',
      },
      fonts: { display: 'Fraunces', body: 'Space Grotesk' },
      radius: 'arredondado',
    },
  },
]

// Mantido por compat: alguns lugares ainda esperam a lista de bases (label/value).
export const CARDAPIO_THEME_BASES: { value: string; label: string }[] = CARDAPIO_THEME_PRESETS.map(
  (preset) => ({ value: preset.value, label: preset.label }),
)

export function findThemePreset(base: string): CardapioThemePreset {
  return CARDAPIO_THEME_PRESETS.find((preset) => preset.value === base) ?? CARDAPIO_THEME_PRESETS[0]
}

// Acento default quando o restaurante ainda nao escolheu cor (preset padrao).
export const CARDAPIO_THEME_DEFAULT_ACCENT = CARDAPIO_THEME_PRESETS[0].theme.colors.accent

// Valida/normaliza um hex (#rgb ou #rrggbb). Fallback se invalido.
function normalizeHex(value: unknown, fallback: string): string {
  const raw = String(value ?? '').trim()
  return /^#([0-9a-fA-F]{3}|[0-9a-fA-F]{6})$/.test(raw) ? raw : fallback
}

// Normaliza o theme jsonb livre no shape RICO (WS-D), com defaults do preset e
// MIGRACAO do shape antigo {base, accent, font, mode}: o accent antigo vira
// colors.accent, a font antiga vira fonts.display, e o resto herda do preset.
export function normalizeTheme(theme: Record<string, unknown> | null | undefined): RestaurantTheme {
  const source = (theme ?? {}) as Record<string, unknown>
  const base = String(source.base ?? '') || CARDAPIO_THEME_PRESETS[0].value
  const preset = findThemePreset(base)
  const fallback = preset.theme

  const mode: ThemeMode =
    source.mode === 'light' || source.mode === 'dark' ? source.mode : fallback.mode

  const colorsSource = (source.colors ?? {}) as Record<string, unknown>
  // Back-compat: shape antigo so tinha `accent` no topo (hex) -> colors.accent.
  const legacyAccent = normalizeHex(source.accent, fallback.colors.accent)
  const colors: ThemeColors = {
    background: normalizeHex(colorsSource.background, fallback.colors.background),
    surface: normalizeHex(colorsSource.surface, fallback.colors.surface),
    text: normalizeHex(colorsSource.text, fallback.colors.text),
    accent: normalizeHex(colorsSource.accent, legacyAccent),
    border: normalizeHex(colorsSource.border, fallback.colors.border),
  }

  const fontsSource = (source.fonts ?? {}) as Record<string, unknown>
  // Back-compat: shape antigo so tinha `font` no topo -> fonts.display.
  const legacyFont = String(source.font ?? '') || fallback.fonts.display
  const fonts: ThemeFonts = {
    display: String(fontsSource.display ?? '') || legacyFont,
    body: String(fontsSource.body ?? '') || fallback.fonts.body,
  }

  const radius: ThemeRadius =
    source.radius === 'reto' || source.radius === 'suave' || source.radius === 'arredondado'
      ? source.radius
      : fallback.radius

  return { base, mode, colors, fonts, radius }
}

// MAPA semantico -> CSS custom properties do PREVIEW (e do TAVOLA). O nome da var
// e o mesmo dos dois lados (prefixo --prev- no painel). Documentado em
// THEME_SEMANTIC_MAP. O TAVOLA aplica o equivalente no seu proprio prefixo.
export function themeToPreviewVars(theme: RestaurantTheme): Record<string, string> {
  const radiusPx = CARDAPIO_THEME_RADII.find((item) => item.value === theme.radius)?.px ?? '10px'
  return {
    '--prev-bg': theme.colors.background,
    '--prev-surface': theme.colors.surface,
    '--prev-text': theme.colors.text,
    '--prev-accent': theme.colors.accent,
    '--prev-border': theme.colors.border,
    '--prev-display': `'${theme.fonts.display}', serif`,
    '--prev-body': `'${theme.fonts.body}', sans-serif`,
    '--prev-radius': radiusPx,
  }
}

// Documentacao do contrato semantico (para aplicar no TAVOLA). NAO e usado em
// runtime — serve de referencia unica do que cada token pinta.
export const THEME_SEMANTIC_MAP = {
  'colors.background': 'fundo da pagina',
  'colors.surface': 'fundo de cards/blocos',
  'colors.text': 'cor do texto',
  'colors.accent': 'destaque: botoes, precos, links, primaria',
  'colors.border': 'linhas, contornos, divisores',
  'fonts.display': 'familia dos titulos (headings)',
  'fonts.body': 'familia do corpo de texto',
  radius: 'raio dos cantos (cards, botoes, imagens)',
  mode: 'esquema claro/escuro do site',
} as const

// --- Pagamento: builder do shape default (WS-B) ---

export function emptyPayment(): RestaurantPayment {
  return {
    cash: false,
    debit: { accepted: false, brands: [] },
    credit: { accepted: false, brands: [] },
    pix: false,
    ticket: false,
    other: '',
  }
}

// Normaliza settings.payment (jsonb opcional) no shape completo, com defaults.
export function normalizePayment(payment: RestaurantPayment | null | undefined): RestaurantPayment {
  const source = payment ?? null
  return {
    cash: Boolean(source?.cash),
    debit: {
      accepted: Boolean(source?.debit?.accepted),
      brands: Array.isArray(source?.debit?.brands) ? source.debit.brands.map(String) : [],
    },
    credit: {
      accepted: Boolean(source?.credit?.accepted),
      brands: Array.isArray(source?.credit?.brands) ? source.credit.brands.map(String) : [],
    },
    pix: Boolean(source?.pix),
    ticket: Boolean(source?.ticket),
    other: String(source?.other ?? ''),
  }
}

// Converte "Visa, Mastercard" -> ["Visa","Mastercard"] (e o inverso na UI).
export function parseBrands(input: string): string[] {
  return String(input || '')
    .split(',')
    .map((brand) => brand.trim())
    .filter(Boolean)
}

export function formatBrands(brands: string[] | null | undefined): string {
  return (Array.isArray(brands) ? brands : []).join(', ')
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
