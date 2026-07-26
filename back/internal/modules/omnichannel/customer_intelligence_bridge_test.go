package omnichannel

import (
	"regexp"
	"testing"
)

func TestDispatchResultFromIntelligenceKeepsDecisionEffectFree(t *testing.T) {
	result, ok := dispatchResultFromIntelligence(CustomerIntelligenceDecision{
		DecisionID:               "decision-1",
		RunID:                    "intelligence-run-1",
		OperationalEffectAllowed: true,
		ProcessRuns: []CustomerIntelligenceProcessRunRef{{
			RunID: "intelligence-run-1", Status: "succeeded", ExecutionMode: "active",
		}},
		Outcome:         "reply",
		ReplyDraft:      "Olá!",
		Confidence:      0.91,
		ExtractedFields: map[string]any{"interest": "produto"},
	}, 7)
	if !ok {
		t.Fatal("decisao valida foi rejeitada")
	}
	if result.Outcome != dispatchTriaged || result.Output.ReplyDraft != "Olá!" {
		t.Fatalf("resultado inesperado: %#v", result)
	}
	if result.AIGeneration != 7 {
		t.Fatalf("generation=%d", result.AIGeneration)
	}
	if result.RunID != "" {
		t.Fatalf("run do schema intelligence nao pode vazar para FK messaging: %q", result.RunID)
	}
}

func TestDispatchResultFromIntelligenceRejectsReplyWithoutDraft(t *testing.T) {
	if _, ok := dispatchResultFromIntelligence(CustomerIntelligenceDecision{
		OperationalEffectAllowed: true,
		ProcessRuns: []CustomerIntelligenceProcessRunRef{{
			RunID: "run-1", Status: "succeeded", ExecutionMode: "active",
		}},
		Outcome: "reply",
	}, 1); ok {
		t.Fatal("reply sem texto deveria cair no fallback legado")
	}
}

func TestDispatchResultFromIntelligenceMapsHandoffAndClose(t *testing.T) {
	handoff, ok := dispatchResultFromIntelligence(CustomerIntelligenceDecision{
		OperationalEffectAllowed: true,
		ProcessRuns: []CustomerIntelligenceProcessRunRef{{
			RunID: "run-1", Status: "succeeded", ExecutionMode: "active",
		}},
		Outcome:       "handoff",
		HandoffReason: "human_requested",
	}, 1)
	if !ok || !handoff.Output.NeedsHuman {
		t.Fatalf("handoff invalido: %#v", handoff)
	}

	closeResult, ok := dispatchResultFromIntelligence(CustomerIntelligenceDecision{
		OperationalEffectAllowed: true,
		ProcessRuns: []CustomerIntelligenceProcessRunRef{{
			RunID: "run-2", Status: "succeeded", ExecutionMode: "active",
		}},
		Outcome:     "close",
		ReplyDraft:  "Posso ajudar em algo mais?",
		CloseReason: "resolved",
	}, 2)
	if !ok || !closeResult.Output.CloseRequested {
		t.Fatalf("close invalido: %#v", closeResult)
	}
}

func TestDispatchResultFromIntelligenceRejectsNonOperationalModes(t *testing.T) {
	for _, mode := range []string{"shadow", "canary", ""} {
		t.Run(mode, func(t *testing.T) {
			decision := CustomerIntelligenceDecision{
				OperationalEffectAllowed: true,
				ProcessRuns: []CustomerIntelligenceProcessRunRef{{
					RunID: "run-1", Status: "succeeded", ExecutionMode: mode,
				}},
				Outcome: "reply", ReplyDraft: "nao enviar",
			}
			if customerIntelligenceDecisionAllowsOperationalEffect(decision) {
				t.Fatalf("modo %q autorizou efeito", mode)
			}
			if _, ok := dispatchResultFromIntelligence(decision, 1); ok {
				t.Fatalf("modo %q foi convertido em efeito operacional", mode)
			}
		})
	}
}

func TestDispatchResultFromIntelligenceRequiresTrustedAdapterAuthorization(t *testing.T) {
	decision := CustomerIntelligenceDecision{
		ProcessRuns: []CustomerIntelligenceProcessRunRef{{
			RunID: "run-1", Status: "succeeded", ExecutionMode: "active",
		}},
		Outcome: "reply", ReplyDraft: "nao enviar",
	}
	if _, ok := dispatchResultFromIntelligence(decision, 1); ok {
		t.Fatal("decisao sem autorizacao do adapter produziu efeito")
	}
}

func TestDeterministicUUIDIsStableAndValid(t *testing.T) {
	first := deterministicUUID("dispatch:7:reply")
	second := deterministicUUID("dispatch:7:reply")
	if first != second {
		t.Fatalf("uuid nao deterministico: %q != %q", first, second)
	}
	pattern := regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-5[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)
	if !pattern.MatchString(first) {
		t.Fatalf("uuid invalido: %q", first)
	}
}
