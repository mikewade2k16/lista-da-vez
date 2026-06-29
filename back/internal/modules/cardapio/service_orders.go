package cardapio

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"github.com/jackc/pgx/v5"
)

const (
	maxOrderItems   = 50
	maxItemQuantity = 50
)

// PlaceOrder recebe o pedido do cliente final, RECALCULA TUDO do banco e
// persiste. O total enviado pelo cliente e ignorado: unitPrice = preco do
// produto + delta da variacao + soma dos adicionais; total do item = unit x qty;
// subtotal = soma dos itens; deliveryFee das settings (zera acima do limiar de
// frete gratis quando configurado). Validacoes fortes (tipo habilitado, itens,
// quantidade, nome, telefone, posse de variacao/adicional, disponibilidade).
func (s *Service) PlaceOrder(ctx context.Context, slug string, in PublicOrderInput) (Order, error) {
	restaurant, accountID, err := s.loadPublicRestaurant(ctx, slug)
	if err != nil {
		return Order{}, err
	}

	orderType := strings.TrimSpace(in.Type)
	if err := validateOrderType(orderType, restaurant.Settings); err != nil {
		return Order{}, err
	}

	customerName := strings.TrimSpace(in.Customer.Name)
	customerPhone := strings.TrimSpace(in.Customer.Phone)
	if customerName == "" {
		return Order{}, ErrNameRequired
	}
	if orderType == OrderTypeDelivery && customerPhone == "" {
		return Order{}, ErrPhoneRequired
	}
	if len(in.Items) < 1 {
		return Order{}, ErrEmptyCart
	}
	if len(in.Items) > maxOrderItems {
		return Order{}, ErrValidation
	}

	items := make([]OrderItem, 0, len(in.Items))
	var subtotal int64
	for _, itemInput := range in.Items {
		item, err := s.recalcItem(ctx, restaurant.ID, itemInput)
		if err != nil {
			return Order{}, err
		}
		subtotal += item.TotalCents
		items = append(items, item)
	}

	if min := restaurant.Settings.MinOrderCents; min > 0 && subtotal < min {
		return Order{}, ErrMinOrder
	}

	// Resolve a zona de entrega (WS-A): so para entrega. Quando deliveryZoneId vem
	// preenchido, a zona TEM que existir, estar ativa e pertencer ao restaurante;
	// senao => ErrOptionInvalid. Sem zona escolhida => frete do settings (fallback).
	zone, hasZone, err := s.resolveDeliveryZone(ctx, orderType, restaurant.ID, in.DeliveryZoneID)
	if err != nil {
		return Order{}, err
	}

	deliveryFee := computeDeliveryFee(orderType, subtotal, restaurant.Settings, zone, hasZone)
	total := subtotal + deliveryFee

	// Forma de pagamento (TAVOLA): quando informada, TEM que ser uma forma que o
	// restaurante aceita (settings.payment). O troco so faz sentido em entrega +
	// dinheiro — zerado nos demais casos.
	paymentMethod, changeForCents, err := resolvePayment(in.PaymentMethod, in.ChangeForCents, orderType, restaurant.Settings)
	if err != nil {
		return Order{}, err
	}

	// Endereco de entrega: prioriza customer.address (formato do contrato); cai
	// para o campo top-level deliveryAddress (compat) e, por fim, objeto vazio.
	deliveryAddress := in.Customer.Address
	if len(deliveryAddress) == 0 {
		deliveryAddress = in.DeliveryAddress
	}
	if len(deliveryAddress) == 0 {
		deliveryAddress = json.RawMessage("{}")
	}
	// Grava o nome do bairro (zona) dentro do delivery_address jsonb, para o painel
	// exibir sem precisar de join. Falha de merge nao derruba o pedido.
	if hasZone {
		deliveryAddress = mergeNeighborhood(deliveryAddress, zone.Name)
	}

	order, err := s.store.CreateOrder(ctx, orderInsert{
		AccountID:        accountID,
		RestaurantID:     restaurant.ID,
		Type:             orderType,
		SessionID:        strings.TrimSpace(in.SessionID),
		CustomerName:     customerName,
		CustomerPhone:    customerPhone,
		DeliveryAddress:  deliveryAddress,
		PaymentMethod:    paymentMethod,
		ChangeForCents:   changeForCents,
		Notes:            strings.TrimSpace(in.Notes),
		SubtotalCents:    subtotal,
		DeliveryFeeCents: deliveryFee,
		DiscountCents:    0,
		TotalCents:       total,
		Items:            items,
	})
	if err != nil {
		return Order{}, err
	}
	return order, nil
}

// recalcItem valida e recalcula um item: produto existe/disponivel/do
// restaurante; variacao (se informada) pertence ao produto; adicionais (se
// informados) TODOS pertencem ao produto. Monta o snapshot do item.
func (s *Service) recalcItem(ctx context.Context, restaurantID string, in PublicOrderItemInput) (OrderItem, error) {
	productID := strings.TrimSpace(in.ProductID)
	if productID == "" || in.Quantity < 1 || in.Quantity > maxItemQuantity {
		return OrderItem{}, ErrValidation
	}

	product, err := s.store.productForOrder(ctx, restaurantID, productID)
	if errors.Is(err, pgx.ErrNoRows) {
		return OrderItem{}, ErrItemUnavailable
	}
	if err != nil {
		return OrderItem{}, err
	}
	if !product.IsAvailable {
		return OrderItem{}, ErrItemUnavailable
	}

	unitPrice := product.PriceCents
	variationName := ""
	if variationID := strings.TrimSpace(in.VariationID); variationID != "" {
		variation, err := s.store.variationForProduct(ctx, productID, variationID)
		if errors.Is(err, pgx.ErrNoRows) {
			return OrderItem{}, ErrOptionInvalid
		}
		if err != nil {
			return OrderItem{}, err
		}
		unitPrice += variation.PriceDeltaCents
		variationName = variation.Name
	}

	addonSnapshots := []OrderAddonSnapshot{}
	if len(in.AddonIDs) > 0 {
		addonIDs := dedupeNonEmpty(in.AddonIDs)
		addons, err := s.store.addonsForProduct(ctx, productID, addonIDs)
		if err != nil {
			return OrderItem{}, err
		}
		// Todos os adicionais informados precisam pertencer ao produto.
		if len(addons) != len(addonIDs) {
			return OrderItem{}, ErrOptionInvalid
		}
		for _, a := range addons {
			unitPrice += a.PriceCents
			addonSnapshots = append(addonSnapshots, OrderAddonSnapshot{Name: a.Name, PriceCents: a.PriceCents})
		}
	}

	productIDCopy := product.ID
	return OrderItem{
		ProductID:      &productIDCopy,
		ProductName:    product.Name,
		VariationName:  variationName,
		Addons:         addonSnapshots,
		Quantity:       in.Quantity,
		UnitPriceCents: unitPrice,
		TotalCents:     unitPrice * int64(in.Quantity),
		Notes:          strings.TrimSpace(in.Notes),
	}, nil
}

// validateOrderType garante que o tipo e valido E habilitado nas settings.
func validateOrderType(orderType string, settings Settings) error {
	switch orderType {
	case OrderTypePickup:
		if !settings.PickupEnabled {
			return ErrTypeUnavailable
		}
	case OrderTypeDelivery:
		if !settings.DeliveryEnabled {
			return ErrTypeUnavailable
		}
	case OrderTypeDineIn:
		if !settings.DineInEnabled {
			return ErrTypeUnavailable
		}
	default:
		return ErrTypeUnavailable
	}
	return nil
}

// resolvePayment normaliza e valida a forma de pagamento escolhida no checkout.
// Vazio e aceito (cliente legado/sem captura) e nao persiste troco. Quando
// informada, TEM que ser um token conhecido E aceito pelo restaurante
// (settings.payment) — senao ErrPaymentInvalid. O troco so e persistido em
// entrega + dinheiro (e > 0); nos demais casos zera (o checkout so oferece troco
// para entrega; "na mesa" nem manda forma).
func resolvePayment(method string, changeFor int64, orderType string, settings Settings) (string, int64, error) {
	normalized := strings.ToLower(strings.TrimSpace(method))
	if normalized == "" {
		return "", 0, nil
	}
	if !paymentAccepted(normalized, settings.Payment) {
		return "", 0, ErrPaymentInvalid
	}
	if normalized == PaymentMethodCash && orderType == OrderTypeDelivery && changeFor > 0 {
		return normalized, changeFor, nil
	}
	return normalized, 0, nil
}

// paymentAccepted diz se o token de pagamento esta entre as formas que o
// restaurante aceita (settings.payment).
func paymentAccepted(method string, p PaymentSettings) bool {
	switch method {
	case PaymentMethodPix:
		return p.Pix
	case PaymentMethodCash:
		return p.Cash
	case PaymentMethodDebit:
		return p.Debit.Accepted
	case PaymentMethodCredit:
		return p.Credit.Accepted
	case PaymentMethodTicket:
		return p.Ticket
	case PaymentMethodOther:
		return strings.TrimSpace(p.Other) != ""
	default:
		return false
	}
}

// resolveDeliveryZone valida a zona de entrega informada (WS-A). So tem efeito em
// entrega: para retirada/local retorna (zero, false, nil). Em entrega, se o
// deliveryZoneId vier vazio o frete cai no fallback do settings (zero, false,
// nil); se vier preenchido, a zona TEM que existir, estar ativa e pertencer ao
// restaurante — caso contrario ErrOptionInvalid.
func (s *Service) resolveDeliveryZone(ctx context.Context, orderType, restaurantID, zoneID string) (DeliveryZone, bool, error) {
	if orderType != OrderTypeDelivery {
		return DeliveryZone{}, false, nil
	}
	id := strings.TrimSpace(zoneID)
	if id == "" {
		return DeliveryZone{}, false, nil
	}
	zone, err := s.store.zoneForOrder(ctx, restaurantID, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return DeliveryZone{}, false, ErrOptionInvalid
	}
	if err != nil {
		return DeliveryZone{}, false, err
	}
	return zone, true, nil
}

// computeDeliveryFee calcula o frete: so para entrega; zera quando o subtotal
// atinge o limiar de frete gratis (quando > 0). Quando ha zona escolhida (WS-A),
// o frete base vem de zone.fee_cents; senao do settings.deliveryFeeCents.
func computeDeliveryFee(orderType string, subtotal int64, settings Settings, zone DeliveryZone, hasZone bool) int64 {
	if orderType != OrderTypeDelivery {
		return 0
	}
	if settings.FreeDeliveryAboveCents > 0 && subtotal >= settings.FreeDeliveryAboveCents {
		return 0
	}
	if hasZone {
		return zone.FeeCents
	}
	return settings.DeliveryFeeCents
}

// mergeNeighborhood injeta neighborhood (nome da zona) no delivery_address jsonb.
// Tolerante: se o raw nao for objeto valido, devolve um objeto novo so com o
// bairro; nunca derruba o pedido.
func mergeNeighborhood(raw json.RawMessage, neighborhood string) json.RawMessage {
	neighborhood = strings.TrimSpace(neighborhood)
	if neighborhood == "" {
		return raw
	}
	obj := map[string]json.RawMessage{}
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &obj)
	}
	if obj == nil {
		obj = map[string]json.RawMessage{}
	}
	name, _ := json.Marshal(neighborhood)
	obj["neighborhood"] = name
	merged, err := json.Marshal(obj)
	if err != nil {
		return raw
	}
	return merged
}

func dedupeNonEmpty(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, v := range values {
		trimmed := strings.TrimSpace(v)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		out = append(out, trimmed)
	}
	return out
}
