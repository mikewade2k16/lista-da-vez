package cardapio

import (
	"context"
	"sort"
	"strings"
	"time"
)

// Camada de regras do analytics do painel (Fase 10 / F2). Valida o range, garante o
// pertencimento do restaurante (404 uniforme fora do escopo), chama o store e calcula
// os derivados (conversao sem divisao por zero, taxas do funil, ranking, serie densa,
// resolucao de nomes). Nunca toca SQL — so orquestra o store. Fonte do contrato:
// docs/cardapio/PLANO_CARDAPIO_TRACKING_ANALYTICS.md (secoes 4/6/8.3).

// AnalyticsParams sao os parametros comuns crus (da query) antes da validacao.
type AnalyticsParams struct {
	From string
	To   string
	TZ   string
}

// ensureRestaurant valida o PERTENCIMENTO do restaurante a account (defesa em
// profundidade do escopo). 0 linhas / erro de nao-encontrado => ErrNotFound (404
// uniforme). Chamado 1x por endpoint, ANTES de qualquer agregacao.
func (s *Service) ensureRestaurant(ctx context.Context, accountID, restaurantID string) error {
	if _, err := s.store.GetRestaurant(ctx, accountID, restaurantID); err != nil {
		return mapStoreErr(err)
	}
	return nil
}

// resolveRange interpreta from/to/tz, aplicando defaults e os limites do contrato:
// range default = ultimos 30 dias; from <= to; to clampa em hoje; span maximo 90
// dias (senao ErrValidation). O intervalo vira half-open [FromTs, ToTsExclusive) em
// UTC (FromTs = from 00:00 no tz; ToTsExclusive = (to+1) 00:00 no tz), casando o
// indice (restaurant_id, created_at). tz invalido cai no default America/Sao_Paulo.
func resolveRange(p AnalyticsParams, now time.Time) (analyticsRange, error) {
	tz := strings.TrimSpace(p.TZ)
	if tz == "" {
		tz = analyticsDefaultTZ
	}
	loc, err := time.LoadLocation(tz)
	if err != nil {
		tz = analyticsDefaultTZ
		loc, err = time.LoadLocation(tz)
		if err != nil {
			loc = time.UTC
			tz = "UTC"
		}
	}

	today := now.In(loc)
	todayDate := time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, loc)

	to, err := parseAnalyticsDate(p.To, loc)
	if err != nil {
		return analyticsRange{}, ErrValidation
	}
	if to.IsZero() {
		to = todayDate
	}
	// to clampa em hoje (nao faz sentido futuro).
	if to.After(todayDate) {
		to = todayDate
	}

	from, err := parseAnalyticsDate(p.From, loc)
	if err != nil {
		return analyticsRange{}, ErrValidation
	}
	if from.IsZero() {
		from = to.AddDate(0, 0, -(analyticsDefaultRangeDays - 1))
	}
	if from.After(to) {
		return analyticsRange{}, ErrValidation
	}

	// Span maximo: numero de dias inclusivos entre from e to.
	spanDays := int(to.Sub(from).Hours()/24) + 1
	if spanDays > analyticsMaxSpanDays {
		return analyticsRange{}, ErrValidation
	}

	fromTs := from
	toTsExclusive := to.AddDate(0, 0, 1)
	return analyticsRange{
		From:          from.Format("2006-01-02"),
		To:            to.Format("2006-01-02"),
		FromTs:        fromTs.UTC(),
		ToTsExclusive: toTsExclusive.UTC(),
		TZ:            tz,
		Location:      loc,
	}, nil
}

// parseAnalyticsDate le YYYY-MM-DD no fuso loc. Vazio => zero time (o caller aplica o
// default). Formato invalido => erro.
func parseAnalyticsDate(value string, loc *time.Location) (time.Time, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}, nil
	}
	return time.ParseInLocation("2006-01-02", value, loc)
}

// clampLimit aplica o default/maximo de limit (default 20, max 100; <=0 vira default).
func clampLimit(limit int) int {
	if limit <= 0 {
		return analyticsDefaultLimit
	}
	if limit > analyticsMaxLimit {
		return analyticsMaxLimit
	}
	return limit
}

// safeRate devolve num/den protegido contra divisao por zero (den<=0 => 0).
func safeRate(num, den int64) float64 {
	if den <= 0 {
		return 0
	}
	return float64(num) / float64(den)
}

// ============================================================================
// Overview
// ============================================================================

// Overview monta os KPIs do periodo, derivando taxas/medias dos crus do store.
func (s *Service) Overview(ctx context.Context, accountID, restaurantID string, rg analyticsRange) (AnalyticsOverview, error) {
	if err := s.ensureRestaurant(ctx, accountID, restaurantID); err != nil {
		return AnalyticsOverview{}, err
	}
	raw, err := s.store.Overview(ctx, accountID, restaurantID, rg.FromTs, rg.ToTsExclusive)
	if err != nil {
		return AnalyticsOverview{}, err
	}

	avgSessionSeconds := 0.0
	if raw.UniqueSessions > 0 {
		avgSessionSeconds = float64(raw.TotalDurationMS) / float64(raw.UniqueSessions) / 1000
	}
	avgTicket := int64(0)
	if raw.Orders > 0 {
		avgTicket = raw.RevenueCents / raw.Orders
	}
	cartNoOrder := raw.SessionsWithCart - raw.SessionsCartOrder
	if cartNoOrder < 0 {
		cartNoOrder = 0
	}
	newSessions := raw.UniqueSessions - raw.ReturningSessions
	if newSessions < 0 {
		newSessions = 0
	}

	return AnalyticsOverview{
		UniqueSessions:      raw.UniqueSessions,
		UniqueDevices:       raw.UniqueDevices,
		Sessions:            raw.UniqueSessions,
		Pageviews:           raw.Pageviews,
		Events:              raw.Events,
		Orders:              raw.Orders,
		RevenueCents:        raw.RevenueCents,
		ConversionRate:      safeRate(raw.Orders, raw.UniqueSessions),
		AvgTicketCents:      avgTicket,
		AvgSessionSeconds:   avgSessionSeconds,
		NewSessions:         newSessions,
		ReturningSessions:   raw.ReturningSessions,
		CartAbandonmentRate: safeRate(cartNoOrder, raw.SessionsWithCart),
		SessionsWithCart:    raw.SessionsWithCart,
		SessionsCartNoOrder: cartNoOrder,
	}, nil
}

// ============================================================================
// Timeseries
// ============================================================================

// Timeseries monta a serie densa conforme a granularidade (day | hour_of_day |
// weekday_hour). Dias/horas/celulas sem dado vem com 0. granularity ja validada.
func (s *Service) Timeseries(ctx context.Context, accountID, restaurantID, granularity string, rg analyticsRange) (AnalyticsTimeseries, error) {
	if err := s.ensureRestaurant(ctx, accountID, restaurantID); err != nil {
		return AnalyticsTimeseries{}, err
	}
	switch granularity {
	case "hour_of_day":
		return s.timeseriesHourOfDay(ctx, accountID, restaurantID, rg)
	case "weekday_hour":
		return s.timeseriesWeekdayHour(ctx, accountID, restaurantID, rg)
	default:
		return s.timeseriesDaily(ctx, accountID, restaurantID, rg)
	}
}

func (s *Service) timeseriesDaily(ctx context.Context, accountID, restaurantID string, rg analyticsRange) (AnalyticsTimeseries, error) {
	byDay, err := s.store.TimeseriesDaily(ctx, accountID, restaurantID, rg.FromTs, rg.ToTsExclusive, rg.TZ)
	if err != nil {
		return AnalyticsTimeseries{}, err
	}
	out := AnalyticsTimeseries{Granularity: "day", Points: []AnalyticsTimePoint{}}
	start, _ := time.ParseInLocation("2006-01-02", rg.From, rg.Location)
	end, _ := time.ParseInLocation("2006-01-02", rg.To, rg.Location)
	for d := start; !d.After(end); d = d.AddDate(0, 0, 1) {
		key := d.Format("2006-01-02")
		pt := byDay[key]
		pt.Bucket = key
		out.Points = append(out.Points, pt)
	}
	return out, nil
}

func (s *Service) timeseriesHourOfDay(ctx context.Context, accountID, restaurantID string, rg analyticsRange) (AnalyticsTimeseries, error) {
	byHour, err := s.store.TimeseriesHourOfDay(ctx, accountID, restaurantID, rg.FromTs, rg.ToTsExclusive, rg.TZ)
	if err != nil {
		return AnalyticsTimeseries{}, err
	}
	out := AnalyticsTimeseries{Granularity: "hour_of_day", Points: make([]AnalyticsTimePoint, 0, 24)}
	for h := 0; h < 24; h++ {
		pt := byHour[h]
		pt.Hour = h
		out.Points = append(out.Points, pt)
	}
	return out, nil
}

func (s *Service) timeseriesWeekdayHour(ctx context.Context, accountID, restaurantID string, rg analyticsRange) (AnalyticsTimeseries, error) {
	byCell, err := s.store.TimeseriesWeekdayHour(ctx, accountID, restaurantID, rg.FromTs, rg.ToTsExclusive, rg.TZ)
	if err != nil {
		return AnalyticsTimeseries{}, err
	}
	out := AnalyticsTimeseries{Granularity: "weekday_hour", Points: make([]AnalyticsTimePoint, 0, 168)}
	for wd := 0; wd < 7; wd++ {
		for h := 0; h < 24; h++ {
			pt := byCell[wd*24+h]
			pt.Weekday = wd
			pt.Hour = h
			out.Points = append(out.Points, pt)
		}
	}
	return out, nil
}

// ============================================================================
// Funnel
// ============================================================================

// Funnel monta o funil por sessao, garantindo a monotonicidade (cada etapa nao pode
// ter mais sessoes que a anterior: usa o minimo corrente) e calculando rateFromStart
// (sobre a 1a etapa) e rateFromPrev (sobre a anterior).
func (s *Service) Funnel(ctx context.Context, accountID, restaurantID string, rg analyticsRange) (AnalyticsFunnel, error) {
	if err := s.ensureRestaurant(ctx, accountID, restaurantID); err != nil {
		return AnalyticsFunnel{}, err
	}
	steps, err := s.store.FunnelSteps(ctx, accountID, restaurantID, rg.FromTs, rg.ToTsExclusive)
	if err != nil {
		return AnalyticsFunnel{}, err
	}
	var start, prev int64
	for i := range steps {
		// Monotonicidade: nenhuma etapa supera a anterior (sessoes "que chegaram ate
		// aqui"). O store conta sessoes que dispararam o evento da etapa; o min
		// corrente garante um funil decrescente.
		if i == 0 {
			start = steps[i].Sessions
			prev = steps[i].Sessions
		} else if steps[i].Sessions > prev {
			steps[i].Sessions = prev
		}
		steps[i].RateFromStart = safeRate(steps[i].Sessions, start)
		steps[i].RateFromPrev = safeRate(steps[i].Sessions, prev)
		prev = steps[i].Sessions
	}
	return AnalyticsFunnel{Steps: steps}, nil
}

// ============================================================================
// Top products
// ============================================================================

// TopProducts ranqueia produtos pela metrica (viewed|clicked|add_to_cart|orders),
// resolve o nome via catalogo (cai no slug se o produto nao existe mais) e calcula a
// conversao visto->comprado. metric ja validada.
func (s *Service) TopProducts(ctx context.Context, accountID, restaurantID, metric string, rg analyticsRange, limit int) (AnalyticsTopProducts, error) {
	if err := s.ensureRestaurant(ctx, accountID, restaurantID); err != nil {
		return AnalyticsTopProducts{}, err
	}
	limit = clampLimit(limit)

	var raw []topProductRaw
	var err error
	if metric == "orders" {
		raw, err = s.store.TopProductsByOrders(ctx, accountID, restaurantID, rg.FromTs, rg.ToTsExclusive, limit)
	} else {
		raw, err = s.store.TopProductsByEvent(ctx, accountID, restaurantID, topProductEventName(metric), rg.FromTs, rg.ToTsExclusive, limit)
	}
	if err != nil {
		return AnalyticsTopProducts{}, err
	}

	slugs := make([]string, 0, len(raw))
	for i := range raw {
		slugs = append(slugs, raw[i].ProductSlug)
	}
	names, err := s.store.ProductNamesBySlug(ctx, accountID, restaurantID, slugs)
	if err != nil {
		return AnalyticsTopProducts{}, err
	}

	items := make([]AnalyticsTopProduct, 0, len(raw))
	for i := range raw {
		name := names[raw[i].ProductSlug]
		if name == "" {
			name = raw[i].ProductSlug
		}
		items = append(items, AnalyticsTopProduct{
			ProductSlug:    raw[i].ProductSlug,
			Name:           name,
			Count:          raw[i].Count,
			Viewed:         raw[i].Viewed,
			Orders:         raw[i].Orders,
			ConversionRate: safeRate(raw[i].Orders, raw[i].Viewed),
		})
	}
	return AnalyticsTopProducts{Metric: metric, Items: items}, nil
}

// ============================================================================
// Sources / devices / pages
// ============================================================================

// Sources agrega sessoes/pedidos por origem. dimension ja validada; value vazio vira
// "(direto)".
func (s *Service) Sources(ctx context.Context, accountID, restaurantID, dimension string, rg analyticsRange, limit int) (AnalyticsSources, error) {
	if err := s.ensureRestaurant(ctx, accountID, restaurantID); err != nil {
		return AnalyticsSources{}, err
	}
	column := sourceColumn(dimension)
	items, err := s.store.Sources(ctx, accountID, restaurantID, column, rg.FromTs, rg.ToTsExclusive, clampLimit(limit))
	if err != nil {
		return AnalyticsSources{}, err
	}
	for i := range items {
		if strings.TrimSpace(items[i].Value) == "" {
			items[i].Value = "(direto)"
		}
	}
	return AnalyticsSources{Dimension: dimension, Items: items}, nil
}

// sourceColumn mapeia a dimensao validada para a coluna real de cardapio.sessions.
func sourceColumn(dimension string) string {
	switch dimension {
	case "utm_medium":
		return "utm_medium"
	case "utm_campaign":
		return "utm_campaign"
	case "referrer":
		return "referrer_host"
	default:
		return "utm_source"
	}
}

// Devices monta os tres breakdowns (device_type/browser/os) lendo cardapio.sessions.
func (s *Service) Devices(ctx context.Context, accountID, restaurantID string, rg analyticsRange) (AnalyticsDevices, error) {
	if err := s.ensureRestaurant(ctx, accountID, restaurantID); err != nil {
		return AnalyticsDevices{}, err
	}
	deviceType, err := s.store.Devices(ctx, accountID, restaurantID, "device_type", rg.FromTs, rg.ToTsExclusive)
	if err != nil {
		return AnalyticsDevices{}, err
	}
	browser, err := s.store.Devices(ctx, accountID, restaurantID, "browser", rg.FromTs, rg.ToTsExclusive)
	if err != nil {
		return AnalyticsDevices{}, err
	}
	os, err := s.store.Devices(ctx, accountID, restaurantID, "os", rg.FromTs, rg.ToTsExclusive)
	if err != nil {
		return AnalyticsDevices{}, err
	}
	return AnalyticsDevices{DeviceType: deviceType, Browser: browser, OS: os}, nil
}

// Pages lista as paginas mais vistas + dwell medio.
func (s *Service) Pages(ctx context.Context, accountID, restaurantID string, rg analyticsRange, limit int) (AnalyticsPages, error) {
	if err := s.ensureRestaurant(ctx, accountID, restaurantID); err != nil {
		return AnalyticsPages{}, err
	}
	items, err := s.store.Pages(ctx, accountID, restaurantID, rg.FromTs, rg.ToTsExclusive, clampLimit(limit))
	if err != nil {
		return AnalyticsPages{}, err
	}
	return AnalyticsPages{Items: items}, nil
}

// ============================================================================
// Dwell / clicks
// ============================================================================

// Dwell agrega o tempo medio por dimensao (page|product|section). dimension ja
// validada; o store traduz para (evento, expressao de chave) da allowlist fechada.
func (s *Service) Dwell(ctx context.Context, accountID, restaurantID, dimension string, rg analyticsRange, limit int) (AnalyticsDwell, error) {
	if err := s.ensureRestaurant(ctx, accountID, restaurantID); err != nil {
		return AnalyticsDwell{}, err
	}
	eventName, keyExpr := dwellConfig(dimension)
	items, err := s.store.Dwell(ctx, accountID, restaurantID, eventName, keyExpr, rg.FromTs, rg.ToTsExclusive, clampLimit(limit))
	if err != nil {
		return AnalyticsDwell{}, err
	}
	return AnalyticsDwell{Dimension: dimension, Items: items}, nil
}

// Clicks agrega os cliques de botoes (cta/outbound/whatsapp/coupon/reservation).
func (s *Service) Clicks(ctx context.Context, accountID, restaurantID string, rg analyticsRange, limit int) (AnalyticsClicks, error) {
	if err := s.ensureRestaurant(ctx, accountID, restaurantID); err != nil {
		return AnalyticsClicks{}, err
	}
	items, err := s.store.Clicks(ctx, accountID, restaurantID, rg.FromTs, rg.ToTsExclusive, clampLimit(limit))
	if err != nil {
		return AnalyticsClicks{}, err
	}
	// Ordem estavel quando ha empate de contagem (defensivo; o SQL ja ordena).
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].Count != items[j].Count {
			return items[i].Count > items[j].Count
		}
		if items[i].Name != items[j].Name {
			return items[i].Name < items[j].Name
		}
		return items[i].Label < items[j].Label
	})
	return AnalyticsClicks{Items: items}, nil
}
