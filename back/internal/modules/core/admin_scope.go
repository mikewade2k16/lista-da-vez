package core

import "context"

// adminManagePermKeys e o conjunto de permissoes que caracterizam "administra a
// account" no caminho (c) da delegacao. Ter QUALQUER uma resolvida na account
// basta. Sao as permissoes ja declaradas (module.go) — a delegacao reusa, sem
// permissao nova. NAO inclui core.users.view/roles.view (apenas leitura).
var adminManagePermKeys = []string{"core.users.manage", "core.roles.manage"}

// AdminScopeRepository abstrai a resolucao de autoridade admin no banco. Todas
// as decisoes sao por-request e por-account; nenhuma depende de Principal.Role/
// Permissions (resolvidos so no login a partir de UMA conta home).
type AdminScopeRepository interface {
	// CanManageAccount: o ator pode MUTAR esta account (membership/papel/override)?
	CanManageAccount(ctx context.Context, actorUserID, accountID string) (bool, error)
	// CanManageUser: o ator pode administrar este usuario (membro de alguma account
	// que o ator administra, ou ator platform_admin)?
	CanManageUser(ctx context.Context, actorUserID, targetUserID string) (bool, error)
	// CanManageOrganization: SO platform_admin OU agency_owner da propria org.
	CanManageOrganization(ctx context.Context, actorUserID, organizationID string) (bool, error)
	// IsPlatformAdmin resolve is_platform_admin ativo no banco.
	IsPlatformAdmin(ctx context.Context, actorUserID string) (bool, error)
	// IsAdminOfAnything: gate barato anti-sondagem (tem algum poder de admin?).
	IsAdminOfAnything(ctx context.Context, actorUserID string) (bool, error)
}

// AdminScopeResolver e a peca-chave da delegacao multi-tenant: resolve, por
// request, se o ATOR autenticado pode administrar uma account/usuario/org. Os
// metodos devolvem booleano; quem decide o codigo HTTP (404 fora de escopo, 403
// para ator sem nenhum poder) e o handler. actorUserID SEMPRE vem do Principal,
// nunca do body.
type AdminScopeResolver struct {
	repo AdminScopeRepository
}

// NewAdminScopeResolver cria o resolver com o repositorio de escopo.
func NewAdminScopeResolver(repo AdminScopeRepository) *AdminScopeResolver {
	return &AdminScopeResolver{repo: repo}
}

// CanManageAccount: o ator pode mutar membership/papel/override desta account?
func (s *AdminScopeResolver) CanManageAccount(ctx context.Context, actorUserID, accountID string) (bool, error) {
	return s.repo.CanManageAccount(ctx, actorUserID, accountID)
}

// CanManageUser: o ator pode administrar este usuario-alvo?
func (s *AdminScopeResolver) CanManageUser(ctx context.Context, actorUserID, targetUserID string) (bool, error) {
	return s.repo.CanManageUser(ctx, actorUserID, targetUserID)
}

// CanManageOrganization: o ator pode vincular/desvincular usuarios desta org?
func (s *AdminScopeResolver) CanManageOrganization(ctx context.Context, actorUserID, organizationID string) (bool, error) {
	return s.repo.CanManageOrganization(ctx, actorUserID, organizationID)
}

// IsPlatformAdmin: o ator e platform_admin ativo? (gate identity-global).
func (s *AdminScopeResolver) IsPlatformAdmin(ctx context.Context, actorUserID string) (bool, error) {
	return s.repo.IsPlatformAdmin(ctx, actorUserID)
}

// IsAdminOfAnything: o ator tem algum poder de admin? (gate anti-sondagem).
func (s *AdminScopeResolver) IsAdminOfAnything(ctx context.Context, actorUserID string) (bool, error) {
	return s.repo.IsAdminOfAnything(ctx, actorUserID)
}
