package cardapio

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
)

// Chaves de permissao do modulo (catalogo declarado em module.go Permissions()).
// GET => permView; mutacao de catalogo/restaurante/layout/zona => permManage;
// mutacao de pedido (status) => permOrdersManage.
const (
	permView         = "cardapio.view"
	permManage       = "cardapio.manage"
	permOrdersManage = "cardapio.orders.manage"
)

// permissionChecker resolve permissoes finas de uma account no banco. Satisfeito
// por *core.RBACService (HasAccountPermission ja existe e e usado por ~20 modulos).
// Mantido como interface para o gate ser injetavel/testavel e nil-safe.
type permissionChecker interface {
	HasAccountPermission(ctx context.Context, accountID, userID, permKey string) (bool, error)
}

// cardapioGate aplica permissao fina nos handlers do painel. platform_admin, o
// owner da conta e o agency_owner (da org dona da account) entram em curto-circuito
// (permitir); demais papeis precisam da permissao RESOLVIDA na account
// (role_permissions + overrides allow/deny). Falha de permissao vira ErrForbidden,
// que o handler traduz para 404 uniforme (nao vaza existencia/escopo) — mesmo padrao do modulo.
//
// Gate nil-safe: quando o RBACService nao foi injetado (ex.: testes que nao
// exercitam o gate), Authorize so aplica o curto-circuito de platform_admin/owner
// (Principal.Role) e nega o resto — fail-closed, nunca fail-open.
type cardapioGate struct {
	perms permissionChecker
	pool  *pgxpool.Pool // so para o curto-circuito de agency_owner (resolvido no banco)
}

// newCardapioGate cria o gate. pool e usado apenas para resolver agency_owner
// (papel de organizacao, fora do Principal); perms resolve a permissao fina.
func newCardapioGate(perms permissionChecker, pool *pgxpool.Pool) *cardapioGate {
	return &cardapioGate{perms: perms, pool: pool}
}

// Authorize libera (nil) quando o Principal pode exercer permKey na account, ou
// devolve ErrForbidden caso contrario. Ordem: platform_admin/owner (Principal) ->
// permissao fina (RBAC) -> agency_owner (org da account). Sem RBAC injetado, so
// platform_admin/owner passam (fail-closed).
func (g *cardapioGate) Authorize(ctx context.Context, principal auth.Principal, accountID, permKey string) error {
	// platform_admin e o owner da conta sempre gerenciam o proprio cardapio (mesmo
	// idioma de access/tenants/users, que gateiam owner por Principal.Role); manager/
	// marketing/director precisam da permissao fina cardapio.* ou do papel
	// cardapio.viewer/manager. agency_owner resolve no banco mais abaixo.
	if principal.Role == auth.RolePlatformAdmin || principal.Role == auth.RoleOwner {
		return nil
	}
	if g != nil && g.perms != nil {
		ok, err := g.perms.HasAccountPermission(ctx, accountID, principal.UserID, permKey)
		if err != nil {
			return err
		}
		if ok {
			return nil
		}
	}
	if g != nil && g.pool != nil {
		owner, err := g.isAgencyOwner(ctx, accountID, principal.UserID)
		if err != nil {
			return err
		}
		if owner {
			return nil
		}
	}
	return ErrForbidden
}

// isAgencyOwner resolve no banco se o user e agency_owner da organizacao dona da
// account (espelha o ramo agency_owner de core.CanAccessAccountRoles). SQL
// parametrizado e schema-qualificado; accountID/userID sao UUID.
func (g *cardapioGate) isAgencyOwner(ctx context.Context, accountID, userID string) (bool, error) {
	const q = `select exists (
		select 1
		from core.accounts a
		join core.organization_users ou
			on ou.organization_id = a.organization_id
		where a.id = $1::uuid
		  and ou.user_id = $2::uuid
		  and ou.org_role = 'agency_owner'
	)`
	var ok bool
	if err := g.pool.QueryRow(ctx, q, accountID, userID).Scan(&ok); err != nil {
		return false, err
	}
	return ok, nil
}
