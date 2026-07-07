package finance

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PostgresConfigStore persiste config (categorias/contas fixas/recorrencias) e
// serve o read model de clientes com billing (recurring-clients).
type PostgresConfigStore struct {
	pool *pgxpool.Pool
}

// NewPostgresConfigStore cria o store.
func NewPostgresConfigStore(pool *pgxpool.Pool) *PostgresConfigStore {
	return &PostgresConfigStore{pool: pool}
}

// GetConfig le a config do escopo (account + coreTenantId). Slices nao-nil.
func (r *PostgresConfigStore) GetConfig(ctx context.Context, accountID, coreTenantID string) (ConfigData, error) {
	out := ConfigData{
		CoreTenantID:     coreTenantID,
		Categories:       []Category{},
		FixedAccounts:    []FixedAccount{},
		RecurringEntries: []RecurringEntry{},
	}

	catRows, err := r.pool.Query(ctx, `
		select id::text, name, kind, description from finance.categories
		where account_id = $1::uuid and core_tenant_id = $2 order by position`, accountID, coreTenantID)
	if err != nil {
		return ConfigData{}, err
	}
	func() {
		defer catRows.Close()
		for catRows.Next() {
			var c Category
			if err = catRows.Scan(&c.ID, &c.Name, &c.Kind, &c.Description); err != nil {
				return
			}
			out.Categories = append(out.Categories, c)
		}
		err = catRows.Err()
	}()
	if err != nil {
		return ConfigData{}, err
	}

	if err := r.loadFixedAccounts(ctx, accountID, coreTenantID, &out); err != nil {
		return ConfigData{}, err
	}

	recRows, err := r.pool.Query(ctx, `
		select source_core_tenant_id, adjustment_amount, notes from finance.recurring_entries
		where account_id = $1::uuid and core_tenant_id = $2 order by id`, accountID, coreTenantID)
	if err != nil {
		return ConfigData{}, err
	}
	func() {
		defer recRows.Close()
		for recRows.Next() {
			var e RecurringEntry
			if err = recRows.Scan(&e.SourceCoreTenantID, &e.AdjustmentAmount, &e.Notes); err != nil {
				return
			}
			out.RecurringEntries = append(out.RecurringEntries, e)
		}
		err = recRows.Err()
	}()
	if err != nil {
		return ConfigData{}, err
	}

	var updated *time.Time
	if err := r.pool.QueryRow(ctx, `
		select updated_at from finance.config_state
		where account_id = $1::uuid and core_tenant_id = $2`, accountID, coreTenantID).Scan(&updated); err != nil {
		if err != pgx.ErrNoRows {
			return ConfigData{}, err
		}
	}
	if updated != nil {
		out.UpdatedAt = tsString(*updated)
	} else {
		out.UpdatedAt = tsString(time.Now())
	}
	return out, nil
}

// loadFixedAccounts carrega contas fixas + membros do escopo.
func (r *PostgresConfigStore) loadFixedAccounts(ctx context.Context, accountID, coreTenantID string, out *ConfigData) error {
	rows, err := r.pool.Query(ctx, `
		select id::text, name, kind, category_id, default_amount, notes from finance.fixed_accounts
		where account_id = $1::uuid and core_tenant_id = $2 order by position`, accountID, coreTenantID)
	if err != nil {
		return err
	}
	ids := make([]string, 0)
	byID := make(map[string]*FixedAccount)
	func() {
		defer rows.Close()
		for rows.Next() {
			var a FixedAccount
			if err = rows.Scan(&a.ID, &a.Name, &a.Kind, &a.CategoryID, &a.DefaultAmount, &a.Notes); err != nil {
				return
			}
			a.Members = []FixedAccountMember{}
			out.FixedAccounts = append(out.FixedAccounts, a)
			byID[a.ID] = &out.FixedAccounts[len(out.FixedAccounts)-1]
			ids = append(ids, a.ID)
		}
		err = rows.Err()
	}()
	if err != nil {
		return err
	}
	if len(ids) == 0 {
		return nil
	}
	mRows, err := r.pool.Query(ctx, `
		select fixed_account_id::text, id::text, name, amount from finance.fixed_account_members
		where fixed_account_id = any($1::uuid[]) order by fixed_account_id, position`, ids)
	if err != nil {
		return err
	}
	defer mRows.Close()
	for mRows.Next() {
		var faID string
		var m FixedAccountMember
		if err := mRows.Scan(&faID, &m.ID, &m.Name, &m.Amount); err != nil {
			return err
		}
		if a := byID[faID]; a != nil {
			a.Members = append(a.Members, m)
		}
	}
	return mRows.Err()
}

// SaveConfig faz o full-replace da config do escopo numa transacao.
func (r *PostgresConfigStore) SaveConfig(ctx context.Context, accountID string, d ConfigData) (ConfigData, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return ConfigData{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	scope := []any{accountID, d.CoreTenantID}
	for _, del := range []string{
		`delete from finance.categories where account_id = $1::uuid and core_tenant_id = $2`,
		`delete from finance.fixed_accounts where account_id = $1::uuid and core_tenant_id = $2`,
		`delete from finance.recurring_entries where account_id = $1::uuid and core_tenant_id = $2`,
	} {
		if _, err := tx.Exec(ctx, del, scope...); err != nil {
			return ConfigData{}, err
		}
	}

	for i, c := range d.Categories {
		if _, err := tx.Exec(ctx, `
			insert into finance.categories (id, account_id, core_tenant_id, name, kind, description, position)
			values (`+fmt.Sprintf(newIDExpr, 1)+`, $2::uuid, $3, $4, $5, $6, $7)`,
			c.ID, accountID, d.CoreTenantID, c.Name, c.Kind, c.Description, i); err != nil {
			return ConfigData{}, err
		}
	}
	if err := insertFixedAccounts(ctx, tx, accountID, d); err != nil {
		return ConfigData{}, err
	}
	for _, e := range d.RecurringEntries {
		if _, err := tx.Exec(ctx, `
			insert into finance.recurring_entries
			  (account_id, core_tenant_id, source_core_tenant_id, adjustment_amount, notes)
			values ($1::uuid, $2, $3, $4, $5)`,
			accountID, d.CoreTenantID, e.SourceCoreTenantID, e.AdjustmentAmount, e.Notes); err != nil {
			return ConfigData{}, err
		}
	}
	if _, err := tx.Exec(ctx, `
		insert into finance.config_state (account_id, core_tenant_id, updated_at)
		values ($1::uuid, $2, now())
		on conflict (account_id, core_tenant_id) do update set updated_at = now()`,
		accountID, d.CoreTenantID); err != nil {
		return ConfigData{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return ConfigData{}, err
	}
	return r.GetConfig(ctx, accountID, d.CoreTenantID)
}

// insertFixedAccounts insere contas fixas + membros na ordem do array (position).
func insertFixedAccounts(ctx context.Context, tx pgx.Tx, accountID string, d ConfigData) error {
	for i, a := range d.FixedAccounts {
		var faID string
		if err := tx.QueryRow(ctx, `
			insert into finance.fixed_accounts
			  (id, account_id, core_tenant_id, name, kind, category_id, default_amount, notes, position)
			values (`+fmt.Sprintf(newIDExpr, 1)+`, $2::uuid, $3, $4, $5, $6, $7, $8, $9)
			returning id::text`,
			a.ID, accountID, d.CoreTenantID, a.Name, a.Kind, a.CategoryID, a.DefaultAmount, a.Notes, i).Scan(&faID); err != nil {
			return err
		}
		for j, m := range a.Members {
			if _, err := tx.Exec(ctx, `
				insert into finance.fixed_account_members (id, fixed_account_id, name, amount, position)
				values (`+fmt.Sprintf(newIDExpr, 1)+`, $2::uuid, $3, $4, $5)`,
				m.ID, faID, m.Name, m.Amount, j); err != nil {
				return err
			}
		}
	}
	return nil
}

// ListRecurringClients devolve contas com mensalidade/billing por loja. Chamado
// apenas para platform_admin (o handler decide); read model core.accounts + queue.stores.
func (r *PostgresConfigStore) ListRecurringClients(ctx context.Context) ([]RecurringClient, error) {
	rows, err := r.pool.Query(ctx, `
		select a.id::text, a.name, a.monthly_payment_amount, a.payment_due_day, a.billing_mode
		from core.accounts a
		where a.is_active = true
		  and (a.monthly_payment_amount > 0 or a.billing_mode = 'per_store')
		order by lower(a.name)`)
	if err != nil {
		return nil, err
	}
	out := make([]RecurringClient, 0)
	ids := make([]string, 0)
	func() {
		defer rows.Close()
		for rows.Next() {
			var c RecurringClient
			var dueDay *int
			if err = rows.Scan(&c.ID, &c.Name, &c.MonthlyPaymentAmount, &dueDay, &c.BillingMode); err != nil {
				return
			}
			c.CoreTenantID = c.ID
			if dueDay != nil {
				c.PaymentDueDay = strconv.Itoa(*dueDay)
			}
			c.Stores = []RecurringClientStore{}
			out = append(out, c)
			ids = append(ids, c.ID)
		}
		err = rows.Err()
	}()
	if err != nil {
		return nil, err
	}
	if err := r.attachStores(ctx, ids, out); err != nil {
		return nil, err
	}
	return out, nil
}

// attachStores preenche as lojas com billing_amount > 0 por conta.
func (r *PostgresConfigStore) attachStores(ctx context.Context, ids []string, clients []RecurringClient) error {
	if len(ids) == 0 {
		return nil
	}
	byID := make(map[string]*RecurringClient, len(clients))
	for i := range clients {
		byID[clients[i].ID] = &clients[i]
	}
	rows, err := r.pool.Query(ctx, `
		select tenant_id::text, id::text, name, coalesce(billing_amount, 0)
		from queue.stores
		where tenant_id = any($1::uuid[]) and coalesce(billing_amount, 0) > 0
		order by tenant_id, lower(name)`, ids)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var tenantID string
		var st RecurringClientStore
		if err := rows.Scan(&tenantID, &st.ID, &st.Name, &st.Amount); err != nil {
			return err
		}
		if c := byID[tenantID]; c != nil {
			c.Stores = append(c.Stores, st)
		}
	}
	return rows.Err()
}
