package core

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresRepository(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{pool: pool}
}

// ============================================================================
// Users
// ============================================================================

func (r *PostgresRepository) FindUserByID(ctx context.Context, userID string) (User, error) {
	const query = `
		select id, email, display_name, avatar_path, must_change_password,
		       is_platform_admin, is_active, created_at, updated_at
		from core.users
		where id = $1::uuid
	`

	row := r.pool.QueryRow(ctx, query, userID)
	user, err := scanUser(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return User{}, ErrUserNotFound
		}
		return User{}, err
	}
	return user, nil
}

// ============================================================================
// Accounts
// ============================================================================

// accountVisibilityWhere e a clausula autoritativa de escopo de accounts. Uma
// account ($-param do user em $1) e visivel quando QUALQUER caminho vale:
//
//	(a) o user e platform_admin (core.users.is_platform_admin) -> ve todas;
//	(b) existe membership ativa em core.account_users.
//
// Membership de organizacao/agencia, isoladamente, nao concede contexto de uma
// account cliente. Operadores da agencia precisam de account_users explicito na
// conta que vao operar; grants do Omnichannel continuam sendo um gate adicional.
//
// A regra vive 100% no banco (defesa em profundidade): nada do client decide
// escopo. SQL totalmente parametrizado ($1 = userID).
const accountVisibilityWhere = `
		a.is_active = true
		and (
		  exists (
		    select 1 from core.users u
		    where u.id = $1::uuid
		      and u.is_active = true
		      and u.is_platform_admin = true
		  )
		  or exists (
		    select 1 from core.account_users au
		    where au.user_id = $1::uuid
		      and au.account_id = a.id
		      and au.is_active = true
		  )
		)`

// listAccountsForUserQuery lista todas as accounts ativas visiveis ao user
// pela regra org-aware acima. Os tres caminhos sao mutuamente OR dentro de um
// unico predicado, entao cada account aparece no maximo uma vez (sem JOIN que
// multiplique linhas) — DISTINCT desnecessario.
//
// Ordenacao MEMBERSHIP-FIRST: as accounts onde o user tem membership explicita
// (core.account_users) vem primeiro, depois o resto por nome. Sem isso, um
// platform_admin (que ve TODAS as accounts) teria como defaultAccountID
// a 1a por nome (ex.: "AM Malls") em vez da sua propria conta de trabalho — foi
// o que mandou o dev para a account errada apos a regra org-aware (2026-06-15).
// O front usa summaries[0] como default, entao a 1a precisa ser a "casa" do user.
const listAccountsForUserQuery = `
	select a.id, a.organization_id, a.slug, a.name, a.is_active, a.plan_code,
	       a.created_at, a.updated_at, a.is_agency, coalesce(o.name, '')
	from core.accounts a
	left join core.organizations o on o.id = a.organization_id
	where ` + accountVisibilityWhere + `
	order by
	  case when exists (
	      select 1 from core.account_users au_pref
	      where au_pref.user_id = $1::uuid
	        and au_pref.account_id = a.id
	        and au_pref.is_active = true
	  ) then 0 else 1 end,
	  lower(a.name) asc
`

// findAccountIfAccessibleQuery resolve uma unica account ($2) aplicando a MESMA
// regra de visibilidade. Se nenhum dos dois caminhos vale -> pgx.ErrNoRows ->
// o repository traduz para ErrAccountNotMember (nao vaza existencia da account).
const findAccountIfAccessibleQuery = `
	select a.id, a.organization_id, a.slug, a.name, a.is_active, a.plan_code,
	       a.created_at, a.updated_at, a.is_agency, coalesce(o.name, '')
	from core.accounts a
	left join core.organizations o on o.id = a.organization_id
	where a.id = $2::uuid
	  and ` + accountVisibilityWhere + `
`

func (r *PostgresRepository) ListAccountsForUser(ctx context.Context, userID string) ([]Account, error) {
	rows, err := r.pool.Query(ctx, listAccountsForUserQuery, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	accounts := make([]Account, 0)
	for rows.Next() {
		account, err := scanAccount(rows)
		if err != nil {
			return nil, err
		}
		accounts = append(accounts, account)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return accounts, nil
}

func (r *PostgresRepository) FindAccountIfMember(ctx context.Context, userID string, accountID string) (Account, error) {
	row := r.pool.QueryRow(ctx, findAccountIfAccessibleQuery, userID, accountID)
	return accountFromAccessibleRow(row)
}

// accountFromAccessibleRow le a row resolvida por findAccountIfAccessibleQuery e
// traduz a ausencia de resultado (pgx.ErrNoRows, ou seja: account inexistente OU
// fora de qualquer um dos tres caminhos de visibilidade) para ErrAccountNotMember.
// Nao distingue "nao existe" de "nao acessivel" — assim nao vaza existencia.
func accountFromAccessibleRow(row scannable) (Account, error) {
	account, err := scanAccount(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Account{}, ErrAccountNotMember
		}
		return Account{}, err
	}
	return account, nil
}

// ============================================================================
// Account modules
// ============================================================================

func (r *PostgresRepository) ListEnabledModuleIDs(ctx context.Context, accountID string) ([]string, error) {
	const query = `
		select module_id
		from core.account_modules
		where account_id = $1::uuid
		  and enabled = true
		order by module_id asc
	`

	rows, err := r.pool.Query(ctx, query, accountID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	moduleIDs := make([]string, 0)
	for rows.Next() {
		var moduleID string
		if err := rows.Scan(&moduleID); err != nil {
			return nil, err
		}
		moduleIDs = append(moduleIDs, moduleID)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return moduleIDs, nil
}

// ListEnabledModuleIDsForAccounts resolve em UMA query os modulos habilitados
// para uma lista de accounts. Retorna map[accountID -> []moduleID] contendo
// apenas as accounts que tiverem ao menos um modulo habilitado; accounts sem
// modulos aparecem com slice vazio depois do merge no service.
//
// Usado por MeAccounts para eliminar o N+1 de chamar ListEnabledModuleIDs
// por account num loop (OPT-2/F-22).
func (r *PostgresRepository) ListEnabledModuleIDsForAccounts(ctx context.Context, accountIDs []string) (map[string][]string, error) {
	out := make(map[string][]string, len(accountIDs))
	if len(accountIDs) == 0 {
		return out, nil
	}

	const query = `
		select account_id::text, module_id
		from core.account_modules
		where account_id = any($1::uuid[])
		  and enabled = true
		order by account_id asc, module_id asc
	`

	rows, err := r.pool.Query(ctx, query, accountIDs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var accountID, moduleID string
		if err := rows.Scan(&accountID, &moduleID); err != nil {
			return nil, err
		}
		out[accountID] = append(out[accountID], moduleID)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

// ============================================================================
// Organizations
// ============================================================================

func (r *PostgresRepository) FindOrganization(ctx context.Context, organizationID string) (Organization, error) {
	if organizationID == "" {
		return Organization{}, ErrOrganizationNotFound
	}

	const query = `
		select id, slug, name, is_active, created_at, updated_at
		from core.organizations
		where id = $1::uuid
	`

	row := r.pool.QueryRow(ctx, query, organizationID)
	var org Organization
	if err := row.Scan(&org.ID, &org.Slug, &org.Name, &org.Active, &org.CreatedAt, &org.UpdatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Organization{}, ErrOrganizationNotFound
		}
		return Organization{}, err
	}
	return org, nil
}

// ============================================================================
// Scanners
// ============================================================================

type scannable interface {
	Scan(dest ...any) error
}

func scanUser(row scannable) (User, error) {
	var user User
	if err := row.Scan(
		&user.ID,
		&user.Email,
		&user.DisplayName,
		&user.AvatarPath,
		&user.MustChangePassword,
		&user.IsPlatformAdmin,
		&user.Active,
		&user.CreatedAt,
		&user.UpdatedAt,
	); err != nil {
		return User{}, err
	}
	return user, nil
}

func scanAccount(row scannable) (Account, error) {
	var account Account
	var orgID *string
	if err := row.Scan(
		&account.ID,
		&orgID,
		&account.Slug,
		&account.Name,
		&account.Active,
		&account.PlanCode,
		&account.CreatedAt,
		&account.UpdatedAt,
		&account.IsAgency,
		&account.OrganizationName,
	); err != nil {
		return Account{}, err
	}
	if orgID != nil {
		account.OrganizationID = *orgID
	}
	return account, nil
}
