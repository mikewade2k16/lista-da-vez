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
	whatsapp, phone, email, instagram, address, hours, settings, theme, is_active,
	created_at, updated_at`

func scanRestaurant(row rowScanner) (Restaurant, error) {
	var r Restaurant
	var address, hours, settings, theme []byte
	err := row.Scan(
		&r.ID, &r.Slug, &r.Name, &r.Tagline, &r.Description, &r.LogoURL, &r.BannerURL,
		&r.WhatsApp, &r.Phone, &r.Email, &r.Instagram, &address, &hours, &settings, &theme,
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
func (s *Store) UpdateRestaurant(ctx context.Context, accountID, id string, in UpdateRestaurantInput) (Restaurant, error) {
	address, hours, settings, theme := encodeJSONUpdates(in)
	const q = `update cardapio.restaurants set
			name        = coalesce($3, name),
			tagline     = coalesce($4, tagline),
			description = coalesce($5, description),
			logo_url    = coalesce($6, logo_url),
			banner_url  = coalesce($7, banner_url),
			whatsapp    = coalesce($8, whatsapp),
			phone       = coalesce($9, phone),
			email       = coalesce($10, email),
			instagram   = coalesce($11, instagram),
			address     = coalesce($12, address),
			hours       = coalesce($13, hours),
			settings    = coalesce($14, settings),
			theme       = coalesce($15, theme),
			is_active   = coalesce($16, is_active),
			updated_at  = now()
		where id = $1 and account_id = $2
		returning ` + restaurantColumns
	return scanRestaurant(s.pool.QueryRow(ctx, q, id, accountID,
		in.Name, in.Tagline, in.Description, in.LogoURL, in.BannerURL,
		in.WhatsApp, in.Phone, in.Email, in.Instagram,
		address, hours, settings, theme, in.IsActive))
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
