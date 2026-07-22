package omnichannel

import (
	"encoding/json"
	"testing"
)

func TestMediaAnalysisPolicyFor(t *testing.T) {
	config := json.RawMessage(`{"audio":{"enabled":true,"provider":"openai","model":"whisper","maxSeconds":120},"image":{"enabled":true,"provider":"openai","model":"vision","maxBytes":1000},"document":{"enabled":true,"provider":"openai","model":"pdf","allowedMime":["application/pdf"],"maxPages":5}}`)
	policy, enabled, err := mediaAnalysisPolicyFor("AUDIO", "audio/ogg", config)
	if err != nil || !enabled || policy.Kind != MediaAnalysisKindTranscription || policy.MaxSeconds != 120 {
		t.Fatalf("audio policy=%+v enabled=%v err=%v", policy, enabled, err)
	}
	_, enabled, err = mediaAnalysisPolicyFor("DOCUMENT", "text/plain", config)
	if err == nil || enabled {
		t.Fatalf("unsupported document MIME accepted: enabled=%v err=%v", enabled, err)
	}
	_, enabled, err = mediaAnalysisPolicyFor("TEXT", "text/plain", config)
	if err != nil || enabled {
		t.Fatalf("text unexpectedly analyzed: enabled=%v err=%v", enabled, err)
	}
}
