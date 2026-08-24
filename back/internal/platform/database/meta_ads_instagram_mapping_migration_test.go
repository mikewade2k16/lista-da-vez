package database

import (
	"strings"
	"testing"
)

func TestMetaAdsInstagramMappingMigrationIsTenantScoped(t *testing.T) {
	t.Parallel()

	raw, err := migrationFiles.ReadFile("migrations/0288_meta_ads_instagram_identity_client_mappings.sql")
	if err != nil {
		t.Fatal(err)
	}
	sql := strings.ToLower(string(raw))
	for _, required := range []string{
		"create table if not exists meta_ads.instagram_identity_client_mappings",
		"account_id uuid not null references core.accounts(id) on delete cascade",
		"client_account_id uuid not null references core.accounts(id) on delete restrict",
		"foreign key (account_id, connection_id)",
		"references meta_ads.connections(account_id, id) on delete cascade",
		"unique (account_id, ig_user_id)",
		"unique (account_id, page_id)",
		"check (account_id <> client_account_id)",
		"on meta_ads.instagram_identity_client_mappings (account_id, client_account_id)",
	} {
		if !strings.Contains(sql, required) {
			t.Fatalf("migration missing %q:\n%s", required, raw)
		}
	}
	for _, forbidden := range []string{"access_token", "encrypted_token", "app_secret"} {
		if strings.Contains(sql, forbidden) {
			t.Fatalf("migration must not persist %q:\n%s", forbidden, raw)
		}
	}
}
