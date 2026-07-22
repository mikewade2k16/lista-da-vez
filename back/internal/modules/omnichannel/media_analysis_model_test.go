package omnichannel

import "testing"

func TestValidateMediaAnalysisCreate(t *testing.T) {
	valid := mediaAnalysisCreate{
		MessageID: "00000000-0000-0000-0000-000000000001", ConversationID: "00000000-0000-0000-0000-000000000002",
		Kind: MediaAnalysisKindVision, ContentHash: "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef", AgentVersionID: "00000000-0000-0000-0000-000000000003",
	}
	if err := validateMediaAnalysisCreate(valid); err != nil {
		t.Fatalf("valid input rejected: %v", err)
	}
	valid.ContentHash = "not-a-hash"
	if err := validateMediaAnalysisCreate(valid); err == nil {
		t.Fatal("invalid hash accepted")
	}
}

func TestValidateMediaAnalysisResult(t *testing.T) {
	text := "transcribed"
	valid := mediaAnalysisComplete{ResultText: &text, ResultJSON: []byte(`{"text":"transcribed"}`), PromptTokens: 2, CompletionTokens: 3, LatencyMS: 20}
	if err := validateMediaAnalysisResult(valid); err != nil {
		t.Fatalf("valid result rejected: %v", err)
	}
	valid.ResultJSON = []byte(`[]`)
	if err := validateMediaAnalysisResult(valid); err == nil {
		t.Fatal("array result accepted")
	}
}

func TestValidateMediaAnalysisShape(t *testing.T) {
	if err := validateMediaAnalysisShape(MediaAnalysisKindDocument, []byte(`{"summary":"ok","extractedText":"x","pageCount":1,"truncated":false,"warnings":[]}`)); err != nil {
		t.Fatalf("valid document rejected: %v", err)
	}
	if err := validateMediaAnalysisShape(MediaAnalysisKindDocument, []byte(`{"summary":"ok","extractedText":"x","pageCount":1.5,"truncated":false,"warnings":[]}`)); err == nil {
		t.Fatal("fractional page count accepted")
	}
	if err := validateMediaAnalysisShape(MediaAnalysisKindVision, []byte(`{"summary":"ok","visibleText":"","objects":[],"safetyFlags":[],"unexpected":true}`)); err == nil {
		t.Fatal("unknown result field accepted")
	}
}
