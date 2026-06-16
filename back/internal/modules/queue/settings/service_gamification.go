package settings

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
)

// GetGamificationSection retorna a config de gamificacao do tenant,
// com fallback nos defaults quando a linha ainda nao existe.
func (service *Service) GetGamificationSection(ctx context.Context, principal auth.Principal, requestedTenantID string) (GamificationConfig, error) {
	if !canViewSettings(principal) {
		return GamificationConfig{}, ErrForbidden
	}

	tenantID, err := service.resolveTenantID(ctx, principal, requestedTenantID)
	if err != nil {
		return GamificationConfig{}, err
	}

	section, found, err := service.repository.GetGamificationSection(ctx, tenantID)
	if err != nil {
		return GamificationConfig{}, err
	}
	if !found {
		return defaultGamificationConfig(), nil
	}
	return normalizeGamificationConfig(section.Config), nil
}

// SaveGamificationSection persiste as badge rules para o tenant.
func (service *Service) SaveGamificationSection(ctx context.Context, principal auth.Principal, input GamificationSectionInput) (MutationAck, error) {
	tenantID, err := service.resolveWritableTenantID(ctx, principal, input.TenantID)
	if err != nil {
		return MutationAck{}, err
	}

	current, found, err := service.repository.GetGamificationSection(ctx, tenantID)
	if err != nil {
		return MutationAck{}, err
	}

	if !found {
		current = GamificationSectionRecord{
			TenantID: tenantID,
			Config:   defaultGamificationConfig(),
		}
	}

	if input.Config != nil {
		current.Config = normalizeGamificationConfig(*input.Config)
	}

	saved, err := service.repository.UpsertGamificationSection(ctx, GamificationSectionRecord{
		TenantID: tenantID,
		Config:   current.Config,
	})
	if err != nil {
		return MutationAck{}, err
	}

	return service.finalizeMutation(ctx, newMutationAck(tenantID, saved.UpdatedAt), nil)
}

// encodeBadgeRules serializa uma lista de BadgeRule para json.RawMessage.
// Retorna nil quando a lista for vazia ou a serializacao falhar.
func encodeBadgeRules(rules []BadgeRule) json.RawMessage {
	if len(rules) == 0 {
		return nil
	}

	data, err := json.Marshal(rules)
	if err != nil {
		return nil
	}

	return json.RawMessage(data)
}

// normalizeGamificationConfig valida e normaliza as badge rules recebidas.
// IDs invalidos ou vazios sao descartados; fields obrigatorios recebem fallback.
func normalizeGamificationConfig(input GamificationConfig) GamificationConfig {
	if len(input.BadgeRules) == 0 {
		return defaultGamificationConfig()
	}

	rules := make([]BadgeRule, 0, len(input.BadgeRules))
	for _, rule := range input.BadgeRules {
		id := strings.TrimSpace(rule.ID)
		if id == "" {
			continue
		}

		label := strings.TrimSpace(rule.Label)
		if label == "" {
			label = id
		}

		rules = append(rules, BadgeRule{
			ID:          id,
			Label:       label,
			Icon:        strings.TrimSpace(rule.Icon),
			Description: strings.TrimSpace(rule.Description),
			Enabled:     rule.Enabled,
			Threshold:   rule.Threshold,
		})
	}

	if len(rules) == 0 {
		return defaultGamificationConfig()
	}

	return GamificationConfig{BadgeRules: rules}
}
