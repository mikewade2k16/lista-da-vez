package cardapio

import (
	"context"
	"encoding/json"

	"github.com/jackc/pgx/v5"

	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/stringsx"
)

// ============================================================================
// Categories
// ============================================================================

const categoryColumns = `id, restaurant_id, slug, name, description, image_url, banner_url, sort_order, is_active, created_at`

func scanCategory(row rowScanner) (Category, error) {
	var c Category
	err := row.Scan(&c.ID, &c.RestaurantID, &c.Slug, &c.Name, &c.Description,
		&c.ImageURL, &c.BannerURL, &c.SortOrder, &c.IsActive, &c.CreatedAt)
	return c, err
}

// ListCategories retorna as categorias de um restaurante. activeOnly filtra as
// inativas (uso publico).
func (s *Store) ListCategories(ctx context.Context, accountID, restaurantID string, activeOnly bool) ([]Category, error) {
	const q = `select ` + categoryColumns + `
		from cardapio.categories
		where restaurant_id = $1 and account_id = $2 and (not $3 or is_active)
		order by sort_order, name`
	rows, err := s.pool.Query(ctx, q, restaurantID, accountID, activeOnly)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]Category, 0)
	for rows.Next() {
		c, err := scanCategory(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// CreateCategory insere uma categoria.
func (s *Store) CreateCategory(ctx context.Context, accountID, restaurantID string, in CategoryInput) (Category, error) {
	const q = `insert into cardapio.categories
		(account_id, restaurant_id, slug, name, description, image_url, banner_url, sort_order, is_active)
		values ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		returning ` + categoryColumns
	return scanCategory(s.pool.QueryRow(ctx, q, accountID, restaurantID,
		in.Slug, in.Name, in.Description, in.ImageURL, in.BannerURL, in.SortOrder, in.IsActive))
}

// UpdateCategory edita uma categoria por id na account.
func (s *Store) UpdateCategory(ctx context.Context, accountID, id string, in CategoryInput) (Category, error) {
	const q = `update cardapio.categories set
			slug = $3, name = $4, description = $5, image_url = $6, banner_url = $7, sort_order = $8, is_active = $9
		where id = $1 and account_id = $2
		returning ` + categoryColumns
	return scanCategory(s.pool.QueryRow(ctx, q, id, accountID,
		in.Slug, in.Name, in.Description, in.ImageURL, in.BannerURL, in.SortOrder, in.IsActive))
}

// DeleteCategory remove uma categoria (produtos ficam sem categoria via FK).
func (s *Store) DeleteCategory(ctx context.Context, accountID, id string) error {
	const q = `delete from cardapio.categories where id = $1 and account_id = $2`
	tag, err := s.pool.Exec(ctx, q, id, accountID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// ============================================================================
// Products
// ============================================================================

const productColumns = `id, restaurant_id, category_id, slug, name, short_desc, description,
	body, price_cents, image_url, gallery, weight, cook_time, diet, allergens, pairing,
	tags, is_available, is_featured, sort_order, rating, review_count, sold_count,
	created_at, updated_at, compare_at_price_cents`

func scanProduct(row rowScanner) (Product, error) {
	var p Product
	var gallery, diet, allergens, pairing, tags []byte
	err := row.Scan(
		&p.ID, &p.RestaurantID, &p.CategoryID, &p.Slug, &p.Name, &p.ShortDesc, &p.Description,
		&p.Body, &p.PriceCents, &p.ImageURL, &gallery, &p.Weight, &p.CookTime, &diet, &allergens,
		&pairing, &tags, &p.IsAvailable, &p.IsFeatured, &p.SortOrder, &p.Rating, &p.ReviewCount,
		&p.SoldCount, &p.CreatedAt, &p.UpdatedAt, &p.CompareAtPriceCents,
	)
	if err != nil {
		return Product{}, err
	}
	p.Gallery = stringsx.DecodeJSONStringSlice(gallery)
	p.Diet = stringsx.DecodeJSONStringSlice(diet)
	p.Allergens = stringsx.DecodeJSONStringSlice(allergens)
	p.Tags = stringsx.DecodeJSONStringSlice(tags)
	if len(pairing) > 0 {
		p.Pairing = json.RawMessage(pairing)
	}
	p.Variations = []Variation{}
	p.Addons = []Addon{}
	return p, nil
}

// ListProductsLean retorna a projecao enxuta de produtos de um restaurante.
func (s *Store) ListProductsLean(ctx context.Context, accountID, restaurantID string) ([]ProductLean, error) {
	const q = `select id, category_id, slug, name, price_cents, image_url,
			is_available, is_featured, sort_order
		from cardapio.products
		where restaurant_id = $1 and account_id = $2
		order by sort_order, name`
	rows, err := s.pool.Query(ctx, q, restaurantID, accountID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]ProductLean, 0)
	for rows.Next() {
		var p ProductLean
		if err := rows.Scan(&p.ID, &p.CategoryID, &p.Slug, &p.Name, &p.PriceCents,
			&p.ImageURL, &p.IsAvailable, &p.IsFeatured, &p.SortOrder); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// GetProduct busca um produto full (com variations/addons embutidos) por id.
func (s *Store) GetProduct(ctx context.Context, accountID, id string) (Product, error) {
	const q = `select ` + productColumns + `
		from cardapio.products where id = $1 and account_id = $2`
	p, err := scanProduct(s.pool.QueryRow(ctx, q, id, accountID))
	if err != nil {
		return Product{}, err
	}
	if err := s.attachOptions(ctx, []string{p.ID}, []*Product{&p}); err != nil {
		return Product{}, err
	}
	return p, nil
}

// ListMenuProducts retorna todos os produtos disponiveis de um restaurante, ja
// com variations/addons embutidos (sem N+1: WHERE product_id = ANY). Uso publico.
func (s *Store) ListMenuProducts(ctx context.Context, accountID, restaurantID string) ([]Product, error) {
	const q = `select ` + productColumns + `
		from cardapio.products
		where restaurant_id = $1 and account_id = $2 and is_available
		order by sort_order, name`
	rows, err := s.pool.Query(ctx, q, restaurantID, accountID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	products := make([]Product, 0)
	for rows.Next() {
		p, err := scanProduct(rows)
		if err != nil {
			return nil, err
		}
		products = append(products, p)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	ids := make([]string, len(products))
	refs := make([]*Product, len(products))
	for i := range products {
		ids[i] = products[i].ID
		refs[i] = &products[i]
	}
	if err := s.attachOptions(ctx, ids, refs); err != nil {
		return nil, err
	}
	return products, nil
}

// attachOptions carrega variations e addons de varios produtos numa unica query
// cada (product_id = ANY) e distribui nos ponteiros recebidos.
func (s *Store) attachOptions(ctx context.Context, ids []string, refs []*Product) error {
	if len(ids) == 0 {
		return nil
	}
	byID := make(map[string]*Product, len(refs))
	for _, p := range refs {
		byID[p.ID] = p
	}

	const vq = `select id, product_id, name, price_delta_cents, sort_order
		from cardapio.product_variations
		where product_id = any($1) order by sort_order, name`
	vrows, err := s.pool.Query(ctx, vq, ids)
	if err != nil {
		return err
	}
	defer vrows.Close()
	for vrows.Next() {
		var v Variation
		if err := vrows.Scan(&v.ID, &v.ProductID, &v.Name, &v.PriceDeltaCents, &v.SortOrder); err != nil {
			return err
		}
		if p, ok := byID[v.ProductID]; ok {
			p.Variations = append(p.Variations, v)
		}
	}
	if err := vrows.Err(); err != nil {
		return err
	}

	const aq = `select id, product_id, name, price_cents, sort_order
		from cardapio.product_addons
		where product_id = any($1) order by sort_order, name`
	arows, err := s.pool.Query(ctx, aq, ids)
	if err != nil {
		return err
	}
	defer arows.Close()
	for arows.Next() {
		var a Addon
		if err := arows.Scan(&a.ID, &a.ProductID, &a.Name, &a.PriceCents, &a.SortOrder); err != nil {
			return err
		}
		if p, ok := byID[a.ProductID]; ok {
			p.Addons = append(p.Addons, a)
		}
	}
	return arows.Err()
}

// CreateProduct insere um produto e, na MESMA transacao, suas variations/addons.
func (s *Store) CreateProduct(ctx context.Context, accountID, restaurantID string, in ProductInput) (Product, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Product{}, err
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	const q = `insert into cardapio.products
		(account_id, restaurant_id, category_id, slug, name, short_desc, description, body,
		 price_cents, image_url, gallery, weight, cook_time, diet, allergens, pairing, tags,
		 is_available, is_featured, sort_order, compare_at_price_cents)
		values ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,$21)
		returning ` + productColumns
	p, err := scanProduct(tx.QueryRow(ctx, q, productInsertArgs(accountID, restaurantID, in)...))
	if err != nil {
		return Product{}, err
	}
	if err := replaceOptions(ctx, tx, accountID, p.ID, in.Variations, in.Addons); err != nil {
		return Product{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Product{}, err
	}
	return s.GetProduct(ctx, accountID, p.ID)
}

// UpdateProduct edita um produto e faz replace-all de variations/addons quando
// nao-nil, tudo na MESMA transacao.
func (s *Store) UpdateProduct(ctx context.Context, accountID, id string, in ProductInput) (Product, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Product{}, err
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	const q = `update cardapio.products set
			category_id = $3, slug = $4, name = $5, short_desc = $6, description = $7, body = $8,
			price_cents = $9, image_url = $10, gallery = $11, weight = $12, cook_time = $13,
			diet = $14, allergens = $15, pairing = $16, tags = $17, is_available = $18,
			is_featured = $19, sort_order = $20, compare_at_price_cents = $21, updated_at = now()
		where id = $1 and account_id = $2
		returning ` + productColumns
	args := append([]any{id, accountID}, productUpdateArgs(in)...)
	p, err := scanProduct(tx.QueryRow(ctx, q, args...))
	if err != nil {
		return Product{}, err
	}
	if err := replaceOptions(ctx, tx, accountID, p.ID, in.Variations, in.Addons); err != nil {
		return Product{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Product{}, err
	}
	return s.GetProduct(ctx, accountID, p.ID)
}

func productInsertArgs(accountID, restaurantID string, in ProductInput) []any {
	return append([]any{accountID, restaurantID}, productUpdateArgs(in)...)
}

func productUpdateArgs(in ProductInput) []any {
	var pairing any
	if len(in.Pairing) > 0 {
		pairing = []byte(in.Pairing)
	}
	return []any{
		in.CategoryID, in.Slug, in.Name, in.ShortDesc, in.Description, in.Body,
		in.PriceCents, in.ImageURL, mustJSON(in.Gallery), in.Weight, in.CookTime,
		mustJSON(in.Diet), mustJSON(in.Allergens), pairing, mustJSON(in.Tags),
		in.IsAvailable, in.IsFeatured, in.SortOrder, in.CompareAtPriceCents,
	}
}

// replaceOptions apaga e reinsere variations/addons quando a lista nao e nil.
// Lista nil => preserva (PATCH parcial); lista vazia => limpa.
func replaceOptions(ctx context.Context, tx pgx.Tx, accountID, productID string, variations []VariationInput, addons []AddonInput) error {
	if variations != nil {
		if _, err := tx.Exec(ctx, `delete from cardapio.product_variations where product_id = $1`, productID); err != nil {
			return err
		}
		for _, v := range variations {
			if _, err := tx.Exec(ctx, `insert into cardapio.product_variations
				(account_id, product_id, name, price_delta_cents, sort_order)
				values ($1,$2,$3,$4,$5)`, accountID, productID, v.Name, v.PriceDeltaCents, v.SortOrder); err != nil {
				return err
			}
		}
	}
	if addons != nil {
		if _, err := tx.Exec(ctx, `delete from cardapio.product_addons where product_id = $1`, productID); err != nil {
			return err
		}
		for _, a := range addons {
			if _, err := tx.Exec(ctx, `insert into cardapio.product_addons
				(account_id, product_id, name, price_cents, sort_order)
				values ($1,$2,$3,$4,$5)`, accountID, productID, a.Name, a.PriceCents, a.SortOrder); err != nil {
				return err
			}
		}
	}
	return nil
}

func mustJSON(v any) []byte {
	b, err := json.Marshal(v)
	if err != nil || len(b) == 0 {
		return []byte("[]")
	}
	return b
}

// DeleteProduct remove um produto (variations/addons via cascade).
func (s *Store) DeleteProduct(ctx context.Context, accountID, id string) error {
	const q = `delete from cardapio.products where id = $1 and account_id = $2`
	tag, err := s.pool.Exec(ctx, q, id, accountID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// ============================================================================
// Reviews
// ============================================================================

const reviewColumns = `id, restaurant_id, product_id, author_name, author_level, rating,
	body, is_highlight, show_on_establishment, date_label, sort_order, created_at`

// scanReview decodifica uma review. product_id e nullable (F2): review do
// estabelecimento tem product_id NULL, mapeado para string vazia no DTO.
func scanReview(row rowScanner) (Review, error) {
	var r Review
	var productID *string
	err := row.Scan(&r.ID, &r.RestaurantID, &productID, &r.AuthorName, &r.AuthorLevel,
		&r.Rating, &r.Body, &r.IsHighlight, &r.ShowOnEstablishment, &r.DateLabel, &r.SortOrder, &r.CreatedAt)
	if err != nil {
		return Review{}, err
	}
	if productID != nil {
		r.ProductID = *productID
	}
	return r, nil
}

// nullableProductID converte o productId do input (string vazia = review de
// estabelecimento) no valor a gravar (NULL quando vazio).
func nullableProductID(productID string) any {
	if productID == "" {
		return nil
	}
	return productID
}

// ListReviewsByProduct retorna avaliacoes de um produto.
func (s *Store) ListReviewsByProduct(ctx context.Context, accountID, productID string) ([]Review, error) {
	const q = `select ` + reviewColumns + `
		from cardapio.reviews
		where product_id = $1 and account_id = $2
		order by is_highlight desc, sort_order, created_at desc`
	rows, err := s.pool.Query(ctx, q, productID, accountID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]Review, 0)
	for rows.Next() {
		r, err := scanReview(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// ListEstablishmentReviews retorna as avaliacoes do estabelecimento (F2): reviews
// proprias do estabelecimento (product_id IS NULL) + reviews de produto marcadas
// para a vitrine (show_on_establishment = true). Ordenadas por sort_order.
func (s *Store) ListEstablishmentReviews(ctx context.Context, accountID, restaurantID string) ([]Review, error) {
	const q = `select ` + reviewColumns + `
		from cardapio.reviews
		where restaurant_id = $1 and account_id = $2
		  and (product_id is null or show_on_establishment = true)
		order by is_highlight desc, sort_order, created_at desc`
	rows, err := s.pool.Query(ctx, q, restaurantID, accountID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]Review, 0)
	for rows.Next() {
		r, err := scanReview(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// CreateReview insere uma avaliacao do restaurante. product_id vem do input
// (vazio = review de estabelecimento, gravado como NULL).
func (s *Store) CreateReview(ctx context.Context, accountID, restaurantID string, in ReviewInput) (Review, error) {
	const q = `insert into cardapio.reviews
		(account_id, restaurant_id, product_id, author_name, author_level, rating, body,
		 is_highlight, show_on_establishment, date_label, sort_order)
		values ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)
		returning ` + reviewColumns
	return scanReview(s.pool.QueryRow(ctx, q, accountID, restaurantID, nullableProductID(in.ProductID),
		in.AuthorName, in.AuthorLevel, in.Rating, in.Body, in.IsHighlight, in.ShowOnEstablishment,
		in.DateLabel, in.SortOrder))
}

// UpdateReview edita uma avaliacao por id na account (full-replace, inclui
// show_on_establishment). product_id NAO muda no update (definido no create).
func (s *Store) UpdateReview(ctx context.Context, accountID, id string, in ReviewInput) (Review, error) {
	const q = `update cardapio.reviews set
			author_name = $3, author_level = $4, rating = $5, body = $6,
			is_highlight = $7, show_on_establishment = $8, date_label = $9, sort_order = $10
		where id = $1 and account_id = $2
		returning ` + reviewColumns
	return scanReview(s.pool.QueryRow(ctx, q, id, accountID, in.AuthorName, in.AuthorLevel,
		in.Rating, in.Body, in.IsHighlight, in.ShowOnEstablishment, in.DateLabel, in.SortOrder))
}

// DeleteReview remove uma avaliacao por id na account.
func (s *Store) DeleteReview(ctx context.Context, accountID, id string) error {
	const q = `delete from cardapio.reviews where id = $1 and account_id = $2`
	tag, err := s.pool.Exec(ctx, q, id, accountID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
