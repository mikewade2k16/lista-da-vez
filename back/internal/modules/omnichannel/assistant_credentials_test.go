package omnichannel

import (
	"strings"
	"testing"
)

func TestAssistantCredentialQueriesRestrictInheritanceToCanonicalAgency(t *testing.T) {
	t.Parallel()

	queries := map[string]string{
		"catalog": assistantCredentialCatalogQuery,
		"runtime": sharedRuntimeAICredentialQuery,
	}
	for name, query := range queries {
		query := query
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			normalized := strings.Join(strings.Fields(strings.ToLower(query)), " ")
			for _, required := range []string{
				"consumer.is_agency=false",
				"agency.organization_id=consumer.organization_id",
				"agency.is_agency=true",
				"agency.is_active",
				"order by agency.created_at,agency.id",
				"owner.id=consumer.id",
			} {
				if !strings.Contains(normalized, required) {
					t.Fatalf("query missing %q:\n%s", required, query)
				}
			}
			if strings.Contains(normalized, "owner.id=consumer.id or owner.is_agency") {
				t.Fatalf("query still accepts any agency in the organization:\n%s", query)
			}
		})
	}
}

func TestOwnedAICredentialViewMarksOwnMutationScope(t *testing.T) {
	t.Parallel()

	view := ownedAICredentialView(AICredentialView{ID: "credential", ReadOnly: true})
	if !view.OwnedByAccount || view.ReadOnly {
		t.Fatalf("owned view = %#v", view)
	}
}

func TestNormalizeAICredentialCapability(t *testing.T) {
	t.Parallel()

	if got, ok := normalizeAICredentialCapability(" Response "); !ok || got != "response" {
		t.Fatalf("capability = %q, %t", got, ok)
	}
	if _, ok := normalizeAICredentialCapability("admin"); ok {
		t.Fatal("unknown capability must be denied")
	}
}

func TestNormalizeAssistantCredentialProviderAllowsAnthropicOnlyInNamedVault(t *testing.T) {
	t.Parallel()

	if got := normalizeAICredentialProvider(" Anthropic "); got != "anthropic" {
		t.Fatalf("provider = %q", got)
	}
	if got := normalizeAIProviderKeyID("anthropic"); got != "" {
		t.Fatalf("legacy agent keyring unexpectedly accepted Anthropic: %q", got)
	}
}
