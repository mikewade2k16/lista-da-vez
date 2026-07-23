package omnichannel

import (
	"encoding/json"
	"errors"
	"testing"
)

func brainResultFixture(t *testing.T, decision string, reply *string) BrainResultV2 {
	t.Helper()
	result := BrainResultV2{
		SchemaVersion: "brain.result.v2", DispatchID: "dispatch-1", Generation: 1,
		Decision:        BrainDecision(decision),
		Classification:  BrainClassificationV2{Intent: "sales", Confidence: 0.9, Sentiment: "neutral"},
		ExtractedFields: map[string]any{},
		Handoff:         BrainHandoffV2{}, Usage: BrainUsageV2{Provider: "provider", Model: "model"},
		Trace: BrainTraceV2{},
	}
	if reply != nil {
		result.Reply = &BrainReplyV2{Text: reply}
	}
	return result
}

func TestDecodeBrainResultV2RejectsUnknownFields(t *testing.T) {
	raw := []byte(`{"schemaVersion":"brain.result.v2","dispatchId":"d","generation":1,"decision":"no_reply","reply":null,"classification":{"intent":"x","confidence":0.8,"sentiment":"neutral"},"extractedFields":{},"suggestedRouting":null,"handoff":{"needed":false,"reasonCode":null,"summary":null},"usage":{"provider":"p","model":"m","promptTokens":0,"completionTokens":0,"cost":0},"trace":{"toolCalls":[],"warnings":[]},"channelSend":true}`)
	if _, err := DecodeBrainResultV2(raw); !errors.Is(err, ErrBrainSchemaInvalid) {
		t.Fatalf("err=%v, want schema invalid", err)
	}
}

func TestApplyBrainPolicyConfidenceAndTurnLimits(t *testing.T) {
	reply := "resposta"
	result := brainResultFixture(t, string(BrainContinueAI), &reply)
	cfg := DefaultBrainPolicyConfig()
	cfg.MinConfidence = 0.95
	out, err := ApplyBrainPolicy(result, cfg, 0)
	if err != nil || out.ShouldSend || out.Decision != BrainHandoff || out.ReasonCode != "low_confidence" {
		t.Fatalf("low confidence outcome=%+v err=%v", out, err)
	}
	cfg.MinConfidence = 0.5
	cfg.MaxAITurns = 6
	out, err = ApplyBrainPolicy(result, cfg, cfg.MaxAITurns)
	if err != nil || out.ShouldSend || out.Decision != BrainHandoff || out.ReasonCode != "max_ai_turns" {
		t.Fatalf("turn limit outcome=%+v err=%v", out, err)
	}
}

func TestApplyBrainPolicyUnlimitedTurns(t *testing.T) {
	reply := "resposta"
	result := brainResultFixture(t, string(BrainContinueAI), &reply)
	cfg := DefaultBrainPolicyConfig()
	cfg.MinConfidence = 0.5
	out, err := ApplyBrainPolicy(result, cfg, 10_000)
	if err != nil || !out.ShouldSend || out.ShouldHandoff || out.ReasonCode == "max_ai_turns" {
		t.Fatalf("unlimited outcome=%+v err=%v", out, err)
	}
}

func TestApplyBrainPolicyDoesNotSendNoReplyOrHandoff(t *testing.T) {
	for _, decision := range []BrainDecision{BrainNoReply, BrainHandoff} {
		result := brainResultFixture(t, string(decision), nil)
		out, err := ApplyBrainPolicy(result, DefaultBrainPolicyConfig(), 0)
		if err != nil || out.ShouldSend || out.ShouldHandoff != (decision == BrainHandoff) {
			t.Fatalf("decision=%s outcome=%+v err=%v", decision, out, err)
		}
	}
}

func TestDecodeBrainResultV2RoundTrip(t *testing.T) {
	reply := "ok"
	raw, err := json.Marshal(brainResultFixture(t, string(BrainContinueAI), &reply))
	if err != nil {
		t.Fatal(err)
	}
	got, err := DecodeBrainResultV2(raw)
	if err != nil || got.Decision != BrainContinueAI || got.Reply == nil || *got.Reply.Text != reply {
		t.Fatalf("got=%+v err=%v", got, err)
	}
}

func TestDecodeBrainResultV3AcceptsCloseProposal(t *testing.T) {
	reply := "Obrigado pelo contato."
	result := brainResultFixture(t, string(BrainClose), &reply)
	result.SchemaVersion = "brain.result.v3"
	result.Closure = &BrainClosureV3{Requested: true, Reason: ptrString("resolved")}
	raw, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	got, err := DecodeBrainResultV2(raw)
	if err != nil || got.Decision != BrainClose || got.Closure == nil || !got.Closure.Requested {
		t.Fatalf("got=%+v err=%v", got, err)
	}
	out, err := ApplyBrainPolicy(got, DefaultBrainPolicyConfig(), 0)
	if err != nil || !out.ShouldClose || !out.ShouldSend {
		t.Fatalf("out=%+v err=%v", out, err)
	}
}

func TestDecodeBrainResultV2RejectsCloseDecision(t *testing.T) {
	result := brainResultFixture(t, string(BrainClose), nil)
	raw, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := DecodeBrainResultV2(raw); !errors.Is(err, ErrBrainSchemaInvalid) {
		t.Fatalf("err=%v, want schema invalid", err)
	}
}

func TestDecodeBrainResultV3RejectsCloseWithoutFinalReply(t *testing.T) {
	result := brainResultFixture(t, string(BrainClose), nil)
	result.SchemaVersion = "brain.result.v3"
	result.Closure = &BrainClosureV3{Requested: true}
	raw, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := DecodeBrainResultV2(raw); !errors.Is(err, ErrBrainSchemaInvalid) {
		t.Fatalf("err=%v, want schema invalid", err)
	}
}

func ptrString(value string) *string { return &value }
