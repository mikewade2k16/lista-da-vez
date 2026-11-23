package observability

import (
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

const collectTimeout = 8 * time.Second

// systemMetrics é o JSON de obs.system_samples.metrics (source = 'api').
// Parte que falhou fica nil e some do JSON.
type systemMetrics struct {
	Go   *goMetrics   `json:"go,omitempty"`
	Pool *poolMetrics `json:"pool,omitempty"`
	PG   *pgMetrics   `json:"pg,omitempty"`
	Jobs []jobMetrics `json:"jobs"`
}

type systemCollector struct {
	pool     *pgxpool.Pool
	cfg      Config
	logger   *slog.Logger
	norm     *sqlNormalizer
	host     string
	prevPool poolCounters
	prevPG   pgCounters
	hasPrev  bool
	maxConns int
	jobs     *jobDiscovery
	warns    warnLimiter
}

func (rt *Runtime) startSystemCollector(ctx context.Context, pool *pgxpool.Pool) {
	host, _ := os.Hostname()
	collector := &systemCollector{
		pool:   pool,
		cfg:    rt.cfg,
		logger: rt.logger,
		norm:   rt.normalizer,
		host:   host,
		jobs:   &jobDiscovery{},
	}
	rt.collector = make(chan struct{})
	go func() {
		defer close(rt.collector)
		collector.run(ctx)
	}()
}

func (c *systemCollector) run(ctx context.Context) {
	ticker := time.NewTicker(c.cfg.SystemInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			c.safeCollect(ctx)
		}
	}
}

func (c *systemCollector) safeCollect(ctx context.Context) {
	defer func() {
		if recovered := recover(); recovered != nil {
			c.logger.Error("observability_system_panic", slog.Any("panic", recovered))
		}
	}()
	runCtx, cancel := context.WithTimeout(WithoutTracing(ctx), collectTimeout)
	defer cancel()
	metrics := c.collect(runCtx)
	encoded, err := json.Marshal(metrics)
	if err != nil {
		return
	}
	_, err = c.pool.Exec(runCtx, `insert into obs.system_samples (source, host, metrics) values ('api', $1, $2::jsonb)`, c.host, string(encoded))
	if err != nil && c.warns.allow(time.Now()) {
		c.logger.Warn("observability_system_write_failed", slog.String("error", SanitizeText(err.Error(), 300)))
	}
}

func (c *systemCollector) collect(ctx context.Context) systemMetrics {
	out := systemMetrics{Jobs: []jobMetrics{}}
	goStats := readGoMetrics()
	out.Go = &goStats
	poolCur := readPoolCounters(c.pool)
	out.Pool = buildPoolMetrics(poolCur, c.prevPool, c.hasPrev)
	pgStats, pgCur, err := c.readPG(ctx)
	if err != nil {
		c.warn("observability_system_pg_failed", err)
	} else {
		out.PG = pgStats
		pgStats.DeadlocksDelta = counterDelta(pgCur.deadlocks, c.prevPG.deadlocks, c.hasPrev && c.prevPG.valid)
		c.prevPG = pgCur
	}
	c.prevPool = poolCur
	c.hasPrev = true
	out.Jobs = c.readJobs(ctx)
	return out
}

func (c *systemCollector) warn(message string, err error) {
	if c.warns.allow(time.Now()) {
		c.logger.Warn(message, slog.String("error", SanitizeText(err.Error(), 300)))
	}
}

// warnLimiter evita inundar o log quando o banco está fora: 1 aviso por minuto.
type warnLimiter struct{ last time.Time }

func (w *warnLimiter) allow(now time.Time) bool {
	if now.Sub(w.last) < time.Minute {
		return false
	}
	w.last = now
	return true
}

// counterDelta é a diferença para a amostra anterior; sem anterior (ou contador
// reiniciado) devolve 0.
func counterDelta(current, previous int64, hasPrev bool) int64 {
	if !hasPrev || current < previous {
		return 0
	}
	return current - previous
}
