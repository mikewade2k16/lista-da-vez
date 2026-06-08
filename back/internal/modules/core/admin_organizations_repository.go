package core

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// PostgresAdminOrganizationRepository implementa AdminOrganizationRepository
// contra core.organizations + core.accounts (para agregados).
type PostgresAdminOrganizationRepository struct {
	*PostgresAdminRepository
}

// NewPostgresAdminOrganizationRepository cria a implementacao Postgres dos
// endpoints admin de organizations, embedando referencia ao pool ja existente.
func NewPostgresAdminOrganizationRepository(base *PostgresAdminRepository) *PostgresAdminOrganizationRepository {
	return &PostgresAdminOrganizationRepository{PostgresAdminRepository: base}
}

// ============================================================================
// ListOrganizations
// ============================================================================

func (r *PostgresAdminOrganizationRepository) ListOrganizations(ctx context.Context, filter AdminOrganizationListFilter) ([]OrganizationAdminView, int, error) {
	args := []any{}
	conds := []string{"1=1"}
	n := 1

	if filter.Q != "" {
		pattern := "%" + strings.ToLower(strings.TrimSpace(filter.Q)) + "%"
		conds = append(conds, fmt.Sprintf("(lower(o.name) like $%d or lower(o.slug) like $%d)", n, n))
		args = append(args, pattern)
		n++
	}
	switch filter.Status {
	case "active":
		conds = append(conds, "o.is_active = true")
	case "inactive":
		conds = append(conds, "o.is_active = false")
	}

	where := strings.Join(conds, " and ")

	var total int
	if err := r.pool.QueryRow(ctx,
		fmt.Sprintf("select count(*) from core.organizations o where %s", where),
		args...,
	).Scan(&total); err != nil {
		return nil, 0, err
	}

	page := filter.Page
	if page < 1 {
		page = 1
	}
	perPage := filter.PerPage
	if perPage < 1 {
		perPage = 20
	}
	if perPage > 100 {
		perPage = 100
	}

	args = append(args, perPage, (page-1)*perPage)
	dataSQL := fmt.Sprintf(`
		select
			o.id, o.slug, o.name, o.is_active, o.created_at, o.updated_at,
			coalesce(agg.account_count, 0),
			coalesce(agg.account_names, '')
		from core.organizations o
		left join lateral (
			select
				count(distinct a.id) as account_count,
				string_agg(distinct a.name, ', ' order by a.name) as account_names
			from core.accounts a
			where a.organization_id = o.id and a.is_active = true
		) agg on true
		where %s
		order by lower(o.name) asc
		limit $%d offset $%d
	`, where, n, n+1)

	rows, err := r.pool.Query(ctx, dataSQL, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	orgs := make([]OrganizationAdminView, 0)
	for rows.Next() {
		o, err := scanAdminOrganization(rows)
		if err != nil {
			return nil, 0, err
		}
		orgs = append(orgs, o)
	}
	return orgs, total, rows.Err()
}

// ============================================================================
// FindAdminOrganization
// ============================================================================

func (r *PostgresAdminOrganizationRepository) FindAdminOrganization(ctx context.Context, orgID string) (OrganizationAdminView, error) {
	const query = `
		select
			o.id, o.slug, o.name, o.is_active, o.created_at, o.updated_at,
			coalesce(agg.account_count, 0),
			coalesce(agg.account_names, '')
		from core.organizations o
		left join lateral (
			select
				count(distinct a.id) as account_count,
				string_agg(distinct a.name, ', ' order by a.name) as account_names
			from core.accounts a
			where a.organization_id = o.id and a.is_active = true
		) agg on true
		where o.id = $1::uuid
	`
	row := r.pool.QueryRow(ctx, query, orgID)
	o, err := scanAdminOrganization(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return OrganizationAdminView{}, ErrOrganizationNotFound
		}
		return OrganizationAdminView{}, err
	}
	return o, nil
}

// ============================================================================
// CreateOrganization
// ============================================================================

func (r *PostgresAdminOrganizationRepository) CreateOrganization(ctx context.Context, input AdminCreateOrganizationInput) (OrganizationAdminView, error) {
	var orgID string
	err := r.pool.QueryRow(ctx, `
		insert into core.organizations (slug, name, is_active, created_at, updated_at)
		values (lower($1), $2, true, now(), now())
		returning id
	`, input.Slug, input.Name).Scan(&orgID)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return OrganizationAdminView{}, ErrOrganizationSlugConflict
		}
		return OrganizationAdminView{}, err
	}
	return r.FindAdminOrganization(ctx, orgID)
}

// ============================================================================
// UpdateOrganization
// ============================================================================

func (r *PostgresAdminOrganizationRepository) UpdateOrganization(ctx context.Context, orgID string, input AdminUpdateOrganizationInput) (OrganizationAdminView, error) {
	sets := []string{}
	args := []any{orgID}
	n := 2

	addSet := func(column string, value any) {
		sets = append(sets, fmt.Sprintf("%s = $%d", column, n))
		args = append(args, value)
		n++
	}

	if input.Slug != nil {
		addSet("slug", strings.ToLower(strings.TrimSpace(*input.Slug)))
	}
	if input.Name != nil {
		addSet("name", *input.Name)
	}
	if input.IsActive != nil {
		addSet("is_active", *input.IsActive)
	}

	if len(sets) == 0 {
		return r.FindAdminOrganization(ctx, orgID)
	}

	sets = append(sets, fmt.Sprintf("updated_at = $%d", n))
	args = append(args, time.Now())

	query := fmt.Sprintf(`
		update core.organizations set %s where id = $1::uuid
	`, strings.Join(sets, ", "))

	tag, err := r.pool.Exec(ctx, query, args...)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return OrganizationAdminView{}, ErrOrganizationSlugConflict
		}
		return OrganizationAdminView{}, err
	}
	if tag.RowsAffected() == 0 {
		return OrganizationAdminView{}, ErrOrganizationNotFound
	}
	return r.FindAdminOrganization(ctx, orgID)
}

// ============================================================================
// SoftDeleteOrganization
// ============================================================================

func (r *PostgresAdminOrganizationRepository) SoftDeleteOrganization(ctx context.Context, orgID string) error {
	tag, err := r.pool.Exec(ctx,
		`update core.organizations set is_active = false, updated_at = now() where id = $1::uuid`,
		orgID,
	)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrOrganizationNotFound
	}
	return nil
}

// ============================================================================
// Scanner
// ============================================================================

func scanAdminOrganization(row scannable) (OrganizationAdminView, error) {
	var o OrganizationAdminView
	if err := row.Scan(
		&o.ID, &o.Slug, &o.Name, &o.IsActive,
		&o.CreatedAt, &o.UpdatedAt,
		&o.AccountCount, &o.AccountNames,
	); err != nil {
		return OrganizationAdminView{}, err
	}
	return o, nil
}
