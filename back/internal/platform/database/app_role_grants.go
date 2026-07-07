package database

import (
	"context"
	"fmt"
	"regexp"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// roleNamePattern valida o nome da role de app antes de interpola-lo em DDL.
// Sem uuid lib e sem identifier injection: so minusculas, digitos e underscore,
// comecando por letra/underscore (mesmo conjunto aceito por identificadores
// nao-quoted do Postgres). Defesa em profundidade — o nome ainda passa por
// pgx.Identifier.Sanitize() antes de entrar no SQL.
var roleNamePattern = regexp.MustCompile(`^[a-z_][a-z0-9_]*$`)

// SyncAppRoleGrants garante os GRANTs de runtime da role da app (AC-04):
// USAGE nos schemas, SELECT/INSERT/UPDATE/DELETE em tabelas/views,
// USAGE/SELECT/UPDATE em sequences, e ALTER DEFAULT PRIVILEGES global para
// objetos futuros criados pela role de migration. Retorna (false, nil) quando
// nao ha o que fazer: appDatabaseURL vazia, mesma role do pool, ou role
// inexistente no cluster (o caller loga e segue).
//
// Roda como a role privilegiada (pool do migrate). Nao concede CREATE,
// TRUNCATE, REFERENCES, TRIGGER nem EXECUTE extra.
func SyncAppRoleGrants(ctx context.Context, pool *pgxpool.Pool, appDatabaseURL string) (bool, error) {
	if appDatabaseURL == "" {
		return false, nil
	}

	appCfg, err := pgxpool.ParseConfig(appDatabaseURL)
	if err != nil {
		return false, fmt.Errorf("parse app database url: %w", err)
	}
	roleName := appCfg.ConnConfig.User

	// App e migrate na mesma role (dev local sem role dedicada): nada a conceder.
	if roleName == pool.Config().ConnConfig.User {
		return false, nil
	}

	if !roleNamePattern.MatchString(roleName) {
		return false, fmt.Errorf("app role name invalido: %q", roleName)
	}

	var roleExists bool
	if err := pool.QueryRow(ctx,
		`select exists(select 1 from pg_roles where rolname = $1)`, roleName,
	).Scan(&roleExists); err != nil {
		return false, fmt.Errorf("check app role exists: %w", err)
	}
	if !roleExists {
		return false, nil
	}

	schemas, err := listUserSchemas(ctx, pool)
	if err != nil {
		return false, err
	}

	roleIdent := pgx.Identifier{roleName}.Sanitize()
	for _, schema := range schemas {
		schemaIdent := pgx.Identifier{schema}.Sanitize()
		stmts := []string{
			fmt.Sprintf("grant usage on schema %s to %s", schemaIdent, roleIdent),
			fmt.Sprintf("grant select, insert, update, delete on all tables in schema %s to %s", schemaIdent, roleIdent),
			fmt.Sprintf("grant usage, select, update on all sequences in schema %s to %s", schemaIdent, roleIdent),
		}
		for _, stmt := range stmts {
			if _, err := pool.Exec(ctx, stmt); err != nil {
				return false, fmt.Errorf("grant on schema %s: %w", schema, err)
			}
		}
	}

	// Default privileges GLOBAIS da role corrente (o migrator): cobrem
	// tabelas/sequences futuras em QUALQUER schema criado por migrations.
	// Sem FOR ROLE e sem IN SCHEMA de proposito: aplica ao current_user
	// (role do migrate) em todos os schemas.
	defaultStmts := []string{
		fmt.Sprintf("alter default privileges grant select, insert, update, delete on tables to %s", roleIdent),
		fmt.Sprintf("alter default privileges grant usage, select, update on sequences to %s", roleIdent),
	}
	for _, stmt := range defaultStmts {
		if _, err := pool.Exec(ctx, stmt); err != nil {
			return false, fmt.Errorf("alter default privileges: %w", err)
		}
	}

	return true, nil
}

// listUserSchemas retorna os schemas de aplicacao (exclui information_schema e
// pg_*). Pega public, core, queue, tasks, notifications, roadmap, site,
// automation, bio, meta_ads, cardapio, calendar e qualquer schema futuro —
// nada hardcoded.
func listUserSchemas(ctx context.Context, pool *pgxpool.Pool) ([]string, error) {
	rows, err := pool.Query(ctx, `
		select nspname
		from pg_namespace
		where nspname not in ('information_schema')
		  and nspname not like 'pg\_%' escape '\'
		order by nspname`)
	if err != nil {
		return nil, fmt.Errorf("list schemas: %w", err)
	}
	defer rows.Close()

	var schemas []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, fmt.Errorf("scan schema: %w", err)
		}
		schemas = append(schemas, name)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate schemas: %w", err)
	}

	return schemas, nil
}
