package socialpublishing

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/secretbox"
)

type Store struct {
	pool *pgxpool.Pool
}

func NewStore(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool}
}

type rowScanner interface {
	Scan(dest ...any) error
}

const postSelectColumns = `
	p.id::text, p.account_id::text, coalesce(p.connection_id::text, ''),
	p.caption, p.media_url, p.alt_text, p.status, p.scheduled_for, p.timezone,
	p.schedule_revision, p.version, p.source_type, coalesce(p.source_ref, ''),
	coalesce(p.external_creation_id, ''), coalesce(p.external_media_id, ''),
	coalesce(p.permalink, ''), coalesce(p.last_error_code, ''),
	coalesce(p.last_error_message, ''), p.publish_attempted_at,
	p.published_at, p.created_at, p.updated_at`

const postReturnColumns = `
	id::text, account_id::text, coalesce(connection_id::text, ''),
	caption, media_url, alt_text, status, scheduled_for, timezone,
	schedule_revision, version, source_type, coalesce(source_ref, ''),
	coalesce(external_creation_id, ''), coalesce(external_media_id, ''),
	coalesce(permalink, ''), coalesce(last_error_code, ''),
	coalesce(last_error_message, ''), publish_attempted_at,
	published_at, created_at, updated_at`

func scanPost(row rowScanner) (Post, error) {
	var post Post
	err := row.Scan(
		&post.ID,
		&post.AccountID,
		&post.ConnectionID,
		&post.Caption,
		&post.MediaURL,
		&post.AltText,
		&post.Status,
		&post.ScheduledFor,
		&post.Timezone,
		&post.ScheduleRevision,
		&post.Version,
		&post.SourceType,
		&post.SourceRef,
		&post.ExternalCreationID,
		&post.ExternalMediaID,
		&post.Permalink,
		&post.LastErrorCode,
		&post.LastErrorMessage,
		&post.PublishAttemptedAt,
		&post.PublishedAt,
		&post.CreatedAt,
		&post.UpdatedAt,
	)
	post.MediaType = "image"
	return post, err
}

func (s *Store) GetConnection(ctx context.Context, accountID string) (ConnectionRecord, error) {
	const query = `
		select id::text, account_id::text, provider, ig_user_id, username,
		       account_type, media_count, status, access_token_ciphertext,
		       token_last4, connected_at, updated_at, version
		from social_publishing.connections
		where account_id = $1::uuid
		order by (status = 'connected') desc, updated_at desc, id desc
		limit 1`
	var record ConnectionRecord
	var last4 string
	err := s.pool.QueryRow(ctx, query, accountID).Scan(
		&record.ID,
		&record.AccountID,
		&record.Provider,
		&record.IGUserID,
		&record.Username,
		&record.AccountType,
		&record.MediaCount,
		&record.Status,
		&record.AccessTokenCiphertext,
		&last4,
		&record.ConnectedAt,
		&record.UpdatedAt,
		&record.Version,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return ConnectionRecord{}, ErrNotConnected
	}
	if err != nil {
		return ConnectionRecord{}, fmt.Errorf("social publishing: obter conexao: %w", err)
	}
	record.Secret = secretbox.Status{Set: record.AccessTokenCiphertext != "", Last4: last4}
	return record, nil
}

func (s *Store) SaveConnection(
	ctx context.Context,
	accountID, userID string,
	profile InstagramProfile,
	ciphertext, tokenLast4 string,
) (Connection, error) {
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return Connection{}, fmt.Errorf("social publishing: iniciar conexao: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// Uma conexao identifica um destino externo imutavel. Reconectar revoga a
	// anterior e cria outro ID; posts antigos nunca mudam silenciosamente de IG.
	const revoke = `
		update social_publishing.connections
		set status = 'revoked',
		    access_token_ciphertext = '',
		    token_last4 = '',
		    updated_by = $2::uuid,
		    updated_at = now(),
		    version = version + 1
		where account_id = $1::uuid
		  and status = 'connected'`
	if _, err := tx.Exec(ctx, revoke, accountID, userID); err != nil {
		return Connection{}, fmt.Errorf("social publishing: revogar conexao anterior: %w", err)
	}
	const query = `
		insert into social_publishing.connections (
			account_id, provider, ig_user_id, username, account_type, media_count,
			status, access_token_ciphertext, token_last4, connected_at,
			created_by, updated_by
		)
		values (
			$1::uuid, 'instagram', $2, $3, $4, $5, 'connected', $6, $7, now(),
			$8::uuid, $8::uuid
		)
		returning id::text, provider, ig_user_id, username, account_type, media_count,
		          status, connected_at, updated_at, version`
	var connection Connection
	err = tx.QueryRow(
		ctx,
		query,
		accountID,
		profile.UserID,
		profile.Username,
		profile.AccountType,
		profile.MediaCount,
		ciphertext,
		tokenLast4,
		userID,
	).Scan(
		&connection.ID,
		&connection.Provider,
		&connection.IGUserID,
		&connection.Username,
		&connection.AccountType,
		&connection.MediaCount,
		&connection.Status,
		&connection.ConnectedAt,
		&connection.UpdatedAt,
		&connection.Version,
	)
	if err != nil {
		return Connection{}, fmt.Errorf("social publishing: salvar conexao: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return Connection{}, fmt.Errorf("social publishing: confirmar conexao: %w", err)
	}
	connection.Secret = secretbox.Status{Set: true, Last4: tokenLast4}
	return connection, nil
}

func (s *Store) DeleteConnection(ctx context.Context, accountID string) error {
	const query = `
		update social_publishing.connections
		set status = 'revoked',
		    access_token_ciphertext = '',
		    token_last4 = '',
		    updated_at = now(),
		    version = version + 1
		where account_id = $1::uuid
		  and status = 'connected'`
	if _, err := s.pool.Exec(ctx, query, accountID); err != nil {
		return fmt.Errorf("social publishing: desconectar: %w", err)
	}
	return nil
}

func (s *Store) CreatePost(
	ctx context.Context,
	command createPostCommand,
) (CreatePostResult, error) {
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return CreatePostResult{}, fmt.Errorf("social publishing: iniciar criacao: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	status := command.Input.Status
	revision := 0
	if status == PostStatusScheduled {
		revision = 1
	}
	const insert = `
		insert into social_publishing.posts (
			account_id, connection_id, caption, media_url, alt_text, status,
			scheduled_for, timezone, schedule_revision, version, source_type,
			source_ref, created_by, updated_by
		)
		values (
			$1::uuid, nullif($2, '')::uuid, $3, $4, $5, $6,
			$7, $8, $9, 1, $10, nullif($11, ''), $12::uuid, $12::uuid
		)
		on conflict (account_id, source_type, source_ref)
			where source_ref is not null
		do nothing
		returning ` + postReturnColumns
	post, scanErr := scanPost(tx.QueryRow(
		ctx,
		insert,
		command.AccountID,
		command.ConnectionID,
		command.Input.Caption,
		command.Input.MediaURL,
		command.Input.AltText,
		status,
		command.Input.ScheduledFor,
		command.Input.Timezone,
		revision,
		command.Input.SourceType,
		command.Input.SourceRef,
		command.UserID,
	))
	created := scanErr == nil
	if errors.Is(scanErr, pgx.ErrNoRows) && command.Input.SourceRef != "" {
		const existing = `
			select ` + postSelectColumns + `
			from social_publishing.posts p
			where p.account_id = $1::uuid
			  and p.source_type = $2
			  and p.source_ref = $3`
		post, scanErr = scanPost(tx.QueryRow(
			ctx,
			existing,
			command.AccountID,
			command.Input.SourceType,
			command.Input.SourceRef,
		))
	}
	if scanErr != nil {
		return CreatePostResult{}, fmt.Errorf("social publishing: criar post: %w", scanErr)
	}
	if created && status == PostStatusScheduled {
		if err := enqueuePublishTx(ctx, tx, post); err != nil {
			return CreatePostResult{}, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return CreatePostResult{}, fmt.Errorf("social publishing: confirmar criacao: %w", err)
	}
	return CreatePostResult{Post: post, Created: created}, nil
}

func (s *Store) GetPost(ctx context.Context, accountID, postID string) (Post, error) {
	const query = `
		select ` + postSelectColumns + `
		from social_publishing.posts p
		where p.account_id = $1::uuid and p.id = $2::uuid`
	post, err := scanPost(s.pool.QueryRow(ctx, query, accountID, postID))
	if errors.Is(err, pgx.ErrNoRows) {
		return Post{}, ErrNotFound
	}
	if err != nil {
		return Post{}, fmt.Errorf("social publishing: obter post: %w", err)
	}
	return post, nil
}

func (s *Store) ListPosts(
	ctx context.Context,
	accountID string,
	filter ListPostsFilter,
) ([]Post, error) {
	const query = `
		select ` + postSelectColumns + `
		from social_publishing.posts p
		where p.account_id = $1::uuid
		  and ($2 = '' or p.status = $2)
		order by p.created_at desc, p.id desc
		limit $3 offset $4`
	rows, err := s.pool.Query(ctx, query, accountID, filter.Status, filter.Limit, filter.Offset)
	if err != nil {
		return nil, fmt.Errorf("social publishing: listar posts: %w", err)
	}
	defer rows.Close()
	posts := make([]Post, 0)
	for rows.Next() {
		post, scanErr := scanPost(rows)
		if scanErr != nil {
			return nil, fmt.Errorf("social publishing: ler post: %w", scanErr)
		}
		posts = append(posts, post)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("social publishing: iterar posts: %w", err)
	}
	return posts, nil
}

func (s *Store) UpdatePost(ctx context.Context, command updatePostCommand) (Post, error) {
	const query = `
		update social_publishing.posts
		set caption = $4,
		    media_url = $5,
		    alt_text = $6,
		    timezone = $7,
		    status = 'draft',
		    scheduled_for = null,
		    connection_id = null,
		    schedule_revision = schedule_revision + 1,
		    external_creation_id = '',
		    publish_attempted_at = null,
		    last_error_code = '',
		    last_error_message = '',
		    updated_by = $3::uuid,
		    updated_at = now(),
		    version = version + 1
		where account_id = $1::uuid
		  and id = $2::uuid
		  and version = $8
		  and status in ('draft', 'scheduled', 'failed', 'cancelled')
		returning ` + postReturnColumns
	post, err := scanPost(s.pool.QueryRow(
		ctx,
		query,
		command.AccountID,
		command.Post.ID,
		command.UserID,
		command.Post.Caption,
		command.Post.MediaURL,
		command.Post.AltText,
		command.Post.Timezone,
		command.Version,
	))
	if errors.Is(err, pgx.ErrNoRows) {
		return Post{}, ErrConflict
	}
	if err != nil {
		return Post{}, fmt.Errorf("social publishing: atualizar post: %w", err)
	}
	return post, nil
}
