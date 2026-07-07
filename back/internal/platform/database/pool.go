package database

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/config"
)

// OpenPool abre o pool privilegiado (DATABASE_URL). Usado pelo binario migrate
// (roda DDL das migrations) e por qualquer caller que precise da role padrao.
func OpenPool(ctx context.Context, cfg config.Config) (*pgxpool.Pool, error) {
	return openPoolWithURL(ctx, cfg, cfg.DatabaseURL)
}

// OpenAppPool abre o pool de RUNTIME da api com DATABASE_APP_URL (role
// least-privilege omni_app, AC-04). Fallback para DATABASE_URL quando a app
// URL nao esta definida (dev local sem a role — em production o Validate() da
// config impede esse fallback).
func OpenAppPool(ctx context.Context, cfg config.Config) (*pgxpool.Pool, error) {
	url := cfg.DatabaseAppURL
	if url == "" {
		url = cfg.DatabaseURL
	}
	return openPoolWithURL(ctx, cfg, url)
}

func openPoolWithURL(ctx context.Context, cfg config.Config, url string) (*pgxpool.Pool, error) {
	if url == "" {
		return nil, errors.New("database_url is required")
	}

	poolConfig, err := pgxpool.ParseConfig(url)
	if err != nil {
		return nil, fmt.Errorf("parse database config: %w", err)
	}

	if cfg.DatabaseMinConns > 0 {
		poolConfig.MinConns = int32(cfg.DatabaseMinConns)
	}

	if cfg.DatabaseMaxConns > 0 {
		poolConfig.MaxConns = int32(cfg.DatabaseMaxConns)
	}

	poolConfig.MaxConnIdleTime = 5 * time.Minute
	poolConfig.MaxConnLifetime = 30 * time.Minute
	poolConfig.HealthCheckPeriod = 30 * time.Second

	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		return nil, fmt.Errorf("open database pool: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}

	return pool, nil
}
