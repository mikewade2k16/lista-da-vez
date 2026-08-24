package database

import (
	"strings"
	"testing"
)

func TestMetaAdsInstagramPostAdActionMigrationIsPausedTreeAtMostOnce(t *testing.T) {
	t.Parallel()

	raw, err := migrationFiles.ReadFile("migrations/0294_meta_ads_instagram_post_ad_actions.sql")
	if err != nil {
		t.Fatal(err)
	}
	sql := strings.ToLower(string(raw))
	for _, required := range []string{
		"'promote_instagram_post'",
		"action in ('create_campaign', 'promote_instagram_post')",
		"create table if not exists meta_ads.action_proposal_steps",
		"step in ('campaign', 'ad_set', 'creative', 'ad')",
		"status in ('executing', 'succeeded', 'failed', 'unknown')",
		"unique (account_id, proposal_id, step)",
		"status = 'succeeded' and external_entity_id <> ''",
		"references meta_ads.action_proposals(account_id, id) on delete cascade",
	} {
		if !strings.Contains(sql, required) {
			t.Fatalf("Instagram post ad migration missing %q:\n%s", required, raw)
		}
	}
	for _, forbidden := range []string{"access_token", "encrypted_token", "status = 'active'"} {
		if strings.Contains(sql, forbidden) {
			t.Fatalf("Instagram post ad migration must not contain %q:\n%s", forbidden, raw)
		}
	}
}
