package core

import (
	"context"
	"encoding/json"
	"fmt"
)

// platformSettingsRepository abstrai a persistência da config global. Permite
// injeção via construtor (sem global) e teste sem Postgres.
type platformSettingsRepository interface {
	GetByKey(ctx context.Context, key string) (config []byte, updatedAt *string, updatedBy *string, err error)
	Upsert(ctx context.Context, key string, config []byte, updatedBy string) (updatedAt string, err error)
}

// PlatformSettingsService orquestra a leitura/escrita da config GLOBAL do menu.
// É de nível plataforma: não recebe accountId. A autorização de escrita
// (platform_admin) é aplicada na camada HTTP.
type PlatformSettingsService struct {
	repo platformSettingsRepository
}

// NewPlatformSettingsService injeta o repositório de platform_settings.
func NewPlatformSettingsService(repo platformSettingsRepository) *PlatformSettingsService {
	return &PlatformSettingsService{repo: repo}
}

// GetMenuLayout lê a chave 'menu_layout'. Quando a linha ainda não existe,
// devolve o default vazio (version=1, sections/items vazios) com
// updatedAt/updatedBy nil — sem erro.
func (s *PlatformSettingsService) GetMenuLayout(ctx context.Context) (MenuLayoutResponse, error) {
	raw, updatedAt, updatedBy, err := s.repo.GetByKey(ctx, menuLayoutKey)
	if err != nil {
		return MenuLayoutResponse{}, err
	}
	if len(raw) == 0 {
		return MenuLayoutResponse{
			Layout:    defaultMenuLayout(),
			UpdatedAt: nil,
			UpdatedBy: nil,
		}, nil
	}

	var layout MenuLayout
	if err := json.Unmarshal(raw, &layout); err != nil {
		return MenuLayoutResponse{}, fmt.Errorf("unmarshal menu_layout: %w", err)
	}
	normalizeLayout(&layout)

	return MenuLayoutResponse{
		Layout:    layout,
		UpdatedAt: updatedAt,
		UpdatedBy: updatedBy,
	}, nil
}

// SaveMenuLayout valida os placements, normaliza, serializa e persiste o layout
// sob a chave 'menu_layout'. userID é o autor (platform_admin) da escrita.
// Placement inválido → ErrValidationFailed (400 no handler).
func (s *PlatformSettingsService) SaveMenuLayout(ctx context.Context, layout MenuLayout, userID string) (MenuLayoutResponse, error) {
	for itemID, item := range layout.Items {
		if !isValidPlacement(item.Placement) {
			return MenuLayoutResponse{}, ErrValidationFailed(
				fmt.Sprintf("placement invalido %q no item %q (use header, sidebar, both ou hidden)", item.Placement, itemID),
			)
		}
	}

	normalizeLayout(&layout)

	raw, err := json.Marshal(layout)
	if err != nil {
		return MenuLayoutResponse{}, fmt.Errorf("marshal menu_layout: %w", err)
	}

	updatedAt, err := s.repo.Upsert(ctx, menuLayoutKey, raw, userID)
	if err != nil {
		return MenuLayoutResponse{}, err
	}

	uid := userID
	return MenuLayoutResponse{
		Layout:    layout,
		UpdatedAt: &updatedAt,
		UpdatedBy: &uid,
	}, nil
}

// normalizeLayout garante version mínima e coleções não-nulas, para que o JSON
// de resposta seja sempre `[]`/`{}` em vez de `null`.
func normalizeLayout(layout *MenuLayout) {
	if layout.Version < 1 {
		layout.Version = 1
	}
	if layout.Sections == nil {
		layout.Sections = []MenuLayoutSection{}
	}
	if layout.Items == nil {
		layout.Items = map[string]MenuLayoutItem{}
	}
}
