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

	deliveryFee := computeDeliveryFee(orderType, subtotal, restaurant.Settings)
	total := subtotal + deliveryFee

	// Endereco de entrega: prioriza customer.address (formato do contrato); cai
	// para o campo top-level deliveryAddress (compat) e, por fim, objeto vazio.
	deliveryAddress := in.Customer.Address
	if len(deliveryAddress) == 0 {
		deliveryAddress = in.DeliveryAddress
	}
	if len(deliveryAddress) == 0 {
		deliveryAddress = json.RawMessage("{}")
	}

	order, err := s.store.CreateOrder(ctx, orderInsert{
		AccountID:        accountID,
		RestaurantID:     restaurant.ID,
		Type:             orderType,
		SessionID:        strings.TrimSpace(in.SessionID),
		CustomerName:     customerName,
		CustomerPhone:    customerPhone,
		DeliveryAddress:  deliveryAddress,
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

// computeDeliveryFee calcula o frete: so para entrega; zera quando o subtotal
// atinge o limiar de frete gratis (quando > 0).
func computeDeliveryFee(orderType string, subtotal int64, settings Settings) int64 {
	if orderType != OrderTypeDelivery {
		return 0
	}
	if settings.FreeDeliveryAboveCents > 0 && subtotal >= settings.FreeDeliveryAboveCents {
		return 0
	}
	return settings.DeliveryFeeCents
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
