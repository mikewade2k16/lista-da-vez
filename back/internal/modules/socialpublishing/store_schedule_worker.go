package socialpublishing

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

const schedulePostUpdateQuery = `
		update social_publishing.posts p
		set connection_id = $4::uuid,
		    status = 'scheduled',
		    scheduled_for = $5,
		    timezone = $6,
		    schedule_revision = p.schedule_revision + 1,
		    external_creation_id = '',
		    last_error_code = '',
		    last_error_message = '',
		    updated_by = $3::uuid,
		    updated_at = now(),
		    version = p.version + 1
		where p.account_id = $1::uuid
		  and p.id = $2::uuid
		  and p.version = $7
		  and p.publish_attempted_at is null
		  and p.status in ('draft', 'scheduled', 'failed', 'cancelled')
		  and (p.connection_id is null or p.connection_id = $4::uuid)
		  and exists (
			select 1
			from social_publishing.connections target_connection
			join social_publishing.connections active_connection
			  on active_connection.account_id = target_connection.account_id
			 and active_connection.ig_user_id = target_connection.ig_user_id
			 and active_connection.status = 'connected'
			 and active_connection.access_token_ciphertext <> ''
			where target_connection.account_id = p.account_id
			  and target_connection.id = $4::uuid
		  )
		returning ` + postReturnColumns

const listPublishedPostIDsQuery = `
		select id::text
		from social_publishing.posts
		where account_id = $1::uuid
		  and status = 'published'
		  and external_media_id <> ''
		order by published_at desc nulls last, id`

const protectPublishOutcomeQuery = `
		update social_publishing.posts
		set status = 'failed',
		    last_error_code = 'publish_outcome_unknown',
		    last_error_message = 'Execucao anterior interrompida; confira o Instagram antes de tentar novamente.',
		    updated_at = case
				when status = 'failed' and last_error_code = 'publish_outcome_unknown'
					then updated_at
				else now()
		    end,
		    version = version + case
				when status = 'failed' and last_error_code = 'publish_outcome_unknown'
					then 0
				else 1
		    end
		where account_id = $1::uuid
		  and id = $2::uuid
		  and schedule_revision = $3
		  and publish_attempted_at is not null
		  and external_media_id = ''
		  and status <> 'published'
		returning true`

const markPublishFailedQuery = `
		update social_publishing.posts
		set status = 'failed',
		    last_error_code = case
				when publish_attempted_at is not null and external_media_id = ''
					then 'publish_outcome_unknown'
				else $4
		    end,
		    last_error_message = case
				when publish_attempted_at is not null and external_media_id = ''
					then 'Confira o Instagram antes de tentar novamente.'
				else $5
		    end,
		    updated_at = now(),
		    version = version + 1
		where account_id = $1::uuid
		  and id = $2::uuid
		  and schedule_revision = $3
		  and status <> 'published'
		  and status <> 'cancelled'`

func (s *Store) SchedulePost(
	ctx context.Context,
	command schedulePostCommand,
) (Post, error) {
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return Post{}, fmt.Errorf("social publishing: iniciar agendamento: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	post, scanErr := scanPost(tx.QueryRow(
		ctx,
		schedulePostUpdateQuery,
		command.AccountID,
		command.PostID,
		command.UserID,
		command.ConnectionID,
		command.ScheduledFor,
		command.Timezone,
		command.ExpectedVersion,
	))
	if errors.Is(scanErr, pgx.ErrNoRows) {
		return Post{}, ErrConflict
	}
	if scanErr != nil {
		return Post{}, fmt.Errorf("social publishing: agendar post: %w", scanErr)
	}
	if err := enqueuePublishTx(ctx, tx, post); err != nil {
		return Post{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Post{}, fmt.Errorf("social publishing: confirmar agendamento: %w", err)
	}
	return post, nil
}

func enqueuePublishTx(ctx context.Context, tx pgx.Tx, post Post) error {
	payload, err := json.Marshal(publishJobPayload{
		PostID:   post.ID,
		Revision: post.ScheduleRevision,
	})
	if err != nil {
		return fmt.Errorf("social publishing: serializar publicacao: %w", err)
	}
	const query = `
		insert into social_publishing.outbox (
			account_id, ordering_key, idempotency_key, kind, payload,
			max_attempts, run_after
		)
		values (
			$1::uuid, $2, $3, $4, $5::jsonb, 5, $6
		)
		on conflict (account_id, idempotency_key) do nothing`
	idempotencyKey := publishJobKey(post.ID, post.ScheduleRevision)
	if _, err := tx.Exec(
		ctx,
		query,
		post.AccountID,
		idempotencyKey,
		idempotencyKey,
		PublishJobKind,
		payload,
		post.ScheduledFor,
	); err != nil {
		return fmt.Errorf("social publishing: enfileirar publicacao: %w", err)
	}
	return nil
}

func publishJobKey(postID string, revision int) string {
	return fmt.Sprintf("publish:%s:v%d", postID, revision)
}

func (s *Store) CancelPost(
	ctx context.Context,
	accountID, postID, userID string,
	version int,
) (Post, error) {
	const query = `
		update social_publishing.posts
		set status = 'cancelled',
		    schedule_revision = schedule_revision + 1,
		    updated_by = $3::uuid,
		    updated_at = now(),
		    version = version + 1
		where account_id = $1::uuid
		  and id = $2::uuid
		  and version = $4
		  and status in ('scheduled', 'failed')
		returning ` + postReturnColumns
	post, err := scanPost(s.pool.QueryRow(ctx, query, accountID, postID, userID, version))
	if errors.Is(err, pgx.ErrNoRows) {
		return Post{}, ErrConflict
	}
	if err != nil {
		return Post{}, fmt.Errorf("social publishing: cancelar post: %w", err)
	}
	return post, nil
}

func (s *Store) ListPublishedPostIDs(
	ctx context.Context,
	accountID string,
) ([]string, error) {
	rows, err := s.pool.Query(ctx, listPublishedPostIDsQuery, accountID)
	if err != nil {
		return nil, fmt.Errorf("social publishing: listar posts publicados: %w", err)
	}
	defer rows.Close()
	ids := make([]string, 0)
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("social publishing: ler post publicado: %w", err)
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("social publishing: iterar posts publicados: %w", err)
	}
	return ids, nil
}

func (s *Store) PreparePublish(
	ctx context.Context,
	accountID, postID string,
	revision int,
) (publishTarget, bool, error) {
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return publishTarget{}, false, fmt.Errorf("social publishing: iniciar publish: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	const selectTarget = `
		select p.status, p.caption, p.media_url, p.alt_text,
		       coalesce(p.external_creation_id, ''),
		       coalesce(p.external_media_id, ''), p.publish_attempted_at,
		       target_connection.ig_user_id,
		       coalesce(active_connection.access_token_ciphertext, ''),
		       coalesce(active_connection.status, '')
		from social_publishing.posts p
		join social_publishing.connections target_connection
		  on target_connection.account_id = p.account_id
		 and target_connection.id = p.connection_id
		left join social_publishing.connections active_connection
		  on active_connection.account_id = p.account_id
		 and active_connection.ig_user_id = target_connection.ig_user_id
		 and active_connection.status = 'connected'
		where p.account_id = $1::uuid
		  and p.id = $2::uuid
		  and p.schedule_revision = $3
		for update of p`
	var (
		status           PostStatus
		connectionStatus string
		target           = publishTarget{PostID: postID, AccountID: accountID, Revision: revision}
	)
	err = tx.QueryRow(ctx, selectTarget, accountID, postID, revision).Scan(
		&status,
		&target.Caption,
		&target.MediaURL,
		&target.AltText,
		&target.ExternalCreationID,
		&target.ExternalMediaID,
		&target.PublishAttemptedAt,
		&target.IGUserID,
		&target.TokenCiphertext,
		&connectionStatus,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return publishTarget{}, false, nil
	}
	if err != nil {
		return publishTarget{}, false, fmt.Errorf("social publishing: carregar publish: %w", err)
	}
	if status == PostStatusPublished || status == PostStatusCancelled || status == PostStatusDraft {
		return publishTarget{}, false, nil
	}
	if status != PostStatusScheduled && status != PostStatusFailed && status != PostStatusPublishing {
		return publishTarget{}, false, nil
	}
	if connectionStatus != "connected" || target.TokenCiphertext == "" {
		return publishTarget{}, false, ErrNotConnected
	}
	const markPublishing = `
		update social_publishing.posts
		set status = 'publishing', updated_at = now(), version = version + 1
		where account_id = $1::uuid and id = $2::uuid and schedule_revision = $3`
	if _, err := tx.Exec(ctx, markPublishing, accountID, postID, revision); err != nil {
		return publishTarget{}, false, fmt.Errorf("social publishing: marcar publishing: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return publishTarget{}, false, fmt.Errorf("social publishing: confirmar publishing: %w", err)
	}
	return target, true, nil
}

func (s *Store) ProtectPublishOutcome(
	ctx context.Context,
	accountID, postID string,
	revision int,
) (bool, error) {
	var protected bool
	err := s.pool.QueryRow(ctx, protectPublishOutcomeQuery, accountID, postID, revision).Scan(&protected)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("social publishing: proteger resultado ambiguo: %w", err)
	}
	return protected, nil
}

func (s *Store) SaveCreationID(
	ctx context.Context,
	accountID, postID string,
	revision int,
	creationID string,
) error {
	const query = `
		update social_publishing.posts
		set external_creation_id = case
				when external_creation_id = '' then $4
				else external_creation_id
		    end,
		    updated_at = now()
		where account_id = $1::uuid
		  and id = $2::uuid
		  and schedule_revision = $3
		  and status = 'publishing'`
	tag, err := s.pool.Exec(ctx, query, accountID, postID, revision, creationID)
	if err != nil {
		return fmt.Errorf("social publishing: salvar container: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrConflict
	}
	return nil
}

func (s *Store) MarkPublishAttempted(
	ctx context.Context,
	accountID, postID string,
	revision int,
) (bool, error) {
	const query = `
		update social_publishing.posts
		set publish_attempted_at = now(), updated_at = now()
		where account_id = $1::uuid
		  and id = $2::uuid
		  and schedule_revision = $3
		  and status = 'publishing'
		  and publish_attempted_at is null`
	tag, err := s.pool.Exec(ctx, query, accountID, postID, revision)
	if err != nil {
		return false, fmt.Errorf("social publishing: marcar tentativa externa: %w", err)
	}
	return tag.RowsAffected() == 1, nil
}

func (s *Store) MarkPublished(
	ctx context.Context,
	accountID, postID string,
	revision int,
	mediaID string,
	publishedAt time.Time,
) error {
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("social publishing: iniciar published: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	const query = `
		update social_publishing.posts
		set status = 'published',
		    external_media_id = $4,
		    published_at = $5,
		    last_error_code = '',
		    last_error_message = '',
		    updated_at = now(),
		    version = version + 1
		where account_id = $1::uuid
		  and id = $2::uuid
		  and schedule_revision = $3
		  and status = 'publishing'`
	tag, err := tx.Exec(ctx, query, accountID, postID, revision, mediaID, publishedAt)
	if err != nil {
		return fmt.Errorf("social publishing: marcar published: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrConflict
	}
	payload, err := json.Marshal(analyticsJobPayload{PostID: postID})
	if err != nil {
		return fmt.Errorf("social publishing: serializar analytics: %w", err)
	}
	stages := []struct {
		key   string
		delay time.Duration
	}{
		{key: "5m", delay: 5 * time.Minute},
		{key: "1h", delay: time.Hour},
		{key: "6h", delay: 6 * time.Hour},
		{key: "24h", delay: 24 * time.Hour},
	}
	const enqueue = `
		insert into social_publishing.analytics_outbox (
			account_id, ordering_key, idempotency_key, kind, payload,
			max_attempts, run_after
		)
		values ($1::uuid, $2, $3, $4, $5::jsonb, 4, $6)
		on conflict (account_id, idempotency_key) do nothing`
	for _, stage := range stages {
		if _, err := tx.Exec(
			ctx,
			enqueue,
			accountID,
			"analytics:"+postID+":"+stage.key,
			"analytics:"+postID+":"+stage.key,
			AnalyticsJobKind,
			payload,
			publishedAt.Add(stage.delay),
		); err != nil {
			return fmt.Errorf("social publishing: enfileirar analytics %s: %w", stage.key, err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("social publishing: confirmar published: %w", err)
	}
	return nil
}

func (s *Store) MarkPublishFailed(
	ctx context.Context,
	accountID, postID string,
	revision int,
	code, message string,
) error {
	if _, err := s.pool.Exec(
		ctx,
		markPublishFailedQuery,
		accountID,
		postID,
		revision,
		code,
		message,
	); err != nil {
		return fmt.Errorf("social publishing: marcar falha: %w", err)
	}
	return nil
}
