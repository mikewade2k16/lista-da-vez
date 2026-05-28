package database

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type ERPStoreBootstrapInput struct {
	TenantID   string
	TenantSlug string
	StoreCode  string
	StoreName  string
	StoreCity  string
}

type ERPStoreBootstrapResult struct {
	Bootstrapped bool
	Reason       string
	TenantID     string
	StoreID      string
	StoreCode    string
}

func BootstrapERPStore(ctx context.Context, pool *pgxpool.Pool, input ERPStoreBootstrapInput) (ERPStoreBootstrapResult, error) {
	storeCode := strings.ToUpper(strings.TrimSpace(input.StoreCode))
	if storeCode == "" {
		return ERPStoreBootstrapResult{Reason: "store_code_empty"}, nil
	}

	tenantID, reason, err := resolveERPBootstrapTenant(ctx, pool, input)
	if err != nil {
		return ERPStoreBootstrapResult{}, err
	}
	if tenantID == "" {
		return ERPStoreBootstrapResult{Reason: reason, StoreCode: storeCode}, nil
	}

	storeName := strings.TrimSpace(input.StoreName)
	if storeName == "" {
		storeName = fmt.Sprintf("Loja %s", storeCode)
	}
	storeCity := strings.TrimSpace(input.StoreCity)

	var storeID string
	if err := pool.QueryRow(ctx, `
		insert into stores (
			tenant_id,
			code,
			name,
			city,
			is_active
		)
		values (
			$1::uuid,
			$2,
			$3,
			$4,
			true
		)
		on conflict (tenant_id, code) do update
		set
			name = excluded.name,
			city = excluded.city,
			is_active = true,
			updated_at = now()
		returning id::text;
	`, tenantID, storeCode, storeName, storeCity).Scan(&storeID); err != nil {
		return ERPStoreBootstrapResult{}, fmt.Errorf("bootstrap erp store: upsert store: %w", err)
	}

	return ERPStoreBootstrapResult{
		Bootstrapped: true,
		TenantID:     tenantID,
		StoreID:      storeID,
		StoreCode:    storeCode,
	}, nil
}

func resolveERPBootstrapTenant(ctx context.Context, pool *pgxpool.Pool, input ERPStoreBootstrapInput) (string, string, error) {
	tenantID := strings.TrimSpace(input.TenantID)
	if tenantID != "" {
		return resolveERPBootstrapTenantByQuery(ctx, pool, `
			select id::text
			from tenants
			where id = $1::uuid
			  and is_active = true
			limit 1;
		`, tenantID, "tenant_id_not_found")
	}

	tenantSlug := normalizeSlug(input.TenantSlug)
	if tenantSlug != "" {
		return resolveERPBootstrapTenantByQuery(ctx, pool, `
			select id::text
			from tenants
			where slug = $1
			  and is_active = true
			limit 1;
		`, tenantSlug, "tenant_slug_not_found")
	}

	rows, err := pool.Query(ctx, `
		select id::text
		from tenants
		where is_active = true
		order by created_at asc, id asc
		limit 2;
	`)
	if err != nil {
		return "", "", fmt.Errorf("bootstrap erp store: resolve default tenant: %w", err)
	}
	defer rows.Close()

	tenantIDs := make([]string, 0, 2)
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return "", "", fmt.Errorf("bootstrap erp store: scan default tenant: %w", err)
		}
		tenantIDs = append(tenantIDs, id)
	}
	if err := rows.Err(); err != nil {
		return "", "", fmt.Errorf("bootstrap erp store: iterate default tenant: %w", err)
	}

	switch len(tenantIDs) {
	case 0:
		return "", "no_active_tenant", nil
	case 1:
		return tenantIDs[0], "", nil
	default:
		return "", "ambiguous_tenant", nil
	}
}

func resolveERPBootstrapTenantByQuery(ctx context.Context, pool *pgxpool.Pool, query string, arg string, notFoundReason string) (string, string, error) {
	var tenantID string
	err := pool.QueryRow(ctx, query, arg).Scan(&tenantID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", notFoundReason, nil
		}
		return "", "", fmt.Errorf("bootstrap erp store: resolve tenant: %w", err)
	}
	return tenantID, "", nil
}
