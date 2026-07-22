package omnichannel

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/secretbox"
)

const aiProviderKeyringVersion = "ai-provider-keys.v1"

var aiProviderKeyIDs = []string{"gemini", "glm", "openai"}

type aiProviderKeyring struct {
	Version string            `json:"version"`
	Keys    map[string]string `json:"keys"`
}

type AIProviderKeyStatusView struct {
	Keys map[string]secretbox.Status `json:"keys"`
}

type AIProviderKeyInput struct {
	APIKey string `json:"apiKey"`
}

func normalizeAIProviderKeyID(provider string) string {
	provider = strings.ToLower(strings.TrimSpace(provider))
	for _, allowed := range aiProviderKeyIDs {
		if provider == allowed {
			return provider
		}
	}
	return ""
}

// decodeProviderKeyring accepts the encrypted multi-provider envelope and the legacy
// single raw key. Legacy data belongs to the active provider at migration time.
func (s *AIService) decodeProviderKeyring(ciphertext, legacyProvider string) (map[string]string, error) {
	keys := make(map[string]string, len(aiProviderKeyIDs))
	if strings.TrimSpace(ciphertext) == "" {
		return keys, nil
	}
	if s.box == nil {
		return nil, ErrAIProviderKeyMissing
	}
	plain, err := s.box.Decrypt(ciphertext)
	if err != nil {
		return nil, err
	}
	var envelope aiProviderKeyring
	if json.Unmarshal([]byte(plain), &envelope) == nil && envelope.Version == aiProviderKeyringVersion && envelope.Keys != nil {
		for provider, key := range envelope.Keys {
			if normalized := normalizeAIProviderKeyID(provider); normalized != "" && strings.TrimSpace(key) != "" {
				keys[normalized] = strings.TrimSpace(key)
			}
		}
		return keys, nil
	}
	if provider := normalizeAIProviderKeyID(legacyProvider); provider != "" && strings.TrimSpace(plain) != "" {
		keys[provider] = strings.TrimSpace(plain)
	}
	return keys, nil
}

func (s *AIService) encryptProviderKeyring(keys map[string]string) (string, string, error) {
	if s.box == nil {
		return "", "", ErrAIProviderKeyMissing
	}
	clean := make(map[string]string, len(aiProviderKeyIDs))
	last4 := ""
	for _, provider := range aiProviderKeyIDs {
		if key := strings.TrimSpace(keys[provider]); key != "" {
			clean[provider] = key
			last4 = secretbox.Mask(key).Last4
		}
	}
	if len(clean) == 0 {
		return "", "", nil
	}
	raw, err := json.Marshal(aiProviderKeyring{Version: aiProviderKeyringVersion, Keys: clean})
	if err != nil {
		return "", "", err
	}
	ciphertext, err := s.box.Encrypt(string(raw))
	return ciphertext, last4, err
}

func (s *AIService) providerAPIKey(agent agentRow, provider string) (string, error) {
	provider = normalizeAIProviderKeyID(provider)
	if provider == "" {
		return "", ErrAIProviderUnsupported
	}
	keys, err := s.decodeProviderKeyring(agent.ProviderKeyCipher, provider)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(keys[provider]), nil
}

func (s *AIService) ListProviderKeys(ctx context.Context, accountID string, p auth.Principal, agentID string) (AIProviderKeyStatusView, error) {
	if err := s.requireAgentPerm(ctx, accountID, p, "omnichannel.agents.manage"); err != nil {
		return AIProviderKeyStatusView{}, err
	}
	agent, err := s.assertAgentScope(ctx, accountID, agentID)
	if err != nil {
		return AIProviderKeyStatusView{}, err
	}
	activeProvider, err := s.store.AgentActiveProvider(ctx, accountID, agentID)
	if err != nil {
		return AIProviderKeyStatusView{}, err
	}
	keys, err := s.decodeProviderKeyring(agent.ProviderKeyCipher, activeProvider)
	if err != nil {
		return AIProviderKeyStatusView{}, err
	}
	statuses := make(map[string]secretbox.Status, len(aiProviderKeyIDs))
	for _, provider := range aiProviderKeyIDs {
		statuses[provider] = secretbox.Mask(keys[provider])
	}
	return AIProviderKeyStatusView{Keys: statuses}, nil
}

func (s *AIService) PutProviderKey(ctx context.Context, accountID string, p auth.Principal, agentID, provider, raw string) (AIProviderKeyStatusView, error) {
	if err := s.requireAgentPerm(ctx, accountID, p, "omnichannel.agents.manage"); err != nil {
		return AIProviderKeyStatusView{}, err
	}
	provider = normalizeAIProviderKeyID(provider)
	if provider == "" {
		return AIProviderKeyStatusView{}, ErrAIProviderUnsupported
	}
	agent, err := s.assertAgentScope(ctx, accountID, agentID)
	if err != nil {
		return AIProviderKeyStatusView{}, err
	}
	activeProvider, err := s.store.AgentActiveProvider(ctx, accountID, agentID)
	if err != nil {
		return AIProviderKeyStatusView{}, err
	}
	keys, err := s.decodeProviderKeyring(agent.ProviderKeyCipher, activeProvider)
	if err != nil {
		return AIProviderKeyStatusView{}, err
	}
	raw = strings.TrimSpace(raw)
	if raw == "" {
		delete(keys, provider)
	} else {
		keys[provider] = raw
	}
	ciphertext, last4, err := s.encryptProviderKeyring(keys)
	if err != nil {
		return AIProviderKeyStatusView{}, err
	}
	if _, err := s.store.UpdateAgentProviderKeys(ctx, accountID, agentID, ciphertext, last4); err != nil {
		return AIProviderKeyStatusView{}, translate(err)
	}
	statuses := make(map[string]secretbox.Status, len(aiProviderKeyIDs))
	for _, id := range aiProviderKeyIDs {
		statuses[id] = secretbox.Mask(keys[id])
	}
	return AIProviderKeyStatusView{Keys: statuses}, nil
}
