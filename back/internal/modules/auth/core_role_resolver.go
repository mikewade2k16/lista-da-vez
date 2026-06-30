package auth

import (
	"context"
	"errors"
	"strings"

	"github.com/jackc/pgx/v5"
)

type authRoleScope struct {
	Role     Role
	TenantID string
	StoreIDs []string
}

// resolveAuthRoleScope resolve papel/escopo do usuario 100% pelo core
// (core.account_users + core.user_role_assignments + is_platform_admin). O
// fallback legado (user_*_roles) foi removido apos o DROP da 0135 — AUTH_ROLES_SOURCE
// e' sempre `core` em todos os ambientes.
func (store *PostgresUserStore) resolveAuthRoleScope(ctx context.Context, record userRecord) (authRoleScope, error) {
	scope, found, err := store.resolveCoreAuthRoleScope(ctx, record.ID)
	if err != nil {
		return authRoleScope{}, err
	}
	if !found {
		return authRoleScope{}, nil
	}
	return scope, nil
}

func (store *PostgresUserStore) resolveCoreAuthRoleScope(ctx context.Context, userID string) (authRoleScope, bool, error) {
	const query = `
		select
			u.is_platform_admin,
			coalesce(selected.account_id, '') as account_id,
			coalesce(selected.role_code, '') as role_code,
			coalesce(selected.template_id, '') as template_id
		from core.users u
		left join lateral (
			select
				au.account_id::text as account_id,
				lower(r.code) as role_code,
				lower(coalesce(r.cloned_from_template_id, '')) as template_id
			from core.account_users au
			join core.accounts a on a.id = au.account_id and a.is_active = true
			join core.user_role_assignments ura
				on ura.account_id = au.account_id
				and ura.user_id = au.user_id
			join core.roles r on r.id = ura.role_id
			where au.user_id = u.id
				and au.is_active = true
			order by case
				when lower(r.code) in ('queue.owner', 'core.owner', 'owner') then 1
				when lower(r.code) in ('queue.director', 'core.admin', 'director') then 2
				when lower(r.code) in ('queue.marketing', 'marketing') then 3
				when lower(r.code) in ('queue.manager', 'manager') then 4
				when lower(r.code) in ('queue.consultant', 'core.member', 'consultant') then 5
				when lower(r.code) in ('queue.store_terminal', 'store_terminal') then 6
				when lower(r.code) = 'queue.supervisor' then 7
				when lower(coalesce(r.cloned_from_template_id, '')) = 'queue.supervisor' then 8
				when lower(coalesce(r.cloned_from_template_id, '')) = 'queue.consultant' then 9
				else 99
			end,
			au.joined_at asc,
			ura.created_at asc,
			r.created_at asc
			limit 1
		) selected on true
		where u.id = $1::uuid
		limit 1;
	`

	var isPlatformAdmin bool
	var accountID string
	var roleCode string
	var templateID string
	err := store.pool.QueryRow(ctx, query, strings.TrimSpace(userID)).Scan(
		&isPlatformAdmin,
		&accountID,
		&roleCode,
		&templateID,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return authRoleScope{}, false, nil
		}
		return authRoleScope{}, false, err
	}

	if isPlatformAdmin {
		return authRoleScope{Role: RolePlatformAdmin}, true, nil
	}

	role := CoarseRoleFromCoreRole(roleCode, templateID)
	if role == "" || strings.TrimSpace(accountID) == "" {
		// account_users nao deu papel-coarse mapeavel (usuario so-agencia ou
		// so-papel-custom). Devolvemos escopo VAZIO: o login segue assim mesmo
		// (igual platform_admin com TenantID/AccountID vazios) e a conta-ativa
		// default + a autorizacao real vem DEPOIS, por account, da Etapa 2
		// (GET /v2/me/accounts + /v2/me/context?accountId=..., RBAC custom).
		//
		// DECISAO DE SEGURANCA (por que NAO preenchemos um TenantID-default por
		// org aqui): a autoridade do que o usuario ve mora na RBAC custom por
		// account (Etapa 2), nao num escopo-coarse derivado no login. Um TenantID
		// de login viraria "papel-coarse" e poderia herdar grants tenant-wide do
		// roleCatalog legado que o papel custom nao concede (over-grant). O
		// account_checker org-aware (Etapa 2) ja concede a conta-agencia ao usuario
		// de org pelo caminho correto (header X-Account-Id validado por requisicao).
		// (O atalho legado do CanAccessTenant — que confiava no principal.TenantID
		// sem rechecar membership — foi REMOVIDO; o acesso a tenant e sempre
		// rechecado no banco. Mesmo assim mantemos o escopo de login vazio: a fonte
		// de verdade do acesso e a RBAC custom por account.)
		return authRoleScope{}, false, nil
	}

	storeIDs, err := store.findCoreStoreIDs(ctx, strings.TrimSpace(userID), accountID, role)
	if err != nil {
		return authRoleScope{}, false, err
	}

	return authRoleScope{
		Role:     role,
		TenantID: strings.TrimSpace(accountID),
		StoreIDs: storeIDs,
	}, true, nil
}

func CoarseRoleFromCoreRole(roleCode string, templateID string) Role {
	switch strings.ToLower(strings.TrimSpace(roleCode)) {
	case "queue.owner", "core.owner", "owner":
		return RoleOwner
	case "queue.director", "core.admin", "director", "queue.supervisor":
		return RoleDirector
	case "queue.marketing", "marketing":
		return RoleMarketing
	case "queue.manager", "manager":
		return RoleManager
	case "queue.consultant", "core.member", "consultant":
		return RoleConsultant
	case "queue.store_terminal", "store_terminal":
		return RoleStoreTerminal
	}

	switch strings.ToLower(strings.TrimSpace(templateID)) {
	case "queue.supervisor":
		return RoleDirector
	case "queue.consultant":
		return RoleConsultant
	default:
		return ""
	}
}

func QueueCoreRoleCodeForCoarse(role Role) string {
	switch role {
	case RoleOwner:
		return "queue.owner"
	case RoleDirector:
		return "queue.director"
	case RoleMarketing:
		return "queue.marketing"
	case RoleManager:
		return "queue.manager"
	case RoleConsultant:
		return "queue.consultant"
	case RoleStoreTerminal:
		return "queue.store_terminal"
	default:
		return ""
	}
}

func QueueCoreRoleTemplateForCoarse(role Role) string {
	switch role {
	case RoleMarketing, RoleConsultant:
		return "queue.consultant"
	case RoleOwner, RoleDirector, RoleManager, RoleStoreTerminal:
		return "queue.supervisor"
	default:
		return ""
	}
}

func CoreRoleCodesForCoarse(role Role) []string {
	switch role {
	case RoleOwner:
		return []string{"queue.owner", "core.owner", "owner"}
	case RoleDirector:
		return []string{"queue.director", "core.admin", "director", "queue.supervisor"}
	case RoleMarketing:
		return []string{"queue.marketing", "marketing"}
	case RoleManager:
		return []string{"queue.manager", "manager"}
	case RoleConsultant:
		return []string{"queue.consultant", "core.member", "consultant"}
	case RoleStoreTerminal:
		return []string{"queue.store_terminal", "store_terminal"}
	default:
		return []string{}
	}
}

func OrderedCoreRoleCodesForCoarseResolution() []string {
	return []string{
		"queue.owner",
		"core.owner",
		"owner",
		"queue.director",
		"core.admin",
		"director",
		"queue.marketing",
		"marketing",
		"queue.manager",
		"manager",
		"queue.consultant",
		"core.member",
		"consultant",
		"queue.store_terminal",
		"store_terminal",
		"queue.supervisor",
	}
}

func QueueCompatibilityCoreRoleCodes() []string {
	return []string{
		"queue.owner",
		"queue.director",
		"queue.marketing",
		"queue.manager",
		"queue.consultant",
		"queue.store_terminal",
		"queue.supervisor",
		"core.owner",
		"core.admin",
		"core.member",
		"owner",
		"director",
		"marketing",
		"manager",
		"consultant",
		"store_terminal",
	}
}

func (store *PostgresUserStore) findCoreStoreIDs(ctx context.Context, userID string, accountID string, role Role) ([]string, error) {
	switch role {
	case RoleOwner, RoleDirector, RoleMarketing:
		return store.findActiveStoreIDsForAccount(ctx, accountID)
	case RoleManager, RoleConsultant, RoleStoreTerminal:
		storeIDs, err := store.findConfiguredStoreIDsForAccount(ctx, userID, accountID)
		if err != nil {
			return nil, err
		}
		if len(storeIDs) > 0 {
			return storeIDs, nil
		}

		activeStoreIDs, err := store.findActiveStoreIDsForAccount(ctx, accountID)
		if err != nil {
			return nil, err
		}
		if len(activeStoreIDs) == 1 {
			return activeStoreIDs, nil
		}
		return []string{}, nil
	default:
		return []string{}, nil
	}
}

func (store *PostgresUserStore) findActiveStoreIDsForAccount(ctx context.Context, accountID string) ([]string, error) {
	rows, err := store.pool.Query(ctx, `
		select s.id::text
		from queue.stores s
		where s.tenant_id = $1::uuid
			and s.is_active = true
		order by s.created_at asc, s.code asc;
	`, strings.TrimSpace(accountID))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	storeIDs := make([]string, 0)
	for rows.Next() {
		var storeID string
		if err := rows.Scan(&storeID); err != nil {
			return nil, err
		}
		storeIDs = append(storeIDs, storeID)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return storeIDs, nil
}

func (store *PostgresUserStore) findConfiguredStoreIDsForAccount(ctx context.Context, userID string, accountID string) ([]string, error) {
	rows, err := store.pool.Query(ctx, `
		with configured as (
			select jsonb_array_elements_text(
				coalesce(config #> array['storeIdsByAccount', $2::text], '[]'::jsonb)
			) as store_id
			from core.user_module_settings
			where user_id = $1::uuid
				and module_id = 'queue'
		)
		select s.id::text
		from configured c
		join queue.stores s on s.id::text = c.store_id
		where s.tenant_id = $2::uuid
			and s.is_active = true
		group by s.id, s.created_at, s.code
		order by s.created_at asc, s.code asc;
	`, strings.TrimSpace(userID), strings.TrimSpace(accountID))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	storeIDs := make([]string, 0)
	for rows.Next() {
		var storeID string
		if err := rows.Scan(&storeID); err != nil {
			return nil, err
		}
		storeIDs = append(storeIDs, storeID)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return storeIDs, nil
}
