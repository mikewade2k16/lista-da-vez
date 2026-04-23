package operations

import "context"

const (
	RoleConsultant    = "consultant"
	RoleStoreTerminal = "store_terminal"
	RoleManager       = "manager"
	RoleMarketing     = "marketing"
	RoleDirector      = "director"
	RoleOwner         = "owner"
	RolePlatformAdmin = "platform_admin"
)

type AccessContext struct {
	UserID   string
	TenantID string
	Role     string
	StoreIDs []string
}

type StoreScopeFilter struct {
	TenantID string
}

type StoreScopeView struct {
	ID       string
	TenantID string
	Code     string
	Name     string
	City     string
}

type StoreScopeProvider interface {
	ListAccessible(ctx context.Context, access AccessContext, filter StoreScopeFilter) ([]StoreScopeView, error)
}
