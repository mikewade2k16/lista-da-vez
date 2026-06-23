package cardapio

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5"
)

// gestaoFakeStore exercita as regras da Fase 9 (F1 duplicar, F2 reviews de
// estabelecimento) sem banco. GetRestaurant simula o filtro account_id (fora do
// escopo => ErrNoRows => o service mapeia para ErrNotFound). Captura os argumentos
// repassados ao store para os testes provarem o comportamento.
type gestaoFakeStore struct {
	unimplementedStore
	ownerAccount string

	dupSlug, dupName, dupSource string
	dupCalled                   bool

	createdReview ReviewInput
	createCalled  bool
	listCalled    bool
}

func (s *gestaoFakeStore) GetRestaurant(_ context.Context, accountID, _ string) (Restaurant, error) {
	if accountID != s.ownerAccount {
		return Restaurant{}, pgx.ErrNoRows
	}
	return Restaurant{ID: "rest-1"}, nil
}

func (s *gestaoFakeStore) DuplicateRestaurant(_ context.Context, accountID, sourceID, slug, name string) (Restaurant, error) {
	if accountID != s.ownerAccount {
		return Restaurant{}, pgx.ErrNoRows
	}
	s.dupCalled = true
	s.dupSource, s.dupSlug, s.dupName = sourceID, slug, name
	return Restaurant{ID: "rest-copia", Slug: slug, Name: name, IsActive: false}, nil
}

func (s *gestaoFakeStore) CreateReview(_ context.Context, accountID, _ string, in ReviewInput) (Review, error) {
	if accountID != s.ownerAccount {
		return Review{}, pgx.ErrNoRows
	}
	s.createCalled = true
	s.createdReview = in
	return Review{ID: "rev-1", ProductID: in.ProductID, ShowOnEstablishment: in.ShowOnEstablishment}, nil
}

func (s *gestaoFakeStore) ListEstablishmentReviews(_ context.Context, accountID, _ string) ([]Review, error) {
	if accountID != s.ownerAccount {
		return nil, pgx.ErrNoRows
	}
	s.listCalled = true
	return []Review{{ID: "rev-1"}}, nil
}

// ============================================================================
// F1 — Duplicar restaurante
// ============================================================================

func TestDuplicateRestaurant(t *testing.T) {
	t.Run("normaliza slug, copia source e nasce inativo", func(t *testing.T) {
		store := &gestaoFakeStore{ownerAccount: "acc-1"}
		svc := newServiceWithStore(store, ServiceConfig{})
		out, err := svc.DuplicateRestaurant(context.Background(), "acc-1", "rest-1",
			DuplicateRestaurantInput{Name: "  Copia da Mostarda ", Slug: "  Copia-Mostarda "})
		if err != nil {
			t.Fatalf("esperava sucesso, recebi %v", err)
		}
		if !store.dupCalled || store.dupSource != "rest-1" {
			t.Fatalf("duplicate nao foi chamado com o source certo: %+v", store)
		}
		if store.dupSlug != "copia-mostarda" {
			t.Fatalf("slug deveria ser normalizado (lower/trim): %q", store.dupSlug)
		}
		if store.dupName != "Copia da Mostarda" {
			t.Fatalf("name deveria ser trimado: %q", store.dupName)
		}
		if out.IsActive {
			t.Fatalf("o restaurante duplicado deve nascer inativo")
		}
	})

	t.Run("nome ou slug vazio => ErrValidation", func(t *testing.T) {
		store := &gestaoFakeStore{ownerAccount: "acc-1"}
		svc := newServiceWithStore(store, ServiceConfig{})
		if _, err := svc.DuplicateRestaurant(context.Background(), "acc-1", "rest-1",
			DuplicateRestaurantInput{Name: "X", Slug: "  "}); err != ErrValidation {
			t.Fatalf("slug vazio: esperava ErrValidation, recebi %v", err)
		}
		if _, err := svc.DuplicateRestaurant(context.Background(), "acc-1", "rest-1",
			DuplicateRestaurantInput{Name: " ", Slug: "ok"}); err != ErrValidation {
			t.Fatalf("nome vazio: esperava ErrValidation, recebi %v", err)
		}
		if store.dupCalled {
			t.Fatalf("validacao deve falhar antes de chamar o store")
		}
	})

	t.Run("source fora de escopo => ErrNotFound", func(t *testing.T) {
		store := &gestaoFakeStore{ownerAccount: "acc-dono"}
		svc := newServiceWithStore(store, ServiceConfig{})
		if _, err := svc.DuplicateRestaurant(context.Background(), "acc-intrusa", "rest-1",
			DuplicateRestaurantInput{Name: "Copia", Slug: "copia"}); err != ErrNotFound {
			t.Fatalf("source de outra account: esperava ErrNotFound, recebi %v", err)
		}
	})
}

// ============================================================================
// F2 — Avaliacoes de estabelecimento
// ============================================================================

func TestCreateEstablishmentReview(t *testing.T) {
	t.Run("forca product_id vazio (NULL) e valida rating 1-5", func(t *testing.T) {
		store := &gestaoFakeStore{ownerAccount: "acc-1"}
		svc := newServiceWithStore(store, ServiceConfig{})
		// productId no input deve ser ignorado (review de estabelecimento => NULL).
		out, err := svc.CreateEstablishmentReview(context.Background(), "acc-1", "rest-1",
			ReviewInput{ProductID: "prod-99", AuthorName: " Maria ", Rating: 5, ShowOnEstablishment: true})
		if err != nil {
			t.Fatalf("esperava sucesso, recebi %v", err)
		}
		if !store.createCalled || store.createdReview.ProductID != "" {
			t.Fatalf("review de estabelecimento deve gravar product_id vazio (NULL): %q", store.createdReview.ProductID)
		}
		if out.ProductID != "" {
			t.Fatalf("DTO de saida deveria ter productId vazio, recebi %q", out.ProductID)
		}
	})

	t.Run("rating fora de 1-5 => ErrValidation", func(t *testing.T) {
		store := &gestaoFakeStore{ownerAccount: "acc-1"}
		svc := newServiceWithStore(store, ServiceConfig{})
		if _, err := svc.CreateEstablishmentReview(context.Background(), "acc-1", "rest-1",
			ReviewInput{AuthorName: "Joao", Rating: 0}); err != ErrValidation {
			t.Fatalf("rating 0: esperava ErrValidation, recebi %v", err)
		}
		if _, err := svc.CreateEstablishmentReview(context.Background(), "acc-1", "rest-1",
			ReviewInput{AuthorName: "Joao", Rating: 6}); err != ErrValidation {
			t.Fatalf("rating 6: esperava ErrValidation, recebi %v", err)
		}
	})

	t.Run("restaurante fora de escopo => ErrNotFound", func(t *testing.T) {
		store := &gestaoFakeStore{ownerAccount: "acc-dono"}
		svc := newServiceWithStore(store, ServiceConfig{})
		if _, err := svc.CreateEstablishmentReview(context.Background(), "acc-intrusa", "rest-1",
			ReviewInput{AuthorName: "Joao", Rating: 5}); err != ErrNotFound {
			t.Fatalf("restaurante de outra account: esperava ErrNotFound, recebi %v", err)
		}
	})
}

func TestListEstablishmentReviews_ScopeValidated(t *testing.T) {
	store := &gestaoFakeStore{ownerAccount: "acc-dono"}
	svc := newServiceWithStore(store, ServiceConfig{})

	if _, err := svc.ListEstablishmentReviews(context.Background(), "acc-dono", "rest-1"); err != nil {
		t.Fatalf("account dona: esperava sucesso, recebi %v", err)
	}
	if !store.listCalled {
		t.Fatalf("list deveria ter sido chamado para a account dona")
	}
	if _, err := svc.ListEstablishmentReviews(context.Background(), "acc-intrusa", "rest-1"); err != ErrNotFound {
		t.Fatalf("account intrusa: esperava ErrNotFound, recebi %v", err)
	}
}
