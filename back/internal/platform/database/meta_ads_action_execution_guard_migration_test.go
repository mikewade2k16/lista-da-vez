package database

import (
	"strings"
	"testing"
)

func TestMetaAdsActionExecutionGuardMigrationIsVersionedAndFailClosed(t *testing.T) {
	t.Parallel()

	raw, err := migrationFiles.ReadFile("migrations/0290_meta_ads_action_execution_guards.sql")
	if err != nil {
		t.Fatal(err)
	}
	sql := strings.ToLower(string(raw))
	for _, required := range []string{
		"add column if not exists guard_snapshot_version smallint not null default 0",
		"add column if not exists guard_snapshot_hash text not null default ''",
		"add column if not exists connection_id_snapshot uuid",
		"add column if not exists connection_revision_snapshot uuid",
		"add column if not exists ad_account_client_account_id_snapshot uuid",
		"add column if not exists policy_updated_at_snapshot timestamptz",
		"add column if not exists policy_hash_snapshot text not null default ''",
		"add column if not exists campaign_synced_at_snapshot timestamptz",
		"add column if not exists campaign_hash_snapshot text not null default ''",
		"add column if not exists claimed_connection_id uuid",
		"add column if not exists claimed_connection_revision uuid",
		"guard_snapshot_version = 0",
		"guard_snapshot_hash ~ '^[0-9a-f]{64}$'",
		"claimed_connection_id is null and claimed_connection_revision is null",
		"claimed_connection_id = connection_id_snapshot",
		"claimed_connection_revision = connection_revision_snapshot",
		"attempt_count = 1",
		"or not (payload ? 'budget')",
		"or (currency = 'brl'",
		"and policy_configured_snapshot",
		"and policy_currency_snapshot = 'brl'",
		"where claimed_connection_revision is not null",
	} {
		if !strings.Contains(sql, required) {
			t.Fatalf("execution guard migration missing %q:\n%s", required, raw)
		}
	}
	for _, forbidden := range []string{
		"access_token", "encrypted_token", "pgp_sym_decrypt", "update meta_ads.action_proposals set guard_snapshot_version = 1",
	} {
		if strings.Contains(sql, forbidden) {
			t.Fatalf("execution guard migration must not contain %q:\n%s", forbidden, raw)
		}
	}
}
