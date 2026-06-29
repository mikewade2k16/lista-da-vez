package feedback

import (
	"context"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/database"
)

// TestFeedbackRLSIsolation prova a fase 1 do Row-Level Security (docs/RLS_PLAN.md):
// com app.account_id setado para a conta A, um SELECT SEM filtro de tenant na
// aplicacao so devolve linhas de A; com app.bypass_rls = 'on' (platform_admin)
// devolve as duas. Roda contra um Postgres real, gated por TEST_DATABASE_URL
// (mesmo padrao de migration_integration_test.go); pulado sem o env.
//
//	TEST_DATABASE_URL="postgres://user:pass@localhost:5432/testdb" \
//	  go test ./internal/modules/queue/feedback/... -run TestFeedbackRLSIsolation -v
func TestFeedbackRLSIsolation(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL nao definido — pulando teste de integracao de RLS")
	}

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("conectar ao banco de teste: %v", err)
	}
	defer pool.Close()

	// RLS NAO se aplica a superuser nem a roles BYPASSRLS — sob esses, o SELECT
	// veria as duas contas e o assert de isolamento daria FALSO-NEGATIVO. O role
	// padrao da app (omni) e superuser, entao por padrao este teste PULA. Para
	// provar isolamento de verdade, aponte TEST_DATABASE_URL a um role comum (sem
	// superuser e sem BYPASSRLS) com os GRANTs adequados.
	var rlsExempt bool
	if err := pool.QueryRow(ctx, `select current_setting('is_superuser')::bool
		or coalesce((select rolbypassrls from pg_roles where rolname = current_user), false)`).Scan(&rlsExempt); err != nil {
		t.Fatalf("checar role de conexao: %v", err)
	}
	if rlsExempt {
		t.Skip("role de conexao e superuser/BYPASSRLS — RLS e bypassado; rode com um role comum para provar isolamento")
	}

	if err := database.ApplyMigrationsWithOptions(ctx, pool, database.MigrationOptions{SkipDataSeeds: true}); err != nil {
		t.Fatalf("migrations falharam: %v", err)
	}

	const (
		tenantA  = "11111111-1111-1111-1111-111111111111"
		tenantB  = "22222222-2222-2222-2222-222222222222"
		userA    = "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
		userB    = "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"
		feedackA = "a1a1a1a1-a1a1-a1a1-a1a1-a1a1a1a1a1a1"
		feedackB = "b1b1b1b1-b1b1-b1b1-b1b1-b1b1b1b1b1b1"
	)

	// Limpeza + seed rodam sob bypass (platform_admin), senao a WITH CHECK da
	// policy barraria inserir linhas de tenant != app.account_id.
	seedConn, err := pool.Acquire(ctx)
	if err != nil {
		t.Fatalf("acquire seed conn: %v", err)
	}
	if _, err := seedConn.Exec(ctx, "select set_config('app.bypass_rls', 'on', false)"); err != nil {
		seedConn.Release()
		t.Fatalf("set bypass: %v", err)
	}

	// Idempotente: remove residuo de execucoes anteriores antes de semear.
	for _, stmt := range []string{
		`delete from feedback_messages where tenant_id in ($1::uuid, $2::uuid)`,
		`delete from user_feedback where tenant_id in ($1::uuid, $2::uuid)`,
	} {
		if _, err := seedConn.Exec(ctx, stmt, tenantA, tenantB); err != nil {
			seedConn.Release()
			t.Fatalf("cleanup feedback: %v", err)
		}
	}

	seed := []struct {
		sql  string
		args []any
	}{
		{`insert into tenants (id, slug, name) values ($1::uuid, $2, $3) on conflict (id) do nothing`, []any{tenantA, "rls-test-a", "RLS Test A"}},
		{`insert into tenants (id, slug, name) values ($1::uuid, $2, $3) on conflict (id) do nothing`, []any{tenantB, "rls-test-b", "RLS Test B"}},
		{`insert into users (id, email, display_name, password_hash) values ($1::uuid, $2, $3, 'x') on conflict (id) do nothing`, []any{userA, "rls-a@test.local", "User A"}},
		{`insert into users (id, email, display_name, password_hash) values ($1::uuid, $2, $3, 'x') on conflict (id) do nothing`, []any{userB, "rls-b@test.local", "User B"}},
		{`insert into user_feedback (id, tenant_id, user_id, user_name, kind, status, subject, body)
		  values ($1::uuid, $2::uuid, $3::uuid, 'User A', 'question', 'open', 'A', 'corpo A')`, []any{feedackA, tenantA, userA}},
		{`insert into user_feedback (id, tenant_id, user_id, user_name, kind, status, subject, body)
		  values ($1::uuid, $2::uuid, $3::uuid, 'User B', 'question', 'open', 'B', 'corpo B')`, []any{feedackB, tenantB, userB}},
	}
	for _, s := range seed {
		if _, err := seedConn.Exec(ctx, s.sql, s.args...); err != nil {
			seedConn.Release()
			t.Fatalf("seed (%s): %v", s.sql, err)
		}
	}
	_, _ = seedConn.Exec(ctx, "reset all")
	seedConn.Release()

	repo := NewPostgresRepository(pool)

	// (1) Conta A: a conn tem app.account_id = A e a query do repo NAO passa
	// filtro de tenant (tenantID = ""). Sem RLS isso devolveria as duas; com RLS
	// so deve devolver A.
	tenantAList := withScopedConn(t, ctx, pool, tenantA, false, func(scopedCtx context.Context) []Feedback {
		feedbacks, err := repo.List(scopedCtx, "", ListInput{})
		if err != nil {
			t.Fatalf("list como tenant A: %v", err)
		}
		return feedbacks
	})
	if len(tenantAList) != 1 {
		t.Fatalf("tenant A deveria ver 1 feedback, viu %d", len(tenantAList))
	}
	if tenantAList[0].TenantID != tenantA {
		t.Fatalf("tenant A viu feedback de outro tenant: %q", tenantAList[0].TenantID)
	}

	// (2) platform_admin (bypass): ve as duas contas, mesmo sem filtro.
	adminList := withScopedConn(t, ctx, pool, "", true, func(scopedCtx context.Context) []Feedback {
		feedbacks, err := repo.List(scopedCtx, "", ListInput{})
		if err != nil {
			t.Fatalf("list como platform_admin: %v", err)
		}
		return feedbacks
	})
	seen := map[string]bool{}
	for _, f := range adminList {
		seen[f.TenantID] = true
	}
	if !seen[tenantA] || !seen[tenantB] {
		t.Fatalf("platform_admin deveria ver A e B; viu %v", seen)
	}
}

// withScopedConn adquire uma conn, seta os GUCs de RLS (espelhando o
// RLSConnGuard do httpapi), poe a conn no context e executa fn. Reseta e
// libera a conn ao fim — exatamente como o middleware faz por request.
func withScopedConn(t *testing.T, ctx context.Context, pool *pgxpool.Pool, accountID string, bypass bool, fn func(context.Context) []Feedback) []Feedback {
	t.Helper()
	conn, err := pool.Acquire(ctx)
	if err != nil {
		t.Fatalf("acquire conn: %v", err)
	}
	defer func() {
		_, _ = conn.Exec(context.Background(), "reset all")
		conn.Release()
	}()

	if _, err := conn.Exec(ctx, "select set_config('app.account_id', $1, false)", accountID); err != nil {
		t.Fatalf("set app.account_id: %v", err)
	}
	if bypass {
		if _, err := conn.Exec(ctx, "select set_config('app.bypass_rls', 'on', false)"); err != nil {
			t.Fatalf("set app.bypass_rls: %v", err)
		}
	}

	return fn(database.WithConn(ctx, conn))
}
