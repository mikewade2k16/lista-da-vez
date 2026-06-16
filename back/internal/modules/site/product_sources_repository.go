package site

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PostgresProductSourceRepository implementa ProductSourceRepository contra
// site.product_sources + site.products.
type PostgresProductSourceRepository struct {
	pool *pgxpool.Pool
}

// NewPostgresProductSourceRepository cria o repo.
func NewPostgresProductSourceRepository(pool *pgxpool.Pool) *PostgresProductSourceRepository {
	return &PostgresProductSourceRepository{pool: pool}
}

// ListByAccount retorna as fontes externas de produtos de uma account.
func (r *PostgresProductSourceRepository) ListByAccount(ctx context.Context, accountID string) ([]ProductSource, error) {
	const query = `
		select id, account_id, type, base_url, enabled
		from site.product_sources
		where account_id = $1::uuid
		order by created_at asc
	`
	rows, err := r.pool.Query(ctx, query, accountID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]ProductSource, 0)
	for rows.Next() {
		var s ProductSource
		if err := rows.Scan(&s.ID, &s.AccountID, &s.Type, &s.BaseURL, &s.Enabled); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// GetAccountSource retorna a fonte external_api da account (a 1a por created_at).
// Sem linha => ErrNoProductSource.
func (r *PostgresProductSourceRepository) GetAccountSource(ctx context.Context, accountID string) (ProductSource, error) {
	const query = `
		select id, account_id, type, base_url, enabled
		from site.product_sources
		where account_id = $1::uuid and type = $2
		order by created_at asc
		limit 1
	`
	var s ProductSource
	err := r.pool.QueryRow(ctx, query, accountID, productSourceType).
		Scan(&s.ID, &s.AccountID, &s.Type, &s.BaseURL, &s.Enabled)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ProductSource{}, ErrNoProductSource
		}
		return ProductSource{}, err
	}
	return s, nil
}

// SetAccountSourceBaseURL atualiza o base_url da fonte external_api da account.
// Sem linha afetada => ErrNoProductSource (nada para atualizar).
func (r *PostgresProductSourceRepository) SetAccountSourceBaseURL(ctx context.Context, accountID, baseURL string) error {
	const query = `
		update site.product_sources
		set base_url = $3, updated_at = now()
		where account_id = $1::uuid and type = $2
	`
	tag, err := r.pool.Exec(ctx, query, accountID, productSourceType, baseURL)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNoProductSource
	}
	return nil
}

// UpsertProducts grava os itens mapeados em site.products de uma vez (sem N+1),
// usando arrays + unnest e ON CONFLICT na chave (account_id, source, external_id).
// Retorna a contagem de inseridos/atualizados/ignorados (deleted + ja inativo).
func (r *PostgresProductSourceRepository) UpsertProducts(ctx context.Context, accountID string, items []ProductUpsertItem) (ProductSyncResult, error) {
	result := ProductSyncResult{}
	if len(items) == 0 {
		return result, nil
	}

	cols := buildUpsertColumns(items)

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return ProductSyncResult{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	rows, err := tx.Query(ctx, upsertProductsSQL, upsertArgs(accountID, cols)...)
	if err != nil {
		return ProductSyncResult{}, err
	}
	defer rows.Close()

	for rows.Next() {
		var inserted bool
		if err := rows.Scan(&inserted); err != nil {
			return ProductSyncResult{}, err
		}
		if inserted {
			result.Inserted++
		} else {
			result.Updated++
		}
	}
	if err := rows.Err(); err != nil {
		return ProductSyncResult{}, err
	}
	rows.Close()

	if err := tx.Commit(ctx); err != nil {
		return ProductSyncResult{}, err
	}

	result.Skipped = len(items) - result.Inserted - result.Updated
	if result.Skipped < 0 {
		result.Skipped = 0
	}
	return result, nil
}

// upsertColumns agrega os campos dos itens em slices paralelas para o unnest.
type upsertColumns struct {
	externalIDs []string
	sources     []string
	names       []string
	codes       []string
	images      []string
	categories  []string // JSON
	campaigns   []string // JSON
	prices      []float64
	fatores     []float64
	stocks      []int32
	statuses    []string
	isActive    []bool
}

func buildUpsertColumns(items []ProductUpsertItem) upsertColumns {
	c := upsertColumns{
		externalIDs: make([]string, 0, len(items)),
		sources:     make([]string, 0, len(items)),
		names:       make([]string, 0, len(items)),
		codes:       make([]string, 0, len(items)),
		images:      make([]string, 0, len(items)),
		categories:  make([]string, 0, len(items)),
		campaigns:   make([]string, 0, len(items)),
		prices:      make([]float64, 0, len(items)),
		fatores:     make([]float64, 0, len(items)),
		stocks:      make([]int32, 0, len(items)),
		statuses:    make([]string, 0, len(items)),
		isActive:    make([]bool, 0, len(items)),
	}
	for i := range items {
		it := items[i]
		status := it.Status
		active := !it.Deleted
		if it.Deleted {
			status = string(ProductStatusInactive)
		}
		catJSON, _ := json.Marshal(orEmptySlice(it.Categories))
		campJSON, _ := json.Marshal(orEmptySlice(it.Campaigns))

		c.externalIDs = append(c.externalIDs, it.ExternalID)
		c.sources = append(c.sources, it.Source)
		c.names = append(c.names, it.Name)
		c.codes = append(c.codes, it.Code)
		c.images = append(c.images, it.Image)
		c.categories = append(c.categories, string(catJSON))
		c.campaigns = append(c.campaigns, string(campJSON))
		c.prices = append(c.prices, it.Price)
		c.fatores = append(c.fatores, it.Fator)
		c.stocks = append(c.stocks, int32(it.Stock))
		c.statuses = append(c.statuses, status)
		c.isActive = append(c.isActive, active)
	}
	return c
}

func upsertArgs(accountID string, c upsertColumns) []any {
	return []any{
		accountID,
		c.externalIDs, c.sources, c.names, c.codes, c.images,
		c.categories, c.campaigns, c.prices, c.fatores, c.stocks,
		c.statuses, c.isActive,
	}
}

func orEmptySlice(in []string) []string {
	if in == nil {
		return []string{}
	}
	return in
}

// upsertProductsSQL faz o insert em lote via unnest e devolve `inserted` (true
// quando criou, false quando atualizou) por linha — `xmax = 0` no RETURNING
// distingue insert de update no ON CONFLICT.
const upsertProductsSQL = `
	insert into site.products (
	    account_id, source, external_id, source_label,
	    name, code, image, categories, campaigns,
	    price, fator, stock, status, is_active,
	    created_at, updated_at
	)
	select
	    $1::uuid, t.source, t.external_id, '',
	    t.name, t.code, t.image, t.categories::jsonb, t.campaigns::jsonb,
	    t.price, t.fator, t.stock, t.status, t.is_active,
	    now(), now()
	from unnest(
	    $2::text[], $3::text[], $4::text[], $5::text[], $6::text[],
	    $7::text[], $8::text[], $9::numeric[], $10::numeric[], $11::int[],
	    $12::text[], $13::bool[]
	) as t(
	    external_id, source, name, code, image,
	    categories, campaigns, price, fator, stock,
	    status, is_active
	)
	on conflict (account_id, source, external_id)
	    where source <> '' and external_id <> ''
	do update set
	    name = excluded.name,
	    code = excluded.code,
	    image = excluded.image,
	    categories = excluded.categories,
	    campaigns = excluded.campaigns,
	    price = excluded.price,
	    fator = excluded.fator,
	    stock = excluded.stock,
	    status = excluded.status,
	    is_active = excluded.is_active,
	    updated_at = now()
	returning (xmax = 0) as inserted
`
