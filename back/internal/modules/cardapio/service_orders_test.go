package cardapio

import (
	"context"
	"strings"
	"testing"
)

// baseStore monta um fake com 1 restaurante (entrega/retirada/local habilitados),
// 1 produto disponivel, 1 variacao e 2 adicionais — base dos testes de recalculo.
func baseStore(settings Settings) *fakeStore {
	store := newFakeStore()
	store.restaurant = Restaurant{ID: "rest-1", IsActive: true, Settings: settings}
	store.products["prod-1"] = productForOrder{ID: "prod-1", Name: "Pizza", PriceCents: 5000, IsAvailable: true}
	store.products["prod-2"] = productForOrder{ID: "prod-2", Name: "Suco", PriceCents: 1000, IsAvailable: true}
	store.variations["var-1"] = Variation{ID: "var-1", ProductID: "prod-1", Name: "Grande", PriceDeltaCents: 1500}
	store.addons["prod-1"] = []Addon{
		{ID: "add-1", ProductID: "prod-1", Name: "Borda", PriceCents: 800},
		{ID: "add-2", ProductID: "prod-1", Name: "Extra queijo", PriceCents: 500},
	}
	store.addons["prod-2"] = []Addon{{ID: "add-9", ProductID: "prod-2", Name: "Gelo", PriceCents: 100}}
	return store
}

func enabledSettings() Settings {
	return Settings{
		DeliveryFeeCents: 700,
		DeliveryEnabled:  true,
		PickupEnabled:    true,
		DineInEnabled:    true,
	}
}

// Recalculo: produto + variacao + 2 adicionais, qty 2, com frete de entrega.
func TestPlaceOrder_RecalcVariationAddonsAndFee(t *testing.T) {
	store := baseStore(enabledSettings())
	svc := newServiceWithStore(store, ServiceConfig{})

	order, err := svc.PlaceOrder(context.Background(), "slug", PublicOrderInput{
		Type:     OrderTypeDelivery,
		Customer: PublicCustomerInput{Name: "Joao", Phone: "11999"},
		Items: []PublicOrderItemInput{{
			ProductID:   "prod-1",
			VariationID: "var-1",
			AddonIDs:    []string{"add-1", "add-2"},
			Quantity:    2,
		}},
	})
	if err != nil {
		t.Fatalf("esperava sucesso, recebi %v", err)
	}
	// unit = 5000 + 1500 + 800 + 500 = 7800; total item = 15600; subtotal = 15600;
	// fee = 700; total = 16300.
	wantUnit := int64(7800)
	if order.Items[0].UnitPriceCents != wantUnit {
		t.Fatalf("unitPrice: esperava %d, recebi %d", wantUnit, order.Items[0].UnitPriceCents)
	}
	if order.SubtotalCents != 15600 {
		t.Fatalf("subtotal: esperava 15600, recebi %d", order.SubtotalCents)
	}
	if order.DeliveryFeeCents != 700 {
		t.Fatalf("fee: esperava 700, recebi %d", order.DeliveryFeeCents)
	}
	if order.TotalCents != 16300 {
		t.Fatalf("total: esperava 16300, recebi %d", order.TotalCents)
	}
	if order.Items[0].VariationName != "Grande" || len(order.Items[0].Addons) != 2 {
		t.Fatalf("snapshot do item incorreto: %+v", order.Items[0])
	}
}

// Frete gratis acima do limiar: subtotal >= freeDeliveryAboveCents zera o frete.
func TestPlaceOrder_FreeDeliveryAboveThreshold(t *testing.T) {
	settings := enabledSettings()
	settings.FreeDeliveryAboveCents = 10000
	store := baseStore(settings)
	svc := newServiceWithStore(store, ServiceConfig{})

	order, err := svc.PlaceOrder(context.Background(), "slug", PublicOrderInput{
		Type:     OrderTypeDelivery,
		Customer: PublicCustomerInput{Name: "Joao", Phone: "11999"},
		Items:    []PublicOrderItemInput{{ProductID: "prod-1", Quantity: 3}}, // 15000 > 10000
	})
	if err != nil {
		t.Fatalf("esperava sucesso, recebi %v", err)
	}
	if order.DeliveryFeeCents != 0 {
		t.Fatalf("frete gratis acima do limiar: esperava 0, recebi %d", order.DeliveryFeeCents)
	}
	if order.TotalCents != 15000 {
		t.Fatalf("total: esperava 15000, recebi %d", order.TotalCents)
	}
}

// Item indisponivel => erro.
func TestPlaceOrder_UnavailableItem(t *testing.T) {
	store := baseStore(enabledSettings())
	p := store.products["prod-1"]
	p.IsAvailable = false
	store.products["prod-1"] = p
	svc := newServiceWithStore(store, ServiceConfig{})

	_, err := svc.PlaceOrder(context.Background(), "slug", PublicOrderInput{
		Type:     OrderTypePickup,
		Customer: PublicCustomerInput{Name: "Joao"},
		Items:    []PublicOrderItemInput{{ProductID: "prod-1", Quantity: 1}},
	})
	if err != ErrItemUnavailable {
		t.Fatalf("item indisponivel: esperava ErrItemUnavailable, recebi %v", err)
	}
}

// Quantidade invalida (0 e > 50) => erro.
func TestPlaceOrder_InvalidQuantity(t *testing.T) {
	store := baseStore(enabledSettings())
	svc := newServiceWithStore(store, ServiceConfig{})

	for _, qty := range []int{0, 51} {
		_, err := svc.PlaceOrder(context.Background(), "slug", PublicOrderInput{
			Type:     OrderTypePickup,
			Customer: PublicCustomerInput{Name: "Joao"},
			Items:    []PublicOrderItemInput{{ProductID: "prod-1", Quantity: qty}},
		})
		if err != ErrValidation {
			t.Fatalf("quantidade %d: esperava ErrValidation, recebi %v", qty, err)
		}
	}
}

// Adicional de outro produto => erro (count != len).
func TestPlaceOrder_AddonFromAnotherProduct(t *testing.T) {
	store := baseStore(enabledSettings())
	svc := newServiceWithStore(store, ServiceConfig{})

	_, err := svc.PlaceOrder(context.Background(), "slug", PublicOrderInput{
		Type:     OrderTypePickup,
		Customer: PublicCustomerInput{Name: "Joao"},
		Items: []PublicOrderItemInput{{
			ProductID: "prod-1",
			AddonIDs:  []string{"add-9"}, // pertence a prod-2
			Quantity:  1,
		}},
	})
	if err != ErrOptionInvalid {
		t.Fatalf("addon de outro produto: esperava ErrOptionInvalid, recebi %v", err)
	}
}

// Variacao de outro produto => erro.
func TestPlaceOrder_VariationFromAnotherProduct(t *testing.T) {
	store := baseStore(enabledSettings())
	store.variations["var-x"] = Variation{ID: "var-x", ProductID: "prod-2", Name: "Errada"}
	svc := newServiceWithStore(store, ServiceConfig{})

	_, err := svc.PlaceOrder(context.Background(), "slug", PublicOrderInput{
		Type:     OrderTypePickup,
		Customer: PublicCustomerInput{Name: "Joao"},
		Items:    []PublicOrderItemInput{{ProductID: "prod-1", VariationID: "var-x", Quantity: 1}},
	})
	if err != ErrOptionInvalid {
		t.Fatalf("variacao de outro produto: esperava ErrOptionInvalid, recebi %v", err)
	}
}

// Tipo nao habilitado nas settings => erro.
func TestPlaceOrder_TypeNotEnabled(t *testing.T) {
	settings := enabledSettings()
	settings.DeliveryEnabled = false
	store := baseStore(settings)
	svc := newServiceWithStore(store, ServiceConfig{})

	_, err := svc.PlaceOrder(context.Background(), "slug", PublicOrderInput{
		Type:     OrderTypeDelivery,
		Customer: PublicCustomerInput{Name: "Joao", Phone: "11999"},
		Items:    []PublicOrderItemInput{{ProductID: "prod-1", Quantity: 1}},
	})
	if err != ErrTypeUnavailable {
		t.Fatalf("tipo desabilitado: esperava ErrTypeUnavailable, recebi %v", err)
	}
}

// Entrega sem telefone => erro.
func TestPlaceOrder_DeliveryRequiresPhone(t *testing.T) {
	store := baseStore(enabledSettings())
	svc := newServiceWithStore(store, ServiceConfig{})

	_, err := svc.PlaceOrder(context.Background(), "slug", PublicOrderInput{
		Type:     OrderTypeDelivery,
		Customer: PublicCustomerInput{Name: "Joao"},
		Items:    []PublicOrderItemInput{{ProductID: "prod-1", Quantity: 1}},
	})
	if err != ErrPhoneRequired {
		t.Fatalf("entrega sem telefone: esperava ErrPhoneRequired, recebi %v", err)
	}
}

// Subtotal abaixo do minimo configurado => erro.
func TestPlaceOrder_BelowMinOrder(t *testing.T) {
	settings := enabledSettings()
	settings.MinOrderCents = 20000
	store := baseStore(settings)
	svc := newServiceWithStore(store, ServiceConfig{})

	_, err := svc.PlaceOrder(context.Background(), "slug", PublicOrderInput{
		Type:     OrderTypePickup,
		Customer: PublicCustomerInput{Name: "Joao"},
		Items:    []PublicOrderItemInput{{ProductID: "prod-1", Quantity: 1}}, // 5000 < 20000
	})
	if err != ErrMinOrder {
		t.Fatalf("abaixo do minimo: esperava ErrMinOrder, recebi %v", err)
	}
}

// WS-A: frete vem da zona de entrega escolhida (zone.fee_cents), nao do settings.
func TestPlaceOrder_DeliveryFeeFromZone(t *testing.T) {
	store := baseStore(enabledSettings()) // settings.DeliveryFeeCents = 700
	store.zones["zone-1"] = DeliveryZone{ID: "zone-1", RestaurantID: "rest-1", Name: "Centro", FeeCents: 1500, IsActive: true}
	svc := newServiceWithStore(store, ServiceConfig{})

	order, err := svc.PlaceOrder(context.Background(), "slug", PublicOrderInput{
		Type:           OrderTypeDelivery,
		Customer:       PublicCustomerInput{Name: "Joao", Phone: "11999"},
		DeliveryZoneID: "zone-1",
		Items:          []PublicOrderItemInput{{ProductID: "prod-1", Quantity: 1}}, // 5000
	})
	if err != nil {
		t.Fatalf("esperava sucesso, recebi %v", err)
	}
	if order.DeliveryFeeCents != 1500 {
		t.Fatalf("frete da zona: esperava 1500, recebi %d", order.DeliveryFeeCents)
	}
	if order.TotalCents != 6500 {
		t.Fatalf("total: esperava 6500, recebi %d", order.TotalCents)
	}
	// O nome do bairro deve ter sido gravado no delivery_address jsonb.
	if !strings.Contains(string(store.createdOrder.DeliveryAddress), "Centro") {
		t.Fatalf("esperava neighborhood no delivery_address, recebi %s", store.createdOrder.DeliveryAddress)
	}
}

// WS-A: zona inexistente/de outro restaurante/inativa => ErrOptionInvalid.
func TestPlaceOrder_InvalidZone(t *testing.T) {
	store := baseStore(enabledSettings())
	store.zones["zone-other"] = DeliveryZone{ID: "zone-other", RestaurantID: "rest-2", Name: "Outro", FeeCents: 999, IsActive: true}
	store.zones["zone-off"] = DeliveryZone{ID: "zone-off", RestaurantID: "rest-1", Name: "Inativa", FeeCents: 999, IsActive: false}
	svc := newServiceWithStore(store, ServiceConfig{})

	for _, zoneID := range []string{"zone-x", "zone-other", "zone-off"} {
		_, err := svc.PlaceOrder(context.Background(), "slug", PublicOrderInput{
			Type:           OrderTypeDelivery,
			Customer:       PublicCustomerInput{Name: "Joao", Phone: "11999"},
			DeliveryZoneID: zoneID,
			Items:          []PublicOrderItemInput{{ProductID: "prod-1", Quantity: 1}},
		})
		if err != ErrOptionInvalid {
			t.Fatalf("zona %q invalida: esperava ErrOptionInvalid, recebi %v", zoneID, err)
		}
	}
}

// WS-A: frete gratis acima do limiar zera mesmo com zona escolhida.
func TestPlaceOrder_FreeDeliveryWithZone(t *testing.T) {
	settings := enabledSettings()
	settings.FreeDeliveryAboveCents = 10000
	store := baseStore(settings)
	store.zones["zone-1"] = DeliveryZone{ID: "zone-1", RestaurantID: "rest-1", Name: "Centro", FeeCents: 1500, IsActive: true}
	svc := newServiceWithStore(store, ServiceConfig{})

	order, err := svc.PlaceOrder(context.Background(), "slug", PublicOrderInput{
		Type:           OrderTypeDelivery,
		Customer:       PublicCustomerInput{Name: "Joao", Phone: "11999"},
		DeliveryZoneID: "zone-1",
		Items:          []PublicOrderItemInput{{ProductID: "prod-1", Quantity: 3}}, // 15000 > 10000
	})
	if err != nil {
		t.Fatalf("esperava sucesso, recebi %v", err)
	}
	if order.DeliveryFeeCents != 0 {
		t.Fatalf("frete gratis com zona: esperava 0, recebi %d", order.DeliveryFeeCents)
	}
	if order.TotalCents != 15000 {
		t.Fatalf("total: esperava 15000, recebi %d", order.TotalCents)
	}
}

// Pedido por retirada nao cobra frete mesmo com fee configurada.
func TestPlaceOrder_PickupNoFee(t *testing.T) {
	store := baseStore(enabledSettings())
	svc := newServiceWithStore(store, ServiceConfig{})

	order, err := svc.PlaceOrder(context.Background(), "slug", PublicOrderInput{
		Type:     OrderTypePickup,
		Customer: PublicCustomerInput{Name: "Joao"},
		Items:    []PublicOrderItemInput{{ProductID: "prod-1", Quantity: 1}},
	})
	if err != nil {
		t.Fatalf("esperava sucesso, recebi %v", err)
	}
	if order.DeliveryFeeCents != 0 || order.TotalCents != 5000 {
		t.Fatalf("retirada: esperava fee 0 / total 5000, recebi fee %d total %d", order.DeliveryFeeCents, order.TotalCents)
	}
}

// resolvePayment: token vazio passa; token aceito normaliza; token nao aceito ou
// desconhecido => ErrPaymentInvalid; troco so sobrevive em entrega + dinheiro.
func TestResolvePayment(t *testing.T) {
	full := PaymentSettings{
		Cash:   true,
		Pix:    true,
		Debit:  PaymentCard{Accepted: true},
		Credit: PaymentCard{Accepted: false},
		Ticket: false,
		Other:  "Vale-refeicao",
	}
	settings := Settings{Payment: full}

	cases := []struct {
		name       string
		method     string
		changeFor  int64
		orderType  string
		wantMethod string
		wantChange int64
		wantErr    error
	}{
		{"vazio", "", 0, OrderTypeDelivery, "", 0, nil},
		{"pix aceito", "PIX", 0, OrderTypePickup, "pix", 0, nil},
		{"other aceito (other != \"\")", "other", 0, OrderTypeDelivery, "other", 0, nil},
		{"credito nao aceito", "credit", 0, OrderTypeDelivery, "", 0, ErrPaymentInvalid},
		{"token desconhecido", "boleto", 0, OrderTypeDelivery, "", 0, ErrPaymentInvalid},
		{"troco em entrega+dinheiro", "cash", 5000, OrderTypeDelivery, "cash", 5000, nil},
		{"troco ignorado em retirada", "cash", 5000, OrderTypePickup, "cash", 0, nil},
		{"troco so vale para dinheiro", "pix", 5000, OrderTypeDelivery, "pix", 0, nil},
		{"troco zero nao persiste", "cash", 0, OrderTypeDelivery, "cash", 0, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			method, change, err := resolvePayment(tc.method, tc.changeFor, tc.orderType, settings)
			if err != tc.wantErr {
				t.Fatalf("erro: esperava %v, recebi %v", tc.wantErr, err)
			}
			if method != tc.wantMethod {
				t.Fatalf("method: esperava %q, recebi %q", tc.wantMethod, method)
			}
			if change != tc.wantChange {
				t.Fatalf("change: esperava %d, recebi %d", tc.wantChange, change)
			}
		})
	}
}
