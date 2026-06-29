package cardapio

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"

	"github.com/jackc/pgx/v5"
)

// orderCodeAlphabet e base32 Crockford (sem I/L/O/U); 32 chars => modulo sem vies.
const orderCodeAlphabet = "0123456789ABCDEFGHJKMNPQRSTVWXYZ"
const orderCodeLen = 6

// generateOrderCode gera um codigo curto e legivel (ex.: "K7Q4PM").
func generateOrderCode() (string, error) {
	buf := make([]byte, orderCodeLen)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	out := make([]byte, orderCodeLen)
	for i, b := range buf {
		out[i] = orderCodeAlphabet[int(b)%len(orderCodeAlphabet)]
	}
	return string(out), nil
}

// uniqueOrderCode gera um codigo unico por restaurante, checando na MESMA tx; o
// unique index parcial e o backstop contra corrida (falha => pedido reenviado).
func (s *Store) uniqueOrderCode(ctx context.Context, tx pgx.Tx, restaurantID string) (string, error) {
	for i := 0; i < 8; i++ {
		code, err := generateOrderCode()
		if err != nil {
			return "", err
		}
		var exists bool
		if err := tx.QueryRow(ctx,
			`select exists(select 1 from cardapio.orders where restaurant_id = $1 and code = $2)`,
			restaurantID, code).Scan(&exists); err != nil {
			return "", err
		}
		if !exists {
			return code, nil
		}
	}
	return "", errors.New("cardapio: nao foi possivel gerar codigo de pedido unico")
}

// orderInsert e o pedido ja recalculado pelo service, pronto para persistir.
type orderInsert struct {
	AccountID        string
	RestaurantID     string
	Type             string
	SessionID        string
	CustomerName     string
	CustomerPhone    string
	DeliveryAddress  json.RawMessage
	PaymentMethod    string
	ChangeForCents   int64
	Notes            string
	SubtotalCents    int64
	DeliveryFeeCents int64
	DiscountCents    int64
	TotalCents       int64
	Items            []OrderItem
}

const orderColumns = `id, restaurant_id, customer_id, order_number, status, type,
	customer_name, customer_phone, delivery_address, payment_method, change_for_cents,
	notes, subtotal_cents, delivery_fee_cents, discount_cents, total_cents,
	created_at, updated_at, code`

func scanOrder(row rowScanner) (Order, error) {
	var o Order
	var deliveryAddress []byte
	err := row.Scan(
		&o.ID, &o.RestaurantID, &o.CustomerID, &o.OrderNumber, &o.Status, &o.Type,
		&o.CustomerName, &o.CustomerPhone, &deliveryAddress, &o.PaymentMethod, &o.ChangeForCents,
		&o.Notes, &o.SubtotalCents, &o.DeliveryFeeCents, &o.DiscountCents, &o.TotalCents,
		&o.CreatedAt, &o.UpdatedAt, &o.Code,
	)
	if err != nil {
		return Order{}, err
	}
	if len(deliveryAddress) == 0 {
		deliveryAddress = []byte("{}")
	}
	o.DeliveryAddress = deliveryAddress
	o.Items = []OrderItem{}
	return o, nil
}

// CreateOrder insere o pedido + itens numa unica transacao, alocando o
// order_number de forma atomica via UPDATE ... RETURNING no mesmo tx.
func (s *Store) CreateOrder(ctx context.Context, in orderInsert) (Order, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Order{}, err
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	var orderNumber int
	const numQ = `update cardapio.restaurants
		set last_order_number = last_order_number + 1, updated_at = now()
		where id = $1
		returning last_order_number`
	if err := tx.QueryRow(ctx, numQ, in.RestaurantID).Scan(&orderNumber); err != nil {
		return Order{}, err
	}

	code, err := s.uniqueOrderCode(ctx, tx, in.RestaurantID)
	if err != nil {
		return Order{}, err
	}

	deliveryAddress := in.DeliveryAddress
	if len(deliveryAddress) == 0 {
		deliveryAddress = json.RawMessage("{}")
	}
	const insQ = `insert into cardapio.orders
		(account_id, restaurant_id, order_number, status, type, session_id, customer_name,
		 customer_phone, delivery_address, payment_method, change_for_cents, notes,
		 subtotal_cents, delivery_fee_cents, discount_cents, total_cents, code)
		values ($1,$2,$3,'recebido',$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16)
		returning ` + orderColumns
	order, err := scanOrder(tx.QueryRow(ctx, insQ,
		in.AccountID, in.RestaurantID, orderNumber, in.Type, in.SessionID, in.CustomerName,
		in.CustomerPhone, []byte(deliveryAddress), in.PaymentMethod, in.ChangeForCents, in.Notes,
		in.SubtotalCents, in.DeliveryFeeCents, in.DiscountCents, in.TotalCents, code))
	if err != nil {
		return Order{}, err
	}

	for i := range in.Items {
		item := in.Items[i]
		addonsJSON, _ := json.Marshal(item.Addons)
		const itQ = `insert into cardapio.order_items
			(account_id, order_id, product_id, product_name, variation_name, addons,
			 quantity, unit_price_cents, total_cents, notes)
			values ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
			returning id`
		var itemID string
		if err := tx.QueryRow(ctx, itQ, in.AccountID, order.ID, item.ProductID, item.ProductName,
			item.VariationName, addonsJSON, item.Quantity, item.UnitPriceCents, item.TotalCents,
			item.Notes).Scan(&itemID); err != nil {
			return Order{}, err
		}
		item.ID = itemID
		item.OrderID = order.ID
		order.Items = append(order.Items, item)
	}

	if err := tx.Commit(ctx); err != nil {
		return Order{}, err
	}
	return order, nil
}

// ListOrders retorna pedidos paginados de um restaurante, filtrando por status
// quando informado. Carrega os itens em batch (order_id = ANY) — sem N+1.
func (s *Store) ListOrders(ctx context.Context, accountID, restaurantID, status string, limit, offset int) ([]Order, int, error) {
	const countQ = `select count(*) from cardapio.orders
		where restaurant_id = $1 and account_id = $2 and ($3 = '' or status = $3)`
	var total int
	if err := s.pool.QueryRow(ctx, countQ, restaurantID, accountID, status).Scan(&total); err != nil {
		return nil, 0, err
	}

	const q = `select ` + orderColumns + `
		from cardapio.orders
		where restaurant_id = $1 and account_id = $2 and ($3 = '' or status = $3)
		order by created_at desc
		limit $4 offset $5`
	rows, err := s.pool.Query(ctx, q, restaurantID, accountID, status, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	orders := make([]Order, 0)
	for rows.Next() {
		o, err := scanOrder(rows)
		if err != nil {
			return nil, 0, err
		}
		orders = append(orders, o)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	if err := s.attachOrderItems(ctx, accountID, orders); err != nil {
		return nil, 0, err
	}
	return orders, total, nil
}

func (s *Store) attachOrderItems(ctx context.Context, accountID string, orders []Order) error {
	if len(orders) == 0 {
		return nil
	}
	ids := make([]string, len(orders))
	for i := range orders {
		ids[i] = orders[i].ID
	}
	items, err := s.orderItemsByOrder(ctx, accountID, ids)
	if err != nil {
		return err
	}
	for i := range orders {
		orders[i].Items = items[orders[i].ID]
		if orders[i].Items == nil {
			orders[i].Items = []OrderItem{}
		}
	}
	return nil
}

func (s *Store) orderItemsByOrder(ctx context.Context, accountID string, orderIDs []string) (map[string][]OrderItem, error) {
	const q = `select id, order_id, product_id, product_name, variation_name, addons,
			quantity, unit_price_cents, total_cents, notes
		from cardapio.order_items
		where account_id = $1 and order_id = any($2)
		order by id`
	rows, err := s.pool.Query(ctx, q, accountID, orderIDs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make(map[string][]OrderItem, len(orderIDs))
	for rows.Next() {
		item, err := scanOrderItem(rows)
		if err != nil {
			return nil, err
		}
		out[item.OrderID] = append(out[item.OrderID], item)
	}
	return out, rows.Err()
}

func (s *Store) orderItems(ctx context.Context, accountID string, orderIDs []string) ([]OrderItem, error) {
	byOrder, err := s.orderItemsByOrder(ctx, accountID, orderIDs)
	if err != nil {
		return nil, err
	}
	out := make([]OrderItem, 0)
	for _, id := range orderIDs {
		out = append(out, byOrder[id]...)
	}
	return out, nil
}

func scanOrderItem(row rowScanner) (OrderItem, error) {
	var it OrderItem
	var addons []byte
	err := row.Scan(&it.ID, &it.OrderID, &it.ProductID, &it.ProductName, &it.VariationName,
		&addons, &it.Quantity, &it.UnitPriceCents, &it.TotalCents, &it.Notes)
	if err != nil {
		return OrderItem{}, err
	}
	it.Addons = []OrderAddonSnapshot{}
	if len(addons) > 0 {
		_ = json.Unmarshal(addons, &it.Addons)
	}
	if it.Addons == nil {
		it.Addons = []OrderAddonSnapshot{}
	}
	return it, nil
}

// UpdateOrderStatus altera o status de um pedido por id na account.
func (s *Store) UpdateOrderStatus(ctx context.Context, accountID, id, status string) (Order, error) {
	const q = `update cardapio.orders set status = $3, updated_at = now()
		where id = $1 and account_id = $2
		returning ` + orderColumns
	o, err := scanOrder(s.pool.QueryRow(ctx, q, id, accountID, status))
	if err != nil {
		return Order{}, err
	}
	o.Items, err = s.orderItems(ctx, accountID, []string{o.ID})
	if err != nil {
		return Order{}, err
	}
	return o, nil
}
