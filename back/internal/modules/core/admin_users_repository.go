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

// PostgresAdminUserRepository implementa AdminUserRepository contra core.users
// e core.account_users. Reaproveita o pool do PostgresAdminRepository para
// evitar duplicacao de conexao.
type PostgresAdminUserRepository struct {
	*PostgresAdminRepository
}

// NewPostgresAdminUserRepository cria a implementacao Postgres dos endpoints
// admin de users, embutindo a referencia ao pool ja existente.
func NewPostgresAdminUserRepository(base *PostgresAdminRepository) *PostgresAdminUserRepository {
	return &PostgresAdminUserRepository{PostgresAdminRepository: base}
}

// ============================================================================
// ListUsers
// ============================================================================

func (r *PostgresAdminUserRepository) ListUsers(ctx context.Context, filter AdminUserListFilter) ([]AdminUserView, int, error) {
	args := []any{}
	conds := []string{"1=1"}
	n := 1

	if filter.Q != "" {
		pattern := "%" + strings.ToLower(strings.TrimSpace(filter.Q)) + "%"
		conds = append(conds, fmt.Sprintf(
			"(lower(u.email) like $%d or lower(u.display_name) like $%d or lower(u.nick) like $%d)",
			n, n, n,
		))
		args = append(args, pattern)
		n++
	}
	switch filter.Status {
	case "active":
		conds = append(conds, "u.is_active = true")
	case "inactive":
		conds = append(conds, "u.is_active = false")
	}
	switch filter.PlatformAdmin {
	case "true":
		conds = append(conds, "u.is_platform_admin = true")
	case "false":
		conds = append(conds, "u.is_platform_admin = false")
	}

	where := strings.Join(conds, " and ")

	var total int
	if err := r.pool.QueryRow(ctx,
		fmt.Sprintf("select count(*) from core.users u where %s", where),
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

	// Projecao lean: quando IncludeAccounts e false, nao agregamos contas por user
	// (sem lateral join). A tela carrega o detalhe de contas sob interacao
	// (popover de memberships). Espelha AGENT_RULES "pedir so o necessario".
	accountSelect := "coalesce(agg.account_count, 0), coalesce(agg.account_names, '')"
	accountJoin := `
		left join lateral (
			select
				count(distinct au.account_id) as account_count,
				string_agg(distinct a.name, ', ' order by a.name) as account_names
			from core.account_users au
			join core.accounts a on a.id = au.account_id
			where au.user_id = u.id and au.is_active = true and a.is_active = true
		) agg on true`
	if !filter.IncludeAccounts {
		accountSelect = "0, ''"
		accountJoin = ""
	}

	dataSQL := fmt.Sprintf(`
		select
			u.id, u.email, u.display_name, u.nick, u.avatar_path,
			u.is_active, u.is_platform_admin, u.must_change_password,
			u.created_at, u.updated_at,
			%s
		from core.users u%s
		where %s
		order by lower(u.display_name) asc
		limit $%d offset $%d
	`, accountSelect, accountJoin, where, n, n+1)

	rows, err := r.pool.Query(ctx, dataSQL, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	users := make([]AdminUserView, 0)
	for rows.Next() {
		u, err := scanAdminUser(rows)
		if err != nil {
			return nil, 0, err
		}
		users = append(users, u)
	}
	return users, total, rows.Err()
}

// ============================================================================
// FindAdminUser
// ============================================================================

func (r *PostgresAdminUserRepository) FindAdminUser(ctx context.Context, userID string) (AdminUserView, error) {
	const query = `
		select
			u.id, u.email, u.display_name, u.nick, u.avatar_path,
			u.is_active, u.is_platform_admin, u.must_change_password,
			u.created_at, u.updated_at,
			coalesce(agg.account_count, 0),
			coalesce(agg.account_names, '')
		from core.users u
		left join lateral (
			select
				count(distinct au.account_id) as account_count,
				string_agg(distinct a.name, ', ' order by a.name) as account_names
			from core.account_users au
			join core.accounts a on a.id = au.account_id
			where au.user_id = u.id and au.is_active = true and a.is_active = true
		) agg on true
		where u.id = $1::uuid
	`
	row := r.pool.QueryRow(ctx, query, userID)
	u, err := scanAdminUser(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return AdminUserView{}, ErrUserNotFound
		}
		return AdminUserView{}, err
	}
	return u, nil
}

// ============================================================================
// CreateUser
// ============================================================================

func (r *PostgresAdminUserRepository) CreateUser(ctx context.Context, input AdminCreateUserInput, passwordHash string) (AdminUserView, error) {
	mustChangePassword := passwordHash == ""
	var userID string
	err := r.pool.QueryRow(ctx, `
		insert into core.users (email, display_name, nick, password_hash, must_change_password, is_platform_admin, is_active, created_at, updated_at)
		values (lower($1), $2, $3, nullif($4, ''), $5, $6, true, now(), now())
		returning id
	`, input.Email, input.DisplayName, input.Nick, passwordHash, mustChangePassword, input.IsPlatformAdmin).Scan(&userID)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return AdminUserView{}, ErrUserEmailConflict
		}
		return AdminUserView{}, err
	}

	// Vinculo opcional a cliente (account) e/ou agencia (organization).
	if accountID := strings.TrimSpace(input.AccountID); accountID != "" {
		// 1. membership no modelo novo (modulos contratados, /manage/users).
		if _, err := r.pool.Exec(ctx, `
			insert into core.account_users (account_id, user_id, is_active, joined_at)
			values ($1::uuid, $2::uuid, true, now())
			on conflict (account_id, user_id) do nothing
		`, accountID, userID); err != nil {
			return AdminUserView{}, err
		}
		// 2. papel no modelo CORE (core.user_role_assignments). Garante o role
		//    queue.<papel> da account (clona do template se faltar) e atribui.
		//    Mapeamento owner/director->queue.supervisor, marketing->queue.consultant
		//    (igual a 0133 + auth core_role_resolver). U4b: substitui o write legado
		//    em user_tenant_roles. accountId == tenantId (core.accounts.id==tenants.id).
		role := input.Role
		if role == "" {
			role = "owner"
		}
		if _, err := r.pool.Exec(ctx, `
			with ins as (
				insert into core.roles (account_id, cloned_from_template_id, code, label, description, is_locked)
				select $1::uuid, rt.id, 'queue.' || $3, rt.label, rt.description, rt.is_locked
				from core.role_templates rt
				where rt.id = case $3::text when 'marketing' then 'queue.consultant' else 'queue.supervisor' end
				on conflict (account_id, code) do nothing
				returning id
			),
			resolved as (
				select id from ins
				union
				select id from core.roles where account_id = $1::uuid and code = 'queue.' || $3
			)
			insert into core.user_role_assignments (account_id, user_id, role_id)
			select $1::uuid, $2::uuid, (select id from resolved limit 1)
			where (select id from resolved limit 1) is not null
			on conflict (account_id, user_id, role_id) do nothing
		`, accountID, userID, role); err != nil {
			return AdminUserView{}, err
		}
		if _, err := r.pool.Exec(ctx, `
			insert into core.role_permissions (role_id, permission_key)
			select r.id, rtp.permission_key
			from core.roles r
			join core.role_template_permissions rtp on rtp.role_template_id = r.cloned_from_template_id
			where r.account_id = $1::uuid and r.code = 'queue.' || $2
			on conflict do nothing
		`, accountID, role); err != nil {
			return AdminUserView{}, err
		}
	}
	if orgID := strings.TrimSpace(input.OrganizationID); orgID != "" {
		if _, err := r.pool.Exec(ctx, `
			insert into core.organization_users (organization_id, user_id, org_role, joined_at)
			values ($1::uuid, $2::uuid, 'agency_member', now())
			on conflict (organization_id, user_id) do nothing
		`, orgID, userID); err != nil {
			return AdminUserView{}, err
		}
	}

	return r.FindAdminUser(ctx, userID)
}

// ============================================================================
// UpdateUser
// ============================================================================

func (r *PostgresAdminUserRepository) UpdateUser(ctx context.Context, userID string, input AdminUpdateUserInput) (AdminUserView, error) {
	sets := []string{}
	args := []any{userID}
	n := 2

	addSet := func(column string, value any) {
		sets = append(sets, fmt.Sprintf("%s = $%d", column, n))
		args = append(args, value)
		n++
	}

	if input.Email != nil {
		addSet("email", strings.ToLower(strings.TrimSpace(*input.Email)))
	}
	if input.DisplayName != nil {
		addSet("display_name", *input.DisplayName)
	}
	if input.Nick != nil {
		addSet("nick", *input.Nick)
	}
	if input.IsActive != nil {
		addSet("is_active", *input.IsActive)
	}
	if input.IsPlatformAdmin != nil {
		addSet("is_platform_admin", *input.IsPlatformAdmin)
	}

	if len(sets) == 0 {
		return r.FindAdminUser(ctx, userID)
	}

	sets = append(sets, fmt.Sprintf("updated_at = $%d", n))
	args = append(args, time.Now())

	query := fmt.Sprintf(`
		update core.users set %s where id = $1::uuid
	`, strings.Join(sets, ", "))

	tag, err := r.pool.Exec(ctx, query, args...)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return AdminUserView{}, ErrUserEmailConflict
		}
		return AdminUserView{}, err
	}
	if tag.RowsAffected() == 0 {
		return AdminUserView{}, ErrUserNotFound
	}
	return r.FindAdminUser(ctx, userID)
}

// ============================================================================
// SoftDeleteUser
// ============================================================================

func (r *PostgresAdminUserRepository) SoftDeleteUser(ctx context.Context, userID string) error {
	tag, err := r.pool.Exec(ctx,
		`update core.users set is_active = false, updated_at = now() where id = $1::uuid`,
		userID,
	)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrUserNotFound
	}
	return nil
}

// ============================================================================
// GetMemberships
// ============================================================================

func (r *PostgresAdminUserRepository) GetMemberships(ctx context.Context, userID string) ([]AccountMembershipView, error) {
	const query = `
		select a.id, a.slug, a.name, au.is_active, au.joined_at
		from core.account_users au
		join core.accounts a on a.id = au.account_id
		where au.user_id = $1::uuid
		order by lower(a.name) asc
	`
	rows, err := r.pool.Query(ctx, query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	memberships := make([]AccountMembershipView, 0)
	for rows.Next() {
		var m AccountMembershipView
		if err := rows.Scan(&m.AccountID, &m.AccountSlug, &m.AccountName, &m.IsActive, &m.JoinedAt); err != nil {
			return nil, err
		}
		memberships = append(memberships, m)
	}
	return memberships, rows.Err()
}

// ============================================================================
// CountActivePlatformAdmins (safeguard)
// ============================================================================

func (r *PostgresAdminUserRepository) CountActivePlatformAdmins(ctx context.Context) (int, error) {
	var count int
	err := r.pool.QueryRow(ctx, `
		select count(*) from core.users where is_platform_admin = true and is_active = true
	`).Scan(&count)
	return count, err
}

// ============================================================================
// Scanner
// ============================================================================

func scanAdminUser(row scannable) (AdminUserView, error) {
	var u AdminUserView
	var nick, avatarPath *string
	if err := row.Scan(
		&u.ID, &u.Email, &u.DisplayName, &nick, &avatarPath,
		&u.IsActive, &u.IsPlatformAdmin, &u.MustChangePassword,
		&u.CreatedAt, &u.UpdatedAt,
		&u.AccountCount, &u.AccountNames,
	); err != nil {
		return AdminUserView{}, err
	}
	if nick != nil {
		u.Nick = *nick
	}
	if avatarPath != nil {
		u.AvatarPath = *avatarPath
	}
	return u, nil
}
