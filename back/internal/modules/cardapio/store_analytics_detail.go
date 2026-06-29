package cardapio

import (
	"context"
	"time"
)

// Queries de detalhe do analytics (funil, top-produtos, dwell, cliques). Mesmas
// regras transversais de store_analytics.go (escopo restaurant_id+account_id, janela
// half-open por created_at, bots fora). SQL parametrizado e schema-qualificado.

// funnelEventSteps sao as etapas baseadas em evento (por sessao, via bool_or). A
// ultima etapa (pedido) vem de cardapio.orders e e calculada a parte.
var funnelEventSteps = []struct {
	Key   string
	Event string
	Label string
}{
	{"restaurant_viewed", "restaurant_viewed", "Acessou o restaurante"},
	{"menu_viewed", "menu_viewed", "Viu o cardapio"},
	{"product_viewed", "product_viewed", "Viu um produto"},
	{"add_to_cart", "add_to_cart", "Adicionou ao carrinho"},
	{"checkout_started", "checkout_started", "Iniciou o checkout"},
}

// FunnelSteps retorna, para cada etapa de evento, quantas SESSOES distintas
// dispararam o evento (sessao com session_id vazio excluida), mais a etapa final de
// pedido (sessoes distintas com pedido nao-cancelado no periodo). A ordem do retorno
// segue funnelEventSteps + "pedido"; o service garante a monotonicidade (min corrente)
// e as taxas. Bots fora.
func (s *Store) FunnelSteps(ctx context.Context, accountID, restaurantID string, from, to time.Time) ([]AnalyticsFunnelStep, error) {
	// Contagem de sessoes distintas por evento das etapas. Um unico passe com
	// agregacao condicional sobre eventos da allowlist do funil.
	const evtQ = `
		select
			count(distinct session_id) filter (where name = 'restaurant_viewed') as s_restaurant_viewed,
			count(distinct session_id) filter (where name = 'menu_viewed')       as s_menu_viewed,
			count(distinct session_id) filter (where name = 'product_viewed')    as s_product_viewed,
			count(distinct session_id) filter (where name = 'add_to_cart')       as s_add_to_cart,
			count(distinct session_id) filter (where name = 'checkout_started')  as s_checkout_started
		from cardapio.events
		where restaurant_id = $1 and account_id = $2
		  and device_type <> 'bot'
		  and session_id <> ''
		  and created_at >= $3 and created_at < $4`
	counts := make(map[string]int64, len(funnelEventSteps))
	var rv, mv, pv, atc, cs int64
	if err := s.pool.QueryRow(ctx, evtQ, restaurantID, accountID, from, to).Scan(&rv, &mv, &pv, &atc, &cs); err != nil {
		return nil, err
	}
	counts["restaurant_viewed"] = rv
	counts["menu_viewed"] = mv
	counts["product_viewed"] = pv
	counts["add_to_cart"] = atc
	counts["checkout_started"] = cs

	// Etapa final: sessoes distintas com pedido nao-cancelado no periodo.
	const ordQ = `
		select count(distinct session_id)
		from cardapio.orders
		where restaurant_id = $1 and account_id = $2
		  and status <> 'cancelado'
		  and session_id <> ''
		  and created_at >= $3 and created_at < $4`
	var ordered int64
	if err := s.pool.QueryRow(ctx, ordQ, restaurantID, accountID, from, to).Scan(&ordered); err != nil {
		return nil, err
	}

	steps := make([]AnalyticsFunnelStep, 0, len(funnelEventSteps)+1)
	for _, st := range funnelEventSteps {
		steps = append(steps, AnalyticsFunnelStep{Key: st.Key, Label: st.Label, Sessions: counts[st.Key]})
	}
	steps = append(steps, AnalyticsFunnelStep{Key: "order_created", Label: "Fez o pedido", Sessions: ordered})
	return steps, nil
}

// topProductEventName mapeia a metrica de top-products para o nome do evento contado.
func topProductEventName(metric string) string {
	switch metric {
	case "viewed":
		return "product_viewed"
	case "clicked":
		return "product_clicked"
	case "add_to_cart":
		return "add_to_cart"
	default:
		return ""
	}
}

// TopProductsByEvent ranqueia produtos por uma metrica de evento (viewed/clicked/
// add_to_cart). Sempre devolve tambem `viewed` (product_viewed) e `orders` (via
// order_items) para a conversao visto->comprado no service. eventName ja vem mapeado
// e validado pelo service. product_slug vazio excluido.
func (s *Store) TopProductsByEvent(ctx context.Context, accountID, restaurantID, eventName string, from, to time.Time, limit int) ([]topProductRaw, error) {
	const q = `
		with metric as (
			select product_slug, count(*) as cnt
			from cardapio.events
			where restaurant_id = $1 and account_id = $2
			  and device_type <> 'bot' and name = $5
			  and product_slug <> ''
			  and created_at >= $3 and created_at < $4
			group by product_slug
		),
		viewed as (
			select product_slug, count(*) as viewed
			from cardapio.events
			where restaurant_id = $1 and account_id = $2
			  and device_type <> 'bot' and name = 'product_viewed'
			  and product_slug <> ''
			  and created_at >= $3 and created_at < $4
			group by product_slug
		),
		ordered as (
			select p.slug as product_slug, count(*) as orders
			from cardapio.order_items oi
			join cardapio.orders o on o.id = oi.order_id
			join cardapio.products p on p.id = oi.product_id
			where o.restaurant_id = $1 and o.account_id = $2
			  and o.status <> 'cancelado'
			  and o.created_at >= $3 and o.created_at < $4
			  and p.restaurant_id = $1
			group by p.slug
		)
		select m.product_slug, m.cnt, coalesce(v.viewed, 0), coalesce(od.orders, 0)
		from metric m
		left join viewed v using (product_slug)
		left join ordered od using (product_slug)
		order by m.cnt desc, m.product_slug asc
		limit $6`
	rows, err := s.pool.Query(ctx, q, restaurantID, accountID, from, to, eventName, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanTopProducts(rows)
}

// TopProductsByOrders ranqueia produtos pela metrica `orders` (pedidos via
// order_items, status<>cancelado). Tambem devolve `viewed` para a conversao.
func (s *Store) TopProductsByOrders(ctx context.Context, accountID, restaurantID string, from, to time.Time, limit int) ([]topProductRaw, error) {
	const q = `
		with ordered as (
			select p.slug as product_slug, count(*) as orders
			from cardapio.order_items oi
			join cardapio.orders o on o.id = oi.order_id
			join cardapio.products p on p.id = oi.product_id
			where o.restaurant_id = $1 and o.account_id = $2
			  and o.status <> 'cancelado'
			  and o.created_at >= $3 and o.created_at < $4
			  and p.restaurant_id = $1
			group by p.slug
		),
		viewed as (
			select product_slug, count(*) as viewed
			from cardapio.events
			where restaurant_id = $1 and account_id = $2
			  and device_type <> 'bot' and name = 'product_viewed'
			  and product_slug <> ''
			  and created_at >= $3 and created_at < $4
			group by product_slug
		)
		select od.product_slug, od.orders as cnt, coalesce(v.viewed, 0), od.orders
		from ordered od
		left join viewed v using (product_slug)
		order by od.orders desc, od.product_slug asc
		limit $5`
	rows, err := s.pool.Query(ctx, q, restaurantID, accountID, from, to, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanTopProducts(rows)
}

func scanTopProducts(rows interface {
	Next() bool
	Scan(...any) error
	Err() error
}) ([]topProductRaw, error) {
	out := make([]topProductRaw, 0)
	for rows.Next() {
		var r topProductRaw
		if err := rows.Scan(&r.ProductSlug, &r.Count, &r.Viewed, &r.Orders); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// dwellConfig mapeia a dimensao de dwell para (nome do evento, expressao da chave).
// A chave de section sai do context (sectionId); page/product usam as colunas
// desnormalizadas.
func dwellConfig(dimension string) (eventName, keyExpr string) {
	switch dimension {
	case "product":
		return "product_dwell", "product_slug"
	case "section":
		return "section_dwell", "coalesce(context->>'sectionId', '')"
	default:
		return "page_dwell", "page_path"
	}
}

// Dwell agrega o tempo medio (avg dwell_ms) + amostras por chave (page_path/
// product_slug/sectionId). Considera so amostras com dwell_ms > 0 e descarta
// heartbeats parciais (context->>'final' = 'false'); sem flag, dwell_ms > 0 basta.
// eventName e keyExpr ja vem da allowlist fechada do service.
func (s *Store) Dwell(ctx context.Context, accountID, restaurantID, eventName, keyExpr string, from, to time.Time, limit int) ([]AnalyticsDwellItem, error) {
	q := `
		select ` + keyExpr + ` as key, avg(dwell_ms)::float8 as avg_dwell_ms, count(*) as samples
		from cardapio.events
		where restaurant_id = $1 and account_id = $2
		  and device_type <> 'bot' and name = $5
		  and dwell_ms > 0
		  and coalesce(context->>'final', '') <> 'false'
		  and created_at >= $3 and created_at < $4
		group by ` + keyExpr + `
		having ` + keyExpr + ` <> ''
		order by avg_dwell_ms desc, key asc
		limit $6`
	rows, err := s.pool.Query(ctx, q, restaurantID, accountID, from, to, eventName, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]AnalyticsDwellItem, 0)
	for rows.Next() {
		var it AnalyticsDwellItem
		var avgMS float64
		if err := rows.Scan(&it.Key, &avgMS, &it.Samples); err != nil {
			return nil, err
		}
		it.AvgDwellSeconds = avgMS / 1000
		out = append(out, it)
	}
	return out, rows.Err()
}

// clickEventNames sao os eventos de clique agregados ("quais botoes").
var clickEventNames = []string{
	"cta_clicked", "outbound_click", "whatsapp_order_clicked", "coupon_used", "reservation_sent",
}

// Clicks agrega cliques por (nome, label, kind), extraindo label/kind do context
// (ctaLabel/ctaKind/kind). Bots fora; nomes restritos a clickEventNames (passados como
// array parametrizado).
func (s *Store) Clicks(ctx context.Context, accountID, restaurantID string, from, to time.Time, limit int) ([]AnalyticsClick, error) {
	const q = `
		select name,
		       coalesce(context->>'ctaLabel', '') as label,
		       coalesce(context->>'ctaKind', context->>'kind', '') as kind,
		       count(*) as cnt
		from cardapio.events
		where restaurant_id = $1 and account_id = $2
		  and device_type <> 'bot'
		  and name = any($5)
		  and created_at >= $3 and created_at < $4
		group by name, label, kind
		order by cnt desc, name asc, label asc
		limit $6`
	rows, err := s.pool.Query(ctx, q, restaurantID, accountID, from, to, clickEventNames, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]AnalyticsClick, 0)
	for rows.Next() {
		var c AnalyticsClick
		if err := rows.Scan(&c.Name, &c.Label, &c.Kind, &c.Count); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// ProductNamesBySlug resolve os nomes dos produtos por slug (catalogo atual). Para
// slugs sem produto vivo, o service cai no proprio slug. Escopado por account +
// restaurant.
func (s *Store) ProductNamesBySlug(ctx context.Context, accountID, restaurantID string, slugs []string) (map[string]string, error) {
	if len(slugs) == 0 {
		return map[string]string{}, nil
	}
	const q = `
		select slug, name
		from cardapio.products
		where restaurant_id = $1 and account_id = $2 and slug = any($3)`
	rows, err := s.pool.Query(ctx, q, restaurantID, accountID, slugs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make(map[string]string, len(slugs))
	for rows.Next() {
		var slug, name string
		if err := rows.Scan(&slug, &name); err != nil {
			return nil, err
		}
		out[slug] = name
	}
	return out, rows.Err()
}
