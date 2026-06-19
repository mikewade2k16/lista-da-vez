package cardapio

import (
	"encoding/json"
	"time"
)

// Status validos do pedido (validados no service).
const (
	OrderStatusReceived  = "recebido"
	OrderStatusPreparing = "em_preparo"
	OrderStatusReady     = "pronto"
	OrderStatusOnRoute   = "saiu_entrega"
	OrderStatusDelivered = "entregue"
	OrderStatusCanceled  = "cancelado"
)

// Tipos validos do pedido (validados no service contra as settings).
const (
	OrderTypePickup   = "retirada"
	OrderTypeDelivery = "entrega"
	OrderTypeDineIn   = "local"
)

// OrderAddonSnapshot e o snapshot de um adicional no item do pedido.
type OrderAddonSnapshot struct {
	Name       string `json:"name"`
	PriceCents int64  `json:"priceCents"`
}

// OrderItem e o snapshot imutavel de um item do pedido.
type OrderItem struct {
	ID             string               `json:"id"`
	OrderID        string               `json:"orderId"`
	ProductID      *string              `json:"productId"`
	ProductName    string               `json:"productName"`
	VariationName  string               `json:"variationName"`
	Addons         []OrderAddonSnapshot `json:"addons"`
	Quantity       int                  `json:"quantity"`
	UnitPriceCents int64                `json:"unitPriceCents"`
	TotalCents     int64                `json:"totalCents"`
	Notes          string               `json:"notes"`
}

// Order e o DTO completo do pedido.
type Order struct {
	ID               string          `json:"id"`
	RestaurantID     string          `json:"restaurantId"`
	CustomerID       *string         `json:"customerId"`
	OrderNumber      int             `json:"orderNumber"`
	Status           string          `json:"status"`
	Type             string          `json:"type"`
	CustomerName     string          `json:"customerName"`
	CustomerPhone    string          `json:"customerPhone"`
	DeliveryAddress  json.RawMessage `json:"deliveryAddress"`
	Notes            string          `json:"notes"`
	SubtotalCents    int64           `json:"subtotalCents"`
	DeliveryFeeCents int64           `json:"deliveryFeeCents"`
	DiscountCents    int64           `json:"discountCents"`
	TotalCents       int64           `json:"totalCents"`
	CreatedAt        time.Time       `json:"createdAt"`
	UpdatedAt        time.Time       `json:"updatedAt"`
	Items            []OrderItem     `json:"items"`
}

// ============================================================================
// Request publico de pedido
// ============================================================================

// PublicOrderItemInput e um item enviado pelo cliente no checkout. addonIds e
// variationId referenciam opcoes do produto; o preco e SEMPRE recalculado do
// banco — qualquer valor enviado pelo cliente e ignorado.
type PublicOrderItemInput struct {
	ProductID   string   `json:"productId"`
	VariationID string   `json:"variationId"`
	AddonIDs    []string `json:"addonIds"`
	Quantity    int      `json:"quantity"`
	Notes       string   `json:"notes"`
}

// PublicCustomerInput sao os dados do cliente final no checkout. Address e o
// endereco de entrega no formato do contrato (customer.address); jsonb livre.
type PublicCustomerInput struct {
	Name    string          `json:"name"`
	Phone   string          `json:"phone"`
	Address json.RawMessage `json:"address"`
}

// PublicOrderInput e o corpo de POST /v1/public/restaurants/{slug}/orders.
type PublicOrderInput struct {
	Type            string                 `json:"type"`
	Customer        PublicCustomerInput    `json:"customer"`
	DeliveryAddress json.RawMessage        `json:"deliveryAddress"`
	Notes           string                 `json:"notes"`
	SessionID       string                 `json:"sessionId"`
	Items           []PublicOrderItemInput `json:"items"`
}

// ============================================================================
// Eventos publicos
// ============================================================================

// PublicEventInput e o corpo de POST /v1/public/restaurants/{slug}/events.
type PublicEventInput struct {
	Name      string          `json:"name"`
	SessionID string          `json:"sessionId"`
	Context   json.RawMessage `json:"context"`
}

// allowedEvents e a allowlist EXATA de nomes de evento aceitos.
var allowedEvents = map[string]struct{}{
	"page_view":              {},
	"restaurant_viewed":      {},
	"menu_viewed":            {},
	"category_viewed":        {},
	"product_viewed":         {},
	"product_clicked":        {},
	"add_to_cart":            {},
	"remove_from_cart":       {},
	"cart_opened":            {},
	"checkout_started":       {},
	"whatsapp_order_clicked": {},
	"reservation_started":    {},
	"reservation_sent":       {},
	"coupon_viewed":          {},
	"coupon_used":            {},
}

// isAllowedEvent informa se o nome do evento esta na allowlist.
func isAllowedEvent(name string) bool {
	_, ok := allowedEvents[name]
	return ok
}
