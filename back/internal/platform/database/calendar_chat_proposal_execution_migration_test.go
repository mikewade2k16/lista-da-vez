package database

import (
	"strings"
	"testing"
)

func TestCalendarChatProposalExecutionMigrationIsScopedAndIdempotent(t *testing.T) {
	t.Parallel()

	raw, err := migrationFiles.ReadFile("migrations/0287_calendar_chat_proposal_executions.sql")
	if err != nil {
		t.Fatal(err)
	}
	sql := strings.ToLower(string(raw))
	for _, required := range []string{
		"create table if not exists calendar.chat_proposal_executions",
		"create table if not exists calendar.chat_ask_requests",
		"unique (account_id, message_id, proposal_id)",
		"foreign key (account_id, conversation_id)",
		"references calendar.chat_conversations(account_id, id)",
		"foreign key (account_id, conversation_id, message_id)",
		"references calendar.chat_messages(account_id, conversation_id, id)",
		"on calendar.chat_proposal_executions (account_id, actor_user_id, confirmation_key)",
		"where confirmation_key is not null",
		"proposal_hash bytea not null",
		"confirmation_request_hash bytea",
		"before_snapshot jsonb not null",
		"expected_version integer",
		"result_snapshot jsonb not null",
		"'pending', 'executing', 'succeeded', 'failed', 'unknown', 'rejected'",
		"unique (account_id, actor_user_id, idempotency_key)",
		"entry_surface text not null",
		"scope_client_id uuid",
		"response_snapshot jsonb not null",
		"length(confirmation_key) between 8 and 200",
		"octet_length(proposal_snapshot::text) <= 65536",
		"octet_length(response_snapshot::text) <= 131072",
		"rejected_at timestamptz",
	} {
		if !strings.Contains(sql, required) {
			t.Fatalf("migration missing %q:\n%s", required, raw)
		}
	}
	askTableStart := strings.Index(sql, "create table if not exists calendar.chat_ask_requests")
	if askTableStart < 0 {
		t.Fatal("migration missing calendar.chat_ask_requests table")
	}
	askTable := sql[askTableStart:]
	for _, forbidden := range []string{
		"requested_conversation_id uuid references",
		"conversation_id uuid references",
		"requested_conversation_id uuid not null",
	} {
		if strings.Contains(askTable, forbidden) {
			t.Fatalf("ask receipt must survive conversation deletion; found %q:\n%s", forbidden, raw)
		}
	}
	for _, forbidden := range []string{"access_token", "api_key", "app_secret"} {
		if strings.Contains(sql, forbidden) {
			t.Fatalf("migration must not persist %q:\n%s", forbidden, raw)
		}
	}
}
