package automation

import (
	"strings"
	"testing"
)

func TestOmniChatInheritedConfigUsesCanonicalAgency(t *testing.T) {
	t.Parallel()

	normalized := strings.Join(strings.Fields(strings.ToLower(omniChatInheritedConfigQuery)), " ")
	for _, required := range []string{
		"candidate.organization_id=consumer.organization_id",
		"candidate.is_agency=true",
		"candidate.is_active",
		"order by candidate.created_at,candidate.id",
		"consumer.is_agency=false",
		"config.account_id=agency.id",
	} {
		if !strings.Contains(normalized, required) {
			t.Fatalf("inheritance query missing %q:\n%s", required, omniChatInheritedConfigQuery)
		}
	}
}

func TestOmniChatConfigViewReportsInheritance(t *testing.T) {
	t.Parallel()

	view := omniChatConfigView(OmniChatConfig{
		Enabled:           true,
		SystemPrompt:      "Prompt da agencia",
		CredentialID:      "credential-id",
		Provider:          "openai",
		Model:             "gpt-test",
		Temperature:       0.3,
		HistoryWindow:     7,
		SurfaceModules:    defaultAssistantSurfaceModules(),
		Inherited:         true,
		SourceAccountName: "Agencia Canonica",
	})

	if !view.Inherited || view.InheritedFrom != "Agencia Canonica" {
		t.Fatalf("inheritance metadata = %#v", view)
	}
	if view.CredentialID != "credential-id" || view.SystemPrompt != "Prompt da agencia" {
		t.Fatalf("inherited config lost effective values: %#v", view)
	}
}

func TestDefaultOmniChatConfigIsAnExplicitLocalFallback(t *testing.T) {
	t.Parallel()

	config := defaultOmniChatConfig("account-id")
	if config.Inherited || config.AccountID != "account-id" || config.SourceAccountID != "account-id" {
		t.Fatalf("default config scope = %#v", config)
	}
	if config.SurfaceModules["meta_ads"]["meta_ads"] != "write" {
		t.Fatalf("default surface modules = %#v", config.SurfaceModules)
	}
}
