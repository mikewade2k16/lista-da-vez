package stores

import (
	"fmt"
	"strings"

	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
)

func buildListAccessibleQuery(principal auth.Principal, input ListInput) (string, []any) {
	tenantID := strings.TrimSpace(input.TenantID)
	activeClause := ""
	if !input.IncludeInactive {
		activeClause = " and s.is_active = true"
	}

	switch principal.Role {
	case auth.RolePlatformAdmin:
		if tenantID != "" {
			return storeSelectSQL() + `
				from queue.stores s
				where s.tenant_id = $1::uuid
				` + activeClause + `
				order by s.created_at asc, s.code asc;
			`, []any{tenantID}
		}

		return storeSelectSQL() + `
			from queue.stores s
			where 1 = 1
			` + activeClause + `
			order by s.created_at asc, s.code asc;
		`, nil
	case auth.RoleOwner, auth.RoleDirector, auth.RoleMarketing:
		query := storeSelectDistinctSQL() + `
			from queue.stores s
			join core.account_users au
				on au.account_id = s.tenant_id
				and au.user_id = $1::uuid
				and au.is_active = true
			join core.user_role_assignments ura
				on ura.account_id = au.account_id
				and ura.user_id = au.user_id
			join core.roles r
				on r.id = ura.role_id
				and r.account_id = au.account_id
			where lower(r.code) = any($2::text[])
		`
		query += activeClause
		args := []any{principal.UserID, tenantScopedCoreRoleCodes()}
		if tenantID != "" {
			args = append(args, tenantID)
			query += fmt.Sprintf(`
				and s.tenant_id = $%d::uuid
			`, len(args))
		}

		query += `
			order by s.created_at asc, s.code asc;
		`

		return query, args
	default:
		query := storeSelectDistinctSQL() + `
			from queue.stores s
			join core.account_users au
				on au.account_id = s.tenant_id
				and au.user_id = $1::uuid
				and au.is_active = true
			join core.user_role_assignments ura
				on ura.account_id = au.account_id
				and ura.user_id = au.user_id
			join core.roles r
				on r.id = ura.role_id
				and r.account_id = au.account_id
			join core.user_module_settings queue_settings
				on queue_settings.user_id = au.user_id
				and queue_settings.module_id = 'queue'
			join lateral jsonb_array_elements_text(
				coalesce(
					queue_settings.config #> array['storeIdsByAccount', au.account_id::text],
					'[]'::jsonb
				)
			) configured(store_id) on configured.store_id = s.id::text
			where lower(r.code) = any($2::text[])
		`
		query += activeClause
		args := []any{principal.UserID, storeScopedCoreRoleCodes()}
		if tenantID != "" {
			args = append(args, tenantID)
			query += fmt.Sprintf(`
				and s.tenant_id = $%d::uuid
			`, len(args))
		}

		query += `
			order by s.created_at asc, s.code asc;
		`

		return query, args
	}
}

func buildFindAccessibleQuery(principal auth.Principal, storeID string) (string, []any) {
	switch principal.Role {
	case auth.RolePlatformAdmin:
		return storeSelectSQL() + `
			from queue.stores s
			where s.id = $1::uuid
			limit 1;
		`, []any{storeID}
	case auth.RoleOwner, auth.RoleDirector, auth.RoleMarketing:
		return storeSelectDistinctSQL() + `
			from queue.stores s
			join core.account_users au
				on au.account_id = s.tenant_id
				and au.user_id = $2::uuid
				and au.is_active = true
			join core.user_role_assignments ura
				on ura.account_id = au.account_id
				and ura.user_id = au.user_id
			join core.roles r
				on r.id = ura.role_id
				and r.account_id = au.account_id
			where s.id = $1::uuid
				and lower(r.code) = any($3::text[])
			limit 1;
		`, []any{storeID, principal.UserID, tenantScopedCoreRoleCodes()}
	default:
		return storeSelectDistinctSQL() + `
			from queue.stores s
			join core.account_users au
				on au.account_id = s.tenant_id
				and au.user_id = $2::uuid
				and au.is_active = true
			join core.user_role_assignments ura
				on ura.account_id = au.account_id
				and ura.user_id = au.user_id
			join core.roles r
				on r.id = ura.role_id
				and r.account_id = au.account_id
			join core.user_module_settings queue_settings
				on queue_settings.user_id = au.user_id
				and queue_settings.module_id = 'queue'
			join lateral jsonb_array_elements_text(
				coalesce(
					queue_settings.config #> array['storeIdsByAccount', au.account_id::text],
					'[]'::jsonb
				)
			) configured(store_id) on configured.store_id = s.id::text
			where s.id = $1::uuid
				and lower(r.code) = any($3::text[])
			limit 1;
		`, []any{storeID, principal.UserID, storeScopedCoreRoleCodes()}
	}
}

func storeSelectSQL() string {
	return `
		select
			s.id::text,
			s.tenant_id::text,
			s.code,
			s.name,
			s.city,
			s.default_template_id,
			s.store_type,
			s.monthly_goal,
			s.weekly_goal,
			s.avg_ticket_goal,
			s.conversion_goal,
			s.pa_goal,
			s.is_active,
			s.created_at,
			s.updated_at
	`
}

func storeSelectDistinctSQL() string {
	return strings.Replace(storeSelectSQL(), "\n\t\tselect", "\n\t\tselect distinct", 1)
}

func tenantScopedCoreRoleCodes() []string {
	return coreRoleCodesForRoles(auth.RoleOwner, auth.RoleDirector, auth.RoleMarketing)
}

func storeScopedCoreRoleCodes() []string {
	return coreRoleCodesForRoles(auth.RoleManager, auth.RoleConsultant, auth.RoleStoreTerminal)
}

func coreRoleCodesForRoles(roles ...auth.Role) []string {
	seen := map[string]struct{}{}
	roleCodes := make([]string, 0)
	for _, role := range roles {
		for _, roleCode := range auth.CoreRoleCodesForCoarse(role) {
			normalized := strings.ToLower(strings.TrimSpace(roleCode))
			if normalized == "" {
				continue
			}
			if _, exists := seen[normalized]; exists {
				continue
			}
			seen[normalized] = struct{}{}
			roleCodes = append(roleCodes, normalized)
		}
	}
	return roleCodes
}
