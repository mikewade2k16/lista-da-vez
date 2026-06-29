package cardapio

import "time"

// DTOs e tipos de apoio do analytics do painel (Fase 10 / F2). Contrato camelCase;
// centavos como int64; duracoes em SEGUNDOS (campos *Seconds). Os horarios/janelas
// usam created_at do servidor (imune a clock-skew/forja); bots sao excluidos por
// padrao em toda agregacao. Fonte de verdade do contrato:
// docs/cardapio/PLANO_CARDAPIO_TRACKING_ANALYTICS.md (secoes 4/6/8.3).

// Limites e defaults comuns dos endpoints de analytics.
const (
	analyticsDefaultRangeDays = 30
	analyticsMaxSpanDays      = 90
	analyticsDefaultLimit     = 20
	analyticsMaxLimit         = 100
	analyticsDefaultTZ        = "America/Sao_Paulo"
)

// analyticsRange e a janela resolvida e validada de um pedido de analytics. From/To
// sao as datas (YYYY-MM-DD) ja interpretadas; FromTs/ToTsExclusive formam o intervalo
// half-open [FromTs, ToTsExclusive) em UTC, casando o indice (restaurant_id,
// created_at). Location e a IANA usada para histogramas por hora (at time zone).
type analyticsRange struct {
	From          string
	To            string
	FromTs        time.Time
	ToTsExclusive time.Time
	TZ            string
	Location      *time.Location
}

// topProductMetrics e a allowlist de metricas de top-products. Cada uma mapeia para
// uma contagem de evento (ou pedidos via order_items).
var topProductMetrics = map[string]struct{}{
	"viewed":      {},
	"clicked":     {},
	"add_to_cart": {},
	"orders":      {},
}

// sourceDimensions e a allowlist de dimensoes de sources (lidas de cardapio.sessions).
var sourceDimensions = map[string]struct{}{
	"utm_source":   {},
	"utm_medium":   {},
	"utm_campaign": {},
	"referrer":     {},
}

// timeseriesGranularities e a allowlist de granularidades da serie temporal.
var timeseriesGranularities = map[string]struct{}{
	"day":          {},
	"hour_of_day":  {},
	"weekday_hour": {},
}

// dwellDimensions e a allowlist de dimensoes do dwell.
var dwellDimensions = map[string]struct{}{
	"page":    {},
	"product": {},
	"section": {},
}

// ============================================================================
// Overview
// ============================================================================

// AnalyticsOverview agrega os KPIs do periodo. visitantes unicos vem em duas
// granularidades (sessao e device); conversao = pedidos/sessoes; ticket medio =
// soma(total_cents)/pedidos; abandono = sessoes com add_to_cart sem pedido sobre
// sessoes com add_to_cart.
type AnalyticsOverview struct {
	UniqueSessions      int64   `json:"uniqueSessions"`
	UniqueDevices       int64   `json:"uniqueDevices"`
	Sessions            int64   `json:"sessions"`
	Pageviews           int64   `json:"pageviews"`
	Events              int64   `json:"events"`
	Orders              int64   `json:"orders"`
	RevenueCents        int64   `json:"revenueCents"`
	ConversionRate      float64 `json:"conversionRate"`
	AvgTicketCents      int64   `json:"avgTicketCents"`
	AvgSessionSeconds   float64 `json:"avgSessionSeconds"`
	NewSessions         int64   `json:"newSessions"`
	ReturningSessions   int64   `json:"returningSessions"`
	CartAbandonmentRate float64 `json:"cartAbandonmentRate"`
	SessionsWithCart    int64   `json:"sessionsWithCart"`
	SessionsCartNoOrder int64   `json:"sessionsCartNoOrder"`
}

// overviewRaw sao os agregados crus que o store devolve; o service deriva taxas/medias.
type overviewRaw struct {
	UniqueSessions    int64
	UniqueDevices     int64
	Pageviews         int64
	Events            int64
	ReturningSessions int64
	TotalDurationMS   int64
	Orders            int64
	RevenueCents      int64
	SessionsWithCart  int64
	SessionsCartOrder int64
}

// ============================================================================
// Timeseries
// ============================================================================

// AnalyticsTimePoint e um ponto da serie. Para granularity=day, Bucket = data
// (YYYY-MM-DD). Para hour_of_day, Hour = 0..23. Para weekday_hour, Weekday = 0..6
// (0 = domingo) e Hour = 0..23.
type AnalyticsTimePoint struct {
	Bucket    string `json:"bucket,omitempty"`
	Weekday   int    `json:"weekday,omitempty"`
	Hour      int    `json:"hour"`
	Visits    int64  `json:"visits"`
	Sessions  int64  `json:"sessions"`
	Pageviews int64  `json:"pageviews"`
	Orders    int64  `json:"orders"`
}

// AnalyticsTimeseries e a serie densa (datas/horas sem dado vem com 0).
type AnalyticsTimeseries struct {
	Granularity string               `json:"granularity"`
	Points      []AnalyticsTimePoint `json:"points"`
}

// ============================================================================
// Funnel
// ============================================================================

// AnalyticsFunnelStep e uma etapa do funil por sessao. RateFromStart e a razao sobre
// a 1a etapa; RateFromPrev sobre a etapa anterior.
type AnalyticsFunnelStep struct {
	Key           string  `json:"key"`
	Label         string  `json:"label"`
	Sessions      int64   `json:"sessions"`
	RateFromStart float64 `json:"rateFromStart"`
	RateFromPrev  float64 `json:"rateFromPrev"`
}

// AnalyticsFunnel e o funil completo (etapas monotonicas por construcao do store).
type AnalyticsFunnel struct {
	Steps []AnalyticsFunnelStep `json:"steps"`
}

// ============================================================================
// Top products
// ============================================================================

// AnalyticsTopProduct e uma linha do ranking de produtos. Count e a metrica pedida;
// Viewed/Orders saem sempre para a conversao visto->comprado.
type AnalyticsTopProduct struct {
	ProductSlug    string  `json:"productSlug"`
	Name           string  `json:"name"`
	Count          int64   `json:"count"`
	Viewed         int64   `json:"viewed"`
	Orders         int64   `json:"orders"`
	ConversionRate float64 `json:"conversionRate"`
}

// topProductRaw e a linha crua do store (sem nome resolvido nem conversao calculada).
type topProductRaw struct {
	ProductSlug string
	Count       int64
	Viewed      int64
	Orders      int64
}

// AnalyticsTopProducts e o ranking + a metrica usada.
type AnalyticsTopProducts struct {
	Metric string                `json:"metric"`
	Items  []AnalyticsTopProduct `json:"items"`
}

// ============================================================================
// Sources
// ============================================================================

// AnalyticsSource e uma origem (utm/referrer) com sessoes e pedidos. Value vazio
// vira "(direto)" no service.
type AnalyticsSource struct {
	Value    string `json:"value"`
	Sessions int64  `json:"sessions"`
	Orders   int64  `json:"orders"`
}

// AnalyticsSources e o breakdown por origem + a dimensao usada.
type AnalyticsSources struct {
	Dimension string            `json:"dimension"`
	Items     []AnalyticsSource `json:"items"`
}

// ============================================================================
// Devices
// ============================================================================

// AnalyticsBreakdownItem e um par label/contagem (device_type/browser/os).
type AnalyticsBreakdownItem struct {
	Value    string `json:"value"`
	Sessions int64  `json:"sessions"`
}

// AnalyticsDevices agrupa os tres breakdowns de dispositivo (fonte = sessions).
type AnalyticsDevices struct {
	DeviceType []AnalyticsBreakdownItem `json:"deviceType"`
	Browser    []AnalyticsBreakdownItem `json:"browser"`
	OS         []AnalyticsBreakdownItem `json:"os"`
}

// ============================================================================
// Pages
// ============================================================================

// AnalyticsPage e uma pagina mais vista + dwell medio (segundos).
type AnalyticsPage struct {
	PagePath        string  `json:"pagePath"`
	Views           int64   `json:"views"`
	AvgDwellSeconds float64 `json:"avgDwellSeconds"`
}

// AnalyticsPages e a lista de paginas.
type AnalyticsPages struct {
	Items []AnalyticsPage `json:"items"`
}

// ============================================================================
// Dwell
// ============================================================================

// AnalyticsDwellItem e o tempo medio (segundos) por chave (page_path/product_slug/
// sectionId) + a contagem de amostras.
type AnalyticsDwellItem struct {
	Key             string  `json:"key"`
	AvgDwellSeconds float64 `json:"avgDwellSeconds"`
	Samples         int64   `json:"samples"`
}

// AnalyticsDwell e o tempo medio por dimensao escolhida.
type AnalyticsDwell struct {
	Dimension string               `json:"dimension"`
	Items     []AnalyticsDwellItem `json:"items"`
}

// ============================================================================
// Clicks
// ============================================================================

// AnalyticsClick e um botao/clique agregado por evento + label/kind do context.
type AnalyticsClick struct {
	Name  string `json:"name"`
	Label string `json:"label"`
	Kind  string `json:"kind"`
	Count int64  `json:"count"`
}

// AnalyticsClicks e a lista de cliques agregados.
type AnalyticsClicks struct {
	Items []AnalyticsClick `json:"items"`
}
