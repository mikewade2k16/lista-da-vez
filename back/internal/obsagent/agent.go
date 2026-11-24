package obsagent

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Config vem do ambiente.
type Config struct {
	Enabled     bool
	Interval    time.Duration
	Host        string
	DiskPaths   string
	DatabaseURL string
	TLSHosts    []string
}

// ConfigFromEnv reaproveita as variaveis da API: OBSERVABILITY_ENABLED e a URL de
// runtime (DATABASE_APP_URL, least-privilege; fallback DATABASE_URL como a API em dev).
func ConfigFromEnv() Config {
	cfg := Config{
		Enabled:   envBool("OBSERVABILITY_ENABLED", true),
		Interval:  30 * time.Second,
		Host:      strings.TrimSpace(os.Getenv("OBS_AGENT_HOST")),
		DiskPaths: os.Getenv("OBS_AGENT_DISK_PATHS"),
	}
	cfg.TLSHosts = ParseTLSHosts(os.Getenv("OBS_AGENT_TLS_HOSTS"))
	if d, err := time.ParseDuration(strings.TrimSpace(os.Getenv("OBS_AGENT_INTERVAL"))); err == nil && d >= 5*time.Second {
		cfg.Interval = d
	}
	if cfg.Host == "" {
		cfg.Host, _ = os.Hostname()
	}
	cfg.DatabaseURL = strings.TrimSpace(os.Getenv("DATABASE_APP_URL"))
	if cfg.DatabaseURL == "" {
		cfg.DatabaseURL = strings.TrimSpace(os.Getenv("DATABASE_URL"))
	}
	return cfg
}

func envBool(key string, fallback bool) bool {
	v, err := strconv.ParseBool(strings.TrimSpace(os.Getenv(key)))
	if err != nil {
		return fallback
	}
	return v
}

// Run executa o laco ate ctx ser cancelado. save nil = nao grava (dry-run).
func Run(ctx context.Context, cfg Config, col *Collector, save func(context.Context, Snapshot) error, log *slog.Logger) {
	ticker := time.NewTicker(cfg.Interval)
	defer ticker.Stop()
	cycle := func() {
		defer func() {
			if r := recover(); r != nil {
				log.Error("obs-agent: panic no ciclo", "panic", r)
			}
		}()
		timeout := cfg.Interval * 2 / 3
		cctx, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()
		snap, err := col.Collect(cctx)
		if err != nil {
			log.Error("obs-agent: coleta falhou", "error", err.Error())
			return
		}
		if save == nil {
			return
		}
		if err := save(cctx, snap); err != nil {
			log.Error("obs-agent: gravacao falhou", "error", err.Error())
			return
		}
		log.Info("obs-agent: ciclo gravado", "containers", len(snap.Containers), "running", snap.System.Docker.Running)
	}
	cycle()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			cycle()
		}
	}
}

// OpenPool abre um pool minimo (1 conexao) com a role de runtime.
func OpenPool(ctx context.Context, url string) (*pgxpool.Pool, error) {
	if url == "" {
		return nil, errors.New("DATABASE_APP_URL/DATABASE_URL ausente")
	}
	pc, err := pgxpool.ParseConfig(url)
	if err != nil {
		return nil, errors.New("URL do banco invalida")
	}
	pc.MaxConns = 1
	pc.MinConns = 0
	pc.MaxConnLifetime = time.Hour
	pc.ConnConfig.RuntimeParams["application_name"] = "omni-obs-agent"
	return pgxpool.NewWithConfig(ctx, pc)
}
