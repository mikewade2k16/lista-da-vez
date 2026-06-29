package database

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/config"
)

// Querier e a interface minima de acesso ao banco que tanto *pgxpool.Pool quanto
// *pgx.Conn satisfazem. Os repositorios passam a depender dela (em vez do pool
// concreto) para que, sob RLS, a query rode na MESMA conexao em que o middleware
// setou o GUC app.account_id da request (ver ConnFromContext).
type Querier interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

// connContextKey e a chave (privada ao pacote) sob a qual o middleware de RLS
// guarda a *pgxpool.Conn da request no context.Context.
type connContextKey struct{}

// WithConn devolve um context derivado carregando a conexao da request. Usado
// pelo middleware de RLS apos o pool.Acquire e o set_config do GUC.
func WithConn(ctx context.Context, conn Querier) context.Context {
	return context.WithValue(ctx, connContextKey{}, conn)
}

// ConnFromContext resolve o Querier a ser usado pela request: a conexao posta no
// context pelo middleware de RLS quando presente, senao o pool (fallback). O
// fallback cobre jobs, boot, testes e rotas ainda sem o middleware — onde o RLS
// nao esta ativo e o pool direto e o comportamento legado correto.
func ConnFromContext(ctx context.Context, pool Querier) Querier {
	if conn, ok := ctx.Value(connContextKey{}).(Querier); ok && conn != nil {
		return conn
	}
	return pool
}

func OpenPool(ctx context.Context, cfg config.Config) (*pgxpool.Pool, error) {
	if cfg.DatabaseURL == "" {
		return nil, errors.New("database_url is required")
	}

	poolConfig, err := pgxpool.ParseConfig(cfg.DatabaseURL)
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
