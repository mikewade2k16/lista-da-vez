package observability

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// IntegrationStats agrega obs.external_calls por integração. Erro = timeout, falha
// de rede (status 0), 5xx ou mensagem de erro; 4xx sozinho não conta.
func (s *Store) IntegrationStats(ctx context.Context, days int) ([]integrationRow, error) {
	rows, err := s.pool.Query(quiet(ctx), `
		select integration, count(*), count(*) filter (where is_error),
		       coalesce(avg(duration_ms), 0), coalesce(percentile_cont(0.95) within group (order by duration_ms), 0),
		       max(occurred_at) filter (where is_error),
		       coalesce((array_agg(left(coalesce(nullif(error, ''), 'HTTP ' || status), 160) order by occurred_at desc)
		                 filter (where is_error))[1], '')
		from (select integration, occurred_at, duration_ms, status, error,
		             (timeout or status = 0 or status >= 500 or coalesce(error, '') <> '') as is_error
		      from obs.external_calls where occurred_at >= $1) c
		group by integration
		order by count(*) desc
		limit 100`, since(days))
	if err != nil {
		return nil, fmt.Errorf("query integrations: %w", err)
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (integrationRow, error) {
		var item integrationRow
		return item, row.Scan(&item.Name, &item.Calls, &item.Errors, &item.AvgMs, &item.P95Ms, &item.LastErrorAt, &item.LastError)
	})
}
