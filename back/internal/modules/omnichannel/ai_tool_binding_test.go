package omnichannel

import (
	"encoding/json"
	"testing"
)

func TestNormalizeAIToolBindingInputDefaultsAndRedactsSecrets(t *testing.T) {
	in := AIToolBindingInput{ToolID: " catalog.search "}
	if err := normalizeAIToolBindingInput(&in); err != nil {
		t.Fatalf("input mínimo deveria usar defaults: %v", err)
	}
	if in.ToolID != "catalog.search" || in.Mode != "read" || in.TimeoutMS != 5000 || in.MaxCallsPerDispatch != 4 {
		t.Fatalf("defaults inesperados: %#v", in)
	}
	if string(jsonArray(in.AllowedOperations)) != "[]" {
		t.Fatalf("operações ausentes devem serializar como array vazio: %s", jsonArray(in.AllowedOperations))
	}

	secret := json.RawMessage(`{"apiKey":"nao-persistir"}`)
	in.InputSchema = secret
	if err := normalizeAIToolBindingInput(&in); err == nil {
		t.Fatal("schema com marcador de segredo deve ser rejeitado")
	}
}

func TestNormalizeAIToolBindingPatchValidatesJSONAndLimits(t *testing.T) {
	valid := json.RawMessage(`{"query":{"type":"string"}}`)
	patch := AIToolBindingPatch{InputSchema: &valid}
	if err := normalizeAIToolBindingPatch(&patch); err != nil {
		t.Fatalf("patch válido rejeitado: %v", err)
	}

	bad := json.RawMessage(`[]`)
	patch = AIToolBindingPatch{Config: &bad}
	if err := normalizeAIToolBindingPatch(&patch); err == nil {
		t.Fatal("config que não é objeto deve ser rejeitada")
	}

	patch = AIToolBindingPatch{Mode: stringPtrAIToolMode("unsafe")}
	if err := normalizeAIToolBindingPatch(&patch); err == nil {
		t.Fatal("modo desconhecido deve ser rejeitado")
	}
}

func stringPtrAIToolMode(value string) *string { return &value }
