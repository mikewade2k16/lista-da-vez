package omnichannel

import (
	"reflect"
	"testing"
)

func enabledAutoClosePolicy() AutoCloseRuntimePolicy {
	return AutoCloseRuntimePolicy{
		Found: true, ProfileEnabled: true, AutoCloseEnabled: true,
		MinimumConfidence: 0.9, RequireAllRequiredFields: true,
		BlockOnHumanRequest: true, BlockSensitiveTopics: true,
		RequiredFields: []string{"name", "accepted", "quantity"},
	}
}

func TestEvaluateAutoCloseAcceptsOnlyWhenEveryGatePasses(t *testing.T) {
	eval := EvaluateAutoClose(enabledAutoClosePolicy(), AutoCloseProposal{Requested: true, Confidence: 0.95},
		map[string]any{"name": "Ana", "accepted": false, "quantity": 0}, 4, 4)
	if !eval.Accepted || len(eval.ReasonCodes) != 0 {
		t.Fatalf("expected accepted evaluation, got %#v", eval)
	}
}

func TestEvaluateAutoCloseReportsAllConfiguredBlockers(t *testing.T) {
	eval := EvaluateAutoClose(enabledAutoClosePolicy(), AutoCloseProposal{
		Requested: true, Confidence: 0.4, HumanRequested: true, SensitiveTopic: true,
	}, map[string]any{"name": " "}, 3, 4)
	wantReasons := []string{
		autoCloseReasonLowConfidence, autoCloseReasonRequiredFields,
		autoCloseReasonHumanRequested, autoCloseReasonSensitiveTopic,
		autoCloseReasonStaleGeneration,
	}
	if eval.Accepted || !reflect.DeepEqual(eval.ReasonCodes, wantReasons) {
		t.Fatalf("unexpected reasons: %#v", eval)
	}
	wantMissing := []string{"accepted", "name", "quantity"}
	if !reflect.DeepEqual(eval.MissingFields, wantMissing) {
		t.Fatalf("unexpected missing fields: %#v", eval.MissingFields)
	}
}

func TestEvaluateAutoCloseConfigurableGatesCanBeDisabledButLeaseCannot(t *testing.T) {
	policy := enabledAutoClosePolicy()
	policy.RequireAllRequiredFields = false
	policy.BlockOnHumanRequest = false
	policy.BlockSensitiveTopics = false
	eval := EvaluateAutoClose(policy, AutoCloseProposal{
		Requested: true, Confidence: 0.95, HumanRequested: true, SensitiveTopic: true,
	}, map[string]any{}, 7, 7)
	if !eval.Accepted {
		t.Fatalf("expected configurable gates to permit close, got %#v", eval)
	}
	eval = EvaluateAutoClose(policy, AutoCloseProposal{Requested: true, Confidence: 0.95}, map[string]any{}, 7, 8)
	if eval.Accepted || !reflect.DeepEqual(eval.ReasonCodes, []string{autoCloseReasonStaleGeneration}) {
		t.Fatalf("generation lease must remain mandatory, got %#v", eval)
	}
}

func TestEvaluateAutoCloseRequiresExplicitProposalAndEnabledProfile(t *testing.T) {
	policy := enabledAutoClosePolicy()
	policy.Found = false
	eval := EvaluateAutoClose(policy, AutoCloseProposal{}, nil, 1, 1)
	want := []string{autoCloseReasonNotRequested, autoCloseReasonProfileMissing}
	if eval.Accepted || !reflect.DeepEqual(eval.ReasonCodes, want) {
		t.Fatalf("unexpected evaluation: %#v", eval)
	}
}
