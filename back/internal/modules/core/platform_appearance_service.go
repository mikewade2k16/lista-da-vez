package core

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// AppearanceService orquestra a leitura/escrita da aparência GLOBAL da
// plataforma (tema visual). Nível plataforma: sem accountId. A autorização de
// escrita (platform_admin) é aplicada na camada HTTP. Reusa o repositório de
// core.platform_settings (mesma tabela do menu_layout, chave 'appearance').
type AppearanceService struct {
	repo platformSettingsRepository
}

// NewAppearanceService injeta o repositório de platform_settings.
func NewAppearanceService(repo platformSettingsRepository) *AppearanceService {
	return &AppearanceService{repo: repo}
}

// GetAppearance lê a chave 'appearance'. Quando a linha ainda não existe,
// devolve o default (tema dark, sem overrides) com updatedAt/updatedBy nil —
// sem erro.
func (s *AppearanceService) GetAppearance(ctx context.Context) (AppearanceResponse, error) {
	raw, updatedAt, updatedBy, err := s.repo.GetByKey(ctx, appearanceKey)
	if err != nil {
		return AppearanceResponse{}, err
	}
	if len(raw) == 0 {
		return AppearanceResponse{Appearance: defaultAppearance()}, nil
	}

	var appearance Appearance
	if err := json.Unmarshal(raw, &appearance); err != nil {
		return AppearanceResponse{}, fmt.Errorf("unmarshal appearance: %w", err)
	}
	normalizeAppearance(&appearance)

	return AppearanceResponse{
		Appearance: appearance,
		UpdatedAt:  updatedAt,
		UpdatedBy:  updatedBy,
	}, nil
}

// SaveAppearance valida o tema (slug), normaliza, serializa e persiste sob a
// chave 'appearance'. userID é o autor (platform_admin) da escrita. Tema
// mal-formado → ErrValidationFailed (400 no handler).
func (s *AppearanceService) SaveAppearance(ctx context.Context, appearance Appearance, userID string) (AppearanceResponse, error) {
	appearance.ActiveTheme = strings.TrimSpace(appearance.ActiveTheme)
	if appearance.ActiveTheme == "" {
		appearance.ActiveTheme = defaultActiveTheme
	}
	if !isValidThemeSlug(appearance.ActiveTheme) {
		return AppearanceResponse{}, ErrValidationFailed(
			fmt.Sprintf("tema invalido %q (use um slug: minusculas, digitos e hifen)", appearance.ActiveTheme),
		)
	}

	normalizeAppearance(&appearance)

	raw, err := json.Marshal(appearance)
	if err != nil {
		return AppearanceResponse{}, fmt.Errorf("marshal appearance: %w", err)
	}

	updatedAt, err := s.repo.Upsert(ctx, appearanceKey, raw, userID)
	if err != nil {
		return AppearanceResponse{}, err
	}

	uid := userID
	return AppearanceResponse{
		Appearance: appearance,
		UpdatedAt:  &updatedAt,
		UpdatedBy:  &uid,
	}, nil
}

// normalizeAppearance garante version mínima, nome custom e mapas não-nulos, e
// limpa temas/chaves vazios dos overrides (chave sem o prefixo '--'), para o
// JSON de resposta nunca ser null e o storage ficar enxuto.
func normalizeAppearance(a *Appearance) {
	if a.Version < 1 {
		a.Version = 1
	}
	if strings.TrimSpace(a.CustomThemeName) == "" {
		a.CustomThemeName = defaultCustomThemeName
	}

	cleaned := map[string]map[string]string{}
	for theme, vars := range a.Overrides {
		theme = strings.TrimSpace(theme)
		if theme == "" || vars == nil {
			continue
		}
		inner := map[string]string{}
		for rawKey, value := range vars {
			key := strings.TrimSpace(strings.TrimPrefix(rawKey, "--"))
			if key == "" {
				continue
			}
			inner[key] = value
		}
		if len(inner) > 0 {
			cleaned[theme] = inner
		}
	}
	a.Overrides = cleaned
}
