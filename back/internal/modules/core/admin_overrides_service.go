package core

import (
	"context"
	"strings"
)

// permissionKeyValidator valida keys de permissao contra o catalogo/modulos
// habilitados de uma account. Implementado por PostgresRBACRepository
// (InvalidPermissionKeys) — injetado para reuso sem duplicar a query.
type permissionKeyValidator interface {
	InvalidPermissionKeys(ctx context.Context, accountID string, keys []string) ([]string, error)
}

// AdminOverridesService orquestra os overrides allow/deny por usuario por account.
// Toda decisao de escopo passa pelo AdminScopeResolver (CanManageAccount); a
// validacao de key reusa InvalidPermissionKeys + bloqueio de scope=platform.
type AdminOverridesService struct {
	repo      AdminOverridesRepository
	scope     *AdminScopeResolver
	validator permissionKeyValidator
}

// NewAdminOverridesService cria o service de overrides.
func NewAdminOverridesService(repo AdminOverridesRepository, scope *AdminScopeResolver, validator permissionKeyValidator) *AdminOverridesService {
	return &AdminOverridesService{repo: repo, scope: scope, validator: validator}
}

// GetOverrides retorna os overrides ativos + o catalogo de keys aplicaveis.
// Escopo: CanManageAccount senao 404; alvo precisa ser membro da account senao
// 404 (nao vaza existencia).
func (s *AdminOverridesService) GetOverrides(ctx context.Context, actorUserID, userID, accountID string) (UserOverridesResponse, error) {
	accountID = strings.TrimSpace(accountID)
	if err := s.ensureScopeAndMember(ctx, actorUserID, userID, accountID); err != nil {
		return UserOverridesResponse{}, err
	}

	overrides, err := s.repo.ListActiveOverrides(ctx, accountID, userID)
	if err != nil {
		return UserOverridesResponse{}, err
	}
	available, err := s.repo.ListAvailablePermissions(ctx, accountID)
	if err != nil {
		return UserOverridesResponse{}, err
	}
	return UserOverridesResponse{Overrides: overrides, Available: available}, nil
}

// ReplaceOverrides valida e substitui os overrides do usuario na account. Regras:
//   - escopo CanManageAccount + alvo membro (senao 404);
//   - effect em {allow,deny} (senao 422 invalid_effect);
//   - key valida no catalogo/modulo habilitado (InvalidPermissionKeys) e
//     scope != platform (senao 422 invalid_permission);
//   - sem keys duplicadas (o indice unico parcial nao aceita; barramos antes).
//
// account_id/user_id vem SEMPRE do path. created_by_user_id=actorUserID.
func (s *AdminOverridesService) ReplaceOverrides(ctx context.Context, actorUserID, userID, accountID string, input ReplaceOverridesInput) (UserOverridesResponse, error) {
	accountID = strings.TrimSpace(accountID)
	if err := s.ensureScopeAndMember(ctx, actorUserID, userID, accountID); err != nil {
		return UserOverridesResponse{}, err
	}

	normalized := make([]UserPermissionOverride, 0, len(input.Overrides))
	seen := make(map[string]bool, len(input.Overrides))
	keys := make([]string, 0, len(input.Overrides))
	for _, o := range input.Overrides {
		key := strings.TrimSpace(o.PermissionKey)
		effect := strings.ToLower(strings.TrimSpace(o.Effect))
		if key == "" {
			return UserOverridesResponse{}, ErrInvalidPermission
		}
		switch effect {
		case "allow", "deny":
		default:
			return UserOverridesResponse{}, ErrInvalidEffect
		}
		if seen[key] {
			// Key repetida -> ambiguo + violaria o indice unico parcial.
			return UserOverridesResponse{}, ErrInvalidPermission
		}
		seen[key] = true
		keys = append(keys, key)
		normalized = append(normalized, UserPermissionOverride{
			PermissionKey: key,
			Effect:        effect,
			Note:          strings.TrimSpace(o.Note),
		})
	}

	if len(keys) > 0 {
		// (a) fora do catalogo / modulo desabilitado.
		invalid, err := s.validator.InvalidPermissionKeys(ctx, accountID, keys)
		if err != nil {
			return UserOverridesResponse{}, err
		}
		if len(invalid) > 0 {
			return UserOverridesResponse{}, ErrInvalidPermission
		}
		// (b) scope='platform' bloqueado para override.
		platform, err := s.repo.PlatformScopedKeys(ctx, keys)
		if err != nil {
			return UserOverridesResponse{}, err
		}
		if len(platform) > 0 {
			return UserOverridesResponse{}, ErrInvalidPermission
		}
	}

	if err := s.repo.ReplaceUserOverrides(ctx, accountID, userID, actorUserID, normalized); err != nil {
		return UserOverridesResponse{}, err
	}
	return s.GetOverrides(ctx, actorUserID, userID, accountID)
}

// ensureScopeAndMember valida que o ator administra a account (404 fora de
// escopo) e que o alvo e membro dela (404 senao). Mesma mensagem para nao vazar.
func (s *AdminOverridesService) ensureScopeAndMember(ctx context.Context, actorUserID, userID, accountID string) error {
	can, err := s.scope.CanManageAccount(ctx, actorUserID, accountID)
	if err != nil {
		return err
	}
	if !can {
		return ErrAccountNotFound
	}
	member, err := s.repo.IsAccountMember(ctx, accountID, userID)
	if err != nil {
		return err
	}
	if !member {
		return ErrNotMember
	}
	return nil
}
