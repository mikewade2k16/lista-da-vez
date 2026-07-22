package omnichannel

import (
	"testing"
)

func TestNormalizeHandoffPolicyConditions(t *testing.T) {
	conditions, err := normalizeHandoffPolicyConditions(map[string]any{
		"reasonCode":    "low_confidence",
		"intent":        []any{"sales", "support"},
		"confidenceMax": 0.65,
		"hourUtc":       map[string]any{"from": 8, "to": 18},
	})
	if err != nil {
		t.Fatalf("conditions válidas rejeitadas: %v", err)
	}
	if conditions["reasonCode"] != "low_confidence" {
		t.Fatalf("reasonCode normalizado inesperado: %#v", conditions["reasonCode"])
	}
	if _, err := normalizeHandoffPolicyConditions(map[string]any{"unknown": "x"}); err == nil {
		t.Fatal("chave desconhecida deve falhar fechado")
	}
	if _, err := normalizeHandoffPolicyConditions(map[string]any{"confidenceMax": 1.5}); err == nil {
		t.Fatal("limite de confiança fora de 0..1 deve falhar")
	}
}

func TestHandoffPolicyMatchesDeterministically(t *testing.T) {
	ctx := handoffPolicyContext{
		Values: map[string]any{
			"reasonCode":  "low_confidence",
			"sourceState": "ai_active",
			"intent":      "sales",
			"confidence":  0.42,
		},
		Tags:    []string{"vip", "priority"},
		HourUTC: 10,
	}
	conditions, err := normalizeHandoffPolicyConditions(map[string]any{
		"reasonCode":    "low_confidence",
		"sourceState":   "ai_active",
		"intent":        []any{"sales", "billing"},
		"confidenceMax": 0.5,
		"tag":           "vip",
		"hourUtc":       map[string]any{"from": 8, "to": 18},
	})
	if err != nil || !policyMatches(conditions, ctx) {
		t.Fatalf("policy deveria casar: conditions=%#v err=%v", conditions, err)
	}
	ctx.HourUTC = 23
	if policyMatches(conditions, ctx) {
		t.Fatal("policy fora da janela deveria não casar")
	}
}

func TestHandoffPolicyOvernightWindow(t *testing.T) {
	conditions, err := normalizeHandoffPolicyConditions(map[string]any{
		"hourUtc": map[string]any{"from": 22, "to": 3},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !policyMatches(conditions, handoffPolicyContext{Values: map[string]any{}, HourUTC: 23}) ||
		!policyMatches(conditions, handoffPolicyContext{Values: map[string]any{}, HourUTC: 2}) ||
		policyMatches(conditions, handoffPolicyContext{Values: map[string]any{}, HourUTC: 12}) {
		t.Fatal("janela que atravessa meia-noite foi avaliada incorretamente")
	}
}
