package database

import (
	"strings"
	"testing"
)

func TestTasksBoardSourcesMigrationDefaultsEmptyAndPersistsUserPreference(t *testing.T) {
	t.Parallel()

	raw, err := migrationFiles.ReadFile("migrations/0296_tasks_board_sources_and_user_preferences.sql")
	if err != nil {
		t.Fatal(err)
	}
	sql := strings.ToLower(string(raw))
	for _, required := range []string{
		"task_source_mode text not null default 'own'",
		"task_source_board_ids uuid[] not null default '{}'::uuid[]",
		"check (task_source_mode in ('own', 'all', 'selected'))",
		"create table if not exists tasks.user_preferences",
		"primary key (account_id, user_id)",
		"foreign key (last_board_id, account_id)",
	} {
		if !strings.Contains(sql, required) {
			t.Fatalf("tasks board sources migration missing %q:\n%s", required, raw)
		}
	}
}
