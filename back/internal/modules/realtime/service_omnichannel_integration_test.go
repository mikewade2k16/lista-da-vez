package realtime

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
	platformdb "github.com/mikewade2k16/lista-da-vez/back/internal/platform/database"
)

func TestAuthorizeOmnichannelAccountIntegration(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv("OMNI_E1_TEST_DATABASE_URL"))
	if dsn == "" {
		t.Skip("OMNI_E1_TEST_DATABASE_URL nao definido")
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	config, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		t.Fatal(err)
	}
	config.ConnConfig.DefaultQueryExecMode = pgx.QueryExecModeSimpleProtocol
	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	if err := platformdb.ApplyMigrationsWithOptions(ctx, pool, platformdb.MigrationOptions{SkipDataSeeds: true}); err != nil {
		t.Fatalf("apply real migrations: %v", err)
	}

	const (
		accountID = "a1111111-1111-4111-8111-111111111111"
		userID    = "a2222222-2222-4222-8222-222222222222"
		roleID    = "a3333333-3333-4333-8333-333333333333"
	)
	cleanup := func() {
		_, _ = pool.Exec(context.Background(), `delete from core.accounts where id=$1::uuid;
			delete from core.users where id=$2::uuid`, accountID, userID)
	}
	cleanup()
	defer cleanup()

	if _, err := pool.Exec(ctx, `insert into core.modules(id,schema_name,label)
		values ('omnichannel','messaging','Omnichannel')
		on conflict(id) do update set schema_name=excluded.schema_name,label=excluded.label;
		insert into core.permissions(key,module_id,label,scope)
		values ($4,'omnichannel','View conversations','account')
		on conflict(key) do update set module_id=excluded.module_id,label=excluded.label,scope=excluded.scope;
		insert into core.accounts(id,slug,name) values ($1::uuid,'p1b-realtime','P1B Realtime');
		insert into core.account_modules(account_id,module_id,enabled) values ($1::uuid,'omnichannel',true);
		insert into core.users(id,email,display_name) values ($2::uuid,'p1b-realtime@example.invalid','P1B Realtime');
		insert into core.account_users(account_id,user_id) values ($1::uuid,$2::uuid);
		insert into core.roles(id,account_id,code,label) values ($3::uuid,$1::uuid,'p1b-realtime','P1B Realtime');
		insert into core.user_role_assignments(account_id,user_id,role_id) values ($1::uuid,$2::uuid,$3::uuid);
		insert into core.role_permissions(role_id,permission_key) values ($3::uuid,$4)`,
		accountID, userID, roleID, omnichannelViewPermission); err != nil {
		t.Fatal(err)
	}

	service := NewService(nil, nil, nil, nil, NewHub(), pool)
	principal := auth.Principal{
		UserID:              userID,
		AccountID:           accountID,
		Role:                auth.RolePlatformAdmin,
		PermissionsResolved: true,
		Permissions:         []string{omnichannelViewPermission},
	}
	if err := service.authorizeOmnichannelAccount(ctx, principal, accountID); err != nil {
		t.Fatalf("valid authorization: %v", err)
	}

	if _, err := pool.Exec(ctx, `delete from core.role_permissions
		where role_id=$1::uuid and permission_key=$2`, roleID, omnichannelViewPermission); err != nil {
		t.Fatal(err)
	}
	if err := service.authorizeOmnichannelAccount(ctx, principal, accountID); !errors.Is(err, errRealtimeForbidden) {
		t.Fatalf("stale token/platform role bypassed current RBAC: %v", err)
	}
	if _, err := pool.Exec(ctx, `insert into core.role_permissions(role_id,permission_key) values ($1::uuid,$2)`,
		roleID, omnichannelViewPermission); err != nil {
		t.Fatal(err)
	}

	if _, err := pool.Exec(ctx, `update core.account_modules set enabled=false
		where account_id=$1::uuid and module_id='omnichannel'`, accountID); err != nil {
		t.Fatal(err)
	}
	if err := service.authorizeOmnichannelAccount(ctx, principal, accountID); !errors.Is(err, errRealtimeForbidden) {
		t.Fatalf("disabled module gate: %v", err)
	}
	if _, err := pool.Exec(ctx, `update core.account_modules set enabled=true
		where account_id=$1::uuid and module_id='omnichannel';
		update core.account_users set is_active=false where account_id=$1::uuid and user_id=$2::uuid`, accountID, userID); err != nil {
		t.Fatal(err)
	}
	if err := service.authorizeOmnichannelAccount(ctx, principal, accountID); !errors.Is(err, errRealtimeNotFound) {
		t.Fatalf("inactive membership/platform role bypass: %v", err)
	}
	if _, err := pool.Exec(ctx, `update core.account_users set is_active=true where account_id=$1::uuid and user_id=$2::uuid;
		update core.accounts set is_active=false where id=$1::uuid`, accountID, userID); err != nil {
		t.Fatal(err)
	}
	if err := service.authorizeOmnichannelAccount(ctx, principal, accountID); !errors.Is(err, errRealtimeNotFound) {
		t.Fatalf("inactive account gate: %v", err)
	}
}
