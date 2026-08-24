package calendar

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestSanitizeAssistantResourcesValidatesKindIDURLsAndLimits(t *testing.T) {
	t.Parallel()
	longTitle := strings.Repeat("x", maxAssistantResourceTitle+20)
	input := []AssistantResource{
		{
			ID: "instagram_post:123", Kind: assistantResourceInstagramPost, Title: longTitle,
			ImageURL: "http://cdn.example/post.jpg", Permalink: "https://instagram.com/p/123",
			Metadata: map[string]string{"mediaType": "IMAGE", "unsafe key": "drop"},
		},
		{ID: "instagram_post:123", Kind: assistantResourceInstagramPost, Title: "duplicado"},
		{ID: "meta_campaign:campaign-1", Kind: assistantResourceMetaAdAccount, Title: "prefixo divergente"},
		{ID: "meta_campaign:campaign/2", Kind: assistantResourceMetaCampaign, Title: "sufixo invalido"},
	}
	for index := 0; index < maxAssistantResources+5; index++ {
		input = append(input, AssistantResource{
			ID: "meta_campaign:campaign-" + string(rune('a'+index)), Kind: assistantResourceMetaCampaign,
			Title: "Campanha",
		})
	}

	got := sanitizeAssistantResources(input)
	if len(got) != maxAssistantResources {
		t.Fatalf("resources=%d want=%d", len(got), maxAssistantResources)
	}
	first := got[0]
	if first.ImageURL != "" || first.Permalink != "https://instagram.com/p/123" {
		t.Fatalf("URLs nao foram sanitizadas: %#v", first)
	}
	if len([]rune(first.Title)) != maxAssistantResourceTitle {
		t.Fatalf("titulo deveria ser truncado em %d runas, veio %d", maxAssistantResourceTitle, len([]rune(first.Title)))
	}
	if len(first.Metadata) != 1 || first.Metadata["mediaType"] != "IMAGE" {
		t.Fatalf("metadata nao foi filtrada: %#v", first.Metadata)
	}
}

func TestSelectAuthorizedAssistantResourcesDropsForgedAndDuplicateIDs(t *testing.T) {
	t.Parallel()
	registry := []AssistantResource{
		{ID: "instagram_post:post-1", Kind: assistantResourceInstagramPost, Title: "Post real"},
		{ID: "meta_campaign:campaign-1", Kind: assistantResourceMetaCampaign, Title: "Campanha real"},
	}
	got := selectAuthorizedAssistantResources(registry, []string{
		"meta_campaign:forged", "meta_campaign:campaign-1", "meta_campaign:campaign-1", "instagram_post:post-1",
	})
	if len(got) != 2 || got[0].Title != "Campanha real" || got[1].Title != "Post real" {
		t.Fatalf("intersecao autoritativa inesperada: %#v", got)
	}
}

func TestLegacyAliasCannotDowngradeMetaOrGlobalConversationRuntime(t *testing.T) {
	t.Parallel()
	legacyRequest := ChatAskRequest{}
	if shouldUseAssistantRuntime(legacyRequest, AssistantSurfaceCalendar) {
		t.Fatal("conversa calendar legada deve preservar o fallback historico")
	}
	for _, surface := range []string{AssistantSurfaceMetaAds, AssistantSurfaceGlobal} {
		if !shouldUseAssistantRuntime(legacyRequest, surface) {
			t.Fatalf("surface %q nao pode cair na policy legada", surface)
		}
	}
	if !shouldUseAssistantRuntime(ChatAskRequest{AssistantRuntime: true}, AssistantSurfaceCalendar) {
		t.Fatal("alias canonico deve usar o runtime compartilhado")
	}
}

func TestScanChatMessageRestoresSanitizedResourcesAndHistorySummary(t *testing.T) {
	t.Parallel()
	resources, err := json.Marshal([]AssistantResource{
		{
			ID: "instagram_post:post-1", Kind: assistantResourceInstagramPost, Title: "Post real",
			ImageURL: "https://cdn.example/post.jpg", Permalink: "javascript:alert(1)", Status: "published",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	createdAt := time.Date(2026, time.August, 18, 15, 0, 0, 0, time.UTC)
	message, err := scanChatMessage(chatMessageResourceRow{
		id: "message", conversationID: "conversation", role: chatRoleAssistant,
		content: "Separei o post.", resources: resources,
		contextModules: json.RawMessage(`["meta_ads"]`), createdAt: createdAt,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(message.Resources) != 1 || message.Resources[0].Permalink != "" ||
		message.Resources[0].ImageURL != "https://cdn.example/post.jpg" {
		t.Fatalf("snapshot persistido nao foi normalizado: %#v", message.Resources)
	}
	view := messageViewFrom(message)
	if len(view.Resources) != 1 || view.Resources[0].ID != "instagram_post:post-1" {
		t.Fatalf("view nao expos recursos: %#v", view.Resources)
	}
	if len(message.ContextModules) != 1 || message.ContextModules[0] != "meta_ads" {
		t.Fatalf("marcador de contexto nao foi restaurado: %#v", message.ContextModules)
	}
	history := toHistory([]ChatMessage{message})
	if len(history) != 1 || !strings.Contains(history[0].Content, "Cards de recursos somente leitura") ||
		!strings.Contains(history[0].Content, "Post real") || strings.Contains(history[0].Content, "cdn.example") {
		t.Fatalf("history inseguro/incompleto: %#v", history)
	}
}

type chatMessageResourceRow struct {
	id             string
	conversationID string
	role           string
	content        string
	resources      json.RawMessage
	contextModules json.RawMessage
	createdAt      time.Time
}

func (row chatMessageResourceRow) Scan(dest ...any) error {
	*dest[0].(*string) = row.id
	*dest[1].(*string) = row.conversationID
	*dest[2].(*string) = row.role
	*dest[3].(*string) = row.content
	*dest[4].(*json.RawMessage) = json.RawMessage("null")
	*dest[5].(*string) = "none"
	*dest[6].(*json.RawMessage) = json.RawMessage("[]")
	*dest[7].(*json.RawMessage) = json.RawMessage("[]")
	*dest[8].(*json.RawMessage) = row.resources
	*dest[9].(*json.RawMessage) = row.contextModules
	*dest[10].(*time.Time) = row.createdAt
	return nil
}
