package performancefeedback

import (
	"context"
	"encoding/json"
	"errors"
	"math"
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

func (repository *PostgresRepository) ListConsultants(ctx context.Context, tenantID string, storeID string, userID string) ([]Consultant, error) {
	rows, err := repository.pool.Query(ctx, `
		select
			c.id::text,
			coalesce(c.user_id::text, ''),
			c.name,
			c.initials,
			c.color
		from queue.consultants c
		where c.tenant_id = $1::uuid
		  and c.store_id = $2::uuid
		  and c.is_active = true
		  and ($3::uuid is null or c.user_id = $3::uuid)
		order by lower(c.name), c.id;
	`, tenantID, storeID, nullableID(userID))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]Consultant, 0)
	for rows.Next() {
		var item Consultant
		if err := rows.Scan(&item.ID, &item.UserID, &item.Name, &item.Initials, &item.Color); err != nil {
			return nil, err
		}
		normalizeConsultant(&item)
		items = append(items, item)
	}
	return items, rows.Err()
}

func (repository *PostgresRepository) FindConsultant(ctx context.Context, tenantID string, storeID string, consultantID string) (Consultant, error) {
	var item Consultant
	err := repository.pool.QueryRow(ctx, `
		select
			c.id::text,
			coalesce(c.user_id::text, ''),
			c.name,
			c.initials,
			c.color
		from queue.consultants c
		where c.tenant_id = $1::uuid
		  and c.store_id = $2::uuid
		  and c.id = $3::uuid
		  and c.is_active = true
		limit 1;
	`, tenantID, storeID, consultantID).Scan(
		&item.ID,
		&item.UserID,
		&item.Name,
		&item.Initials,
		&item.Color,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return Consultant{}, ErrConsultantNotFound
	}
	if err != nil {
		return Consultant{}, err
	}
	normalizeConsultant(&item)
	return item, nil
}

func (repository *PostgresRepository) LoadGoal(ctx context.Context, tenantID string, storeID string, consultantID string, period Period) (GoalSnapshot, error) {
	month, err := time.Parse(monthLayout, period.Month)
	if err != nil {
		return GoalSnapshot{}, ErrValidation
	}

	var goal GoalSnapshot
	err = repository.pool.QueryRow(ctx, `
		select
			coalesce(target.monthly_goal, 0)::float8,
			coalesce(nullif(target.avg_ticket_goal, 0), s.avg_ticket_goal, 0)::float8,
			coalesce(nullif(target.conversion_goal, 0), s.conversion_goal, 0)::float8,
			coalesce(nullif(target.pa_goal, 0), s.pa_goal, 0)::float8
		from queue.stores s
		left join lateral (
			select
				g.monthly_goal,
				g.avg_ticket_goal,
				g.conversion_goal,
				g.pa_goal
			from queue.operation_goal_targets g
			where g.tenant_id = $1::uuid
			  and g.store_id = $2::uuid
			  and g.target_month = $4::date
			  and (g.consultant_id = $3::uuid or g.consultant_id is null)
			  and (g.week = $5 or g.week = 0)
			order by
				case
					when g.consultant_id = $3::uuid and g.week = $5 then 0
					when g.consultant_id is null and g.week = $5 then 1
					when g.consultant_id = $3::uuid and g.week = 0 then 2
					else 3
				end
			limit 1
		) target on true
		where s.tenant_id = $1::uuid
		  and s.id = $2::uuid
		limit 1;
	`, tenantID, storeID, consultantID, month, period.Week).Scan(
		&goal.SalesGoal,
		&goal.TicketGoal,
		&goal.ConversionGoal,
		&goal.PAGoal,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return GoalSnapshot{}, ErrNotFound
	}
	return goal, err
}

func (repository *PostgresRepository) LoadTranscriptionScore(
	ctx context.Context,
	tenantID string,
	storeID string,
	consultantID string,
	period Period,
) (*float64, int, error) {
	rows, err := repository.pool.Query(ctx, `
		select recording.analysis_report
		from queue.attendance_recordings recording
		where recording.account_id = $1::uuid
		  and recording.store_id = $2::uuid
		  and recording.consultant_id = $3::uuid
		  and recording.analysis_status = 'completed'
		  and (to_timestamp(recording.started_at / 1000.0) at time zone 'America/Sao_Paulo')::date
			  between $4::date and $5::date
		order by recording.started_at desc;
	`, tenantID, storeID, consultantID, period.DateFrom, period.DateTo)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	total := 0.0
	samples := 0
	for rows.Next() {
		var report json.RawMessage
		if err := rows.Scan(&report); err != nil {
			return nil, 0, err
		}
		if score, ok := extractTranscriptionScore(report); ok {
			total += score
			samples++
		}
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	if samples == 0 {
		return nil, 0, nil
	}
	average := math.Round((total/float64(samples))*100) / 100
	return &average, samples, nil
}

func (repository *PostgresRepository) FindByPeriod(ctx context.Context, tenantID string, storeID string, consultantID string, period Period) (Review, error) {
	month, err := time.Parse(monthLayout, period.Month)
	if err != nil {
		return Review{}, ErrValidation
	}
	return scanReview(repository.pool.QueryRow(ctx, reviewSelect+`
		where review.tenant_id = $1::uuid
		  and review.store_id = $2::uuid
		  and review.consultant_id = $3::uuid
		  and review.period_month = $4::date
		  and review.week = $5
		limit 1;
	`, tenantID, storeID, consultantID, month, period.Week))
}

func (repository *PostgresRepository) FindByID(ctx context.Context, tenantID string, reviewID string) (Review, error) {
	return scanReview(repository.pool.QueryRow(ctx, reviewSelect+`
		where review.tenant_id = $1::uuid
		  and review.id = $2::uuid
		limit 1;
	`, tenantID, reviewID))
}

func (repository *PostgresRepository) ListHistory(ctx context.Context, tenantID string, storeID string, consultantID string, limit int) ([]HistoryItem, error) {
	rows, err := repository.pool.Query(ctx, `
		select id::text, period_month, week, status, metrics_snapshot, updated_at, version
		from queue.performance_feedback_reviews
		where tenant_id = $1::uuid
		  and store_id = $2::uuid
		  and consultant_id = $3::uuid
		  and status <> 'draft'
		order by period_month desc, week desc, updated_at desc
		limit $4;
	`, tenantID, storeID, consultantID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]HistoryItem, 0)
	for rows.Next() {
		var item HistoryItem
		var month time.Time
		var metricsJSON []byte
		if err := rows.Scan(&item.ID, &month, &item.Period.Week, &item.Status, &metricsJSON, &item.UpdatedAt, &item.Version); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(metricsJSON, &item.Metrics); err != nil {
			return nil, err
		}
		item.Period = periodFromStorage(month, item.Period.Week)
		items = append(items, item)
	}
	return items, rows.Err()
}

func (repository *PostgresRepository) UpsertManager(ctx context.Context, review Review, expectedVersion int) (Review, error) {
	metricsJSON, err := json.Marshal(review.Metrics)
	if err != nil {
		return Review{}, err
	}
	sectionsJSON, err := json.Marshal(review.FeedbackSections)
	if err != nil {
		return Review{}, err
	}
	month, err := time.Parse(monthLayout, review.Period.Month)
	if err != nil {
		return Review{}, ErrValidation
	}

	if review.ID == "" {
		var createdID string
		err = repository.pool.QueryRow(ctx, `
			insert into queue.performance_feedback_reviews (
				tenant_id, store_id, consultant_id, consultant_user_id,
				period_month, week, status, feedback_sections,
				metrics_snapshot, created_by_user_id, updated_by_user_id, shared_at
			)
			values (
				$1::uuid, $2::uuid, $3::uuid, $4::uuid,
				$5::date, $6, $7, $8::jsonb,
				$9::jsonb, $10::uuid, $11::uuid, $12
			)
			returning id::text;
		`,
			review.TenantID,
			review.StoreID,
			review.ConsultantID,
			nullableID(review.ConsultantUserID),
			month,
			review.Period.Week,
			review.Status,
			sectionsJSON,
			metricsJSON,
			nullableID(review.CreatedByUserID),
			nullableID(review.UpdatedByUserID),
			review.SharedAt,
		).Scan(&createdID)
		if err != nil {
			if isUniqueViolation(err) {
				return Review{}, ErrConflict
			}
			return Review{}, err
		}
		return repository.FindByID(ctx, review.TenantID, createdID)
	}

	var updatedID string
	err = repository.pool.QueryRow(ctx, `
		update queue.performance_feedback_reviews
	set
			status = $4,
			feedback_sections = $5::jsonb,
			metrics_snapshot = $6::jsonb,
			updated_by_user_id = $7::uuid,
			shared_at = $8,
			updated_at = now(),
			version = version + 1
		where tenant_id = $1::uuid
		  and id = $2::uuid
		  and version = $3
		returning id::text;
	`, review.TenantID, review.ID, expectedVersion, review.Status, sectionsJSON, metricsJSON, nullableID(review.UpdatedByUserID), review.SharedAt).Scan(&updatedID)
	if errors.Is(err, pgx.ErrNoRows) {
		return Review{}, ErrConflict
	}
	if err != nil {
		return Review{}, err
	}
	return repository.FindByID(ctx, review.TenantID, updatedID)
}

func (repository *PostgresRepository) UpdateConsultant(ctx context.Context, review Review, expectedVersion int) (Review, error) {
	var updatedID string
	err := repository.pool.QueryRow(ctx, `
		update queue.performance_feedback_reviews
		set
			status = $4,
			consultant_notes_html = $5,
			acknowledged_at = $6,
			updated_by_user_id = $7::uuid,
			updated_at = now(),
			version = version + 1
		where tenant_id = $1::uuid
		  and id = $2::uuid
		  and version = $3
		returning id::text;
	`, review.TenantID, review.ID, expectedVersion, review.Status, review.ConsultantNotesHTML, review.AcknowledgedAt, nullableID(review.UpdatedByUserID)).Scan(&updatedID)
	if errors.Is(err, pgx.ErrNoRows) {
		return Review{}, ErrConflict
	}
	if err != nil {
		return Review{}, err
	}
	return repository.FindByID(ctx, review.TenantID, updatedID)
}

const reviewSelect = `
	select
		review.id::text,
		review.tenant_id::text,
		review.store_id::text,
		store.name,
		review.consultant_id::text,
		coalesce(review.consultant_user_id::text, ''),
		consultant.name,
		review.period_month,
		review.week,
		review.status,
		review.feedback_sections,
		review.consultant_notes_html,
		review.metrics_snapshot,
		coalesce(review.created_by_user_id::text, ''),
		coalesce(review.updated_by_user_id::text, ''),
		review.shared_at,
		review.acknowledged_at,
		review.created_at,
		review.updated_at,
		review.version
	from queue.performance_feedback_reviews review
	join queue.stores store
	  on store.id = review.store_id
	 and store.tenant_id = review.tenant_id
	join queue.consultants consultant
	  on consultant.id = review.consultant_id
	 and consultant.tenant_id = review.tenant_id
`

type reviewScanner interface {
	Scan(dest ...any) error
}

func scanReview(scanner reviewScanner) (Review, error) {
	var review Review
	var month time.Time
	var metricsJSON []byte
	var sectionsJSON []byte
	err := scanner.Scan(
		&review.ID,
		&review.TenantID,
		&review.StoreID,
		&review.StoreName,
		&review.ConsultantID,
		&review.ConsultantUserID,
		&review.ConsultantName,
		&month,
		&review.Period.Week,
		&review.Status,
		&sectionsJSON,
		&review.ConsultantNotesHTML,
		&metricsJSON,
		&review.CreatedByUserID,
		&review.UpdatedByUserID,
		&review.SharedAt,
		&review.AcknowledgedAt,
		&review.CreatedAt,
		&review.UpdatedAt,
		&review.Version,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return Review{}, ErrNotFound
	}
	if err != nil {
		return Review{}, err
	}
	if err := json.Unmarshal(metricsJSON, &review.Metrics); err != nil {
		return Review{}, err
	}
	if err := json.Unmarshal(sectionsJSON, &review.FeedbackSections); err != nil {
		return Review{}, err
	}
	review.Period = periodFromStorage(month, review.Period.Week)
	return review, nil
}

func extractTranscriptionScore(raw json.RawMessage) (float64, bool) {
	var report map[string]any
	if len(raw) == 0 || json.Unmarshal(raw, &report) != nil {
		return 0, false
	}
	for _, key := range []string{"overallScore", "overall_score", "score", "nota"} {
		if value, ok := numericValue(report[key]); ok {
			return normalizeTranscriptionScore(value)
		}
	}
	for _, containerKey := range []string{"quality", "evaluation", "result"} {
		container, ok := report[containerKey].(map[string]any)
		if !ok {
			continue
		}
		for _, key := range []string{"overallScore", "overall_score", "score", "nota"} {
			if value, ok := numericValue(container[key]); ok {
				return normalizeTranscriptionScore(value)
			}
		}
	}
	return 0, false
}

func numericValue(value any) (float64, bool) {
	switch typed := value.(type) {
	case float64:
		return typed, true
	case json.Number:
		parsed, err := typed.Float64()
		return parsed, err == nil
	default:
		return 0, false
	}
}

func normalizeTranscriptionScore(value float64) (float64, bool) {
	if math.IsNaN(value) || math.IsInf(value, 0) || value < 0 || value > 100 {
		return 0, false
	}
	if value <= 1 {
		value *= 10
	} else if value > 10 {
		value /= 10
	}
	return math.Round(value*100) / 100, true
}

func normalizeConsultant(consultant *Consultant) {
	consultant.ID = strings.TrimSpace(consultant.ID)
	consultant.UserID = strings.TrimSpace(consultant.UserID)
	consultant.Name = strings.TrimSpace(consultant.Name)
	consultant.Initials = strings.TrimSpace(consultant.Initials)
	consultant.Color = strings.TrimSpace(consultant.Color)
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
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}
