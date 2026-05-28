package operationgoals

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresRepository(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{pool: pool}
}

func (repository *PostgresRepository) List(ctx context.Context, input RepositoryListInput) ([]GoalTarget, error) {
	storeIDs := input.StoreIDs
	if len(storeIDs) == 0 {
		storeIDs = nil
	}

	rows, err := repository.pool.Query(ctx, `
		select
			g.id::text,
			g.tenant_id::text,
			g.store_id::text,
			coalesce(s.code, ''),
			coalesce(s.name, ''),
			coalesce(g.consultant_id::text, ''),
			coalesce(c.name, ''),
			g.target_month,
			g.monthly_goal,
			g.avg_ticket_goal,
			g.conversion_goal,
			g.pa_goal,
			coalesce(g.created_by_user_id::text, ''),
			coalesce(g.updated_by_user_id::text, ''),
			g.created_at,
			g.updated_at
		from queue.operation_goal_targets g
		join queue.stores s on s.id = g.store_id
		left join queue.consultants c on c.id = g.consultant_id
		where g.target_month = $1::date
		  and ($2::uuid[] is null or g.store_id = any($2::uuid[]))
		order by
			lower(coalesce(s.name, '')) asc,
			case when g.consultant_id is null then 0 else 1 end asc,
			lower(coalesce(c.name, '')) asc,
			g.created_at asc;
	`, input.Month, storeIDs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]GoalTarget, 0)
	for rows.Next() {
		item, err := scanGoalTarget(rows)
		if err != nil {
			return nil, err
		}

		items = append(items, item)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return items, nil
}

func (repository *PostgresRepository) FindByID(ctx context.Context, id string) (GoalTarget, error) {
	goal, err := scanGoalTarget(repository.pool.QueryRow(ctx, `
		select
			g.id::text,
			g.tenant_id::text,
			g.store_id::text,
			coalesce(s.code, ''),
			coalesce(s.name, ''),
			coalesce(g.consultant_id::text, ''),
			coalesce(c.name, ''),
			g.target_month,
			g.monthly_goal,
			g.avg_ticket_goal,
			g.conversion_goal,
			g.pa_goal,
			coalesce(g.created_by_user_id::text, ''),
			coalesce(g.updated_by_user_id::text, ''),
			g.created_at,
			g.updated_at
		from queue.operation_goal_targets g
		join queue.stores s on s.id = g.store_id
		left join queue.consultants c on c.id = g.consultant_id
		where g.id = $1::uuid
		limit 1;
	`, id))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return GoalTarget{}, ErrGoalNotFound
		}

		return GoalTarget{}, err
	}

	return goal, nil
}

func (repository *PostgresRepository) Create(ctx context.Context, goal GoalTarget) (GoalTarget, error) {
	var createdID string
	err := repository.pool.QueryRow(ctx, `
		insert into queue.operation_goal_targets (
			tenant_id,
			store_id,
			consultant_id,
			target_month,
			monthly_goal,
			avg_ticket_goal,
			conversion_goal,
			pa_goal,
			created_by_user_id,
			updated_by_user_id
		)
		values (
			$1::uuid,
			$2::uuid,
			$3::uuid,
			$4::date,
			$5,
			$6,
			$7,
			$8,
			$9::uuid,
			$10::uuid
		)
		returning id::text;
	`,
		goal.TenantID,
		goal.StoreID,
		nullableID(goal.ConsultantID),
		goal.TargetMonth,
		goal.MonthlyGoal,
		goal.AvgTicketGoal,
		goal.ConversionGoal,
		goal.PAGoal,
		nullableID(goal.CreatedByUserID),
		nullableID(goal.UpdatedByUserID),
	).Scan(&createdID)
	if err != nil {
		if isUniqueViolation(err) {
			return GoalTarget{}, ErrGoalConflict
		}

		return GoalTarget{}, err
	}

	return repository.FindByID(ctx, createdID)
}

func (repository *PostgresRepository) Update(ctx context.Context, goal GoalTarget) (GoalTarget, error) {
	var updatedID string
	err := repository.pool.QueryRow(ctx, `
		update queue.operation_goal_targets
		set
			monthly_goal = $2,
			avg_ticket_goal = $3,
			conversion_goal = $4,
			pa_goal = $5,
			updated_by_user_id = $6::uuid,
			updated_at = now()
		where id = $1::uuid
		returning id::text;
	`,
		goal.ID,
		goal.MonthlyGoal,
		goal.AvgTicketGoal,
		goal.ConversionGoal,
		goal.PAGoal,
		nullableID(goal.UpdatedByUserID),
	).Scan(&updatedID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return GoalTarget{}, ErrGoalNotFound
		}

		return GoalTarget{}, err
	}

	return repository.FindByID(ctx, updatedID)
}

func (repository *PostgresRepository) Delete(ctx context.Context, id string) error {
	commandTag, err := repository.pool.Exec(ctx, `
		delete from queue.operation_goal_targets
		where id = $1::uuid;
	`, id)
	if err != nil {
		return err
	}

	if commandTag.RowsAffected() == 0 {
		return ErrGoalNotFound
	}

	return nil
}

func (repository *PostgresRepository) FindConsultantByID(ctx context.Context, consultantID string) (ConsultantReference, error) {
	var reference ConsultantReference
	err := repository.pool.QueryRow(ctx, `
		select
			id::text,
			tenant_id::text,
			store_id::text,
			name
		from queue.consultants
		where id = $1::uuid
		  and is_active = true
		limit 1;
	`, consultantID).Scan(
		&reference.ID,
		&reference.TenantID,
		&reference.StoreID,
		&reference.Name,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ConsultantReference{}, ErrConsultantNotFound
		}

		return ConsultantReference{}, err
	}

	reference.Name = strings.TrimSpace(reference.Name)
	return reference, nil
}

type goalScanner interface {
	Scan(dest ...any) error
}

func scanGoalTarget(scanner goalScanner) (GoalTarget, error) {
	var goal GoalTarget
	var targetMonth time.Time
	err := scanner.Scan(
		&goal.ID,
		&goal.TenantID,
		&goal.StoreID,
		&goal.StoreCode,
		&goal.StoreName,
		&goal.ConsultantID,
		&goal.ConsultantName,
		&targetMonth,
		&goal.MonthlyGoal,
		&goal.AvgTicketGoal,
		&goal.ConversionGoal,
		&goal.PAGoal,
		&goal.CreatedByUserID,
		&goal.UpdatedByUserID,
		&goal.CreatedAt,
		&goal.UpdatedAt,
	)
	if err != nil {
		return GoalTarget{}, err
	}

	goal.StoreCode = strings.TrimSpace(goal.StoreCode)
	goal.StoreName = strings.TrimSpace(goal.StoreName)
	goal.ConsultantID = strings.TrimSpace(goal.ConsultantID)
	goal.ConsultantName = strings.TrimSpace(goal.ConsultantName)
	goal.CreatedByUserID = strings.TrimSpace(goal.CreatedByUserID)
	goal.UpdatedByUserID = strings.TrimSpace(goal.UpdatedByUserID)
	goal.TargetMonth = time.Date(targetMonth.Year(), targetMonth.Month(), 1, 0, 0, 0, 0, time.UTC)

	return goal, nil
}

func nullableID(value string) any {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil
	}

	return trimmed
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code == "23505"
	}

	return false
}
