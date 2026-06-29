package cardapio

import (
	"context"
	"testing"
	"time"
)

// analyticsFake e um dataStore focado nos testes de analytics. Embute
// unimplementedStore (defaults inertes) e sobrescreve so o que cada teste exercita.
// GetRestaurant honra restaurantErr para simular o 404 de pertencimento (escopo).
type analyticsFake struct {
	unimplementedStore
	restaurantErr error
	overview      overviewRaw
	daily         map[string]AnalyticsTimePoint
	funnel        []AnalyticsFunnelStep
}

func (f *analyticsFake) GetRestaurant(_ context.Context, _, _ string) (Restaurant, error) {
	if f.restaurantErr != nil {
		return Restaurant{}, f.restaurantErr
	}
	return Restaurant{ID: "rest-1", IsActive: true}, nil
}

func (f *analyticsFake) Overview(_ context.Context, _, _ string, _, _ time.Time) (overviewRaw, error) {
	return f.overview, nil
}

func (f *analyticsFake) TimeseriesDaily(_ context.Context, _, _ string, _, _ time.Time, _ string) (map[string]AnalyticsTimePoint, error) {
	return f.daily, nil
}

func (f *analyticsFake) FunnelSteps(_ context.Context, _, _ string, _, _ time.Time) ([]AnalyticsFunnelStep, error) {
	return f.funnel, nil
}

// fixedNow ancora "agora" para os testes de range serem determinísticos.
func fixedNow() time.Time {
	return time.Date(2026, 6, 25, 15, 0, 0, 0, time.UTC)
}

// ============================================================================
// resolveRange — defaults, clamp e span maximo
// ============================================================================

func TestResolveRange_DefaultLast30Days(t *testing.T) {
	rg, err := resolveRange(AnalyticsParams{}, fixedNow())
	if err != nil {
		t.Fatalf("esperava sucesso, recebi %v", err)
	}
	// to = hoje (2026-06-25); from = hoje - 29 dias (janela de 30 dias inclusiva).
	if rg.To != "2026-06-25" {
		t.Fatalf("to default: esperava 2026-06-25, recebi %s", rg.To)
	}
	if rg.From != "2026-05-27" {
		t.Fatalf("from default: esperava 2026-05-27, recebi %s", rg.From)
	}
	// Half-open: toTsExclusive = (to+1) 00:00 no tz.
	if rg.ToTsExclusive.Before(rg.FromTs) {
		t.Fatalf("intervalo half-open invertido: %v..%v", rg.FromTs, rg.ToTsExclusive)
	}
}

func TestResolveRange_ClampToToday(t *testing.T) {
	rg, err := resolveRange(AnalyticsParams{From: "2026-06-20", To: "2030-01-01"}, fixedNow())
	if err != nil {
		t.Fatalf("esperava sucesso, recebi %v", err)
	}
	if rg.To != "2026-06-25" {
		t.Fatalf("to deveria clampar em hoje (2026-06-25), recebi %s", rg.To)
	}
}

func TestResolveRange_SpanTooLarge(t *testing.T) {
	// 91 dias inclusivos > 90 => ErrValidation.
	_, err := resolveRange(AnalyticsParams{From: "2026-03-01", To: "2026-05-30"}, fixedNow())
	if err != ErrValidation {
		t.Fatalf("span > 90 dias: esperava ErrValidation, recebi %v", err)
	}
}

func TestResolveRange_FromAfterTo(t *testing.T) {
	_, err := resolveRange(AnalyticsParams{From: "2026-06-20", To: "2026-06-10"}, fixedNow())
	if err != ErrValidation {
		t.Fatalf("from > to: esperava ErrValidation, recebi %v", err)
	}
}

func TestResolveRange_InvalidDate(t *testing.T) {
	_, err := resolveRange(AnalyticsParams{From: "20-06-2026"}, fixedNow())
	if err != ErrValidation {
		t.Fatalf("data invalida: esperava ErrValidation, recebi %v", err)
	}
}

func TestResolveRange_InvalidTZFallsBack(t *testing.T) {
	rg, err := resolveRange(AnalyticsParams{TZ: "Marte/Olympus"}, fixedNow())
	if err != nil {
		t.Fatalf("tz invalido nao deveria falhar (cai no default), recebi %v", err)
	}
	if rg.TZ != analyticsDefaultTZ {
		t.Fatalf("tz invalido deveria cair em %s, recebi %s", analyticsDefaultTZ, rg.TZ)
	}
}

// ============================================================================
// Escopo — 404 com account divergente
// ============================================================================

func TestAnalyticsScope_NotFoundOnForeignAccount(t *testing.T) {
	store := &analyticsFake{restaurantErr: ErrNotFound}
	svc := newServiceWithStore(store, ServiceConfig{})
	rg, _ := resolveRange(AnalyticsParams{}, fixedNow())

	if _, err := svc.Overview(context.Background(), "acc-outra", "rest-1", rg); err != ErrNotFound {
		t.Fatalf("restaurante fora do escopo: esperava ErrNotFound (404), recebi %v", err)
	}
}

// ============================================================================
// Overview — conversao sem divisao por zero + derivados
// ============================================================================

func TestOverview_ConversionRateNoDivByZero(t *testing.T) {
	store := &analyticsFake{overview: overviewRaw{
		UniqueSessions: 0, Orders: 0, RevenueCents: 0, SessionsWithCart: 0,
	}}
	svc := newServiceWithStore(store, ServiceConfig{})
	rg, _ := resolveRange(AnalyticsParams{}, fixedNow())

	out, err := svc.Overview(context.Background(), "acc-1", "rest-1", rg)
	if err != nil {
		t.Fatalf("esperava sucesso, recebi %v", err)
	}
	if out.ConversionRate != 0 || out.AvgTicketCents != 0 || out.AvgSessionSeconds != 0 || out.CartAbandonmentRate != 0 {
		t.Fatalf("sem dados deveria zerar tudo, recebi %+v", out)
	}
}

func TestOverview_DerivedValues(t *testing.T) {
	store := &analyticsFake{overview: overviewRaw{
		UniqueSessions:    100,
		UniqueDevices:     80,
		Pageviews:         300,
		Events:            900,
		ReturningSessions: 30,
		TotalDurationMS:   100 * 60 * 1000, // 100 sessoes * 60s
		Orders:            10,
		RevenueCents:      50000,
		SessionsWithCart:  40,
		SessionsCartOrder: 10,
	}}
	svc := newServiceWithStore(store, ServiceConfig{})
	rg, _ := resolveRange(AnalyticsParams{}, fixedNow())

	out, err := svc.Overview(context.Background(), "acc-1", "rest-1", rg)
	if err != nil {
		t.Fatalf("esperava sucesso, recebi %v", err)
	}
	if out.ConversionRate != 0.1 {
		t.Fatalf("conversao: esperava 0.1 (10/100), recebi %v", out.ConversionRate)
	}
	if out.AvgTicketCents != 5000 {
		t.Fatalf("ticket medio: esperava 5000 (50000/10), recebi %d", out.AvgTicketCents)
	}
	if out.AvgSessionSeconds != 60 {
		t.Fatalf("duracao media: esperava 60s, recebi %v", out.AvgSessionSeconds)
	}
	if out.NewSessions != 70 {
		t.Fatalf("novos: esperava 70 (100-30), recebi %d", out.NewSessions)
	}
	// abandono = (40 com carrinho - 10 com pedido) / 40 = 0.75.
	if out.CartAbandonmentRate != 0.75 {
		t.Fatalf("abandono: esperava 0.75, recebi %v", out.CartAbandonmentRate)
	}
	if out.SessionsCartNoOrder != 30 {
		t.Fatalf("carrinho sem pedido: esperava 30, recebi %d", out.SessionsCartNoOrder)
	}
}

// ============================================================================
// Funnel — monotonicidade + taxas
// ============================================================================

func TestFunnel_MonotonicAndRates(t *testing.T) {
	// Etapas com um "salto" (product_viewed 90 > menu_viewed 80) que o service deve
	// aparar para o minimo corrente (monotonico).
	store := &analyticsFake{funnel: []AnalyticsFunnelStep{
		{Key: "restaurant_viewed", Sessions: 100},
		{Key: "menu_viewed", Sessions: 80},
		{Key: "product_viewed", Sessions: 90},
		{Key: "add_to_cart", Sessions: 40},
		{Key: "checkout_started", Sessions: 20},
		{Key: "order_created", Sessions: 10},
	}}
	svc := newServiceWithStore(store, ServiceConfig{})
	rg, _ := resolveRange(AnalyticsParams{}, fixedNow())

	out, err := svc.Funnel(context.Background(), "acc-1", "rest-1", rg)
	if err != nil {
		t.Fatalf("esperava sucesso, recebi %v", err)
	}
	prev := int64(1 << 62)
	for _, st := range out.Steps {
		if st.Sessions > prev {
			t.Fatalf("funil nao-monotonico em %s: %d > %d", st.Key, st.Sessions, prev)
		}
		prev = st.Sessions
	}
	// product_viewed aparado para 80 (min com menu_viewed).
	if out.Steps[2].Sessions != 80 {
		t.Fatalf("etapa product_viewed deveria ser aparada para 80, recebi %d", out.Steps[2].Sessions)
	}
	// rateFromStart da ultima = 10/100 = 0.1.
	last := out.Steps[len(out.Steps)-1]
	if last.RateFromStart != 0.1 {
		t.Fatalf("rateFromStart final: esperava 0.1, recebi %v", last.RateFromStart)
	}
	// rateFromPrev de order_created = 10/20 = 0.5.
	if last.RateFromPrev != 0.5 {
		t.Fatalf("rateFromPrev final: esperava 0.5, recebi %v", last.RateFromPrev)
	}
}

// ============================================================================
// Timeseries — serie densa preenche 0
// ============================================================================

func TestTimeseriesDaily_DenseFillsZeros(t *testing.T) {
	// Range curto (3 dias) com dado so no dia do meio; os outros dois vem com 0.
	store := &analyticsFake{daily: map[string]AnalyticsTimePoint{
		"2026-06-21": {Bucket: "2026-06-21", Visits: 5, Sessions: 7, Pageviews: 20, Orders: 2},
	}}
	svc := newServiceWithStore(store, ServiceConfig{})
	rg, err := resolveRange(AnalyticsParams{From: "2026-06-20", To: "2026-06-22"}, fixedNow())
	if err != nil {
		t.Fatalf("range: %v", err)
	}

	out, err := svc.Timeseries(context.Background(), "acc-1", "rest-1", "day", rg)
	if err != nil {
		t.Fatalf("esperava sucesso, recebi %v", err)
	}
	if len(out.Points) != 3 {
		t.Fatalf("serie densa: esperava 3 pontos (20,21,22), recebi %d", len(out.Points))
	}
	if out.Points[0].Bucket != "2026-06-20" || out.Points[0].Visits != 0 {
		t.Fatalf("dia 20 deveria existir com 0, recebi %+v", out.Points[0])
	}
	if out.Points[1].Bucket != "2026-06-21" || out.Points[1].Sessions != 7 {
		t.Fatalf("dia 21 deveria ter o dado, recebi %+v", out.Points[1])
	}
	if out.Points[2].Bucket != "2026-06-22" || out.Points[2].Orders != 0 {
		t.Fatalf("dia 22 deveria existir com 0, recebi %+v", out.Points[2])
	}
}

func TestTimeseriesHourOfDay_Fills24(t *testing.T) {
	svc := newServiceWithStore(&analyticsFake{}, ServiceConfig{})
	rg, _ := resolveRange(AnalyticsParams{}, fixedNow())
	out, err := svc.Timeseries(context.Background(), "acc-1", "rest-1", "hour_of_day", rg)
	if err != nil {
		t.Fatalf("esperava sucesso, recebi %v", err)
	}
	if len(out.Points) != 24 {
		t.Fatalf("hour_of_day deveria ter 24 pontos, recebi %d", len(out.Points))
	}
	for h, pt := range out.Points {
		if pt.Hour != h {
			t.Fatalf("hora fora de ordem: indice %d com hour %d", h, pt.Hour)
		}
	}
}
