package omnichannel

import (
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

func TestFilterAgentChatModelsNormalizesGeminiPrefix(t *testing.T) {
	got := filterAgentChatModels("gemini", []string{"models/gemini-2.5-flash", "models/text-embedding-004"})
	want := []string{"gemini-2.5-flash"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected models: got %#v want %#v", got, want)
	}
}
