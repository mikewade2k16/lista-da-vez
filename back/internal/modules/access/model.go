package access

import (
	"context"

	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
)

const (
	ScopeStore    = "store"
	ScopeTenant   = "tenant"
	ScopePlatform = "platform"

	EffectAllow = "allow"
	EffectDeny  = "deny"

	PermissionOperationsView    = "workspace.operacao.view"
	PermissionOperationsEdit    = "workspace.operacao.edit"
	PermissionConsultantView    = "workspace.consultor.view"
	PermissionRankingView       = "workspace.ranking.view"
	PermissionDataView          = "workspace.dados.view"
	PermissionIntelligenceView  = "workspace.inteligencia.view"
	PermissionReportsView       = "workspace.relatorios.view"
	PermissionCampaignsView     = "workspace.campanhas.view"
	PermissionCampaignsEdit     = "workspace.campanhas.edit"
	PermissionClientsView       = "workspace.clientes.view"
	PermissionClientsEdit       = "workspace.clientes.edit"
	PermissionMultiStoreView    = "workspace.multiloja.view"
	PermissionMultiStoreEdit    = "workspace.multiloja.edit"
	PermissionUsersView         = "workspace.usuarios.view"
	PermissionUsersEdit         = "workspace.usuarios.edit"
	PermissionSettingsView      = "workspace.configuracoes.view"
	PermissionSettingsEdit      = "workspace.configuracoes.edit"
	PermissionFeedbackView      = "workspace.feedback.view"
	PermissionFeedbackEdit      = "workspace.feedback.edit"
	PermissionUsersPasswordEdit = "users.password.manage"
	PermissionRoleMatrixEdit    = "access.role_defaults.manage"
)

type PermissionDefinition struct {
	Key         string `json:"key"`
	Scope       string `json:"scope"`
	Description string `json:"description"`
}

type RoleGrant struct {
	Role          auth.Role `json:"role"`
	PermissionKey string    `json:"permissionKey"`
}

type UserOverride struct {
	ID            string `json:"id,omitempty"`
	UserID        string `json:"userId,omitempty"`
	PermissionKey string `json:"permissionKey"`
	Effect        string `json:"effect"`
	TenantID      string `json:"tenantId,omitempty"`
	StoreID       string `json:"storeId,omitempty"`
	Note          string `json:"note,omitempty"`
	IsActive      bool   `json:"isActive"`
}

type RoleMatrixEntry struct {
	Role           auth.Role        `json:"role"`
	Label          string           `json:"label"`
	Scope          auth.AccessScope `json:"scope"`
	PermissionKeys []string         `json:"permissionKeys"`
}

type RoleMatrixView struct {
	Permissions []PermissionDefinition `json:"permissions"`
	Roles       []RoleMatrixEntry      `json:"roles"`
}

type UserSubject struct {
	UserID    string
	Role      auth.Role
	TenantID  string
	StoreIDs  []string
	IsActive  bool
	ManagedBy string
}

type UserAccessView struct {
	UserID                  string                 `json:"userId"`
	Role                    auth.Role              `json:"role"`
	TenantID                string                 `json:"tenantId,omitempty"`
	StoreIDs                []string               `json:"storeIds,omitempty"`
	Permissions             []PermissionDefinition `json:"permissions"`
	BasePermissionKeys      []string               `json:"basePermissionKeys"`
	EffectivePermissionKeys []string               `json:"effectivePermissionKeys"`
	Overrides               []UserOverride         `json:"overrides"`
}

type Repository interface {
	ListRolePermissions(ctx context.Context, role auth.Role) ([]string, error)
	ListAllRolePermissions(ctx context.Context) ([]RoleGrant, error)
	ListActiveTenantIDs(ctx context.Context) ([]string, error)
	ReplaceRolePermissions(ctx context.Context, role auth.Role, permissionKeys []string) error
	ListUserOverrides(ctx context.Context, userID string) ([]UserOverride, error)
	ReplaceUserOverrides(ctx context.Context, userID string, overrides []UserOverride, createdByUserID string) ([]UserOverride, error)
}

type SubjectResolver interface {
	FindAccessibleSubject(ctx context.Context, principal auth.Principal, userID string) (UserSubject, error)
}
