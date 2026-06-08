package users

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
)

func TestBuildScopedQueryReadsUsersFromCore(t *testing.T) {
	query, args := buildScopedQuery(auth.Principal{
		UserID:    "platform-1",
		Role:      auth.RolePlatformAdmin,
		AccountID: "account-1",
	}, ListInput{}, "")

	if !strings.Contains(query, "from core.account_users") {
		t.Fatalf("expected query to read core.account_users, got:\n%s", query)
	}
	for _, legacyTable := range []string{"user_tenant_roles", "user_store_roles", "user_platform_roles"} {
		if strings.Contains(query, legacyTable) {
			t.Fatalf("query still reads legacy table %s:\n%s", legacyTable, query)
		}
	}
	if len(args) < 2 || args[1] != "account-1" {
		t.Fatalf("expected account id from Principal.AccountID as arg[1], got %#v", args)
	}
}

func TestScanUserMapsCoreOnlyProjection(t *testing.T) {
	now := time.Now().UTC()
	row := fakeUserRow{values: []any{
		"user-1",
		"Gerente Core",
		"gerente",
		"gerente@example.test",
		"259",
		"Gerente de Loja",
		"queue.manager",
		"queue.supervisor",
		"account-1",
		[]string{"store-1"},
		true,
		true,
		false,
		"",
		"",
		"",
		nil,
		now,
		now,
	}}

	user, err := scanUser(row)
	if err != nil {
		t.Fatalf("scanUser returned error: %v", err)
	}
	if user.Role != auth.RoleManager {
		t.Fatalf("role = %q, want %q", user.Role, auth.RoleManager)
	}
	if user.TenantID != "account-1" {
		t.Fatalf("tenant = %q, want account-1", user.TenantID)
	}
	if len(user.StoreIDs) != 1 || user.StoreIDs[0] != "store-1" {
		t.Fatalf("store ids = %#v, want [store-1]", user.StoreIDs)
	}
}

func TestUpsertCoreAssignmentsCreatesMembershipAndRoleAssignment(t *testing.T) {
	tx := &recordingTx{}
	err := upsertCoreAssignmentsTx(context.Background(), tx, User{
		ID:       "user-1",
		Role:     auth.RoleManager,
		TenantID: "account-1",
		StoreIDs: []string{"store-1"},
	})
	if err != nil {
		t.Fatalf("upsertCoreAssignmentsTx returned error: %v", err)
	}

	assertExecContains(t, tx, "update core.users")
	assertExecContains(t, tx, "insert into core.account_users")
	assertExecContains(t, tx, "insert into core.user_role_assignments")
	assertExecContains(t, tx, "insert into core.user_module_settings")
	if !tx.hasArg("queue.manager") {
		t.Fatalf("expected queue.manager role code in exec args, got %#v", tx.execs)
	}
}

type fakeUserRow struct {
	values []any
}

func (row fakeUserRow) Scan(dest ...any) error {
	for index, value := range row.values {
		switch target := dest[index].(type) {
		case *string:
			*target = value.(string)
		case *[]string:
			*target = append([]string{}, value.([]string)...)
		case *bool:
			*target = value.(bool)
		case **time.Time:
			if value == nil {
				*target = nil
			} else {
				timeValue := value.(time.Time)
				*target = &timeValue
			}
		case *time.Time:
			*target = value.(time.Time)
		default:
			return ErrValidation
		}
	}
	return nil
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
	return pgconn.NewCommandTag("INSERT 0 1"), nil
}

func (tx *recordingTx) Query(context.Context, string, ...any) (pgx.Rows, error) {
	return nil, nil
}

func (tx *recordingTx) QueryRow(context.Context, string, ...any) pgx.Row {
	return fakeUserRow{}
}

func (tx *recordingTx) Conn() *pgx.Conn {
	return nil
}

func (tx *recordingTx) hasArg(want string) bool {
	for _, exec := range tx.execs {
		for _, arg := range exec.args {
			if arg == want {
				return true
			}
		}
	}
	return false
}

func assertExecContains(t *testing.T, tx *recordingTx, fragment string) {
	t.Helper()
	for _, exec := range tx.execs {
		if strings.Contains(exec.sql, fragment) {
			return
		}
	}
	t.Fatalf("expected exec containing %q, got %#v", fragment, tx.execs)
}
