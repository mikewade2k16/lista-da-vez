package auth

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresUserStore struct {
	pool *pgxpool.Pool
}

type userRecord struct {
	ID                 string
	Email              string
	DisplayName        string
	Nick               string
	PasswordHash       string
	MustChangePassword bool
	AvatarPath         string
	Active             bool
	CreatedAt          time.Time
}

type invitationRecord struct {
	ID              string
	UserID          string
	Email           string
	InvitedByUserID string
	Status          string
	ExpiresAt       time.Time
	AcceptedAt      *time.Time
	RevokedAt       *time.Time
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

type passwordResetRecord struct {
	ID         string
	UserID     string
	Email      string
	Status     string
	ExpiresAt  time.Time
	ConsumedAt *time.Time
	RevokedAt  *time.Time
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

func NewPostgresUserStore(pool *pgxpool.Pool) *PostgresUserStore {
	return &PostgresUserStore{
		pool: pool,
	}
}

func (store *PostgresUserStore) FindByEmail(ctx context.Context, email string) (User, error) {
	record, err := store.findRecord(ctx, "lower(u.email) = lower($1)", strings.ToLower(strings.TrimSpace(email)))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return User{}, ErrInvalidCredentials
		}

		return User{}, err
	}

	return store.buildUser(ctx, record)
}

func (store *PostgresUserStore) FindByID(ctx context.Context, id string) (User, error) {
	record, err := store.findRecord(ctx, "u.id = $1::uuid", strings.TrimSpace(id))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return User{}, ErrUnauthorized
		}

		return User{}, err
	}

	return store.buildUser(ctx, record)
}

// LoadUserForAuth carrega o usuario por id no hot-path do middleware de auth.
// Mantido no contrato Repository; delega para FindByID, que ja resolve
// role/tenant/store via buildUser/resolveAuthRoleScope.
func (store *PostgresUserStore) LoadUserForAuth(ctx context.Context, userID string) (User, error) {
	return store.FindByID(ctx, userID)
}

func (store *PostgresUserStore) findRecord(ctx context.Context, predicate string, arg string) (userRecord, error) {
	query := `
		select
			u.id::text,
			lower(u.email) as email,
			u.display_name,
			coalesce(u.nick, '') as nick,
			u.password_hash,
			u.must_change_password,
			coalesce(u.avatar_path, '') as avatar_path,
			u.is_active,
			u.created_at
		from core.users u
		where ` + predicate + `
		limit 1;
	`

	var record userRecord
	var passwordHash pgtype.Text
	err := store.pool.QueryRow(ctx, query, arg).Scan(
		&record.ID,
		&record.Email,
		&record.DisplayName,
		&record.Nick,
		&passwordHash,
		&record.MustChangePassword,
		&record.AvatarPath,
		&record.Active,
		&record.CreatedAt,
	)
	if err != nil {
		return userRecord{}, err
	}

	if passwordHash.Valid {
		record.PasswordHash = strings.TrimSpace(passwordHash.String)
	}

	return record, nil
}

func (store *PostgresUserStore) buildUser(ctx context.Context, record userRecord) (User, error) {
	scope, err := store.resolveAuthRoleScope(ctx, record)
	if err != nil {
		return User{}, err
	}

	user := User{
		ID:                 record.ID,
		DisplayName:        record.DisplayName,
		Nick:               strings.TrimSpace(record.Nick),
		Email:              strings.ToLower(strings.TrimSpace(record.Email)),
		PasswordHash:       strings.TrimSpace(record.PasswordHash),
		MustChangePassword: record.MustChangePassword,
		AvatarPath:         strings.TrimSpace(record.AvatarPath),
		Role:               scope.Role,
		TenantID:           scope.TenantID,
		StoreIDs:           append([]string{}, scope.StoreIDs...),
		Active:             record.Active,
		CreatedAt:          record.CreatedAt,
	}

	// Authn != authz: escopo-coarse vazio NAO barra o login. Um usuario ATIVO
	// sem papel-coarse resolvido autentica com escopo vazio (igual ao
	// platform_admin com TenantID/AccountID vazios) e resolve o que pode ver
	// DEPOIS, por account, na RBAC custom. So validamos a corretude do escopo
	// quando um papel realmente existe — assim consultant/manager/store_terminal
	// continuam exigindo exatamente uma loja, sem que escopo vazio derrube ninguem.
	if HasEmptyScope(user) {
		return user, nil
	}

	if err := ValidateUserScope(user); err != nil {
		return User{}, err
	}

	return user, nil
}

func (store *PostgresUserStore) ReplacePendingInvitation(ctx context.Context, user User, invitedByUserID string, tokenHash string, expiresAt time.Time) (Invitation, error) {
	tx, err := store.pool.Begin(ctx)
	if err != nil {
		return Invitation{}, err
	}

	defer func() {
		_ = tx.Rollback(ctx)
	}()

	if _, err := tx.Exec(ctx, `
		update user_invitations
		set
			status = 'revoked',
			revoked_at = now(),
			updated_at = now()
		where user_id = $1::uuid
			and status = 'pending';
	`, user.ID); err != nil {
		return Invitation{}, err
	}

	record, err := scanInvitation(tx.QueryRow(ctx, `
		insert into user_invitations (
			user_id,
			email,
			invited_by_user_id,
			token_hash,
			status,
			expires_at
		)
		values ($1::uuid, $2, nullif($3, '')::uuid, $4, 'pending', $5)
		returning
			id::text,
			user_id::text,
			lower(email),
			invited_by_user_id::text,
			status,
			expires_at,
			accepted_at,
			revoked_at,
			created_at,
			updated_at;
	`, user.ID, strings.ToLower(strings.TrimSpace(user.Email)), strings.TrimSpace(invitedByUserID), tokenHash, expiresAt))
	if err != nil {
		return Invitation{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return Invitation{}, err
	}

	return toInvitation(record), nil
}

func (store *PostgresUserStore) UpdateProfile(ctx context.Context, userID string, displayName string, email string) (User, error) {
	if _, err := store.pool.Exec(ctx, `
		update core.users
		set
			display_name = $2,
			email = $3,
			updated_at = now()
		where id = $1::uuid;
	`, strings.TrimSpace(userID), strings.TrimSpace(displayName), strings.ToLower(strings.TrimSpace(email))); err != nil {
		if isUniqueViolation(err) {
			return User{}, ErrConflict
		}

		return User{}, err
	}

	return store.FindByID(ctx, userID)
}

func (store *PostgresUserStore) UpdatePassword(ctx context.Context, userID string, passwordHash string, mustChangePassword bool) (User, error) {
	if _, err := store.pool.Exec(ctx, `
		update core.users
		set
			password_hash = $2,
			must_change_password = $3,
			updated_at = now()
		where id = $1::uuid;
	`, strings.TrimSpace(userID), strings.TrimSpace(passwordHash), mustChangePassword); err != nil {
		return User{}, err
	}

	return store.FindByID(ctx, userID)
}

func (store *PostgresUserStore) UpdateAvatarPath(ctx context.Context, userID string, avatarPath string) (User, error) {
	if _, err := store.pool.Exec(ctx, `
		update core.users
		set
			avatar_path = $2,
			updated_at = now()
		where id = $1::uuid;
	`, strings.TrimSpace(userID), strings.TrimSpace(avatarPath)); err != nil {
		return User{}, err
	}

	return store.FindByID(ctx, userID)
}

func (store *PostgresUserStore) FindInvitationByTokenHash(ctx context.Context, tokenHash string) (Invitation, User, error) {
	record, err := scanInvitation(store.pool.QueryRow(ctx, `
		select
			id::text,
			user_id::text,
			lower(email),
			invited_by_user_id::text,
			status,
			expires_at,
			accepted_at,
			revoked_at,
			created_at,
			updated_at
		from user_invitations
		where token_hash = $1
		limit 1;
	`, tokenHash))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Invitation{}, User{}, ErrInvitationNotFound
		}

		return Invitation{}, User{}, err
	}

	user, err := store.FindByID(ctx, record.UserID)
	if err != nil {
		if errors.Is(err, ErrUnauthorized) {
			return Invitation{}, User{}, ErrInvitationNotFound
		}

		return Invitation{}, User{}, err
	}

	return toInvitation(record), user, nil
}

func (store *PostgresUserStore) AcceptInvitation(ctx context.Context, invitationID string, userID string, passwordHash string, acceptedAt time.Time) (User, error) {
	tx, err := store.pool.Begin(ctx)
	if err != nil {
		return User{}, err
	}

	defer func() {
		_ = tx.Rollback(ctx)
	}()

	result, err := tx.Exec(ctx, `
		update user_invitations
		set
			status = 'accepted',
			accepted_at = $3,
			updated_at = $3
		where id = $1::uuid
			and user_id = $2::uuid
			and status = 'pending';
	`, invitationID, userID, acceptedAt)
	if err != nil {
		return User{}, err
	}

	if result.RowsAffected() == 0 {
		return User{}, ErrInvitationNotFound
	}

	if _, err := tx.Exec(ctx, `
		update user_invitations
		set
			status = 'revoked',
			revoked_at = $2,
			updated_at = $2
		where user_id = $1::uuid
			and status = 'pending'
			and id <> $3::uuid;
	`, userID, acceptedAt, invitationID); err != nil {
		return User{}, err
	}

	if _, err := tx.Exec(ctx, `
		update core.users
		set
			password_hash = $2,
			must_change_password = false,
			is_active = true,
			updated_at = $3
		where id = $1::uuid;
	`, userID, passwordHash, acceptedAt); err != nil {
		return User{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return User{}, err
	}

	return store.FindByID(ctx, userID)
}

func (store *PostgresUserStore) ReplacePendingPasswordReset(ctx context.Context, user User, codeHash string, expiresAt time.Time) (PasswordReset, error) {
	tx, err := store.pool.Begin(ctx)
	if err != nil {
		return PasswordReset{}, err
	}

	defer func() {
		_ = tx.Rollback(ctx)
	}()

	if _, err := tx.Exec(ctx, `
		update user_password_resets
		set
			status = 'revoked',
			revoked_at = now(),
			updated_at = now()
		where user_id = $1::uuid
			and status = 'pending';
	`, user.ID); err != nil {
		return PasswordReset{}, err
	}

	record, err := scanPasswordReset(tx.QueryRow(ctx, `
		insert into user_password_resets (
			user_id,
			email,
			code_hash,
			status,
			expires_at
		)
		values ($1::uuid, $2, $3, 'pending', $4)
		returning
			id::text,
			user_id::text,
			lower(email),
			status,
			expires_at,
			consumed_at,
			revoked_at,
			created_at,
			updated_at;
	`, user.ID, strings.ToLower(strings.TrimSpace(user.Email)), codeHash, expiresAt))
	if err != nil {
		return PasswordReset{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return PasswordReset{}, err
	}

	return toPasswordReset(record), nil
}

func (store *PostgresUserStore) FindPasswordResetByEmailAndCodeHash(ctx context.Context, email string, codeHash string) (PasswordReset, User, error) {
	record, err := scanPasswordReset(store.pool.QueryRow(ctx, `
		select
			id::text,
			user_id::text,
			lower(email),
			status,
			expires_at,
			consumed_at,
			revoked_at,
			created_at,
			updated_at
		from user_password_resets
		where lower(email) = lower($1)
			and code_hash = $2
		order by created_at desc
		limit 1;
	`, strings.ToLower(strings.TrimSpace(email)), codeHash))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return PasswordReset{}, User{}, ErrPasswordResetNotFound
		}

		return PasswordReset{}, User{}, err
	}

	user, err := store.FindByID(ctx, record.UserID)
	if err != nil {
		if errors.Is(err, ErrUnauthorized) {
			return PasswordReset{}, User{}, ErrPasswordResetNotFound
		}

		return PasswordReset{}, User{}, err
	}

	return toPasswordReset(record), user, nil
}

func (store *PostgresUserStore) ConsumePasswordReset(ctx context.Context, passwordResetID string, userID string, passwordHash string, consumedAt time.Time) (User, error) {
	tx, err := store.pool.Begin(ctx)
	if err != nil {
		return User{}, err
	}

	defer func() {
		_ = tx.Rollback(ctx)
	}()

	result, err := tx.Exec(ctx, `
		update user_password_resets
		set
			status = 'consumed',
			consumed_at = $3,
			updated_at = $3
		where id = $1::uuid
			and user_id = $2::uuid
			and status = 'pending';
	`, passwordResetID, userID, consumedAt)
	if err != nil {
		return User{}, err
	}

	if result.RowsAffected() == 0 {
		return User{}, ErrPasswordResetNotFound
	}

	if _, err := tx.Exec(ctx, `
		update user_password_resets
		set
			status = 'revoked',
			revoked_at = $2,
			updated_at = $2
		where user_id = $1::uuid
			and status = 'pending'
			and id <> $3::uuid;
	`, userID, consumedAt, passwordResetID); err != nil {
		return User{}, err
	}

	if _, err := tx.Exec(ctx, `
		update core.users
		set
			password_hash = $2,
			must_change_password = false,
			updated_at = $3
		where id = $1::uuid;
	`, userID, passwordHash, consumedAt); err != nil {
		return User{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return User{}, err
	}

	return store.FindByID(ctx, userID)
}

func scanInvitation(row pgx.Row) (invitationRecord, error) {
	var record invitationRecord
	var invitedByUserID pgtype.Text
	var acceptedAt pgtype.Timestamptz
	var revokedAt pgtype.Timestamptz
	err := row.Scan(
		&record.ID,
		&record.UserID,
		&record.Email,
		&invitedByUserID,
		&record.Status,
		&record.ExpiresAt,
		&acceptedAt,
		&revokedAt,
		&record.CreatedAt,
		&record.UpdatedAt,
	)
	if err != nil {
		return invitationRecord{}, err
	}

	if invitedByUserID.Valid {
		record.InvitedByUserID = strings.TrimSpace(invitedByUserID.String)
	}

	if acceptedAt.Valid {
		value := acceptedAt.Time.UTC()
		record.AcceptedAt = &value
	}

	if revokedAt.Valid {
		value := revokedAt.Time.UTC()
		record.RevokedAt = &value
	}

	return record, nil
}

func scanPasswordReset(row pgx.Row) (passwordResetRecord, error) {
	var record passwordResetRecord
	var consumedAt pgtype.Timestamptz
	var revokedAt pgtype.Timestamptz
	err := row.Scan(
		&record.ID,
		&record.UserID,
		&record.Email,
		&record.Status,
		&record.ExpiresAt,
		&consumedAt,
		&revokedAt,
		&record.CreatedAt,
		&record.UpdatedAt,
	)
	if err != nil {
		return passwordResetRecord{}, err
	}

	if consumedAt.Valid {
		value := consumedAt.Time.UTC()
		record.ConsumedAt = &value
	}

	if revokedAt.Valid {
		value := revokedAt.Time.UTC()
		record.RevokedAt = &value
	}

	return record, nil
}

func toInvitation(record invitationRecord) Invitation {
	return Invitation{
		ID:              strings.TrimSpace(record.ID),
		UserID:          strings.TrimSpace(record.UserID),
		Email:           strings.ToLower(strings.TrimSpace(record.Email)),
		InvitedByUserID: strings.TrimSpace(record.InvitedByUserID),
		Status:          InvitationStatus(strings.TrimSpace(record.Status)),
		ExpiresAt:       record.ExpiresAt,
		AcceptedAt:      cloneTimePointer(record.AcceptedAt),
		RevokedAt:       cloneTimePointer(record.RevokedAt),
		CreatedAt:       record.CreatedAt,
		UpdatedAt:       record.UpdatedAt,
	}
}

func toPasswordReset(record passwordResetRecord) PasswordReset {
	return PasswordReset{
		ID:         strings.TrimSpace(record.ID),
		UserID:     strings.TrimSpace(record.UserID),
		Email:      strings.ToLower(strings.TrimSpace(record.Email)),
		Status:     PasswordResetStatus(strings.TrimSpace(record.Status)),
		ExpiresAt:  record.ExpiresAt,
		ConsumedAt: cloneTimePointer(record.ConsumedAt),
		RevokedAt:  cloneTimePointer(record.RevokedAt),
		CreatedAt:  record.CreatedAt,
		UpdatedAt:  record.UpdatedAt,
	}
}

func cloneTimePointer(value *time.Time) *time.Time {
	if value == nil {
		return nil
	}

	cloned := *value
	return &cloned
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code == "23505"
	}

	return false
}
