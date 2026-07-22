package omnichannel

import (
	"strings"
	"testing"
)

func TestAppendAIToolInstructionsIsDeterministicAndAllowlisted(t *testing.T) {
	got := appendAIToolInstructions("base", []string{"knowledge.search", "crm.contact.lookup"})
	if !strings.Contains(got, "knowledge.search") || !strings.Contains(got, "crm.contact.lookup") {
		t.Fatalf("allowlist ausente: %s", got)
	}
	if strings.Index(got, "crm.contact.lookup") > strings.Index(got, "knowledge.search") {
		t.Fatal("allowlist deveria ser ordenada")
	}
	if strings.Contains(got, "apiKey") || strings.Contains(got, "Authorization") {
		t.Fatal("prompt de tool não deve conter credenciais")
	}
}
