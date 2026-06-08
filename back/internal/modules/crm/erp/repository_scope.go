package erp

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
)

func (repository *PostgresRepository) ResolveStoreScope(ctx context.Context, principal auth.Principal, requestedTenantID string, requestedStoreCode string) (StoreScope, error) {
	normalizedStoreCode := strings.TrimSpace(requestedStoreCode)
	if normalizedStoreCode == "" {
		return StoreScope{}, ErrStoreRequired
	}

	tenantID := strings.TrimSpace(requestedTenantID)
	if tenantID == "" {
		resolvedTenantID, err := repository.ResolveDefaultTenantID(ctx, principal)
		if err != nil {
			return StoreScope{}, err
		}
		tenantID = resolvedTenantID
	}

	allowed, err := repository.CanAccessTenant(ctx, principal, tenantID)
	if err != nil {
		return StoreScope{}, err
	}
	if !allowed {
		return StoreScope{}, ErrForbidden
	}

	if requiresStoreScopedFilter(principal.Role) && len(principal.StoreIDs) == 0 {
		return StoreScope{}, ErrForbidden
	}

	query := `
		select
			s.tenant_id::text,
			s.id::text,
			s.code,
			s.name,
			s.city,
			coalesce(last_file.store_cnpj, '')
		from queue.stores s
		left join lateral (
			select sf.store_cnpj
			from erp_sync_files sf
			where sf.tenant_id = s.tenant_id
			  and sf.store_id = s.id
			order by sf.imported_at desc
			limit 1
		) last_file on true
		where s.tenant_id = $1::uuid
		  and s.code = $2
		  and s.is_active = true
	`
	args := []any{tenantID, normalizedStoreCode}
	if requiresStoreScopedFilter(principal.Role) {
		query += ` and s.id = any($3::uuid[])`
		args = append(args, principal.StoreIDs)
	}
	query += ` limit 1;`

	var scope StoreScope
	err = repository.pool.QueryRow(ctx, query, args...).Scan(
		&scope.TenantID,
		&scope.StoreID,
		&scope.StoreCode,
		&scope.StoreName,
		&scope.StoreCity,
		&scope.StoreCNPJ,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return StoreScope{}, ErrStoreNotFound
		}
		return StoreScope{}, err
	}

	return scope, nil
}

func (repository *PostgresRepository) ResolveRootStoreScope(ctx context.Context, principal auth.Principal, requestedTenantID string, rootStoreCode string) (StoreScope, error) {
	normalizedRootStoreCode := strings.TrimSpace(rootStoreCode)
	if normalizedRootStoreCode == "" {
		return StoreScope{}, ErrStoreRequired
	}

	if requestedTenantID = strings.TrimSpace(requestedTenantID); requestedTenantID != "" {
		scope, err := repository.ResolveStoreScope(ctx, principal, requestedTenantID, normalizedRootStoreCode)
		if err == nil {
			return scope, nil
		}
		if !errors.Is(err, ErrStoreNotFound) {
			return StoreScope{}, err
		}
	}

	query := `
		select
			s.tenant_id::text,
			s.id::text,
			s.code,
			s.name,
			s.city,
			coalesce(last_file.store_cnpj, '')
		from queue.stores s
		join core.accounts t on t.id = s.tenant_id
		left join lateral (
			select sf.store_cnpj
			from erp_sync_files sf
			where sf.tenant_id = s.tenant_id
			  and sf.store_id = s.id
			order by sf.imported_at desc
			limit 1
		) last_file on true
		where s.code = $1
		  and s.is_active = true
		  and t.is_active = true
	`
	args := []any{normalizedRootStoreCode}

	switch principal.Role {
	case auth.RolePlatformAdmin:
		// Dev/platform admins can resolve the unique configured ERP root scope.
	case auth.RoleOwner, auth.RoleDirector, auth.RoleMarketing:
		query += erpRootAccountAccessPredicate(2)
		args = append(args, strings.TrimSpace(principal.UserID), strings.TrimSpace(principal.TenantID))
	default:
		query += erpRootStoreAccessPredicate(2)
		args = append(args, strings.TrimSpace(principal.UserID), strings.TrimSpace(principal.TenantID))
	}

	query += `
		order by s.created_at asc, s.id asc
		limit 2;
	`

	rows, err := repository.pool.Query(ctx, query, args...)
	if err != nil {
		return StoreScope{}, err
	}
	defer rows.Close()

	scopes := make([]StoreScope, 0, 2)
	for rows.Next() {
		var scope StoreScope
		if err := rows.Scan(
			&scope.TenantID,
			&scope.StoreID,
			&scope.StoreCode,
			&scope.StoreName,
			&scope.StoreCity,
			&scope.StoreCNPJ,
		); err != nil {
			return StoreScope{}, err
		}
		scopes = append(scopes, scope)
	}
	if err := rows.Err(); err != nil {
		return StoreScope{}, err
	}

	switch len(scopes) {
	case 0:
		exists, err := repository.rootStoreExists(ctx, normalizedRootStoreCode)
		if err != nil {
			return StoreScope{}, err
		}
		if exists {
			return StoreScope{}, ErrForbidden
		}
		return StoreScope{}, ErrStoreNotFound
	case 1:
		return scopes[0], nil
	default:
		return StoreScope{}, ErrTenantRequired
	}
}

func (repository *PostgresRepository) ResolveDefaultERPScope(ctx context.Context, principal auth.Principal, requestedTenantID string) (StoreScope, error) {
	tenantID := strings.TrimSpace(requestedTenantID)
	if tenantID == "" {
		resolvedTenantID, err := repository.ResolveDefaultTenantID(ctx, principal)
		if err != nil {
			return StoreScope{}, err
		}
		tenantID = resolvedTenantID
	}

	allowed, err := repository.CanAccessTenant(ctx, principal, tenantID)
	if err != nil {
		return StoreScope{}, err
	}
	if !allowed {
		return StoreScope{}, ErrForbidden
	}

	if requiresStoreScopedFilter(principal.Role) && len(principal.StoreIDs) == 0 {
		return StoreScope{}, ErrForbidden
	}

	query := `
		select
			s.tenant_id::text,
			s.id::text,
			s.code,
			s.name,
			s.city,
			coalesce(last_file.store_cnpj, '')
		from queue.stores s
		left join lateral (
			select sf.store_cnpj
			from erp_sync_files sf
			where sf.tenant_id = s.tenant_id
			  and sf.store_id = s.id
			order by sf.imported_at desc
			limit 1
		) last_file on true
		left join lateral (
			select
				count(*)::int as file_count,
				max(sf.imported_at) as last_imported_at
			from erp_sync_files sf
			where sf.tenant_id = s.tenant_id
			  and sf.store_id = s.id
		) erp_stats on true
		where s.tenant_id = $1::uuid
		  and s.is_active = true
	`
	args := []any{tenantID}
	if requiresStoreScopedFilter(principal.Role) {
		query += ` and s.id = any($2::uuid[])`
		args = append(args, principal.StoreIDs)
	}
	query += `
		order by
			case when coalesce(erp_stats.file_count, 0) > 0 then 0 else 1 end asc,
			coalesce(erp_stats.file_count, 0) desc,
			case when s.code ~ '^[0-9]+$' then 0 else 1 end asc,
			coalesce(erp_stats.last_imported_at, 'epoch'::timestamptz) desc,
			s.created_at asc,
			s.id asc
		limit 1;
	`

	var scope StoreScope
	err = repository.pool.QueryRow(ctx, query, args...).Scan(
		&scope.TenantID,
		&scope.StoreID,
		&scope.StoreCode,
		&scope.StoreName,
		&scope.StoreCity,
		&scope.StoreCNPJ,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return StoreScope{}, ErrStoreNotFound
		}
		return StoreScope{}, err
	}

	return scope, nil
}

func (repository *PostgresRepository) ListActiveStores(ctx context.Context) ([]StoreScope, error) {
	rows, err := repository.pool.Query(ctx, `
		select
			s.tenant_id::text,
			s.id::text,
			s.code,
			s.name,
			s.city,
			''
		from queue.stores s
		where s.is_active = true
		order by s.code asc, s.created_at asc, s.id asc;
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	stores := make([]StoreScope, 0, 32)
	for rows.Next() {
		var store StoreScope
		if err := rows.Scan(
			&store.TenantID,
			&store.StoreID,
			&store.StoreCode,
			&store.StoreName,
			&store.StoreCity,
			&store.StoreCNPJ,
		); err != nil {
			return nil, err
		}
		stores = append(stores, store)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return stores, nil
}

func (repository *PostgresRepository) CanAccessTenant(ctx context.Context, principal auth.Principal, tenantID string) (bool, error) {
	normalizedTenantID := strings.TrimSpace(tenantID)
	if normalizedTenantID == "" {
		return false, nil
	}

	if principalTenantID := strings.TrimSpace(principal.TenantID); principalTenantID != "" {
		if principalTenantID == normalizedTenantID {
			return true, nil
		}
	}

	var (
		query string
		args  []any
	)

	switch principal.Role {
	case auth.RolePlatformAdmin:
		query = `
			select exists(
				select 1
				from core.accounts t
				where t.id::text = $1
				  and t.is_active = true
			);
		`
		args = []any{normalizedTenantID}
	default:
		// Acesso ao tenant via membership unificada em core.account_users (a 0133
		// backfillou TODOS os papeis legados, tenant E store-scoped) OU membership
		// de agencia (core.organization_users). Cobre owner/director/marketing e
		// store-scoped num unico caminho. core.accounts.id == public.tenants.id.
		query = `
			select exists(
				select 1
				from core.accounts t
				where t.id::text = $1
				  and t.is_active = true
				  and (
					exists (
						select 1
						from core.account_users au
						where au.account_id = t.id
						  and au.user_id::text = $2
						  and au.is_active = true
					)
					or exists (
						select 1
						from core.accounts a
						join core.organizations o on o.id = a.organization_id
						join core.organization_users ou on ou.organization_id = o.id
						where a.id = t.id
						  and a.is_active = true
						  and o.is_active = true
						  and ou.user_id::text = $2
					)
				  )
			);
		`
		args = []any{normalizedTenantID, strings.TrimSpace(principal.UserID)}
	}

	var allowed bool
	if err := repository.pool.QueryRow(ctx, query, args...).Scan(&allowed); err != nil {
		return false, err
	}
	return allowed, nil
}

func (repository *PostgresRepository) rootStoreExists(ctx context.Context, rootStoreCode string) (bool, error) {
	var exists bool
	err := repository.pool.QueryRow(ctx, `
		select exists(
			select 1
			from queue.stores s
			join core.accounts t on t.id = s.tenant_id
			where s.code = $1
			  and s.is_active = true
			  and t.is_active = true
		);
	`, strings.TrimSpace(rootStoreCode)).Scan(&exists)
	return exists, err
}

func erpRootAccountAccessPredicate(firstArgIndex int) string {
	userArg := fmt.Sprintf("$%d", firstArgIndex)
	tenantArg := fmt.Sprintf("$%d", firstArgIndex+1)

	return fmt.Sprintf(`
		  and (
			(%[2]s <> '' and s.tenant_id::text = %[2]s)
			or exists (
				select 1
				from core.account_users au
				where au.account_id = s.tenant_id
				  and au.user_id::text = %[1]s
				  and au.is_active = true
			)
			or exists (
				select 1
				from core.accounts a
				join core.organizations o on o.id = a.organization_id
				join core.organization_users ou on ou.organization_id = o.id
				where a.id = s.tenant_id
				  and a.is_active = true
				  and o.is_active = true
				  and ou.user_id::text = %[1]s
			)
		  )
	`, userArg, tenantArg)
}

func erpRootStoreAccessPredicate(firstArgIndex int) string {
	userArg := fmt.Sprintf("$%d", firstArgIndex)
	tenantArg := fmt.Sprintf("$%d", firstArgIndex+1)

	return fmt.Sprintf(`
		  and (
			(%[2]s <> '' and s.tenant_id::text = %[2]s)
			or exists (
				select 1
				from core.account_users au
				where au.account_id = s.tenant_id
				  and au.user_id::text = %[1]s
				  and au.is_active = true
			)
		  )
	`, userArg, tenantArg)
}

func (repository *PostgresRepository) ResolveDefaultTenantID(ctx context.Context, principal auth.Principal) (string, error) {
	if tenantID := strings.TrimSpace(principal.TenantID); tenantID != "" {
		return tenantID, nil
	}

	var (
		query string
		args  []any
	)

	switch principal.Role {
	case auth.RolePlatformAdmin:
		query = `
			select t.id::text
			from core.accounts t
			where t.is_active = true
			order by t.name asc, t.created_at asc, t.id asc
			limit 2;
		`
	default:
		// Tenants do usuario via membership unificada em core.account_users (0133
		// backfillou tenant E store-scoped). Cobre owner/director/marketing e
		// store-scoped num unico caminho.
		query = `
			select distinct t.id::text
			from core.accounts t
			join core.account_users au on au.account_id = t.id
			where au.user_id = $1::uuid
			  and au.is_active = true
			  and t.is_active = true
			order by t.id asc
			limit 2;
		`
		args = []any{principal.UserID}
	}

	rows, err := repository.pool.Query(ctx, query, args...)
	if err != nil {
		return "", err
	}
	defer rows.Close()

	tenantIDs := make([]string, 0, 2)
	for rows.Next() {
		var tenantID string
		if err := rows.Scan(&tenantID); err != nil {
			return "", err
		}
		tenantIDs = append(tenantIDs, tenantID)
	}
	if err := rows.Err(); err != nil {
		return "", err
	}
	if len(tenantIDs) != 1 {
		return "", ErrTenantRequired
	}
	return tenantIDs[0], nil
}

func requiresStoreScopedFilter(role auth.Role) bool {
	switch role {
	case auth.RoleConsultant, auth.RoleManager, auth.RoleStoreTerminal:
		return true
	default:
		return false
	}
}
