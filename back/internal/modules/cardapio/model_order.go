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

// Formas de pagamento escolhidas no checkout. Tokens livres validados contra o
// que o restaurante aceita (settings.payment). Espelham PaymentSettings.
const (
	PaymentMethodPix    = "pix"
	PaymentMethodCash   = "cash"
	PaymentMethodDebit  = "debit"
	PaymentMethodCredit = "credit"
	PaymentMethodTicket = "ticket"
	PaymentMethodOther  = "other"
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
	Code             string          `json:"code"`
	Status           string          `json:"status"`
	Type             string          `json:"type"`
	CustomerName     string          `json:"customerName"`
	CustomerPhone    string          `json:"customerPhone"`
	DeliveryAddress  json.RawMessage `json:"deliveryAddress"`
	PaymentMethod    string          `json:"paymentMethod"`
	ChangeForCents   int64           `json:"changeForCents"`
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
// DeliveryZoneID (WS-A) e o id da zona de entrega escolhida; quando valido (zona
// ativa do restaurante) o frete vem da zona, senao cai no settings.deliveryFeeCents.
type PublicOrderInput struct {
	Type            string              `json:"type"`
	Customer        PublicCustomerInput `json:"customer"`
	DeliveryAddress json.RawMessage     `json:"deliveryAddress"`
	DeliveryZoneID  string              `json:"deliveryZoneId"`
	// PaymentMethod e a forma escolhida no checkout (pix/cash/debit/credit/ticket/
	// other); validada contra settings.payment. ChangeForCents e o "troco para"
	// (so faz sentido em entrega + dinheiro; ignorado nos demais casos).
	PaymentMethod  string                 `json:"paymentMethod"`
	ChangeForCents int64                  `json:"changeForCents"`
	Notes          string                 `json:"notes"`
	SessionID      string                 `json:"sessionId"`
	Items          []PublicOrderItemInput `json:"items"`
}

// ============================================================================
// Eventos publicos
// ============================================================================

// Limites da ingestao em lote (POST .../events/batch). maxBatchBodyBytes cobre o
// corpo inteiro do lote (decoder dedicado, menor que o 1MB fixo do httpapi.ReadJSON);
// maxBatchEvents limita a contagem de eventos por requisicao.
const (
	maxBatchEvents    = 50
	maxBatchBodyBytes = 256 * 1024
)

// PublicEventInput e o corpo de POST /v1/public/restaurants/{slug}/events (singular,
// legado mantido para compat).
type PublicEventInput struct {
	Name      string          `json:"name"`
	SessionID string          `json:"sessionId"`
	Context   json.RawMessage `json:"context"`
}

// PublicEventEntry e um evento dentro do lote. EventID (uuid do cliente) permite
// dedupe na ingestao; OccurredAt (RFC3339, relogio do cliente) so ordena dentro do
// lote — o horario canonico (histograma de picos) e sempre created_at do servidor.
type PublicEventEntry struct {
	EventID    string          `json:"eventId"`
	Name       string          `json:"name"`
	SessionID  string          `json:"sessionId"`
	OccurredAt string          `json:"occurredAt"`
	Context    json.RawMessage `json:"context"`
}

// PublicEventBatchInput e o corpo de POST /v1/public/restaurants/{slug}/events/batch.
type PublicEventBatchInput struct {
	SessionID string             `json:"sessionId"`
	DeviceID  string             `json:"deviceId"`
	Events    []PublicEventEntry `json:"events"`
}

// allowedEvents e a allowlist EXATA de nomes de evento aceitos (36 nomes, secao 5 do
// PLANO_CARDAPIO_TRACKING_ANALYTICS.md). Sincronizar com o AGENT.md e o teste de
// allowlist. Nome fora da lista = descartado na ingestao (rejected++), nunca derruba
// o lote. Nenhum context carrega PII (defesa no back via sanitizeContext).
var allowedEvents = map[string]struct{}{
	// Navegacao/sessao
	"page_view":     {},
	"session_start": {},
	"session_end":   {},
	// Acesso
	"restaurant_viewed": {},
	"menu_viewed":       {},
	// Produto
	"product_impression":     {},
	"product_clicked":        {},
	"product_viewed":         {},
	"product_option_changed": {},
	// Carrinho/qtd
	"add_to_cart":      {},
	"cart_qty_changed": {},
	"remove_from_cart": {},
	"cart_opened":      {},
	"cart_cleared":     {},
	// Checkout
	"checkout_started":          {},
	"checkout_type_changed":     {},
	"checkout_payment_selected": {},
	"checkout_submitted":        {},
	"checkout_failed":           {},
	"order_created":             {},
	"whatsapp_order_clicked":    {},
	// Catalogo/busca
	"category_viewed":      {},
	"category_tab_clicked": {},
	"menu_search":          {},
	"menu_filter_changed":  {},
	"menu_sort_changed":    {},
	// CTA/saida
	"cta_clicked":    {},
	"outbound_click": {},
	// Engajamento
	"scroll_depth":  {},
	"page_dwell":    {},
	"product_dwell": {},
	"section_dwell": {},
	// Cupom/reserva
	"coupon_viewed":       {},
	"coupon_used":         {},
	"reservation_started": {},
	"reservation_sent":    {},
}

// isAllowedEvent informa se o nome do evento esta na allowlist.
func isAllowedEvent(name string) bool {
	_, ok := allowedEvents[name]
	return ok
}
