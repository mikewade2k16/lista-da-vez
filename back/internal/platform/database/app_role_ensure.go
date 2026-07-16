package database

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

// EnsureAppRoleResult descreve o resultado da auto-provisão da role de app.
type EnsureAppRoleResult struct {
	Created    bool   // role não existia e foi criada nesta execução
	Synced     bool   // senha/atributos/CONNECT convergidos
	SkipReason string // "empty_url" | "same_role" | "empty_password" | "" (não pulou)
}

// EnsureAppRole garante que a role de runtime da app (AC-04b) exista com a
// senha e os atributos esperados, extraídos de DATABASE_APP_URL. Roda como a
// role privilegiada do migrate ANTES de SyncAppRoleGrants — juntas elas tornam
// o deploy self-healing (incidente 2026-07-03: role ausente = api em
// crash-loop 28P01). Não decide política de erro: skips voltam em SkipReason
// e o caller (cmd/migrate) decide se falha (production) ou segue (dev).
func EnsureAppRole(ctx context.Context, pool *pgxpool.Pool, appDatabaseURL string) (EnsureAppRoleResult, error) {
	if appDatabaseURL == "" {
		return EnsureAppRoleResult{SkipReason: "empty_url"}, nil
	}

	appCfg, err := pgxpool.ParseConfig(appDatabaseURL)
	if err != nil {
		return EnsureAppRoleResult{}, fmt.Errorf("parse app database url: %w", err)
	}
	roleName := appCfg.ConnConfig.User
	password := appCfg.ConnConfig.Password

	// App e migrate na mesma role (dev local sem role dedicada): nada a criar.
	if roleName == pool.Config().ConnConfig.User {
		return EnsureAppRoleResult{SkipReason: "same_role"}, nil
	}
	if !roleNamePattern.MatchString(roleName) {
		return EnsureAppRoleResult{}, fmt.Errorf("app role name invalido: %q", roleName)
	}
	if password == "" {
		return EnsureAppRoleResult{SkipReason: "empty_password"}, nil
	}

	tx, err := pool.Begin(ctx)
	if err != nil {
		return EnsureAppRoleResult{}, fmt.Errorf("begin ensure app role: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// A senha entra no DDL montado (CREATE/ALTER ROLE não aceita bind param);
	// silenciar o statement log desta tx para ela não vazar no log do Postgres.
	if _, err := tx.Exec(ctx, "set local log_statement = 'none'"); err != nil {
		return EnsureAppRoleResult{}, fmt.Errorf("silence statement log: %w", err)
	}

	var exists bool
	if err := tx.QueryRow(ctx,
		`select exists(select 1 from pg_roles where rolname = $1)`, roleName,
	).Scan(&exists); err != nil {
		return EnsureAppRoleResult{}, fmt.Errorf("check app role exists: %w", err)
	}

	// Quoting seguro delegado ao próprio Postgres via format(%I/%L) — mesmo
	// padrão do scripts/db/create-app-role.sql, sem montar literal em Go.
	var ddl string
	if !exists {
		if err := tx.QueryRow(ctx,
			`select format('create role %I login', $1::text)`, roleName,
		).Scan(&ddl); err != nil {
			return EnsureAppRoleResult{}, fmt.Errorf("format create role: %w", err)
		}
		if _, err := tx.Exec(ctx, ddl); err != nil {
			return EnsureAppRoleResult{}, fmt.Errorf("create app role: %w", err)
		}
	}

	// SEMPRE convergir senha + atributos (cura rotação de APP_DB_ROLE_PASSWORD
	// e role pré-existente com atributos errados). Idempotente, 1x por boot.
	if err := tx.QueryRow(ctx,
		`select format('alter role %I with login password %L nosuperuser nocreatedb nocreaterole nobypassrls noreplication', $1::text, $2::text)`,
		roleName, password,
	).Scan(&ddl); err != nil {
		return EnsureAppRoleResult{}, fmt.Errorf("format alter role: %w", err)
	}
	if _, err := tx.Exec(ctx, ddl); err != nil {
		return EnsureAppRoleResult{}, fmt.Errorf("alter app role: %w", err)
	}

	if err := tx.QueryRow(ctx,
		`select format('grant connect on database %I to %I', current_database(), $1::text)`,
		roleName,
	).Scan(&ddl); err != nil {
		return EnsureAppRoleResult{}, fmt.Errorf("format grant connect: %w", err)
	}
	if _, err := tx.Exec(ctx, ddl); err != nil {
		return EnsureAppRoleResult{}, fmt.Errorf("grant connect: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return EnsureAppRoleResult{}, fmt.Errorf("commit ensure app role: %w", err)
	}

	return EnsureAppRoleResult{Created: !exists, Synced: true}, nil
}
