package observability

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	platformobs "github.com/mikewade2k16/lista-da-vez/back/internal/platform/observability"
)

// Store lê o schema obs. Todas as leituras usam WithoutTracing para o painel
// não medir a si mesmo.
type Store struct {
	pool *pgxpool.Pool
}

func NewStore(pool *pgxpool.Pool) *Store { return &Store{pool: pool} }

func quiet(ctx context.Context) context.Context { return platformobs.WithoutTracing(ctx) }

func since(days int) time.Time { return time.Now().UTC().AddDate(0, 0, -days) }

type routeRow struct {
	Route        string
	PathModule   string
	Module       string
	Calls        int64
	Errors4xx    int64
	Errors5xx    int64
	TotalMS      float64
	MaxMS        float64
	QueriesTotal int64
	QueriesMax   int
	Buckets      []int64
}

// RouteStats agrega http_route_hourly no período, unindo o módulo do inventário.
func (s *Store) RouteStats(ctx context.Context, days int, source string) ([]routeRow, error) {
	rows, err := s.pool.Query(quiet(ctx), `
		select h.route, max(h.path_module), coalesce(max(i.module), ''),
		       sum(h.calls), sum(h.errors_4xx), sum(h.errors_5xx), sum(h.total_ms), max(h.max_ms),
		       sum(h.queries_total), max(h.queries_max),
		       (select array_agg(s order by o) from (
		          select o, sum(v) as s from obs.http_route_hourly h2,
		                 unnest(h2.buckets) with ordinality as u(v, o)
		          where h2.route = h.route and h2.bucket >= $1 group by o) b)
		from obs.http_route_hourly h
		left join obs.inventory_items i on i.kind = 'endpoint' and i.key = h.route
		where h.bucket >= $1 and ($2 = '' or h.route <> 'UNMATCHED')
		group by h.route
		order by sum(h.calls) desc
		limit 2000`, since(days), source)
	if err != nil {
		return nil, fmt.Errorf("query route stats: %w", err)
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (routeRow, error) {
		var item routeRow
		var buckets []int64
		err := row.Scan(&item.Route, &item.PathModule, &item.Module, &item.Calls, &item.Errors4xx, &item.Errors5xx,
			&item.TotalMS, &item.MaxMS, &item.QueriesTotal, &item.QueriesMax, &buckets)
		item.Buckets = buckets
		return item, err
	})
}

type liveTotals struct {
	Requests  int64
	Errors5xx int64
	Buckets   []int64
}

func (s *Store) LiveTotals(ctx context.Context, days int) (liveTotals, error) {
	var totals liveTotals
	err := s.pool.QueryRow(quiet(ctx), `
		select coalesce(sum(calls), 0), coalesce(sum(errors_5xx), 0),
		       coalesce((select array_agg(s order by o) from (
		          select o, sum(v) as s from obs.http_route_hourly h,
		                 unnest(h.buckets) with ordinality as u(v, o)
		          where h.bucket >= $1 group by o) b), '{}')
		from obs.http_route_hourly where bucket >= $1`, since(days)).Scan(&totals.Requests, &totals.Errors5xx, &totals.Buckets)
	if err != nil {
		return totals, fmt.Errorf("query live totals: %w", err)
	}
	return totals, nil
}

// collectMaps devolve linhas como mapas JSON-friendly (consultas de leitura do painel).
func (s *Store) collectMaps(ctx context.Context, query string, args ...any) ([]map[string]any, error) {
	rows, err := s.pool.Query(quiet(ctx), query, args...)
	if err != nil {
		return nil, err
	}
	items, err := pgx.CollectRows(rows, pgx.RowToMap)
	if err != nil {
		return nil, err
	}
	if items == nil {
		items = []map[string]any{}
	}
	return items, nil
}

func (s *Store) SlowQueries(ctx context.Context, days int) ([]map[string]any, error) {
	return s.collectMaps(ctx, `
		select fingerprint, min(statement) as statement, count(*) as occurrences,
		       round(avg(duration_ms)::numeric, 1) as "avgMs", round(max(duration_ms)::numeric, 1) as "maxMs",
		       count(*) filter (where error is not null) as errors,
		       (array_agg(route order by occurred_at desc))[1] as route,
		       (array_agg(correlation_id order by occurred_at desc))[1] as "lastCorrelationId",
		       max(occurred_at) as "lastSeen"
		from obs.db_slow_queries where occurred_at >= $1
		group by fingerprint order by max(duration_ms) desc limit 50`, since(days))
}

func (s *Store) NPlusOne(ctx context.Context, days int) ([]map[string]any, error) {
	return s.collectMaps(ctx, `
		select route, fingerprint, min(statement) as statement, count(*) as occurrences,
		       max(repetitions) as "maxRepetitions",
		       (array_agg(correlation_id order by occurred_at desc))[1] as "lastCorrelationId",
		       max(occurred_at) as "lastSeen"
		from obs.db_n_plus_one where occurred_at >= $1
		group by route, fingerprint order by max(repetitions) desc limit 50`, since(days))
}

func (s *Store) TopQueries(ctx context.Context, days int) ([]map[string]any, error) {
	return s.collectMaps(ctx, `
		select fingerprint, min(statement) as statement, min(verb) as verb, sum(calls) as calls,
		       round(sum(total_ms)::numeric, 1) as "totalMs", round(max(max_ms)::numeric, 1) as "maxMs",
		       round((sum(total_ms) / nullif(sum(calls), 0))::numeric, 2) as "avgMs", sum(errors) as errors,
		       count(distinct route) as routes
		from obs.db_query_stats where bucket >= $1
		group by fingerprint order by sum(total_ms) desc limit 50`, since(days))
}

func (s *Store) PageVitals(ctx context.Context, days int) ([]map[string]any, error) {
	return s.collectMaps(ctx, `
		select page, count(distinct page_view_id) as samples,
		       round((percentile_cont(0.75) within group (order by value) filter (where kind = 'vital' and name = 'LCP'))::numeric, 0) as "lcpP75",
		       round((percentile_cont(0.75) within group (order by value) filter (where kind = 'vital' and name = 'INP'))::numeric, 0) as "inpP75",
		       round((percentile_cont(0.75) within group (order by value) filter (where kind = 'vital' and name = 'CLS'))::numeric, 3) as "clsP75",
		       round((percentile_cont(0.75) within group (order by value) filter (where kind = 'vital' and name = 'TTFB'))::numeric, 0) as "ttfbP75",
		       round((percentile_cont(0.75) within group (order by value) filter (where kind = 'pageload'))::numeric, 0) as "loadP75",
		       count(*) filter (where kind = 'js_error') as "jsErrors",
		       count(*) filter (where kind = 'api_failure') as "apiFailures"
		from obs.frontend_events where occurred_at >= $1
		group by page order by count(distinct page_view_id) desc limit 300`, since(days))
}

func (s *Store) ErrorGroups(ctx context.Context, days int) ([]map[string]any, error) {
	return s.collectMaps(ctx, `
		select * from (
		  select 'api' as source, coalesce(error_code, 'http_' || status) || ' ' || coalesce(left(error_message, 160), '') as signature,
		         route as target, status, count(*) as count, max(occurred_at) as "lastSeen",
		         (array_agg(correlation_id order by occurred_at desc))[1:5] as "correlationIds"
		  from obs.http_requests where occurred_at >= $1 and status >= 500
		  group by 2, route, status
		  union all
		  select 'front', left(name, 200), page, null, count(*), max(occurred_at),
		         (array_agg(coalesce(correlation_id, page_view_id) order by occurred_at desc))[1:5]
		  from obs.frontend_events where occurred_at >= $1 and kind in ('js_error', 'api_failure')
		  group by kind, left(name, 200), page
		) e order by count desc limit 200`, since(days))
}
