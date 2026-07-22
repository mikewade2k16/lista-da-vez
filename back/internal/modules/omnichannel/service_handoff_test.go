package omnichannel

import (
	"encoding/json"
	"testing"
)

func TestNormalizeHandoffRequestFailClosed(t *testing.T) {
	in := HandoffRequest{Summary: " resumo ", CollectedFields: json.RawMessage(`{"intent":"orcamento"}`), IdempotencyKey: "handoff-1"}
	if err := normalizeHandoffRequest(&in); err != nil {
		t.Fatalf("valid handoff rejected: %v", err)
	}
	if in.ReasonCode != HandoffReasonRequested || in.Summary != "resumo" || string(in.CollectedFields) != `{"intent":"orcamento"}` {
		t.Fatalf("normalized=%+v", in)
	}
	for _, raw := range []json.RawMessage{json.RawMessage(`[]`), json.RawMessage(`{"secret":"x"}`)} {
		bad := HandoffRequest{CollectedFields: raw, IdempotencyKey: "x"}
		if err := normalizeHandoffRequest(&bad); err == nil {
			t.Fatalf("invalid fields accepted: %s", raw)
		}
	}
	badReason := HandoffRequest{ReasonCode: "invented", IdempotencyKey: "x"}
	if err := normalizeHandoffRequest(&badReason); err == nil {
		t.Fatal("unknown reason accepted")
	}
}

func TestNormalizeAIHandoffReasonDistinguishesModelFromSafetyPolicy(t *testing.T) {
	if got := normalizeAIHandoffReason("model_handoff", false); got != HandoffReasonModel {
		t.Fatalf("reason=%q, want %q", got, HandoffReasonModel)
	}
	if got := normalizeAIHandoffReason("unknown_policy", false); got != HandoffReasonPolicy {
		t.Fatalf("reason=%q, want %q", got, HandoffReasonPolicy)
	}
}
