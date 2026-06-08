package core

import (
	"context"
	"crypto/rand"
	"encoding/hex"
)

// Métodos secundários do AdminRepository: modules, stores, webhook.
// Separados de admin_repository.go para manter cada arquivo focado.

// ============================================================================
// Modules
// ============================================================================

func (r *PostgresAdminRepository) GetAccountModules(ctx context.Context, accountID string) ([]AccountModuleView, error) {
	const query = `
		select m.id, m.label, m.is_core, coalesce(am.enabled, false)
		from core.modules m
		left join core.account_modules am
		       on am.module_id = m.id and am.account_id = $1::uuid
		order by m.sort_order asc, m.id asc
	`
	rows, err := r.pool.Query(ctx, query, accountID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	modules := make([]AccountModuleView, 0)
	for rows.Next() {
		var mv AccountModuleView
		if err := rows.Scan(&mv.ModuleID, &mv.Label, &mv.IsCore, &mv.Enabled); err != nil {
			return nil, err
		}
		modules = append(modules, mv)
	}
	return modules, rows.Err()
}

func (r *PostgresAdminRepository) SetAccountModuleEnabled(ctx context.Context, accountID, moduleID string, enabled bool) error {
	_, err := r.pool.Exec(ctx, `
		insert into core.account_modules (account_id, module_id, enabled, enabled_at)
		values ($1::uuid, $2, $3, now())
		on conflict (account_id, module_id) do update set enabled = $3, enabled_at = now()
	`, accountID, moduleID, enabled)
	return err
}

// ============================================================================
// Stores
// ============================================================================

func (r *PostgresAdminRepository) GetAccountStores(ctx context.Context, accountID string) ([]StoreAdminView, error) {
	const query = `
		select id, code, name, city, is_active, coalesce(billing_amount, 0)
		from queue.stores
		where tenant_id = $1::uuid
		order by lower(name) asc
	`
	rows, err := r.pool.Query(ctx, query, accountID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	stores := make([]StoreAdminView, 0)
	for rows.Next() {
		var sv StoreAdminView
		if err := rows.Scan(&sv.ID, &sv.Code, &sv.Name, &sv.City, &sv.Active, &sv.BillingAmount); err != nil {
			return nil, err
		}
		stores = append(stores, sv)
	}
	return stores, rows.Err()
}

func (r *PostgresAdminRepository) SetStoreBillingAmount(ctx context.Context, storeID string, amount float64) error {
	_, err := r.pool.Exec(ctx,
		`update queue.stores set billing_amount = $2 where id = $1::uuid`,
		storeID, amount,
	)
	return err
}

// ============================================================================
// Webhook
// ============================================================================

func (r *PostgresAdminRepository) RotateWebhookKey(ctx context.Context, accountID string) (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	key := hex.EncodeToString(b)

	tag, err := r.pool.Exec(ctx,
		`update core.accounts set webhook_key = $2, updated_at = now() where id = $1::uuid`,
		accountID, key,
	)
	if err != nil {
		return "", err
	}
	if tag.RowsAffected() == 0 {
		return "", ErrAccountNotFound
	}
	return key, nil
}
