package observability

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

type httpWindow struct {
	Total5xx  int64
	Total     int64
	Hour5xx   int64
	HourCalls int64
	Buckets   []int64
}

// RecentHTTP: taxa de 5xx dos últimos 15 min (http_requests) e latência da última
// hora cheia + atual (http_route_hourly).
func (s *Store) RecentHTTP(ctx context.Context, now time.Time) (httpWindow, error) {
	var w httpWindow
	if err := s.pool.QueryRow(quiet(ctx), `
		select count(*), count(*) filter (where status >= 500)
		from obs.http_requests where source = 'live' and occurred_at >= $1`,
		now.Add(-15*time.Minute)).Scan(&w.Total, &w.Total5xx); err != nil {
		return w, fmt.Errorf("health http window: %w", err)
	}
	err := s.pool.QueryRow(quiet(ctx), `
		select coalesce(sum(calls), 0), coalesce(sum(errors_5xx), 0),
		       coalesce((select array_agg(s order by o) from (
		          select o, sum(v) as s from obs.http_route_hourly h,
		                 unnest(h.buckets) with ordinality as u(v, o)
		          where h.bucket >= $1 group by o) b), '{}')
		from obs.http_route_hourly where bucket >= $1`,
		now.Add(-time.Hour).Truncate(time.Hour)).Scan(&w.HourCalls, &w.Hour5xx, &w.Buckets)
	if err != nil {
		return w, fmt.Errorf("health http hourly: %w", err)
	}
	return w, nil
}

type errorCounts struct {
	API1h, API24h, Front1h, Front24h int64
}

func (s *Store) ErrorCounts(ctx context.Context, now time.Time) (errorCounts, error) {
	var c errorCounts
	err := s.pool.QueryRow(quiet(ctx), `
		select
		  (select count(*) from obs.http_requests where source = 'live' and status >= 500 and status <> 499 and occurred_at >= $1),
		  (select count(*) from obs.http_requests where source = 'live' and status >= 500 and status <> 499 and occurred_at >= $2),
		  (select count(*) from obs.frontend_events where kind in ('js_error', 'api_failure') and occurred_at >= $1),
		  (select count(*) from obs.frontend_events where kind in ('js_error', 'api_failure') and occurred_at >= $2)`,
		now.Add(-time.Hour), now.Add(-24*time.Hour)).Scan(&c.API1h, &c.API24h, &c.Front1h, &c.Front24h)
	if err != nil {
		return c, fmt.Errorf("health error counts: %w", err)
	}
	return c, nil
}

type errorSignature struct {
	Source    string `json:"source"`
	Signature string `json:"signature"`
	Count     int64  `json:"count"`
}

// TopErrors: até 5 assinaturas de cada origem nas últimas 24 h (o service une e corta).
func (s *Store) TopErrors(ctx context.Context, now time.Time) ([]errorSignature, error) {
	rows, err := s.pool.Query(quiet(ctx), `
		(select 'api', coalesce(error_code, 'http_' || status) || ' ' || route, count(*)
		   from obs.http_requests where source = 'live' and status >= 500 and status <> 499 and occurred_at >= $1
		  group by 2 order by 3 desc limit 5)
		union all
		(select 'front', left(kind || ' ' || name, 200), count(*)
		   from obs.frontend_events where kind in ('js_error', 'api_failure') and occurred_at >= $1
		  group by kind, name order by 3 desc limit 5)`, now.Add(-24*time.Hour))
	if err != nil {
		return nil, fmt.Errorf("health top errors: %w", err)
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (errorSignature, error) {
		var item errorSignature
		return item, row.Scan(&item.Source, &item.Signature, &item.Count)
	})
}

type onlineUser struct {
	UserID   string    `json:"userId"`
	Name     string    `json:"name"`
	Page     string    `json:"page"`
	LastSeen time.Time `json:"lastSeen"`
	Tabs     int       `json:"tabs"`
}

// OnlineUsers agrupa a presença dos últimos 90 s por usuário. Só id e nome.
func (s *Store) OnlineUsers(ctx context.Context, now time.Time) ([]onlineUser, error) {
	rows, err := s.pool.Query(quiet(ctx), `
		select p.user_id, coalesce(max(u.display_name), ''),
		       (array_agg(p.page order by p.last_seen desc))[1], max(p.last_seen), count(*)::int
		from obs.presence p
		left join core.users u on u.id::text = p.user_id
		where p.last_seen >= $1
		group by p.user_id order by max(p.last_seen) desc limit 100`, now.Add(-90*time.Second))
	if err != nil {
		return nil, fmt.Errorf("health online: %w", err)
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (onlineUser, error) {
		var item onlineUser
		return item, row.Scan(&item.UserID, &item.Name, &item.Page, &item.LastSeen, &item.Tabs)
	})
}

type agentSample struct {
	At     time.Time
	Disk   []diskMetric `json:"disk"`
	Docker struct {
		Containers int `json:"containers"`
		Running    int `json:"running"`
	} `json:"docker"`
}

func (s *Store) latestAPISample(ctx context.Context) (*apiSample, error) {
	var sample apiSample
	var raw []byte
	err := s.pool.QueryRow(quiet(ctx), `
		select occurred_at, metrics from obs.system_samples where source = 'api'
		order by occurred_at desc limit 1`).Scan(&sample.At, &raw)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("health api sample: %w", err)
	}
	_ = json.Unmarshal(raw, &sample.Metrics)
	return &sample, nil
}

func (s *Store) latestAgentSample(ctx context.Context) (*agentSample, error) {
	var sample agentSample
	var raw []byte
	err := s.pool.QueryRow(quiet(ctx), `
		select occurred_at, metrics from obs.system_samples where source = 'agent'
		order by occurred_at desc limit 1`).Scan(&sample.At, &raw)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("health agent sample: %w", err)
	}
	_ = json.Unmarshal(raw, &sample)
	return &sample, nil
}

type pageStats struct {
	Events24h int64
	LCP       *float64
	INP       *float64
	CLS       *float64
	JSErr15m  int64
	JSErr1h   int64
}

func (s *Store) PageStats(ctx context.Context, now time.Time) (pageStats, error) {
	var p pageStats
	err := s.pool.QueryRow(quiet(ctx), `
		select count(*),
		       percentile_cont(0.75) within group (order by value) filter (where kind = 'vital' and name = 'LCP'),
		       percentile_cont(0.75) within group (order by value) filter (where kind = 'vital' and name = 'INP'),
		       percentile_cont(0.75) within group (order by value) filter (where kind = 'vital' and name = 'CLS'),
		       count(*) filter (where kind = 'js_error' and occurred_at >= $2),
		       count(*) filter (where kind = 'js_error' and occurred_at >= $3)
		from obs.frontend_events where occurred_at >= $1`,
		now.Add(-24*time.Hour), now.Add(-15*time.Minute), now.Add(-time.Hour)).
		Scan(&p.Events24h, &p.LCP, &p.INP, &p.CLS, &p.JSErr15m, &p.JSErr1h)
	if err != nil {
		return p, fmt.Errorf("health page stats: %w", err)
	}
	return p, nil
}

type securityCheck struct {
	Key       string     `json:"key"`
	Category  string     `json:"category"`
	Title     string     `json:"title"`
	Standard  string     `json:"standard"`
	Status    string     `json:"status"`
	Detail    string     `json:"detail"`
	CheckedAt *time.Time `json:"checkedAt"`
	Source    string     `json:"source"`
	UpdatedAt time.Time  `json:"-"`
}

func (s *Store) SecurityChecks(ctx context.Context) ([]securityCheck, error) {
	rows, err := s.pool.Query(quiet(ctx), `
		select key, category, title, standard, status, detail, checked_at, source, updated_at
		from obs.security_checks order by category, key`)
	if err != nil {
		return nil, fmt.Errorf("security checks: %w", err)
	}
	items, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (securityCheck, error) {
		var c securityCheck
		return c, row.Scan(&c.Key, &c.Category, &c.Title, &c.Standard, &c.Status, &c.Detail, &c.CheckedAt, &c.Source, &c.UpdatedAt)
	})
	if items == nil {
		items = []securityCheck{}
	}
	return items, err
}

type systemPoint struct {
	At      time.Time       `json:"at"`
	Metrics json.RawMessage `json:"metrics"`
}

type containerView struct {
	Container    string    `json:"container"`
	Service      string    `json:"service"`
	State        string    `json:"state"`
	Health       string    `json:"health"`
	RestartCount int       `json:"restartCount"`
	CPUPct       *float64  `json:"cpuPct"`
	MemBytes     *int64    `json:"memBytes"`
	MemLimit     *int64    `json:"memLimitBytes"`
	At           time.Time `json:"at"`
}

// SystemSeries devolve amostras de uma origem nas últimas `hours`, em ordem
// cronológica, reduzidas a no máximo ~360 pontos.
func (s *Store) SystemSeries(ctx context.Context, source string, hours int) ([]systemPoint, error) {
	rows, err := s.pool.Query(quiet(ctx), `
		select occurred_at, metrics from (
		  select occurred_at, metrics, row_number() over (order by occurred_at) as n,
		         count(*) over () as total
		  from obs.system_samples where source = $1 and occurred_at >= now() - make_interval(hours => $2)) t
		where n % greatest(total / 360, 1) = 0 or n = total
		order by occurred_at`, source, hours)
	if err != nil {
		return nil, fmt.Errorf("system series: %w", err)
	}
	items, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (systemPoint, error) {
		var p systemPoint
		return p, row.Scan(&p.At, &p.Metrics)
	})
	if items == nil {
		items = []systemPoint{}
	}
	return items, err
}

func (s *Store) LatestContainerViews(ctx context.Context, hours int) ([]containerView, error) {
	rows, err := s.pool.Query(quiet(ctx), `
		select distinct on (container) container, service, state, health, restart_count,
		       cpu_pct, mem_bytes, mem_limit_bytes, occurred_at
		from obs.container_samples where occurred_at >= now() - make_interval(hours => $1)
		order by container, occurred_at desc`, hours)
	if err != nil {
		return nil, fmt.Errorf("container views: %w", err)
	}
	items, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (containerView, error) {
		var c containerView
		return c, row.Scan(&c.Container, &c.Service, &c.State, &c.Health, &c.RestartCount, &c.CPUPct, &c.MemBytes, &c.MemLimit, &c.At)
	})
	if items == nil {
		items = []containerView{}
	}
	return items, err
}
