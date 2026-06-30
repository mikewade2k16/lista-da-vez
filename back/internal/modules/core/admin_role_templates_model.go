package core

import (
	"errors"
	"regexp"
)

// admin_role_templates_model.go — DTOs e validacao do CRUD de role templates
// (catalogo GLOBAL em core.role_templates + core.role_template_permissions),
// gerenciado por platform_admin. Templates sao o molde que TODA conta nova clona
// (cloneRoleTemplates em admin_repository.go). Templates is_system=true sao
// declarados por codigo (module.RoleTemplates) e congelados (CONTRACT_FREEZE):
// nao podem ser editados/deletados pelo painel. Templates is_system=false sao
// criados aqui e sobrevivem ao boot (SyncCatalog nunca deleta nao-declarados).

const (
	// roleTemplateCustomModuleID e o modulo dono dos templates criados pelo
	// painel. Sempre 'core' (modulo sempre habilitado, permissoes workspace.*).
	roleTemplateCustomModuleID = "core"

	// roleTemplateCustomSortOrder e o sort_order default de template criado pelo
	// painel (apos os de sistema, que usam 0/10/100).
	roleTemplateCustomSortOrder = 200
)

// roleTemplateIDPattern restringe o id de um template criado pelo painel a um
// charset seguro: minusculas, digitos, ponto, underscore e hifen. Bloqueia
// espacos, maiusculas e simbolos (defesa contra id arbitrario/injecao de label
// em UI). 1..120 chars.
var roleTemplateIDPattern = regexp.MustCompile(`^[a-z0-9._-]{1,120}$`)

// isValidRoleTemplateID informa se id respeita o charset seguro.
func isValidRoleTemplateID(id string) bool {
	return roleTemplateIDPattern.MatchString(id)
}

var (
	// ErrRoleTemplateConflict — id ja existe em core.role_templates. 409.
	ErrRoleTemplateConflict = errors.New("core: role template id already exists")

	// ErrRoleTemplateSystem — tentativa de editar/deletar template is_system=true.
	// Templates de sistema sao congelados (CONTRACT_FREEZE). 409.
	ErrRoleTemplateSystem = errors.New("core: system role template cannot be modified")

	// ErrRoleTemplateInvalidID — id fora do charset seguro. 400.
	ErrRoleTemplateInvalidID = errors.New("core: invalid role template id charset")

	// ErrRoleTemplateLabelRequired — label vazio na criacao. 400.
	ErrRoleTemplateLabelRequired = errors.New("core: role template label is required")
)

// RoleTemplate e o shape de um template no contrato HTTP (camelCase no JSON).
// PermissionKeys vem de core.role_template_permissions, ordenadas.
type RoleTemplate struct {
	ID             string   `json:"id"`
	ModuleID       string   `json:"moduleId"`
	Label          string   `json:"label"`
	Description    string   `json:"description"`
	IsSystem       bool     `json:"isSystem"`
	IsLocked       bool     `json:"isLocked"`
	SortOrder      int      `json:"sortOrder"`
	PermissionKeys []string `json:"permissionKeys"`
}

// RoleTemplatesListResponse e o shape do GET /v1/admin/role-templates:
// todos os templates + o catalogo de permissoes para montar a matriz.
// Available reusa AvailablePermission (admin_overrides_model.go): mesmo shape
// key/label/moduleId/scope, evita DTO duplicado.
type RoleTemplatesListResponse struct {
	Templates []RoleTemplate        `json:"templates"`
	Available []AvailablePermission `json:"available"`
}

// CreateRoleTemplateInput e o body do POST. ID e Label obrigatorios; module_id,
// sort_order, is_system e is_locked sao FIXADOS pelo service (nao vem do client).
type CreateRoleTemplateInput struct {
	ID             string   `json:"id"`
	Label          string   `json:"label"`
	Description    string   `json:"description"`
	PermissionKeys []string `json:"permissionKeys"`
}

// PatchRoleTemplateInput e o body do PATCH. Campos nil = nao alterar (patch
// semantico). Permissoes NAO entram aqui — tem endpoint proprio (PUT .../permissions).
type PatchRoleTemplateInput struct {
	Label       *string `json:"label"`
	Description *string `json:"description"`
	SortOrder   *int    `json:"sortOrder"`
}

// ReplaceRoleTemplatePermissionsInput e o body do PUT .../permissions.
type ReplaceRoleTemplatePermissionsInput struct {
	PermissionKeys []string `json:"permissionKeys"`
}
