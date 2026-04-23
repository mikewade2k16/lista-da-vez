package tenants

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
)

type PostgresRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresRepository(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{pool: pool}
}

func (repository *PostgresRepository) ListAccessible(ctx context.Context, principal auth.Principal) ([]Tenant, error) {
	query, args := buildListAccessibleQuery(principal)
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

func buildListAccessibleQuery(principal auth.Principal) (string, []any) {
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
			where t.is_active = true
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
				and t.is_active = true
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
				and t.is_active = true
			order by t.name asc;
		`, []any{principal.UserID}
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
