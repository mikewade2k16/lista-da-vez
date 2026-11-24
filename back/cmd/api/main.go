package main

import (
	"context"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
	// tzdata embute a base de fusos no binario: LoadLocation("America/Sao_Paulo")
	// funciona mesmo em container sem /usr/share/zoneinfo (due date de task, C10).
	_ "time/tzdata"

	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/app"
	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/background"
	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/config"
	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/database"
	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/observability"
	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/server"
)

func main() {
	os.Exit(run())
}

// shutdownTimeout le SHUTDOWN_TIMEOUT (duration Go, ex.: 20s); invalido ou <=0 => padrao.
func shutdownTimeout() time.Duration {
	raw := strings.TrimSpace(os.Getenv("SHUTDOWN_TIMEOUT"))
	if raw == "" {
		return server.DefaultShutdownTimeout
	}
	d, err := time.ParseDuration(raw)
	if err != nil || d <= 0 {
		return server.DefaultShutdownTimeout
	}
	return d
}

// run devolve o exit code; os defers rodam antes do os.Exit de main.
func run() int {
	cfg := config.Load()
	// JSON por padrao (LOG_FORMAT=text volta ao formato chave=valor), com
	// correlation ID e sanitizacao de atributos.
	logger := observability.NewLogger(os.Stdout)
	obs := observability.New(observability.ConfigFromEnv(cfg.Env), logger)

	if err := cfg.Validate(); err != nil {
		logger.Error("config_invalid", slog.Any("error", err))
		return 1
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	pool, err := database.OpenAppPoolWithTracer(ctx, cfg, obs.QueryTracer())
	if err != nil {
		logger.Error("database_connect_failed", slog.Any("error", err))
		return 1
	}
	// O pool fecha por ultimo (depois de HTTP, workers e observabilidade).
	defer pool.Close()

	obs.Start(pool)
	obs.InstrumentDefaultTransport()

	// Loga a role efetiva de runtime (AC-04), nunca a senha. Em prod deve ser omni_app.
	logger.Info("database_connected", slog.String("db_user", pool.Config().ConnConfig.User))

	// Contexto raiz dos workers: independente do sinal, para que o HTTP drene primeiro
	// e so entao os workers sejam cancelados.
	workersCtx, cancelWorkers := context.WithCancel(context.Background())
	defer cancelWorkers()
	bg := background.New(logger)

	handler, err := app.BuildHTTPHandler(workersCtx, bg, cfg, logger, pool, obs)
	if err != nil {
		logger.Error("bootstrap_failed", slog.Any("error", err))
		cancelWorkers()
		stopCtx, stopCancel := context.WithTimeout(context.Background(), 5*time.Second)
		bg.Shutdown(stopCtx)
		obs.Stop(stopCtx)
		stopCancel()
		return 1
	}

	httpServer := server.New(cfg.HTTPAddr, handler)
	listener, err := (&net.ListenConfig{}).Listen(context.Background(), "tcp", cfg.HTTPAddr)
	if err != nil {
		logger.Error("server_listen_failed", slog.Any("error", err))
		cancelWorkers()
		bg.Shutdown(context.Background())
		return 1
	}

	sigCtx, stopSignals := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer stopSignals()

	logger.Info(
		"api_listening",
		slog.String("addr", cfg.HTTPAddr),
		slog.String("env", cfg.Env),
		slog.Int("http_rate_limit_requests", cfg.HTTPRateLimitRequests),
		slog.Duration("http_rate_limit_window", cfg.HTTPRateLimitWindow),
		slog.Bool("observability_enabled", obs.Enabled()),
	)

	timeout := shutdownTimeout()
	exitCode := 0
	timedOut, serveErr := server.Serve(sigCtx, httpServer, listener, timeout)
	if serveErr != nil {
		logger.Error("server_stopped", slog.Any("error", serveErr))
		exitCode = 1
	}

	// Desligamento ordenado: HTTP (ja drenado por Serve) -> workers/modulos ->
	// observabilidade -> pool (defer). Um unico prazo global para workers e obs.
	started := time.Now()
	logger.Info("shutdown_started",
		slog.Duration("timeout", timeout),
		slog.Bool("http_timed_out", timedOut),
	)
	cancelWorkers()
	stopCtx, stopCancel := context.WithTimeout(context.Background(), timeout)
	defer stopCancel()
	workersDone := bg.Shutdown(stopCtx)
	obs.Stop(stopCtx)
	logger.Info("shutdown_completed",
		slog.Duration("duration", time.Since(started)),
		slog.Bool("http_timed_out", timedOut),
		slog.Bool("workers_done", workersDone),
	)
	return exitCode
}
