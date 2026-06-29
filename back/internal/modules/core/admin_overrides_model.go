package core

import "context"

// UserPermissionOverride e um override allow/deny de UMA permissao para UM
// usuario em UMA account. Espelha core.user_permission_overrides (linhas ativas).
type UserPermissionOverride struct {
	PermissionKey string `json:"permissionKey"`
	Effect        string `json:"effect"` // "allow" | "deny"
	Note          string `json:"note"`
}

// AvailablePermission descreve uma permissao que PODE receber override naquela
// account (modulo habilitado, nao deprecated, scope != platform). Alimenta a UI.
type AvailablePermission struct {
	Key      string `json:"key"`
	Label    string `json:"label"`
	ModuleID string `json:"moduleId"`
	Scope    string `json:"scope"`
}

// UserOverridesResponse e o body de GET/PUT /v1/admin/users/{id}/accounts/{accountId}/overrides.
// overrides = estado atual; available = catalogo de keys aplicaveis (para a UI
// montar a matriz sem hardcode).
type UserOverridesResponse struct {
	Overrides []UserPermissionOverride `json:"overrides"`
	Available []AvailablePermission    `json:"available"`
}

// ReplaceOverridesInput e o body do PUT. account_id/user_id vem SEMPRE do path.
type ReplaceOverridesInput struct {
	Overrides []UserPermissionOverride `json:"overrides"`
}

// AdminOverridesRepository abstrai a persistencia de overrides por usuario por
// account em core.user_permission_overrides.
type AdminOverridesRepository interface {
	// IsAccountMember diz se o usuario-alvo e membro (account_users) da account —
	// override so para membro (senao 404, nao vaza existencia).
	IsAccountMember(ctx context.Context, accountID, userID string) (bool, error)
	// ListActiveOverrides retorna os overrides ativos do usuario na account.
	ListActiveOverrides(ctx context.Context, accountID, userID string) ([]UserPermissionOverride, error)
	// ListAvailablePermissions retorna as permissoes aplicaveis (modulos
	// habilitados na account, nao deprecated, scope != platform).
	ListAvailablePermissions(ctx context.Context, accountID string) ([]AvailablePermission, error)
	// PlatformScopedKeys retorna, dentre as keys informadas, as que tem
	// scope='platform' (bloqueadas para override). Lista vazia = nenhuma.
	PlatformScopedKeys(ctx context.Context, keys []string) ([]string, error)
	// ReplaceUserOverrides faz o replace transacional: desativa os ativos e insere
	// os novos com created_by_user_id=actorUserID, respeitando o indice unico
	// parcial (account_id,user_id,permission_key) where is_active. account_id/
	// user_id vem do path (nunca do body).
	ReplaceUserOverrides(ctx context.Context, accountID, userID, actorUserID string, overrides []UserPermissionOverride) error
}
