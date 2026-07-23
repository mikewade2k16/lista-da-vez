package omnichannel

import "testing"

func TestOperatorForcedReplyBypassesConversationalPolicy(t *testing.T) {
	if aiTurnLimitReached(true, 6, 6) {
		t.Fatal("operator reply must bypass the configured turn limit for one response")
	}
	if confidenceBelowReplyMinimum(true, 0.20, 0.65) {
		t.Fatal("operator reply must bypass minimum confidence for one response")
	}
	if !aiTurnLimitReached(false, 6, 6) || !confidenceBelowReplyMinimum(false, 0.20, 0.65) {
		t.Fatal("automatic dispatch must keep enforcing turn and confidence limits")
	}
}

func TestApplyOperatorForcedReplyOverridesModelPolicy(t *testing.T) {
	result := DispatchResult{Outcome: dispatchTriaged, Output: TriageOutput{
		ReplyDraft: "Resposta segura", NeedsHuman: true, HumanRequested: true,
		SensitiveTopic: true, CloseRequested: true, CloseReason: "done",
		HandoffReason: "model_handoff", HandoffSummary: "handoff",
	}}

	result = applyOperatorForcedReply(result)
	if result.Output.NeedsHuman || result.Output.CloseRequested {
		t.Fatal("operator command must produce the requested reply instead of another automatic stop")
	}
	if result.Output.ReplyDraft != "Resposta segura" || result.Output.HandoffReason != "" || result.Output.CloseReason != "" {
		t.Fatalf("unexpected forced result: %+v", result.Output)
	}
}

func TestNormalizedAutomationAttendanceReasonRecoversMaxTurns(t *testing.T) {
	reason, summary := normalizedAutomationAttendanceReason(automationAttendanceRow{
		ReasonCode:  HandoffReasonLowConfidence,
		Summary:     "A IA interrompeu o atendimento por baixa confianca.",
		AIRunStatus: runLimitExceeded,
		AIRunError:  "max_ai_turns",
	})
	if reason != HandoffReasonMaxTurns || summary != defaultAIHandoffSummary(HandoffReasonMaxTurns) {
		t.Fatalf("reason=%q summary=%q", reason, summary)
	}
}

func TestAutomationReplyStateAllowsHumanTakeoverReturn(t *testing.T) {
	if !automationReplyStateAllowed(StateHumanActive) {
		t.Fatal("human_active must be eligible for an explicit operator transfer back to AI")
	}
	if automationReplyStateAllowed(StateAIActive) || automationReplyStateAllowed(StateClosed) {
		t.Fatal("ai_active and closed must remain ineligible for immediate replay")
	}
}

func TestHandoffReasonForResultPreservesSpecificLimit(t *testing.T) {
	result := DispatchResult{Outcome: dispatchLimitExceeded, ReasonCode: HandoffReasonMaxTurns}
	if got := handoffReasonForResult(result); got != HandoffReasonMaxTurns {
		t.Fatalf("reason=%q", got)
	}
}
