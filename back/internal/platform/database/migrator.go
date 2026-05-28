package database

import (
	"context"
	"embed"
	"fmt"
	"io/fs"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed migrations/*.sql
var migrationFiles embed.FS

type Migration struct {
	Version string
	Name    string
	SQL     string
}

type AppliedMigration struct {
	Version   string
	Name      string
	AppliedAt time.Time
}

type MigrationOptions struct {
	SkipDataSeeds bool
}

var dataSeedMigrationNames = map[string]struct{}{
	"0002_seed_demo_auth.sql":                   {},
	"0004_seed_demo_consultants.sql":            {},
	"0015_mvp_access_foundation.sql":            {},
	"0018_seed_default_password_since1967.sql":  {},
	"0033_seed_dev_platform_admin_password.sql": {},
	"0036_seed_dev_erp_store_184.sql":           {},
}

func LoadMigrations() ([]Migration, error) {
	return LoadMigrationsWithOptions(MigrationOptions{})
}

func LoadMigrationsWithOptions(options MigrationOptions) ([]Migration, error) {
	fileNames, err := fs.Glob(migrationFiles, "migrations/*.sql")
	if err != nil {
		return nil, fmt.Errorf("glob migrations: %w", err)
	}

	sort.Strings(fileNames)

	migrations := make([]Migration, 0, len(fileNames))
	for _, fileName := range fileNames {
		baseName := strings.TrimPrefix(fileName, "migrations/")
		if options.SkipDataSeeds {
			if _, exists := dataSeedMigrationNames[baseName]; exists {
				continue
			}
		}

		content, err := migrationFiles.ReadFile(fileName)
		if err != nil {
			return nil, fmt.Errorf("read migration %s: %w", fileName, err)
		}

		migrations = append(migrations, Migration{
			Version: baseName,
			Name:    baseName,
			SQL:     string(content),
		})
	}

	return migrations, nil
}

func EnsureMigrationsTable(ctx context.Context, pool *pgxpool.Pool) error {
	_, err := pool.Exec(ctx, `
		create table if not exists schema_migrations (
			version text primary key,
			name text not null,
			applied_at timestamptz not null default now()
		);
	`)
	if err != nil {
		return fmt.Errorf("ensure schema_migrations: %w", err)
	}

	return nil
}

func ListAppliedMigrations(ctx context.Context, pool *pgxpool.Pool) ([]AppliedMigration, error) {
	if err := EnsureMigrationsTable(ctx, pool); err != nil {
		return nil, err
	}

	rows, err := pool.Query(ctx, `
		select version, name, applied_at
		from schema_migrations
		order by version asc
	`)
	if err != nil {
		return nil, fmt.Errorf("query schema_migrations: %w", err)
	}
	defer rows.Close()

	applied := make([]AppliedMigration, 0)
	for rows.Next() {
		var item AppliedMigration
		if err := rows.Scan(&item.Version, &item.Name, &item.AppliedAt); err != nil {
			return nil, fmt.Errorf("scan schema_migrations: %w", err)
		}

		applied = append(applied, item)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate schema_migrations: %w", err)
	}

	return applied, nil
}

func ApplyMigrations(ctx context.Context, pool *pgxpool.Pool) error {
	return ApplyMigrationsWithOptions(ctx, pool, MigrationOptions{})
}

func ApplyMigrationsWithOptions(ctx context.Context, pool *pgxpool.Pool, options MigrationOptions) error {
	if err := EnsureMigrationsTable(ctx, pool); err != nil {
		return err
	}

	migrations, err := LoadMigrationsWithOptions(options)
	if err != nil {
		return err
	}

	appliedList, err := ListAppliedMigrations(ctx, pool)
	if err != nil {
		return err
	}

	applied := make(map[string]struct{}, len(appliedList))
	for _, item := range appliedList {
		applied[item.Version] = struct{}{}
	}

	for _, migration := range migrations {
		if _, exists := applied[migration.Version]; exists {
			continue
		}

		tx, err := pool.Begin(ctx)
		if err != nil {
			return fmt.Errorf("begin migration tx %s: %w", migration.Name, err)
		}

		if _, err := tx.Exec(ctx, migration.SQL); err != nil {
			_ = tx.Rollback(ctx)
			return fmt.Errorf("apply migration %s: %w", migration.Name, err)
		}

		if _, err := tx.Exec(ctx, `
			insert into schema_migrations (version, name)
			values ($1, $2)
		`, migration.Version, migration.Name); err != nil {
			_ = tx.Rollback(ctx)
			return fmt.Errorf("record migration %s: %w", migration.Name, err)
		}

		if err := tx.Commit(ctx); err != nil {
			return fmt.Errorf("commit migration %s: %w", migration.Name, err)
		}
	}

	return nil
}
