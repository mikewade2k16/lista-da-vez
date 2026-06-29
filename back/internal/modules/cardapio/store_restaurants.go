package cardapio

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Store e a persistencia do modulo (schema cardapio.*). Todo metodo filtra por
// account_id (defesa em profundidade); o escopo ja foi validado no service.
type Store struct {
	pool *pgxpool.Pool
}

// NewStore cria o Store.
func NewStore(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool}
}

type rowScanner interface {
	Scan(dest ...any) error
}

const restaurantColumns = `id, slug, name, tagline, description, logo_url, banner_url,
	whatsapp, phone, email, instagram, address, hours, settings, theme,
	segment, facebook, youtube, google_analytics_id, facebook_pixel_id, custom_head_html,
	is_active, created_at, updated_at`

func scanRestaurant(row rowScanner) (Restaurant, error) {
	var r Restaurant
	var address, hours, settings, theme []byte
	err := row.Scan(
		&r.ID, &r.Slug, &r.Name, &r.Tagline, &r.Description, &r.LogoURL, &r.BannerURL,
		&r.WhatsApp, &r.Phone, &r.Email, &r.Instagram, &address, &hours, &settings, &theme,
		&r.Segment, &r.Facebook, &r.Youtube, &r.GoogleAnalyticsID, &r.FacebookPixelID, &r.CustomHeadHTML,
		&r.IsActive, &r.CreatedAt, &r.UpdatedAt,
	)
	if err != nil {
		return Restaurant{}, err
	}
	hydrateRestaurantJSON(&r, address, hours, settings, theme)
	return r, nil
}

// jsonUnmarshalInto decodifica raw em dst tolerando bytes vazios.
func jsonUnmarshalInto(raw []byte, dst any) error {
	if len(raw) == 0 {
		return nil
	}
	return json.Unmarshal(raw, dst)
}

// CreateRestaurant insere um restaurante novo (so nome + slug; demais campos no
// default). Retorna o restaurante completo.
func (s *Store) CreateRestaurant(ctx context.Context, accountID, slug, name string) (Restaurant, error) {
	const q = `insert into cardapio.restaurants (account_id, slug, name)
		values ($1, $2, $3)
		returning ` + restaurantColumns
	return scanRestaurant(s.pool.QueryRow(ctx, q, accountID, slug, name))
}

// GetRestaurant busca por id dentro da account.
func (s *Store) GetRestaurant(ctx context.Context, accountID, id string) (Restaurant, error) {
	const q = `select ` + restaurantColumns + `
		from cardapio.restaurants where id = $1 and account_id = $2`
	return scanRestaurant(s.pool.QueryRow(ctx, q, id, accountID))
}

// ListRestaurantsLean retorna a projecao enxuta da listagem do painel, com o
// nome da account e o dominio primario (1 query, sem N+1). accountID vazio =
// todas (platform_admin); senao filtra.
func (s *Store) ListRestaurantsLean(ctx context.Context, accountID, query string) ([]RestaurantLean, error) {
	const q = `select r.id, r.account_id, coalesce(a.name, ''), r.slug, r.name,
			r.is_active,
			coalesce((select d.host from cardapio.restaurant_domains d
				where d.restaurant_id = r.id order by d.is_primary desc, d.created_at limit 1), ''),
			r.updated_at
		from cardapio.restaurants r
		join core.accounts a on a.id = r.account_id
		where ($1 = '' or r.account_id = $1::uuid)
		  and ($2 = '' or r.name ilike '%' || $2 || '%' or r.slug ilike '%' || $2 || '%')
		order by r.updated_at desc`
	rows, err := s.pool.Query(ctx, q, accountID, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]RestaurantLean, 0)
	for rows.Next() {
		var r RestaurantLean
		if err := rows.Scan(&r.ID, &r.AccountID, &r.AccountName, &r.Slug, &r.Name,
			&r.IsActive, &r.PrimaryDomain, &r.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// UpdateRestaurant aplica um PATCH parcial. Campos nil sao preservados via COALESCE.
// O WHERE escopa pela account ATUAL ($2). Mover de conta NAO passa por aqui: o
// service redireciona o move para MoveRestaurantToAccount (subarvore inteira numa
// transacao), entao este UPDATE so toca os campos do proprio restaurante.
func (s *Store) UpdateRestaurant(ctx context.Context, accountID, id string, in UpdateRestaurantInput) (Restaurant, error) {
	address, hours, settings, theme := encodeJSONUpdates(in)
	const q = `update cardapio.restaurants set
			name                = coalesce($3, name),
			tagline             = coalesce($4, tagline),
			description         = coalesce($5, description),
			logo_url            = coalesce($6, logo_url),
			banner_url          = coalesce($7, banner_url),
			whatsapp            = coalesce($8, whatsapp),
			phone               = coalesce($9, phone),
			email               = coalesce($10, email),
			instagram           = coalesce($11, instagram),
			address             = coalesce($12, address),
			hours               = coalesce($13, hours),
			settings            = coalesce($14, settings),
			theme               = coalesce($15, theme),
			segment             = coalesce($16, segment),
			facebook            = coalesce($17, facebook),
			youtube             = coalesce($18, youtube),
			google_analytics_id = coalesce($19, google_analytics_id),
			facebook_pixel_id   = coalesce($20, facebook_pixel_id),
			custom_head_html    = coalesce($21, custom_head_html),
			is_active           = coalesce($22, is_active),
			updated_at          = now()
		where id = $1 and account_id = $2
		returning ` + restaurantColumns
	return scanRestaurant(s.pool.QueryRow(ctx, q, id, accountID,
		in.Name, in.Tagline, in.Description, in.LogoURL, in.BannerURL,
		in.WhatsApp, in.Phone, in.Email, in.Instagram,
		address, hours, settings, theme,
		in.Segment, in.Facebook, in.Youtube, in.GoogleAnalyticsID, in.FacebookPixelID, in.CustomHeadHTML,
		in.IsActive))
}

// moveChildTables lista as tabelas-filhas do schema cardapio que tem account_id e
// se ligam ao restaurante por restaurant_id (atualizadas direto no move).
var moveChildTablesByRestaurant = []string{
	"cardapio.restaurant_domains",
	"cardapio.categories",
	"cardapio.products",
	"cardapio.reviews",
	"cardapio.delivery_zones",
	"cardapio.orders",
	"cardapio.events",
	"cardapio.site_layouts",
}

// MoveRestaurantToAccount move a SUBARVORE INTEIRA do restaurante para a conta
// destino numa unica transacao e habilita o modulo cardapio no destino (decisao
// de negocio: auto-habilita). Sem isso o cardapio fica orfao (filhas com
// account_id antigo) e o site publico cai (o publico exige core.account_modules
// habilitado na conta nova).
//
// A linha do proprio restaurante e escopada pela conta ATUAL ($current): 0 linhas
// afetadas => ErrNotFound (fora de escopo, defesa em profundidade, sem vazar
// existencia). As filhas sao escopadas pelo restaurant_id (direta ou via subquery
// no produto/pedido) e movidas para $target. Retorna o restaurante ja sob a conta
// NOVA para o PATCH refletir o move.
func (s *Store) MoveRestaurantToAccount(ctx context.Context, currentAccountID, restaurantID, targetAccountID string) (Restaurant, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Restaurant{}, err
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	// 1. O proprio restaurante (escopado pela conta atual: 0 linhas => fora de escopo).
	const moveRoot = `update cardapio.restaurants
		set account_id = $3::uuid, updated_at = now()
		where id = $1 and account_id = $2`
	tag, err := tx.Exec(ctx, moveRoot, restaurantID, currentAccountID, targetAccountID)
	if err != nil {
		return Restaurant{}, err
	}
	if tag.RowsAffected() == 0 {
		return Restaurant{}, ErrNotFound
	}

	// 2. Tabelas-filhas ligadas direto por restaurant_id.
	for _, table := range moveChildTablesByRestaurant {
		q := `update ` + table + ` set account_id = $2::uuid where restaurant_id = $1`
		if _, err := tx.Exec(ctx, q, restaurantID, targetAccountID); err != nil {
			return Restaurant{}, err
		}
	}

	// 3. Variacoes/adicionais: ligadas por product_id (filhas dos produtos).
	const moveVariations = `update cardapio.product_variations set account_id = $2::uuid
		where product_id in (select id from cardapio.products where restaurant_id = $1)`
	if _, err := tx.Exec(ctx, moveVariations, restaurantID, targetAccountID); err != nil {
		return Restaurant{}, err
	}
	const moveAddons = `update cardapio.product_addons set account_id = $2::uuid
		where product_id in (select id from cardapio.products where restaurant_id = $1)`
	if _, err := tx.Exec(ctx, moveAddons, restaurantID, targetAccountID); err != nil {
		return Restaurant{}, err
	}

	// 4. Itens de pedido: ligados por order_id (filhas dos pedidos).
	const moveOrderItems = `update cardapio.order_items set account_id = $2::uuid
		where order_id in (select id from cardapio.orders where restaurant_id = $1)`
	if _, err := tx.Exec(ctx, moveOrderItems, restaurantID, targetAccountID); err != nil {
		return Restaurant{}, err
	}

	// 5. Habilita o modulo cardapio na conta destino (auto-habilita no move).
	const enableModule = `insert into core.account_modules (account_id, module_id, enabled)
		values ($1::uuid, 'cardapio', true)
		on conflict (account_id, module_id) do update set enabled = true`
	if _, err := tx.Exec(ctx, enableModule, targetAccountID); err != nil {
		return Restaurant{}, err
	}

	// 6. Re-scan sob a conta NOVA para a resposta refletir o move.
	const reread = `select ` + restaurantColumns + `
		from cardapio.restaurants where id = $1 and account_id = $2::uuid`
	r, err := scanRestaurant(tx.QueryRow(ctx, reread, restaurantID, targetAccountID))
	if err != nil {
		return Restaurant{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Restaurant{}, err
	}
	return r, nil
}

// DuplicateRestaurant copia um restaurante inteiro (catalogo + zonas + layout)
// para um novo id/slug/name na MESMA account do source, numa UNICA transacao (F1).
// O source e escopado pela account ($accountID): inexistente/fora de escopo =>
// ErrNotFound (sem vazar existencia). O novo restaurante nasce is_active=false e
// last_order_number=0. NAO copia: restaurant_domains (host unico), reviews
// (curadas), orders/order_items, events. Espelha o padrao de MoveRestaurantToAccount
// (transacao + subquery por restaurant_id). Retorna o restaurante novo (full).
func (s *Store) DuplicateRestaurant(ctx context.Context, accountID, sourceID, slug, name string) (Restaurant, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Restaurant{}, err
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	// 1. Restaurante: copia todos os campos do source (escopado pela account),
	//    sob novo id/slug/name; is_active=false, last_order_number=0.
	const insertRoot = `insert into cardapio.restaurants (
			account_id, slug, name, tagline, description, logo_url, banner_url,
			whatsapp, phone, email, instagram, address, hours, settings, theme,
			segment, facebook, youtube, google_analytics_id, facebook_pixel_id, custom_head_html,
			is_active, last_order_number)
		select account_id, $3, $4, tagline, description, logo_url, banner_url,
			whatsapp, phone, email, instagram, address, hours, settings, theme,
			segment, facebook, youtube, google_analytics_id, facebook_pixel_id, custom_head_html,
			false, 0
		from cardapio.restaurants
		where id = $1 and account_id = $2
		returning ` + restaurantColumns
	// scanRestaurant devolve pgx.ErrNoRows quando o source esta fora de escopo; o
	// service traduz para ErrNotFound (404 uniforme) via mapStoreErr.
	newRestaurant, err := scanRestaurant(tx.QueryRow(ctx, insertRoot, sourceID, accountID, slug, name))
	if err != nil {
		return Restaurant{}, err
	}
	newID := newRestaurant.ID

	// 2. Categorias: novos ids; preserva slug/name/subtitulo/capa/banner/ordem/ativo.
	//    O remapeamento category_id dos produtos e feito por slug (unico por
	//    restaurante) direto no SQL do passo 3 — sem mapa em memoria.
	const copyCategories = `insert into cardapio.categories
			(account_id, restaurant_id, slug, name, description, image_url, banner_url, sort_order, is_active)
		select account_id, $2, slug, name, description, image_url, banner_url, sort_order, is_active
		from cardapio.categories
		where restaurant_id = $1`
	if _, err := tx.Exec(ctx, copyCategories, sourceID, newID); err != nil {
		return Restaurant{}, err
	}

	// 3. Produtos: novos ids; remapeia category_id (via slug da categoria source)
	//    para a categoria nova; preserva slug/precos/flags/jsonb. O slug e unico por
	//    restaurante, entao variacoes/adicionais reencontram o produto novo por slug.
	const copyProducts = `insert into cardapio.products
			(account_id, restaurant_id, category_id, slug, name, short_desc, description, body,
			 price_cents, image_url, gallery, weight, cook_time, diet, allergens, pairing, tags,
			 is_available, is_featured, sort_order, compare_at_price_cents)
		select p.account_id, $2,
			case when p.category_id is null then null
			     else (select c2.id from cardapio.categories c2
			           where c2.restaurant_id = $2 and c2.slug = c1.slug) end,
			p.slug, p.name, p.short_desc, p.description, p.body,
			p.price_cents, p.image_url, p.gallery, p.weight, p.cook_time, p.diet, p.allergens,
			p.pairing, p.tags, p.is_available, p.is_featured, p.sort_order, p.compare_at_price_cents
		from cardapio.products p
		left join cardapio.categories c1 on c1.id = p.category_id
		where p.restaurant_id = $1`
	if _, err := tx.Exec(ctx, copyProducts, sourceID, newID); err != nil {
		return Restaurant{}, err
	}

	// 4. Variacoes e adicionais: novos ids, ligados ao produto NOVO. O produto novo
	//    e encontrado pelo slug (unico por restaurante).
	const copyVariations = `insert into cardapio.product_variations
			(account_id, product_id, name, price_delta_cents, sort_order)
		select v.account_id, np.id, v.name, v.price_delta_cents, v.sort_order
		from cardapio.product_variations v
		join cardapio.products sp on sp.id = v.product_id
		join cardapio.products np on np.restaurant_id = $2 and np.slug = sp.slug
		where sp.restaurant_id = $1`
	if _, err := tx.Exec(ctx, copyVariations, sourceID, newID); err != nil {
		return Restaurant{}, err
	}
	const copyAddons = `insert into cardapio.product_addons
			(account_id, product_id, name, price_cents, sort_order)
		select a.account_id, np.id, a.name, a.price_cents, a.sort_order
		from cardapio.product_addons a
		join cardapio.products sp on sp.id = a.product_id
		join cardapio.products np on np.restaurant_id = $2 and np.slug = sp.slug
		where sp.restaurant_id = $1`
	if _, err := tx.Exec(ctx, copyAddons, sourceID, newID); err != nil {
		return Restaurant{}, err
	}

	// 5. Zonas de entrega: novos ids; preserva name/fee/ativo/ordem.
	const copyZones = `insert into cardapio.delivery_zones
			(account_id, restaurant_id, name, fee_cents, is_active, sort_order)
		select account_id, $2, name, fee_cents, is_active, sort_order
		from cardapio.delivery_zones
		where restaurant_id = $1`
	if _, err := tx.Exec(ctx, copyZones, sourceID, newID); err != nil {
		return Restaurant{}, err
	}

	// 6. Layout do site: copia draft + published + version (novo restaurant_id).
	const copyLayout = `insert into cardapio.site_layouts
			(account_id, restaurant_id, draft, published, version)
		select account_id, $2, draft, published, version
		from cardapio.site_layouts
		where restaurant_id = $1`
	if _, err := tx.Exec(ctx, copyLayout, sourceID, newID); err != nil {
		return Restaurant{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return Restaurant{}, err
	}
	return newRestaurant, nil
}

// AccountExists informa se a conta destino existe (espelha bio). Usado pelo
// service antes de mover um restaurante de conta — devolve um 404 limpo se a
// conta nao existe, alem da protecao da FK account_id.
func (s *Store) AccountExists(ctx context.Context, accountID string) (bool, error) {
	const q = `select exists(select 1 from core.accounts where id = $1::uuid)`
	var exists bool
	if err := s.pool.QueryRow(ctx, q, accountID).Scan(&exists); err != nil {
		return false, err
	}
	return exists, nil
}

func encodeJSONUpdates(in UpdateRestaurantInput) (address, hours, settings, theme []byte) {
	if in.Address != nil {
		address, _ = json.Marshal(in.Address)
	}
	if in.Hours != nil {
		hours, _ = json.Marshal(in.Hours)
	}
	if in.Settings != nil {
		settings, _ = json.Marshal(in.Settings)
	}
	if in.Theme != nil {
		theme = *in.Theme
	}
	return address, hours, settings, theme
}

// DeleteRestaurant remove o restaurante (cascade nas filhas). Retorna ErrNotFound.
func (s *Store) DeleteRestaurant(ctx context.Context, accountID, id string) error {
	const q = `delete from cardapio.restaurants where id = $1 and account_id = $2`
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
// Domains
// ============================================================================

// ListDomains retorna os dominios de um restaurante.
func (s *Store) ListDomains(ctx context.Context, accountID, restaurantID string) ([]Domain, error) {
	const q = `select host, restaurant_id, is_primary, created_at
		from cardapio.restaurant_domains
		where restaurant_id = $1 and account_id = $2
		order by is_primary desc, created_at`
	rows, err := s.pool.Query(ctx, q, restaurantID, accountID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]Domain, 0)
	for rows.Next() {
		var d Domain
		if err := rows.Scan(&d.Host, &d.RestaurantID, &d.IsPrimary, &d.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

// CreateDomain insere um dominio (host ja normalizado pelo service).
func (s *Store) CreateDomain(ctx context.Context, accountID, restaurantID, host string, isPrimary bool) (Domain, error) {
	const q = `insert into cardapio.restaurant_domains (host, restaurant_id, account_id, is_primary)
		values ($1, $2, $3, $4)
		returning host, restaurant_id, is_primary, created_at`
	var d Domain
	err := s.pool.QueryRow(ctx, q, host, restaurantID, accountID, isPrimary).
		Scan(&d.Host, &d.RestaurantID, &d.IsPrimary, &d.CreatedAt)
	return d, err
}

// DeleteDomain remove um dominio por host dentro da account.
func (s *Store) DeleteDomain(ctx context.Context, accountID, host string) error {
	const q = `delete from cardapio.restaurant_domains where host = $1 and account_id = $2`
	tag, err := s.pool.Exec(ctx, q, strings.ToLower(strings.TrimSpace(host)), accountID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
