package omnichannel

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestNormalizeContactMemoryRejectsSensitiveAndNestedData(t *testing.T) {
	t.Parallel()
	out := normalizeContactMemory(ContactMemorySuggestion{
		Facts: map[string]any{
			"produto": "Camiseta",
			"cpf":     "00000000000",
			"nested":  map[string]any{"unsafe": true},
		},
		Preferences: map[string]any{"cor": "azul"},
	})
	if out.Facts["produto"] != "Camiseta" || out.Preferences["cor"] != "azul" {
		t.Fatalf("memoria segura perdida: %#v", out)
	}
	if _, found := out.Facts["cpf"]; found {
		t.Fatal("documento sensivel nao pode entrar na memoria")
	}
	if _, found := out.Facts["nested"]; found {
		t.Fatal("objeto aninhado nao pode entrar na memoria")
	}
}

func TestWithContactMemoryOutputSchemaUpgradesLegacyAgent(t *testing.T) {
	t.Parallel()
	raw := json.RawMessage(`{
		"type":"object",
		"properties":{"intent":{"type":"string"}},
		"required":["intent"],
		"additionalProperties":false
	}`)
	upgraded := withContactMemoryOutputSchema(raw)
	var schema map[string]any
	if err := json.Unmarshal(upgraded, &schema); err != nil {
		t.Fatal(err)
	}
	properties := schema["properties"].(map[string]any)
	if properties["contact_memory"] == nil || properties["sentiment"] == nil {
		t.Fatalf("schema nao foi ampliado: %s", upgraded)
	}
	required := string(upgraded)
	if !strings.Contains(required, `"contact_memory"`) || !strings.Contains(required, `"sentiment"`) {
		t.Fatalf("novos campos nao sao obrigatorios: %s", upgraded)
	}
}

func TestUserPromptUsesOnlySafeName(t *testing.T) {
	t.Parallel()
	prompt := buildUserPromptWithContactIntelligence(nil, "", nil, nil, nil)
	if !strings.Contains(prompt, "cumprimente sem nome") {
		t.Fatalf("fallback de saudacao ausente: %s", prompt)
	}
	prompt = buildUserPromptWithContactIntelligence(nil, "Tamara", nil, nil, nil)
	if !strings.Contains(prompt, "Nome pessoal confiavel para saudacao: Tamara") {
		t.Fatalf("nome confiavel ausente: %s", prompt)
	}
}

func TestSuppressedOperationalMemoryNeverEntersPromptOrBrain(t *testing.T) {
	t.Parallel()
	name := "Contato Seguro"
	intelligence := ContactIntelligenceView{
		PreferredName:           &name,
		NameSource:              "contact",
		RelationshipStatus:      "customer",
		Tags:                    json.RawMessage(`["vip-safe"]`),
		Summary:                 "old-derived-sentinel",
		Facts:                   json.RawMessage(`{"sentinel":"old-derived-sentinel"}`),
		Preferences:             json.RawMessage(`{"secret":"old-derived-sentinel"}`),
		InteractionCount:        8,
		AIReplyCount:            5,
		HandoffCount:            2,
		LastIntent:              "old-derived-sentinel",
		LastSentiment:           "positive",
		LastOutcome:             "old-derived-sentinel",
		DerivedMemorySuppressed: true,
	}
	prompt := buildUserPromptWithContactIntelligence(nil, name, nil, nil, &intelligence)
	if strings.Contains(prompt, "old-derived-sentinel") || strings.Contains(prompt, "Memoria autoritativa do contato") {
		t.Fatalf("prompt reintroduziu memoria derivada suprimida: %s", prompt)
	}
	if !strings.Contains(prompt, "Nome pessoal confiavel para saudacao: "+name) {
		t.Fatalf("prompt perdeu nome CRM seguro: %s", prompt)
	}

	conversationID := "conversation-safe"
	request := buildBrainRequestV2(triageParams{
		AccountID:           "account-safe",
		ConversationID:      &conversationID,
		ContactID:           "contact-safe",
		ContactName:         name,
		ContactIntelligence: &intelligence,
		Agent:               agentRow{ID: "agent-safe"},
		Version:             versionRow{ID: "version-safe", Model: "test"},
	}, nil)
	if request.Contact.Summary != nil || request.Contact.RelationshipStatus != "customer" ||
		len(request.Contact.Tags) != 1 || request.Contact.Tags[0] != "vip-safe" ||
		request.Contact.Name == nil || *request.Contact.Name != name {
		t.Fatalf("brain context nao preservou somente CRM seguro: %+v", request.Contact)
	}
}
