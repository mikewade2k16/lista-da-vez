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
	UserID string
	// TenantID e o tenant do usuario (pode ser vazio p/ platform_admin, que opera por
	// account). AccountID e a account ativa resolvida do X-Account-Id pelo middleware
	// e e a chave correta dos dados scoped por account (metas/ERP). Prefira AccountID.
	TenantID            string
	AccountID           string
	Role                string
	StoreIDs            []string
	Permissions         []string
	PermissionsResolved bool
}

// ScopeTenantID devolve o id de escopo para dados por account (metas/ERP): a account
// ativa quando presente, caindo no TenantID do usuario. Vazio so quando nenhum existe.
func (access AccessContext) ScopeTenantID() string {
	if access.AccountID != "" {
		return access.AccountID
	}
	return access.TenantID
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
