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
		insert into core.accounts (
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
		update core.accounts
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
