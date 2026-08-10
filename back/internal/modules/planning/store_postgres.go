package planning

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/goalperiod"
)

type PostgresRepository struct{ pool *pgxpool.Pool }

func NewPostgresRepository(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{pool: pool}
}

func (repository *PostgresRepository) FindConfiguration(ctx context.Context, tenantID, storeID string) (*StoreConfiguration, error) {
	var item StoreConfiguration
	err := repository.pool.QueryRow(ctx, `select id::text, store_id::text, configuration, version, updated_at from queue.planning_store_configs where tenant_id=$1::uuid and store_id=$2::uuid`, tenantID, storeID).
		Scan(&item.ID, &item.StoreID, &item.Configuration, &item.Version, &item.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	return &item, err
}

func (repository *PostgresRepository) FindSchedule(ctx context.Context, tenantID, storeID string, weekStart time.Time) (*Schedule, error) {
	var item Schedule
	var shiftsJSON, allocationsJSON []byte
	var week time.Time
	var targetMonth time.Time
	err := repository.pool.QueryRow(ctx, `select id::text, store_id::text, week_start, target_month, goal_week, status, shifts, goal_allocations, version, published_at, updated_at from queue.planning_schedules where tenant_id=$1::uuid and store_id=$2::uuid and week_start=$3::date`, tenantID, storeID, weekStart).
		Scan(&item.ID, &item.StoreID, &week, &targetMonth, &item.GoalWeek, &item.Status, &shiftsJSON, &allocationsJSON, &item.Version, &item.PublishedAt, &item.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if err = json.Unmarshal(shiftsJSON, &item.Shifts); err != nil {
		return nil, err
	}
	if err = json.Unmarshal(allocationsJSON, &item.GoalAllocations); err != nil {
		return nil, err
	}
	item.WeekStart = week.Format("2006-01-02")
	item.TargetMonth = targetMonth.Format("2006-01")
	return &item, nil
}

func (repository *PostgresRepository) ListScheduleRevisions(ctx context.Context, tenantID, storeID, scheduleID string) ([]ScheduleRevision, error) {
	rows, err := repository.pool.Query(ctx, `
		select revision.version, revision.status, coalesce(users.display_name,'Sistema'), revision.created_at
		from queue.planning_schedule_revisions revision
		left join core.users users on users.id=revision.changed_by_user_id
		where revision.tenant_id=$1::uuid and revision.store_id=$2::uuid and revision.schedule_id=$3::uuid
		order by revision.version desc limit 30;
	`, tenantID, storeID, scheduleID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]ScheduleRevision, 0)
	for rows.Next() {
		var item ScheduleRevision
		if err = rows.Scan(&item.Version, &item.Status, &item.ChangedByName, &item.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (repository *PostgresRepository) ListContracts(ctx context.Context, tenantID, storeID string) ([]StaffContract, error) {
	rows, err := repository.pool.Query(ctx, `
		select c.id::text, coalesce(pc.weekly_hours,44), coalesce(pc.max_daily_hours,8),
			coalesce(pc.target_weight,1), coalesce(pc.available_weekdays,array['mon','tue','wed','thu','fri','sat','sun']::text[]), coalesce(pc.version,0)
		from queue.consultants c
		left join queue.planning_staff_contracts pc on pc.tenant_id=c.tenant_id and pc.store_id=c.store_id and pc.consultant_id=c.id
		where c.tenant_id=$1::uuid and c.store_id=$2::uuid and c.is_active=true
		order by lower(c.name), c.id;
	`, tenantID, storeID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	contracts := make([]StaffContract, 0)
	for rows.Next() {
		var contract StaffContract
		if err := rows.Scan(&contract.ConsultantID, &contract.WeeklyHours, &contract.MaxDailyHours, &contract.TargetWeight, &contract.AvailableWeekdays, &contract.Version); err != nil {
			return nil, err
		}
		contracts = append(contracts, contract)
	}
	return contracts, rows.Err()
}

func (repository *PostgresRepository) UpsertConfiguration(ctx context.Context, tenantID, storeID, userID string, configuration json.RawMessage, contracts []StaffContract) (StoreConfiguration, error) {
	tx, err := repository.pool.Begin(ctx)
	if err != nil {
		return StoreConfiguration{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	_, err = tx.Exec(ctx, `insert into queue.planning_store_configs (tenant_id,store_id,configuration,created_by_user_id,updated_by_user_id) values ($1::uuid,$2::uuid,$3::jsonb,$4::uuid,$4::uuid) on conflict (tenant_id,store_id) do update set configuration=excluded.configuration, version=queue.planning_store_configs.version+1, updated_by_user_id=excluded.updated_by_user_id, updated_at=now()`, tenantID, storeID, configuration, nullableUUID(userID))
	if err != nil {
		return StoreConfiguration{}, err
	}
	contractIDs := make([]string, 0, len(contracts))
	for _, contract := range contracts {
		contractIDs = append(contractIDs, contract.ConsultantID)
		_, err = tx.Exec(ctx, `
			insert into queue.planning_staff_contracts (tenant_id,store_id,consultant_id,weekly_hours,max_daily_hours,target_weight,available_weekdays,created_by_user_id,updated_by_user_id)
			values ($1::uuid,$2::uuid,$3::uuid,$4,$5,$6,$7::text[],$8::uuid,$8::uuid)
			on conflict (tenant_id,store_id,consultant_id) do update set weekly_hours=excluded.weekly_hours,max_daily_hours=excluded.max_daily_hours,target_weight=excluded.target_weight,available_weekdays=excluded.available_weekdays,version=queue.planning_staff_contracts.version+1,updated_by_user_id=excluded.updated_by_user_id,updated_at=now();
		`, tenantID, storeID, contract.ConsultantID, contract.WeeklyHours, contract.MaxDailyHours, contract.TargetWeight, contract.AvailableWeekdays, nullableUUID(userID))
		if err != nil {
			return StoreConfiguration{}, err
		}
	}
	_, err = tx.Exec(ctx, `delete from queue.planning_staff_contracts where tenant_id=$1::uuid and store_id=$2::uuid and not (consultant_id=any($3::uuid[]))`, tenantID, storeID, contractIDs)
	if err != nil {
		return StoreConfiguration{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return StoreConfiguration{}, err
	}
	item, err := repository.FindConfiguration(ctx, tenantID, storeID)
	if err != nil || item == nil {
		return StoreConfiguration{}, err
	}
	return *item, nil
}

func (repository *PostgresRepository) FindStoreWeeklyGoal(ctx context.Context, tenantID, storeID string, month time.Time, week int) (float64, error) {
	var target float64
	err := repository.pool.QueryRow(ctx, `
		select coalesce(
			(select monthly_goal from queue.operation_goal_targets where tenant_id=$1::uuid and store_id=$2::uuid and consultant_id is null and target_month=$3::date and week=$4 limit 1),
			(select monthly_goal/(case when extract(day from (date_trunc('month',$3::date)+interval '1 month - 1 day')) > 28 then 5 else 4 end) from queue.operation_goal_targets where tenant_id=$1::uuid and store_id=$2::uuid and consultant_id is null and target_month=$3::date and week=0 limit 1),
			0
		);
	`, tenantID, storeID, month, week).Scan(&target)
	return target, err
}

func (repository *PostgresRepository) SaveSchedule(ctx context.Context, input RepositoryScheduleInput) (Schedule, error) {
	shiftsJSON, err := json.Marshal(input.Shifts)
	if err != nil {
		return Schedule{}, err
	}
	allocationsJSON, err := json.Marshal(input.Allocations)
	if err != nil {
		return Schedule{}, err
	}
	tx, err := repository.pool.Begin(ctx)
	if err != nil {
		return Schedule{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var scheduleID, currentStatus string
	var currentVersion int64
	err = tx.QueryRow(ctx, `select id::text,status,version from queue.planning_schedules where tenant_id=$1::uuid and store_id=$2::uuid and week_start=$3::date for update`, input.TenantID, input.StoreID, input.WeekStart).Scan(&scheduleID, &currentStatus, &currentVersion)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		if input.ExpectedVersion != nil {
			return Schedule{}, ErrVersionConflict
		}
		err = tx.QueryRow(ctx, `
			insert into queue.planning_schedules (tenant_id,store_id,week_start,target_month,goal_week,status,shifts,goal_allocations,published_at,published_by_user_id,created_by_user_id,updated_by_user_id)
			values ($1::uuid,$2::uuid,$3::date,$4::date,$5,$6,$7::jsonb,$8::jsonb,case when $6='published' then now() end,case when $6='published' then $9::uuid end,$9::uuid,$9::uuid)
			returning id::text,version;
		`, input.TenantID, input.StoreID, input.WeekStart, input.TargetMonth, input.GoalWeek, input.Status, shiftsJSON, allocationsJSON, nullableUUID(input.UserID)).Scan(&scheduleID, &currentVersion)
	case err != nil:
		return Schedule{}, err
	default:
		if input.ExpectedVersion == nil || *input.ExpectedVersion != currentVersion {
			return Schedule{}, ErrVersionConflict
		}
		if currentStatus == "published" {
			return Schedule{}, ErrPublished
		}
		currentVersion++
		_, err = tx.Exec(ctx, `
			update queue.planning_schedules set target_month=$4::date,goal_week=$5,status=$6,shifts=$7::jsonb,goal_allocations=$8::jsonb,version=$9,
				published_at=case when $6='published' then now() else null end,published_by_user_id=case when $6='published' then $10::uuid else null end,
				updated_by_user_id=$10::uuid,updated_at=now()
			where tenant_id=$1::uuid and store_id=$2::uuid and week_start=$3::date;
		`, input.TenantID, input.StoreID, input.WeekStart, input.TargetMonth, input.GoalWeek, input.Status, shiftsJSON, allocationsJSON, currentVersion, nullableUUID(input.UserID))
	}
	if err != nil {
		return Schedule{}, err
	}
	_, err = tx.Exec(ctx, `insert into queue.planning_schedule_revisions (tenant_id,store_id,schedule_id,version,status,shifts,goal_allocations,changed_by_user_id) values ($1::uuid,$2::uuid,$3::uuid,$4,$5,$6::jsonb,$7::jsonb,$8::uuid)`, input.TenantID, input.StoreID, scheduleID, currentVersion, input.Status, shiftsJSON, allocationsJSON, nullableUUID(input.UserID))
	if err != nil {
		return Schedule{}, err
	}
	if err = replaceWeeklyAllocations(ctx, tx, input, input.Allocations); err != nil {
		return Schedule{}, err
	}
	if err = seedMissingWeeklyAllocations(ctx, tx, input, input.Allocations); err != nil {
		return Schedule{}, err
	}
	if err = replaceMonthlyAllocationSummary(ctx, tx, input); err != nil {
		return Schedule{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return Schedule{}, err
	}
	item, err := repository.FindSchedule(ctx, input.TenantID, input.StoreID, input.WeekStart)
	if err != nil || item == nil {
		return Schedule{}, err
	}
	return *item, nil
}

func replaceWeeklyAllocations(ctx context.Context, tx pgx.Tx, input RepositoryScheduleInput, allocations []GoalAllocation) error {
	_, err := tx.Exec(ctx, `delete from queue.operation_goal_targets where tenant_id=$1::uuid and store_id=$2::uuid and consultant_id is not null and target_month=$3::date and week=$4`, input.TenantID, input.StoreID, input.TargetMonth, input.GoalWeek)
	if err != nil {
		return err
	}
	return insertWeeklyAllocations(ctx, tx, input, input.GoalWeek, allocations)
}

func seedMissingWeeklyAllocations(ctx context.Context, tx pgx.Tx, input RepositoryScheduleInput, allocations []GoalAllocation) error {
	if len(allocations) == 0 {
		return nil
	}
	for week := 1; week <= goalperiod.Count(input.TargetMonth); week++ {
		if week == input.GoalWeek {
			continue
		}
		var target float64
		var hasGeneratedTargets bool
		err := tx.QueryRow(ctx, `
			select
				coalesce(
					(select monthly_goal from queue.operation_goal_targets where tenant_id=$1::uuid and store_id=$2::uuid and consultant_id is null and target_month=$3::date and week=$4 limit 1),
					(select monthly_goal/(case when extract(day from (date_trunc('month',$3::date)+interval '1 month - 1 day')) > 28 then 5 else 4 end) from queue.operation_goal_targets where tenant_id=$1::uuid and store_id=$2::uuid and consultant_id is null and target_month=$3::date and week=0 limit 1),
					0
				)::float8,
				exists(
					select 1 from queue.operation_goal_targets
					where tenant_id=$1::uuid and store_id=$2::uuid and consultant_id is not null
						and target_month=$3::date and week=$4 and monthly_goal > 0
				);
		`, input.TenantID, input.StoreID, input.TargetMonth, week).Scan(&target, &hasGeneratedTargets)
		if err != nil {
			return err
		}
		if target <= 0 || hasGeneratedTargets {
			continue
		}
		_, err = tx.Exec(ctx, `delete from queue.operation_goal_targets where tenant_id=$1::uuid and store_id=$2::uuid and consultant_id is not null and target_month=$3::date and week=$4`, input.TenantID, input.StoreID, input.TargetMonth, week)
		if err != nil {
			return err
		}
		if err = insertWeeklyAllocations(ctx, tx, input, week, scaleGoalAllocations(target, allocations)); err != nil {
			return err
		}
	}
	return nil
}

func insertWeeklyAllocations(ctx context.Context, tx pgx.Tx, input RepositoryScheduleInput, week int, allocations []GoalAllocation) error {
	for _, allocation := range allocations {
		_, err := tx.Exec(ctx, `
			insert into queue.operation_goal_targets (tenant_id,store_id,consultant_id,target_month,week,monthly_goal,avg_ticket_goal,conversion_goal,pa_goal,created_by_user_id,updated_by_user_id)
			select $1::uuid,$2::uuid,$3::uuid,$4::date,$5,$6,
				coalesce(g.avg_ticket_goal,0),coalesce(g.conversion_goal,0),coalesce(g.pa_goal,0),$7::uuid,$7::uuid
			from (values (1)) seed(value)
			left join lateral (
				select avg_ticket_goal,conversion_goal,pa_goal from queue.operation_goal_targets
				where tenant_id=$1::uuid and store_id=$2::uuid and consultant_id is null and target_month=$4::date and week in ($5,0)
				order by case when week=$5 then 0 else 1 end limit 1
			) g on true;
		`, input.TenantID, input.StoreID, allocation.ConsultantID, input.TargetMonth, week, allocation.Target, nullableUUID(input.UserID))
		if err != nil {
			return err
		}
	}
	return nil
}

func replaceMonthlyAllocationSummary(ctx context.Context, tx pgx.Tx, input RepositoryScheduleInput) error {
	rows, err := tx.Query(ctx, `
		select goals.consultant_id::text,sum(goals.monthly_goal)::float8
		from queue.operation_goal_targets goals
		join queue.consultants consultants on consultants.id=goals.consultant_id
			and consultants.tenant_id=goals.tenant_id and consultants.store_id=goals.store_id and consultants.is_active=true
		where goals.tenant_id=$1::uuid and goals.store_id=$2::uuid
			and goals.target_month=$3::date and goals.week between 1 and 5
		group by goals.consultant_id;
	`, input.TenantID, input.StoreID, input.TargetMonth)
	if err != nil {
		return err
	}
	weeklyTotals := map[string]float64{}
	for rows.Next() {
		var consultantID string
		var target float64
		if err = rows.Scan(&consultantID, &target); err != nil {
			rows.Close()
			return err
		}
		weeklyTotals[consultantID] = target
	}
	if err = rows.Err(); err != nil {
		rows.Close()
		return err
	}
	rows.Close()

	var monthlyTarget float64
	err = tx.QueryRow(ctx, `
		select coalesce(
			(select monthly_goal from queue.operation_goal_targets where tenant_id=$1::uuid and store_id=$2::uuid and consultant_id is null and target_month=$3::date and week=0 limit 1),
			(select sum(monthly_goal) from queue.operation_goal_targets where tenant_id=$1::uuid and store_id=$2::uuid and consultant_id is null and target_month=$3::date and week between 1 and 5),
			0
		)::float8;
	`, input.TenantID, input.StoreID, input.TargetMonth).Scan(&monthlyTarget)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `delete from queue.operation_goal_targets where tenant_id=$1::uuid and store_id=$2::uuid and consultant_id is not null and target_month=$3::date and week=0`, input.TenantID, input.StoreID, input.TargetMonth)
	if err != nil {
		return err
	}
	for _, goal := range calculateMonthlyConsultantGoals(monthlyTarget, weeklyTotals) {
		_, err = tx.Exec(ctx, `
			insert into queue.operation_goal_targets (tenant_id,store_id,consultant_id,target_month,week,monthly_goal,avg_ticket_goal,conversion_goal,pa_goal,created_by_user_id,updated_by_user_id)
			select $1::uuid,$2::uuid,$3::uuid,$4::date,0,$5,
				coalesce(g.avg_ticket_goal,0),coalesce(g.conversion_goal,0),coalesce(g.pa_goal,0),$6::uuid,$6::uuid
			from (values (1)) seed(value)
			left join lateral (
				select avg_ticket_goal,conversion_goal,pa_goal from queue.operation_goal_targets
				where tenant_id=$1::uuid and store_id=$2::uuid and consultant_id is null and target_month=$4::date and week between 0 and 5
				order by case when week=0 then 0 else 1 end,week limit 1
			) g on true;
		`, input.TenantID, input.StoreID, goal.ConsultantID, input.TargetMonth, goal.Target, nullableUUID(input.UserID))
		if err != nil {
			return err
		}
	}
	return nil
}

func (repository *PostgresRepository) ReopenSchedule(ctx context.Context, tenantID, storeID, userID string, weekStart time.Time, expectedVersion int64) (Schedule, error) {
	tx, err := repository.pool.Begin(ctx)
	if err != nil {
		return Schedule{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var scheduleID, status string
	var version int64
	err = tx.QueryRow(ctx, `select id::text,status,version from queue.planning_schedules where tenant_id=$1::uuid and store_id=$2::uuid and week_start=$3::date for update`, tenantID, storeID, weekStart).Scan(&scheduleID, &status, &version)
	if errors.Is(err, pgx.ErrNoRows) {
		return Schedule{}, ErrScheduleNotFound
	}
	if err != nil {
		return Schedule{}, err
	}
	if version != expectedVersion {
		return Schedule{}, ErrVersionConflict
	}
	if status != "published" {
		return Schedule{}, ErrValidation
	}
	version++
	_, err = tx.Exec(ctx, `update queue.planning_schedules set status='saved',version=$4,published_at=null,published_by_user_id=null,updated_by_user_id=$5::uuid,updated_at=now() where tenant_id=$1::uuid and store_id=$2::uuid and week_start=$3::date`, tenantID, storeID, weekStart, version, nullableUUID(userID))
	if err != nil {
		return Schedule{}, err
	}
	_, err = tx.Exec(ctx, `insert into queue.planning_schedule_revisions (tenant_id,store_id,schedule_id,version,status,shifts,goal_allocations,changed_by_user_id) select tenant_id,store_id,id,$4,status,shifts,goal_allocations,$5::uuid from queue.planning_schedules where tenant_id=$1::uuid and store_id=$2::uuid and week_start=$3::date`, tenantID, storeID, weekStart, version, nullableUUID(userID))
	if err != nil {
		return Schedule{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return Schedule{}, err
	}
	item, err := repository.FindSchedule(ctx, tenantID, storeID, weekStart)
	if err != nil || item == nil {
		return Schedule{}, err
	}
	return *item, nil
}

func (repository *PostgresRepository) CountStoreConsultants(ctx context.Context, tenantID, storeID string, ids []string) (int, error) {
	if len(ids) == 0 {
		return 0, nil
	}
	var count int
	err := repository.pool.QueryRow(ctx, `select count(*) from queue.consultants where tenant_id=$1::uuid and store_id=$2::uuid and is_active=true and id=any($3::uuid[])`, tenantID, storeID, ids).Scan(&count)
	return count, err
}

func nullableUUID(value string) any {
	if value == "" {
		return nil
	}
	return value
}
