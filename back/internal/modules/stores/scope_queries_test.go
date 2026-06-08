package stores

import (
	"context"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
)

func TestBuildListAccessibleQueryReadsTenantScopedStoresFromCore(t *testing.T) {
	query, args := buildListAccessibleQuery(auth.Principal{
		UserID: "user-1",
		Role:   auth.RoleOwner,
	}, ListInput{})

	assertQueryUsesCoreRoles(t, query)
	assertQuerySkipsLegacyScopeTables(t, query)
	assertStringSliceArgContains(t, args, 1, "queue.owner")
	assertStringSliceArgContains(t, args, 1, "queue.director")
	assertStringSliceArgContains(t, args, 1, "queue.marketing")
}

func TestBuildListAccessibleQueryReadsStoreScopedStoresFromCoreSettings(t *testing.T) {
	query, args := buildListAccessibleQuery(auth.Principal{
		UserID: "user-1",
		Role:   auth.RoleConsultant,
	}, ListInput{TenantID: "account-1"})

	assertQueryUsesCoreRoles(t, query)
	assertQuerySkipsLegacyScopeTables(t, query)
	assertQueryContains(t, query, "core.user_module_settings")
	assertQueryContains(t, query, "storeIdsByAccount")
	assertStringSliceArgContains(t, args, 1, "queue.consultant")
	if len(args) != 3 || args[2] != "account-1" {
		t.Fatalf("expected tenant filter as third arg, got %#v", args)
	}
}

func TestBuildFindAccessibleQueryReadsStoreScopedStoreFromCoreSettings(t *testing.T) {
	query, args := buildFindAccessibleQuery(auth.Principal{
		UserID: "user-1",
		Role:   auth.RoleManager,
	}, "store-1")

	assertQueryUsesCoreRoles(t, query)
	assertQuerySkipsLegacyScopeTables(t, query)
	assertQueryContains(t, query, "core.user_module_settings")
	assertQueryContains(t, query, "storeIdsByAccount")
	assertStringSliceArgContains(t, args, 2, "queue.manager")
}

func TestDeleteCoreStoreScopeTxRemovesStoreFromQueueSettings(t *testing.T) {
	tx := &recordingTx{}
	err := deleteCoreStoreScopeTx(context.Background(), tx, "store-1")
	if err != nil {
		t.Fatalf("deleteCoreStoreScopeTx returned error: %v", err)
	}

	if len(tx.execs) != 1 {
		t.Fatalf("expected one exec, got %#v", tx.execs)
	}
	assertQueryContains(t, tx.execs[0].sql, "update core.user_module_settings")
	assertQueryContains(t, tx.execs[0].sql, "storeIdsByAccount")
	assertQueryContains(t, tx.execs[0].sql, "queue.stores")
	if len(tx.execs[0].args) != 1 || tx.execs[0].args[0] != "store-1" {
		t.Fatalf("expected store id arg, got %#v", tx.execs[0].args)
	}
}

func assertQueryUsesCoreRoles(t *testing.T, query string) {
	t.Helper()
	assertQueryContains(t, query, "core.account_users")
	assertQueryContains(t, query, "core.user_role_assignments")
	assertQueryContains(t, query, "core.roles")
}

func assertQuerySkipsLegacyScopeTables(t *testing.T, query string) {
	t.Helper()
	for _, legacyTable := range []string{"user_tenant_roles", "user_store_roles"} {
		if strings.Contains(query, legacyTable) {
			t.Fatalf("query still reads legacy table %s:\n%s", legacyTable, query)
		}
	}
}

func assertQueryContains(t *testing.T, query string, fragment string) {
	t.Helper()
	if !strings.Contains(query, fragment) {
		t.Fatalf("expected query to contain %q, got:\n%s", fragment, query)
	}
}

func assertStringSliceArgContains(t *testing.T, args []any, index int, want string) {
	t.Helper()
	if len(args) <= index {
		t.Fatalf("expected arg %d in %#v", index, args)
	}
	values, ok := args[index].([]string)
	if !ok {
		t.Fatalf("expected arg %d to be []string, got %T", index, args[index])
	}
	for _, value := range values {
		if value == want {
			return
		}
	}
	t.Fatalf("expected arg %d to contain %q, got %#v", index, want, values)
}

type recordedExec struct {
	sql  string
	args []any
}

type recordingTx struct {
	execs []recordedExec
}

func (tx *recordingTx) Begin(context.Context) (pgx.Tx, error) {
	return tx, nil
}

func (tx *recordingTx) Commit(context.Context) error {
	return nil
}

func (tx *recordingTx) Rollback(context.Context) error {
	return nil
}

func (tx *recordingTx) CopyFrom(context.Context, pgx.Identifier, []string, pgx.CopyFromSource) (int64, error) {
	return 0, nil
}

func (tx *recordingTx) SendBatch(context.Context, *pgx.Batch) pgx.BatchResults {
	return nil
}

func (tx *recordingTx) LargeObjects() pgx.LargeObjects {
	return pgx.LargeObjects{}
}

func (tx *recordingTx) Prepare(context.Context, string, string) (*pgconn.StatementDescription, error) {
	return nil, nil
}

func (tx *recordingTx) Exec(_ context.Context, sql string, arguments ...any) (pgconn.CommandTag, error) {
	tx.execs = append(tx.execs, recordedExec{sql: sql, args: append([]any{}, arguments...)})
	return pgconn.NewCommandTag("UPDATE 1"), nil
}

func (tx *recordingTx) Query(context.Context, string, ...any) (pgx.Rows, error) {
	return nil, nil
}

func (tx *recordingTx) QueryRow(context.Context, string, ...any) pgx.Row {
	return nil
}

func (tx *recordingTx) Conn() *pgx.Conn {
	return nil
}
