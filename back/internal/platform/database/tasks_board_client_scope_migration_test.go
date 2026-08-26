package database

import (
	"strings"
	"testing"
)

func TestTasksBoardClientScopeMigrationIsConstrainedAndIndexed(t *testing.T) {
	t.Parallel()

	raw, err := migrationFiles.ReadFile("migrations/0295_tasks_board_client_scope.sql")
	if err != nil {
		t.Fatal(err)
	}
	sql := strings.ToLower(string(raw))
	for _, required := range []string{
		"alter table tasks.boards",
		"client_scope_mode text not null default 'active'",
		"client_scope_ids uuid[] not null",
		"check (client_scope_mode in ('all', 'active', 'selected'))",
		"using gin (client_scope_ids)",
	} {
		if !strings.Contains(sql, required) {
			t.Fatalf("tasks board client scope migration missing %q:\n%s", required, raw)
		}
	}
}
