package database

import (
	"os"
	"strings"
	"testing"
)

func TestAssistantAnthropicCredentialMigrationContract(t *testing.T) {
	raw, err := os.ReadFile("migrations/0292_assistant_anthropic_credentials.sql")
	if err != nil {
		t.Fatal(err)
	}
	sql := strings.ToLower(string(raw))
	for _, required := range []string{
		"messaging.ai_credentials",
		"automation.omni_chat_configs",
		"'anthropic'",
		"messaging_ai_credentials_provider_check",
		"automation_omni_chat_configs_provider_check",
	} {
		if !strings.Contains(sql, required) {
			t.Fatalf("migration missing %q", required)
		}
	}
	if strings.Contains(sql, "intelligence.ai_credentials") || strings.Contains(sql, "messaging.ai_agents") {
		t.Fatal("migration must not broaden legacy agent providers")
	}
}
