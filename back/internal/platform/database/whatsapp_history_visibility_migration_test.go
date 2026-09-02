package database

import (
	"regexp"
	"strings"
	"testing"
)

func TestWhatsAppHistoryVisibilityMigrationIsLogicalAndPreservesAuditUnion(t *testing.T) {
	t.Parallel()

	raw, err := migrationFiles.ReadFile("migrations/0297_messaging_whatsapp_history_visibility.sql")
	if err != nil {
		t.Fatal(err)
	}
	sql := strings.ToLower(string(raw))
	for _, required := range []string{
		"alter table messaging.whatsapp_instances",
		"add column if not exists history_visible_from timestamptz",
		"add column if not exists history_reset_revision bigint not null default 0",
		"alter table messaging.audit_events",
		"'whatsapp_instance_history_reset'",
	} {
		if !strings.Contains(sql, required) {
			t.Fatalf("WhatsApp history migration missing %q:\n%s", required, raw)
		}
	}
	for _, forbidden := range []string{
		"delete from messaging.messages",
		"delete from messaging.conversations",
		"delete from messaging.audit_events",
		"truncate ",
		"drop table",
	} {
		if strings.Contains(sql, forbidden) {
			t.Fatalf("logical history reset migration must not contain %q:\n%s", forbidden, raw)
		}
	}

	previous, err := migrationFiles.ReadFile("migrations/0225_messaging_ai_tool_approvals.sql")
	if err != nil {
		t.Fatal(err)
	}
	eventPattern := regexp.MustCompile(`'[A-Z][A-Z0-9_]+'`)
	for _, eventType := range eventPattern.FindAllString(string(previous), -1) {
		if !strings.Contains(string(raw), eventType) {
			t.Fatalf("0297 removed audit event %s from the union", eventType)
		}
	}
}
