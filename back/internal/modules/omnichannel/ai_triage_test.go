package omnichannel

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/llm"
)

// TestAIAllowedHardBlock prova o hard-block da IA (C9.6 gate 1): a IA so fala em new/ai_active;
// human_active/pending/queued/routing/closed calam. E o gate 1 do Dispatch.
func TestAIAllowedHardBlock(t *testing.T) {
	allowed := map[State]bool{
		StateNew: true, StateAIActive: true,
		StateHumanActive: false, StatePending: false, StateQueued: false,
		StateRouting: false, StateClosed: false,
	}
	for state, want := range allowed {
		if got := AIAllowed(state); got != want {
			t.Errorf("AIAllowed(%q) = %v, quer %v", state, got, want)
		}
	}
}

// TestDefaultOutputSchemaValidatesContract prova C9.3: a saida no formato do contrato valida;
// campo alucinado / tipo errado sao REJEITADOS (additionalProperties:false + tipos). O schema
// que a F9 embarca e o mesmo que o client LLM (F3) cobra — sem isto, JSON nao-validado vazaria.
func TestDefaultOutputSchemaValidatesContract(t *testing.T) {
	schema := &llm.Schema{Name: "omnichannel_triage", Version: 1, Definition: defaultOutputSchema()}

	valid := `{"intent":"comprar","confidence":0.9,"extracted_fields":{"produto":"x"},
		"suggested_department":"vendas","suggested_queue":null,"needs_human":false,"reply_draft":null}`
	if err := llm.Validate(schema, json.RawMessage(valid)); err != nil {
		t.Fatalf("saida valida foi rejeitada: %v", err)
	}

	// Campo fora do contrato (o modelo alucinou "foo").
	extra := `{"intent":"x","confidence":0.5,"extracted_fields":{},"needs_human":false,"foo":1}`
	if err := llm.Validate(schema, json.RawMessage(extra)); err == nil {
		t.Error("campo extra deveria ser rejeitado (additionalProperties:false)")
	}

	// Tipo errado: confidence como string.
	badType := `{"intent":"x","confidence":"alta","extracted_fields":{},"needs_human":false}`
	if err := llm.Validate(schema, json.RawMessage(badType)); err == nil {
		t.Error("confidence string deveria ser rejeitada (esperado number)")
	}

	// Obrigatorio ausente: sem needs_human.
	missing := `{"intent":"x","confidence":0.5,"extracted_fields":{}}`
	if err := llm.Validate(schema, json.RawMessage(missing)); err == nil {
		t.Error("needs_human ausente deveria ser rejeitado (required)")
	}
}

// TestMaskedTriageInputHasNoPII prova §10: ai_runs.input carrega estrutura (contagem/chaves),
// NUNCA o texto cru das mensagens (telefone, e-mail do cliente).
func TestMaskedTriageInputHasNoPII(t *testing.T) {
	fields := []CollectFieldView{{Key: "produto"}, {Key: "cidade"}}
	raw := maskedTriageInput(fields, 3)

	var got map[string]any
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("input mascarado ilegivel: %v", err)
	}
	if got["messages"].(float64) != 3 {
		t.Errorf("messages = %v, quer 3", got["messages"])
	}
	if s := string(raw); strings.Contains(s, "5199") || strings.Contains(s, "@") {
		t.Errorf("input mascarado nao pode conter PII: %s", s)
	}
	keys, _ := got["collectFields"].([]any)
	if len(keys) != 2 {
		t.Errorf("collectFields = %v, quer 2 chaves", keys)
	}
}

// TestTriageOutputNormalizesNulls prova que suggested_*/reply_draft nulos viram string vazia
// (o motor da F8 le string, nao ponteiro) e extracted_fields nunca e nil.
func TestTriageOutputNormalizesNulls(t *testing.T) {
	raw := `{"intent":"x","confidence":0.7,"extracted_fields":null,
		"suggested_department":null,"suggested_queue":"vendas-sp","needs_human":true,"reply_draft":null}`
	var parsed triageOutputJSON
	if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
		t.Fatalf("parse: %v", err)
	}
	out := parsed.toTriageOutput()
	if out.SuggestedDepartment != "" {
		t.Errorf("suggested_department null deveria virar \"\", veio %q", out.SuggestedDepartment)
	}
	if out.SuggestedQueue != "vendas-sp" {
		t.Errorf("suggested_queue = %q", out.SuggestedQueue)
	}
	if out.ExtractedFields == nil {
		t.Error("extracted_fields nunca pode ser nil (o motor faz range nele)")
	}
	if !out.NeedsHuman {
		t.Error("needs_human deveria ser true")
	}
}

// TestBuildSystemPromptCoversLayers prova C9.2: as camadas server-side (4 catalogo, 5 campos)
// entram do banco (o painel nao pode inventar fila), e as camadas de texto livre caem no
// default quando vazias. O catalogo usa os slugs EXATOS.
func TestBuildSystemPromptCoversLayers(t *testing.T) {
	catalog := []catalogTarget{{DepartmentSlug: "vendas", DepartmentName: "Vendas",
		QueueSlug: "vendas-sp", QueueName: "Vendas SP"}}
	fields := []CollectFieldView{{Key: "produto", FieldType: "text", Required: true, Label: "Produto"}}

	prompt := buildSystemPrompt(promptLayers{}, catalog, fields, "v1")

	for _, want := range []string{"## 1.", "## 4.", "## 5.", "## 8.", "vendas-sp", "produto", "schema v1"} {
		if !strings.Contains(prompt, want) {
			t.Errorf("prompt de sistema nao contem %q", want)
		}
	}
}

// TestBuildUserPromptRendersHistory prova a camada 7: o historico entra no userPrompt com os
// papeis traduzidos (Cliente/Atendente), sem vazar para o system.
func TestBuildUserPromptRendersHistory(t *testing.T) {
	history := []SimMessage{{Role: "contact", Text: "quero um orcamento"}, {Role: "agent", Text: "claro"}}
	user := buildUserPrompt(history, "Maria")
	for _, want := range []string{"Maria", "Cliente: quero um orcamento", "Atendente: claro"} {
		if !strings.Contains(user, want) {
			t.Errorf("user prompt nao contem %q", want)
		}
	}
}
