package observability

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func nullable(value string) any {
	if value == "" {
		return nil
	}
	return value
}

// writeBatch grava dados brutos com COPY e agregados com upsert, numa transação.
func writeBatch(ctx context.Context, pool *pgxpool.Pool, batch *eventBatch, routes map[routeKey]*routeAggregate, queries map[queryKey]*queryAggregate) (int, error) {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return 0, fmt.Errorf("begin obs flush: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	written := 0
	copies := []struct {
		table   string
		columns []string
		rows    [][]any
	}{
		{"http_requests", []string{"occurred_at", "correlation_id", "method", "route", "path_module", "status", "duration_ms", "response_bytes", "user_id", "account_id", "query_count", "query_ms", "external_count", "error_code", "error_message", "error_stack", "source"}, requestRows(batch.requests)},
		{"db_slow_queries", []string{"occurred_at", "correlation_id", "route", "fingerprint", "statement", "duration_ms", "error"}, slowRows(batch.slow)},
		{"db_n_plus_one", []string{"occurred_at", "correlation_id", "route", "fingerprint", "statement", "repetitions"}, nPlusOneRows(batch.nPlusOne)},
		{"external_calls", []string{"occurred_at", "correlation_id", "route", "integration", "method", "path", "status", "duration_ms", "timeout", "error"}, externalRows(batch.externals)},
		{"frontend_events", []string{"occurred_at", "page_view_id", "page", "kind", "name", "value", "status", "correlation_id", "detail", "user_id", "user_agent"}, frontendRows(batch.frontend)},
	}
	for _, item := range copies {
		if len(item.rows) == 0 {
			continue
		}
		count, err := tx.CopyFrom(ctx, pgx.Identifier{"obs", item.table}, item.columns, pgx.CopyFromRows(item.rows))
		if err != nil {
			return 0, fmt.Errorf("copy obs.%s: %w", item.table, err)
		}
		written += int(count)
	}

	upserts := &pgx.Batch{}
	for key, agg := range routes {
		upserts.Queue(upsertRouteHourlySQL, key.Bucket, key.Route, agg.PathModule, agg.Calls, agg.Errors4xx, agg.Errors5xx, agg.TotalMS, agg.MaxMS, agg.QueriesTotal, agg.QueriesMax, agg.Buckets)
	}
	for key, agg := range queries {
		upserts.Queue(upsertQueryStatsSQL, key.Bucket, key.Fingerprint, key.Route, agg.Statement, agg.Verb, agg.Calls, agg.Errors, agg.TotalMS, agg.MaxMS)
	}
	if upserts.Len() > 0 {
		if err := tx.SendBatch(ctx, upserts).Close(); err != nil {
			return 0, fmt.Errorf("upsert obs aggregates: %w", err)
		}
		written += upserts.Len()
	}
	if len(batch.presence) > 0 {
		presence := &pgx.Batch{}
		for _, item := range batch.presence {
			presence.Queue(upsertPresenceSQL, item.PageViewID, item.UserID, item.Page, item.UserAgent, item.SeenAt)
		}
		if err := tx.SendBatch(ctx, presence).Close(); err != nil {
			return 0, fmt.Errorf("upsert obs.presence: %w", err)
		}
		written += presence.Len()
	}
	if err := tx.Commit(ctx); err != nil {
		return 0, fmt.Errorf("commit obs flush: %w", err)
	}
	return written, nil
}

const upsertRouteHourlySQL = `
insert into obs.http_route_hourly
  (bucket, route, path_module, calls, errors_4xx, errors_5xx, total_ms, max_ms, queries_total, queries_max, buckets)
values ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
on conflict (bucket, route) do update set
  calls = obs.http_route_hourly.calls + excluded.calls,
  errors_4xx = obs.http_route_hourly.errors_4xx + excluded.errors_4xx,
  errors_5xx = obs.http_route_hourly.errors_5xx + excluded.errors_5xx,
  total_ms = obs.http_route_hourly.total_ms + excluded.total_ms,
  max_ms = greatest(obs.http_route_hourly.max_ms, excluded.max_ms),
  queries_total = obs.http_route_hourly.queries_total + excluded.queries_total,
  queries_max = greatest(obs.http_route_hourly.queries_max, excluded.queries_max),
  buckets = obs.add_bigint_arrays(obs.http_route_hourly.buckets, excluded.buckets)`

const upsertQueryStatsSQL = `
insert into obs.db_query_stats
  (bucket, fingerprint, route, statement, verb, calls, errors, total_ms, max_ms)
values ($1, $2, $3, $4, $5, $6, $7, $8, $9)
on conflict (bucket, fingerprint, route) do update set
  calls = obs.db_query_stats.calls + excluded.calls,
  errors = obs.db_query_stats.errors + excluded.errors,
  total_ms = obs.db_query_stats.total_ms + excluded.total_ms,
  max_ms = greatest(obs.db_query_stats.max_ms, excluded.max_ms)`

func requestRows(events []httpEvent) [][]any {
	rows := make([][]any, 0, len(events))
	for _, e := range events {
		rows = append(rows, []any{e.OccurredAt, e.CorrelationID, e.Method, e.Route, e.PathModule, e.Status, e.DurationMS, e.ResponseBytes, nullable(e.UserID), nullable(e.AccountID), e.QueryCount, e.QueryMS, e.ExternalCount, nullable(e.ErrorCode), nullable(e.ErrorMessage), nullable(e.ErrorStack), e.Source})
	}
	return rows
}

func slowRows(events []slowQueryEvent) [][]any {
	rows := make([][]any, 0, len(events))
	for _, e := range events {
		rows = append(rows, []any{e.OccurredAt, nullable(e.CorrelationID), e.Route, e.Fingerprint, e.Statement, e.DurationMS, nullable(e.Error)})
	}
	return rows
}

func nPlusOneRows(events []nPlusOneEvent) [][]any {
	rows := make([][]any, 0, len(events))
	for _, e := range events {
		rows = append(rows, []any{e.OccurredAt, e.CorrelationID, e.Route, e.Fingerprint, e.Statement, e.Repetitions})
	}
	return rows
}

func externalRows(events []externalEvent) [][]any {
	rows := make([][]any, 0, len(events))
	for _, e := range events {
		rows = append(rows, []any{e.OccurredAt, nullable(e.CorrelationID), e.Route, e.Integration, e.Method, e.Path, e.Status, e.DurationMS, e.Timeout, nullable(e.Error)})
	}
	return rows
}

func frontendRows(events []frontendEvent) [][]any {
	rows := make([][]any, 0, len(events))
	for _, e := range events {
		var detail any
		if len(e.Detail) > 0 {
			if encoded, err := json.Marshal(e.Detail); err == nil {
				detail = json.RawMessage(encoded)
			}
		}
		rows = append(rows, []any{e.OccurredAt, e.PageViewID, e.Page, e.Kind, e.Name, e.Value, e.Status, nullable(e.CorrelationID), detail, nullable(e.UserID), nullable(e.UserAgent)})
	}
	return rows
}
