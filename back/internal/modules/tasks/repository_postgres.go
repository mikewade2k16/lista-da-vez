package tasks

import (
	"context"
	"errors"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresRepository(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{pool: pool}
}

func (repository *PostgresRepository) scopedQuery(accountID string, baseSQL string, args ...any) (string, []any) {
	if strings.TrimSpace(accountID) == "" {
		panic("tasks: scopedQuery called without accountID")
	}
	return baseSQL, append([]any{accountID}, args...)
}

func (repository *PostgresRepository) AccountExists(ctx context.Context, accountID string) (bool, error) {
	var exists bool
	err := repository.pool.QueryRow(ctx, `
		select exists (
			select 1 from core.accounts where id = $1::uuid and is_active = true
		)
	`, accountID).Scan(&exists)
	return exists, err
}

func (repository *PostgresRepository) IsAccountMember(ctx context.Context, accountID, userID string) (bool, error) {
	var exists bool
	err := repository.pool.QueryRow(ctx, `
		select exists (
			select 1
			from core.account_users
			where account_id = $1::uuid and user_id = $2::uuid and is_active = true
		)
	`, accountID, userID).Scan(&exists)
	return exists, err
}

func (repository *PostgresRepository) ListPermissionsForUser(ctx context.Context, accountID, userID string) ([]string, error) {
	rows, err := repository.pool.Query(ctx, `
		select rp.permission_key
		from core.user_role_assignments ura
		join core.role_permissions rp on rp.role_id = ura.role_id
		join core.permissions p on p.key = rp.permission_key and p.deprecated_at is null
		where ura.account_id = $1::uuid and ura.user_id = $2::uuid

		union

		select permission_key
		from core.user_permission_overrides
		where account_id = $1::uuid and user_id = $2::uuid
		  and effect = 'allow' and is_active = true

		except

		select permission_key
		from core.user_permission_overrides
		where account_id = $1::uuid and user_id = $2::uuid
		  and effect = 'deny' and is_active = true

		order by 1 asc
	`, accountID, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	permissions := make([]string, 0)
	for rows.Next() {
		var key string
		if err := rows.Scan(&key); err != nil {
			return nil, err
		}
		permissions = append(permissions, key)
	}
	return permissions, rows.Err()
}

func (repository *PostgresRepository) FindOrganizationIDForAccount(ctx context.Context, accountID string) (*string, error) {
	var organizationID *string
	err := repository.pool.QueryRow(ctx, `
		select organization_id::text from core.accounts where id = $1::uuid
	`, accountID).Scan(&organizationID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrAccountNotFound
	}
	return organizationID, err
}
