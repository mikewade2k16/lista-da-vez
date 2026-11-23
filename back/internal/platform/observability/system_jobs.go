package observability

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"
)

const (
	jobRediscoverEvery = 10 * time.Minute
	jobQueryTimeout    = 3 * time.Second
)

var identPattern = regexp.MustCompile(`^[a-z0-9_]+$`)

type jobMetrics struct {
	Table               string  `json:"table"`
	Pending             int64   `json:"pending"`
	Due                 int64   `json:"due"`
	OldestDueLagSeconds float64 `json:"oldestDueLagSeconds"`
	Processing          int64   `json:"processing"`
	StuckProcessing     int64   `json:"stuckProcessing"`
	Dead                int64   `json:"dead"`
	DeadLastHour        int64   `json:"deadLastHour"`
}

// jobTable é uma fila descoberta: só identificadores já validados.
type jobTable struct {
	schema, name string
	hasLockedAt  bool
	hasUpdatedAt bool
}

func (t jobTable) label() string { return t.schema + "." + t.name }

type jobDiscovery struct {
	tables     []jobTable
	discovered time.Time
}

const jobDiscoverySQL = `
select table_schema, table_name,
       bool_or(column_name = 'locked_at'), bool_or(column_name = 'updated_at')
  from information_schema.columns
 where table_schema not in ('pg_catalog', 'information_schema', 'obs')
   and column_name in ('status', 'run_after', 'locked_at', 'updated_at')
 group by table_schema, table_name
having bool_or(column_name = 'status') and bool_or(column_name = 'run_after')
 order by table_schema, table_name`

func validIdent(value string) bool { return len(value) <= 63 && identPattern.MatchString(value) }

func quoteIdent(value string) string { return `"` + strings.ReplaceAll(value, `"`, `""`) + `"` }

// buildJobSQL monta a consulta de uma fila com identificadores quotados.
func buildJobSQL(t jobTable) (string, error) {
	if !validIdent(t.schema) || !validIdent(t.name) {
		return "", fmt.Errorf("identificador de fila invalido: %q.%q", t.schema, t.name)
	}
	stuck, deadHour := "0", "0"
	if t.hasLockedAt {
		stuck = `count(*) filter (where status = 'processing' and locked_at < now() - interval '10 minutes')`
	}
	if t.hasUpdatedAt {
		deadHour = `count(*) filter (where status = 'dead' and updated_at > now() - interval '1 hour')`
	}
	return `select count(*) filter (where status = 'pending')::bigint,
       count(*) filter (where status = 'pending' and run_after <= now())::bigint,
       coalesce(extract(epoch from now() - min(run_after) filter (where status = 'pending' and run_after <= now())), 0)::float8,
       count(*) filter (where status = 'processing')::bigint,
       ` + stuck + `::bigint,
       count(*) filter (where status = 'dead')::bigint,
       ` + deadHour + `::bigint
  from ` + quoteIdent(t.schema) + `.` + quoteIdent(t.name), nil
}

func (c *systemCollector) discoverJobs(ctx context.Context) {
	if time.Since(c.jobs.discovered) < jobRediscoverEvery && !c.jobs.discovered.IsZero() {
		return
	}
	rows, err := c.pool.Query(ctx, jobDiscoverySQL)
	if err != nil {
		c.warn("observability_job_discovery_failed", err)
		return
	}
	defer rows.Close()
	var found []jobTable
	for rows.Next() {
		var t jobTable
		if err := rows.Scan(&t.schema, &t.name, &t.hasLockedAt, &t.hasUpdatedAt); err != nil {
			c.warn("observability_job_discovery_failed", err)
			return
		}
		if validIdent(t.schema) && validIdent(t.name) {
			found = append(found, t)
		}
	}
	if rows.Err() != nil {
		return
	}
	c.jobs.tables = found
	c.jobs.discovered = time.Now()
}

func (c *systemCollector) readJobs(ctx context.Context) []jobMetrics {
	c.discoverJobs(ctx)
	out := make([]jobMetrics, 0, len(c.jobs.tables))
	for _, table := range c.jobs.tables {
		statement, err := buildJobSQL(table)
		if err != nil {
			c.warn("observability_job_sql_invalid", err)
			continue
		}
		metrics := jobMetrics{Table: table.label()}
		queryCtx, cancel := context.WithTimeout(ctx, jobQueryTimeout)
		err = c.pool.QueryRow(queryCtx, statement).Scan(&metrics.Pending, &metrics.Due, &metrics.OldestDueLagSeconds,
			&metrics.Processing, &metrics.StuckProcessing, &metrics.Dead, &metrics.DeadLastHour)
		cancel()
		if err != nil {
			c.warn("observability_job_table_failed", fmt.Errorf("%s: %w", table.label(), err))
			continue
		}
		out = append(out, metrics)
	}
	return out
}
