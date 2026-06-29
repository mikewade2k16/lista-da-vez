// Tipos do dashboard de Analytics do Cardapio (painel /cardapio -> aba Relatorios).
// Port EXATO do contrato da API de analytics (F2): base
// GET /v1/cardapio/restaurants/{id}/analytics/*. camelCase; dinheiro SEMPRE em
// centavos inteiros (`...Cents`); duracoes SEMPRE em SEGUNDOS (`...Seconds`);
// taxas como fracao 0..1 (`...Rate`). Fonte da verdade do shape:
// docs/cardapio/PLANO_CARDAPIO_TRACKING_ANALYTICS.md (secao 8.3 = endpoints).
//
// Os DTOs sao identicos aos do back (mesma decisao "DTOs identicos nos dois
// lados"). Nenhum campo carrega PII. value vazio em sources = origem "(direto)".

// Periodo de leitura comum a todos os blocos. from/to em YYYY-MM-DD (servidor
// aplica tz America/Sao_Paulo); span maximo 90 dias; default = ultimos 30 dias.
export interface AnalyticsRange {
  from: string
  to: string
}

// 1) overview — KPIs do periodo. Le sessions + orders.
export interface AnalyticsOverview {
  uniqueSessions: number
  uniqueDevices: number
  sessions: number
  pageviews: number
  events: number
  orders: number
  revenueCents: number
  conversionRate: number // 0..1 (orders / sessoes)
  avgTicketCents: number
  avgSessionSeconds: number
  newSessions: number
  returningSessions: number
  cartAbandonmentRate: number // 0..1
  sessionsWithCart: number
  sessionsCartNoOrder: number
}

// 2) timeseries — granularidade define quais campos vem preenchidos:
//  - day:          bucket (YYYY-MM-DD), serie densa preenchendo todas as datas.
//  - hour_of_day:  hour (0..23), 24 pontos.
//  - weekday_hour: weekday (0..6, 0=domingo) + hour (0..23), 7x24 = 168 pontos.
export type AnalyticsGranularity = 'day' | 'hour_of_day' | 'weekday_hour'

export interface AnalyticsTimeseriesPoint {
  bucket?: string // YYYY-MM-DD (granularity=day)
  weekday?: number // 0..6, 0=domingo (granularity=weekday_hour)
  hour: number // 0..23
  visits: number
  sessions: number
  pageviews: number
  orders: number
}

export interface AnalyticsTimeseries {
  granularity: AnalyticsGranularity
  points: AnalyticsTimeseriesPoint[]
}

// 3) funnel — etapas por sessao, da visita ao pedido. rate* como fracao 0..1.
export interface AnalyticsFunnelStep {
  key: string
  label: string
  sessions: number
  rateFromStart: number // 0..1 (vs primeira etapa)
  rateFromPrev: number // 0..1 (vs etapa anterior)
}

export interface AnalyticsFunnel {
  steps: AnalyticsFunnelStep[]
}

// 4) top-products — metrica selecionavel; conversao visto->comprado por produto.
export type AnalyticsTopProductsMetric = 'viewed' | 'clicked' | 'add_to_cart' | 'orders'

export interface AnalyticsTopProductItem {
  productSlug: string
  name: string
  count: number // valor da metrica selecionada
  viewed: number
  orders: number
  conversionRate: number // 0..1 (visto->comprado)
}

export interface AnalyticsTopProducts {
  metric: AnalyticsTopProductsMetric
  items: AnalyticsTopProductItem[]
}

// 5) sources — origem do trafego. value vazio ("") => exibir "(direto)".
export type AnalyticsSourceDimension = 'utm_source' | 'utm_medium' | 'utm_campaign' | 'referrer'

export interface AnalyticsSourceItem {
  value: string // "" => "(direto)"
  sessions: number
  orders: number
}

export interface AnalyticsSources {
  dimension: AnalyticsSourceDimension
  items: AnalyticsSourceItem[]
}

// 6) devices — breakdown por tipo de dispositivo / navegador / sistema.
export interface AnalyticsDeviceItem {
  value: string
  sessions: number
}

export interface AnalyticsDevices {
  deviceType: AnalyticsDeviceItem[]
  browser: AnalyticsDeviceItem[]
  os: AnalyticsDeviceItem[]
}

// 7) pages — paginas mais vistas + dwell medio (segundos).
export interface AnalyticsPageItem {
  pagePath: string
  views: number
  avgDwellSeconds: number
}

export interface AnalyticsPages {
  items: AnalyticsPageItem[]
}

// 8) dwell — tempo medio por pagina/produto/secao (so eventos final:true).
export type AnalyticsDwellDimension = 'page' | 'product' | 'section'

export interface AnalyticsDwellItem {
  key: string
  avgDwellSeconds: number
  samples: number
}

export interface AnalyticsDwell {
  dimension: AnalyticsDwellDimension
  items: AnalyticsDwellItem[]
}

// 9) clicks — quais botoes foram clicados, agregados por label/kind.
export interface AnalyticsClickItem {
  name: string
  label: string
  kind: string
  count: number
}

export interface AnalyticsClicks {
  items: AnalyticsClickItem[]
}

// --- Defaults / limites do contrato ---

// Span maximo aceito pela API (dias). O front nao precisa cravar isso no fetch,
// mas usa para validar/clampar o range do toolbar antes de chamar.
export const ANALYTICS_MAX_SPAN_DAYS = 90

// Janela default da aba (decisao F4: ultimos 7 dias no Toolbar; a API aceita ate
// 90 e usa 30 como default proprio quando from/to vem vazios).
export const ANALYTICS_DEFAULT_RANGE_DAYS = 7

// Limite default de itens em listas (top-products/sources/pages/dwell/clicks).
export const ANALYTICS_DEFAULT_LIMIT = 20

// --- Helpers de formatacao (duracoes em SEGUNDOS; taxas em fracao 0..1) ---

const integerFormatter = new Intl.NumberFormat('pt-BR', { maximumFractionDigits: 0 })

// Inteiro pt-BR (visitas, sessoes, contagens). NaN/ausente => "0".
export function formatAnalyticsInt(value: number | null | undefined): string {
  return integerFormatter.format(Number.isFinite(value) ? Number(value) : 0)
}

// Fracao 0..1 -> "12,3%". 1 casa decimal (taxas costumam ser pequenas).
export function formatAnalyticsRate(value: number | null | undefined): string {
  const fraction = Number.isFinite(value) ? Number(value) : 0
  return `${(fraction * 100).toLocaleString('pt-BR', {
    minimumFractionDigits: 1,
    maximumFractionDigits: 1,
  })}%`
}

// Segundos -> "mm:ss" (ex.: 95s => "1:35"). Usado em dwell/tempo de sessao.
export function formatAnalyticsDuration(seconds: number | null | undefined): string {
  const total = Math.max(0, Math.round(Number.isFinite(seconds) ? Number(seconds) : 0))
  const minutes = Math.floor(total / 60)
  const rest = total % 60
  return `${minutes}:${String(rest).padStart(2, '0')}`
}
