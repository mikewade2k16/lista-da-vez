package cardapio

import (
	"context"
)

// publicRestaurant carrega um restaurante para uso publico SO se: is_active,
// account ativa e modulo cardapio habilitado na account. Junta tudo numa query;
// pgx.ErrNoRows => 404 uniforme. O lookup e por slug (lower).
func (s *Store) publicRestaurant(ctx context.Context, slug string) (Restaurant, string, error) {
	const q = `select ` + publicRestaurantColumns + `, r.account_id
		from cardapio.restaurants r
		join core.accounts a on a.id = r.account_id and a.is_active
		join core.account_modules m
			on m.account_id = r.account_id and m.module_id = 'cardapio' and m.enabled
		where lower(r.slug) = lower($1) and r.is_active`
	return scanPublicRestaurant(s.pool.QueryRow(ctx, q, slug))
}

const publicRestaurantColumns = `r.id, r.slug, r.name, r.tagline, r.description, r.logo_url,
	r.banner_url, r.whatsapp, r.phone, r.email, r.instagram, r.address, r.hours, r.settings,
	r.theme, r.segment, r.facebook, r.youtube, r.google_analytics_id, r.facebook_pixel_id,
	r.custom_head_html, r.is_active, r.created_at, r.updated_at`

func scanPublicRestaurant(row rowScanner) (Restaurant, string, error) {
	var accountID string
	// O Scan publico tras a coluna extra r.account_id no fim (sentinela): passamos
	// &accountID para o helper compartilhado capturar o 25o campo.
	r, err := scanRestaurantInto(row, &accountID)
	if err != nil {
		return Restaurant{}, "", err
	}
	return r, accountID, nil
}

// scanRestaurantInto e o Scan compartilhado dos 24 campos do restaurante (mesma
// ordem em restaurantColumns e publicRestaurantColumns). Quando accountIDDst nao
// e nil, le tambem a coluna extra account_id (sentinela do Scan publico). Hidrata
// os jsonb no struct. Mantem o comportamento IDENTICO dos dois scans antigos.
func scanRestaurantInto(row rowScanner, accountIDDst *string) (Restaurant, error) {
	var r Restaurant
	var address, hours, settings, theme []byte
	dest := []any{
		&r.ID, &r.Slug, &r.Name, &r.Tagline, &r.Description, &r.LogoURL, &r.BannerURL,
		&r.WhatsApp, &r.Phone, &r.Email, &r.Instagram, &address, &hours, &settings, &theme,
		&r.Segment, &r.Facebook, &r.Youtube, &r.GoogleAnalyticsID, &r.FacebookPixelID, &r.CustomHeadHTML,
		&r.IsActive, &r.CreatedAt, &r.UpdatedAt,
	}
	if accountIDDst != nil {
		dest = append(dest, accountIDDst)
	}
	if err := row.Scan(dest...); err != nil {
		return Restaurant{}, err
	}
	hydrateRestaurantJSON(&r, address, hours, settings, theme)
	return r, nil
}

// hydrateRestaurantJSON decodifica os campos jsonb no struct (compartilhado
// entre o scan publico e o do painel).
func hydrateRestaurantJSON(r *Restaurant, address, hours, settings, theme []byte) {
	_ = jsonUnmarshalInto(address, &r.Address)
	_ = jsonUnmarshalInto(hours, &r.Hours)
	_ = jsonUnmarshalInto(settings, &r.Settings)
	if len(theme) == 0 {
		theme = []byte("{}")
	}
	r.Theme = theme
	if r.Hours == nil {
		r.Hours = []HourSpan{}
	}
}

// resolveSlugActive verifica se um slug existe + esta ativo + modulo habilitado.
// Usado no Resolve (subdominio). Retorna o slug canonico (do banco) ou ErrNotFound.
func (s *Store) resolveSlugActive(ctx context.Context, slug string) (string, error) {
	const q = `select r.slug
		from cardapio.restaurants r
		join core.accounts a on a.id = r.account_id and a.is_active
		join core.account_modules m
			on m.account_id = r.account_id and m.module_id = 'cardapio' and m.enabled
		where lower(r.slug) = lower($1) and r.is_active
		limit 1`
	var out string
	if err := s.pool.QueryRow(ctx, q, slug).Scan(&out); err != nil {
		return "", err
	}
	return out, nil
}

// resolveHostActive resolve um host normalizado -> slug via restaurant_domains,
// exigindo restaurante ativo + account ativa + modulo habilitado.
func (s *Store) resolveHostActive(ctx context.Context, host string) (string, error) {
	const q = `select r.slug
		from cardapio.restaurant_domains d
		join cardapio.restaurants r on r.id = d.restaurant_id and r.is_active
		join core.accounts a on a.id = r.account_id and a.is_active
		join core.account_modules m
			on m.account_id = r.account_id and m.module_id = 'cardapio' and m.enabled
		where d.host = $1
		limit 1`
	var out string
	if err := s.pool.QueryRow(ctx, q, host).Scan(&out); err != nil {
		return "", err
	}
	return out, nil
}

// publicProductBySlug carrega um produto disponivel por slug dentro do
// restaurante, ja com variations/addons. ErrNoRows se indisponivel/inexistente.
func (s *Store) publicProductBySlug(ctx context.Context, accountID, restaurantID, productSlug string) (Product, error) {
	const q = `select ` + productColumns + `
		from cardapio.products
		where restaurant_id = $1 and account_id = $2 and lower(slug) = lower($3) and is_available`
	p, err := scanProduct(s.pool.QueryRow(ctx, q, restaurantID, accountID, productSlug))
	if err != nil {
		return Product{}, err
	}
	if err := s.attachOptions(ctx, []string{p.ID}, []*Product{&p}); err != nil {
		return Product{}, err
	}
	return p, nil
}

// productForOrder carrega os dados minimos de um produto para recalcular um
// pedido: preco, disponibilidade e que pertence ao restaurante. ErrNoRows quando
// nao existe ou e de outro restaurante.
type productForOrder struct {
	ID          string
	Name        string
	PriceCents  int64
	IsAvailable bool
}

func (s *Store) productForOrder(ctx context.Context, restaurantID, productID string) (productForOrder, error) {
	const q = `select id, name, price_cents, is_available
		from cardapio.products
		where id = $1 and restaurant_id = $2`
	var p productForOrder
	err := s.pool.QueryRow(ctx, q, productID, restaurantID).
		Scan(&p.ID, &p.Name, &p.PriceCents, &p.IsAvailable)
	return p, err
}

// variationForProduct carrega uma variacao garantindo que pertence ao produto.
func (s *Store) variationForProduct(ctx context.Context, productID, variationID string) (Variation, error) {
	const q = `select id, product_id, name, price_delta_cents, sort_order
		from cardapio.product_variations
		where id = $1 and product_id = $2`
	var v Variation
	err := s.pool.QueryRow(ctx, q, variationID, productID).
		Scan(&v.ID, &v.ProductID, &v.Name, &v.PriceDeltaCents, &v.SortOrder)
	return v, err
}

// addonsForProduct carrega os adicionais informados garantindo que TODOS
// pertencem ao produto (count != len => addon estranho). Uma unica query.
func (s *Store) addonsForProduct(ctx context.Context, productID string, addonIDs []string) ([]Addon, error) {
	const q = `select id, product_id, name, price_cents, sort_order
		from cardapio.product_addons
		where product_id = $1 and id = any($2)`
	rows, err := s.pool.Query(ctx, q, productID, addonIDs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]Addon, 0, len(addonIDs))
	for rows.Next() {
		var a Addon
		if err := rows.Scan(&a.ID, &a.ProductID, &a.Name, &a.PriceCents, &a.SortOrder); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}
