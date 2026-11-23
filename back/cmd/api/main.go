package main

import (
	"context"
	"log/slog"
	"os"
	"time"
	// tzdata embute a base de fusos no binario: LoadLocation("America/Sao_Paulo")
	// funciona mesmo em container sem /usr/share/zoneinfo (due date de task, C10).
	_ "time/tzdata"

	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/app"
	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/config"
	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/database"
	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/observability"
	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/server"
)

func main() {
	cfg := config.Load()
	// JSON por padrao (LOG_FORMAT=text volta ao formato chave=valor), com
	// correlation ID e sanitizacao de atributos.
	logger := observability.NewLogger(os.Stdout)
	obs := observability.New(observability.ConfigFromEnv(cfg.Env), logger)

	if err := cfg.Validate(); err != nil {
		logger.Error("config_invalid", slog.Any("error", err))
		os.Exit(1)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	pool, err := database.OpenAppPoolWithTracer(ctx, cfg, obs.QueryTracer())
	if err != nil {
		logger.Error("database_connect_failed", slog.Any("error", err))
		os.Exit(1)
	}
	defer pool.Close()

	obs.Start(pool)
	obs.InstrumentDefaultTransport()

	// Loga a role efetiva de runtime (AC-04), nunca a senha. Em prod deve ser omni_app.
	logger.Info("database_connected", slog.String("db_user", pool.Config().ConnConfig.User))

	handler, err := app.BuildHTTPHandler(cfg, logger, pool, obs)
	if err != nil {
		logger.Error("bootstrap_failed", slog.Any("error", err))
		os.Exit(1)
	}

	httpServer := server.New(cfg.HTTPAddr, handler)

	logger.Info(
		"api_listening",
		slog.String("addr", cfg.HTTPAddr),
		slog.String("env", cfg.Env),
		slog.Int("http_rate_limit_requests", cfg.HTTPRateLimitRequests),
		slog.Duration("http_rate_limit_window", cfg.HTTPRateLimitWindow),
		slog.Bool("observability_enabled", obs.Enabled()),
	)
	if err := httpServer.ListenAndServe(); err != nil {
		logger.Error("server_stopped", slog.Any("error", err))
		os.Exit(1)
	}
}
