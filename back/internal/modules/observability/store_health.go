package observability

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

// AlertMetrics reúne as métricas das regras de alerta. Qualquer falha devolve erro
// e a rodada é descartada: avaliar com dados parciais resolveria alertas por engano.
func (s *Store) AlertMetrics(ctx context.Context, now time.Time) (alertMetrics, error) {
	m := alertMetrics{Now: now}
	if err := s.pool.QueryRow(quiet(ctx), `
		select count(*), count(*) filter (where status >= 500)
		from obs.http_requests
		where source = 'live' and occurred_at >= $1`, now.Add(-5*time.Minute)).Scan(&m.HTTPTotal5m, &m.HTTP5xx5m); err != nil {
		return m, fmt.Errorf("alert metrics http: %w", err)
	}
	var err error
	if m.RouteErrors5m, err = s.routeErrors(ctx, now.Add(-5*time.Minute)); err != nil {
		return m, err
	}
	if m.SlowRoutes15m, err = s.slowRoutes(ctx, now.Add(-15*time.Minute)); err != nil {
		return m, err
	}
	if m.APISamples, err = s.apiSamples(ctx, now.Add(-30*time.Minute)); err != nil {
		return m, err
	}
	if err = s.pool.QueryRow(quiet(ctx), `
		select (select max(occurred_at) from obs.system_samples where source = 'api'),
		       (select max(occurred_at) from obs.system_samples where source = 'agent')`).Scan(&m.LastAPIAt, &m.LastAgentAt); err != nil {
		return m, fmt.Errorf("alert metrics last samples: %w", err)
	}
	if m.Containers, err = s.latestContainers(ctx, now.Add(-10*time.Minute)); err != nil {
		return m, err
	}
	if m.Disks, err = s.latestDisks(ctx); err != nil {
		return m, err
	}
	if m.TLS, m.TLSErrorSince, err = s.latestTLS(ctx, now); err != nil {
		return m, err
	}
	if err = s.pool.QueryRow(quiet(ctx), `
		select count(*) from obs.frontend_events where kind = 'js_error' and occurred_at >= $1`,
		now.Add(-15*time.Minute)).Scan(&m.FrontendJSErr15); err != nil {
		return m, fmt.Errorf("alert metrics front: %w", err)
	}
	return m, nil
}

func (s *Store) routeErrors(ctx context.Context, from time.Time) ([]routeCount, error) {
	rows, err := s.pool.Query(quiet(ctx), `
		select route, count(*) from obs.http_requests
		where source = 'live' and status >= 500 and occurred_at >= $1
		group by route having count(*) >= 5 order by count(*) desc limit 50`, from)
	if err != nil {
		return nil, fmt.Errorf("alert metrics route errors: %w", err)
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (routeCount, error) {
		var item routeCount
		return item, row.Scan(&item.Route, &item.Count)
	})
}

func (s *Store) slowRoutes(ctx context.Context, from time.Time) ([]routeLatency, error) {
	rows, err := s.pool.Query(quiet(ctx), `
		select route, percentile_cont(0.95) within group (order by duration_ms), count(*)
		from obs.http_requests
		where source = 'live' and occurred_at >= $1 and route <> 'UNMATCHED'
		group by route
		having count(*) >= 10 and percentile_cont(0.95) within group (order by duration_ms) > 2000
		order by 2 desc limit 50`, from)
	if err != nil {
		return nil, fmt.Errorf("alert metrics slow routes: %w", err)
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (routeLatency, error) {
		var item routeLatency
		return item, row.Scan(&item.Route, &item.P95Ms, &item.Calls)
	})
}

// apiSamples devolve amostras source='api' da mais recente para a mais antiga.
func (s *Store) apiSamples(ctx context.Context, from time.Time) ([]apiSample, error) {
	rows, err := s.pool.Query(quiet(ctx), `
		select occurred_at, metrics from obs.system_samples
		where source = 'api' and occurred_at >= $1
		order by occurred_at desc limit 200`, from)
	if err != nil {
		return nil, fmt.Errorf("alert metrics api samples: %w", err)
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (apiSample, error) {
		var sample apiSample
		var raw []byte
		if err := row.Scan(&sample.At, &raw); err != nil {
			return sample, err
		}
		// Amostra com JSON inesperado vira zeros: não derruba a avaliação.
		_ = json.Unmarshal(raw, &sample.Metrics)
		return sample, nil
	})
}

// latestContainers devolve a última amostra de cada container (coletada desde
// `from`) e o aumento de restart_count na última hora.
func (s *Store) latestContainers(ctx context.Context, from time.Time) ([]containerState, error) {
	rows, err := s.pool.Query(quiet(ctx), `
		select c.container, c.service, c.state, c.health, c.restart_count,
		       greatest(c.restart_count - coalesce((
		         select min(c2.restart_count) from obs.container_samples c2
		         where c2.container = c.container and c2.occurred_at >= c.occurred_at - interval '1 hour'), c.restart_count), 0),
		       c.occurred_at, c.cpu_pct, c.mem_bytes, c.mem_limit_bytes,
		       c.restart_policy, c.exit_code
		from (select distinct on (container) * from obs.container_samples
		      where occurred_at >= $1 order by container, occurred_at desc) c
		order by c.container`, from)
	if err != nil {
		return nil, fmt.Errorf("alert metrics containers: %w", err)
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (containerState, error) {
		var item containerState
		return item, row.Scan(&item.Container, &item.Service, &item.State, &item.Health, &item.RestartCount,
			&item.RestartDelta, &item.At, &item.CPUPct, &item.MemBytes, &item.MemLimit, &item.RestartPolicy, &item.ExitCode)
	})
}

func (s *Store) latestDisks(ctx context.Context) ([]diskMetric, error) {
	var raw []byte
	err := s.pool.QueryRow(quiet(ctx), `
		select coalesce(metrics->'disk', '[]'::jsonb) from obs.system_samples
		where source = 'agent' order by occurred_at desc limit 1`).Scan(&raw)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("alert metrics disks: %w", err)
	}
	var disks []diskMetric
	_ = json.Unmarshal(raw, &disks)
	return disks, nil
}

// alertRow é uma linha de obs.alerts. Estado interno do motor fica em detail._notify.
type alertRow struct {
	ID              int64          `json:"id"`
	Rule            string         `json:"rule"`
	Fingerprint     string         `json:"-"`
	Severity        string         `json:"severity"`
	Area            string         `json:"area"`
	Title           string         `json:"title"`
	Detail          map[string]any `json:"detail"`
	Status          string         `json:"status"`
	FirstSeen       time.Time      `json:"firstSeen"`
	LastSeen        time.Time      `json:"lastSeen"`
	ResolvedAt      *time.Time     `json:"resolvedAt"`
	NotifiedAt      *time.Time     `json:"notifiedAt"`
	NotifyCount     int            `json:"notifyCount"`
	LastNotifyError *string        `json:"lastNotifyError"`
	AcknowledgedAt  *time.Time     `json:"acknowledgedAt"`
	AcknowledgedBy  *string        `json:"acknowledgedBy"`
	SilencedUntil   *time.Time     `json:"silencedUntil"`
	SilencedBy      *string        `json:"silencedBy"`
}

const alertColumns = `id, rule, fingerprint, severity, area, title, detail, status, first_seen, last_seen,
	resolved_at, notified_at, notify_count, last_notify_error,
	acknowledged_at, acknowledged_by, silenced_until, silenced_by`

func scanAlert(row pgx.CollectableRow) (alertRow, error) {
	var a alertRow
	var raw []byte
	err := row.Scan(&a.ID, &a.Rule, &a.Fingerprint, &a.Severity, &a.Area, &a.Title, &raw, &a.Status, &a.FirstSeen,
		&a.LastSeen, &a.ResolvedAt, &a.NotifiedAt, &a.NotifyCount, &a.LastNotifyError,
		&a.AcknowledgedAt, &a.AcknowledgedBy, &a.SilencedUntil, &a.SilencedBy)
	a.Detail = map[string]any{}
	_ = json.Unmarshal(raw, &a.Detail)
	if a.Detail == nil {
		a.Detail = map[string]any{}
	}
	return a, err
}

// ListAlerts filtra por status open|resolved|all; abertos primeiro por severidade.
func (s *Store) ListAlerts(ctx context.Context, status string, limit int) ([]alertRow, error) {
	rows, err := s.pool.Query(quiet(ctx), `
		select `+alertColumns+` from obs.alerts
		where $1 = 'all' or status = $1
		order by (status = 'open') desc,
		         case severity when 'critical' then 4 when 'high' then 3 when 'medium' then 2 else 1 end desc,
		         last_seen desc
		limit $2`, status, limit)
	if err != nil {
		return nil, fmt.Errorf("list alerts: %w", err)
	}
	return pgx.CollectRows(rows, scanAlert)
}

func (s *Store) InsertAlert(ctx context.Context, c condition, now time.Time) (int64, error) {
	detail, _ := json.Marshal(c.Detail)
	var id int64
	err := s.pool.QueryRow(quiet(ctx), `
		insert into obs.alerts (rule, fingerprint, severity, area, title, detail, first_seen, last_seen)
		values ($1, $2, $3, $4, $5, $6::jsonb, $7, $7)
		on conflict (fingerprint) where status = 'open'
		do update set last_seen = excluded.last_seen
		returning id`, c.Rule, c.Fingerprint(), c.Severity, c.Area, c.Title, string(detail), now).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("insert alert: %w", err)
	}
	return id, nil
}

// UpdateAlert atualiza um alerta aberto preservando o estado interno de notificação.
func (s *Store) UpdateAlert(ctx context.Context, id int64, c condition, now time.Time) error {
	detail, _ := json.Marshal(c.Detail)
	_, err := s.pool.Exec(quiet(ctx), `
		update obs.alerts set severity = $2, title = $3, last_seen = $4,
		       detail = $5::jsonb || jsonb_strip_nulls(jsonb_build_object('_notify', detail->'_notify'))
		where id = $1 and status = 'open'`, id, c.Severity, c.Title, now, string(detail))
	if err != nil {
		return fmt.Errorf("update alert: %w", err)
	}
	return nil
}

func (s *Store) ResolveAlert(ctx context.Context, id int64, now time.Time) error {
	_, err := s.pool.Exec(quiet(ctx), `
		update obs.alerts set status = 'resolved', resolved_at = $2 where id = $1 and status = 'open'`, id, now)
	if err != nil {
		return fmt.Errorf("resolve alert: %w", err)
	}
	return nil
}

// MarkNotified registra o envio: notified_at, contador, severidade notificada.
func (s *Store) MarkNotified(ctx context.Context, id int64, severity string, now time.Time) error {
	state, _ := json.Marshal(notifyState{Severity: severity, AttemptAt: &now})
	_, err := s.pool.Exec(quiet(ctx), `
		update obs.alerts set notified_at = $2, notify_count = notify_count + 1, last_notify_error = null,
		       detail = jsonb_set(detail, '{_notify}', $3::jsonb) where id = $1`, id, now, string(state))
	if err != nil {
		return fmt.Errorf("mark notified: %w", err)
	}
	return nil
}

// MarkNotifyError grava a falha e o instante da tentativa (para o backoff).
func (s *Store) MarkNotifyError(ctx context.Context, row alertRow, message string, now time.Time) error {
	state := notifyStateFrom(row.Detail)
	state.AttemptAt = &now
	encoded, _ := json.Marshal(state)
	_, err := s.pool.Exec(quiet(ctx), `
		update obs.alerts set last_notify_error = $2, detail = jsonb_set(detail, '{_notify}', $3::jsonb)
		where id = $1`, row.ID, message, string(encoded))
	if err != nil {
		return fmt.Errorf("mark notify error: %w", err)
	}
	return nil
}

// latestTLS lê metrics.tls da última amostra source='agent'. Ausente = sem condição.
// Quando algum host tem erro, busca desde quando o erro aparece nas amostras
// contínuas (janela de 13 h) para a regra exigir duas checagens (a cada 6 h).
func (s *Store) latestTLS(ctx context.Context, now time.Time) ([]tlsMetric, map[string]time.Time, error) {
	var raw []byte
	err := s.pool.QueryRow(quiet(ctx), `
		select metrics->'tls' from obs.system_samples
		where source = 'agent' order by occurred_at desc limit 1`).Scan(&raw)
	if err == pgx.ErrNoRows {
		return nil, nil, nil
	}
	if err != nil {
		return nil, nil, fmt.Errorf("alert metrics tls: %w", err)
	}
	items := parseTLS(raw)
	hasError := false
	for _, item := range items {
		hasError = hasError || item.Error != ""
	}
	if !hasError {
		return items, nil, nil
	}
	rows, err := s.pool.Query(quiet(ctx), `
		select e->>'host', min(s.occurred_at)
		from obs.system_samples s,
		     jsonb_array_elements(case when jsonb_typeof(s.metrics->'tls') = 'array' then s.metrics->'tls' else '[]'::jsonb end) e
		where s.source = 'agent' and s.occurred_at >= $1 and coalesce(e->>'error', '') <> ''
		group by 1`, now.Add(-13*time.Hour))
	if err != nil {
		return nil, nil, fmt.Errorf("alert metrics tls errors: %w", err)
	}
	since := map[string]time.Time{}
	for rows.Next() {
		var host string
		var at time.Time
		if err := rows.Scan(&host, &at); err != nil {
			rows.Close()
			return nil, nil, fmt.Errorf("alert metrics tls errors: %w", err)
		}
		since[host] = at
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, nil, fmt.Errorf("alert metrics tls errors: %w", err)
	}
	return items, since, nil
}
