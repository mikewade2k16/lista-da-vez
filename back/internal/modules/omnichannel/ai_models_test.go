package omnichannel

import (
	"context"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
)

func TestFilterAgentChatModelsKeepsOnlyChatModels(t *testing.T) {
	got := filterAgentChatModels("openai", []string{
		"text-embedding-3-small",
		"gpt-4o-mini",
		"gpt-4o-mini",
		"gpt-4o-audio-preview",
		"o3-mini",
	})
	want := []string{"gpt-4o-mini", "o3-mini"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected models: got %#v want %#v", got, want)
	}
}

func TestFetchProviderModelIDsUsesAnthropicHeaders(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/models" || r.Header.Get("x-api-key") != "secret" ||
			r.Header.Get("anthropic-version") != "2023-06-01" || r.Header.Get("Authorization") != "" {
			t.Errorf("unexpected request path=%q headers=%v", r.URL.Path, r.Header)
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		_, _ = w.Write([]byte(`{"data":[{"id":"claude-sonnet-4-6"}]}`))
	}))
	defer server.Close()

	got, err := fetchProviderModelIDs(context.Background(), server.URL, "secret", "anthropic")
	if err != nil || !reflect.DeepEqual(got, []string{"claude-sonnet-4-6"}) {
		t.Fatalf("models=%#v err=%v", got, err)
	}
}

func TestFilterAgentChatModelsKeepsAnthropicClaudeModels(t *testing.T) {
	got := filterAgentChatModels("anthropic", []string{"claude-sonnet-4-6", "embedding-model"})
	if !reflect.DeepEqual(got, []string{"claude-sonnet-4-6"}) {
		t.Fatalf("unexpected Anthropic models: %#v", got)
	}
}

func TestFilterAgentChatModelsNormalizesGeminiPrefix(t *testing.T) {
	got := filterAgentChatModels("gemini", []string{"models/gemini-2.5-flash", "models/text-embedding-004"})
	want := []string{"gemini-2.5-flash"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected models: got %#v want %#v", got, want)
	}
}

func TestFilterAgentModelsByMediaCapability(t *testing.T) {
	openAI := []string{"gpt-4o", "whisper-1", "gpt-4o-transcribe"}
	if got := filterAgentModels("openai", "audio", openAI); !reflect.DeepEqual(got, []string{"gpt-4o-transcribe", "whisper-1"}) {
		t.Fatalf("unexpected OpenAI audio models: %#v", got)
	}
	if got := filterAgentModels("openai", "document", openAI); len(got) != 0 {
		t.Fatalf("OpenAI document models should be hidden until the workflow supports them: %#v", got)
	}
	gemini := []string{"models/gemini-2.5-flash", "models/text-embedding-004"}
	if got := filterAgentModels("gemini", "video", gemini); !reflect.DeepEqual(got, []string{"gemini-2.5-flash"}) {
		t.Fatalf("unexpected Gemini video models: %#v", got)
	}
}
