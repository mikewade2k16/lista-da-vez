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
		role := input.Role
		if role == "" {
			role = "owner"
		}
		if err := r.enrollUserInAccount(ctx, accountID, userID, role); err != nil {
			return AdminUserView{}, err
		}
	}
	if orgID := strings.TrimSpace(input.OrganizationID); orgID != "" {
		orgRole := strings.TrimSpace(input.OrgRole)
		if orgRole == "" {
			orgRole = "agency_member"
		}
		if _, err := r.pool.Exec(ctx, `
			insert into core.organization_users (organization_id, user_id, org_role, joined_at)
			values ($1::uuid, $2::uuid, $3, now())
			on conflict (organization_id, user_id) do update set org_role = $3
		`, orgID, userID, orgRole); err != nil {
			return AdminUserView{}, err
		}
		// Cargo de agencia precisa logar: vira membro da conta-agencia (is_agency=true)
		// da org com papel conforme o cargo (dono->owner total, membro->director
		// limitado). O switcher org-aware (AGENCY_TENANT_ARCHITECTURE) abre os clientes
		// da agencia. Sem isto o usuario de agencia nao resolve papel e o login falha.
		var agencyAccountID string
		err := r.pool.QueryRow(ctx, `
			select id::text
			from core.accounts
			where organization_id = $1::uuid and is_agency = true and is_active = true
			order by created_at asc
			limit 1
		`, orgID).Scan(&agencyAccountID)
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return AdminUserView{}, err
		}
		if strings.TrimSpace(agencyAccountID) != "" {
			if err := r.enrollUserInAccount(ctx, agencyAccountID, userID, agencyAccountRole(orgRole)); err != nil {
				return AdminUserView{}, err
			}
		}
	}

	return r.FindAdminUser(ctx, userID)
}

// enrollUserInAccount garante membership + papel CORE do usuario numa account:
// (1) core.account_users, (2) clona o role queue.<papel> do template e atribui em
// core.user_role_assignments, (3) copia as role_permissions do template. Reaproveitado
// pela conta-cliente e pela conta-agencia. Mapeamento owner/director->queue.supervisor,
// marketing->queue.consultant (igual a 0133 + auth core_role_resolver).
func (r *PostgresAdminUserRepository) enrollUserInAccount(ctx context.Context, accountID, userID, role string) error {
	if _, err := r.pool.Exec(ctx, `
		insert into core.account_users (account_id, user_id, is_active, joined_at)
		values ($1::uuid, $2::uuid, true, now())
		on conflict (account_id, user_id) do nothing
	`, accountID, userID); err != nil {
		return err
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
		return err
	}
	if _, err := r.pool.Exec(ctx, `
		insert into core.role_permissions (role_id, permission_key)
		select r.id, rtp.permission_key
		from core.roles r
		join core.role_template_permissions rtp on rtp.role_template_id = r.cloned_from_template_id
		where r.account_id = $1::uuid and r.code = 'queue.' || $2
		on conflict do nothing
	`, accountID, role); err != nil {
		return err
	}
	return nil
}

// agencyAccountRole mapeia o cargo de agencia para o papel na conta-agencia:
// agency_owner -> owner (acesso total da agencia); demais (agency_member) -> director
// (acesso limitado, tenant-scoped, sem exigir vinculo de loja).
func agencyAccountRole(orgRole string) string {
	if strings.TrimSpace(orgRole) == "agency_owner" {
		return "owner"
	}
	return "director"
}

// ============================================================================
// UpdateUser
// ============================================================================

func (r *PostgresAdminUserRepository) UpdateUser(ctx context.Context, userID string, input AdminUpdateUserInput, passwordHash string) (AdminUserView, error) {
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
	// Senha so e tocada quando o service mandou um hash (acao explicita). Definir
	// uma nova senha limpa o flag must_change_password (o admin acabou de defini-la).
	if passwordHash != "" {
		addSet("password_hash", passwordHash)
		addSet("must_change_password", false)
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
	// Role: papel coarse do user naquela conta (queue.<papel> -> <papel>). LATERAL
	// pega o assignment de maior prioridade. is_agency separa conta-cliente da
	// conta-agencia. Agencias primeiro, depois nome.
	const query = `
		select a.id, a.slug, a.name, au.is_active, au.joined_at, a.is_agency,
			coalesce(case
				when lower(sel.role_code) like 'queue.%' then substr(lower(sel.role_code), 7)
				else lower(sel.role_code)
			end, '') as role
		from core.account_users au
		join core.accounts a on a.id = au.account_id
		left join lateral (
			select lower(r.code) as role_code
			from core.user_role_assignments ura
			join core.roles r on r.id = ura.role_id
			where ura.account_id = a.id and ura.user_id = au.user_id
			order by case
				when lower(r.code) like 'queue.owner%' then 1
				when lower(r.code) like 'queue.director%' then 2
				when lower(r.code) like 'queue.marketing%' then 3
				when lower(r.code) like 'queue.manager%' then 4
				else 99
			end, r.created_at asc
			limit 1
		) sel on true
		where au.user_id = $1::uuid
		order by a.is_agency desc, lower(a.name) asc
	`
	rows, err := r.pool.Query(ctx, query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	memberships := make([]AccountMembershipView, 0)
	for rows.Next() {
		var m AccountMembershipView
		if err := rows.Scan(&m.AccountID, &m.AccountSlug, &m.AccountName, &m.IsActive, &m.JoinedAt, &m.IsAgency, &m.Role); err != nil {
			return nil, err
		}
		memberships = append(memberships, m)
	}
	return memberships, rows.Err()
}

// IsAccountMember diz se o usuario ja e membro ativo/inativo (account_users) da conta.
func (r *PostgresAdminUserRepository) IsAccountMember(ctx context.Context, accountID, userID string) (bool, error) {
	var exists bool
	err := r.pool.QueryRow(ctx, `
		select exists(
			select 1 from core.account_users
			where account_id = $1::uuid and user_id = $2::uuid
		)
	`, accountID, userID).Scan(&exists)
	return exists, err
}

// SetUserAccountRole remove os papeis atuais do usuario naquela conta e atribui o
// novo (reaproveitando enrollUserInAccount, que garante membership + role + perms).
func (r *PostgresAdminUserRepository) SetUserAccountRole(ctx context.Context, accountID, userID, role string) error {
	if _, err := r.pool.Exec(ctx, `
		delete from core.user_role_assignments
		where account_id = $1::uuid and user_id = $2::uuid
	`, accountID, userID); err != nil {
		return err
	}
	return r.enrollUserInAccount(ctx, accountID, userID, role)
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
