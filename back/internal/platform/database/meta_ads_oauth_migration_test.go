package database

import (
	"strings"
	"testing"
)

func TestMetaAdsOAuthStateMigrationStoresOnlySingleUseHash(t *testing.T) {
	t.Parallel()

	raw, err := migrationFiles.ReadFile("migrations/0285_meta_ads_oauth_states.sql")
	if err != nil {
		t.Fatal(err)
	}
	sql := strings.ToLower(string(raw))
	for _, required := range []string{
		"create table if not exists meta_ads.oauth_states",
		"account_id uuid not null references core.accounts(id)",
		"created_by_user_id uuid not null references core.users(id)",
		"state_hash bytea not null",
		"unique (state_hash)",
		"octet_length(state_hash) = 32",
		"expires_at timestamptz not null",
		"consumed_at timestamptz",
		"where consumed_at is null",
	} {
		if !strings.Contains(sql, required) {
			t.Fatalf("migration missing %q:\n%s", required, raw)
		}
	}
	for _, forbidden := range []string{"access_token", "authorization_code", "code text", "state text"} {
		if strings.Contains(sql, forbidden) {
			t.Fatalf("migration must not persist %q:\n%s", forbidden, raw)
		}
	}
}
