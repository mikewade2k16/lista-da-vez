package core

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

// listUsersScopeWhere monta o predicado de escopo do GET /v1/admin/users: um
// usuario `u` so e visivel ao ator quando QUALQUER caminho vale:
//
//	(a) o ator e platform_admin -> ve todos;
//	(b) o usuario u e membro (account_users) de ALGUMA account que o ator
//	    administra (manageableAccountWhere). Cobre agency_owner (ve usuarios das
//	    accounts da org) e admin de cliente (ve membros das accounts onde tem
//	    core.users.manage). Sem distincao de existencia entre os perfis.
//
// Parametros (indices do caller): actorIdx=$ do actorUserID, permsIdx=$ do
// text[] de permissoes de gestao. A subquery interna usa `a` (alias da account
// administravel) e referencia u.id do escopo externo (correlacionada). O caller
// usa o MESMO where no count(*) e no SELECT (total e linhas batem — sem
// enumeration). Toda a regra vive no SQL (defesa em profundidade).
func listUsersScopeWhere(actorIdx, permsIdx int) string {
	manageable := fmt.Sprintf(`
		a.is_active = true
		and (
		  exists (
		    select 1 from core.users pu
		    where pu.id = $%[1]d::uuid and pu.is_active = true and pu.is_platform_admin = true
		  )
		  or exists (
		    select 1 from core.organization_users ou
		    where ou.user_id = $%[1]d::uuid
		      and ou.org_role = 'agency_owner'
		      and ou.organization_id = a.organization_id
		  )
		  or exists (
		    select 1 from (
		      select rp.permission_key
		      from core.user_role_assignments ura
		      join core.role_permissions rp on rp.role_id = ura.role_id
		      where ura.account_id = a.id and ura.user_id = $%[1]d::uuid

		      union

		      select permission_key
		      from core.user_permission_overrides
		      where account_id = a.id and user_id = $%[1]d::uuid
		        and effect = 'allow' and is_active = true

		      except

		      select permission_key
		      from core.user_permission_overrides
		      where account_id = a.id and user_id = $%[1]d::uuid
		        and effect = 'deny' and is_active = true
		    ) eff(permission_key)
		    where eff.permission_key = any($%[2]d::text[])
		  )
		)`, actorIdx, permsIdx)

	return fmt.Sprintf(`(
	  exists (
	    select 1 from core.users padmin
	    where padmin.id = $%[1]d::uuid and padmin.is_active = true and padmin.is_platform_admin = true
	  )
	  or exists (
	    select 1
	    from core.account_users tau
	    join core.accounts a on a.id = tau.account_id
	    where tau.user_id = u.id
	      and %[2]s
	  )
	)`, actorIdx, manageable)
}

// PostgresAdminScopeRepository resolve, 100% no banco e por-request, se um ATOR
// pode administrar uma account/usuario arbitrario. NAO confia em
// Principal.Permissions/Role (resolvidos uma vez no login a partir de UMA conta
// "home" — ver auth/core_role_resolver.go); a delegacao multi-tenant precisa de
// uma decisao por-account, espelhando accountVisibilityWhere (store_postgres.go)
// e account_checker.go. SQL totalmente parametrizado.
type PostgresAdminScopeRepository struct {
	pool *pgxpool.Pool
}

// NewPostgresAdminScopeRepository cria a implementacao Postgres do resolver de
// escopo admin reaproveitando o pool ja existente.
func NewPostgresAdminScopeRepository(pool *pgxpool.Pool) *PostgresAdminScopeRepository {
	return &PostgresAdminScopeRepository{pool: pool}
}

// manageablePermsExists monta a subquery UNION allow EXCEPT deny (espelhada de
// rbac_repository.go ListPermissionsForUser): "o ator ($actorIdx, uuid) tem AO
// MENOS UMA das permissoes de $permsIdx (text[]) RESOLVIDA na account indicada por
// accountExpr?" — role_permissions dos cargos atribuidos MAIS overrides allow,
// MENOS overrides deny ativos. accountExpr e o id da account no contexto da query
// (`$n::uuid` quando vem por parametro; `a.id` quando vem do join). actorIdx/permsIdx
// sao os indices ($n) dos parametros no caller — assim o predicado se encaixa sem
// criar buracos de parametro (PG NAO infere o tipo de um parametro nao referenciado
// na SQL -> SQLSTATE 42P18). Caminho (c): NAO basta membership — exige permissao de gestao.
func manageablePermsExists(accountExpr string, actorIdx, permsIdx int) string {
	return fmt.Sprintf(`
		exists (
		  select 1 from (
		    select rp.permission_key
		    from core.user_role_assignments ura
		    join core.role_permissions rp on rp.role_id = ura.role_id
		    where ura.account_id = %[1]s and ura.user_id = $%[2]d::uuid

		    union

		    select permission_key
		    from core.user_permission_overrides
		    where account_id = %[1]s and user_id = $%[2]d::uuid
		      and effect = 'allow' and is_active = true

		    except

		    select permission_key
		    from core.user_permission_overrides
		    where account_id = %[1]s and user_id = $%[2]d::uuid
		      and effect = 'deny' and is_active = true
		  ) eff(permission_key)
		  where eff.permission_key = any($%[3]d::text[])
		)`, accountExpr, actorIdx, permsIdx)
}

// manageableAccountWhere monta o predicado "o ator ($actorIdx=userID) pode MUTAR a
// account (alias `a` para core.accounts, $permsIdx=text[] de permissoes-chave de
// admin)". Vale por QUALQUER caminho:
//
//	(a) o ator e platform_admin -> administra TODAS as accounts, ATIVAS OU INATIVAS;
//	(b) a account esta ATIVA e o ator e agency_owner da org dona da account; ou
//	(c) a account esta ATIVA e o ator tem alguma das permissoes de $permsIdx
//	    RESOLVIDA naquela account (a.id).
//
// platform_admin e o acesso maximo do painel: DEVE sempre poder mutar/remover
// qualquer account, inclusive inativa. Se o gate exigisse is_active=true para
// TODOS (como era antes), nem o platform_admin conseguia remover um vinculo
// apontando para uma conta desativada (ex.: conta de teste inativada):
// RemoveMembership -> CanManageAccount=false -> 404 "conta destino nao encontrada
// ou inativa", e o vinculo ficava preso para sempre sem caminho na UI. Por isso o
// ramo (a) fica FORA do gate is_active. agency_owner e admin-de-cliente (b/c)
// seguem limitados a contas ativas — nao ha caso de uso para eles mutarem contas
// desativadas, e o limite evita gestao sobre contas fora do fluxo ativo.
//
// Espelha accountVisibilityWhere mas (b/c) e MAIS RESTRITO: la basta membership,
// aqui exige permissao de gestao (core.users.manage / core.roles.manage). actorIdx/
// permsIdx sao os indices dos parametros no caller (sem buracos de parametro -> sem 42P18).
func manageableAccountWhere(actorIdx, permsIdx int) string {
	return fmt.Sprintf(`(
		  exists (
		    select 1 from core.users u
		    where u.id = $%[1]d::uuid
		      and u.is_active = true
		      and u.is_platform_admin = true
		  )
		  or (
		    a.is_active = true
		    and (
		      exists (
		        select 1 from core.organization_users ou
		        where ou.user_id = $%[1]d::uuid
		          and ou.org_role = 'agency_owner'
		          and ou.organization_id = a.organization_id
		      )
		      or %[2]s
		    )
		  )
		)`, actorIdx, manageablePermsExists("a.id", actorIdx, permsIdx))
}

// CanManageAccount diz se o ator pode MUTAR a account (membership/papel/override
// daquela account). Aplica manageableAccountWhere com as permissoes de gestao
// padrao (core.users.manage OU core.roles.manage). false (sem erro) quando nao
// administra — o handler traduz para 404 (nao vaza existencia de outro tenant).
func (r *PostgresAdminScopeRepository) CanManageAccount(ctx context.Context, actorUserID, accountID string) (bool, error) {
	query := `select exists (
		select 1 from core.accounts a
		where a.id = $2::uuid and ` + manageableAccountWhere(1, 3) + `
	)`
	var ok bool
	err := r.pool.QueryRow(ctx, query, actorUserID, accountID, adminManagePermKeys).Scan(&ok)
	return ok, err
}

// IsPlatformAdmin resolve no banco se o ator e platform_admin ativo. Usado pelos
// gates de identidade-global (is_platform_admin, email, senha, soft-delete) e de
// move destrutivo — campos que SO platform_admin pode tocar.
func (r *PostgresAdminScopeRepository) IsPlatformAdmin(ctx context.Context, actorUserID string) (bool, error) {
	const query = `
		select exists (
			select 1 from core.users
			where id = $1::uuid and is_active = true and is_platform_admin = true
		)`
	var ok bool
	err := r.pool.QueryRow(ctx, query, actorUserID).Scan(&ok)
	return ok, err
}

// IsAdminOfAnything e o gate barato anti-sondagem: o ator tem ALGUM poder de
// admin? (platform_admin OU agency_owner de alguma org OU core.users/roles.manage
// resolvido em alguma account). Usado no inicio de cada handler para responder
// 403 cedo a quem nao administra nada — sem precisar resolver o alvo do path.
func (r *PostgresAdminScopeRepository) IsAdminOfAnything(ctx context.Context, actorUserID string) (bool, error) {
	// $1=ator (uuid), $2=perms (text[]). Sem parametro fantasma: PG nao infere o
	// tipo de um parametro nao referenciado na SQL (SQLSTATE 42P18).
	query := `
		select
		  exists (
		    select 1 from core.users u
		    where u.id = $1::uuid and u.is_active = true and u.is_platform_admin = true
		  )
		  or exists (
		    select 1 from core.organization_users ou
		    where ou.user_id = $1::uuid and ou.org_role = 'agency_owner'
		  )
		  or exists (
		    select 1
		    from core.accounts a
		    where a.is_active = true and ` + manageablePermsExists("a.id", 1, 2) + `
		  )`
	var ok bool
	err := r.pool.QueryRow(ctx, query, actorUserID, adminManagePermKeys).Scan(&ok)
	return ok, err
}

// CanManageUser diz se o ator pode administrar o usuario-alvo: existe ALGUMA
// account onde o ator administra (manageableAccountWhere) E o alvo e membro
// (account_users, ativo OU inativo — preserva poder sobre vinculos desativados).
// platform_admin curto-circuita para true (administra qualquer usuario). false
// (sem erro) -> o handler traduz para 404.
func (r *PostgresAdminScopeRepository) CanManageUser(ctx context.Context, actorUserID, targetUserID string) (bool, error) {
	// $1=ator (uuid), $2=alvo (uuid), $3=perms (text[]). Sem parametro fantasma
	// (PG nao infere tipo de parametro nao referenciado -> SQLSTATE 42P18).
	query := `
		select
		  exists (
		    select 1 from core.users u
		    where u.id = $1::uuid and u.is_active = true and u.is_platform_admin = true
		  )
		  or exists (
		    select 1
		    from core.account_users tau
		    join core.accounts a on a.id = tau.account_id
		    where tau.user_id = $2::uuid
		      and ` + manageableAccountWhere(1, 3) + `
		  )`
	var ok bool
	err := r.pool.QueryRow(ctx, query, actorUserID, targetUserID, adminManagePermKeys).Scan(&ok)
	return ok, err
}

// CanManageOrganization diz se o ator pode vincular/desvincular usuarios DESTA
// organization: SO platform_admin OU agency_owner da PROPRIA org. Admin de
// cliente (caminho (c)) NAO entra — virar membro de agencia da visao ampla, poder
// reservado. false (sem erro) -> 404.
func (r *PostgresAdminScopeRepository) CanManageOrganization(ctx context.Context, actorUserID, organizationID string) (bool, error) {
	const query = `
		select exists (
		  select 1 from core.organizations o
		  where o.id = $2::uuid and o.is_active = true
		    and (
		      exists (
		        select 1 from core.users u
		        where u.id = $1::uuid and u.is_active = true and u.is_platform_admin = true
		      )
		      or exists (
		        select 1 from core.organization_users ou
		        where ou.user_id = $1::uuid
		          and ou.org_role = 'agency_owner'
		          and ou.organization_id = o.id
		      )
		    )
		)`
	var ok bool
	err := r.pool.QueryRow(ctx, query, actorUserID, organizationID).Scan(&ok)
	return ok, err
}
