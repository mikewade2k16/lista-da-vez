package main

import (
	"context"
	"log/slog"
	"os"
	"time"

	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/app"
	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/config"
	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/database"
	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/server"
)

func main() {
	cfg := config.Load()
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	pool, err := database.OpenPool(ctx, cfg)
	if err != nil {
		logger.Error("database_connect_failed", slog.Any("error", err))
		os.Exit(1)
	}
	defer pool.Close()

	handler, err := app.BuildHTTPHandler(cfg, logger, pool)
	if err != nil {
		logger.Error("bootstrap_failed", slog.Any("error", err))
		os.Exit(1)
	}

	httpServer := server.New(cfg.HTTPAddr, handler)

	logger.Info("api_listening", slog.String("addr", cfg.HTTPAddr), slog.String("env", cfg.Env))
	if err := httpServer.ListenAndServe(); err != nil {
		logger.Error("server_stopped", slog.Any("error", err))
		os.Exit(1)
	}
}
