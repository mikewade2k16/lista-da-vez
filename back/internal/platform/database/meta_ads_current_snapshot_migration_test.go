package database

import (
	"strings"
	"testing"
)

func TestMetaAdsCurrentSnapshotMigrationPreservesRowsAndVersionsConnections(t *testing.T) {
	t.Parallel()

	raw, err := migrationFiles.ReadFile("migrations/0291_meta_ads_current_snapshots.sql")
	if err != nil {
		t.Fatal(err)
	}
	sql := strings.ToLower(string(raw))
	for _, required := range []string{
		"alter table meta_ads.connections",
		"add column if not exists revision uuid",
		"set revision = gen_random_uuid()",
		"alter column revision set not null",
		"alter table meta_ads.ad_accounts",
		"add column is_current boolean not null default false",
		"alter table meta_ads.campaigns",
		"update meta_ads.ad_accounts",
		"update meta_ads.campaigns",
		"where is_current",
	} {
		if !strings.Contains(sql, required) {
			t.Fatalf("migration missing %q:\n%s", required, raw)
		}
	}
	for _, forbidden := range []string{"delete from meta_ads.ad_accounts", "delete from meta_ads.campaigns"} {
		if strings.Contains(sql, forbidden) {
			t.Fatalf("migration must preserve audit/cache identities (%q):\n%s", forbidden, raw)
		}
	}
	if strings.Contains(sql, "is_current boolean not null default true") {
		t.Fatal("legacy cache must not be trusted as current during backfill")
	}
}
