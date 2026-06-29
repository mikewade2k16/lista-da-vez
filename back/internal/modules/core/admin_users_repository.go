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

// clientAccountIDSelect e a subquery escalar que resolve o id do UNICO cliente
// ativo NAO-agencia do usuario (u.id), ou ” quando ele tem 0 ou >1 clientes.
// O `having count(*) = 1` agrega o conjunto filtrado num so grupo: com exatamente
// 1 cliente o max() devolve o id e o having passa; com 0 (grupo vazio) ou >1 o
// having falha e a subquery devolve NULL -> coalesce para ”. Reusada por
// ListUsers e FindAdminUser (ambas com alias `u` para core.users).
const clientAccountIDSelect = `coalesce((
		select max(au3.account_id::text)
		from core.account_users au3
		join core.accounts a3 on a3.id = au3.account_id
		where au3.user_id = u.id and au3.is_active = true and a3.is_active = true
			and a3.is_agency = false
		having count(*) = 1
	), '')`

// isAgencyMemberSelect e a subquery escalar (bool) que diz se o usuario (u.id) e
// membro ATIVO de ao menos uma conta-agencia ATIVA (core.accounts.is_agency=true).
// Reusada por ListUsers e FindAdminUser (ambas com alias `u` para core.users),
// projetada como ultima coluna na mesma ordem nas duas queries.
const isAgencyMemberSelect = `exists(
		select 1
		from core.account_users au_ag
		join core.accounts a_ag on a_ag.id = au_ag.account_id
		where au_ag.user_id = u.id and au_ag.is_active = true
			and a_ag.is_active = true and a_ag.is_agency = true
	)`

// accountAggLateralJoin e o left join lateral que agrega as contas ATIVAS do
// usuario (u.id) em (account_count, account_names). Projeta o alias `agg`, lido
// como coalesce(agg.account_count,0)/coalesce(agg.account_names,”). Reusado por
// ListUsers (quando IncludeAccounts=true) e FindAdminUser (ambas com alias `u`
// para core.users).
const accountAggLateralJoin = `
		left join lateral (
			select
				count(distinct au.account_id) as account_count,
				string_agg(distinct a.name, ', ' order by a.name) as account_names
			from core.account_users au
			join core.accounts a on a.id = au.account_id
			where au.user_id = u.id and au.is_active = true and a.is_active = true
		) agg on true`

// ============================================================================
// ListUsers
// ============================================================================

func (r *PostgresAdminUserRepository) ListUsers(ctx context.Context, filter AdminUserListFilter) ([]AdminUserListItem, int, error) {
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
	// Filtro por conta: so usuarios que sao membros ATIVOS daquela conta ATIVA
	// (defesa em profundidade — exige conta ativa, nao so a membership). exists()
	// nao multiplica linhas.
	if accountID := strings.TrimSpace(filter.AccountID); accountID != "" {
		conds = append(conds, fmt.Sprintf(`exists (
			select 1 from core.account_users au2
			join core.accounts a2 on a2.id = au2.account_id
			where au2.user_id = u.id and au2.account_id = $%d::uuid
				and au2.is_active = true and a2.is_active = true
		)`, n))
		args = append(args, accountID)
		n++
	}

	// Escopo de delegacao (multi-tenant): so usuarios visiveis ao ator. O MESMO
	// predicado entra no count(*) e no SELECT (total e linhas batem — sem
	// enumeration). actorUserID vem do Principal (nunca da query). Defesa em
	// profundidade: a regra vive 100% no SQL, mesmo o handler tendo gateado antes.
	if actor := strings.TrimSpace(filter.ActorUserID); actor != "" {
		conds = append(conds, listUsersScopeWhere(n, n+1))
		args = append(args, actor, adminManagePermKeys)
		n += 2
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
	accountJoin := accountAggLateralJoin
	if !filter.IncludeAccounts {
		accountSelect = "0, ''"
		accountJoin = ""
	}

	// Projecao LEAN (OPT-4/F-26): a listagem NAO seleciona avatar_path/created_at/
	// updated_at — nenhum consumidor da tabela /manage/users (workspace + colunas +
	// popover de detalhes/acoes) renderiza esses campos. O detalhe (FindAdminUser)
	// continua selecionando o conjunto completo. As colunas abaixo seguem a ordem
	// lida por scanAdminUserListRow.
	dataSQL := fmt.Sprintf(`
		select
			u.id, u.email, u.display_name, u.nick,
			u.is_active, u.is_platform_admin, u.must_change_password,
			%s,
			%s,
			%s
		from core.users u%s
		where %s
		order by lower(u.display_name) asc
		limit $%d offset $%d
	`, accountSelect, clientAccountIDSelect, isAgencyMemberSelect, accountJoin, where, n, n+1)

	rows, err := r.pool.Query(ctx, dataSQL, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	users := make([]AdminUserListItem, 0)
	for rows.Next() {
		u, err := scanAdminUserListRow(rows)
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
	query := `
		select
			u.id, u.email, u.display_name, u.nick, u.avatar_path,
			u.is_active, u.is_platform_admin, u.must_change_password,
			u.created_at, u.updated_at,
			coalesce(agg.account_count, 0),
			coalesce(agg.account_names, ''),
			` + clientAccountIDSelect + `,
			` + isAgencyMemberSelect + `
		from core.users u` + accountAggLateralJoin + `
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
		if err := enrollUserInAccount(ctx, r.pool, accountID, userID, role); err != nil {
			return AdminUserView{}, err
		}
	}
	if orgID := strings.TrimSpace(input.OrganizationID); orgID != "" {
		orgRole := strings.TrimSpace(input.OrgRole)
		if orgRole == "" {
			orgRole = "agency_member"
		}
		// Bloco de vinculo de agencia extraido para linkUserToOrganization (DRY):
		// reaproveitado pelo endpoint POST /v1/admin/users/{id}/organizations/{orgId}.
		if err := linkUserToOrganization(ctx, r.pool, orgID, userID, orgRole); err != nil {
			return AdminUserView{}, err
		}
	}

	return r.FindAdminUser(ctx, userID)
}

// O bloco de matricula/membership (pgxExec, enrollUserInAccount, agencyAccountRole,
// SetUserAccountRole, MoveUserAccount) vive em admin_users_enroll_repository.go,
// no mesmo pacote core.

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

// SetUserAccountRole e MoveUserAccount vivem em admin_users_enroll_repository.go
// (mesmo pacote core), junto do bloco de matricula/membership.

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

// scanAdminUserListRow le a projecao LEAN da listagem (OPT-4/F-26): mesma ordem do
// SELECT de ListUsers, SEM avatar_path/created_at/updated_at. nick e nullable
// (ponteiro). Ordem das colunas importa — manter alinhada com o SELECT.
func scanAdminUserListRow(row scannable) (AdminUserListItem, error) {
	var u AdminUserListItem
	var nick *string
	if err := row.Scan(
		&u.ID, &u.Email, &u.DisplayName, &nick,
		&u.IsActive, &u.IsPlatformAdmin, &u.MustChangePassword,
		&u.AccountCount, &u.AccountNames, &u.ClientAccountID,
		&u.IsAgencyMember,
	); err != nil {
		return AdminUserListItem{}, err
	}
	if nick != nil {
		u.Nick = *nick
	}
	return u, nil
}

func scanAdminUser(row scannable) (AdminUserView, error) {
	var u AdminUserView
	var nick, avatarPath *string
	if err := row.Scan(
		&u.ID, &u.Email, &u.DisplayName, &nick, &avatarPath,
		&u.IsActive, &u.IsPlatformAdmin, &u.MustChangePassword,
		&u.CreatedAt, &u.UpdatedAt,
		&u.AccountCount, &u.AccountNames, &u.ClientAccountID,
		&u.IsAgencyMember,
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
