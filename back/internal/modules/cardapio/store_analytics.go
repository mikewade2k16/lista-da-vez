package cardapio

import (
	"context"
	"time"
)

// Queries de leitura/agregacao do analytics do painel (Fase 10 / F2). Regras
// transversais a TODA query:
//   - filtro `restaurant_id = $1 AND account_id = $2` (defesa em profundidade; o
//     pertencimento ja foi validado no service);
//   - janela half-open `created_at >= $3 AND created_at < $4` (casa o indice
//     (restaurant_id, created_at); usa o relogio do servidor);
//   - bots excluidos por padrao (`device_type <> 'bot'`) onde a tabela classifica UA.
// SQL 100% parametrizado e schema-qualificado. O service nao monta SQL; aqui nao ha
// regra de negocio (so leitura tipada).

// Overview agrega os KPIs do periodo lendo cardapio.sessions (sessoes/duracao/device/
// novos-recorrentes), cardapio.orders (pedidos/receita) e cardapio.events (pageviews
// e abandono de sacola via add_to_cart). Retorna os crus; o service deriva taxas.
func (s *Store) Overview(ctx context.Context, accountID, restaurantID string, from, to time.Time) (overviewRaw, error) {
	var out overviewRaw

	// Sessoes, devices unicos, duracao total e novos vs recorrentes. Uma sessao e
	// "recorrente" quando o device_id ja teve uma sessao iniciada ANTES da janela.
	const sessQ = `
		with win as (
			select session_id, device_id, duration_ms, first_seen_at
			from cardapio.sessions
			where restaurant_id = $1 and account_id = $2
			  and device_type <> 'bot'
			  and first_seen_at >= $3 and first_seen_at < $4
		)
		select
			count(*) as sessions,
			count(distinct nullif(device_id, '')) as devices,
			coalesce(sum(duration_ms), 0) as total_duration_ms,
			coalesce(sum(case when w.device_id <> '' and exists (
				select 1 from cardapio.sessions p
				where p.restaurant_id = $1 and p.account_id = $2
				  and p.device_id = w.device_id
				  and p.first_seen_at < $3
			) then 1 else 0 end), 0) as returning_sessions
		from win w`
	if err := s.pool.QueryRow(ctx, sessQ, restaurantID, accountID, from, to).Scan(
		&out.UniqueSessions, &out.UniqueDevices, &out.TotalDurationMS, &out.ReturningSessions,
	); err != nil {
		return overviewRaw{}, err
	}

	// Pageviews (eventos page_view) + total de eventos no periodo (sem bots).
	const evtQ = `
		select
			coalesce(sum(case when name = 'page_view' then 1 else 0 end), 0) as pageviews,
			count(*) as events
		from cardapio.events
		where restaurant_id = $1 and account_id = $2
		  and device_type <> 'bot'
		  and created_at >= $3 and created_at < $4`
	if err := s.pool.QueryRow(ctx, evtQ, restaurantID, accountID, from, to).Scan(
		&out.Pageviews, &out.Events,
	); err != nil {
		return overviewRaw{}, err
	}

	// Pedidos + receita no periodo (fonte de verdade = cardapio.orders; cancelados
	// fora). Orders nao classifica bot — a conversao ancora no pedido real.
	const ordQ = `
		select count(*) as orders, coalesce(sum(total_cents), 0) as revenue
		from cardapio.orders
		where restaurant_id = $1 and account_id = $2
		  and status <> 'cancelado'
		  and created_at >= $3 and created_at < $4`
	if err := s.pool.QueryRow(ctx, ordQ, restaurantID, accountID, from, to).Scan(
		&out.Orders, &out.RevenueCents,
	); err != nil {
		return overviewRaw{}, err
	}

	// Abandono de sacola: sessoes com add_to_cart no periodo; destas, quantas NAO
	// tem pedido (join evento<->pedido por session_id). session_id vazio fora.
	const cartQ = `
		with cart_sessions as (
			select distinct e.session_id
			from cardapio.events e
			where e.restaurant_id = $1 and e.account_id = $2
			  and e.device_type <> 'bot'
			  and e.name = 'add_to_cart'
			  and e.session_id <> ''
			  and e.created_at >= $3 and e.created_at < $4
		)
		select
			count(*) as with_cart,
			coalesce(sum(case when exists (
				select 1 from cardapio.orders o
				where o.restaurant_id = $1 and o.account_id = $2
				  and o.session_id = cs.session_id
				  and o.status <> 'cancelado'
			) then 1 else 0 end), 0) as cart_with_order
		from cart_sessions cs`
	if err := s.pool.QueryRow(ctx, cartQ, restaurantID, accountID, from, to).Scan(
		&out.SessionsWithCart, &out.SessionsCartOrder,
	); err != nil {
		return overviewRaw{}, err
	}
	return out, nil
}

// TimeseriesDaily agrega visitas/sessoes/pageviews/orders por DIA no fuso tz. Retorna
// um mapa (data YYYY-MM-DD -> ponto); o service preenche os dias faltantes com 0.
func (s *Store) TimeseriesDaily(ctx context.Context, accountID, restaurantID string, from, to time.Time, tz string) (map[string]AnalyticsTimePoint, error) {
	const q = `
		with sess as (
			select to_char(first_seen_at at time zone $5, 'YYYY-MM-DD') as d,
			       count(*) as sessions,
			       count(distinct nullif(device_id, '')) as visits
			from cardapio.sessions
			where restaurant_id = $1 and account_id = $2
			  and device_type <> 'bot'
			  and first_seen_at >= $3 and first_seen_at < $4
			group by 1
		),
		pv as (
			select to_char(created_at at time zone $5, 'YYYY-MM-DD') as d,
			       count(*) as pageviews
			from cardapio.events
			where restaurant_id = $1 and account_id = $2
			  and device_type <> 'bot' and name = 'page_view'
			  and created_at >= $3 and created_at < $4
			group by 1
		),
		ord as (
			select to_char(created_at at time zone $5, 'YYYY-MM-DD') as d,
			       count(*) as orders
			from cardapio.orders
			where restaurant_id = $1 and account_id = $2
			  and status <> 'cancelado'
			  and created_at >= $3 and created_at < $4
			group by 1
		)
		select d,
		       coalesce(visits, 0), coalesce(sessions, 0),
		       coalesce(pageviews, 0), coalesce(orders, 0)
		from (
			select d from sess union select d from pv union select d from ord
		) days
		left join sess using (d)
		left join pv using (d)
		left join ord using (d)`
	rows, err := s.pool.Query(ctx, q, restaurantID, accountID, from, to, tz)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make(map[string]AnalyticsTimePoint)
	for rows.Next() {
		var p AnalyticsTimePoint
		if err := rows.Scan(&p.Bucket, &p.Visits, &p.Sessions, &p.Pageviews, &p.Orders); err != nil {
			return nil, err
		}
		out[p.Bucket] = p
	}
	return out, rows.Err()
}

// TimeseriesHourOfDay agrega por hora-do-dia (0..23) no fuso tz. Mapa hour->ponto; o
// service preenche as 24 horas.
func (s *Store) TimeseriesHourOfDay(ctx context.Context, accountID, restaurantID string, from, to time.Time, tz string) (map[int]AnalyticsTimePoint, error) {
	const q = `
		with sess as (
			select extract(hour from first_seen_at at time zone $5)::int as h,
			       count(*) as sessions,
			       count(distinct nullif(device_id, '')) as visits
			from cardapio.sessions
			where restaurant_id = $1 and account_id = $2
			  and device_type <> 'bot'
			  and first_seen_at >= $3 and first_seen_at < $4
			group by 1
		),
		pv as (
			select extract(hour from created_at at time zone $5)::int as h,
			       count(*) as pageviews
			from cardapio.events
			where restaurant_id = $1 and account_id = $2
			  and device_type <> 'bot' and name = 'page_view'
			  and created_at >= $3 and created_at < $4
			group by 1
		),
		ord as (
			select extract(hour from created_at at time zone $5)::int as h,
			       count(*) as orders
			from cardapio.orders
			where restaurant_id = $1 and account_id = $2
			  and status <> 'cancelado'
			  and created_at >= $3 and created_at < $4
			group by 1
		)
		select h,
		       coalesce(visits, 0), coalesce(sessions, 0),
		       coalesce(pageviews, 0), coalesce(orders, 0)
		from (
			select h from sess union select h from pv union select h from ord
		) hours
		left join sess using (h)
		left join pv using (h)
		left join ord using (h)`
	rows, err := s.pool.Query(ctx, q, restaurantID, accountID, from, to, tz)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make(map[int]AnalyticsTimePoint)
	for rows.Next() {
		var p AnalyticsTimePoint
		if err := rows.Scan(&p.Hour, &p.Visits, &p.Sessions, &p.Pageviews, &p.Orders); err != nil {
			return nil, err
		}
		out[p.Hour] = p
	}
	return out, rows.Err()
}

// TimeseriesWeekdayHour agrega o heatmap dia-da-semana (0=domingo) x hora (0..23) no
// fuso tz. Chave = weekday*24+hour; o service preenche as 168 celulas.
func (s *Store) TimeseriesWeekdayHour(ctx context.Context, accountID, restaurantID string, from, to time.Time, tz string) (map[int]AnalyticsTimePoint, error) {
	const q = `
		with sess as (
			select extract(dow from first_seen_at at time zone $5)::int as wd,
			       extract(hour from first_seen_at at time zone $5)::int as h,
			       count(*) as sessions,
			       count(distinct nullif(device_id, '')) as visits
			from cardapio.sessions
			where restaurant_id = $1 and account_id = $2
			  and device_type <> 'bot'
			  and first_seen_at >= $3 and first_seen_at < $4
			group by 1, 2
		),
		ord as (
			select extract(dow from created_at at time zone $5)::int as wd,
			       extract(hour from created_at at time zone $5)::int as h,
			       count(*) as orders
			from cardapio.orders
			where restaurant_id = $1 and account_id = $2
			  and status <> 'cancelado'
			  and created_at >= $3 and created_at < $4
			group by 1, 2
		)
		select wd, h,
		       coalesce(visits, 0), coalesce(sessions, 0), coalesce(orders, 0)
		from (
			select wd, h from sess union select wd, h from ord
		) cells
		left join sess using (wd, h)
		left join ord using (wd, h)`
	rows, err := s.pool.Query(ctx, q, restaurantID, accountID, from, to, tz)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make(map[int]AnalyticsTimePoint)
	for rows.Next() {
		var p AnalyticsTimePoint
		if err := rows.Scan(&p.Weekday, &p.Hour, &p.Visits, &p.Sessions, &p.Orders); err != nil {
			return nil, err
		}
		out[p.Weekday*24+p.Hour] = p
	}
	return out, rows.Err()
}

// Sources agrega sessoes e pedidos por origem, lendo cardapio.sessions (utm/referrer
// ja agregados na ingestao). column e o nome de coluna ja validado pelo service
// (allowlist sourceDimensions) — nunca vem do cliente cru. Pedidos por origem casam
// orders.session_id = sessions.session_id.
func (s *Store) Sources(ctx context.Context, accountID, restaurantID, column string, from, to time.Time, limit int) ([]AnalyticsSource, error) {
	// column e interpolado de uma allowlist fechada (utm_source/utm_medium/
	// utm_campaign/referrer_host); jamais entrada de usuario. Os valores seguem
	// parametrizados.
	q := `
		select s.` + column + ` as value,
		       count(*) as sessions,
		       coalesce(sum(case when exists (
		           select 1 from cardapio.orders o
		           where o.restaurant_id = $1 and o.account_id = $2
		             and o.session_id = s.session_id
		             and o.status <> 'cancelado'
		       ) then 1 else 0 end), 0) as orders
		from cardapio.sessions s
		where s.restaurant_id = $1 and s.account_id = $2
		  and s.device_type <> 'bot'
		  and s.first_seen_at >= $3 and s.first_seen_at < $4
		group by s.` + column + `
		order by sessions desc, value asc
		limit $5`
	rows, err := s.pool.Query(ctx, q, restaurantID, accountID, from, to, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]AnalyticsSource, 0)
	for rows.Next() {
		var it AnalyticsSource
		if err := rows.Scan(&it.Value, &it.Sessions, &it.Orders); err != nil {
			return nil, err
		}
		out = append(out, it)
	}
	return out, rows.Err()
}

// Devices agrega o breakdown de UM atributo (device_type/browser/os) de
// cardapio.sessions. column ja vem da allowlist fechada do service.
func (s *Store) Devices(ctx context.Context, accountID, restaurantID, column string, from, to time.Time) ([]AnalyticsBreakdownItem, error) {
	q := `
		select coalesce(nullif(` + column + `, ''), 'unknown') as value, count(*) as sessions
		from cardapio.sessions
		where restaurant_id = $1 and account_id = $2
		  and device_type <> 'bot'
		  and first_seen_at >= $3 and first_seen_at < $4
		group by 1
		order by sessions desc, value asc`
	rows, err := s.pool.Query(ctx, q, restaurantID, accountID, from, to)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]AnalyticsBreakdownItem, 0)
	for rows.Next() {
		var it AnalyticsBreakdownItem
		if err := rows.Scan(&it.Value, &it.Sessions); err != nil {
			return nil, err
		}
		out = append(out, it)
	}
	return out, rows.Err()
}

// Pages agrega as paginas mais vistas (page_view) + dwell medio (segundos) por
// page_path. O dwell vem dos eventos page_dwell com dwell_ms > 0 da mesma page_path.
func (s *Store) Pages(ctx context.Context, accountID, restaurantID string, from, to time.Time, limit int) ([]AnalyticsPage, error) {
	const q = `
		with views as (
			select page_path, count(*) as views
			from cardapio.events
			where restaurant_id = $1 and account_id = $2
			  and device_type <> 'bot' and name = 'page_view'
			  and created_at >= $3 and created_at < $4
			group by page_path
		),
		dwell as (
			select page_path, avg(dwell_ms)::float8 as avg_dwell_ms
			from cardapio.events
			where restaurant_id = $1 and account_id = $2
			  and device_type <> 'bot' and name = 'page_dwell'
			  and dwell_ms > 0
			  and created_at >= $3 and created_at < $4
			group by page_path
		)
		select v.page_path, v.views, coalesce(d.avg_dwell_ms, 0)
		from views v
		left join dwell d using (page_path)
		order by v.views desc, v.page_path asc
		limit $5`
	rows, err := s.pool.Query(ctx, q, restaurantID, accountID, from, to, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]AnalyticsPage, 0)
	for rows.Next() {
		var p AnalyticsPage
		var avgMS float64
		if err := rows.Scan(&p.PagePath, &p.Views, &avgMS); err != nil {
			return nil, err
		}
		p.AvgDwellSeconds = avgMS / 1000
		out = append(out, p)
	}
	return out, rows.Err()
}
