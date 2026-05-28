package tenants

import (
	"context"
	"errors"

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

func (repository *PostgresRepository) ListAccessible(ctx context.Context, principal auth.Principal, input ListInput) ([]Tenant, error) {
	query, args := buildListAccessibleQuery(principal, input)
	rows, err := repository.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	tenants := make([]Tenant, 0)
	for rows.Next() {
		tenant, err := scanTenant(rows)
		if err != nil {
			return nil, err
		}

		tenants = append(tenants, tenant)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return tenants, nil
}

func (repository *PostgresRepository) FindAccessibleByID(ctx context.Context, principal auth.Principal, tenantID string) (Tenant, error) {
	query, args := buildFindAccessibleQuery(principal, tenantID)
	tenant, err := scanTenant(repository.pool.QueryRow(ctx, query, args...))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Tenant{}, ErrTenantNotFound
		}

		return Tenant{}, err
	}

	return tenant, nil
}

func (repository *PostgresRepository) Create(ctx context.Context, tenant Tenant) (Tenant, error) {
	created, err := scanTenant(repository.pool.QueryRow(ctx, `
		insert into tenants (
			slug,
			name,
			is_active
		)
		values (
			$1,
			$2,
			$3
		)
		returning
			id::text,
			slug,
			name,
			is_active,
			created_at,
			updated_at;
	`, tenant.Slug, tenant.Name, tenant.Active))
	if err != nil {
		if isUniqueViolation(err) {
			return Tenant{}, ErrTenantConflict
		}

		return Tenant{}, err
	}

	return created, nil
}

func (repository *PostgresRepository) Update(ctx context.Context, tenant Tenant) (Tenant, error) {
	updated, err := scanTenant(repository.pool.QueryRow(ctx, `
		update tenants
		set
			slug = $2,
			name = $3,
			is_active = $4,
			updated_at = now()
		where id = $1::uuid
		returning
			id::text,
			slug,
			name,
			is_active,
			created_at,
			updated_at;
	`, tenant.ID, tenant.Slug, tenant.Name, tenant.Active))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Tenant{}, ErrTenantNotFound
		}
		if isUniqueViolation(err) {
			return Tenant{}, ErrTenantConflict
		}

		return Tenant{}, err
	}

	return updated, nil
}

func buildListAccessibleQuery(principal auth.Principal, input ListInput) (string, []any) {
	activeClause := " and t.is_active = true"
	activeClauseForStore := " and t.is_active = true"
	if input.IncludeInactive {
		activeClause = ""
		activeClauseForStore = ""
	}

	switch principal.Role {
	case auth.RolePlatformAdmin:
		return `
			select
				t.id::text,
				t.slug,
				t.name,
				t.is_active,
				t.created_at,
				t.updated_at
			from tenants t
			where 1 = 1` + activeClause + `
			order by t.name asc;
		`, nil
	case auth.RoleOwner, auth.RoleDirector, auth.RoleMarketing:
		return `
			select distinct
				t.id::text,
				t.slug,
				t.name,
				t.is_active,
				t.created_at,
				t.updated_at
			from tenants t
			join user_tenant_roles utr on utr.tenant_id = t.id
			where utr.user_id = $1::uuid
			` + activeClause + `
			order by t.name asc;
		`, []any{principal.UserID}
	default:
		return `
			select distinct
				t.id::text,
				t.slug,
				t.name,
				t.is_active,
				t.created_at,
				t.updated_at
			from tenants t
			join stores s on s.tenant_id = t.id
			join user_store_roles usr on usr.store_id = s.id
			where usr.user_id = $1::uuid
				and s.is_active = true
				` + activeClauseForStore + `
			order by t.name asc;
		`, []any{principal.UserID}
	}
}

func buildFindAccessibleQuery(principal auth.Principal, tenantID string) (string, []any) {
	switch principal.Role {
	case auth.RolePlatformAdmin:
		return `
			select
				t.id::text,
				t.slug,
				t.name,
				t.is_active,
				t.created_at,
				t.updated_at
			from tenants t
			where t.id = $1::uuid;
		`, []any{tenantID}
	case auth.RoleOwner, auth.RoleDirector, auth.RoleMarketing:
		return `
			select distinct
				t.id::text,
				t.slug,
				t.name,
				t.is_active,
				t.created_at,
				t.updated_at
			from tenants t
			join user_tenant_roles utr on utr.tenant_id = t.id
			where t.id = $1::uuid
				and utr.user_id = $2::uuid;
		`, []any{tenantID, principal.UserID}
	default:
		return `
			select distinct
				t.id::text,
				t.slug,
				t.name,
				t.is_active,
				t.created_at,
				t.updated_at
			from tenants t
			join stores s on s.tenant_id = t.id
			join user_store_roles usr on usr.store_id = s.id
			where t.id = $1::uuid
				and usr.user_id = $2::uuid;
		`, []any{tenantID, principal.UserID}
	}
}

func scanTenant(row pgx.Row) (Tenant, error) {
	var tenant Tenant
	err := row.Scan(
		&tenant.ID,
		&tenant.Slug,
		&tenant.Name,
		&tenant.Active,
		&tenant.CreatedAt,
		&tenant.UpdatedAt,
	)
	if err != nil {
		return Tenant{}, err
	}

	return tenant, nil
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code == "23505"
	}

	return false
}
