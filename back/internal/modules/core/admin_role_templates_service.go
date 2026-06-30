package core

import (
	"context"
	"strings"
)

// admin_role_templates_service.go — regras de negocio do CRUD de role templates.
// Camada que valida ANTES de tocar o repo: charset do id, label obrigatorio,
// bloqueio de template is_system (CONTRACT_FREEZE) e validade das permission keys
// contra o catalogo (core.permissions). O repo so executa SQL parametrizado.

// RoleTemplateAdminService orquestra o CRUD de templates. Injecao via construtor
// (sem global). A autorizacao platform_admin e aplicada na borda HTTP.
type RoleTemplateAdminService struct {
	repo RoleTemplateAdminRepository
}

// NewRoleTemplateAdminService injeta o repositorio de templates.
func NewRoleTemplateAdminService(repo RoleTemplateAdminRepository) *RoleTemplateAdminService {
	return &RoleTemplateAdminService{repo: repo}
}

// List devolve todos os templates (com permissionKeys) + o catalogo de
// permissoes disponiveis para montar a matriz.
func (s *RoleTemplateAdminService) List(ctx context.Context) (RoleTemplatesListResponse, error) {
	templates, err := s.repo.ListRoleTemplates(ctx)
	if err != nil {
		return RoleTemplatesListResponse{}, err
	}
	available, err := s.repo.ListAvailablePermissions(ctx)
	if err != nil {
		return RoleTemplatesListResponse{}, err
	}
	return RoleTemplatesListResponse{Templates: templates, Available: available}, nil
}

// Create valida e cria um template custom (is_system=false). Erros de validacao:
// ErrRoleTemplateInvalidID (charset), ErrRoleTemplateLabelRequired (label vazio),
// ErrInvalidPermission (key fora do catalogo). Conflito de id -> ErrRoleTemplateConflict.
func (s *RoleTemplateAdminService) Create(ctx context.Context, in CreateRoleTemplateInput) (RoleTemplate, error) {
	in.ID = strings.TrimSpace(in.ID)
	in.Label = strings.TrimSpace(in.Label)
	in.Description = strings.TrimSpace(in.Description)
	in.PermissionKeys = normalizeKeys(in.PermissionKeys)

	if !isValidRoleTemplateID(in.ID) {
		return RoleTemplate{}, ErrRoleTemplateInvalidID
	}
	if in.Label == "" {
		return RoleTemplate{}, ErrRoleTemplateLabelRequired
	}
	if err := s.validatePermissionKeys(ctx, in.PermissionKeys); err != nil {
		return RoleTemplate{}, err
	}

	return s.repo.CreateRoleTemplate(ctx, in)
}

// Patch atualiza metadados (label/description/sortOrder) de um template CUSTOM.
// Bloqueia template de sistema (ErrRoleTemplateSystem). Label, se enviado, nao
// pode ser vazio (ErrRoleTemplateLabelRequired). 404 se o id nao existe.
func (s *RoleTemplateAdminService) Patch(ctx context.Context, id string, in PatchRoleTemplateInput) (RoleTemplate, error) {
	if err := s.ensureCustom(ctx, id); err != nil {
		return RoleTemplate{}, err
	}
	if in.Label != nil {
		trimmed := strings.TrimSpace(*in.Label)
		if trimmed == "" {
			return RoleTemplate{}, ErrRoleTemplateLabelRequired
		}
		in.Label = &trimmed
	}
	if in.Description != nil {
		trimmed := strings.TrimSpace(*in.Description)
		in.Description = &trimmed
	}
	return s.repo.PatchRoleTemplate(ctx, id, in)
}

// ReplacePermissions troca o conjunto de permissoes de um template CUSTOM (delete
// + insert). Bloqueia template de sistema (CONTRACT_FREEZE) e valida as keys.
func (s *RoleTemplateAdminService) ReplacePermissions(ctx context.Context, id string, keys []string) (RoleTemplate, error) {
	if err := s.ensureCustom(ctx, id); err != nil {
		return RoleTemplate{}, err
	}
	keys = normalizeKeys(keys)
	if err := s.validatePermissionKeys(ctx, keys); err != nil {
		return RoleTemplate{}, err
	}
	return s.repo.ReplaceTemplatePermissions(ctx, id, keys)
}

// Delete remove um template CUSTOM. Bloqueia template de sistema. 404 se nao existe.
func (s *RoleTemplateAdminService) Delete(ctx context.Context, id string) error {
	if err := s.ensureCustom(ctx, id); err != nil {
		return err
	}
	return s.repo.DeleteRoleTemplate(ctx, id)
}

// ensureCustom resolve o template e garante que NAO e de sistema. Retorna
// ErrTemplateNotFound (404) ou ErrRoleTemplateSystem (409) quando aplicavel.
func (s *RoleTemplateAdminService) ensureCustom(ctx context.Context, id string) error {
	t, err := s.repo.FindRoleTemplate(ctx, id)
	if err != nil {
		return err
	}
	if t.IsSystem {
		return ErrRoleTemplateSystem
	}
	return nil
}

// validatePermissionKeys garante que toda key existe no catalogo, esta viva e
// nao e de escopo plataforma. Keys invalidas -> ErrInvalidPermission (422).
func (s *RoleTemplateAdminService) validatePermissionKeys(ctx context.Context, keys []string) error {
	if len(keys) == 0 {
		return nil
	}
	invalid, err := s.repo.InvalidPermissionKeys(ctx, keys)
	if err != nil {
		return err
	}
	if len(invalid) > 0 {
		return ErrInvalidPermission
	}
	return nil
}

// normalizeKeys remove vazios/espacos e duplicatas, preservando determinismo
// (ordena). Garante slice nao-nulo.
func normalizeKeys(keys []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(keys))
	for _, k := range keys {
		k = strings.TrimSpace(k)
		if k == "" {
			continue
		}
		if _, dup := seen[k]; dup {
			continue
		}
		seen[k] = struct{}{}
		out = append(out, k)
	}
	return out
}
