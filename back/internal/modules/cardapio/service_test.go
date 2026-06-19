package cardapio

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/jackc/pgx/v5"
)

// fakeStore implementa dataStore para testes. Os metodos nao usados em um teste
// permanecem com o comportamento default da base (retornam zero/ErrNoRows). Cada
// teste preenche apenas os campos do fluxo que exercita.
type fakeStore struct {
	unimplementedStore

	restaurant     Restaurant
	restaurantAcc  string
	restaurantErr  error
	products       map[string]productForOrder // productID -> dados
	variations     map[string]Variation       // variationID -> dados (com ProductID)
	addons         map[string][]Addon         // productID -> addons
	zones          map[string]DeliveryZone    // zoneID -> dados (com RestaurantID, so ativas)
	resolveSlug    map[string]string          // slug pedido -> slug canonico
	resolveHost    map[string]string          // host -> slug
	createdOrder   orderInsert
	insertedEvents []string
}

func newFakeStore() *fakeStore {
	return &fakeStore{
		products:    map[string]productForOrder{},
		variations:  map[string]Variation{},
		addons:      map[string][]Addon{},
		zones:       map[string]DeliveryZone{},
		resolveSlug: map[string]string{},
		resolveHost: map[string]string{},
	}
}

func (f *fakeStore) zoneForOrder(_ context.Context, restaurantID, zoneID string) (DeliveryZone, error) {
	z, ok := f.zones[zoneID]
	if !ok || z.RestaurantID != restaurantID || !z.IsActive {
		return DeliveryZone{}, pgx.ErrNoRows
	}
	return z, nil
}

func (f *fakeStore) publicRestaurant(_ context.Context, _ string) (Restaurant, string, error) {
	if f.restaurantErr != nil {
		return Restaurant{}, "", f.restaurantErr
	}
	acc := f.restaurantAcc
	if acc == "" {
		acc = "acc-1"
	}
	return f.restaurant, acc, nil
}

func (f *fakeStore) productForOrder(_ context.Context, restaurantID, productID string) (productForOrder, error) {
	p, ok := f.products[productID]
	if !ok {
		return productForOrder{}, pgx.ErrNoRows
	}
	return p, nil
}

func (f *fakeStore) variationForProduct(_ context.Context, productID, variationID string) (Variation, error) {
	v, ok := f.variations[variationID]
	if !ok || v.ProductID != productID {
		return Variation{}, pgx.ErrNoRows
	}
	return v, nil
}

func (f *fakeStore) addonsForProduct(_ context.Context, productID string, addonIDs []string) ([]Addon, error) {
	want := map[string]struct{}{}
	for _, id := range addonIDs {
		want[id] = struct{}{}
	}
	out := []Addon{}
	for _, a := range f.addons[productID] {
		if _, ok := want[a.ID]; ok {
			out = append(out, a)
		}
	}
	return out, nil
}

func (f *fakeStore) CreateOrder(_ context.Context, in orderInsert) (Order, error) {
	f.createdOrder = in
	return Order{
		ID:               "order-1",
		RestaurantID:     in.RestaurantID,
		OrderNumber:      1,
		Status:           OrderStatusReceived,
		Type:             in.Type,
		CustomerName:     in.CustomerName,
		CustomerPhone:    in.CustomerPhone,
		SubtotalCents:    in.SubtotalCents,
		DeliveryFeeCents: in.DeliveryFeeCents,
		TotalCents:       in.TotalCents,
		Items:            in.Items,
	}, nil
}

func (f *fakeStore) resolveSlugActive(_ context.Context, slug string) (string, error) {
	if canonical, ok := f.resolveSlug[slug]; ok {
		return canonical, nil
	}
	return "", pgx.ErrNoRows
}

func (f *fakeStore) resolveHostActive(_ context.Context, host string) (string, error) {
	if slug, ok := f.resolveHost[host]; ok {
		return slug, nil
	}
	return "", pgx.ErrNoRows
}

func (f *fakeStore) InsertEvent(_ context.Context, _, _, name, _ string, _ json.RawMessage) error {
	f.insertedEvents = append(f.insertedEvents, name)
	return nil
}

// ============================================================================
// Resolve por host
// ============================================================================

func TestResolve_Subdomain(t *testing.T) {
	store := newFakeStore()
	store.resolveSlug["tavola"] = "tavola"
	svc := newServiceWithStore(store, ServiceConfig{BaseDomain: "tavola.app"})

	slug, err := svc.Resolve(context.Background(), "tavola.tavola.app")
	if err != nil || slug != "tavola" {
		t.Fatalf("subdominio: esperava slug 'tavola', recebi %q err=%v", slug, err)
	}
}

func TestResolve_CustomDomain(t *testing.T) {
	store := newFakeStore()
	store.resolveHost["restaurante.com.br"] = "bistro"
	svc := newServiceWithStore(store, ServiceConfig{BaseDomain: "tavola.app"})

	slug, err := svc.Resolve(context.Background(), "www.restaurante.com.br:8080")
	if err != nil || slug != "bistro" {
		t.Fatalf("dominio custom (www/porta removidos): esperava 'bistro', recebi %q err=%v", slug, err)
	}
}

func TestResolve_LocalhostDevDefault(t *testing.T) {
	store := newFakeStore()
	store.resolveSlug["dev-slug"] = "dev-slug"
	svc := newServiceWithStore(store, ServiceConfig{DevDefaultSlug: "dev-slug"})

	slug, err := svc.Resolve(context.Background(), "localhost:3000")
	if err != nil || slug != "dev-slug" {
		t.Fatalf("localhost dev default: esperava 'dev-slug', recebi %q err=%v", slug, err)
	}
}

func TestResolve_NotFound(t *testing.T) {
	store := newFakeStore()
	svc := newServiceWithStore(store, ServiceConfig{BaseDomain: "tavola.app"})

	if _, err := svc.Resolve(context.Background(), "inexistente.com"); err != ErrNotFound {
		t.Fatalf("host inexistente: esperava ErrNotFound, recebi %v", err)
	}
}

// ============================================================================
// Allowlist de eventos
// ============================================================================

func TestRecordEvent_AllowlistRejectsUnknown(t *testing.T) {
	store := newFakeStore()
	store.restaurant = Restaurant{ID: "rest-1", IsActive: true}
	svc := newServiceWithStore(store, ServiceConfig{})

	if err := svc.RecordEvent(context.Background(), "slug", PublicEventInput{Name: "hack_event"}); err != ErrValidation {
		t.Fatalf("evento fora da allowlist: esperava ErrValidation, recebi %v", err)
	}
	if err := svc.RecordEvent(context.Background(), "slug", PublicEventInput{Name: "add_to_cart"}); err != nil {
		t.Fatalf("evento valido: esperava nil, recebi %v", err)
	}
	if len(store.insertedEvents) != 1 || store.insertedEvents[0] != "add_to_cart" {
		t.Fatalf("evento valido nao foi gravado: %v", store.insertedEvents)
	}
}

func TestRecordEvent_ContextTooLarge(t *testing.T) {
	store := newFakeStore()
	store.restaurant = Restaurant{ID: "rest-1", IsActive: true}
	svc := newServiceWithStore(store, ServiceConfig{})

	big := make([]byte, 9*1024)
	for i := range big {
		big[i] = 'a'
	}
	ctx := json.RawMessage(`"` + string(big) + `"`)
	if err := svc.RecordEvent(context.Background(), "slug", PublicEventInput{Name: "page_view", Context: ctx}); err != ErrValidation {
		t.Fatalf("context > 8KB: esperava ErrValidation, recebi %v", err)
	}
}

// ============================================================================
// Helpers puros
// ============================================================================

// ============================================================================
// Escopo multitenant — recurso fora do escopo => 404 uniforme
// ============================================================================

// scopeFakeStore devolve ErrNoRows quando o accountID nao bate (simula o filtro
// account_id = $1 da query real). O service deve mapear para ErrNotFound (404),
// nunca 403 — sem revelar a existencia do recurso de outra account.
type scopeFakeStore struct {
	unimplementedStore
	ownerAccount string
	restaurant   Restaurant
}

func (s *scopeFakeStore) GetRestaurant(_ context.Context, accountID, _ string) (Restaurant, error) {
	if accountID != s.ownerAccount {
		return Restaurant{}, pgx.ErrNoRows
	}
	return s.restaurant, nil
}

func TestService_OutOfScopeReturnsNotFound(t *testing.T) {
	store := &scopeFakeStore{ownerAccount: "acc-dono", restaurant: Restaurant{ID: "rest-1"}}
	svc := newServiceWithStore(store, ServiceConfig{})

	// Account dona enxerga.
	if _, err := svc.GetRestaurant(context.Background(), "acc-dono", "rest-1"); err != nil {
		t.Fatalf("account dona: esperava sucesso, recebi %v", err)
	}
	// Outra account => 404 uniforme (ErrNotFound), nunca vazar 403/existencia.
	if _, err := svc.GetRestaurant(context.Background(), "acc-intrusa", "rest-1"); err != ErrNotFound {
		t.Fatalf("account intrusa: esperava ErrNotFound, recebi %v", err)
	}
}

// ============================================================================
// Mover restaurante de conta (coluna Cliente, espelha bio)
// ============================================================================

// moveFakeStore captura o ponteiro de account_id repassado ao UpdateRestaurant e
// controla a existencia da conta destino (AccountExists).
type moveFakeStore struct {
	unimplementedStore
	exists     bool
	gotAccount *string
}

func (s *moveFakeStore) AccountExists(_ context.Context, _ string) (bool, error) {
	return s.exists, nil
}

func (s *moveFakeStore) UpdateRestaurant(_ context.Context, _, _ string, in UpdateRestaurantInput) (Restaurant, error) {
	s.gotAccount = in.AccountID
	return Restaurant{ID: "rest-1"}, nil
}

func ptr(v string) *string { return &v }

func TestUpdateRestaurant_MoveAccount(t *testing.T) {
	const current = "acc-atual"

	// Admin move para conta destino valida: o ponteiro chega normalizado ao store.
	t.Run("conta valida move", func(t *testing.T) {
		store := &moveFakeStore{exists: true}
		svc := newServiceWithStore(store, ServiceConfig{})
		if _, err := svc.UpdateRestaurant(context.Background(), current, "rest-1",
			UpdateRestaurantInput{AccountID: ptr(" acc-destino ")}); err != nil {
			t.Fatalf("esperava sucesso, recebi %v", err)
		}
		if store.gotAccount == nil || *store.gotAccount != "acc-destino" {
			t.Fatalf("esperava account_id 'acc-destino' no store, recebi %v", store.gotAccount)
		}
	})

	// Conta destino inexistente => ErrNotFound antes do update.
	t.Run("conta inexistente", func(t *testing.T) {
		store := &moveFakeStore{exists: false}
		svc := newServiceWithStore(store, ServiceConfig{})
		if _, err := svc.UpdateRestaurant(context.Background(), current, "rest-1",
			UpdateRestaurantInput{AccountID: ptr("acc-fantasma")}); err != ErrNotFound {
			t.Fatalf("esperava ErrNotFound, recebi %v", err)
		}
		if store.gotAccount != nil {
			t.Fatalf("update nao deveria ter ocorrido, recebi %v", store.gotAccount)
		}
	})

	// Conta igual a atual (ou vazia) => nil (nao move), sem consultar AccountExists.
	t.Run("mesma conta nao move", func(t *testing.T) {
		store := &moveFakeStore{exists: false}
		svc := newServiceWithStore(store, ServiceConfig{})
		if _, err := svc.UpdateRestaurant(context.Background(), current, "rest-1",
			UpdateRestaurantInput{AccountID: ptr(current)}); err != nil {
			t.Fatalf("esperava sucesso, recebi %v", err)
		}
		if store.gotAccount != nil {
			t.Fatalf("mesma conta nao deve mover, recebi %v", store.gotAccount)
		}
	})

	// Sem accountId (nao-admin chega assim, zerado no handler) => nao move.
	t.Run("nil nao move", func(t *testing.T) {
		store := &moveFakeStore{exists: false}
		svc := newServiceWithStore(store, ServiceConfig{})
		if _, err := svc.UpdateRestaurant(context.Background(), current, "rest-1",
			UpdateRestaurantInput{}); err != nil {
			t.Fatalf("esperava sucesso, recebi %v", err)
		}
		if store.gotAccount != nil {
			t.Fatalf("sem accountId nao deve mover, recebi %v", store.gotAccount)
		}
	})
}

func TestNormalizeHost(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"WWW.Exemplo.COM:443", "exemplo.com"},
		{"sub.exemplo.com", "sub.exemplo.com"},
		{"localhost:3000", "localhost"},
		{" exemplo.com/path ", "exemplo.com"}, // espacos + path + porta removidos
		{"www.loja.app", "loja.app"},
	}
	for _, c := range cases {
		if got := normalizeHost(c.in); got != c.want {
			t.Fatalf("normalizeHost(%q) = %q, esperava %q", c.in, got, c.want)
		}
	}
}
