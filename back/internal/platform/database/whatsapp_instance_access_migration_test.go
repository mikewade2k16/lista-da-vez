package database

import (
	"regexp"
	"strings"
	"testing"
)

func TestWhatsAppInstanceAccessMigrationIsTenantScopedAndPreservesAuditUnion(t *testing.T) {
	t.Parallel()

	raw, err := migrationFiles.ReadFile("migrations/0298_messaging_whatsapp_instance_access.sql")
	if err != nil {
		t.Fatal(err)
	}
	sql := strings.ToLower(string(raw))
	for _, required := range []string{
		"add column if not exists access_policy text not null default 'restricted'",
		"add column if not exists access_revision bigint not null default 0",
		"create table if not exists messaging.whatsapp_instance_user_grants",
		"primary key (account_id, instance_id, user_id)",
		"foreign key (account_id, instance_id)",
		"references messaging.whatsapp_instances(account_id, id)",
		"foreign key (account_id, user_id)",
		"references core.account_users(account_id, user_id)",
		"where is_active = true",
		"'whatsapp_instance_access_changed'",
		"'p1_access_backfill'",
	} {
		if !strings.Contains(sql, required) {
			t.Fatalf("WhatsApp instance access migration missing %q:\n%s", required, raw)
		}
	}
	for _, forbidden := range []string{
		"default 'account_shared'",
		"delete from messaging.whatsapp_instance_user_grants",
		"delete from messaging.whatsapp_instances",
		"truncate ",
		"drop table",
	} {
		if strings.Contains(sql, forbidden) {
			t.Fatalf("WhatsApp instance access migration must not contain %q:\n%s", forbidden, raw)
		}
	}

	previous, err := migrationFiles.ReadFile("migrations/0297_messaging_whatsapp_history_visibility.sql")
	if err != nil {
		t.Fatal(err)
	}
	eventPattern := regexp.MustCompile(`'[A-Z][A-Z0-9_]+'`)
	for _, eventType := range eventPattern.FindAllString(string(previous), -1) {
		if !strings.Contains(string(raw), eventType) {
			t.Fatalf("0298 removed audit event %s from the union", eventType)
		}
	}
}
