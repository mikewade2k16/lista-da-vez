package app

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/core"
	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/finance"
)

func TestFinanceHostUsesAccountRBACWithoutImplicitAccountingGrants(t *testing.T) {
	url := os.Getenv("FINANCE_SCOPE_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("requires migrated disposable accounting_test database")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	if !strings.HasPrefix(pool.Config().ConnConfig.Database, "accounting_test_") {
		t.Fatal("refusing non-disposable database")
	}
	exec := func(sql string, args ...any) {
		t.Helper()
		if _, err := pool.Exec(ctx, sql, args...); err != nil {
			t.Fatal(err)
		}
	}
	id := func(sql string, args ...any) string {
		t.Helper()
		var out string
		if err := pool.QueryRow(ctx, sql, args...).Scan(&out); err != nil {
			t.Fatal(err)
		}
		return out
	}
	exec(`insert into core.modules(id,schema_name,label) values('finance','finance','Finance'),('core','core','Core') on conflict do nothing`)
	exec(`insert into core.permissions(key,module_id,label) values('finance.sheets.view','finance','View'),('core.account.manage','core','Manage') on conflict do nothing`)
	account := id(`insert into core.accounts(slug,name) values(gen_random_uuid()::text,'Synthetic Finance scope') returning id::text`)
	user := id(`insert into core.users(email,display_name,is_platform_admin) values(gen_random_uuid()::text||'@scope.invalid','Synthetic scope',true) returning id::text`)
	defer exec(`delete from core.users where id=$1::uuid`, user)
	defer exec(`delete from core.accounts where id=$1::uuid`, account)
	exec(`insert into core.account_modules(account_id,module_id,enabled) values($1::uuid,'finance',true)`, account)
	authorize := plusAuthorizer(auth.NewPostgresAccountMemberChecker(pool), core.NewModuleAccess(pool))
	sc := finance.PlusScope{AccountID: account, ActorID: user}
	check := func(want bool, permission string) {
		t.Helper()
		err := authorize(ctx, sc, permission)
		var denied *finance.PlusError
		if err != nil && !errors.As(err, &denied) {
			t.Fatalf("unexpected authorization error: %v", err)
		}
		if (err == nil) != want {
			t.Fatalf("allowed=%t, error=%v", want, err)
		}
	}
	check(true, "finance.sheets.view")      // canonical platform visibility, no direct membership
	check(false, "accounting.issuers.view") // business grants remain explicit
	exec(`update core.users set is_active=false where id=$1::uuid`, user)
	check(false, "finance.sheets.view")
	exec(`update core.users set is_active=true,is_platform_admin=false where id=$1::uuid`, user)
	check(false, "finance.sheets.view")
	exec(`insert into core.account_users(account_id,user_id) values($1::uuid,$2::uuid)`, account, user)
	role := id(`insert into core.roles(account_id,code,label) values($1::uuid,'core.owner','Owner') returning id::text`, account)
	exec(`insert into core.user_role_assignments(account_id,user_id,role_id) values($1::uuid,$2::uuid,$3::uuid)`, account, user, role)
	exec(`insert into core.role_permissions(role_id,permission_key) values($1::uuid,'core.account.manage')`, role)
	check(true, "finance.sheets.view")
	exec(`update core.roles set code='custom' where id=$1::uuid`, role)
	check(false, "finance.sheets.view")
	exec(`insert into core.role_permissions(role_id,permission_key) values($1::uuid,'finance.sheets.view')`, role)
	check(true, "finance.sheets.view")
	exec(`delete from core.role_permissions where role_id=$1::uuid and permission_key='finance.sheets.view'`, role)
	check(false, "finance.sheets.view")
	exec(`update core.roles set code='core.owner' where id=$1::uuid`, role)
	exec(`update core.account_modules set enabled=false where account_id=$1::uuid and module_id='finance'`, account)
	check(false, "finance.sheets.view")
	exec(`update core.account_modules set enabled=true where account_id=$1::uuid and module_id='finance'`, account)
	exec(`update core.account_users set is_active=false where account_id=$1::uuid and user_id=$2::uuid`, account, user)
	check(false, "finance.sheets.view")
}
