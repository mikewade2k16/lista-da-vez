package omnichannel

import (
	"context"
	"errors"
	"testing"

	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/jobs"
)

func TestCustomerIntelligenceFailurePolicyLegacyFallback(t *testing.T) {
	handler := aiDispatchHandler{}
	called := false
	want := DispatchResult{Outcome: dispatchNoReply}
	got, decision, err := handler.resolveCustomerIntelligenceFailure(
		jobs.Job{Attempts: 1, MaxAttempts: 3},
		7,
		"legacy_fallback",
		NewCustomerIntelligenceFailure(
			"temporarily_unavailable", "provider_unavailable", true,
			context.DeadlineExceeded,
		),
		func() (DispatchResult, *CustomerIntelligenceDecision, error) {
			called = true
			return want, nil, nil
		},
	)
	if err != nil || decision != nil || !called || got.Outcome != want.Outcome {
		t.Fatalf(
			"fallback inesperado: got=%#v decision=%#v err=%v called=%v",
			got, decision, err, called,
		)
	}
}

func TestCustomerIntelligenceFailurePolicyRetriesThenHandsOff(t *testing.T) {
	handler := aiDispatchHandler{}
	failure := NewCustomerIntelligenceFailure(
		"temporarily_unavailable", "provider_unavailable", true,
		context.DeadlineExceeded,
	)
	_, _, err := handler.resolveCustomerIntelligenceFailure(
		jobs.Job{Attempts: 1, MaxAttempts: 3},
		7,
		"retry_then_handoff",
		failure,
		func() (DispatchResult, *CustomerIntelligenceDecision, error) {
			t.Fatal("legacy não deveria ser chamado")
			return DispatchResult{}, nil, nil
		},
	)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("primeira tentativa deveria retornar falha retryable: %v", err)
	}

	got, decision, err := handler.resolveCustomerIntelligenceFailure(
		jobs.Job{Attempts: 3, MaxAttempts: 3},
		7,
		"retry_then_handoff",
		failure,
		func() (DispatchResult, *CustomerIntelligenceDecision, error) {
			t.Fatal("legacy não deveria ser chamado")
			return DispatchResult{}, nil, nil
		},
	)
	if err != nil || decision != nil ||
		got.Outcome != dispatchTriaged ||
		!got.Output.NeedsHuman ||
		got.Output.HandoffReason != HandoffReasonError {
		t.Fatalf("falha terminal deveria fazer handoff: %#v, %#v, %v", got, decision, err)
	}
}

func TestCustomerIntelligencePermanentFailureHandsOffImmediately(t *testing.T) {
	handler := aiDispatchHandler{}
	got, _, err := handler.resolveCustomerIntelligenceFailure(
		jobs.Job{Attempts: 1, MaxAttempts: 3},
		11,
		"retry_then_handoff",
		NewCustomerIntelligenceFailure(
			"permanent_failure", "credential_not_configured", false,
			ErrValidation,
		),
		func() (DispatchResult, *CustomerIntelligenceDecision, error) {
			t.Fatal("legacy não deveria ser chamado")
			return DispatchResult{}, nil, nil
		},
	)
	if err != nil || got.AIGeneration != 11 ||
		got.Outcome != dispatchTriaged || !got.Output.NeedsHuman {
		t.Fatalf("falha permanente deveria transferir: %#v, %v", got, err)
	}
}
