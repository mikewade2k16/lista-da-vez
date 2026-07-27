package cardapio

import (
	"context"
	"encoding/json"
	"time"
)

// dataStore e o contrato de persistencia consumido pelo Service. *Store o
// implementa contra Postgres; os testes usam fakes para exercitar recalculo de
// pedido e resolucao por host sem banco.
type dataStore interface {
	// Restaurants + domains
	CreateRestaurant(ctx context.Context, accountID, slug, name string) (Restaurant, error)
	GetRestaurant(ctx context.Context, accountID, id string) (Restaurant, error)
	ListRestaurantsLean(ctx context.Context, accountID, query string) ([]RestaurantLean, error)
	UpdateRestaurant(ctx context.Context, accountID, id string, in UpdateRestaurantInput) (Restaurant, error)
	MoveRestaurantToAccount(ctx context.Context, currentAccountID, restaurantID, targetAccountID string) (Restaurant, error)
	DuplicateRestaurant(ctx context.Context, accountID, sourceID, slug, name string) (Restaurant, error)
	DeleteRestaurant(ctx context.Context, accountID, id string) error
	AccountExists(ctx context.Context, accountID string) (bool, error)
	ListDomains(ctx context.Context, accountID, restaurantID string) ([]Domain, error)
	CreateDomain(ctx context.Context, accountID, restaurantID, host string, isPrimary bool) (Domain, error)
	DeleteDomain(ctx context.Context, accountID, host string) error

	// Categories
	ListCategories(ctx context.Context, accountID, restaurantID string, activeOnly bool) ([]Category, error)
	CreateCategory(ctx context.Context, accountID, restaurantID string, in CategoryInput) (Category, error)
	UpdateCategory(ctx context.Context, accountID, id string, in CategoryInput) (Category, error)
	DeleteCategory(ctx context.Context, accountID, id string) error

	// Products
	ListProductsLean(ctx context.Context, accountID, restaurantID string) ([]ProductLean, error)
	ListProductsFull(ctx context.Context, accountID, restaurantID string) ([]Product, error)
	GetProduct(ctx context.Context, accountID, id string) (Product, error)
	ListMenuProducts(ctx context.Context, accountID, restaurantID string) ([]Product, error)
	CreateProduct(ctx context.Context, accountID, restaurantID string, in ProductInput) (Product, error)
	UpdateProduct(ctx context.Context, accountID, id string, in ProductInput) (Product, error)
	DeleteProduct(ctx context.Context, accountID, id string) error
	BulkProducts(ctx context.Context, accountID, restaurantID string, ids []string, action ProductBulkAction) (int, error)

	// Reviews
	ListReviewsByProduct(ctx context.Context, accountID, productID string) ([]Review, error)
	ListEstablishmentReviews(ctx context.Context, accountID, restaurantID string) ([]Review, error)
	CreateReview(ctx context.Context, accountID, restaurantID string, in ReviewInput) (Review, error)
	UpdateReview(ctx context.Context, accountID, id string, in ReviewInput) (Review, error)
	DeleteReview(ctx context.Context, accountID, id string) error

	// Delivery zones (WS-A)
	ListZones(ctx context.Context, accountID, restaurantID string) ([]DeliveryZone, error)
	ListPublicZones(ctx context.Context, accountID, restaurantID string) ([]DeliveryZone, error)
	CreateZone(ctx context.Context, accountID, restaurantID string, in DeliveryZoneInput) (DeliveryZone, error)
	UpdateZone(ctx context.Context, accountID, id string, in UpdateDeliveryZoneInput) (DeliveryZone, error)
	DeleteZone(ctx context.Context, accountID, id string) error
	zoneForOrder(ctx context.Context, restaurantID, zoneID string) (DeliveryZone, error)

	// Orders + events
	CreateOrder(ctx context.Context, in orderInsert) (Order, error)
	ListOrders(ctx context.Context, accountID, restaurantID, status string, limit, offset int) ([]Order, int, error)
	UpdateOrderStatus(ctx context.Context, accountID, id, status string) (Order, error)
	InsertEvent(ctx context.Context, accountID, restaurantID, name, sessionID string, eventContext json.RawMessage) error
	InsertEventsBatch(ctx context.Context, accountID, restaurantID string, rows []eventInsert) (int, error)
	UpsertSession(ctx context.Context, in sessionUpsert) error
	listProductSlugs(ctx context.Context, accountID, restaurantID string) (map[string]struct{}, error)
	PruneTelemetry(ctx context.Context, retentionDays int) (events, sessions int64, err error)
	ListEvents(ctx context.Context, accountID, restaurantID string, limit, offset int) ([]EventView, int, error)

	// Analytics (Fase 10 / F2) — leitura/agregacao. O pertencimento do restaurante
	// e validado no service via GetRestaurant (404 fora de escopo); estas queries
	// filtram restaurant_id + account_id e a janela half-open [from, to) por
	// created_at, excluindo bots por padrao. column/eventName/keyExpr vem sempre de
	// allowlists fechadas do service (nunca entrada de usuario crua).
	Overview(ctx context.Context, accountID, restaurantID string, from, to time.Time) (overviewRaw, error)
	TimeseriesDaily(ctx context.Context, accountID, restaurantID string, from, to time.Time, tz string) (map[string]AnalyticsTimePoint, error)
	TimeseriesHourOfDay(ctx context.Context, accountID, restaurantID string, from, to time.Time, tz string) (map[int]AnalyticsTimePoint, error)
	TimeseriesWeekdayHour(ctx context.Context, accountID, restaurantID string, from, to time.Time, tz string) (map[int]AnalyticsTimePoint, error)
	FunnelSteps(ctx context.Context, accountID, restaurantID string, from, to time.Time) ([]AnalyticsFunnelStep, error)
	TopProductsByEvent(ctx context.Context, accountID, restaurantID, eventName string, from, to time.Time, limit int) ([]topProductRaw, error)
	TopProductsByOrders(ctx context.Context, accountID, restaurantID string, from, to time.Time, limit int) ([]topProductRaw, error)
	Sources(ctx context.Context, accountID, restaurantID, column string, from, to time.Time, limit int) ([]AnalyticsSource, error)
	Devices(ctx context.Context, accountID, restaurantID, column string, from, to time.Time) ([]AnalyticsBreakdownItem, error)
	Pages(ctx context.Context, accountID, restaurantID string, from, to time.Time, limit int) ([]AnalyticsPage, error)
	Dwell(ctx context.Context, accountID, restaurantID, eventName, keyExpr string, from, to time.Time, limit int) ([]AnalyticsDwellItem, error)
	Clicks(ctx context.Context, accountID, restaurantID string, from, to time.Time, limit int) ([]AnalyticsClick, error)
	ProductNamesBySlug(ctx context.Context, accountID, restaurantID string, slugs []string) (map[string]string, error)

	// Site layout (Fase 3 / Opcao B)
	GetPublishedLayout(ctx context.Context, accountID, restaurantID string) (json.RawMessage, int64, error)
	GetDraftLayout(ctx context.Context, accountID, restaurantID string) (json.RawMessage, int64, bool, error)
	PutDraftLayout(ctx context.Context, accountID, restaurantID string, draft json.RawMessage, expectedVersion *int64) (json.RawMessage, int64, error)
	PublishLayout(ctx context.Context, accountID, restaurantID string) (json.RawMessage, int64, error)

	// Public lookups
	publicRestaurant(ctx context.Context, slug string) (Restaurant, string, error)
	publicProductBySlug(ctx context.Context, accountID, restaurantID, productSlug string) (Product, error)
	resolveSlugActive(ctx context.Context, slug string) (string, error)
	resolveHostActive(ctx context.Context, host string) (string, error)
	productForOrder(ctx context.Context, restaurantID, productID string) (productForOrder, error)
	variationForProduct(ctx context.Context, productID, variationID string) (Variation, error)
	addonsForProduct(ctx context.Context, productID string, addonIDs []string) ([]Addon, error)
}

// compile-time: *Store implementa dataStore.
var _ dataStore = (*Store)(nil)
