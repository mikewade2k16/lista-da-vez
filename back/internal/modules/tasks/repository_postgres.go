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

func (repository *PostgresRepository) ValidateBoardClientScope(ctx context.Context, accountID, boardID string, clientIDs []string) error {
	var validCount int
	err := repository.pool.QueryRow(ctx, `
		select count(*)
		from unnest($3::text[]) requested(client_id)
		where exists (
			select 1
			from tasks.boards b
			join core.accounts client
			  on client.id::text = requested.client_id
			 and client.is_agency = false
			where b.account_id = $1::uuid
			  and b.id = $2::uuid
			  and (
				client.id = b.account_id
				or (b.organization_id is not null and client.organization_id = b.organization_id)
			  )
		)
	`, accountID, boardID, clientIDs).Scan(&validCount)
	if err != nil {
		return err
	}
	if validCount != len(clientIDs) {
		return ErrValidation
	}
	return nil
}

func (repository *PostgresRepository) ValidateBoardTaskSources(ctx context.Context, accountID, boardID string, sourceBoardIDs []string) error {
	var validCount int
	err := repository.pool.QueryRow(ctx, `
		select count(distinct source.id)
		from unnest($3::text[]) requested(board_id)
		join tasks.boards source
		  on source.id::text = requested.board_id
		 and source.account_id = $1::uuid
		 and source.id <> $2::uuid
		 and source.archived = false
	`, accountID, boardID, sourceBoardIDs).Scan(&validCount)
	if err != nil {
		return err
	}
	if validCount != len(sourceBoardIDs) {
		return ErrBoardNotFound
	}
	return nil
}

func (repository *PostgresRepository) GetUserPreferences(ctx context.Context, accountID, userID string) (UserPreferences, error) {
	var preferences UserPreferences
	err := repository.pool.QueryRow(ctx, `
		select preference.last_board_id::text
		from tasks.user_preferences preference
		join tasks.boards board
		  on board.id = preference.last_board_id
		 and board.account_id = preference.account_id
		 and board.archived = false
		where preference.account_id = $1::uuid
		  and preference.user_id = $2::uuid
	`, accountID, userID).Scan(&preferences.LastBoardID)
	if errors.Is(err, pgx.ErrNoRows) {
		return UserPreferences{}, nil
	}
	return preferences, err
}

func (repository *PostgresRepository) SaveUserPreferences(ctx context.Context, accountID, userID, lastBoardID string) (UserPreferences, error) {
	var preferences UserPreferences
	err := repository.pool.QueryRow(ctx, `
		insert into tasks.user_preferences (account_id, user_id, last_board_id, updated_at)
		select board.account_id, $2::uuid, board.id, now()
		from tasks.boards board
		where board.account_id = $1::uuid
		  and board.id = $3::uuid
		  and board.archived = false
		on conflict (account_id, user_id) do update set
			last_board_id = excluded.last_board_id,
			updated_at = excluded.updated_at
		returning last_board_id::text
	`, accountID, userID, lastBoardID).Scan(&preferences.LastBoardID)
	if errors.Is(err, pgx.ErrNoRows) {
		return UserPreferences{}, ErrBoardNotFound
	}
	return preferences, err
}
