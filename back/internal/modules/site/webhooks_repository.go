package site

import (
	"context"
	"errors"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PostgresWebhookSourceRepository implementa WebhookSourceRepository.
type PostgresWebhookSourceRepository struct {
	pool *pgxpool.Pool
}

// NewPostgresWebhookSourceRepository cria o repo.
func NewPostgresWebhookSourceRepository(pool *pgxpool.Pool) *PostgresWebhookSourceRepository {
	return &PostgresWebhookSourceRepository{pool: pool}
}

func (r *PostgresWebhookSourceRepository) List(ctx context.Context, accountID string) ([]WebhookSourceView, error) {
	const query = `
		select id, account_id, slug, name, entity_type, is_active, created_at, updated_at
		from site.webhook_sources
		where account_id = $1::uuid
		order by lower(name) asc
	`
	rows, err := r.pool.Query(ctx, query, accountID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]WebhookSourceView, 0)
	for rows.Next() {
		var v WebhookSourceView
		if err := rows.Scan(&v.ID, &v.AccountID, &v.Slug, &v.Name, &v.EntityType,
			&v.IsActive, &v.CreatedAt, &v.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

func (r *PostgresWebhookSourceRepository) Find(ctx context.Context, accountID, sourceID string) (WebhookSourceView, error) {
	const query = `
		select id, account_id, slug, name, entity_type, is_active, created_at, updated_at
		from site.webhook_sources
		where account_id = $1::uuid and id = $2::uuid
	`
	row := r.pool.QueryRow(ctx, query, accountID, sourceID)
	var v WebhookSourceView
	if err := row.Scan(&v.ID, &v.AccountID, &v.Slug, &v.Name, &v.EntityType,
		&v.IsActive, &v.CreatedAt, &v.UpdatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return WebhookSourceView{}, ErrSourceNotFound
		}
		return WebhookSourceView{}, err
	}
	return v, nil
}

// FindBySlug devolve a view + secret. Usado pelo handler de ingest para
// validar HMAC. Slug e unico por account, mas o ingest precisa achar pela
// combinacao accountSlug/sourceSlug — para isso o caminho do webhook embute
// o slug do account no path: POST /v1/webhooks/{accountSlug}/{sourceSlug}.
// Aqui passamos so o sourceSlug e descobrimos a account pelo handler.
func (r *PostgresWebhookSourceRepository) FindBySlug(ctx context.Context, slug string) (WebhookSourceView, string, error) {
	const query = `
		select id, account_id, slug, name, entity_type, is_active, created_at, updated_at, secret
		from site.webhook_sources
		where slug = $1 and is_active = true
		limit 1
	`
	row := r.pool.QueryRow(ctx, query, strings.ToLower(strings.TrimSpace(slug)))
	var v WebhookSourceView
	var secret string
	if err := row.Scan(&v.ID, &v.AccountID, &v.Slug, &v.Name, &v.EntityType,
		&v.IsActive, &v.CreatedAt, &v.UpdatedAt, &secret); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return WebhookSourceView{}, "", ErrSourceNotFound
		}
		return WebhookSourceView{}, "", err
	}
	return v, secret, nil
}

func (r *PostgresWebhookSourceRepository) Create(ctx context.Context, accountID string, input WebhookSourceCreateInput, secret string) (WebhookSourceView, error) {
	var id string
	err := r.pool.QueryRow(ctx, `
		insert into site.webhook_sources (account_id, slug, name, entity_type, secret, is_active, created_at, updated_at)
		values ($1::uuid, lower($2), $3, $4, $5, true, now(), now())
		returning id
	`, accountID, input.Slug, input.Name, input.EntityType, secret).Scan(&id)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return WebhookSourceView{}, ErrSourceSlugConflict
		}
		return WebhookSourceView{}, err
	}
	return r.Find(ctx, accountID, id)
}

func (r *PostgresWebhookSourceRepository) UpdateSecret(ctx context.Context, sourceID, secret string) error {
	tag, err := r.pool.Exec(ctx,
		`update site.webhook_sources set secret = $1, updated_at = now() where id = $2::uuid`,
		secret, sourceID,
	)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrSourceNotFound
	}
	return nil
}

func (r *PostgresWebhookSourceRepository) SoftDelete(ctx context.Context, accountID, sourceID string) error {
	tag, err := r.pool.Exec(ctx,
		`update site.webhook_sources set is_active = false, updated_at = now() where account_id = $1::uuid and id = $2::uuid`,
		accountID, sourceID,
	)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrSourceNotFound
	}
	return nil
}
