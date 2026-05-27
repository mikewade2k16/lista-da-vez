package stores

import (
	"context"
	"errors"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
)

type PostgresRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresRepository(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{pool: pool}
}

func (repository *PostgresRepository) ListAccessible(ctx context.Context, principal auth.Principal, input ListInput) ([]Store, error) {
	query, args := buildListAccessibleQuery(principal, input)
	rows, err := repository.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	stores := make([]Store, 0)
	for rows.Next() {
		store, err := scanStore(rows)
		if err != nil {
			return nil, err
		}

		stores = append(stores, store)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return stores, nil
}

func (repository *PostgresRepository) FindAccessibleByID(ctx context.Context, principal auth.Principal, storeID string) (Store, error) {
	query, args := buildFindAccessibleQuery(principal, storeID)
	store, err := scanStore(repository.pool.QueryRow(ctx, query, args...))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Store{}, ErrStoreNotFound
		}

		return Store{}, err
	}

	return store, nil
}

func (repository *PostgresRepository) Create(ctx context.Context, store Store) (Store, error) {
	query := `
		insert into stores (
			tenant_id,
			code,
			name,
			city,
			default_template_id,
			monthly_goal,
			weekly_goal,
			avg_ticket_goal,
			conversion_goal,
			pa_goal,
			is_active
		)
		values (
			$1::uuid,
			$2,
			$3,
			$4,
			$5,
			$6,
			$7,
			$8,
			$9,
			$10,
			$11
		)
		returning
			id::text,
			tenant_id::text,
			code,
			name,
			city,
			default_template_id,
			monthly_goal,
			weekly_goal,
			avg_ticket_goal,
			conversion_goal,
			pa_goal,
			is_active,
			created_at,
			updated_at;
	`

	created, err := scanStore(repository.pool.QueryRow(
		ctx,
		query,
		store.TenantID,
		store.Code,
		store.Name,
		store.City,
		store.DefaultTemplateID,
		store.MonthlyGoal,
		store.WeeklyGoal,
		store.AvgTicketGoal,
		store.ConversionGoal,
		store.PAGoal,
		store.Active,
	))
	if err != nil {
		if isUniqueViolation(err) {
			return Store{}, ErrStoreConflict
		}

		return Store{}, err
	}

	return created, nil
}

func (repository *PostgresRepository) Update(ctx context.Context, store Store) (Store, error) {
	query := `
		update stores
		set
			code = $2,
			name = $3,
			city = $4,
			default_template_id = $5,
			monthly_goal = $6,
			weekly_goal = $7,
			avg_ticket_goal = $8,
			conversion_goal = $9,
			pa_goal = $10,
			is_active = $11,
			updated_at = now()
		where id = $1::uuid
		returning
			id::text,
			tenant_id::text,
			code,
			name,
			city,
			default_template_id,
			monthly_goal,
			weekly_goal,
			avg_ticket_goal,
			conversion_goal,
			pa_goal,
			is_active,
			created_at,
			updated_at;
	`

	updated, err := scanStore(repository.pool.QueryRow(
		ctx,
		query,
		store.ID,
		store.Code,
		store.Name,
		store.City,
		store.DefaultTemplateID,
		store.MonthlyGoal,
		store.WeeklyGoal,
		store.AvgTicketGoal,
		store.ConversionGoal,
		store.PAGoal,
		store.Active,
	))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Store{}, ErrStoreNotFound
		}

		if isUniqueViolation(err) {
			return Store{}, ErrStoreConflict
		}

		return Store{}, err
	}

	return updated, nil
}

func (repository *PostgresRepository) Delete(ctx context.Context, storeID string) error {
	tx, err := repository.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	if _, err := tx.Exec(ctx, `
		update consultants
		set store_id = null
		where store_id = $1::uuid;
	`, storeID); err != nil {
		return err
	}

	if _, err := tx.Exec(ctx, `
		delete from user_store_roles
		where store_id = $1::uuid;
	`, storeID); err != nil {
		return err
	}

	commandTag, err := tx.Exec(ctx, `
		delete from stores
		where id = $1::uuid;
	`, storeID)
	if err != nil {
		return err
	}

	if commandTag.RowsAffected() == 0 {
		return ErrStoreNotFound
	}

	if err := tx.Commit(ctx); err != nil {
		return err
	}

	return nil
}

func buildListAccessibleQuery(principal auth.Principal, input ListInput) (string, []any) {
	tenantID := strings.TrimSpace(input.TenantID)
	activeClause := ""
	if !input.IncludeInactive {
		activeClause = " and s.is_active = true"
	}

	switch principal.Role {
	case auth.RolePlatformAdmin:
		if tenantID != "" {
			return `
				select
					s.id::text,
					s.tenant_id::text,
					s.code,
					s.name,
					s.city,
					s.default_template_id,
					s.monthly_goal,
					s.weekly_goal,
					s.avg_ticket_goal,
					s.conversion_goal,
					s.pa_goal,
					s.is_active,
					s.created_at,
					s.updated_at
				from stores s
				where s.tenant_id = $1::uuid
				` + activeClause + `
				order by s.created_at asc, s.code asc;
			`, []any{tenantID}
		}

		return `
			select
				s.id::text,
				s.tenant_id::text,
				s.code,
				s.name,
				s.city,
				s.default_template_id,
				s.monthly_goal,
				s.weekly_goal,
				s.avg_ticket_goal,
				s.conversion_goal,
				s.pa_goal,
				s.is_active,
				s.created_at,
				s.updated_at
			from stores s
			where 1 = 1
			` + activeClause + `
			order by s.created_at asc, s.code asc;
		`, nil
	case auth.RoleOwner, auth.RoleDirector, auth.RoleMarketing:
		query := `
			select distinct
				s.id::text,
				s.tenant_id::text,
				s.code,
				s.name,
				s.city,
				s.default_template_id,
				s.monthly_goal,
				s.weekly_goal,
				s.avg_ticket_goal,
				s.conversion_goal,
				s.pa_goal,
				s.is_active,
				s.created_at,
				s.updated_at
			from stores s
			join user_tenant_roles utr on utr.tenant_id = s.tenant_id
			where utr.user_id = $1::uuid
		`
		query += activeClause
		args := []any{principal.UserID}
		if tenantID != "" {
			query += `
				and s.tenant_id = $2::uuid
			`
			args = append(args, tenantID)
		}

		query += `
			order by s.created_at asc, s.code asc;
		`

		return query, args
	default:
		query := `
			select distinct
				s.id::text,
				s.tenant_id::text,
				s.code,
				s.name,
				s.city,
				s.default_template_id,
				s.monthly_goal,
				s.weekly_goal,
				s.avg_ticket_goal,
				s.conversion_goal,
				s.pa_goal,
				s.is_active,
				s.created_at,
				s.updated_at
			from stores s
			join user_store_roles usr on usr.store_id = s.id
			where usr.user_id = $1::uuid
		`
		query += activeClause
		args := []any{principal.UserID}
		if tenantID != "" {
			query += `
				and s.tenant_id = $2::uuid
			`
			args = append(args, tenantID)
		}

		query += `
			order by s.created_at asc, s.code asc;
		`

		return query, args
	}
}

func buildFindAccessibleQuery(principal auth.Principal, storeID string) (string, []any) {
	switch principal.Role {
	case auth.RolePlatformAdmin:
		return `
			select
				s.id::text,
				s.tenant_id::text,
				s.code,
				s.name,
				s.city,
				s.default_template_id,
				s.monthly_goal,
				s.weekly_goal,
				s.avg_ticket_goal,
				s.conversion_goal,
				s.pa_goal,
				s.is_active,
				s.created_at,
				s.updated_at
			from stores s
			where s.id = $1::uuid
			limit 1;
		`, []any{storeID}
	case auth.RoleOwner, auth.RoleDirector, auth.RoleMarketing:
		return `
			select distinct
				s.id::text,
				s.tenant_id::text,
				s.code,
				s.name,
				s.city,
				s.default_template_id,
				s.monthly_goal,
				s.weekly_goal,
				s.avg_ticket_goal,
				s.conversion_goal,
				s.pa_goal,
				s.is_active,
				s.created_at,
				s.updated_at
			from stores s
			join user_tenant_roles utr on utr.tenant_id = s.tenant_id
			where s.id = $1::uuid
				and utr.user_id = $2::uuid
			limit 1;
		`, []any{storeID, principal.UserID}
	default:
		return `
			select distinct
				s.id::text,
				s.tenant_id::text,
				s.code,
				s.name,
				s.city,
				s.default_template_id,
				s.monthly_goal,
				s.weekly_goal,
				s.avg_ticket_goal,
				s.conversion_goal,
				s.pa_goal,
				s.is_active,
				s.created_at,
				s.updated_at
			from stores s
			join user_store_roles usr on usr.store_id = s.id
			where s.id = $1::uuid
				and usr.user_id = $2::uuid
			limit 1;
		`, []any{storeID, principal.UserID}
	}
}

func scanStore(row pgx.Row) (Store, error) {
	var store Store
	err := row.Scan(
		&store.ID,
		&store.TenantID,
		&store.Code,
		&store.Name,
		&store.City,
		&store.DefaultTemplateID,
		&store.MonthlyGoal,
		&store.WeeklyGoal,
		&store.AvgTicketGoal,
		&store.ConversionGoal,
		&store.PAGoal,
		&store.Active,
		&store.CreatedAt,
		&store.UpdatedAt,
	)
	if err != nil {
		return Store{}, err
	}

	store.Code = strings.ToUpper(strings.TrimSpace(store.Code))
	store.Name = strings.TrimSpace(store.Name)
	store.City = strings.TrimSpace(store.City)
	store.DefaultTemplateID = strings.TrimSpace(store.DefaultTemplateID)

	return store, nil
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code == "23505"
	}

	return false
}
