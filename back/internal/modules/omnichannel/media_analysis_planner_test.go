package omnichannel

import (
	"encoding/json"
	"testing"
)

const plannerMessageID = "00000000-0000-0000-0000-000000000001"
const plannerConversationID = "00000000-0000-0000-0000-000000000002"
const plannerVersionID = "00000000-0000-0000-0000-000000000003"
const plannerHash = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

func TestPlanMediaAnalysisIsDisabledByDefaultConfig(t *testing.T) {
	plan, enabled, err := planMediaAnalysis(mediaAnalysisPlanInput{
		MessageID: plannerMessageID, ConversationID: plannerConversationID, MessageType: "IMAGE",
		MimeType: "image/png", SizeBytes: 10, ContentHash: plannerHash,
		AgentVersionID: plannerVersionID, MediaConfig: json.RawMessage(`{}`),
	})
	if err != nil || enabled || plan.Blocked {
		t.Fatalf("disabled plan=%+v enabled=%v err=%v", plan, enabled, err)
	}
}

func TestPlanMediaAnalysisRejectsUnsupportedAndTooLargeMedia(t *testing.T) {
	config := json.RawMessage(`{"image":{"enabled":true,"provider":"openai","model":"vision","maxBytes":100}}`)
	_, _, err := planMediaAnalysis(mediaAnalysisPlanInput{
		MessageID: plannerMessageID, ConversationID: plannerConversationID, MessageType: "IMAGE",
		MimeType: "application/pdf", SizeBytes: 10, ContentHash: plannerHash,
		AgentVersionID: plannerVersionID, MediaConfig: config,
	})
	if err == nil {
		t.Fatal("unsupported MIME accepted by image policy")
	}
	plan, enabled, err := planMediaAnalysis(mediaAnalysisPlanInput{
		MessageID: plannerMessageID, ConversationID: plannerConversationID, MessageType: "IMAGE",
		MimeType: "image/png", SizeBytes: 101, ContentHash: plannerHash,
		AgentVersionID: plannerVersionID, MediaConfig: config,
	})
	if err != nil || !enabled || !plan.Blocked || plan.Code != "media_too_large" {
		t.Fatalf("large plan=%+v enabled=%v err=%v", plan, enabled, err)
	}
}

func TestPlanMediaAnalysisBuildsStableDedupeInput(t *testing.T) {
	config := json.RawMessage(`{"audio":{"enabled":true,"provider":"openai","model":"whisper","maxSeconds":120}}`)
	plan, enabled, err := planMediaAnalysis(mediaAnalysisPlanInput{
		MessageID: plannerMessageID, ConversationID: plannerConversationID, MessageType: "AUDIO",
		MimeType: "audio/ogg", SizeBytes: 512, ContentHash: plannerHash,
		AgentVersionID: plannerVersionID, MediaConfig: config,
	})
	if err != nil || !enabled || plan.Blocked {
		t.Fatalf("audio plan=%+v enabled=%v err=%v", plan, enabled, err)
	}
	if plan.Create.Kind != MediaAnalysisKindTranscription || plan.Create.Provider != "openai" || plan.Create.Model != "whisper" || plan.Create.ContentHash != plannerHash {
		t.Fatalf("dedupe input=%+v", plan.Create)
	}
	plan2, enabled2, err := planMediaAnalysis(mediaAnalysisPlanInput{
		MessageID: plannerMessageID, ConversationID: plannerConversationID, MessageType: "AUDIO",
		MimeType: "audio/ogg", SizeBytes: 512, ContentHash: "ABCDEF0123456789abcdef0123456789abcdef0123456789abcdef0123456789",
		AgentVersionID: plannerVersionID, MediaConfig: config,
	})
	if err != nil || !enabled2 || plan2.Create.ContentHash == plan.Create.ContentHash {
		t.Fatalf("different content hash collapsed: first=%+v second=%+v err=%v", plan.Create, plan2.Create, err)
	}
}
