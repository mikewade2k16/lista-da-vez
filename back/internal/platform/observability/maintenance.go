package observability

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Tabelas brutas sujeitas à retenção. Agregados (http_route_hourly,
// db_query_stats) e execuções do runner são mantidos.
var rawTables = []string{"http_requests", "db_slow_queries", "db_n_plus_one", "external_calls", "frontend_events"}

func runMaintenance(ctx context.Context, pool *pgxpool.Pool, cfg Config, logger *slog.Logger) {
	timer := time.NewTimer(time.Minute)
	defer timer.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
			deleted, err := purgeRawData(ctx, pool, cfg.RetentionDays)
			if err != nil {
				logger.Warn("observability_retention_failed", slog.String("error", SanitizeText(err.Error(), 300)))
			} else if deleted > 0 {
				logger.Info("observability_retention_completed", slog.Int64("deleted_rows", deleted), slog.Int("retention_days", cfg.RetentionDays))
			}
			timer.Reset(cfg.MaintenanceEvery)
		}
	}
}

// purgeRawData apaga em lotes para não segurar locks longos.
func purgeRawData(ctx context.Context, pool *pgxpool.Pool, retentionDays int) (int64, error) {
	cutoff := time.Now().UTC().AddDate(0, 0, -retentionDays)
	ctx = WithoutTracing(ctx)
	var total int64
	for _, table := range rawTables {
		statement := fmt.Sprintf(`delete from obs.%s where id in (select id from obs.%s where occurred_at < $1 limit 5000)`, table, table)
		for range 200 {
			tag, err := pool.Exec(ctx, statement, cutoff)
			if err != nil {
				return total, fmt.Errorf("purge obs.%s: %w", table, err)
			}
			total += tag.RowsAffected()
			if tag.RowsAffected() < 5000 {
				break
			}
		}
	}
	extra, err := purgeHealthData(ctx, pool, cutoff)
	return total + extra, err
}

// purgeHealthData limpa amostras de sistema/containers (retenção padrão) e a
// presença (last_seen com mais de 1 dia).
func purgeHealthData(ctx context.Context, pool *pgxpool.Pool, cutoff time.Time) (int64, error) {
	statements := []struct {
		sql string
		arg time.Time
	}{
		{`delete from obs.system_samples where id in (select id from obs.system_samples where occurred_at < $1 limit 5000)`, cutoff},
		{`delete from obs.container_samples where id in (select id from obs.container_samples where occurred_at < $1 limit 5000)`, cutoff},
		{`delete from obs.presence where last_seen < $1`, time.Now().UTC().Add(-24 * time.Hour)},
	}
	var total int64
	for _, item := range statements {
		tag, err := pool.Exec(ctx, item.sql, item.arg)
		if err != nil {
			return total, fmt.Errorf("purge obs health: %w", err)
		}
		total += tag.RowsAffected()
	}
	return total, nil
}
