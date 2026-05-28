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

func (r *PostgresRepository) ListAccountsForUser(ctx context.Context, userID string) ([]Account, error) {
	const query = `
		select a.id, a.organization_id, a.slug, a.name, a.is_active, a.plan_code,
		       a.created_at, a.updated_at
		from core.accounts a
		join core.account_users au on au.account_id = a.id
		where au.user_id = $1::uuid
		  and au.is_active = true
		  and a.is_active = true
		order by lower(a.name) asc
	`

	rows, err := r.pool.Query(ctx, query, userID)
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
	const query = `
		select a.id, a.organization_id, a.slug, a.name, a.is_active, a.plan_code,
		       a.created_at, a.updated_at
		from core.accounts a
		join core.account_users au on au.account_id = a.id
		where au.user_id = $1::uuid
		  and au.account_id = $2::uuid
		  and au.is_active = true
		  and a.is_active = true
	`

	row := r.pool.QueryRow(ctx, query, userID, accountID)
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
	); err != nil {
		return Account{}, err
	}
	if orgID != nil {
		account.OrganizationID = *orgID
	}
	return account, nil
}
