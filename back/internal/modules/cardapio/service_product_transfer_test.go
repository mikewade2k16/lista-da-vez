package cardapio

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5"
)

type productTransferFakeStore struct {
	unimplementedStore
	accountID  string
	categories []Category
	products   []Product
	bulkIDs    []string
	bulkAction ProductBulkAction
}

func (s *productTransferFakeStore) GetRestaurant(_ context.Context, accountID, restaurantID string) (Restaurant, error) {
	if accountID != s.accountID || restaurantID != "rest-1" {
		return Restaurant{}, pgx.ErrNoRows
	}
	return Restaurant{ID: restaurantID}, nil
}

func (s *productTransferFakeStore) ListCategories(_ context.Context, accountID, restaurantID string, _ bool) ([]Category, error) {
	if accountID != s.accountID || restaurantID != "rest-1" {
		return nil, pgx.ErrNoRows
	}
	return append([]Category(nil), s.categories...), nil
}

func (s *productTransferFakeStore) CreateCategory(_ context.Context, accountID, restaurantID string, in CategoryInput) (Category, error) {
	if accountID != s.accountID || restaurantID != "rest-1" {
		return Category{}, pgx.ErrNoRows
	}
	category := Category{
		ID:           "cat-" + in.Slug,
		RestaurantID: restaurantID,
		Slug:         in.Slug,
		Name:         in.Name,
		SortOrder:    in.SortOrder,
		IsActive:     in.IsActive,
	}
	s.categories = append(s.categories, category)
	return category, nil
}

func (s *productTransferFakeStore) ListProductsLean(_ context.Context, accountID, restaurantID string) ([]ProductLean, error) {
	if accountID != s.accountID || restaurantID != "rest-1" {
		return nil, pgx.ErrNoRows
	}
	items := make([]ProductLean, len(s.products))
	for i, product := range s.products {
		items[i] = ProductLean{ID: product.ID, Slug: product.Slug, Name: product.Name}
	}
	return items, nil
}

func (s *productTransferFakeStore) ListProductsFull(_ context.Context, accountID, restaurantID string) ([]Product, error) {
	if accountID != s.accountID || restaurantID != "rest-1" {
		return nil, pgx.ErrNoRows
	}
	return append([]Product(nil), s.products...), nil
}

func (s *productTransferFakeStore) CreateProduct(_ context.Context, accountID, restaurantID string, in ProductInput) (Product, error) {
	if accountID != s.accountID || restaurantID != "rest-1" {
		return Product{}, pgx.ErrNoRows
	}
	product := Product{
		ID:           "prod-" + in.Slug,
		RestaurantID: restaurantID,
		CategoryID:   in.CategoryID,
		Slug:         in.Slug,
		Name:         in.Name,
		PriceCents:   in.PriceCents,
	}
	s.products = append(s.products, product)
	return product, nil
}

func (s *productTransferFakeStore) UpdateProduct(_ context.Context, accountID, id string, in ProductInput) (Product, error) {
	if accountID != s.accountID {
		return Product{}, pgx.ErrNoRows
	}
	for i := range s.products {
		if s.products[i].ID != id {
			continue
		}
		s.products[i].CategoryID = in.CategoryID
		s.products[i].Slug = in.Slug
		s.products[i].Name = in.Name
		s.products[i].PriceCents = in.PriceCents
		return s.products[i], nil
	}
	return Product{}, pgx.ErrNoRows
}

func (s *productTransferFakeStore) BulkProducts(_ context.Context, accountID, restaurantID string, ids []string, action ProductBulkAction) (int, error) {
	if accountID != s.accountID || restaurantID != "rest-1" {
		return 0, pgx.ErrNoRows
	}
	s.bulkIDs = append([]string(nil), ids...)
	s.bulkAction = action
	return len(ids), nil
}

func TestBulkProductsValidatesAndScopesSelection(t *testing.T) {
	store := &productTransferFakeStore{accountID: "acc-1"}
	svc := newServiceWithStore(store, ServiceConfig{})

	result, err := svc.BulkProducts(context.Background(), "acc-1", "rest-1", ProductBulkInput{
		IDs:    []string{"prod-1", "prod-2"},
		Action: ProductBulkDisable,
	})
	if err != nil {
		t.Fatalf("esperava sucesso, recebi %v", err)
	}
	if result.Affected != 2 || store.bulkAction != ProductBulkDisable {
		t.Fatalf("acao em lote incorreta: result=%+v action=%q", result, store.bulkAction)
	}

	_, err = svc.BulkProducts(context.Background(), "acc-1", "rest-1", ProductBulkInput{
		IDs:    []string{"prod-1", "prod-1"},
		Action: ProductBulkDelete,
	})
	if !errors.Is(err, ErrValidation) {
		t.Fatalf("IDs duplicados deveriam falhar com ErrValidation, recebi %v", err)
	}

	_, err = svc.BulkProducts(context.Background(), "acc-2", "rest-1", ProductBulkInput{
		IDs:    []string{"prod-1"},
		Action: ProductBulkDelete,
	})
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("account fora do escopo deveria virar 404, recebi %v", err)
	}
}

func TestExportProductsUsesPortableCategoryAndOptions(t *testing.T) {
	categoryID := "cat-1"
	store := &productTransferFakeStore{
		accountID:  "acc-1",
		categories: []Category{{ID: categoryID, Slug: "entradas", Name: "Entradas"}},
		products: []Product{{
			ID:           "prod-1",
			RestaurantID: "rest-1",
			CategoryID:   &categoryID,
			Slug:         "bolinho",
			Name:         "Bolinho",
			PriceCents:   4200,
			Variations:   []Variation{{Name: "Grande", PriceDeltaCents: 500}},
			Addons:       []Addon{{Name: "Molho", PriceCents: 200}},
		}},
	}
	svc := newServiceWithStore(store, ServiceConfig{})

	document, err := svc.ExportProducts(context.Background(), "acc-1", "rest-1")
	if err != nil {
		t.Fatalf("esperava sucesso, recebi %v", err)
	}
	if document.Version != 1 || len(document.Products) != 1 {
		t.Fatalf("documento inesperado: %+v", document)
	}
	item := document.Products[0]
	if item.CategorySlug != "entradas" || item.CategoryName != "Entradas" {
		t.Fatalf("categoria portavel ausente: %+v", item)
	}
	if len(item.Variations) != 1 || len(item.Addons) != 1 {
		t.Fatalf("opcoes nao foram exportadas: %+v", item)
	}
}

func TestImportProductsCreatesCategoriesAndUpsertsBySlug(t *testing.T) {
	store := &productTransferFakeStore{
		accountID: "acc-1",
		products:  []Product{{ID: "prod-existing", RestaurantID: "rest-1", Slug: "cafe", Name: "Cafe antigo"}},
	}
	svc := newServiceWithStore(store, ServiceConfig{})

	result, err := svc.ImportProducts(context.Background(), "acc-1", "rest-1", ProductImportInput{
		UpdateExisting:        true,
		AcceptedCategorySlugs: []string{"bebidas"},
		Products: []ProductTransferItem{
			{
				CategorySlug: "bebidas",
				CategoryName: "Bebidas",
				ProductInput: ProductInput{Slug: "cafe", Name: "Cafe novo", PriceCents: 900},
			},
			{
				CategorySlug: "bebidas",
				CategoryName: "Bebidas",
				ProductInput: ProductInput{Slug: "cha", Name: "Cha", PriceCents: 700},
			},
		},
	})
	if err != nil {
		t.Fatalf("esperava sucesso, recebi %v", err)
	}
	if result.Updated != 1 || result.Created != 1 || result.Failed != 0 {
		t.Fatalf("resultado inesperado: %+v", result)
	}
	if len(store.categories) != 1 || store.categories[0].Slug != "bebidas" {
		t.Fatalf("categoria ausente: %+v", store.categories)
	}
	if store.products[0].Name != "Cafe novo" || len(store.products) != 2 {
		t.Fatalf("upsert incorreto: %+v", store.products)
	}
}

func TestPreviewProductImportReportsOnlyMissingCategories(t *testing.T) {
	store := &productTransferFakeStore{
		accountID:  "acc-1",
		categories: []Category{{ID: "cat-existing", Slug: "entradas", Name: "Entradas"}},
	}
	svc := newServiceWithStore(store, ServiceConfig{})

	preview, err := svc.PreviewProductImport(context.Background(), "acc-1", "rest-1", ProductImportInput{
		Products: []ProductTransferItem{
			{CategorySlug: "entradas", CategoryName: "Entradas"},
			{CategorySlug: "bebidas", CategoryName: "Bebidas"},
			{CategorySlug: " Bebidas ", CategoryName: "Bebidas atualizadas"},
			{CategorySlug: "", CategoryName: "Sem categoria"},
		},
	})
	if err != nil {
		t.Fatalf("esperava sucesso, recebi %v", err)
	}
	if len(preview.NewCategories) != 1 {
		t.Fatalf("preview inesperado: %+v", preview)
	}
	category := preview.NewCategories[0]
	if category.Slug != "bebidas" || category.Name != "Bebidas" || category.ProductCount != 2 {
		t.Fatalf("categoria nova inesperada: %+v", category)
	}
}

func TestImportProductsRequiresExplicitCategoryAcceptance(t *testing.T) {
	store := &productTransferFakeStore{accountID: "acc-1"}
	svc := newServiceWithStore(store, ServiceConfig{})
	input := ProductImportInput{Products: []ProductTransferItem{{
		CategorySlug: "bebidas",
		CategoryName: "Bebidas",
		ProductInput: ProductInput{Slug: "cafe", Name: "Cafe", PriceCents: 900},
	}}}

	_, err := svc.ImportProducts(context.Background(), "acc-1", "rest-1", input)
	if !errors.Is(err, ErrValidation) {
		t.Fatalf("categoria nova sem aceite deveria falhar, recebi %v", err)
	}
	if len(store.categories) != 0 || len(store.products) != 0 {
		t.Fatalf("preview pendente nao pode escrever: categories=%+v products=%+v", store.categories, store.products)
	}

	input.AcceptedCategorySlugs = []string{"bebidas"}
	result, err := svc.ImportProducts(context.Background(), "acc-1", "rest-1", input)
	if err != nil || result.Created != 1 {
		t.Fatalf("categoria aceita deveria importar: result=%+v err=%v", result, err)
	}
}

func TestImportProductsReportsInvalidRowsWithoutWritingThem(t *testing.T) {
	store := &productTransferFakeStore{accountID: "acc-1"}
	svc := newServiceWithStore(store, ServiceConfig{})

	result, err := svc.ImportProducts(context.Background(), "acc-1", "rest-1", ProductImportInput{
		AcceptedCategorySlugs: []string{"nova"},
		Products: []ProductTransferItem{
			{ProductInput: ProductInput{Slug: "sem-nome", PriceCents: 100}},
			{CategorySlug: "nova", ProductInput: ProductInput{Slug: "valido", Name: "Valido", PriceCents: 100}},
		},
	})
	if err != nil {
		t.Fatalf("erros por linha devem voltar no resultado, recebi %v", err)
	}
	if result.Failed != 2 || len(store.products) != 0 {
		t.Fatalf("linhas invalidas nao deveriam ser persistidas: result=%+v products=%+v", result, store.products)
	}
}
