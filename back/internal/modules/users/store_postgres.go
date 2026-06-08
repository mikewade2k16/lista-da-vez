package users

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

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

func (repository *PostgresRepository) ListAccessible(ctx context.Context, principal auth.Principal, input ListInput) ([]User, error) {
	query, args := buildScopedQuery(principal, input, "")
	rows, err := repository.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	users := make([]User, 0)
	for rows.Next() {
		user, err := scanUser(rows)
		if err != nil {
			return nil, err
		}

		users = append(users, user)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return users, nil
}

func (repository *PostgresRepository) FindAccessibleByID(ctx context.Context, principal auth.Principal, userID string) (User, error) {
	query, args := buildScopedQuery(principal, ListInput{}, userID)
	user, err := scanUser(repository.pool.QueryRow(ctx, query, args...))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return User{}, ErrNotFound
		}

		return User{}, err
	}

	return user, nil
}

func (repository *PostgresRepository) ResolveStoreScopes(ctx context.Context, storeIDs []string) ([]StoreScope, error) {
	if len(storeIDs) == 0 {
		return []StoreScope{}, nil
	}

	placeholders := make([]string, 0, len(storeIDs))
	args := make([]any, 0, len(storeIDs))
	for index, storeID := range storeIDs {
		placeholders = append(placeholders, fmt.Sprintf("$%d::uuid", index+1))
		args = append(args, storeID)
	}

	query := `
		select
			id::text,
			tenant_id::text,
			is_active
		from queue.stores
		where id in (` + strings.Join(placeholders, ", ") + `);
	`

	rows, err := repository.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	storeScopes := make([]StoreScope, 0, len(storeIDs))
	for rows.Next() {
		var storeScope StoreScope
		if err := rows.Scan(&storeScope.ID, &storeScope.TenantID, &storeScope.Active); err != nil {
			return nil, err
		}

		storeScopes = append(storeScopes, storeScope)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return storeScopes, nil
}

func (repository *PostgresRepository) Create(ctx context.Context, user User, passwordHash *string) (User, error) {
	tx, err := repository.pool.Begin(ctx)
	if err != nil {
		return User{}, err
	}

	defer func() {
		_ = tx.Rollback(ctx)
	}()

	var created User
	err = tx.QueryRow(ctx, `
		insert into core.users (
			email,
			display_name,
			employee_code,
			job_title,
			password_hash,
			must_change_password,
			is_active
		)
		values ($1, $2, $3, $4, $5, $6, $7)
		returning
			id::text,
			display_name,
			lower(email),
			is_active,
			created_at,
			updated_at;
	`, user.Email, user.DisplayName, user.EmployeeCode, user.JobTitle, passwordHash, user.MustChangePassword, user.Active).Scan(
		&created.ID,
		&created.DisplayName,
		&created.Email,
		&created.Active,
		&created.CreatedAt,
		&created.UpdatedAt,
	)
	if err != nil {
		if isUniqueViolation(err) {
			return User{}, ErrConflict
		}

		return User{}, err
	}

	created.Role = user.Role
	created.TenantID = user.TenantID
	created.StoreIDs = cloneStringSlice(user.StoreIDs)
	created.EmployeeCode = user.EmployeeCode
	created.JobTitle = user.JobTitle
	created.HasPassword = passwordHash != nil
	created.MustChangePassword = user.MustChangePassword

	if err := upsertAssignmentsTx(ctx, tx, created); err != nil {
		return User{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return User{}, err
	}

	return created, nil
}

func (repository *PostgresRepository) Update(ctx context.Context, user User, passwordHash *string) (User, error) {
	tx, err := repository.pool.Begin(ctx)
	if err != nil {
		return User{}, err
	}

	defer func() {
		_ = tx.Rollback(ctx)
	}()

	query := `
		update core.users
		set
			email = $2,
			display_name = $3,
			employee_code = $4,
			password_hash = coalesce($5, password_hash),
			must_change_password = $6,
			is_active = $7,
			updated_at = now()
		where id = $1::uuid
		returning
			id::text,
			display_name,
			lower(email),
			is_active,
			created_at,
			updated_at;
	`

	var passwordValue any
	if passwordHash != nil {
		passwordValue = *passwordHash
	}

	var updated User
	err = tx.QueryRow(ctx, query, user.ID, user.Email, user.DisplayName, user.EmployeeCode, passwordValue, user.MustChangePassword, user.Active).Scan(
		&updated.ID,
		&updated.DisplayName,
		&updated.Email,
		&updated.Active,
		&updated.CreatedAt,
		&updated.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return User{}, ErrNotFound
		}

		if isUniqueViolation(err) {
			return User{}, ErrConflict
		}

		return User{}, err
	}

	updated.Role = user.Role
	updated.TenantID = user.TenantID
	updated.StoreIDs = cloneStringSlice(user.StoreIDs)
	updated.EmployeeCode = user.EmployeeCode
	updated.JobTitle = user.JobTitle
	updated.HasPassword = user.HasPassword || passwordHash != nil
	updated.MustChangePassword = user.MustChangePassword

	if err := upsertAssignmentsTx(ctx, tx, updated); err != nil {
		return User{}, err
	}

	if passwordHash != nil || !updated.Active {
		if _, err := tx.Exec(ctx, `
			update user_invitations
			set
				status = 'revoked',
				revoked_at = now(),
				updated_at = now()
			where user_id = $1::uuid
				and status = 'pending';
		`, updated.ID); err != nil {
			return User{}, err
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return User{}, err
	}

	return updated, nil
}

func upsertAssignmentsTx(ctx context.Context, tx pgx.Tx, user User) error {
	// U4b: papel/escopo gravados SO em core via upsertCoreAssignmentsTx (cobre
	// platform_admin via is_platform_admin; tenant e store-scoped via
	// account_users + user_role_assignments + user_module_settings storeIds, e
	// limpa assignments antigos). Writes legados em user_*_roles removidos —
	// auth e /operacao/usuarios ja leem de core.
	return upsertCoreAssignmentsTx(ctx, tx, user)
}

func scanUser(row pgx.Row) (User, error) {
	var user User
	var roleCode string
	var roleTemplateID string
	var storeIDs []string
	var managedBy string
	var managedResourceID string
	var invitationStatus string
	var invitationExpiresAt *time.Time
	err := row.Scan(
		&user.ID,
		&user.DisplayName,
		&user.Nick,
		&user.Email,
		&user.EmployeeCode,
		&user.JobTitle,
		&roleCode,
		&roleTemplateID,
		&user.TenantID,
		&storeIDs,
		&user.Active,
		&user.HasPassword,
		&user.MustChangePassword,
		&managedBy,
		&managedResourceID,
		&invitationStatus,
		&invitationExpiresAt,
		&user.CreatedAt,
		&user.UpdatedAt,
	)
	if err != nil {
		return User{}, err
	}

	user.Email = strings.ToLower(strings.TrimSpace(user.Email))
	user.DisplayName = strings.TrimSpace(user.DisplayName)
	user.Nick = strings.TrimSpace(user.Nick)
	user.Role = auth.CoarseRoleFromCoreRole(roleCode, roleTemplateID)
	user.TenantID = strings.TrimSpace(user.TenantID)
	user.StoreIDs = cloneStringSlice(storeIDs)
	user.ManagedBy = strings.TrimSpace(managedBy)
	user.ManagedResourceID = strings.TrimSpace(managedResourceID)
	user.Invitation = InvitationSummary{
		Status:    auth.InvitationStatus(strings.TrimSpace(invitationStatus)),
		ExpiresAt: cloneTimePointer(invitationExpiresAt),
	}

	return user, nil
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code == "23505"
	}

	return false
}
