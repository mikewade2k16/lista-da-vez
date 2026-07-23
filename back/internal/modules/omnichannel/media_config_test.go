package omnichannel

import "testing"

func TestNormalizeMediaConfig(t *testing.T) {
	tests := []struct {
		name string
		json string
		ok   bool
	}{
		{"empty defaults", `{}`, true},
		{"valid sections", `{"audio":{"enabled":true,"provider":"openai","model":"whisper-1","maxSeconds":600},"image":{"enabled":true,"provider":"openai","model":"vision","maxBytes":5242880},"document":{"enabled":true,"provider":"openai","model":"pdf","allowedMime":["application/pdf"],"maxPages":20},"retentionDays":90,"includeInReply":true}`, true},
		{"unknown field", `{"audio":{"apiKey":"do-not-store"}}`, false},
		{"video requires provider and model", `{"video":{"enabled":true}}`, false},
		{"valid video", `{"video":{"enabled":true,"provider":"gemini","model":"gemini-2.5-flash","credentialId":"00000000-0000-4000-8000-000000000001","maxBytes":104857600}}`, true},
		{"audio max too high", `{"audio":{"maxSeconds":601}}`, false},
		{"image max too high", `{"image":{"maxBytes":62914561}}`, false},
		{"document mime invalid", `{"document":{"allowedMime":["pdf"]}}`, false},
		{"enabled without model", `{"audio":{"enabled":true,"provider":"openai"}}`, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := normalizeMediaConfig([]byte(tt.json))
			if (err == nil) != tt.ok {
				t.Fatalf("normalizeMediaConfig() error=%v, want ok=%v", err, tt.ok)
			}
			if tt.ok && len(got) == 0 {
				t.Fatal("valid config returned empty bytes")
			}
		})
	}
}
