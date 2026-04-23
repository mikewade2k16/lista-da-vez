package consultants

import (
	"context"
	"errors"
	"strings"

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

func (repository *PostgresRepository) StoreExists(ctx context.Context, storeID string) (bool, error) {
	var exists bool
	err := repository.pool.QueryRow(ctx, `
		select exists(
			select 1
			from stores
			where id = $1::uuid
		);
	`, storeID).Scan(&exists)
	if err != nil {
		return false, err
	}

	return exists, nil
}

func (repository *PostgresRepository) ResolveStoreAccessContext(ctx context.Context, storeID string) (StoreAccessContext, error) {
	var storeContext StoreAccessContext
	err := repository.pool.QueryRow(ctx, `
		select tenant_id::text, code
		from stores
		where id = $1::uuid
		limit 1;
	`, storeID).Scan(&storeContext.TenantID, &storeContext.StoreCode)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return StoreAccessContext{}, ErrStoreNotFound
		}

		return StoreAccessContext{}, err
	}

	storeContext.TenantID = strings.TrimSpace(storeContext.TenantID)
	storeContext.StoreCode = strings.TrimSpace(storeContext.StoreCode)
	return storeContext, nil
}

func (repository *PostgresRepository) ListByStore(ctx context.Context, storeID string) ([]Consultant, error) {
	rows, err := repository.pool.Query(ctx, consultantSelectQuery()+`
		where c.store_id = $1::uuid
			and c.is_active = true
		order by c.name asc;
	`, storeID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	consultants := make([]Consultant, 0)
	for rows.Next() {
		consultant, err := scanConsultant(rows)
		if err != nil {
			return nil, err
		}

		consultants = append(consultants, consultant)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return consultants, nil
}

func (repository *PostgresRepository) FindByID(ctx context.Context, consultantID string) (Consultant, error) {
	consultant, err := scanConsultant(repository.pool.QueryRow(ctx, consultantSelectQuery()+`
		where c.id = $1::uuid
		limit 1;
	`, consultantID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Consultant{}, ErrConsultantNotFound
		}

		return Consultant{}, err
	}

	return consultant, nil
}

func (repository *PostgresRepository) SyncLinkedIdentity(ctx context.Context, userID string, name string, initials string) error {
	_, err := repository.pool.Exec(ctx, `
		update consultants
		set
			name = $2,
			initials = $3,
			updated_at = now()
		where user_id = $1::uuid
			and is_active = true;
	`, userID, strings.TrimSpace(name), strings.TrimSpace(initials))
	return err
}

func (repository *PostgresRepository) SyncLinkedAccess(ctx context.Context, input LinkedAccessSyncInput) error {
	trimmedUserID := strings.TrimSpace(input.UserID)
	if trimmedUserID == "" {
		return nil
	}

	tx, err := repository.pool.Begin(ctx)
	if err != nil {
		return err
	}

	defer func() {
		_ = tx.Rollback(ctx)
	}()

	var consultantID string
	err = tx.QueryRow(ctx, `
		select id::text
		from consultants
		where user_id = $1::uuid
		order by is_active desc, updated_at desc
		limit 1;
	`, trimmedUserID).Scan(&consultantID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}

		return err
	}

	if input.Role != auth.RoleConsultant {
		if _, err := tx.Exec(ctx, `
			update consultants
			set
				is_active = false,
				updated_at = now()
			where id = $1::uuid;
		`, consultantID); err != nil {
			return err
		}

		return tx.Commit(ctx)
	}

	trimmedName := strings.TrimSpace(input.DisplayName)
	trimmedTenantID := strings.TrimSpace(input.TenantID)
	trimmedStoreID := strings.TrimSpace(input.StoreID)
	if trimmedName == "" || trimmedTenantID == "" || trimmedStoreID == "" {
		return ErrValidation
	}

	if _, err := tx.Exec(ctx, `
		update consultants
		set
			tenant_id = $2::uuid,
			store_id = $3::uuid,
			name = $4,
			initials = $5,
			is_active = $6,
			updated_at = now()
		where id = $1::uuid;
	`, consultantID, trimmedTenantID, trimmedStoreID, trimmedName, buildInitials(trimmedName), input.Active); err != nil {
		if isConsultantNameConflict(err) {
			return ErrConsultantConflict
		}

		return err
	}

	return tx.Commit(ctx)
}

func (repository *PostgresRepository) Create(ctx context.Context, consultant Consultant, access ConsultantAccessSeed) (Consultant, error) {
	tx, err := repository.pool.Begin(ctx)
	if err != nil {
		return Consultant{}, err
	}

	defer func() {
		_ = tx.Rollback(ctx)
	}()

	userID, err := repository.insertConsultantUser(ctx, tx, consultant, access)
	if err != nil {
		return Consultant{}, err
	}

	var createdID string
	err = tx.QueryRow(ctx, `
		insert into consultants (
			tenant_id,
			store_id,
			user_id,
			name,
			role_label,
			initials,
			color,
			monthly_goal,
			commission_rate,
			conversion_goal,
			avg_ticket_goal,
			pa_goal,
			is_active
		)
		values (
			$1::uuid,
			$2::uuid,
			$3::uuid,
			$4,
			$5,
			$6,
			$7,
			$8,
			$9,
			$10,
			$11,
			$12,
			$13
		)
		returning id::text;
	`,
		consultant.TenantID,
		consultant.StoreID,
		userID,
		consultant.Name,
		defaultRoleLabel(consultant.RoleLabel),
		consultant.Initials,
		consultant.Color,
		consultant.MonthlyGoal,
		consultant.CommissionRate,
		consultant.ConversionGoal,
		consultant.AvgTicketGoal,
		consultant.PAGoal,
		consultant.Active,
	).Scan(&createdID)
	if err != nil {
		if isConsultantNameConflict(err) {
			return Consultant{}, ErrConsultantConflict
		}
		if isAccessEmailConflict(err) {
			return Consultant{}, ErrAccessConflict
		}

		return Consultant{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return Consultant{}, err
	}

	return repository.FindByID(ctx, createdID)
}

func (repository *PostgresRepository) AttachAccess(ctx context.Context, consultant Consultant, access ConsultantAccessSeed) (Consultant, error) {
	tx, err := repository.pool.Begin(ctx)
	if err != nil {
		return Consultant{}, err
	}

	defer func() {
		_ = tx.Rollback(ctx)
	}()

	userID, err := repository.insertConsultantUser(ctx, tx, consultant, access)
	if err != nil {
		return Consultant{}, err
	}

	commandTag, err := tx.Exec(ctx, `
		update consultants
		set
			user_id = $2::uuid,
			updated_at = now()
		where id = $1::uuid
			and user_id is null;
	`, consultant.ID, userID)
	if err != nil {
		if isAccessEmailConflict(err) {
			return Consultant{}, ErrAccessConflict
		}

		return Consultant{}, err
	}

	if commandTag.RowsAffected() == 0 {
		return Consultant{}, ErrConsultantNotFound
	}

	if err := tx.Commit(ctx); err != nil {
		return Consultant{}, err
	}

	return repository.FindByID(ctx, consultant.ID)
}

func (repository *PostgresRepository) Update(ctx context.Context, consultant Consultant) (Consultant, error) {
	tx, err := repository.pool.Begin(ctx)
	if err != nil {
		return Consultant{}, err
	}

	defer func() {
		_ = tx.Rollback(ctx)
	}()

	var updatedID string
	err = tx.QueryRow(ctx, `
		update consultants
		set
			tenant_id = $2::uuid,
			store_id = $3::uuid,
			name = $4,
			role_label = $5,
			initials = $6,
			color = $7,
			monthly_goal = $8,
			commission_rate = $9,
			conversion_goal = $10,
			avg_ticket_goal = $11,
			pa_goal = $12,
			is_active = $13,
			updated_at = now()
		where id = $1::uuid
		returning id::text;
	`,
		consultant.ID,
		consultant.TenantID,
		consultant.StoreID,
		consultant.Name,
		defaultRoleLabel(consultant.RoleLabel),
		consultant.Initials,
		consultant.Color,
		consultant.MonthlyGoal,
		consultant.CommissionRate,
		consultant.ConversionGoal,
		consultant.AvgTicketGoal,
		consultant.PAGoal,
		consultant.Active,
	).Scan(&updatedID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Consultant{}, ErrConsultantNotFound
		}
		if isConsultantNameConflict(err) {
			return Consultant{}, ErrConsultantConflict
		}

		return Consultant{}, err
	}

	if strings.TrimSpace(consultant.UserID) != "" {
		if _, err := tx.Exec(ctx, `
			update users
			set
				display_name = $2,
				employee_code = $3,
				job_title = $4,
				is_active = $5,
				updated_at = now()
			where id = $1::uuid;
		`, consultant.UserID, consultant.Name, consultant.EmployeeCode, defaultConsultantJobTitle(consultant.RoleLabel), consultant.Active); err != nil {
			if isAccessEmailConflict(err) || isEmployeeCodeConflict(err) {
				return Consultant{}, ErrAccessConflict
			}

			return Consultant{}, err
		}

		if _, err := tx.Exec(ctx, `
			delete from user_store_roles
			where user_id = $1::uuid
				and role = 'consultant';
		`, consultant.UserID); err != nil {
			return Consultant{}, err
		}

		if _, err := tx.Exec(ctx, `
			insert into user_store_roles (user_id, store_id, role)
			values ($1::uuid, $2::uuid, 'consultant');
		`, consultant.UserID, consultant.StoreID); err != nil {
			return Consultant{}, err
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return Consultant{}, err
	}

	return repository.FindByID(ctx, updatedID)
}

func (repository *PostgresRepository) Archive(ctx context.Context, consultantID string) error {
	tx, err := repository.pool.Begin(ctx)
	if err != nil {
		return err
	}

	defer func() {
		_ = tx.Rollback(ctx)
	}()

	var userID string
	err = tx.QueryRow(ctx, `
		update consultants
		set
			is_active = false,
			updated_at = now()
		where id = $1::uuid
			and is_active = true
		returning coalesce(user_id::text, '');
	`, consultantID).Scan(&userID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrConsultantNotFound
		}

		return err
	}

	if strings.TrimSpace(userID) != "" {
		if _, err := tx.Exec(ctx, `
			update users
			set
				is_active = false,
				updated_at = now()
			where id = $1::uuid;
		`, userID); err != nil {
			return err
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return err
	}

	return nil
}

func (repository *PostgresRepository) insertConsultantUser(ctx context.Context, tx pgx.Tx, consultant Consultant, access ConsultantAccessSeed) (string, error) {
	var userID string
	err := tx.QueryRow(ctx, `
		insert into users (
			email,
			display_name,
			employee_code,
			job_title,
			password_hash,
			must_change_password,
			is_active
		)
		values ($1, $2, $3, $4, $5, true, true)
		returning id::text;
	`, strings.ToLower(strings.TrimSpace(access.Email)), consultant.Name, consultant.EmployeeCode, defaultConsultantJobTitle(consultant.RoleLabel), access.PasswordHash).Scan(&userID)
	if err != nil {
		if isAccessEmailConflict(err) || isEmployeeCodeConflict(err) {
			return "", ErrAccessConflict
		}

		return "", err
	}

	if _, err := tx.Exec(ctx, `
		insert into user_store_roles (user_id, store_id, role)
		values ($1::uuid, $2::uuid, 'consultant');
	`, userID, consultant.StoreID); err != nil {
		if isAccessEmailConflict(err) {
			return "", ErrAccessConflict
		}

		return "", err
	}

	return userID, nil
}

func consultantSelectQuery() string {
	return `
		select
			c.id::text,
			c.tenant_id::text,
			c.store_id::text,
			coalesce(c.user_id::text, '') as user_id,
			coalesce(lower(u.email), '') as access_email,
			coalesce(u.is_active, false) as access_active,
			coalesce(u.employee_code, '') as employee_code,
			c.name,
			c.role_label,
			c.initials,
			c.color,
			c.monthly_goal,
			c.commission_rate,
			c.conversion_goal,
			c.avg_ticket_goal,
			c.pa_goal,
			c.is_active,
			c.created_at,
			c.updated_at
		from consultants c
		left join users u on u.id = c.user_id
	`
}

func scanConsultant(row pgx.Row) (Consultant, error) {
	var consultant Consultant
	err := row.Scan(
		&consultant.ID,
		&consultant.TenantID,
		&consultant.StoreID,
		&consultant.UserID,
		&consultant.AccessEmail,
		&consultant.AccessActive,
		&consultant.EmployeeCode,
		&consultant.Name,
		&consultant.RoleLabel,
		&consultant.Initials,
		&consultant.Color,
		&consultant.MonthlyGoal,
		&consultant.CommissionRate,
		&consultant.ConversionGoal,
		&consultant.AvgTicketGoal,
		&consultant.PAGoal,
		&consultant.Active,
		&consultant.CreatedAt,
		&consultant.UpdatedAt,
	)
	if err != nil {
		return Consultant{}, err
	}

	consultant.UserID = strings.TrimSpace(consultant.UserID)
	consultant.AccessEmail = strings.ToLower(strings.TrimSpace(consultant.AccessEmail))
	consultant.EmployeeCode = strings.TrimSpace(consultant.EmployeeCode)
	consultant.Name = strings.TrimSpace(consultant.Name)
	consultant.RoleLabel = defaultRoleLabel(consultant.RoleLabel)
	consultant.Color = strings.TrimSpace(consultant.Color)

	return consultant, nil
}

func defaultRoleLabel(roleLabel string) string {
	trimmed := strings.TrimSpace(roleLabel)
	if trimmed == "" {
		return "Atendimento"
	}

	return trimmed
}

func defaultConsultantJobTitle(roleLabel string) string {
	trimmed := strings.TrimSpace(roleLabel)
	if trimmed == "" || strings.EqualFold(trimmed, "Atendimento") {
		return "Consultor de Atendimento"
	}

	return trimmed
}

func isConsultantNameConflict(err error) bool {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code == "23505" && pgErr.ConstraintName == "consultants_store_name_active_uidx"
	}

	return false
}

func isAccessEmailConflict(err error) bool {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code == "23505" && pgErr.ConstraintName == "users_email_lower_uidx"
	}

	return false
}

func isEmployeeCodeConflict(err error) bool {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code == "23505" && pgErr.ConstraintName == "users_employee_code_uidx"
	}

	return false
}
