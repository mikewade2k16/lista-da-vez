package omnichannel

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
)

// registerBuiltinAITools registra somente adaptadores internos cujo contrato e
// fonte autoritativa pertencem ao omnichannel. O registry injetado pelo
// compositor continua soberano; estes handlers so entram no registry default.
// Nenhum handler recebe credencial, SQL ou URL escolhida pelo modelo.
func registerBuiltinAITools(registry *AIToolRegistry, store *Store) {
	if registry == nil || store == nil {
		return
	}
	_ = registry.Register("knowledge.search", func(ctx context.Context, invocation AIToolInvocation) (json.RawMessage, error) {
		var input struct {
			Query    string  `json:"query"`
			TopK     int     `json:"topK"`
			MinScore float64 `json:"minScore"`
		}
		decoder := json.NewDecoder(bytes.NewReader(invocation.Arguments))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&input); err != nil || strings.TrimSpace(input.Query) == "" || len([]rune(input.Query)) > 500 {
			return nil, ErrAIToolArguments
		}
		if input.TopK == 0 {
			input.TopK = 5
		}
		if input.TopK < 1 || input.TopK > 20 || input.MinScore < 0 || input.MinScore > 1 {
			return nil, ErrAIToolArguments
		}
		results, err := store.SearchKnowledge(ctx, invocation.AccountID, invocation.AgentID, input.Query, input.TopK, input.MinScore)
		if err != nil {
			return nil, err
		}
		return json.Marshal(map[string]any{"results": results, "evidenceCount": len(results)})
	})
}
