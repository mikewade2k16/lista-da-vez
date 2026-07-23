package omnichannel

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
)

var (
	ErrAIModelsUnavailable   = errors.New("omnichannel: ai models unavailable")
	ErrAIProviderUnsupported = errors.New("omnichannel: ai provider unsupported")
	ErrAIProviderKeyMissing  = errors.New("omnichannel: ai provider key missing")
)

const aiModelsMaxResponseBytes = 2 << 20

var aiProviderModelsBaseURL = map[string]string{
	"openai": "https://api.openai.com/v1",
	"gemini": "https://generativelanguage.googleapis.com/v1beta/openai",
	"glm":    "https://api.z.ai/api/paas/v4",
}

type aiModelsResponse struct {
	Data []struct {
		ID string `json:"id"`
	} `json:"data"`
}

// ListAgentModels popula o select do painel usando a chave cifrada do próprio agente.
// A chave só existe durante esta chamada server-side e nunca volta ao navegador.
func (s *AIService) ListAgentModels(ctx context.Context, accountID string, p auth.Principal, agentID, provider string) ([]string, error) {
	if err := s.requireAgentPerm(ctx, accountID, p, "omnichannel.agents.manage"); err != nil {
		return nil, err
	}
	agent, err := s.assertAgentScope(ctx, accountID, agentID)
	if err != nil {
		return nil, err
	}
	provider = strings.ToLower(strings.TrimSpace(provider))
	baseURL, supported := aiProviderModelsBaseURL[provider]
	if !supported {
		return nil, ErrAIProviderUnsupported
	}
	if s.box == nil || strings.TrimSpace(agent.ProviderKeyCipher) == "" {
		return nil, ErrAIProviderKeyMissing
	}
	apiKey, err := s.providerAPIKey(agent, provider)
	if err != nil || strings.TrimSpace(apiKey) == "" {
		return nil, ErrAIProviderKeyMissing
	}
	return fetchAgentProviderModels(ctx, baseURL, apiKey, provider)
}

func fetchAgentProviderModels(ctx context.Context, baseURL, apiKey, provider string) ([]string, error) {
	ids, err := fetchProviderModelIDs(ctx, baseURL, apiKey)
	if err != nil {
		return nil, err
	}
	return filterAgentModels(provider, "response", ids), nil
}

func fetchProviderModelIDs(ctx context.Context, baseURL, apiKey string) ([]string, error) {
	callCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(callCtx, http.MethodGet, strings.TrimRight(baseURL, "/")+"/models", nil)
	if err != nil {
		return nil, ErrAIModelsUnavailable
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "Omni-Omnichannel/1.0")

	response, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, ErrAIModelsUnavailable
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return nil, ErrAIModelsUnavailable
	}
	raw, err := io.ReadAll(io.LimitReader(response.Body, aiModelsMaxResponseBytes))
	if err != nil {
		return nil, ErrAIModelsUnavailable
	}
	var payload aiModelsResponse
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, ErrAIModelsUnavailable
	}
	ids := make([]string, 0, len(payload.Data))
	for _, model := range payload.Data {
		ids = append(ids, model.ID)
	}
	return ids, nil
}

func filterAgentModels(provider, capability string, ids []string) []string {
	seen := make(map[string]struct{}, len(ids))
	out := make([]string, 0, len(ids))
	for _, raw := range ids {
		id := strings.TrimPrefix(strings.TrimSpace(raw), "models/")
		lower := strings.ToLower(id)
		if id == "" || !isAgentCapabilityModel(provider, capability, lower) {
			continue
		}
		if _, exists := seen[id]; exists {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	sort.Strings(out)
	return out
}

func filterAgentChatModels(provider string, ids []string) []string {
	return filterAgentModels(provider, "response", ids)
}

func isAgentCapabilityModel(provider, capability, id string) bool {
	switch capability {
	case "audio":
		if provider == "openai" {
			return strings.Contains(id, "whisper") || strings.Contains(id, "transcri")
		}
		return provider == "gemini" && strings.Contains(id, "gemini") && !strings.Contains(id, "embedding")
	case "video":
		return provider == "gemini" && strings.Contains(id, "gemini") && !strings.Contains(id, "embedding")
	case "image":
		if provider == "openai" {
			return hasAgentModelPrefix(id, "gpt", "chatgpt") && isAgentChatModel(provider, id)
		}
		return provider == "gemini" && isAgentChatModel(provider, id)
	case "document":
		return provider == "gemini" && isAgentChatModel(provider, id)
	default:
		return isAgentChatModel(provider, id)
	}
}

func (s *AIService) ListCredentialModels(ctx context.Context, accountID string, p auth.Principal, credentialID, capability string) ([]string, error) {
	if err := s.requireAgentPerm(ctx, accountID, p, "omnichannel.agents.manage"); err != nil {
		return nil, err
	}
	row, err := s.store.GetAICredential(ctx, accountID, credentialID)
	if err != nil {
		return nil, err
	}
	capability = strings.ToLower(strings.TrimSpace(capability))
	switch capability {
	case "response", "audio", "image", "video", "document":
	default:
		return nil, ErrValidation
	}
	baseURL, ok := aiProviderModelsBaseURL[row.Provider]
	if !ok {
		return nil, ErrAIProviderUnsupported
	}
	apiKey, err := s.credentialAPIKey(ctx, accountID, credentialID, row.Provider)
	if err != nil {
		return nil, err
	}
	ids, err := fetchProviderModelIDs(ctx, baseURL, apiKey)
	if err != nil {
		return nil, err
	}
	return filterAgentModels(row.Provider, capability, ids), nil
}

func isAgentChatModel(provider, id string) bool {
	switch provider {
	case "openai":
		return hasAgentModelPrefix(id, "gpt", "o1", "o3", "o4", "chatgpt") &&
			!containsAgentModelMarker(id, "embedding", "audio", "realtime", "transcribe", "image", "tts", "moderation", "instruct", "codex", "deep-research", "computer-use")
	case "gemini":
		return strings.Contains(id, "gemini") && !containsAgentModelMarker(id, "embedding", "aqa", "imagen")
	case "glm":
		return strings.HasPrefix(id, "glm") && !containsAgentModelMarker(id, "embedding", "voice", "video", "image", "cogview", "rerank")
	default:
		return false
	}
}

func hasAgentModelPrefix(value string, prefixes ...string) bool {
	for _, prefix := range prefixes {
		if strings.HasPrefix(value, prefix) {
			return true
		}
	}
	return false
}

func containsAgentModelMarker(value string, markers ...string) bool {
	for _, marker := range markers {
		if strings.Contains(value, marker) {
			return true
		}
	}
	return false
}
