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
	ensureTestRole  = "ac04b_ensure_app"
	ensureTestPass  = "ac04bensurepw"
	ensureTestPass2 = "ac04brotatedpw"
)

// TestEnsureAppRole cobre a auto-provisão da role de runtime (AC-04b): criação,
// idempotência, rotação de senha, os três skips e a rejeição de nome inválido.
//
// Para rodar:
//
//	TEST_DATABASE_URL="postgres://omni:omni_dev@localhost:5432/omni?sslmode=disable" \
//	  go test ./internal/platform/database/ -run TestEnsureAppRole -v
func TestEnsureAppRole(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL não definido — pulando teste de integração da auto-provisão da role")
	}

	ctx := context.Background()

	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("conectar ao banco de teste: %v", err)
	}
	defer pool.Close()

	dropEnsureTestRole(ctx, t, pool)
	t.Cleanup(func() { dropEnsureTestRole(context.Background(), t, pool) })

	// 1. Cria role nova: URL com role inexistente + senha.
	appURL := replaceUserInfo(t, dsn, ensureTestRole, ensureTestPass)
	res, err := database.EnsureAppRole(ctx, pool, appURL)
	if err != nil {
		t.Fatalf("EnsureAppRole (criação): %v", err)
	}
	if !res.Created || !res.Synced || res.SkipReason != "" {
		t.Fatalf("esperava Created=true Synced=true SkipReason=\"\"; veio %+v", res)
	}
	// Conectar COMO a role recém-criada deve funcionar.
	assertConnects(ctx, t, appURL)
	assertRoleAttributes(ctx, t, pool)

	// 2. Idempotência: 2ª chamada não recria.
	res, err = database.EnsureAppRole(ctx, pool, appURL)
	if err != nil {
		t.Fatalf("EnsureAppRole (idempotência): %v", err)
	}
	if res.Created || !res.Synced || res.SkipReason != "" {
		t.Fatalf("2ª chamada: esperava Created=false Synced=true SkipReason=\"\"; veio %+v", res)
	}

	// 3. Rotação de senha: mesma role, senha nova na URL.
	rotatedURL := replaceUserInfo(t, dsn, ensureTestRole, ensureTestPass2)
	res, err = database.EnsureAppRole(ctx, pool, rotatedURL)
	if err != nil {
		t.Fatalf("EnsureAppRole (rotação): %v", err)
	}
	if res.Created || !res.Synced {
		t.Fatalf("rotação: esperava Created=false Synced=true; veio %+v", res)
	}
	assertConnects(ctx, t, rotatedURL) // senha nova funciona
	assertConnectFails(ctx, t, appURL) // senha antiga falha

	// 4. Skips não tocam DDL: pega a contagem de roles antes e confere que não muda.
	before := countRoles(ctx, t, pool)

	if res, err = database.EnsureAppRole(ctx, pool, ""); err != nil {
		t.Fatalf("EnsureAppRole (url vazia): %v", err)
	}
	if res.SkipReason != "empty_url" {
		t.Fatalf("url vazia: esperava SkipReason=empty_url; veio %+v", res)
	}

	// URL com a MESMA role do pool → same_role.
	poolRole := pool.Config().ConnConfig.User
	sameRoleURL := replaceUserInfo(t, dsn, poolRole, ensureTestPass)
	if res, err = database.EnsureAppRole(ctx, pool, sameRoleURL); err != nil {
		t.Fatalf("EnsureAppRole (mesma role): %v", err)
	}
	if res.SkipReason != "same_role" {
		t.Fatalf("mesma role: esperava SkipReason=same_role; veio %+v", res)
	}

	// URL sem senha → empty_password.
	noPassURL := userOnlyURL(t, dsn, ensureTestRole)
	if res, err = database.EnsureAppRole(ctx, pool, noPassURL); err != nil {
		t.Fatalf("EnsureAppRole (sem senha): %v", err)
	}
	if res.SkipReason != "empty_password" {
		t.Fatalf("sem senha: esperava SkipReason=empty_password; veio %+v", res)
	}

	if after := countRoles(ctx, t, pool); after != before {
		t.Fatalf("skips não deviam criar role: pg_roles foi de %d para %d", before, after)
	}

	// 5. Nome inválido rejeitado antes de qualquer DDL.
	evilURL := replaceUserInfo(t, dsn, "evil\"role", ensureTestPass)
	if _, err = database.EnsureAppRole(ctx, pool, evilURL); err == nil {
		t.Fatal("nome inválido: esperava erro (roleNamePattern), veio nil")
	}
}

func assertConnects(ctx context.Context, t *testing.T, url string) {
	t.Helper()
	conn, err := pgx.Connect(ctx, url)
	if err != nil {
		t.Fatalf("conectar com %s deveria funcionar: %v", url, err)
	}
	_ = conn.Close(ctx)
}

func assertConnectFails(ctx context.Context, t *testing.T, url string) {
	t.Helper()
	conn, err := pgx.Connect(ctx, url)
	if err == nil {
		_ = conn.Close(ctx)
		t.Fatalf("conexão com senha antiga deveria falhar, mas funcionou (%s)", url)
	}
}

func assertRoleAttributes(ctx context.Context, t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	var canLogin, super, bypassRLS, createDB, createRole, replication bool
	if err := pool.QueryRow(ctx,
		`select rolcanlogin, rolsuper, rolbypassrls, rolcreatedb, rolcreaterole, rolreplication
		 from pg_roles where rolname = $1`, ensureTestRole,
	).Scan(&canLogin, &super, &bypassRLS, &createDB, &createRole, &replication); err != nil {
		t.Fatalf("ler atributos da role: %v", err)
	}
	if !canLogin || super || bypassRLS || createDB || createRole || replication {
		t.Fatalf("atributos da role fora do least-privilege: login=%v super=%v bypassrls=%v createdb=%v createrole=%v replication=%v",
			canLogin, super, bypassRLS, createDB, createRole, replication)
	}
}

func countRoles(ctx context.Context, t *testing.T, pool *pgxpool.Pool) int {
	t.Helper()
	var n int
	if err := pool.QueryRow(ctx, `select count(*) from pg_roles`).Scan(&n); err != nil {
		t.Fatalf("contar roles: %v", err)
	}
	return n
}

// userOnlyURL troca o usuário da DSN e REMOVE a senha (para exercitar o skip
// empty_password), preservando host, porta, db e query.
func userOnlyURL(t *testing.T, dsn, user string) string {
	t.Helper()
	parsed, err := url.Parse(dsn)
	if err != nil {
		t.Fatalf("parse TEST_DATABASE_URL: %v", err)
	}
	parsed.User = url.User(user)
	return parsed.String()
}

func dropEnsureTestRole(ctx context.Context, t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	if _, err := pool.Exec(ctx, fmt.Sprintf("drop owned by %s cascade", ensureTestRole)); err != nil {
		var pgErr *pgconn.PgError
		// 42704 = undefined_object (role ainda não existe) — ok no primeiro run.
		if !errors.As(err, &pgErr) || pgErr.Code != "42704" {
			t.Logf("drop owned by %s: %v", ensureTestRole, err)
		}
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf("drop role if exists %s", ensureTestRole)); err != nil {
		t.Logf("drop role %s: %v", ensureTestRole, err)
	}
}
