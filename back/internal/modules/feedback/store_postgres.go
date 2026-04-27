package feedback

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresRepository(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{pool: pool}
}

func nullableUUID(s string) interface{} {
	if s == "" {
		return nil
	}
	return s
}

func (r *PostgresRepository) Create(feedback *Feedback) (*Feedback, error) {
	var id string
	err := r.pool.QueryRow(context.Background(), `
		insert into user_feedback (
			tenant_id, store_id, user_id, user_name,
			kind, status, subject, body, admin_note
		) values (
			$1::uuid, $2::uuid, $3::uuid, $4,
			$5, $6, $7, $8, $9
		) returning id::text, created_at, updated_at;
	`,
		nullableUUID(feedback.TenantID),
		nullableUUID(feedback.StoreID),
		feedback.UserID,
		feedback.UserName,
		feedback.Kind,
		feedback.Status,
		feedback.Subject,
		feedback.Body,
		feedback.AdminNote,
	).Scan(&id, &feedback.CreatedAt, &feedback.UpdatedAt)

	if err != nil {
		return nil, err
	}

	feedback.ID = id
	return feedback, nil
}

func scanFeedback(scan func(...any) error) (*Feedback, error) {
	var f Feedback
	var tenantID, storeID *string
	err := scan(
		&f.ID, &tenantID, &storeID, &f.UserID, &f.UserName,
		&f.Kind, &f.Status, &f.Subject, &f.Body, &f.AdminNote,
		&f.CreatedAt, &f.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	if tenantID != nil {
		f.TenantID = *tenantID
	}
	if storeID != nil {
		f.StoreID = *storeID
	}
	return &f, nil
}

func (r *PostgresRepository) GetByID(id string) (*Feedback, error) {
	row := r.pool.QueryRow(context.Background(), `
		select id::text, tenant_id::text, store_id::text, user_id::text, user_name,
		       kind, status, subject, body, admin_note, created_at, updated_at
		from user_feedback
		where id = $1::uuid
		limit 1;
	`, id)

	f, err := scanFeedback(row.Scan)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return f, nil
}

func (r *PostgresRepository) List(tenantID string, input ListInput) ([]Feedback, error) {
	query := `
		select id::text, tenant_id::text, store_id::text, user_id::text, user_name,
		       kind, status, subject, body, admin_note, created_at, updated_at
		from user_feedback
		where 1=1
	`

	args := []interface{}{}
	argCount := 1

	if tenantID != "" {
		query += fmt.Sprintf(" and tenant_id = $%d::uuid", argCount)
		args = append(args, tenantID)
		argCount++
	}

	if input.Kind != "" {
		query += fmt.Sprintf(" and kind = $%d", argCount)
		args = append(args, input.Kind)
		argCount++
	}

	if input.Status != "" {
		query += fmt.Sprintf(" and status = $%d", argCount)
		args = append(args, input.Status)
		argCount++
	}

	query += " order by created_at desc;"

	rows, err := r.pool.Query(context.Background(), query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	feedbacks := make([]Feedback, 0)
	for rows.Next() {
		f, err := scanFeedback(rows.Scan)
		if err != nil {
			return nil, err
		}
		feedbacks = append(feedbacks, *f)
	}

	return feedbacks, rows.Err()
}

func (r *PostgresRepository) Update(feedback *Feedback) error {
	_, err := r.pool.Exec(context.Background(), `
		update user_feedback
		set status = $2, admin_note = $3, updated_at = now()
		where id = $1::uuid;
	`,
		feedback.ID,
		feedback.Status,
		feedback.AdminNote,
	)
	return err
}
