package site

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PostgresProductRepository implementa ProductRepository contra site.products.
type PostgresProductRepository struct {
	pool *pgxpool.Pool
}

// NewPostgresProductRepository cria o repo.
func NewPostgresProductRepository(pool *pgxpool.Pool) *PostgresProductRepository {
	return &PostgresProductRepository{pool: pool}
}

func (r *PostgresProductRepository) List(ctx context.Context, filter ProductListFilter) ([]ProductView, int, error) {
	args := []any{filter.AccountID}
	conds := []string{"p.account_id = $1::uuid", "p.is_active = true"}
	n := 2

	if filter.Q != "" {
		pattern := "%" + strings.ToLower(strings.TrimSpace(filter.Q)) + "%"
		conds = append(conds, fmt.Sprintf(
			"(lower(p.name) like $%d or lower(p.code) like $%d or lower(p.description) like $%d)",
			n, n, n,
		))
		args = append(args, pattern)
		n++
	}
	if filter.Status != "" {
		conds = append(conds, fmt.Sprintf("p.status = $%d", n))
		args = append(args, filter.Status)
		n++
	}
	if filter.Category != "" {
		conds = append(conds, fmt.Sprintf("p.categories @> to_jsonb(array[$%d::text])", n))
		args = append(args, filter.Category)
		n++
	}
	if filter.Campaign != "" {
		conds = append(conds, fmt.Sprintf("p.campaigns @> to_jsonb(array[$%d::text])", n))
		args = append(args, filter.Campaign)
		n++
	}

	where := strings.Join(conds, " and ")

	var total int
	if err := r.pool.QueryRow(ctx,
		fmt.Sprintf("select count(*) from site.products p where %s", where),
		args...,
	).Scan(&total); err != nil {
		return nil, 0, err
	}

	page := filter.Page
	if page < 1 {
		page = 1
	}
	perPage := filter.PerPage
	if perPage < 1 {
		perPage = 50
	}
	// Cap alto: o admin de produtos carrega o catalogo inteiro de uma vez (filtros
	// e dropdowns sao client-side e precisam ver tudo). Imagens sao locais (cache),
	// entao listar tudo nao martela a origem.
	if perPage > 5000 {
		perPage = 5000
	}

	args = append(args, perPage, (page-1)*perPage)
	dataSQL := fmt.Sprintf(`
		select p.id, p.account_id, p.source_id, p.source_label,
		       p.name, p.code, p.description, p.image,
		       p.categories::text, p.campaigns::text,
		       p.price, p.fator, p.tipo, p.stock, p.status,
		       (erp.erp_sku is not null) as erp_synced, erp.erp_name, erp.erp_description,
		       p.created_at, p.updated_at
		from site.products p
		left join lateral (
		    select l.erp_sku, l.erp_name, l.erp_description
		    from site.product_erp_links l
		    where l.product_id = p.id
		    order by l.erp_sku
		    limit 1
		) erp on true
		where %s
		order by p.created_at desc, p.id desc
		limit $%d offset $%d
	`, where, n, n+1)

	rows, err := r.pool.Query(ctx, dataSQL, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	out := make([]ProductView, 0)
	for rows.Next() {
		v, err := scanProduct(rows)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, v)
	}
	return out, total, rows.Err()
}

func (r *PostgresProductRepository) Find(ctx context.Context, accountID, productID string) (ProductView, error) {
	const query = `
		select p.id, p.account_id, p.source_id, p.source_label,
		       p.name, p.code, p.description, p.image,
		       p.categories::text, p.campaigns::text,
		       p.price, p.fator, p.tipo, p.stock, p.status,
		       (erp.erp_sku is not null) as erp_synced, erp.erp_name, erp.erp_description,
		       p.created_at, p.updated_at
		from site.products p
		left join lateral (
		    select l.erp_sku, l.erp_name, l.erp_description
		    from site.product_erp_links l
		    where l.product_id = p.id
		    order by l.erp_sku
		    limit 1
		) erp on true
		where p.account_id = $1::uuid and p.id = $2::uuid and p.is_active = true
	`
	row := r.pool.QueryRow(ctx, query, accountID, productID)
	v, err := scanProduct(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ProductView{}, ErrProductNotFound
		}
		return ProductView{}, err
	}
	return v, nil
}

func (r *PostgresProductRepository) Create(ctx context.Context, accountID string, input ProductCreateInput) (ProductView, error) {
	categoriesJSON, _ := json.Marshal(input.Categories)
	campaignsJSON, _ := json.Marshal(input.Campaigns)
	var id string
	err := r.pool.QueryRow(ctx, `
		insert into site.products (
		    account_id, source_label, name, code, description, image,
		    categories, campaigns, price, fator, tipo, stock,
		    status, is_active, created_at, updated_at
		) values (
		    $1::uuid, '', $2, $3, $4, $5,
		    $6::jsonb, $7::jsonb, $8, $9, $10, $11,
		    'active', true, now(), now()
		)
		returning id
	`, accountID, input.Name, input.Code, input.Description, input.Image,
		string(categoriesJSON), string(campaignsJSON),
		input.Price, input.Fator, input.Tipo, input.Stock).Scan(&id)
	if err != nil {
		return ProductView{}, err
	}
	return r.Find(ctx, accountID, id)
}

// CreateFromErp cria um produto a partir de um item do ERP: name/description do
// ERP, code = sku, source = 'erp', sem external_id, status active e ativo.
func (r *PostgresProductRepository) CreateFromErp(ctx context.Context, accountID string, item ErpUnmatchedItem) (ProductView, error) {
	var id string
	err := r.pool.QueryRow(ctx, `
		insert into site.products (
		    account_id, source_label, source, name, code, description, image,
		    categories, campaigns, price, fator, tipo, stock,
		    status, is_active, created_at, updated_at
		) values (
		    $1::uuid, '', 'erp', $2, $3, $4, '',
		    '[]'::jsonb, '[]'::jsonb, 0, 1, '', 0,
		    'active', true, now(), now()
		)
		returning id
	`, accountID, item.Name, item.Sku, item.Description).Scan(&id)
	if err != nil {
		return ProductView{}, err
	}
	return r.Find(ctx, accountID, id)
}

func (r *PostgresProductRepository) CreateFromWebhook(ctx context.Context, accountID, sourceID, sourceLabel string, fields map[string]any, raw string) (ProductView, error) {
	getStr := func(key string) string {
		if v, ok := fields[key]; ok {
			return fmt.Sprintf("%v", v)
		}
		return ""
	}
	getNum := func(key string) float64 {
		if v, ok := fields[key]; ok {
			if f, ok := v.(float64); ok {
				return f
			}
		}
		return 0
	}
	getInt := func(key string) int {
		if v, ok := fields[key]; ok {
			if f, ok := v.(float64); ok {
				return int(f)
			}
		}
		return 0
	}
	getList := func(key string) []string {
		if v, ok := fields[key]; ok {
			if arr, ok := v.([]any); ok {
				out := make([]string, 0, len(arr))
				for _, item := range arr {
					out = append(out, fmt.Sprintf("%v", item))
				}
				return out
			}
		}
		return nil
	}

	categoriesJSON, _ := json.Marshal(getList("categories"))
	campaignsJSON, _ := json.Marshal(getList("campaigns"))

	var id string
	err := r.pool.QueryRow(ctx, `
		insert into site.products (
		    account_id, source_id, source_label, name, code, description, image,
		    categories, campaigns, price, fator, tipo, stock,
		    payload_raw, status, is_active, created_at, updated_at
		) values (
		    $1::uuid, $2::uuid, $3, $4, $5, $6, $7,
		    $8::jsonb, $9::jsonb, $10, $11, $12, $13,
		    nullif($14, '')::jsonb, 'active', true, now(), now()
		)
		returning id
	`, accountID, sourceID, sourceLabel,
		getStr("name"), getStr("code"), getStr("description"), getStr("image"),
		string(categoriesJSON), string(campaignsJSON),
		getNum("price"), getNum("fator"), getStr("tipo"), getInt("stock"),
		raw).Scan(&id)
	if err != nil {
		return ProductView{}, err
	}
	return r.Find(ctx, accountID, id)
}

func (r *PostgresProductRepository) Update(ctx context.Context, accountID, productID string, input ProductUpdateInput) (ProductView, error) {
	sets := []string{}
	args := []any{accountID, productID}
	n := 3
	addSet := func(column string, value any) {
		sets = append(sets, fmt.Sprintf("%s = $%d", column, n))
		args = append(args, value)
		n++
	}

	if input.Name != nil {
		addSet("name", *input.Name)
	}
	if input.Code != nil {
		addSet("code", *input.Code)
	}
	if input.Description != nil {
		addSet("description", *input.Description)
	}
	if input.Image != nil {
		addSet("image", *input.Image)
	}
	if input.Categories != nil {
		b, _ := json.Marshal(input.Categories)
		sets = append(sets, fmt.Sprintf("categories = $%d::jsonb", n))
		args = append(args, string(b))
		n++
	}
	if input.Campaigns != nil {
		b, _ := json.Marshal(input.Campaigns)
		sets = append(sets, fmt.Sprintf("campaigns = $%d::jsonb", n))
		args = append(args, string(b))
		n++
	}
	if input.Price != nil {
		addSet("price", *input.Price)
	}
	if input.Fator != nil {
		addSet("fator", *input.Fator)
	}
	if input.Tipo != nil {
		addSet("tipo", *input.Tipo)
	}
	if input.Stock != nil {
		addSet("stock", *input.Stock)
	}
	if input.Status != nil {
		addSet("status", *input.Status)
	}

	if len(sets) == 0 {
		return r.Find(ctx, accountID, productID)
	}

	sets = append(sets, fmt.Sprintf("updated_at = $%d", n))
	args = append(args, time.Now())

	query := fmt.Sprintf(`
		update site.products set %s
		where account_id = $1::uuid and id = $2::uuid and is_active = true
	`, strings.Join(sets, ", "))

	tag, err := r.pool.Exec(ctx, query, args...)
	if err != nil {
		return ProductView{}, err
	}
	if tag.RowsAffected() == 0 {
		return ProductView{}, ErrProductNotFound
	}
	return r.Find(ctx, accountID, productID)
}

func (r *PostgresProductRepository) SoftDelete(ctx context.Context, accountID, productID string) error {
	tag, err := r.pool.Exec(ctx,
		`update site.products set is_active = false, updated_at = now() where account_id = $1::uuid and id = $2::uuid`,
		accountID, productID,
	)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrProductNotFound
	}
	return nil
}

func scanProduct(row scannable) (ProductView, error) {
	var v ProductView
	var sourceID, categoriesJSON, campaignsJSON, erpName, erpDescription *string
	if err := row.Scan(
		&v.ID, &v.AccountID, &sourceID, &v.SourceLabel,
		&v.Name, &v.Code, &v.Description, &v.Image,
		&categoriesJSON, &campaignsJSON,
		&v.Price, &v.Fator, &v.Tipo, &v.Stock, &v.Status,
		&v.ErpSynced, &erpName, &erpDescription,
		&v.CreatedAt, &v.UpdatedAt,
	); err != nil {
		return ProductView{}, err
	}
	if sourceID != nil {
		v.SourceID = *sourceID
	}
	if erpName != nil {
		v.ErpName = *erpName
	}
	if erpDescription != nil {
		v.ErpDescription = *erpDescription
	}
	if categoriesJSON != nil && *categoriesJSON != "" {
		_ = json.Unmarshal([]byte(*categoriesJSON), &v.Categories)
	}
	if campaignsJSON != nil && *campaignsJSON != "" {
		_ = json.Unmarshal([]byte(*campaignsJSON), &v.Campaigns)
	}
	if v.Categories == nil {
		v.Categories = []string{}
	}
	if v.Campaigns == nil {
		v.Campaigns = []string{}
	}
	return v, nil
}
