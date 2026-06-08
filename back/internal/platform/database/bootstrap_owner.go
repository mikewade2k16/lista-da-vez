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
		insert into core.accounts (slug, name, is_active)
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
		insert into queue.stores (
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
		insert into core.users (
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

	// U4b: papel/membership do owner inicial em core.* (sem legado).
	// accountId == tenantId (core.accounts.id == public.tenants.id na 0101).
	if _, err := tx.Exec(ctx, `
		insert into core.account_users (account_id, user_id, is_active, joined_at)
		values ($1::uuid, $2::uuid, true, now())
		on conflict (account_id, user_id) do nothing
	`, tenantID, userID); err != nil {
		return fmt.Errorf("bootstrap initial owner: core membership: %w", err)
	}
	if _, err := tx.Exec(ctx, `
		with ins as (
			insert into core.roles (account_id, cloned_from_template_id, code, label, description, is_locked)
			select $1::uuid, rt.id, 'queue.owner', rt.label, rt.description, rt.is_locked
			from core.role_templates rt
			where rt.id = 'queue.supervisor'
			on conflict (account_id, code) do nothing
			returning id
		),
		resolved as (
			select id from ins
			union
			select id from core.roles where account_id = $1::uuid and code = 'queue.owner'
		)
		insert into core.user_role_assignments (account_id, user_id, role_id)
		select $1::uuid, $2::uuid, (select id from resolved limit 1)
		where (select id from resolved limit 1) is not null
		on conflict (account_id, user_id, role_id) do nothing
	`, tenantID, userID); err != nil {
		return fmt.Errorf("bootstrap initial owner: core role assignment: %w", err)
	}
	if _, err := tx.Exec(ctx, `
		insert into core.role_permissions (role_id, permission_key)
		select r.id, rtp.permission_key
		from core.roles r
		join core.role_template_permissions rtp on rtp.role_template_id = r.cloned_from_template_id
		where r.account_id = $1::uuid and r.code = 'queue.owner'
		on conflict do nothing
	`, tenantID); err != nil {
		return fmt.Errorf("bootstrap initial owner: core role permissions: %w", err)
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
