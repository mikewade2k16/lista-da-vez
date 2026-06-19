package cardapio

import (
	"context"
)

// Store dos zonas de entrega (WS-A). Toda query filtra account_id (defesa em
// profundidade); o escopo ja foi validado no service. centavos int64.

const deliveryZoneColumns = `id, restaurant_id, name, fee_cents, is_active, sort_order`

func scanDeliveryZone(row rowScanner) (DeliveryZone, error) {
	var z DeliveryZone
	err := row.Scan(&z.ID, &z.RestaurantID, &z.Name, &z.FeeCents, &z.IsActive, &z.SortOrder)
	if err != nil {
		return DeliveryZone{}, err
	}
	return z, nil
}

// ListZones lista todas as zonas (ativas e inativas) de um restaurante, na ordem
// de exibicao. Usado pelo painel.
func (s *Store) ListZones(ctx context.Context, accountID, restaurantID string) ([]DeliveryZone, error) {
	const q = `select ` + deliveryZoneColumns + `
		from cardapio.delivery_zones
		where restaurant_id = $1 and account_id = $2
		order by sort_order, lower(name)`
	return s.queryZones(ctx, q, restaurantID, accountID)
}

// ListPublicZones lista SO as zonas ativas de um restaurante (menu publico).
func (s *Store) ListPublicZones(ctx context.Context, accountID, restaurantID string) ([]DeliveryZone, error) {
	const q = `select ` + deliveryZoneColumns + `
		from cardapio.delivery_zones
		where restaurant_id = $1 and account_id = $2 and is_active
		order by sort_order, lower(name)`
	return s.queryZones(ctx, q, restaurantID, accountID)
}

func (s *Store) queryZones(ctx context.Context, q, restaurantID, accountID string) ([]DeliveryZone, error) {
	rows, err := s.pool.Query(ctx, q, restaurantID, accountID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]DeliveryZone, 0)
	for rows.Next() {
		z, err := scanDeliveryZone(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, z)
	}
	return out, rows.Err()
}

// CreateZone insere uma zona (nome ja normalizado pelo service). Unique
// (restaurant_id, lower(name)) => ErrSlugConflict mapeado no service.
func (s *Store) CreateZone(ctx context.Context, accountID, restaurantID string, in DeliveryZoneInput) (DeliveryZone, error) {
	const q = `insert into cardapio.delivery_zones
		(account_id, restaurant_id, name, fee_cents, is_active, sort_order)
		values ($1, $2, $3, $4, $5, $6)
		returning ` + deliveryZoneColumns
	return scanDeliveryZone(s.pool.QueryRow(ctx, q,
		accountID, restaurantID, in.Name, in.FeeCents, in.IsActive, in.SortOrder))
}

// UpdateZone aplica um PATCH parcial. Campos nil sao preservados via COALESCE.
func (s *Store) UpdateZone(ctx context.Context, accountID, id string, in UpdateDeliveryZoneInput) (DeliveryZone, error) {
	const q = `update cardapio.delivery_zones set
			name       = coalesce($3, name),
			fee_cents  = coalesce($4, fee_cents),
			is_active  = coalesce($5, is_active),
			sort_order = coalesce($6, sort_order)
		where id = $1 and account_id = $2
		returning ` + deliveryZoneColumns
	return scanDeliveryZone(s.pool.QueryRow(ctx, q,
		id, accountID, in.Name, in.FeeCents, in.IsActive, in.SortOrder))
}

// DeleteZone remove uma zona por id na account. ErrNotFound se nada apagou.
func (s *Store) DeleteZone(ctx context.Context, accountID, id string) error {
	const q = `delete from cardapio.delivery_zones where id = $1 and account_id = $2`
	tag, err := s.pool.Exec(ctx, q, id, accountID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// zoneForOrder carrega uma zona ATIVA garantindo que pertence ao restaurante.
// Usado no recalculo do frete do pedido publico. ErrNoRows quando nao existe,
// esta inativa ou e de outro restaurante.
func (s *Store) zoneForOrder(ctx context.Context, restaurantID, zoneID string) (DeliveryZone, error) {
	const q = `select ` + deliveryZoneColumns + `
		from cardapio.delivery_zones
		where id = $1 and restaurant_id = $2 and is_active`
	return scanDeliveryZone(s.pool.QueryRow(ctx, q, zoneID, restaurantID))
}
