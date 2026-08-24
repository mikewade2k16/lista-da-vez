package calendar

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type assistantWorkflowExport struct {
	Nodes []struct {
		Name       string `json:"name"`
		Parameters struct {
			JSCode       string `json:"jsCode"`
			URL          string `json:"url"`
			HeaderParams any    `json:"headerParameters"`
			Body         any    `json:"jsonBody"`
		} `json:"parameters"`
	} `json:"nodes"`
}

func TestAssistantWorkflowExportsClosedMetaActionContract(t *testing.T) {
	t.Parallel()

	path := filepath.Join("..", "..", "..", "..", "automation", "export", "workflow-calendar-chat.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var workflows []assistantWorkflowExport
	if err := json.Unmarshal(raw, &workflows); err != nil {
		t.Fatalf("invalid workflow export: %v", err)
	}
	if len(workflows) != 1 {
		t.Fatalf("expected one exported workflow, got %d", len(workflows))
	}
	nodes := make(map[string]string)
	for _, node := range workflows[0].Nodes {
		nodes[node.Name] = node.Parameters.JSCode
	}

	prompt := nodes["Montar contexto"]
	for _, required := range []string{
		"kind (event|task|taskItem|note|clientProfile|metaAction)",
		"cap.effectiveMode === 'read' || cap.effectiveMode === 'write'",
		"cap.module === 'meta_ads' && cap.effectiveMode === 'write'",
		"if (metaRead) system += 'Meta Ads disponivel no contexto autoritativo ja fornecido; modo efetivo: '",
		"fields.metaAction tem shape FECHADO",
		"create_campaign",
		"duplicate_campaign",
		"update_campaign",
		"pause_campaign",
		"resume_campaign",
		"promote_instagram_post",
		"instagramPostId EXATO de context.metaAds.instagram.posts",
		"campanha, conjunto, criativo e anuncio todos PAUSED",
		"CONFIRMAR GASTO META",
	} {
		if !strings.Contains(prompt, required) {
			t.Fatalf("Montar contexto missing %q", required)
		}
	}

	extract := nodes["Extrair resposta"]
	for _, required := range []string{
		"if (k === 'metaaction' || k === 'meta_action') return 'metaAction'",
		"const allowedMetaAdAccounts = new Set",
		"const allowedInstagramPosts = new Set",
		"const metaCampaignAccounts = new Map",
		"if (!metaWrite) continue",
		"allowedMetaAdAccounts.has(metaAction.adAccountId)",
		"metaCampaignAccounts.has(metaAction.campaignId)",
		"allowedInstagramPosts.has(metaAction.instagramPostId || '')",
		"metaAction.adAccountId = metaCampaignAccounts.get(metaAction.campaignId)",
		"proposals.push({ action: 'create', kind, fields: { metaAction } })",
	} {
		if !strings.Contains(extract, required) {
			t.Fatalf("Extrair resposta missing %q", required)
		}
	}
	for _, forbidden := range []string{
		"metaAction.metaAdAccountId",
		"metaAction.metaCampaignId",
	} {
		if strings.Contains(extract, forbidden) {
			t.Fatalf("Extrair resposta must not trust external Meta target %q", forbidden)
		}
	}
}

func TestAssistantWorkflowUsesNativeAnthropicMessagesContract(t *testing.T) {
	t.Parallel()

	path := filepath.Join("..", "..", "..", "..", "automation", "export", "workflow-calendar-chat.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var workflows []assistantWorkflowExport
	if err := json.Unmarshal(raw, &workflows); err != nil {
		t.Fatalf("invalid workflow export: %v", err)
	}
	if len(workflows) != 1 {
		t.Fatalf("expected one exported workflow, got %d", len(workflows))
	}

	serialized := string(raw)
	for _, required := range []string{
		`"name": "Selecionar provedor"`,
		`"rightValue": "anthropic"`,
		`"url": "https://api.anthropic.com/v1/messages"`,
		`"name": "x-api-key"`,
		`"name": "anthropic-version"`,
		`"value": "2023-06-01"`,
		`"name": "Normalizar Anthropic"`,
	} {
		if !strings.Contains(serialized, required) {
			t.Fatalf("Anthropic workflow contract missing %q", required)
		}
	}
	if strings.Contains(serialized, "api.anthropic.com/v1/chat/completions") {
		t.Fatal("Anthropic must use its native Messages API, not an OpenAI-compatible production path")
	}
}
