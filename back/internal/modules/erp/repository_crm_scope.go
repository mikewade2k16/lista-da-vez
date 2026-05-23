package erp

import (
	"context"
	"strings"
)

func crmPrimaryStoreKeyFromSlug(slug string) string {
	switch strings.TrimSpace(slug) {
	case "riomar":
		return "12583959000186"
	case "jardins":
		return "56173889000163"
	case "garcia":
		return "53578278000107"
	case "treze":
		return "43068099000176"
	default:
		return ""
	}
}

func crmStoreKeyFromOperationalStore(code string, name string) string {
	slug, _ := crmStoreSlugFromOperationalStore(code, name)
	return crmPrimaryStoreKeyFromSlug(slug)
}

func crmEmployeeSpecialStoreKey(employeeID string) string {
	return strings.TrimSpace(crmEmployeeSpecialStoreKeys[strings.TrimSpace(employeeID)])
}

func resolveCRMOrderStoreKey(explicitStoreKey string, fallbackStoreCNPJ string, employeeID string, employeeStoreFallbacks map[string]string, employeeDominantStoreKeys map[string]string) string {
	if normalized := onlyDigits(strings.TrimSpace(explicitStoreKey)); normalized != "" {
		return normalized
	}

	normalizedEmployeeID := strings.TrimSpace(employeeID)
	if normalizedEmployeeID != "" {
		if specialKey := crmEmployeeSpecialStoreKey(normalizedEmployeeID); specialKey != "" {
			return specialKey
		}
		if normalized := onlyDigits(employeeStoreFallbacks[normalizedEmployeeID]); normalized != "" {
			return normalized
		}
		if normalized := onlyDigits(employeeDominantStoreKeys[normalizedEmployeeID]); normalized != "" {
			return normalized
		}
	}

	return onlyDigits(strings.TrimSpace(fallbackStoreCNPJ))
}

func (repository *PostgresRepository) listCRMStoreTargets(ctx context.Context, tenantID string) (map[string]crmStoreTarget, error) {
	rows, err := repository.pool.Query(ctx, `
		select
			code,
			name,
			coalesce(round(monthly_goal * 100), 0)::bigint,
			coalesce(round(avg_ticket_goal * 100), 0)::bigint,
			coalesce(pa_goal, 0)::float8
		from stores
		where tenant_id = $1::uuid
		  and is_active = true;
	`, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	targets := make(map[string]crmStoreTarget, 4)
	for rows.Next() {
		var (
			code               string
			name               string
			monthlyGoalCents   int64
			avgTicketGoalCents int64
			paGoal             float64
		)
		if err := rows.Scan(&code, &name, &monthlyGoalCents, &avgTicketGoalCents, &paGoal); err != nil {
			return nil, err
		}

		slug, label := crmStoreSlugFromOperationalStore(code, name)
		if slug == "" {
			continue
		}

		targets[slug] = crmStoreTarget{
			Slug:               slug,
			Label:              label,
			Code:               code,
			Name:               name,
			MonthlyGoalCents:   monthlyGoalCents,
			AvgTicketGoalCents: avgTicketGoalCents,
			PAGoal:             paGoal,
		}
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return targets, nil
}

func (repository *PostgresRepository) listCRMEmployeeStoreFallbacks(ctx context.Context, tenantID string) (map[string]string, error) {
	rows, err := repository.pool.Query(ctx, `
		select
			trim(u.employee_code) as employee_code,
			coalesce(max(nullif(consultant_store.code, '')), max(nullif(role_store.code, '')), '') as store_code,
			coalesce(max(nullif(consultant_store.name, '')), max(nullif(role_store.name, '')), '') as store_name
		from users u
		left join consultants c
			on c.user_id = u.id
		   and c.tenant_id = $1::uuid
		left join stores consultant_store
			on consultant_store.id = c.store_id
		   and consultant_store.tenant_id = $1::uuid
		left join user_store_roles usr
			on usr.user_id = u.id
		left join stores role_store
			on role_store.id = usr.store_id
		   and role_store.tenant_id = $1::uuid
		where nullif(trim(u.employee_code), '') is not null
		group by trim(u.employee_code);
	`, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	fallbacks := make(map[string]string)
	for rows.Next() {
		var employeeCode string
		var storeCode string
		var storeName string
		if err := rows.Scan(&employeeCode, &storeCode, &storeName); err != nil {
			return nil, err
		}

		employeeCode = strings.TrimSpace(employeeCode)
		if employeeCode == "" {
			continue
		}

		if storeKey := crmStoreKeyFromOperationalStore(storeCode, storeName); storeKey != "" {
			fallbacks[employeeCode] = storeKey
		}
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return fallbacks, nil
}

func (repository *PostgresRepository) listCRMDominantEmployeeStoreKeys(ctx context.Context, store StoreScope) (map[string]string, error) {
	rows, err := repository.pool.Query(ctx, `
		with known_orders as (
			select
				order_id,
				coalesce(max(nullif(trim(store_id_raw), '')), '') as explicit_store_key,
				coalesce(max(nullif(trim(employee_id), '')), '') as employee_id,
				case
					when max(total_amount_cents) > 0 then max(total_amount_cents)::bigint
					else sum(amount_cents)::bigint
				end as order_total_cents
			from erp_order_raw
			where tenant_id = $1::uuid
			  and store_id = $2::uuid
			  and nullif(trim(order_id), '') is not null
			  and coalesce(nullif(trim(store_id_raw), ''), '') <> ''
			group by order_id
		), canceled_orders as (
			select distinct order_id
			from erp_order_canceled_raw
			where tenant_id = $1::uuid
			  and store_id = $2::uuid
		), active_known_orders as (
			select *
			from known_orders known
			where not exists (
				select 1
				from canceled_orders canceled
				where canceled.order_id = known.order_id
			)
		), ranked as (
			select
				employee_id,
				explicit_store_key,
				row_number() over (
					partition by employee_id
					order by count(*) desc, sum(order_total_cents) desc, explicit_store_key asc
				) as row_number
			from active_known_orders
			where employee_id <> ''
			group by employee_id, explicit_store_key
		)
		select employee_id, explicit_store_key
		from ranked
		where row_number = 1;
	`, store.TenantID, store.StoreID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	dominantKeys := make(map[string]string)
	for rows.Next() {
		var employeeID string
		var storeKey string
		if err := rows.Scan(&employeeID, &storeKey); err != nil {
			return nil, err
		}

		employeeID = strings.TrimSpace(employeeID)
		storeKey = onlyDigits(strings.TrimSpace(storeKey))
		if employeeID == "" || storeKey == "" {
			continue
		}

		dominantKeys[employeeID] = storeKey
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return dominantKeys, nil
}

func (repository *PostgresRepository) listCRMEmployeeNames(ctx context.Context, store StoreScope) (map[string]string, error) {
	rows, err := repository.pool.Query(ctx, `
		select distinct on (original_id)
			original_id,
			name
		from erp_employee_raw
		where tenant_id = $1::uuid
		  and store_id = $2::uuid
		  and nullif(trim(original_id), '') is not null
		order by original_id, created_at_imported desc, source_batch_date desc, id desc;
	`, store.TenantID, store.StoreID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	names := make(map[string]string)
	for rows.Next() {
		var employeeID string
		var name string
		if err := rows.Scan(&employeeID, &name); err != nil {
			return nil, err
		}

		employeeID = strings.TrimSpace(employeeID)
		name = strings.TrimSpace(name)
		if employeeID == "" || name == "" {
			continue
		}

		names[employeeID] = name
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return names, nil
}
