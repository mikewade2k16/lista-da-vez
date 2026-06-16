package consultants

import (
	"context"
	"strings"
)

// ListStoreStaff retorna os membros das lojas informadas que NAO operam a fila
// (ou seja, todo papel store-scoped diferente de consultor). A fonte e o modelo
// core RBAC: membership ativa (core.account_users), papel atribuido
// (core.user_role_assignments -> core.roles) e o escopo de loja por usuario
// gravado no JSONB de core.user_module_settings(module_id='queue') em
// config -> storeIdsByAccount -> <accountId>. O nome da loja vem de queue.stores.
//
// Defesa em profundidade: a query so considera lojas em storeIDs (ja validadas
// contra o Principal pelo service). Sem N+1: uma unica query agregada com
// s.id = any($1).
func (repository *PostgresRepository) ListStoreStaff(ctx context.Context, storeIDs []string) ([]StoreStaffMember, error) {
	if len(storeIDs) == 0 {
		return []StoreStaffMember{}, nil
	}

	rows, err := repository.pool.Query(ctx, `
		select distinct
			u.id::text as user_id,
			coalesce(u.display_name, u.email) as name,
			lower(r.code) as role_code,
			coalesce(r.cloned_from_template_id, '') as role_template,
			coalesce(r.label, '') as role_label,
			s.id::text as store_id,
			s.name as store_name
		from queue.stores s
		join core.account_users au
			on au.account_id = s.tenant_id
			and au.is_active = true
		join core.user_role_assignments ura
			on ura.account_id = au.account_id
			and ura.user_id = au.user_id
		join core.roles r
			on r.id = ura.role_id
			and r.account_id = au.account_id
		join core.users u
			on u.id = au.user_id
			and u.is_active = true
		join core.user_module_settings queue_settings
			on queue_settings.user_id = au.user_id
			and queue_settings.module_id = 'queue'
		join lateral jsonb_array_elements_text(
			coalesce(
				queue_settings.config #> array['storeIdsByAccount', au.account_id::text],
				'[]'::jsonb
			)
		) configured(store_id) on configured.store_id = s.id::text
		where s.id = any($1::uuid[])
			and s.is_active = true
			and lower(r.code) <> all($2::text[])
			and lower(coalesce(r.cloned_from_template_id, '')) <> all($3::text[])
		order by s.name asc, name asc, role_code asc;
	`, storeIDs, queueConsultantRoleCodes(), queueConsultantTemplateCodes())
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	members := make([]StoreStaffMember, 0)
	for rows.Next() {
		var member StoreStaffMember
		var roleTemplate *string
		var roleLabel *string
		if err := rows.Scan(
			&member.UserID,
			&member.Name,
			&member.RoleCode,
			&roleTemplate,
			&roleLabel,
			&member.StoreID,
			&member.StoreName,
		); err != nil {
			return nil, err
		}

		member.UserID = strings.TrimSpace(member.UserID)
		member.Name = strings.TrimSpace(member.Name)
		member.RoleCode = strings.ToLower(strings.TrimSpace(member.RoleCode))
		if roleTemplate != nil {
			member.RoleTemplate = strings.ToLower(strings.TrimSpace(*roleTemplate))
		}
		if roleLabel != nil {
			member.RoleLabel = strings.TrimSpace(*roleLabel)
		}
		member.StoreID = strings.TrimSpace(member.StoreID)
		member.StoreName = strings.TrimSpace(member.StoreName)

		members = append(members, member)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return members, nil
}

// ListAccessibleStoreIDsForTenant resolve todas as lojas ativas de um tenant.
// Usado apenas para principal platform_admin (StoreIDs vem vazio no token);
// para os demais papeis o escopo ja chega resolvido em principal.StoreIDs.
func (repository *PostgresRepository) ListAccessibleStoreIDsForTenant(ctx context.Context, tenantID string) ([]string, error) {
	trimmed := strings.TrimSpace(tenantID)
	if trimmed == "" {
		return []string{}, nil
	}

	rows, err := repository.pool.Query(ctx, `
		select s.id::text
		from queue.stores s
		where s.tenant_id = $1::uuid
			and s.is_active = true
		order by s.created_at asc, s.code asc;
	`, trimmed)
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
		storeIDs = append(storeIDs, strings.TrimSpace(storeID))
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return storeIDs, nil
}
