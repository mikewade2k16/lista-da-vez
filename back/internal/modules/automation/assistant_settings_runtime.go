package automation

import (
	"context"
	"strings"
)

// ResolveAssistantSettings usa o mesmo cofre canônico, sem regravar a configuração de outra surface.
func (s *Service) ResolveAssistantSettings(ctx context.Context, account, credentialID, model string, temperature float64, history int) (OmniChatRuntimeConfig, error) {
	if s.omniChatCredentials == nil || strings.TrimSpace(credentialID) == "" {
		return OmniChatRuntimeConfig{}, ErrOmniChatCredentialUnavailable
	}
	credential, err := s.omniChatCredentials.ResolveCredential(ctx, account, credentialID)
	if err != nil || strings.TrimSpace(credential.APIKey) == "" {
		return OmniChatRuntimeConfig{}, ErrOmniChatCredentialUnavailable
	}
	provider := normalizeOmniChatProvider(credential.Provider)
	if provider == "" || strings.TrimSpace(model) == "" {
		return OmniChatRuntimeConfig{}, ErrOmniChatInvalidConfig
	}
	return OmniChatRuntimeConfig{Enabled: true, Provider: provider, Model: strings.TrimSpace(model), APIKey: credential.APIKey, Temperature: temperature, HistoryWindow: history}, nil
}
