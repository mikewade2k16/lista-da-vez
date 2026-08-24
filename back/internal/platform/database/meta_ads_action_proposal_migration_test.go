package database

import (
	"strings"
	"testing"
)

func TestMetaAdsActionProposalMigrationIsTenantScopedAndAtMostOnce(t *testing.T) {
	t.Parallel()

	raw, err := migrationFiles.ReadFile("migrations/0286_meta_ads_action_proposals.sql")
	if err != nil {
		t.Fatal(err)
	}
	sql := strings.ToLower(string(raw))
	for _, required := range []string{
		"create table if not exists meta_ads.action_policies",
		"create table if not exists meta_ads.action_proposals",
		"create table if not exists meta_ads.action_proposal_events",
		"resource_account_id uuid not null references core.accounts(id) on delete restrict",
		"foreign key (account_id, ad_account_id)",
		"references meta_ads.ad_accounts(account_id, id)",
		"unique (account_id, idempotency_key)",
		"on meta_ads.action_proposals (account_id, confirmation_idempotency_key)",
		"where confirmation_idempotency_key is not null",
		"attempt_count smallint not null default 0 check (attempt_count between 0 and 1)",
		"'pending', 'executing', 'succeeded', 'failed', 'unknown'",
		"jsonb_typeof(payload) = 'object'",
		"jsonb_typeof(result_snapshot) = 'object'",
		"allow_resume boolean not null default false",
		"max_daily_budget numeric(15,2)",
		"max_lifetime_budget numeric(15,2)",
	} {
		if !strings.Contains(sql, required) {
			t.Fatalf("migration missing %q:\n%s", required, raw)
		}
	}
	for _, forbidden := range []string{
		"access_token", "encrypted_token", "app_secret",
		"source_bound", "cancellation_idempotency_key", "expires_at",
		"'cancelled'", "'expired'", "'bound'",
	} {
		if strings.Contains(sql, forbidden) {
			t.Fatalf("migration must not persist %q:\n%s", forbidden, raw)
		}
	}
}

func TestMetaAdsActionLifecycleMigrationUpgradesOriginal0286FailClosed(t *testing.T) {
	t.Parallel()

	raw, err := migrationFiles.ReadFile("migrations/0289_meta_ads_action_proposal_lifecycle.sql")
	if err != nil {
		t.Fatal(err)
	}
	sql := strings.ToLower(string(raw))
	for _, required := range []string{
		"alter table meta_ads.action_proposals",
		"add column if not exists source_bound boolean",
		"set source_bound = (source = 'manual')",
		"alter column source_bound set default false",
		"add column if not exists cancellation_idempotency_key text",
		"add column if not exists expires_at timestamptz",
		"set expires_at = created_at + interval '30 minutes'",
		"alter column expires_at set not null",
		"drop constraint if exists action_proposals_status_check",
		"'cancelled', 'expired'",
		"meta_ads_action_proposals_cancellation_uidx",
		"where status = 'pending'",
		"drop constraint if exists action_proposal_events_event_type_check",
		"'proposed', 'bound', 'confirmed', 'cancelled', 'expired'",
	} {
		if !strings.Contains(sql, required) {
			t.Fatalf("lifecycle migration missing %q:\n%s", required, raw)
		}
	}
	if strings.Contains(sql, "create table") {
		t.Fatalf("0289 must upgrade the already-applied 0286 instead of recreating tables:\n%s", raw)
	}
}
