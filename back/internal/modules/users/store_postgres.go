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
		from stores
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
		insert into users (
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
		update users
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
	if _, err := tx.Exec(ctx, `delete from user_platform_roles where user_id = $1::uuid;`, user.ID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `delete from user_tenant_roles where user_id = $1::uuid;`, user.ID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `delete from user_store_roles where user_id = $1::uuid;`, user.ID); err != nil {
		return err
	}

	switch user.Role {
	case auth.RolePlatformAdmin:
		_, err := tx.Exec(ctx, `
			insert into user_platform_roles (user_id, role)
			values ($1::uuid, 'platform_admin');
		`, user.ID)
		return err
	case auth.RoleOwner, auth.RoleDirector, auth.RoleMarketing:
		_, err := tx.Exec(ctx, `
			insert into user_tenant_roles (user_id, tenant_id, role)
			values ($1::uuid, $2::uuid, $3);
		`, user.ID, user.TenantID, string(user.Role))
		return err
	case auth.RoleManager, auth.RoleConsultant, auth.RoleStoreTerminal:
		for _, storeID := range user.StoreIDs {
			if _, err := tx.Exec(ctx, `
				insert into user_store_roles (user_id, store_id, role)
				values ($1::uuid, $2::uuid, $3);
			`, user.ID, storeID, string(user.Role)); err != nil {
				return err
			}
		}

		return nil
	default:
		return ErrValidation
	}
}

func buildScopedQuery(principal auth.Principal, input ListInput, userID string) (string, []any) {
	query := baseProjectedUsersQuery()
	clauses := []string{"projected.effective_role <> ''"}
	args := make([]any, 0)

	if principal.Role == auth.RoleOwner {
		args = append(args, principal.TenantID)
		clauses = append(clauses, fmt.Sprintf("projected.tenant_id = $%d", len(args)))
	}

	if strings.TrimSpace(input.TenantID) != "" {
		args = append(args, strings.TrimSpace(input.TenantID))
		clauses = append(clauses, fmt.Sprintf("projected.tenant_id = $%d", len(args)))
	}

	if strings.TrimSpace(input.StoreID) != "" {
		args = append(args, strings.TrimSpace(input.StoreID))
		clauses = append(clauses, fmt.Sprintf("$%d = any(projected.store_ids)", len(args)))
	}

	if input.Role != "" {
		args = append(args, string(input.Role))
		clauses = append(clauses, fmt.Sprintf("projected.effective_role = $%d", len(args)))
	}

	if input.Active != nil {
		args = append(args, *input.Active)
		clauses = append(clauses, fmt.Sprintf("projected.is_active = $%d", len(args)))
	}

	if strings.TrimSpace(userID) != "" {
		args = append(args, strings.TrimSpace(userID))
		clauses = append(clauses, fmt.Sprintf("projected.id = $%d", len(args)))
	}

	query += `
		where ` + strings.Join(clauses, " and ") + `
		order by projected.created_at desc, projected.email asc
	`

	if strings.TrimSpace(userID) != "" {
		query += `
			limit 1
		`
	}

	query += `;
	`

	return query, args
}

func baseProjectedUsersQuery() string {
	return `
		select
			projected.id,
			projected.display_name,
			projected.email,
			projected.employee_code,
			projected.job_title,
			projected.effective_role,
			projected.tenant_id,
			projected.store_ids,
			projected.is_active,
			projected.has_password,
			projected.must_change_password,
			projected.managed_by,
			projected.managed_resource_id,
			projected.invitation_status,
			projected.invitation_expires_at,
			projected.created_at,
			projected.updated_at
		from (
			select
				u.id::text as id,
				u.display_name,
				lower(u.email) as email,
				coalesce(u.employee_code, '') as employee_code,
				coalesce(u.job_title, '') as job_title,
				u.is_active,
				coalesce(nullif(trim(u.password_hash), ''), '') <> '' as has_password,
				u.must_change_password,
				u.created_at,
				u.updated_at,
				coalesce(platform_role.role, tenant_role.role, store_role.role, '') as effective_role,
				coalesce(tenant_role.tenant_id, store_role.tenant_id, '') as tenant_id,
				coalesce(store_role.store_ids, array[]::text[]) as store_ids,
				case
					when coalesce(consultant_link.consultant_id, '') <> '' and coalesce(consultant_link.is_active, false) then 'consultants'
					else ''
				end as managed_by,
				coalesce(consultant_link.consultant_id, '') as managed_resource_id,
				coalesce(invitation.status, '') as invitation_status,
				invitation.expires_at as invitation_expires_at
			from users u
			left join lateral (
				select upr.role
				from user_platform_roles upr
				where upr.user_id = u.id
				limit 1
			) as platform_role on true
			left join lateral (
				select
					utr.role,
					utr.tenant_id::text as tenant_id
				from user_tenant_roles utr
				where utr.user_id = u.id
				order by case
					when utr.role = 'owner' then 1
					when utr.role = 'director' then 2
					when utr.role = 'marketing' then 3
					else 99
				end,
				utr.created_at asc
				limit 1
			) as tenant_role on true
			left join lateral (
				select
					(array_agg(usr.role order by case
						when usr.role = 'manager' then 1
						when usr.role = 'consultant' then 2
						when usr.role = 'store_terminal' then 3
						else 99
					end))[1] as role,
					min(s.tenant_id::text) as tenant_id,
					array_agg(distinct usr.store_id::text order by usr.store_id::text) as store_ids
				from user_store_roles usr
				join stores s on s.id = usr.store_id
				where usr.user_id = u.id
			) as store_role on true
			left join lateral (
				select
					coalesce(c.id::text, '') as consultant_id,
					c.is_active
				from consultants c
				where c.user_id = u.id
				order by c.is_active desc, c.updated_at desc
				limit 1
			) as consultant_link on true
			left join lateral (
				select
					case
						when ui.status = 'pending' and ui.expires_at < now() then 'expired'
						else ui.status
					end as status,
					ui.expires_at
				from user_invitations ui
				where ui.user_id = u.id
				order by ui.created_at desc
				limit 1
			) as invitation on true
		) as projected
	`
}

func scanUser(row pgx.Row) (User, error) {
	var user User
	var role string
	var storeIDs []string
	var managedBy string
	var managedResourceID string
	var invitationStatus string
	var invitationExpiresAt *time.Time
	err := row.Scan(
		&user.ID,
		&user.DisplayName,
		&user.Email,
		&user.EmployeeCode,
		&user.JobTitle,
		&role,
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
	user.Role = auth.Role(strings.TrimSpace(role))
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
