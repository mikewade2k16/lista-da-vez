package database

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

type InitialOwnerBootstrapInput struct {
	TenantSlug        string
	TenantName        string
	StoreCode         string
	StoreName         string
	StoreCity         string
	OwnerName         string
	OwnerEmail        string
	OwnerPasswordHash string
}

func BootstrapInitialOwner(ctx context.Context, pool *pgxpool.Pool, input InitialOwnerBootstrapInput) error {
	tenantSlug := normalizeSlug(input.TenantSlug)
	tenantName := strings.TrimSpace(input.TenantName)
	storeCode := strings.ToUpper(strings.TrimSpace(input.StoreCode))
	storeName := strings.TrimSpace(input.StoreName)
	storeCity := strings.TrimSpace(input.StoreCity)
	ownerName := strings.TrimSpace(input.OwnerName)
	ownerEmail := strings.ToLower(strings.TrimSpace(input.OwnerEmail))
	ownerPasswordHash := strings.TrimSpace(input.OwnerPasswordHash)

	if tenantSlug == "" || tenantName == "" || storeCode == "" || storeName == "" || ownerName == "" || ownerEmail == "" || ownerPasswordHash == "" {
		return fmt.Errorf("bootstrap initial owner: missing required input")
	}

	tx, err := pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("bootstrap initial owner: begin tx: %w", err)
	}

	defer func() {
		_ = tx.Rollback(ctx)
	}()

	var tenantID string
	err = tx.QueryRow(ctx, `
		insert into tenants (slug, name, is_active)
		values ($1, $2, true)
		on conflict (slug) do update
		set
			name = excluded.name,
			is_active = true,
			updated_at = now()
		returning id::text;
	`, tenantSlug, tenantName).Scan(&tenantID)
	if err != nil {
		return fmt.Errorf("bootstrap initial owner: upsert tenant: %w", err)
	}

	var storeID string
	err = tx.QueryRow(ctx, `
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
	`, tenantID, storeCode, storeName, storeCity).Scan(&storeID)
	if err != nil {
		return fmt.Errorf("bootstrap initial owner: upsert store: %w", err)
	}

	var userID string
	err = tx.QueryRow(ctx, `
		insert into users (
			email,
			display_name,
			employee_code,
			job_title,
			password_hash,
			must_change_password,
			is_active
		)
		values (
			$1,
			$2,
			'',
			'Proprietario',
			$3,
			false,
			true
		)
		on conflict (lower(email)) do update
		set
			display_name = excluded.display_name,
			job_title = excluded.job_title,
			password_hash = excluded.password_hash,
			must_change_password = excluded.must_change_password,
			is_active = true,
			updated_at = now()
		returning id::text;
	`, ownerEmail, ownerName, ownerPasswordHash).Scan(&userID)
	if err != nil {
		return fmt.Errorf("bootstrap initial owner: upsert owner user: %w", err)
	}

	if _, err := tx.Exec(ctx, `delete from user_platform_roles where user_id = $1::uuid;`, userID); err != nil {
		return fmt.Errorf("bootstrap initial owner: clear platform roles: %w", err)
	}
	if _, err := tx.Exec(ctx, `delete from user_store_roles where user_id = $1::uuid;`, userID); err != nil {
		return fmt.Errorf("bootstrap initial owner: clear store roles: %w", err)
	}
	if _, err := tx.Exec(ctx, `delete from user_tenant_roles where user_id = $1::uuid;`, userID); err != nil {
		return fmt.Errorf("bootstrap initial owner: clear tenant roles: %w", err)
	}

	if _, err := tx.Exec(ctx, `
		insert into user_tenant_roles (user_id, tenant_id, role)
		values ($1::uuid, $2::uuid, 'owner');
	`, userID, tenantID); err != nil {
		return fmt.Errorf("bootstrap initial owner: assign owner role: %w", err)
	}

	if _, err := tx.Exec(ctx, `
		update user_invitations
		set
			status = 'revoked',
			revoked_at = now(),
			updated_at = now()
		where user_id = $1::uuid
			and status = 'pending';
	`, userID); err != nil {
		return fmt.Errorf("bootstrap initial owner: revoke pending invitations: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("bootstrap initial owner: commit: %w", err)
	}

	_ = storeID
	return nil
}

func normalizeSlug(value string) string {
	trimmed := strings.TrimSpace(strings.ToLower(value))
	if trimmed == "" {
		return ""
	}

	parts := strings.Fields(trimmed)
	return strings.Join(parts, "-")
}
