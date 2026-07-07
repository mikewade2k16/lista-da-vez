package database_test

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/database"
)

const (
	ac04TestRole     = "ac04_test_app"
	ac04TestPassword = "ac04testpw"
)

// TestAppRoleGrantsLeastPrivilege prova que a role de runtime (AC-04) recebe
// DML nos schemas de aplicacao mas NAO ganha DDL (create table/schema).
//
// Para rodar:
//
//	TEST_DATABASE_URL="postgres://omni:omni_dev@localhost:5432/omni?sslmode=disable" \
//	  go test ./internal/platform/database/ -run TestAppRoleGrantsLeastPrivilege -v
func TestAppRoleGrantsLeastPrivilege(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL não definido — pulando teste de integração da role de app")
	}

	ctx := context.Background()

	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("conectar ao banco de teste: %v", err)
	}
	defer pool.Close()

	// Reproduz o boot: aplica todas as migrations (idempotente).
	if err := database.ApplyMigrationsWithOptions(ctx, pool, database.MigrationOptions{
		SkipDataSeeds: true,
	}); err != nil {
		t.Fatalf("migrations falharam: %v", err)
	}

	dropTestRole(ctx, t, pool)
	if _, err := pool.Exec(ctx, fmt.Sprintf(
		"create role %s login password '%s'", ac04TestRole, ac04TestPassword,
	)); err != nil {
		t.Fatalf("criar role de teste: %v", err)
	}
	t.Cleanup(func() { dropTestRole(context.Background(), t, pool) })

	appURL := replaceUserInfo(t, dsn, ac04TestRole, ac04TestPassword)

	granted, err := database.SyncAppRoleGrants(ctx, pool, appURL)
	if err != nil {
		t.Fatalf("SyncAppRoleGrants: %v", err)
	}
	if !granted {
		t.Fatal("SyncAppRoleGrants retornou granted=false; esperava true (role existe e difere do pool)")
	}

	// Conecta COMO a role de app e verifica DML permitido + DDL negado.
	appConn, err := pgx.Connect(ctx, appURL)
	if err != nil {
		t.Fatalf("conectar como %s: %v", ac04TestRole, err)
	}
	defer appConn.Close(ctx)

	// POSITIVO: DML numa tabela real do schema core.
	var count int
	if err := appConn.QueryRow(ctx, "select count(*) from core.users").Scan(&count); err != nil {
		t.Fatalf("DML positivo (select core.users) falhou: %v", err)
	}

	// NEGATIVO: create table em public deve retornar insufficient_privilege (42501).
	assertPermissionDenied(ctx, t, appConn, "create table public.ac04_should_fail(x int)")
	// NEGATIVO: create schema deve retornar insufficient_privilege (42501).
	assertPermissionDenied(ctx, t, appConn, "create schema ac04_hack")

	// Idempotencia: rodar de novo continua (true, nil).
	granted, err = database.SyncAppRoleGrants(ctx, pool, appURL)
	if err != nil {
		t.Fatalf("SyncAppRoleGrants (2a chamada): %v", err)
	}
	if !granted {
		t.Fatal("SyncAppRoleGrants (2a chamada) retornou granted=false")
	}
}

func assertPermissionDenied(ctx context.Context, t *testing.T, conn *pgx.Conn, stmt string) {
	t.Helper()
	_, err := conn.Exec(ctx, stmt)
	if err == nil {
		t.Fatalf("esperava permission denied (42501) para %q, mas o comando teve sucesso", stmt)
	}
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		t.Fatalf("esperava *pgconn.PgError para %q, veio: %v", stmt, err)
	}
	if pgErr.Code != "42501" {
		t.Fatalf("esperava SQLSTATE 42501 para %q, veio %s (%s)", stmt, pgErr.Code, pgErr.Message)
	}
}

func dropTestRole(ctx context.Context, t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	if _, err := pool.Exec(ctx, fmt.Sprintf("drop owned by %s cascade", ac04TestRole)); err != nil {
		var pgErr *pgconn.PgError
		// 42704 = undefined_object (role nao existe ainda) — ok no primeiro run.
		if !errors.As(err, &pgErr) || pgErr.Code != "42704" {
			t.Logf("drop owned by %s: %v", ac04TestRole, err)
		}
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf("drop role if exists %s", ac04TestRole)); err != nil {
		t.Logf("drop role %s: %v", ac04TestRole, err)
	}
}

// replaceUserInfo troca usuario/senha da DSN preservando host, porta, db e query.
func replaceUserInfo(t *testing.T, dsn, user, password string) string {
	t.Helper()
	parsed, err := url.Parse(dsn)
	if err != nil {
		t.Fatalf("parse TEST_DATABASE_URL: %v", err)
	}
	parsed.User = url.UserPassword(user, password)
	return parsed.String()
}
