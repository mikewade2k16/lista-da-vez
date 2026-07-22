package tenants

import (
	"strings"

	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
)

func buildListAccessibleQuery(principal auth.Principal, input ListInput) (string, []any) {
	activeClause := " and t.is_active = true"
	if input.IncludeInactive {
		activeClause = ""
	}
	moduleID := strings.TrimSpace(input.ModuleID)

	switch principal.Role {
	case auth.RolePlatformAdmin:
		args := []any{}
		moduleClause := ""
		if moduleID != "" {
			args = append(args, moduleID)
			moduleClause = listModuleScopeClause("$1")
		}
		return tenantSelectSQL() + `
			from core.accounts t
			where 1 = 1` + activeClause + moduleClause + `
			order by t.name asc;
		`, args
	case auth.RoleOwner, auth.RoleDirector, auth.RoleMarketing:
		args := []any{principal.UserID, tenantScopedCoreRoleCodes()}
		moduleClause := ""
		if moduleID != "" {
			args = append(args, moduleID)
			moduleClause = listModuleScopeClause("$3")
		}
		return tenantSelectDistinctSQL() + `
			from core.accounts t
			join core.account_users au
				on au.account_id = t.id
				and au.user_id = $1::uuid
				and au.is_active = true
			join core.user_role_assignments ura
				on ura.account_id = au.account_id
				and ura.user_id = au.user_id
			join core.roles r
				on r.id = ura.role_id
				and r.account_id = au.account_id
			where lower(r.code) = any($2::text[])
			` + activeClause + moduleClause + `
			order by t.name asc;
		`, args
	default:
		args := []any{principal.UserID, storeScopedCoreRoleCodes()}
		moduleClause := ""
		if moduleID != "" {
			args = append(args, moduleID)
			moduleClause = listModuleScopeClause("$3")
		}
		return tenantSelectDistinctSQL() + `
			from core.accounts t
			join core.account_users au
				on au.account_id = t.id
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
			) configured(store_id) on true
			join queue.stores s
				on s.tenant_id = t.id
				and s.id::text = configured.store_id
			where lower(r.code) = any($2::text[])
				and s.is_active = true
				` + activeClause + moduleClause + `
			order by t.name asc;
		`, args
	}
}

func listModuleScopeClause(placeholder string) string {
	return `
			and exists (
				select 1
				from core.account_modules am
				where am.account_id = t.id
					and am.module_id = ` + placeholder + `
					and am.enabled = true
			)`
}

func buildFindAccessibleQuery(principal auth.Principal, tenantID string) (string, []any) {
	switch principal.Role {
	case auth.RolePlatformAdmin:
		return tenantSelectSQL() + `
			from core.accounts t
			where t.id = $1::uuid;
		`, []any{tenantID}
	case auth.RoleOwner, auth.RoleDirector, auth.RoleMarketing:
		return tenantSelectDistinctSQL() + `
			from core.accounts t
			join core.account_users au
				on au.account_id = t.id
				and au.user_id = $2::uuid
				and au.is_active = true
			join core.user_role_assignments ura
				on ura.account_id = au.account_id
				and ura.user_id = au.user_id
			join core.roles r
				on r.id = ura.role_id
				and r.account_id = au.account_id
			where t.id = $1::uuid
				and lower(r.code) = any($3::text[]);
		`, []any{tenantID, principal.UserID, tenantScopedCoreRoleCodes()}
	default:
		return tenantSelectDistinctSQL() + `
			from core.accounts t
			join core.account_users au
				on au.account_id = t.id
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
			) configured(store_id) on true
			join queue.stores s
				on s.tenant_id = t.id
				and s.id::text = configured.store_id
			where t.id = $1::uuid
				and lower(r.code) = any($3::text[]);
		`, []any{tenantID, principal.UserID, storeScopedCoreRoleCodes()}
	}
}

func tenantSelectSQL() string {
	return `
		select
			t.id::text,
			t.slug,
			t.name,
			t.is_active,
			t.created_at,
			t.updated_at
	`
}

func tenantSelectDistinctSQL() string {
	return strings.Replace(tenantSelectSQL(), "\n\t\tselect", "\n\t\tselect distinct", 1)
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
