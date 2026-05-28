package roadmap

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository interface {
	AccountExists(ctx context.Context, accountID string) (bool, error)
	IsAccountMember(ctx context.Context, accountID, userID string) (bool, error)
	ListPermissionsForUser(ctx context.Context, accountID, userID string) ([]string, error)

	ListModules(ctx context.Context, accountID string) ([]ModuleRecord, error)
	GetModule(ctx context.Context, id string) (*ModuleRecord, error)
	UpsertModuleForAccount(ctx context.Context, accountID string, input UpsertModuleInput) (*ModuleRecord, error)
	UpdateModule(ctx context.Context, id string, input UpsertModuleInput) (*ModuleRecord, error)
	DeleteModule(ctx context.Context, id string) error

	ListRules(ctx context.Context, accountID string) ([]Rule, error)
	GetRule(ctx context.Context, id string) (*Rule, error)
	UpsertRuleForAccount(ctx context.Context, accountID string, input UpsertRuleInput) (*Rule, error)
	UpdateRule(ctx context.Context, id string, input UpsertRuleInput) (*Rule, error)
	DeleteRule(ctx context.Context, id string) error

	ListDashboardTasks(ctx context.Context, accountID string) (map[string][]DashboardTask, error)
}

type PostgresRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresRepository(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{pool: pool}
}

func (repository *PostgresRepository) AccountExists(ctx context.Context, accountID string) (bool, error) {
	var exists bool
	err := repository.pool.QueryRow(ctx, `
		select exists (
			select 1 from core.accounts where id = $1::uuid and is_active = true
		)
	`, accountID).Scan(&exists)
	return exists, err
}

func (repository *PostgresRepository) IsAccountMember(ctx context.Context, accountID, userID string) (bool, error) {
	var exists bool
	err := repository.pool.QueryRow(ctx, `
		select exists (
			select 1
			from core.account_users
			where account_id = $1::uuid and user_id = $2::uuid and is_active = true
		)
	`, accountID, userID).Scan(&exists)
	return exists, err
}

func (repository *PostgresRepository) ListPermissionsForUser(ctx context.Context, accountID, userID string) ([]string, error) {
	rows, err := repository.pool.Query(ctx, `
		select rp.permission_key
		from core.user_role_assignments ura
		join core.role_permissions rp on rp.role_id = ura.role_id
		join core.permissions p on p.key = rp.permission_key and p.deprecated_at is null
		where ura.account_id = $1::uuid and ura.user_id = $2::uuid

		union

		select permission_key
		from core.user_permission_overrides
		where account_id = $1::uuid and user_id = $2::uuid
		  and effect = 'allow' and is_active = true

		except

		select permission_key
		from core.user_permission_overrides
		where account_id = $1::uuid and user_id = $2::uuid
		  and effect = 'deny' and is_active = true

		order by 1 asc
	`, accountID, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	permissions := make([]string, 0)
	for rows.Next() {
		var key string
		if err := rows.Scan(&key); err != nil {
			return nil, err
		}
		permissions = append(permissions, key)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return permissions, nil
}

const modulesSelectFields = `
	id::text as id, source_id, account_id::text as account_id, label, route, status, priority,
	coalesce(category, '') as category, description,
	coalesce(scope, '[]'::jsonb) as scope, coalesce(depends_on, '[]'::jsonb) as depends_on,
	sort_order, created_at, updated_at
`

func (repository *PostgresRepository) ListModules(ctx context.Context, accountID string) ([]ModuleRecord, error) {
	rows, err := repository.pool.Query(ctx, `
		with combined as (
			select `+modulesSelectFields+` from roadmap.modules
			where account_id is null
			   or account_id = $1::uuid
		)
		select distinct on (source_id) source_id, id, account_id, label, route, status, priority,
			category, description, scope, depends_on, sort_order, created_at, updated_at
		from combined
		order by source_id, (account_id is not null) desc
	`, accountID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	modules := make([]ModuleRecord, 0)
	for rows.Next() {
		m, err := scanModuleListRow(rows)
		if err != nil {
			return nil, err
		}
		modules = append(modules, m)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return modules, nil
}

func scanModuleListRow(rows pgx.Rows) (ModuleRecord, error) {
	var (
		m            ModuleRecord
		accountID    *string
		scopeBytes   []byte
		dependsBytes []byte
	)
	err := rows.Scan(&m.SourceID, &m.ID, &accountID, &m.Label, &m.Route, &m.Status, &m.Priority,
		&m.Category, &m.Description, &scopeBytes, &dependsBytes, &m.SortOrder, &m.CreatedAt, &m.UpdatedAt)
	if err != nil {
		return ModuleRecord{}, err
	}
	m.AccountID = accountID
	m.IsGlobal = accountID == nil
	if len(scopeBytes) > 0 {
		if err := json.Unmarshal(scopeBytes, &m.Scope); err != nil {
			return ModuleRecord{}, err
		}
	}
	if m.Scope == nil {
		m.Scope = []string{}
	}
	if len(dependsBytes) > 0 {
		if err := json.Unmarshal(dependsBytes, &m.DependsOn); err != nil {
			return ModuleRecord{}, err
		}
	}
	if m.DependsOn == nil {
		m.DependsOn = []string{}
	}
	return m, nil
}

func (repository *PostgresRepository) GetModule(ctx context.Context, id string) (*ModuleRecord, error) {
	row := repository.pool.QueryRow(ctx, `
		select `+modulesSelectFields+` from roadmap.modules where id = $1::uuid
	`, id)
	m, err := scanSingleModule(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &m, nil
}

func scanSingleModule(row pgx.Row) (ModuleRecord, error) {
	var (
		m            ModuleRecord
		accountID    *string
		scopeBytes   []byte
		dependsBytes []byte
	)
	err := row.Scan(&m.ID, &m.SourceID, &accountID, &m.Label, &m.Route, &m.Status, &m.Priority,
		&m.Category, &m.Description, &scopeBytes, &dependsBytes, &m.SortOrder, &m.CreatedAt, &m.UpdatedAt)
	if err != nil {
		return ModuleRecord{}, err
	}
	m.AccountID = accountID
	m.IsGlobal = accountID == nil
	if len(scopeBytes) > 0 {
		_ = json.Unmarshal(scopeBytes, &m.Scope)
	}
	if m.Scope == nil {
		m.Scope = []string{}
	}
	if len(dependsBytes) > 0 {
		_ = json.Unmarshal(dependsBytes, &m.DependsOn)
	}
	if m.DependsOn == nil {
		m.DependsOn = []string{}
	}
	return m, nil
}

func (repository *PostgresRepository) UpsertModuleForAccount(ctx context.Context, accountID string, input UpsertModuleInput) (*ModuleRecord, error) {
	scopeJSON, err := json.Marshal(input.Scope)
	if err != nil {
		return nil, err
	}
	dependsJSON, err := json.Marshal(input.DependsOn)
	if err != nil {
		return nil, err
	}
	row := repository.pool.QueryRow(ctx, `
		insert into roadmap.modules (source_id, account_id, label, route, status, priority, category, description, scope, depends_on, sort_order)
		values ($1, $2::uuid, $3, $4, $5, $6, $7, $8, $9::jsonb, $10::jsonb, $11)
		on conflict (source_id, account_id) do update set
			label = excluded.label,
			route = excluded.route,
			status = excluded.status,
			priority = excluded.priority,
			category = excluded.category,
			description = excluded.description,
			scope = excluded.scope,
			depends_on = excluded.depends_on,
			sort_order = excluded.sort_order,
			updated_at = now()
		returning `+modulesSelectFields,
		input.SourceID, accountID, input.Label, input.Route, input.Status, input.Priority,
		input.Category, input.Description, scopeJSON, dependsJSON, input.SortOrder,
	)
	m, err := scanSingleModule(row)
	if err != nil {
		return nil, err
	}
	return &m, nil
}

func (repository *PostgresRepository) UpdateModule(ctx context.Context, id string, input UpsertModuleInput) (*ModuleRecord, error) {
	scopeJSON, err := json.Marshal(input.Scope)
	if err != nil {
		return nil, err
	}
	dependsJSON, err := json.Marshal(input.DependsOn)
	if err != nil {
		return nil, err
	}
	row := repository.pool.QueryRow(ctx, `
		update roadmap.modules set
			label = $2, route = $3, status = $4, priority = $5,
			category = $6, description = $7, scope = $8::jsonb,
			depends_on = $9::jsonb, sort_order = $10, updated_at = now()
		where id = $1::uuid
		returning `+modulesSelectFields,
		id, input.Label, input.Route, input.Status, input.Priority,
		input.Category, input.Description, scopeJSON, dependsJSON, input.SortOrder,
	)
	m, err := scanSingleModule(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &m, nil
}

func (repository *PostgresRepository) DeleteModule(ctx context.Context, id string) error {
	tag, err := repository.pool.Exec(ctx, `delete from roadmap.modules where id = $1::uuid`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

const rulesSelectFields = `
	id::text as id, source_id, account_id::text as account_id, category, title, body, why, applies_when,
	sort_order, created_at, updated_at
`

func (repository *PostgresRepository) ListRules(ctx context.Context, accountID string) ([]Rule, error) {
	rows, err := repository.pool.Query(ctx, `
		with combined as (
			select `+rulesSelectFields+` from roadmap.rules
			where account_id is null
			   or account_id = $1::uuid
		)
		select distinct on (source_id) source_id, id, account_id, category, title, body, why, applies_when,
			sort_order, created_at, updated_at
		from combined
		order by source_id, (account_id is not null) desc
	`, accountID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	rules := make([]Rule, 0)
	for rows.Next() {
		r, err := scanRuleListRow(rows)
		if err != nil {
			return nil, err
		}
		rules = append(rules, r)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return rules, nil
}

func scanRuleListRow(rows pgx.Rows) (Rule, error) {
	var (
		r         Rule
		accountID *string
	)
	err := rows.Scan(&r.SourceID, &r.ID, &accountID, &r.Category, &r.Title, &r.Body, &r.Why, &r.AppliesWhen,
		&r.SortOrder, &r.CreatedAt, &r.UpdatedAt)
	if err != nil {
		return Rule{}, err
	}
	r.AccountID = accountID
	r.IsGlobal = accountID == nil
	return r, nil
}

func (repository *PostgresRepository) GetRule(ctx context.Context, id string) (*Rule, error) {
	row := repository.pool.QueryRow(ctx, `
		select `+rulesSelectFields+` from roadmap.rules where id = $1::uuid
	`, id)
	r, err := scanSingleRule(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &r, nil
}

func scanSingleRule(row pgx.Row) (Rule, error) {
	var (
		r         Rule
		accountID *string
	)
	err := row.Scan(&r.ID, &r.SourceID, &accountID, &r.Category, &r.Title, &r.Body, &r.Why, &r.AppliesWhen,
		&r.SortOrder, &r.CreatedAt, &r.UpdatedAt)
	if err != nil {
		return Rule{}, err
	}
	r.AccountID = accountID
	r.IsGlobal = accountID == nil
	return r, nil
}

func (repository *PostgresRepository) UpsertRuleForAccount(ctx context.Context, accountID string, input UpsertRuleInput) (*Rule, error) {
	row := repository.pool.QueryRow(ctx, `
		insert into roadmap.rules (source_id, account_id, category, title, body, why, applies_when, sort_order)
		values ($1, $2::uuid, $3, $4, $5, $6, $7, $8)
		on conflict (source_id, account_id) do update set
			category = excluded.category,
			title = excluded.title,
			body = excluded.body,
			why = excluded.why,
			applies_when = excluded.applies_when,
			sort_order = excluded.sort_order,
			updated_at = now()
		returning `+rulesSelectFields,
		input.SourceID, accountID, input.Category, input.Title, input.Body, input.Why, input.AppliesWhen, input.SortOrder,
	)
	r, err := scanSingleRule(row)
	if err != nil {
		return nil, err
	}
	return &r, nil
}

func (repository *PostgresRepository) UpdateRule(ctx context.Context, id string, input UpsertRuleInput) (*Rule, error) {
	row := repository.pool.QueryRow(ctx, `
		update roadmap.rules set
			category = $2, title = $3, body = $4, why = $5, applies_when = $6,
			sort_order = $7, updated_at = now()
		where id = $1::uuid
		returning `+rulesSelectFields,
		id, input.Category, input.Title, input.Body, input.Why, input.AppliesWhen, input.SortOrder,
	)
	r, err := scanSingleRule(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &r, nil
}

func (repository *PostgresRepository) DeleteRule(ctx context.Context, id string) error {
	tag, err := repository.pool.Exec(ctx, `delete from roadmap.rules where id = $1::uuid`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (repository *PostgresRepository) ListDashboardTasks(ctx context.Context, accountID string) (map[string][]DashboardTask, error) {
	rows, err := repository.pool.Query(ctx, `
		select t.id::text, t.title, t.status, t.priority, t.archived,
		       t.board_id::text, t.column_id::text, t.responsible_user_id::text,
		       coalesce(t.roadmap_module_id::text, '')
		from tasks.tasks t
		where t.account_id = $1::uuid
		  and t.roadmap_module_id is not null
		  and t.archived = false
		order by t.updated_at desc
	`, accountID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[string][]DashboardTask)
	for rows.Next() {
		var (
			task     DashboardTask
			moduleID string
		)
		if err := rows.Scan(&task.ID, &task.Title, &task.Status, &task.Priority, &task.Archived,
			&task.BoardID, &task.ColumnID, &task.ResponsibleUserID, &moduleID); err != nil {
			return nil, err
		}
		key := strings.TrimSpace(moduleID)
		result[key] = append(result[key], task)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return result, nil
}

func normalizeStringSlice(in []string) []string {
	out := make([]string, 0, len(in))
	for _, s := range in {
		v := strings.TrimSpace(s)
		if v != "" {
			out = append(out, v)
		}
	}
	return out
}
