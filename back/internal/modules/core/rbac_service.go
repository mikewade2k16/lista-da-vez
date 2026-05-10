package core

import (
	"context"
	"strings"
)

// RBACService gerencia roles e permissões de uma Account.
//
// Item 1 (Fase 3): InitAccountRoles — seed de roles ao criar/habilitar módulos.
// Itens seguintes (Fase 3): CreateRole, UpdateRolePermissions, AssignRoleToUser,
// e resolução de permissões reais em MeContext.
type RBACService struct {
	rbac RBACRepository
}

// NewRBACService cria o serviço RBAC.
func NewRBACService(rbac RBACRepository) *RBACService {
	return &RBACService{rbac: rbac}
}

// InitAccountRoles faz o seed de todos os role templates dos módulos informados
// para a account, criando entradas em core.roles e core.role_permissions.
//
// Idempotente: roles que já existem (mesmo code) são ignorados sem erro.
// Chamado ao criar uma nova account (todos os módulos habilitados de uma vez)
// ou ao habilitar um módulo adicional numa account existente.
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
