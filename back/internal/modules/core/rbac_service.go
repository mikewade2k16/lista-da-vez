package core

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

// RBACService gerencia roles e permissões de uma Account.
type RBACService struct {
	rbac RBACRepository
}

// NewRBACService cria o serviço RBAC.
func NewRBACService(rbac RBACRepository) *RBACService {
	return &RBACService{rbac: rbac}
}

// ============================================================================
// Seed (Item 1)
// ============================================================================

// InitAccountRoles faz o seed de todos os role templates dos módulos informados
// para a account, criando entradas em core.roles e core.role_permissions.
//
// Idempotente: roles que já existem (mesmo code) são ignorados sem erro.
// Chamado ao criar uma nova account ou ao habilitar um módulo adicional.
func (s *RBACService) InitAccountRoles(
	ctx context.Context,
	accountID string,
	moduleIDs []string,
) error {
	if strings.TrimSpace(accountID) == "" {
		return ErrAccountNotFound
	}
	if len(moduleIDs) == 0 {
		return nil
	}

	templates, err := s.rbac.ListTemplatesForModules(ctx, moduleIDs)
	if err != nil {
		return err
	}

	for _, tmpl := range templates {
		permKeys, err := s.rbac.ListTemplatePermissionKeys(ctx, tmpl.ID)
		if err != nil {
			return err
		}

		roleID, created, err := s.rbac.CloneTemplate(ctx, accountID, tmpl)
		if err != nil {
			return err
		}

		if !created || len(permKeys) == 0 {
			continue
		}
		if err := s.rbac.SetRolePermissions(ctx, roleID, permKeys); err != nil {
			return err
		}
	}

	return nil
}

// ============================================================================
// CRUD de roles (Item 3)
// ============================================================================

// ListRoles retorna todos os roles da account.
func (s *RBACService) ListRoles(
	ctx context.Context,
	accountID string,
) ([]Role, error) {
	return s.rbac.ListRolesForAccount(ctx, accountID)
}

// GetRole retorna um role verificando que pertence à account.
func (s *RBACService) GetRole(
	ctx context.Context,
	accountID, roleID string,
) (Role, error) {
	return s.rbac.FindRole(ctx, accountID, roleID)
}

// CreateRole cria um cargo customizado (não derivado de template).
// Valida que o code não está em uso na account.
func (s *RBACService) CreateRole(
	ctx context.Context,
	accountID, code, label, description string,
) (Role, error) {
	code = strings.TrimSpace(code)
	label = strings.TrimSpace(label)
	if code == "" || label == "" {
		return Role{}, fmt.Errorf("core: code e label são obrigatórios")
	}
	return s.rbac.CreateRole(ctx, accountID, code, label, description)
}

// UpdateRolePermissions substitui as permissões do role, validando cada key
// contra o catálogo de permissões habilitadas para a account.
func (s *RBACService) UpdateRolePermissions(
	ctx context.Context,
	accountID, roleID string,
	label, description string,
	permKeys []string,
) (Role, error) {
	role, err := s.rbac.FindRole(ctx, accountID, roleID)
	if err != nil {
		return Role{}, err
	}

	if len(permKeys) > 0 {
		invalid, err := s.rbac.InvalidPermissionKeys(ctx, accountID, permKeys)
		if err != nil {
			return Role{}, err
		}
		if len(invalid) > 0 {
			return Role{}, fmt.Errorf("core: permissões inválidas ou módulo desabilitado: %v", invalid)
		}
		// Defesa em profundidade (M1): bloqueia permissoes scope='platform' na
		// matriz do papel — espelha o bloqueio dos overrides por-usuario. Sem isto
		// um core.roles.manage de cliente concederia uma permissao de plataforma
		// (ex.: core.organization.consolidated_read) via papel custom.
		platform, err := s.rbac.PlatformScopedKeys(ctx, permKeys)
		if err != nil {
			return Role{}, err
		}
		if len(platform) > 0 {
			return Role{}, ErrInvalidPermission
		}
	}

	newLabel := strings.TrimSpace(label)
	if newLabel == "" {
		newLabel = role.Label
	}
	newDesc := description
	if newDesc == "" {
		newDesc = role.Description
	}

	if err := s.rbac.UpdateRole(ctx, accountID, roleID, newLabel, newDesc); err != nil {
		return Role{}, err
	}

	if err := s.rbac.ReplaceRolePermissions(ctx, roleID, permKeys); err != nil {
		return Role{}, err
	}

	return s.rbac.FindRole(ctx, accountID, roleID)
}

// DeleteRole remove o role. Falha se is_locked=true.
func (s *RBACService) DeleteRole(
	ctx context.Context,
	accountID, roleID string,
) error {
	role, err := s.rbac.FindRole(ctx, accountID, roleID)
	if err != nil {
		return err
	}
	if role.IsLocked {
		return ErrRoleIsLocked
	}
	return s.rbac.DeleteRole(ctx, accountID, roleID)
}

// ============================================================================
// Atribuição de roles (Item 3)
// ============================================================================

// AssignRoleToUser atribui um role a um user dentro da account. Verifica que
// o role pertence à account antes de atribuir. Idempotente.
func (s *RBACService) AssignRoleToUser(
	ctx context.Context,
	accountID, userID, roleID string,
) error {
	if _, err := s.rbac.FindRole(ctx, accountID, roleID); err != nil {
		return err
	}
	return s.rbac.AssignRoleToUser(ctx, accountID, userID, roleID)
}

// RemoveRoleFromUser remove a atribuição user→role. Não falha se já não existia.
func (s *RBACService) RemoveRoleFromUser(
	ctx context.Context,
	accountID, userID, roleID string,
) error {
	return s.rbac.RemoveRoleFromUser(ctx, accountID, userID, roleID)
}

// SetUserRoles substitui em LOTE todos os papéis do usuário na account pelo
// conjunto roleIDs. Valida que o alvo é membro da account (ErrNotMember -> 404) e
// que CADA roleID pertence à account (FindRole -> ErrRoleNotFound). Replace
// transacional. Mantém os assign/remove 1-a-1 (coexistência). Sem dedupe manual:
// o ON CONFLICT do replace ignora repetidos. Retorna os papéis efetivos do
// usuário NAQUELA account após o replace (RoleSummary[], mesmo shape do
// GET .../roles) para o painel atualizar sem refetch.
func (s *RBACService) SetUserRoles(
	ctx context.Context,
	accountID, userID string,
	roleIDs []string,
) ([]RoleSummary, error) {
	if err := s.rbac.CheckMembership(ctx, accountID, userID); err != nil {
		if errors.Is(err, ErrAccountNotMember) {
			return nil, ErrNotMember
		}
		return nil, err
	}
	for _, roleID := range roleIDs {
		if _, err := s.rbac.FindRole(ctx, accountID, roleID); err != nil {
			return nil, err
		}
	}
	if err := s.rbac.ReplaceUserRoleAssignments(ctx, accountID, userID, roleIDs); err != nil {
		return nil, err
	}

	return s.ListUserRoles(ctx, accountID, userID)
}

// ListUserRoles retorna os papéis core.roles atribuídos ao usuário NAQUELA account
// como RoleSummary[] (mesmo shape do GET .../roles e da resposta do PUT roles).
// Complemento de leitura de SetUserRoles — o gate de escopo fica no handler
// (requireRolesManage em modo leitura).
func (s *RBACService) ListUserRoles(
	ctx context.Context,
	accountID, userID string,
) ([]RoleSummary, error) {
	rawRoles, err := s.rbac.ListRolesForUser(ctx, accountID, userID)
	if err != nil {
		return nil, err
	}
	summaries := make([]RoleSummary, 0, len(rawRoles))
	for _, ro := range rawRoles {
		summaries = append(summaries, ro.ToSummary())
	}
	return summaries, nil
}

// HasAccountPermission resolve se o usuário tem a permissão na account (delega ao
// repositório). Exposto para reuso por outros services/gates do core.
func (s *RBACService) HasAccountPermission(
	ctx context.Context,
	accountID, userID, permKey string,
) (bool, error) {
	return s.rbac.HasAccountPermission(ctx, accountID, userID, permKey)
}

// ============================================================================
// Resolução (Item 5 — usado por MeContext)
// ============================================================================

// ResolveUserContext retorna os roles e permissões efetivas do user na account.
func (s *RBACService) ResolveUserContext(
	ctx context.Context,
	accountID, userID string,
) (roles []RoleSummary, permissions []string, err error) {
	rawRoles, err := s.rbac.ListRolesForUser(ctx, accountID, userID)
	if err != nil {
		return nil, nil, err
	}

	summaries := make([]RoleSummary, 0, len(rawRoles))
	for _, r := range rawRoles {
		summaries = append(summaries, r.ToSummary())
	}

	perms, err := s.rbac.ListPermissionsForUser(ctx, accountID, userID)
	if err != nil {
		return nil, nil, err
	}

	return summaries, perms, nil
}
