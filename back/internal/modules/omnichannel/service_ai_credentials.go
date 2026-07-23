package omnichannel

import (
	"context"
	"fmt"
	"strings"

	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/secretbox"
)

func normalizeAICredentialName(value string) (string, error) {
	value = strings.TrimSpace(value)
	if len([]rune(value)) < 1 || len([]rune(value)) > 120 {
		return "", ErrValidation
	}
	return value, nil
}

func (s *AIService) ListAICredentials(ctx context.Context, accountID string, p auth.Principal) ([]AICredentialView, error) {
	if err := s.requireAgentPerm(ctx, accountID, p, "omnichannel.agents.manage"); err != nil {
		return nil, err
	}
	rows, err := s.store.ListAICredentials(ctx, accountID)
	if err != nil {
		return nil, err
	}
	out := make([]AICredentialView, 0, len(rows))
	for _, row := range rows {
		out = append(out, row.AICredentialView)
	}
	return out, nil
}

func (s *AIService) CreateAICredential(ctx context.Context, accountID string, p auth.Principal, in AICredentialInput) (AICredentialView, error) {
	if err := s.requireAgentPerm(ctx, accountID, p, "omnichannel.agents.manage"); err != nil {
		return AICredentialView{}, err
	}
	name, err := normalizeAICredentialName(in.Name)
	if err != nil {
		return AICredentialView{}, err
	}
	provider := normalizeAIProviderKeyID(in.Provider)
	raw := strings.TrimSpace(in.APIKey)
	if provider == "" || raw == "" || len(raw) > 8192 || s.box == nil {
		return AICredentialView{}, ErrValidation
	}
	ciphertext, err := s.box.Encrypt(raw)
	if err != nil {
		return AICredentialView{}, err
	}
	row, err := s.store.CreateAICredential(ctx, accountID, p.UserID, name, provider, ciphertext, secretbox.Mask(raw).Last4)
	if err != nil {
		return AICredentialView{}, translate(err)
	}
	return row.AICredentialView, nil
}

func (s *AIService) UpdateAICredential(ctx context.Context, accountID string, p auth.Principal, credentialID string, in AICredentialPatch) (AICredentialView, error) {
	if err := s.requireAgentPerm(ctx, accountID, p, "omnichannel.agents.manage"); err != nil {
		return AICredentialView{}, err
	}
	if !omnichannelUUIDPattern.MatchString(strings.TrimSpace(credentialID)) || (in.Name == nil && in.APIKey == nil) {
		return AICredentialView{}, ErrValidation
	}
	var name, ciphertext, last4 *string
	if in.Name != nil {
		normalized, err := normalizeAICredentialName(*in.Name)
		if err != nil {
			return AICredentialView{}, err
		}
		name = &normalized
	}
	if in.APIKey != nil {
		raw := strings.TrimSpace(*in.APIKey)
		if raw == "" || len(raw) > 8192 || s.box == nil {
			return AICredentialView{}, ErrValidation
		}
		value, err := s.box.Encrypt(raw)
		if err != nil {
			return AICredentialView{}, err
		}
		masked := secretbox.Mask(raw).Last4
		ciphertext, last4 = &value, &masked
	}
	row, err := s.store.UpdateAICredential(ctx, accountID, credentialID, name, ciphertext, last4)
	if err != nil {
		return AICredentialView{}, translate(err)
	}
	return row.AICredentialView, nil
}

func (s *AIService) DeleteAICredential(ctx context.Context, accountID string, p auth.Principal, credentialID string) error {
	if err := s.requireAgentPerm(ctx, accountID, p, "omnichannel.agents.manage"); err != nil {
		return err
	}
	if !omnichannelUUIDPattern.MatchString(strings.TrimSpace(credentialID)) {
		return ErrNotFound
	}
	return s.store.DeleteAICredential(ctx, accountID, credentialID)
}

func (s *AIService) credentialAPIKey(ctx context.Context, accountID, credentialID, provider string) (string, error) {
	row, err := s.store.GetAICredential(ctx, accountID, credentialID)
	if err != nil {
		return "", err
	}
	if normalizeAIProviderKeyID(provider) == "" || row.Provider != normalizeAIProviderKeyID(provider) || s.box == nil {
		return "", ErrAIProviderKeyMissing
	}
	raw, err := s.box.Decrypt(row.SecretCiphertext)
	if err != nil || strings.TrimSpace(raw) == "" {
		return "", ErrAIProviderKeyMissing
	}
	return strings.TrimSpace(raw), nil
}

func (s *AIService) versionAPIKey(ctx context.Context, accountID string, agent agentRow, version versionRow) (string, error) {
	if version.ResponseCredentialID != nil && strings.TrimSpace(*version.ResponseCredentialID) != "" {
		return s.credentialAPIKey(ctx, accountID, *version.ResponseCredentialID, version.Provider)
	}
	return s.providerAPIKey(agent, version.Provider)
}

// ImportLegacyAICredentials is an explicit, idempotent transition from per-agent
// keyrings. Plaintext exists only in memory and is immediately re-encrypted into
// the account-scoped vault; the legacy source remains for rollback compatibility.
func (s *AIService) ImportLegacyAICredentials(ctx context.Context, accountID string, p auth.Principal) (AICredentialImportView, error) {
	if err := s.requireAgentPerm(ctx, accountID, p, "omnichannel.agents.manage"); err != nil {
		return AICredentialImportView{}, err
	}
	if s.box == nil {
		return AICredentialImportView{}, ErrAIProviderKeyMissing
	}
	existing, err := s.store.ListAICredentials(ctx, accountID)
	if err != nil {
		return AICredentialImportView{}, err
	}
	known := make(map[string]struct{})
	for _, credential := range existing {
		plain, decryptErr := s.box.Decrypt(credential.SecretCiphertext)
		if decryptErr == nil {
			known[credential.Provider+"\x00"+plain] = struct{}{}
		}
	}
	agents, err := s.store.ListAgents(ctx, accountID)
	if err != nil {
		return AICredentialImportView{}, err
	}
	result := AICredentialImportView{}
	for _, agent := range agents {
		activeProvider, providerErr := s.store.AgentActiveProvider(ctx, accountID, agent.ID)
		if providerErr != nil {
			return AICredentialImportView{}, providerErr
		}
		keys, decodeErr := s.decodeProviderKeyring(agent.ProviderKeyCipher, activeProvider)
		if decodeErr != nil {
			return AICredentialImportView{}, decodeErr
		}
		for provider, raw := range keys {
			identity := provider + "\x00" + raw
			if _, ok := known[identity]; ok {
				result.Existing++
				continue
			}
			last4 := secretbox.Mask(raw).Last4
			baseName := fmt.Sprintf("%s_%s", provider, last4)
			name := baseName
			for suffix := 2; ; suffix++ {
				exists, existsErr := s.store.AICredentialNameExists(ctx, accountID, name)
				if existsErr != nil {
					return AICredentialImportView{}, existsErr
				}
				if !exists {
					break
				}
				name = fmt.Sprintf("%s_%d", baseName, suffix)
			}
			ciphertext, encryptErr := s.box.Encrypt(raw)
			if encryptErr != nil {
				return AICredentialImportView{}, encryptErr
			}
			if _, createErr := s.store.CreateAICredential(ctx, accountID, p.UserID, name, provider, ciphertext, last4); createErr != nil {
				return AICredentialImportView{}, translate(createErr)
			}
			known[identity] = struct{}{}
			result.Imported++
		}
	}
	return result, nil
}
