package database

import (
	"strings"
	"testing"
)

func TestAssistantCredentialReferenceMigrationIsRestrictiveAndIdempotent(t *testing.T) {
	t.Parallel()

	raw, err := migrationFiles.ReadFile("migrations/0284_assistant_ai_credential_references.sql")
	if err != nil {
		t.Fatal(err)
	}
	sql := strings.ToLower(string(raw))
	for _, required := range []string{
		"automation_omni_chat_configs_credential_fk",
		"queue_attendance_analysis_configs_credential_fk",
		"references messaging.ai_credentials(id)",
		"references messaging.ai_credentials(account_id, id)",
		"on delete restrict",
		"from pg_constraint",
		"conrelid = 'automation.omni_chat_configs'::regclass",
		"conrelid = 'queue.attendance_analysis_configs'::regclass",
	} {
		if !strings.Contains(sql, required) {
			t.Fatalf("migration missing %q:\n%s", required, raw)
		}
	}
	if strings.Count(sql, "not valid") < 2 {
		t.Fatalf("both foreign keys must be NOT VALID:\n%s", raw)
	}
	if strings.Contains(sql, "on delete cascade") {
		t.Fatalf("credential references must never cascade:\n%s", raw)
	}
}
