package omnichannel

import (
	"encoding/json"
	"testing"
)

func TestVersionMatchesInputIgnoresJSONFormatting(t *testing.T) {
	enabled := true
	in := AIVersionInput{
		Provider: "openai", Model: "gpt-4.1-mini", Temperature: 0.2,
		MediaConfig: json.RawMessage(`{"image":{"enabled":true}}`), SchemaVersion: "v1",
		DebounceMS: 2500, MaxContextMessages: 30, MaxAITurns: 6, MinConfidence: float64Pointer(0.65),
		HandoffOnError: &enabled, HandoffOnLimit: &enabled, WorkflowContract: "brain.v3",
	}
	version := versionRow{
		Provider: "openai", Model: "gpt-4.1-mini", Temperature: 0.2,
		Layers:       json.RawMessage(`{"identity":"Atenda bem","tone":"claro"}`),
		OutputSchema: json.RawMessage(`{"type":"object"}`),
		MediaConfig:  json.RawMessage(`{"image": {"enabled": true}}`), SchemaVersion: "v1",
		DebounceMS: 2500, MaxContextMessages: 30, MaxAITurns: 6, MinConfidence: 0.65,
		HandoffOnError: true, HandoffOnLimit: true, WorkflowContract: "brain.v3",
	}
	if !versionMatchesInput(version, in, json.RawMessage(`{"type": "object"}`),
		json.RawMessage(`{"tone":"claro","identity":"Atenda bem"}`)) {
		t.Fatal("equivalent JSON configuration must be idempotent")
	}
	version.Model = "gpt-4.1"
	if versionMatchesInput(version, in, json.RawMessage(`{"type":"object"}`),
		json.RawMessage(`{"identity":"Atenda bem","tone":"claro"}`)) {
		t.Fatal("different model must create a new active version")
	}
}

func TestNormalizeVersionInputAppliesRuntimeDefaults(t *testing.T) {
	in, schema, layers, err := normalizeVersionInput(AIVersionInput{
		Provider: "openai", Model: "gpt-4.1-mini", Layers: json.RawMessage(`{"identity":"Oi"}`),
	})
	if err != nil {
		t.Fatal(err)
	}
	if in.DebounceMS != 2500 || in.MaxContextMessages != 30 || in.MaxAITurns != 0 ||
		in.MinConfidence == nil || *in.MinConfidence != 0.65 || in.WorkflowContract != "brain.v2" {
		t.Fatalf("unexpected defaults: %+v", in)
	}
	if len(schema) == 0 || string(layers) != `{"identity":"Oi"}` {
		t.Fatal("schema and prompt layers must be preserved")
	}
}

func TestNormalizeVersionInputPreservesZeroConfidence(t *testing.T) {
	in, _, _, err := normalizeVersionInput(AIVersionInput{
		Provider: "openai", Model: "gpt-test", MinConfidence: float64Pointer(0),
	})
	if err != nil {
		t.Fatalf("normalize: %v", err)
	}
	if in.MinConfidence == nil || *in.MinConfidence != 0 {
		t.Fatalf("min confidence=%v", in.MinConfidence)
	}
}

func TestAITurnLimitZeroMeansUnlimited(t *testing.T) {
	if aiTurnLimitReached(false, 0, 10_000) {
		t.Fatal("zero max turns must keep automatic replies unlimited")
	}
	if !aiTurnLimitReached(false, 3, 3) {
		t.Fatal("positive max turns must still enforce the configured limit")
	}
}

func float64Pointer(value float64) *float64 { return &value }
