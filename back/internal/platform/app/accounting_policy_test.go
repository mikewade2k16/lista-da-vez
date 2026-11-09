package app

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/accounting"
	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/core"
)

func TestAccountingHostPolicyUsesLiveMembershipModuleAndRBAC(t *testing.T) {
	url := os.Getenv("ACCOUNTING_HOST_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("requires disposable accounting_test_host database")
	}
	ctx := context.Background()
	pool, e := pgxpool.New(ctx, url)
	if e != nil {
		t.Fatal(e)
	}
	defer pool.Close()
	if !strings.HasPrefix(pool.Config().ConnConfig.Database, "accounting_test_host") {
		t.Fatal("refusing non-disposable host database")
	}
	_, e = pool.Exec(ctx, `create schema if not exists core;
 create table if not exists core.accounts(id uuid primary key default gen_random_uuid(),is_active boolean not null default true);
 create table if not exists core.users(id uuid primary key default gen_random_uuid(),is_active boolean not null default true,is_platform_admin boolean not null default false);
 create table if not exists core.account_users(account_id uuid,user_id uuid,is_active boolean not null default true);
 create table if not exists core.account_modules(account_id uuid,module_id text,enabled boolean);
 create table if not exists core.permissions(key text primary key,deprecated_at timestamptz);
 create table if not exists core.user_role_assignments(account_id uuid,user_id uuid,role_id uuid);
 create table if not exists core.role_permissions(role_id uuid,permission_key text);
 create table if not exists core.user_permission_overrides(account_id uuid,user_id uuid,permission_key text,effect text,is_active boolean);
 insert into core.permissions(key) values('accounting.documents.issue') on conflict do nothing;`)
	if e != nil {
		t.Fatal(e)
	}
	var sc accounting.Scope
	if e = pool.QueryRow(ctx, `insert into core.accounts default values returning id::text`).Scan(&sc.AccountID); e != nil {
		t.Fatal(e)
	}
	if e = pool.QueryRow(ctx, `insert into core.users(is_platform_admin) values(true) returning id::text`).Scan(&sc.ActorID); e != nil {
		t.Fatal(e)
	}
	policy := accountingPolicy{checker: auth.NewPostgresAccountMemberChecker(pool), catalog: core.NewModuleAccess(pool)}
	assertDenied := func(code string) {
		t.Helper()
		err := policy.Check(ctx, sc, "accounting.documents.issue")
		var domain *accounting.Error
		if !errors.As(err, &domain) || domain.Code != code {
			t.Fatalf("wanted %s got %v", code, err)
		}
	}
	assertDenied("permission_denied") // platform admin is not an implicit business member
	if _, e = pool.Exec(ctx, `insert into core.account_users(account_id,user_id) values($1::uuid,$2::uuid)`, sc.AccountID, sc.ActorID); e != nil {
		t.Fatal(e)
	}
	assertDenied("module_disabled")
	if _, e = pool.Exec(ctx, `insert into core.account_modules(account_id,module_id,enabled) values($1::uuid,'accounting',true)`, sc.AccountID); e != nil {
		t.Fatal(e)
	}
	assertDenied("permission_denied")
	if _, e = pool.Exec(ctx, `insert into core.user_permission_overrides(account_id,user_id,permission_key,effect,is_active) values($1::uuid,$2::uuid,'accounting.documents.issue','allow',true)`, sc.AccountID, sc.ActorID); e != nil {
		t.Fatal(e)
	}
	if e = policy.Check(ctx, sc, "accounting.documents.issue"); e != nil {
		t.Fatal(e)
	}
	if _, e = pool.Exec(ctx, `update core.user_permission_overrides set effect='deny' where account_id=$1::uuid and user_id=$2::uuid`, sc.AccountID, sc.ActorID); e != nil {
		t.Fatal(e)
	}
	assertDenied("permission_denied")
}
