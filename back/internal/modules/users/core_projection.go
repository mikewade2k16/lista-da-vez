package users

import (
	"fmt"
	"strings"

	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
)

func buildScopedQuery(principal auth.Principal, input ListInput, userID string) (string, []any) {
	query := baseProjectedUsersQuery()
	args := []any{auth.OrderedCoreRoleCodesForCoarseResolution()}
	clauses := []string{"projected.role_code <> ''"}

	if accountID := activeAccountID(principal, input); accountID != "" {
		args = append(args, accountID)
		clauses = append(clauses, fmt.Sprintf("projected.tenant_id = $%d", len(args)))
	}

	if strings.TrimSpace(input.StoreID) != "" {
		args = append(args, strings.TrimSpace(input.StoreID))
		clauses = append(clauses, fmt.Sprintf("$%d = any(projected.store_ids)", len(args)))
	}

	if input.Role != "" {
		roleCodes := auth.CoreRoleCodesForCoarse(input.Role)
		if len(roleCodes) == 0 {
			roleCodes = []string{"__invalid_role__"}
		}
		args = append(args, roleCodes)
		clauses = append(clauses, fmt.Sprintf(
			"(projected.role_code = any($%d::text[]) or projected.role_template_id = any($%d::text[]))",
			len(args),
			len(args),
		))
	}

	if input.Active != nil {
		args = append(args, *input.Active)
		clauses = append(clauses, fmt.Sprintf("projected.is_active = $%d", len(args)))
	}

	if strings.TrimSpace(userID) != "" {
		args = append(args, strings.TrimSpace(userID))
		clauses = append(clauses, fmt.Sprintf("projected.id = $%d", len(args)))
	}

	query += `
		where ` + strings.Join(clauses, " and ") + `
		order by projected.created_at desc, projected.email asc
	`

	if strings.TrimSpace(userID) != "" {
		query += `
			limit 1
		`
	}

	query += `;
	`

	return query, args
}

func activeAccountID(principal auth.Principal, input ListInput) string {
	if strings.TrimSpace(principal.AccountID) != "" {
		return strings.TrimSpace(principal.AccountID)
	}

	if principal.Role != auth.RolePlatformAdmin && strings.TrimSpace(principal.TenantID) != "" {
		return strings.TrimSpace(principal.TenantID)
	}

	return strings.TrimSpace(input.TenantID)
}

func baseProjectedUsersQuery() string {
	return `
		select
			projected.id,
			projected.display_name,
			projected.nick,
			projected.email,
			projected.employee_code,
			projected.job_title,
			projected.role_code,
			projected.role_template_id,
			projected.tenant_id,
			projected.store_ids,
			projected.is_active,
			projected.has_password,
			projected.must_change_password,
			projected.managed_by,
			projected.managed_resource_id,
			projected.invitation_status,
			projected.invitation_expires_at,
			projected.created_at,
			projected.updated_at
		from (
			select
				u.id::text as id,
				u.display_name,
				coalesce(u.nick, '') as nick,
				lower(u.email) as email,
				coalesce(u.employee_code, '') as employee_code,
				coalesce(u.job_title, '') as job_title,
				u.is_active,
				coalesce(nullif(trim(u.password_hash), ''), '') <> '' as has_password,
				u.must_change_password,
				u.created_at,
				u.updated_at,
				coalesce(selected_role.role_code, '') as role_code,
				coalesce(selected_role.role_template_id, '') as role_template_id,
				au.account_id::text as tenant_id,
				coalesce(store_scope.store_ids, array[]::text[]) as store_ids,
				case
					when coalesce(consultant_link.consultant_id, '') <> '' and coalesce(consultant_link.is_active, false) then 'consultants'
					else ''
				end as managed_by,
				coalesce(consultant_link.consultant_id, '') as managed_resource_id,
				coalesce(invitation.status, '') as invitation_status,
				invitation.expires_at as invitation_expires_at
			from core.account_users au
			join core.accounts a on a.id = au.account_id and a.is_active = true
			join core.users u on u.id = au.user_id
			left join lateral (
				select
					lower(r.code) as role_code,
					lower(coalesce(r.cloned_from_template_id, '')) as role_template_id
				from core.user_role_assignments ura
				join core.roles r on r.id = ura.role_id
				where ura.account_id = au.account_id
					and ura.user_id = au.user_id
				order by
					coalesce(array_position($1::text[], lower(r.code)), 999),
					coalesce(array_position($1::text[], lower(coalesce(r.cloned_from_template_id, ''))), 999),
					ura.created_at asc,
					r.created_at asc
				limit 1
			) as selected_role on true
			left join core.user_module_settings queue_settings
				on queue_settings.user_id = u.id
				and queue_settings.module_id = 'queue'
			left join lateral (
				select array_agg(s.id::text order by s.created_at asc, s.code asc) as store_ids
				from jsonb_array_elements_text(
					coalesce(
						queue_settings.config #> array['storeIdsByAccount', au.account_id::text],
						'[]'::jsonb
					)
				) configured(store_id)
				join queue.stores s on s.id::text = configured.store_id
				where s.tenant_id = au.account_id
					and s.is_active = true
			) as store_scope on true
			left join lateral (
				select
					coalesce(c.id::text, '') as consultant_id,
					c.is_active
				from queue.consultants c
				where c.user_id = u.id
					and c.tenant_id = au.account_id
				order by c.is_active desc, c.updated_at desc
				limit 1
			) as consultant_link on true
			left join lateral (
				select
					case
						when ui.status = 'pending' and ui.expires_at < now() then 'expired'
						else ui.status
					end as status,
					ui.expires_at
				from user_invitations ui
				where ui.user_id = u.id
				order by ui.created_at desc
				limit 1
			) as invitation on true
			where au.is_active = true
		) as projected
	`
}
