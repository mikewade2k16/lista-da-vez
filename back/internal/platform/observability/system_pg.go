package observability

import (
	"context"
	"fmt"
	"strconv"
)

type pgMetrics struct {
	Connections              int     `json:"connections"`
	MaxConnections           int     `json:"maxConnections"`
	Active                   int     `json:"active"`
	IdleInTransaction        int     `json:"idleInTransaction"`
	IdleInTransactionOver60s int     `json:"idleInTransactionOver60s"`
	WaitingOnLock            int     `json:"waitingOnLock"`
	LongestQuerySeconds      float64 `json:"longestQuerySeconds"`
	LongestQuery             string  `json:"longestQuery"`
	DeadlocksDelta           int64   `json:"deadlocksDelta"`
	CacheHitRatio            float64 `json:"cacheHitRatio"`
	DBSizeBytes              int64   `json:"dbSizeBytes"`
}

type pgCounters struct {
	deadlocks int64
	valid     bool
}

const pgActivitySQL = `
select count(*)::int,
       count(*) filter (where state = 'active')::int,
       count(*) filter (where state like 'idle in transaction%')::int,
       count(*) filter (where state like 'idle in transaction%' and state_change < now() - interval '60 seconds')::int,
       count(*) filter (where wait_event_type = 'Lock')::int
  from pg_catalog.pg_stat_activity
 where datname = current_database() and pid <> pg_backend_pid()`

const pgLongestSQL = `
select greatest(coalesce(extract(epoch from clock_timestamp() - query_start), 0), 0)::float8, query
  from pg_catalog.pg_stat_activity
 where datname = current_database() and pid <> pg_backend_pid()
   and state = 'active' and query_start is not null and coalesce(query, '') <> ''
 order by query_start asc
 limit 1`

const pgDatabaseSQL = `
select coalesce(deadlocks, 0)::bigint, coalesce(blks_hit, 0)::bigint, coalesce(blks_read, 0)::bigint,
       pg_catalog.pg_database_size(datname)::bigint
  from pg_catalog.pg_stat_database
 where datname = current_database()`

func (c *systemCollector) readPG(ctx context.Context) (*pgMetrics, pgCounters, error) {
	out := &pgMetrics{}
	err := c.pool.QueryRow(ctx, pgActivitySQL).Scan(&out.Connections, &out.Active, &out.IdleInTransaction, &out.IdleInTransactionOver60s, &out.WaitingOnLock)
	if err != nil {
		return nil, pgCounters{}, fmt.Errorf("pg_stat_activity: %w", err)
	}
	var query string
	rows, err := c.pool.Query(ctx, pgLongestSQL)
	if err != nil {
		return nil, pgCounters{}, fmt.Errorf("longest query: %w", err)
	}
	if rows.Next() {
		if err := rows.Scan(&out.LongestQuerySeconds, &query); err == nil {
			out.LongestQuery = SanitizeText(c.norm.Normalize(query).Statement, 500)
		}
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, pgCounters{}, fmt.Errorf("longest query: %w", err)
	}
	var deadlocks, hit, read int64
	if err := c.pool.QueryRow(ctx, pgDatabaseSQL).Scan(&deadlocks, &hit, &read, &out.DBSizeBytes); err != nil {
		return nil, pgCounters{}, fmt.Errorf("pg_stat_database: %w", err)
	}
	out.CacheHitRatio = cacheHitRatio(hit, read)
	out.MaxConnections = c.maxConnections(ctx)
	return out, pgCounters{deadlocks: deadlocks, valid: true}, nil
}

func (c *systemCollector) maxConnections(ctx context.Context) int {
	if c.maxConns > 0 {
		return c.maxConns
	}
	var raw string
	if err := c.pool.QueryRow(ctx, `show max_connections`).Scan(&raw); err != nil {
		return 0
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		return 0
	}
	c.maxConns = value
	return value
}

func cacheHitRatio(hit, read int64) float64 {
	if hit+read <= 0 {
		return 0
	}
	return float64(hit) / float64(hit+read)
}
