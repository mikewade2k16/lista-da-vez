package cardapio

import (
	"context"
	"encoding/json"
)

// unimplementedStore fornece defaults inertes para todos os metodos de dataStore.
// Os fakes embutem este tipo e sobrescrevem apenas os metodos do fluxo testado.
// Metodos nao sobrescritos retornam zero-value (sem panic) para nao quebrar
// caminhos colaterais nos testes.
type unimplementedStore struct{}

func (unimplementedStore) CreateRestaurant(context.Context, string, string, string) (Restaurant, error) {
	return Restaurant{}, nil
}
func (unimplementedStore) GetRestaurant(context.Context, string, string) (Restaurant, error) {
	return Restaurant{}, nil
}
func (unimplementedStore) ListRestaurantsLean(context.Context, string, string) ([]RestaurantLean, error) {
	return nil, nil
}
func (unimplementedStore) UpdateRestaurant(context.Context, string, string, UpdateRestaurantInput) (Restaurant, error) {
	return Restaurant{}, nil
}
func (unimplementedStore) DeleteRestaurant(context.Context, string, string) error { return nil }
func (unimplementedStore) ListDomains(context.Context, string, string) ([]Domain, error) {
	return nil, nil
}
func (unimplementedStore) CreateDomain(context.Context, string, string, string, bool) (Domain, error) {
	return Domain{}, nil
}
func (unimplementedStore) DeleteDomain(context.Context, string, string) error { return nil }
func (unimplementedStore) ListCategories(context.Context, string, string, bool) ([]Category, error) {
	return nil, nil
}
func (unimplementedStore) CreateCategory(context.Context, string, string, CategoryInput) (Category, error) {
	return Category{}, nil
}
func (unimplementedStore) UpdateCategory(context.Context, string, string, CategoryInput) (Category, error) {
	return Category{}, nil
}
func (unimplementedStore) DeleteCategory(context.Context, string, string) error { return nil }
func (unimplementedStore) ListProductsLean(context.Context, string, string) ([]ProductLean, error) {
	return nil, nil
}
func (unimplementedStore) GetProduct(context.Context, string, string) (Product, error) {
	return Product{}, nil
}
func (unimplementedStore) ListMenuProducts(context.Context, string, string) ([]Product, error) {
	return nil, nil
}
func (unimplementedStore) CreateProduct(context.Context, string, string, ProductInput) (Product, error) {
	return Product{}, nil
}
func (unimplementedStore) UpdateProduct(context.Context, string, string, ProductInput) (Product, error) {
	return Product{}, nil
}
func (unimplementedStore) DeleteProduct(context.Context, string, string) error { return nil }
func (unimplementedStore) ListReviewsByProduct(context.Context, string, string) ([]Review, error) {
	return nil, nil
}
func (unimplementedStore) CreateReview(context.Context, string, string, ReviewInput) (Review, error) {
	return Review{}, nil
}
func (unimplementedStore) UpdateReview(context.Context, string, string, ReviewInput) (Review, error) {
	return Review{}, nil
}
func (unimplementedStore) DeleteReview(context.Context, string, string) error { return nil }
func (unimplementedStore) CreateOrder(context.Context, orderInsert) (Order, error) {
	return Order{}, nil
}
func (unimplementedStore) ListOrders(context.Context, string, string, string, int, int) ([]Order, int, error) {
	return nil, 0, nil
}
func (unimplementedStore) UpdateOrderStatus(context.Context, string, string, string) (Order, error) {
	return Order{}, nil
}
func (unimplementedStore) InsertEvent(context.Context, string, string, string, string, json.RawMessage) error {
	return nil
}
func (unimplementedStore) ListEvents(context.Context, string, string, int, int) ([]EventView, int, error) {
	return nil, 0, nil
}
func (unimplementedStore) publicRestaurant(context.Context, string) (Restaurant, string, error) {
	return Restaurant{}, "", nil
}
func (unimplementedStore) publicProductBySlug(context.Context, string, string, string) (Product, error) {
	return Product{}, nil
}
func (unimplementedStore) resolveSlugActive(context.Context, string) (string, error) {
	return "", nil
}
func (unimplementedStore) resolveHostActive(context.Context, string) (string, error) {
	return "", nil
}
func (unimplementedStore) productForOrder(context.Context, string, string) (productForOrder, error) {
	return productForOrder{}, nil
}
func (unimplementedStore) variationForProduct(context.Context, string, string) (Variation, error) {
	return Variation{}, nil
}
func (unimplementedStore) addonsForProduct(context.Context, string, []string) ([]Addon, error) {
	return nil, nil
}
