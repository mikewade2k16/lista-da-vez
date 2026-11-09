package app

import (
	"context"
	"encoding/json"
	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/llm"
)

type plusAppLLMFixture struct{ calls *int }

func (f plusAppLLMFixture) Complete(_ context.Context, req llm.Request) (llm.Response, error) {
	*f.calls++
	var context struct {
		Evidence []struct {
			ID string `json:"chunkId"`
		} `json:"untrustedEvidence"`
	}
	if e := json.Unmarshal([]byte(req.UserPrompt), &context); e != nil {
		return llm.Response{}, e
	}
	ids := []string{}
	for _, v := range context.Evidence {
		ids = append(ids, v.ID)
	}
	raw, e := json.Marshal(map[string]any{"answer": "O material é uma fonte sintética para consulta.", "evidenceIds": ids, "assumptions": []string{}, "missingData": []string{"Contexto fiscal não confirmado."}, "coverage": "informational"})
	return llm.Response{Model: req.Model, JSON: raw, UsageKnown: true, Usage: llm.Usage{PromptTokens: 10, CompletionTokens: 10, TotalTokens: 20}}, e
}
