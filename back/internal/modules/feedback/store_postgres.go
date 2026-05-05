package feedback

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

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
	ctx := context.Background()
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	err = tx.QueryRow(ctx, `
		insert into user_feedback (
			tenant_id, store_id, user_id, user_name,
			kind, status, subject, body, admin_note
		) values (
			$1::uuid, $2::uuid, $3::uuid, $4,
			$5, $6, $7, $8, $9
		) returning id::text, created_at, updated_at, user_last_read_at;
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
	).Scan(&id, &feedback.CreatedAt, &feedback.UpdatedAt, &feedback.UserLastReadAt)

	if err != nil {
		return nil, err
	}

	feedback.ID = id

	_, err = tx.Exec(ctx, `
		insert into feedback_read_states (
			feedback_id, user_id, last_read_at
		) values (
			$1::uuid, $2::uuid, $3
		)
		on conflict (feedback_id, user_id)
		do update set
			last_read_at = greatest(feedback_read_states.last_read_at, excluded.last_read_at),
			updated_at = now();
	`,
		feedback.ID,
		feedback.UserID,
		feedback.CreatedAt,
	)
	if err != nil {
		return nil, err
	}

	_, err = tx.Exec(ctx, `
		insert into feedback_messages (
			tenant_id, feedback_id, author_user_id, author_name, author_role, body,
			image_path, image_content_type, image_size_bytes
		) values (
			$1::uuid, $2::uuid, $3::uuid, $4, $5, $6, $7, $8, $9
		);
	`,
		nullableUUID(feedback.TenantID),
		feedback.ID,
		feedback.UserID,
		feedback.UserName,
		"user",
		feedback.Body,
		feedback.ImagePath,
		feedback.ImageContentType,
		feedback.ImageSizeBytes,
	)
	if err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	return feedback, nil
}

func scanFeedback(scan func(...any) error) (*Feedback, error) {
	var f Feedback
	var tenantID, storeID *string
	var closedAt *time.Time
	err := scan(
		&f.ID, &tenantID, &storeID, &f.UserID, &f.UserName,
		&f.Kind, &f.Status, &f.Subject, &f.Body, &f.AdminNote,
		&f.CreatedAt, &f.UpdatedAt, &closedAt, &f.UserLastReadAt,
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
	if closedAt != nil {
		f.ClosedAt = closedAt
	}
	return &f, nil
}

func (r *PostgresRepository) GetByID(id string) (*Feedback, error) {
	row := r.pool.QueryRow(context.Background(), `
		select id::text, tenant_id::text, store_id::text, user_id::text, user_name,
		       kind, status, subject, body, admin_note, created_at, updated_at, closed_at, user_last_read_at
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

func (r *PostgresRepository) getByIDForViewer(id string, viewerUserID string) (*Feedback, error) {
	row := r.pool.QueryRow(context.Background(), `
		select user_feedback.id::text, user_feedback.tenant_id::text, user_feedback.store_id::text,
		       user_feedback.user_id::text, user_feedback.user_name, user_feedback.kind,
		       user_feedback.status, user_feedback.subject, user_feedback.body, user_feedback.admin_note,
		       user_feedback.created_at, user_feedback.updated_at, user_feedback.closed_at,
		       coalesce(feedback_read_states.last_read_at, user_feedback.created_at)
		from user_feedback
		left join feedback_read_states
		  on feedback_read_states.feedback_id = user_feedback.id
		 and feedback_read_states.user_id = $2::uuid
		where user_feedback.id = $1::uuid
		limit 1;
	`, id, viewerUserID)

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
	readAtExpr := "coalesce(user_feedback.user_last_read_at, user_feedback.created_at)"
	readJoin := ""
	query := `
		select user_feedback.id::text, user_feedback.tenant_id::text, user_feedback.store_id::text,
		       user_feedback.user_id::text, user_feedback.user_name, user_feedback.kind,
		       user_feedback.status, user_feedback.subject, user_feedback.body, user_feedback.admin_note,
		       user_feedback.created_at, user_feedback.updated_at, user_feedback.closed_at,
		       %s
		from user_feedback
		%s
		where 1=1
	`

	args := []interface{}{}
	argCount := 1

	if input.ViewerUserID != "" {
		readAtExpr = "coalesce(feedback_read_states.last_read_at, user_feedback.created_at)"
		readJoin = fmt.Sprintf(`
		left join feedback_read_states
		  on feedback_read_states.feedback_id = user_feedback.id
		 and feedback_read_states.user_id = $%d::uuid`, argCount)
		args = append(args, input.ViewerUserID)
		argCount++
	}

	query = fmt.Sprintf(query, readAtExpr, readJoin)

	if tenantID != "" {
		query += fmt.Sprintf(" and user_feedback.tenant_id = $%d::uuid", argCount)
		args = append(args, tenantID)
		argCount++
	}

	if input.Kind != "" {
		query += fmt.Sprintf(" and user_feedback.kind = $%d", argCount)
		args = append(args, input.Kind)
		argCount++
	}

	if input.Status != "" {
		query += fmt.Sprintf(" and user_feedback.status = $%d", argCount)
		args = append(args, input.Status)
		argCount++
	}

	if input.UserID != "" {
		query += fmt.Sprintf(" and user_feedback.user_id = $%d::uuid", argCount)
		args = append(args, input.UserID)
		argCount++
	}

	if input.Since != nil {
		if input.ViewerUserID != "" {
			query += fmt.Sprintf(" and greatest(user_feedback.updated_at, %s) > $%d", readAtExpr, argCount)
		} else {
			query += fmt.Sprintf(" and user_feedback.updated_at > $%d", argCount)
		}
		args = append(args, *input.Since)
		argCount++
	}

	query += " order by user_feedback.created_at desc;"

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

func (r *PostgresRepository) MarkRead(feedbackID string, userID string, readAt time.Time) (*Feedback, error) {
	_, err := r.pool.Exec(context.Background(), `
		insert into feedback_read_states (
			feedback_id, user_id, last_read_at
		) values (
			$1::uuid, $2::uuid,
			coalesce(
				(select max(created_at) from feedback_messages where feedback_id = $1::uuid),
				$3
			)
		)
		on conflict (feedback_id, user_id)
		do update set
			last_read_at = greatest(feedback_read_states.last_read_at, excluded.last_read_at),
			updated_at = now();
	`, feedbackID, userID, readAt)
	if err != nil {
		return nil, err
	}
	if _, err := r.GetByID(feedbackID); err != nil {
		return nil, err
	}

	return r.getByIDForViewer(feedbackID, userID)
}

func (r *PostgresRepository) Update(feedback *Feedback) error {
	ctx := context.Background()
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	_, err = tx.Exec(ctx, `
		update user_feedback
		set status = $2, admin_note = $3, closed_at = $4, updated_at = now()
		where id = $1::uuid;
	`,
		feedback.ID,
		feedback.Status,
		feedback.AdminNote,
		feedback.ClosedAt,
	)
	if err != nil {
		return err
	}

	if feedback.Status == StatusClosed && feedback.ClosedAt != nil {
		_, err = tx.Exec(ctx, `
			update feedback_messages
			set image_expires_at = $2
			where feedback_id = $1::uuid
			  and image_path <> '';
		`, feedback.ID, feedback.ClosedAt.Add(feedbackImageRetention))
	} else {
		_, err = tx.Exec(ctx, `
			update feedback_messages
			set image_expires_at = null
			where feedback_id = $1::uuid
			  and image_path <> '';
		`, feedback.ID)
	}
	if err != nil {
		return err
	}

	return tx.Commit(ctx)
}

func scanFeedbackMessage(scan func(...any) error) (*FeedbackMessage, error) {
	var message FeedbackMessage
	var tenantID *string
	var imageExpiresAt *time.Time
	err := scan(
		&message.ID,
		&tenantID,
		&message.FeedbackID,
		&message.AuthorUserID,
		&message.AuthorName,
		&message.AuthorRole,
		&message.Body,
		&message.ImagePath,
		&message.ImageContentType,
		&message.ImageSizeBytes,
		&imageExpiresAt,
		&message.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	if tenantID != nil {
		message.TenantID = *tenantID
	}
	if imageExpiresAt != nil {
		message.ImageExpiresAt = imageExpiresAt
	}
	return &message, nil
}

func (r *PostgresRepository) CreateMessage(message *FeedbackMessage) (*FeedbackMessage, error) {
	ctx := context.Background()
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	err = tx.QueryRow(ctx, `
		insert into feedback_messages (
			tenant_id, feedback_id, author_user_id, author_name, author_role, body,
			image_path, image_content_type, image_size_bytes
		) values (
			$1::uuid, $2::uuid, $3::uuid, $4, $5, $6, $7, $8, $9
		) returning id::text, created_at;
	`,
		nullableUUID(message.TenantID),
		message.FeedbackID,
		message.AuthorUserID,
		message.AuthorName,
		message.AuthorRole,
		message.Body,
		message.ImagePath,
		message.ImageContentType,
		message.ImageSizeBytes,
	).Scan(&message.ID, &message.CreatedAt)
	if err != nil {
		return nil, err
	}

	_, err = tx.Exec(ctx, `
		update user_feedback
		set updated_at = now()
		where id = $1::uuid;
	`, message.FeedbackID)
	if err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	return message, nil
}

func (r *PostgresRepository) ListMessages(feedbackID string, input ListMessagesInput) ([]FeedbackMessage, error) {
	query := `
		select id::text, tenant_id::text, feedback_id::text, author_user_id::text,
		       author_name, author_role, body, image_path, image_content_type,
		       image_size_bytes, image_expires_at, created_at
		from feedback_messages
		where feedback_id = $1::uuid
	`
	args := []interface{}{feedbackID}
	argCount := 2

	if input.After != nil {
		query += fmt.Sprintf(" and created_at > $%d", argCount)
		args = append(args, *input.After)
	}

	query += " order by created_at asc;"

	rows, err := r.pool.Query(context.Background(), query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	messages := make([]FeedbackMessage, 0)
	for rows.Next() {
		message, err := scanFeedbackMessage(rows.Scan)
		if err != nil {
			return nil, err
		}
		messages = append(messages, *message)
	}

	return messages, rows.Err()
}

func (r *PostgresRepository) PurgeExpiredAttachments(cutoff time.Time, limit int) ([]string, error) {
	rows, err := r.pool.Query(context.Background(), `
		with expired as (
			select id, image_path
			from feedback_messages
			where image_path <> ''
			  and image_expires_at is not null
			  and image_expires_at <= $1
			order by image_expires_at asc
			limit $2
		), updated as (
			update feedback_messages as messages
			set image_path = '', image_content_type = '', image_size_bytes = 0, image_expires_at = null
			from expired
			where messages.id = expired.id
			returning expired.image_path
		)
		select image_path
		from updated;
	`, cutoff, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	paths := make([]string, 0)
	for rows.Next() {
		var path string
		if err := rows.Scan(&path); err != nil {
			return nil, err
		}
		if strings.TrimSpace(path) != "" {
			paths = append(paths, path)
		}
	}

	return paths, rows.Err()
}
