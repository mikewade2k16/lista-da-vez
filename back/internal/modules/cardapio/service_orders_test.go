package cardapio

import (
	"context"
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
	if err != ErrValidation {
		t.Fatalf("item indisponivel: esperava ErrValidation, recebi %v", err)
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
	if err != ErrValidation {
		t.Fatalf("addon de outro produto: esperava ErrValidation, recebi %v", err)
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
	if err != ErrValidation {
		t.Fatalf("variacao de outro produto: esperava ErrValidation, recebi %v", err)
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
	if err != ErrValidation {
		t.Fatalf("tipo desabilitado: esperava ErrValidation, recebi %v", err)
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
	if err != ErrValidation {
		t.Fatalf("entrega sem telefone: esperava ErrValidation, recebi %v", err)
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
	if err != ErrValidation {
		t.Fatalf("abaixo do minimo: esperava ErrValidation, recebi %v", err)
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
