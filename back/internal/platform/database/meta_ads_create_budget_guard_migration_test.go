package database

import (
	"strings"
	"testing"
)

func TestMetaAdsCreateBudgetGuardMigrationExtendsConstraintWithoutRewritingHistory(t *testing.T) {
	t.Parallel()

	raw, err := migrationFiles.ReadFile("migrations/0293_meta_ads_create_budget_guard.sql")
	if err != nil {
		t.Fatal(err)
	}
	sql := strings.ToLower(string(raw))
	for _, required := range []string{
		"drop constraint if exists meta_ads_action_proposals_budget_currency_ck",
		"action not in ('create_campaign', 'update_campaign')",
		"or not (payload ? 'budget')",
		"currency = 'brl'",
		"policy_configured_snapshot",
		"policy_currency_snapshot = 'brl'",
	} {
		if !strings.Contains(sql, required) {
			t.Fatalf("create budget guard migration missing %q:\n%s", required, raw)
		}
	}
	for _, forbidden := range []string{"access_token", "encrypted_token", "update meta_ads.action_proposals"} {
		if strings.Contains(sql, forbidden) {
			t.Fatalf("create budget guard migration must not contain %q:\n%s", forbidden, raw)
		}
	}
}
