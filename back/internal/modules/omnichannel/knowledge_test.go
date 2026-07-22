package omnichannel

import (
	"encoding/json"
	"testing"
)

func TestNormalizeKnowledgeDocumentRejectsCredentialURL(t *testing.T) {
	in := KnowledgeDocumentInput{SourceRef: "https://user:password@example.com/doc", Checksum: "abc"}
	if err := normalizeKnowledgeDocumentInput(&in); err == nil {
		t.Fatal("URL com credencial não pode entrar na fonte de conhecimento")
	}
}

func TestNormalizeKnowledgeChunksIsDeterministicAndBounded(t *testing.T) {
	in := KnowledgeChunksInput{Chunks: []KnowledgeChunkInput{{Ordinal: 0, BodyText: "um dois três"}}}
	if err := normalizeKnowledgeChunksInput(&in); err != nil {
		t.Fatal(err)
	}
	if in.Chunks[0].TokenCount != 3 {
		t.Fatalf("token_count derivado inesperado: %d", in.Chunks[0].TokenCount)
	}
	duplicate := KnowledgeChunksInput{Chunks: []KnowledgeChunkInput{{Ordinal: 1, BodyText: "a"}, {Ordinal: 1, BodyText: "b"}}}
	if err := normalizeKnowledgeChunksInput(&duplicate); err != ErrConflict {
		t.Fatalf("ordinais duplicados deveriam conflitar: %v", err)
	}
}

func TestNormalizeKnowledgeConfigRejectsSecret(t *testing.T) {
	in := KnowledgeBaseInput{Name: "FAQ", SearchConfig: json.RawMessage(`{"apiKey":"x"}`)}
	if err := normalizeKnowledgeBaseInput(&in); err == nil {
		t.Fatal("search config não pode receber segredo")
	}
}
