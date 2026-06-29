package core

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

// platformScopedKeys retorna, dentre as keys informadas, as que tem
// scope='platform' em core.permissions. Funcao livre compartilhada por
// PostgresRBACRepository.PlatformScopedKeys (bloqueio em matriz de papel) e
// PostgresAdminOverridesRepository.PlatformScopedKeys (bloqueio em override por
// usuario) — corpo unico, parametrizado via unnest. Lista vazia quando nenhuma e
// platform-scoped.
func platformScopedKeys(ctx context.Context, pool *pgxpool.Pool, keys []string) ([]string, error) {
	if len(keys) == 0 {
		return []string{}, nil
	}
	const query = `
		select p.key
		from core.permissions p
		join unnest($1::text[]) as k(key) on k.key = p.key
		where p.scope = 'platform'
		order by p.key asc
	`
	rows, err := pool.Query(ctx, query, keys)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]string, 0)
	for rows.Next() {
		var k string
		if err := rows.Scan(&k); err != nil {
			return nil, err
		}
		out = append(out, k)
	}
	return out, rows.Err()
}

// accountPermResolvedExists e a subquery UNION allow EXCEPT deny (espelhada de
// ListPermissionsForUser) que diz se o user ($2) tem AO MENOS UMA das permissoes
// de $3 (text[]) RESOLVIDA na account ($1). Considera role_permissions dos cargos
// atribuidos MAIS overrides allow, MENOS overrides deny ativos.
const accountPermResolvedExists = `
		exists (
		  select 1 from (
		    select rp.permission_key
		    from core.user_role_assignments ura
		    join core.role_permissions rp on rp.role_id = ura.role_id
		    where ura.account_id = $1::uuid and ura.user_id = $2::uuid

		    union

		    select permission_key
		    from core.user_permission_overrides
		    where account_id = $1::uuid and user_id = $2::uuid
		      and effect = 'allow' and is_active = true

		    except

		    select permission_key
		    from core.user_permission_overrides
		    where account_id = $1::uuid and user_id = $2::uuid
		      and effect = 'deny' and is_active = true
		  ) eff(permission_key)
		  where eff.permission_key = any($3::text[])
		)`

// PlatformScopedKeys retorna, dentre as keys informadas, as que tem
// scope='platform' em core.permissions (bloqueadas em papel custom). Espelha o
// bloqueio ja aplicado aos overrides por-usuario (admin_overrides_repository.go),
// para que um core.roles.manage de cliente NAO consiga conceder uma permissao de
// plataforma (ex.: core.organization.consolidated_read) via matriz de papel.
// Parametrizado via unnest. Lista vazia quando nenhuma e platform-scoped.
func (r *PostgresRBACRepository) PlatformScopedKeys(ctx context.Context, keys []string) ([]string, error) {
	return platformScopedKeys(ctx, r.pool, keys)
}

// HasAccountPermission resolve se o user tem a permissao informada na account.
func (r *PostgresRBACRepository) HasAccountPermission(ctx context.Context, accountID, userID, permKey string) (bool, error) {
	const query = `select ` + accountPermResolvedExists
	var ok bool
	err := r.pool.QueryRow(ctx, query, accountID, userID, []string{permKey}).Scan(&ok)
	return ok, err
}

// CanAccessAccountRoles e o gate de /v1/accounts/{id}/roles*. Resolve 100% no
// banco: a account precisa existir/ativa E o user precisa de UM dos caminhos:
//
//	(a) platform_admin;
//	(b) agency_owner da org dona da account;
//	(c) ter a permissao requerida resolvida naquela account ($3 = keys: leitura ->
//	    {core.roles.view, core.roles.manage}; escrita -> {core.roles.manage}).
//
// Espelha manageableAccountWhere mas com o conjunto de permissoes variando por
// leitura/escrita. false (sem erro) -> handler responde 404.
func (r *PostgresRBACRepository) CanAccessAccountRoles(ctx context.Context, accountID, userID string, requireManage bool) (bool, error) {
	permKeys := []string{"core.roles.view", "core.roles.manage"}
	if requireManage {
		permKeys = []string{"core.roles.manage"}
	}
	const query = `
		select exists (
		  select 1 from core.accounts a
		  where a.id = $1::uuid and a.is_active = true
		    and (
		      exists (
		        select 1 from core.users u
		        where u.id = $2::uuid and u.is_active = true and u.is_platform_admin = true
		      )
		      or exists (
		        select 1 from core.organization_users ou
		        where ou.user_id = $2::uuid
		          and ou.org_role = 'agency_owner'
		          and ou.organization_id = a.organization_id
		      )
		      or ` + accountPermResolvedExists + `
		    )
		)`
	var ok bool
	err := r.pool.QueryRow(ctx, query, accountID, userID, permKeys).Scan(&ok)
	return ok, err
}

// ReplaceUserRoleAssignments substitui TODOS os papeis do user na account numa
// transacao: remove os user_role_assignments atuais e insere os roleIDs novos. Os
// roleIDs ja foram validados pelo service (pertencem a account + alvo e membro).
func (r *PostgresRBACRepository) ReplaceUserRoleAssignments(ctx context.Context, accountID, userID string, roleIDs []string) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err := tx.Exec(ctx, `
		delete from core.user_role_assignments
		where account_id = $1::uuid and user_id = $2::uuid
	`, accountID, userID); err != nil {
		return err
	}

	if len(roleIDs) > 0 {
		if _, err := tx.Exec(ctx, `
			insert into core.user_role_assignments (account_id, user_id, role_id)
			select $1::uuid, $2::uuid, rid::uuid
			from unnest($3::uuid[]) as t(rid)
			on conflict (account_id, user_id, role_id) do nothing
		`, accountID, userID, roleIDs); err != nil {
			return err
		}
	}

	return tx.Commit(ctx)
}
