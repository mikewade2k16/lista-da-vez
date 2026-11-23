package observability

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

var ErrNotFound = errors.New("not found")

func (s *Store) Runs(ctx context.Context, limit int) ([]map[string]any, error) {
	return s.collectMaps(ctx, `
		select id, started_at as "startedAt", finished_at as "finishedAt", git_sha as "gitSha", branch,
		       env, base_url as "baseUrl", filters, summary, coverage, scores, regressions, exit_code as "exitCode"
		from obs.runs order by started_at desc limit $1`, limit)
}

func (s *Store) Run(ctx context.Context, id string) (map[string]any, []map[string]any, error) {
	runs, err := s.collectMaps(ctx, `
		select id, started_at as "startedAt", finished_at as "finishedAt", git_sha as "gitSha", branch,
		       env, base_url as "baseUrl", filters, summary, coverage, scores, regressions, exit_code as "exitCode"
		from obs.runs where id = $1`, id)
	if err != nil {
		return nil, nil, err
	}
	if len(runs) == 0 {
		return nil, nil, ErrNotFound
	}
	results, err := s.collectMaps(ctx, `
		select kind, module, target, status, reason, metrics
		from obs.run_results where run_id = $1 order by kind, module, target`, id)
	if err != nil {
		return nil, nil, err
	}
	return runs[0], results, nil
}

func (s *Store) LatestRunResults(ctx context.Context, kind string) ([]map[string]any, error) {
	return s.collectMaps(ctx, `
		select r.run_id as "runId", r.module, r.target, r.status, r.reason, r.metrics
		from obs.run_results r
		where r.kind = $1 and r.run_id = (
		  select rr.run_id from obs.run_results rr join obs.runs ru on ru.id = rr.run_id
		  where rr.kind = $1 order by ru.started_at desc limit 1)
		order by r.module, r.target`, kind)
}

func (s *Store) Trace(ctx context.Context, correlationID string) (map[string]any, error) {
	sections := map[string]string{
		"requests":       `select occurred_at as "occurredAt", method, route, status, duration_ms as "durationMs", response_bytes as "responseBytes", user_id as "userId", account_id as "accountId", query_count as "queryCount", query_ms as "queryMs", external_count as "externalCount", error_code as "errorCode", error_message as "errorMessage", error_stack as "errorStack", source from obs.http_requests where correlation_id = $1 order by occurred_at limit 20`,
		"slowQueries":    `select occurred_at as "occurredAt", route, fingerprint, statement, duration_ms as "durationMs", error from obs.db_slow_queries where correlation_id = $1 order by occurred_at limit 100`,
		"nPlusOne":       `select occurred_at as "occurredAt", route, fingerprint, statement, repetitions from obs.db_n_plus_one where correlation_id = $1 limit 50`,
		"externalCalls":  `select occurred_at as "occurredAt", integration, method, path, status, duration_ms as "durationMs", timeout, error from obs.external_calls where correlation_id = $1 order by occurred_at limit 100`,
		"frontendEvents": `select occurred_at as "occurredAt", page_view_id as "pageViewId", page, kind, name, value, status, correlation_id as "correlationId", detail from obs.frontend_events where correlation_id = $1 or page_view_id = $2 order by occurred_at limit 200`,
	}
	pageViewID := correlationID
	if index := lastDash(correlationID); index > 0 {
		pageViewID = correlationID[:index]
	}
	result := make(map[string]any, len(sections))
	total := 0
	for name, query := range sections {
		var items []map[string]any
		var err error
		if name == "frontendEvents" {
			items, err = s.collectMaps(ctx, query, correlationID, pageViewID)
		} else {
			items, err = s.collectMaps(ctx, query, correlationID)
		}
		if err != nil {
			return nil, fmt.Errorf("trace %s: %w", name, err)
		}
		total += len(items)
		result[name] = items
	}
	if total == 0 {
		return nil, ErrNotFound
	}
	return result, nil
}

func lastDash(value string) int {
	for index := len(value) - 1; index >= 0; index-- {
		if value[index] == '-' {
			return index
		}
	}
	return -1
}

func (s *Store) RunRequests(ctx context.Context) ([]map[string]any, error) {
	return s.collectMaps(ctx, `
		select id, requested_at as "requestedAt", requested_by as "requestedBy", module, only_kinds as "only",
		       status, run_id as "runId", started_at as "startedAt", finished_at as "finishedAt", right(log, 4000) as log
		from obs.run_requests order by requested_at desc limit 30`)
}

func (s *Store) WorkerOnline(ctx context.Context, within time.Duration) (bool, error) {
	var online bool
	err := s.pool.QueryRow(quiet(ctx), `select exists(select 1 from obs.workers where last_seen >= $1)`, time.Now().UTC().Add(-within)).Scan(&online)
	if err != nil {
		return false, fmt.Errorf("query workers: %w", err)
	}
	return online, nil
}

func (s *Store) EnqueueRunRequest(ctx context.Context, requestedBy, module, only string) (int64, error) {
	var id int64
	err := s.pool.QueryRow(quiet(ctx), `
		insert into obs.run_requests (requested_by, module, only_kinds)
		values ($1, nullif($2, ''), nullif($3, '')) returning id`, requestedBy, module, only).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("enqueue run request: %w", err)
	}
	return id, nil
}

func (s *Store) PendingRequests(ctx context.Context) (int, error) {
	var count int
	err := s.pool.QueryRow(quiet(ctx), `select count(*) from obs.run_requests where status in ('queued', 'running')`).Scan(&count)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, nil
	}
	return count, err
}
