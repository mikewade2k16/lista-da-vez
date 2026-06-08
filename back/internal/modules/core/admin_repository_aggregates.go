package core

import (
	"context"
)

// Loaders agregados usados por ListAccounts e FindAdminAccount para popular
// userCount, userNicks, projectCount, projectSegments, modules e stores em batch.

type accountUserAggregate struct {
	UserCount int
	UserNicks string
}

type accountProjectAggregate struct {
	ProjectCount    int
	ProjectSegments string
}

// loadUserAggregates devolve por accountID a contagem de membros ativos e a
// lista de nicks (fallback display_name) agregada por vírgula. Uma query única.
func (r *PostgresAdminRepository) loadUserAggregates(ctx context.Context, accountIDs []string) (map[string]accountUserAggregate, error) {
	out := make(map[string]accountUserAggregate, len(accountIDs))
	if len(accountIDs) == 0 {
		return out, nil
	}

	const query = `
		select
			au.account_id::text,
			count(distinct au.user_id) as user_count,
			coalesce(string_agg(distinct coalesce(nullif(u.nick, ''), u.display_name), ', '), '') as user_nicks
		from core.account_users au
		join core.users u on u.id = au.user_id
		where au.account_id = any($1::uuid[]) and au.is_active = true and u.is_active = true
		group by au.account_id
	`
	rows, err := r.pool.Query(ctx, query, accountIDs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var id string
		var agg accountUserAggregate
		if err := rows.Scan(&id, &agg.UserCount, &agg.UserNicks); err != nil {
			return nil, err
		}
		out[id] = agg
	}
	return out, rows.Err()
}

// loadProjectAggregates devolve por accountID a contagem de boards de tasks
// ativos e a lista de nomes agregada (usada como segmentos no UI admin).
func (r *PostgresAdminRepository) loadProjectAggregates(ctx context.Context, accountIDs []string) (map[string]accountProjectAggregate, error) {
	out := make(map[string]accountProjectAggregate, len(accountIDs))
	if len(accountIDs) == 0 {
		return out, nil
	}

	const query = `
		select
			b.account_id::text,
			count(*) as project_count,
			coalesce(string_agg(b.name, ', ' order by b.name), '') as project_segments
		from tasks.boards b
		where b.account_id = any($1::uuid[]) and b.archived = false
		group by b.account_id
	`
	rows, err := r.pool.Query(ctx, query, accountIDs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var id string
		var agg accountProjectAggregate
		if err := rows.Scan(&id, &agg.ProjectCount, &agg.ProjectSegments); err != nil {
			return nil, err
		}
		out[id] = agg
	}
	return out, rows.Err()
}

// loadModulesByAccount devolve por accountID a lista completa de módulos
// (todos os modules registrados em core.modules) com status enabled/disabled
// para aquela account. Uma query única usando left join.
func (r *PostgresAdminRepository) loadModulesByAccount(ctx context.Context, accountIDs []string) (map[string][]AccountModuleView, error) {
	out := make(map[string][]AccountModuleView, len(accountIDs))
	if len(accountIDs) == 0 {
		return out, nil
	}

	// Para cada account, um row por módulo: m.id, m.label, m.is_core, enabled.
	// Cross-join entre accounts e modules garante que módulos ainda não
	// configurados em core.account_modules apareçam como enabled=false.
	const query = `
		select
			a.id::text as account_id,
			m.id, m.label, m.is_core,
			coalesce(am.enabled, false) as enabled,
			m.sort_order
		from core.accounts a
		cross join core.modules m
		left join core.account_modules am
		       on am.account_id = a.id and am.module_id = m.id
		where a.id = any($1::uuid[])
		order by a.id, m.sort_order asc, m.id asc
	`
	rows, err := r.pool.Query(ctx, query, accountIDs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var accountID string
		var mv AccountModuleView
		var sortOrder int
		if err := rows.Scan(&accountID, &mv.ModuleID, &mv.Label, &mv.IsCore, &mv.Enabled, &sortOrder); err != nil {
			return nil, err
		}
		out[accountID] = append(out[accountID], mv)
	}
	return out, rows.Err()
}

// loadStoresByAccount devolve por accountID a lista de stores em queue.stores
// (todas, independente do billing_mode — UI decide quando mostrar editor de
// billing por loja).
func (r *PostgresAdminRepository) loadStoresByAccount(ctx context.Context, accountIDs []string) (map[string][]StoreAdminView, error) {
	out := make(map[string][]StoreAdminView, len(accountIDs))
	if len(accountIDs) == 0 {
		return out, nil
	}

	const query = `
		select tenant_id::text, id, code, name, city, is_active, coalesce(billing_amount, 0)
		from queue.stores
		where tenant_id = any($1::uuid[])
		order by tenant_id, lower(name) asc
	`
	rows, err := r.pool.Query(ctx, query, accountIDs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var accountID string
		var sv StoreAdminView
		if err := rows.Scan(&accountID, &sv.ID, &sv.Code, &sv.Name, &sv.City, &sv.Active, &sv.BillingAmount); err != nil {
			return nil, err
		}
		out[accountID] = append(out[accountID], sv)
	}
	return out, rows.Err()
}

// enrichAccounts mescla todos os agregados nas views e garante slices
// não-nil em Modules/Stores (frontend espera array vazio, não null).
func (r *PostgresAdminRepository) enrichAccounts(ctx context.Context, views []AccountAdminView) error {
	if len(views) == 0 {
		return nil
	}

	ids := make([]string, len(views))
	for i, v := range views {
		ids[i] = v.ID
	}

	users, err := r.loadUserAggregates(ctx, ids)
	if err != nil {
		return err
	}
	projects, err := r.loadProjectAggregates(ctx, ids)
	if err != nil {
		return err
	}
	modules, err := r.loadModulesByAccount(ctx, ids)
	if err != nil {
		return err
	}
	stores, err := r.loadStoresByAccount(ctx, ids)
	if err != nil {
		return err
	}

	for i := range views {
		id := views[i].ID
		if u, ok := users[id]; ok {
			views[i].UserCount = u.UserCount
			views[i].UserNicks = u.UserNicks
		}
		if p, ok := projects[id]; ok {
			views[i].ProjectCount = p.ProjectCount
			views[i].ProjectSegments = p.ProjectSegments
		}
		if m, ok := modules[id]; ok {
			views[i].Modules = m
		} else {
			views[i].Modules = []AccountModuleView{}
		}
		if s, ok := stores[id]; ok {
			views[i].Stores = s //nolint:gosec
		} else {
			views[i].Stores = []StoreAdminView{} //nolint:gosec
		}
	}
	return nil
}
