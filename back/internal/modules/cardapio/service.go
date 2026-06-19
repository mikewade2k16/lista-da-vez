package cardapio

import (
	"context"
	"errors"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ServiceConfig agrupa a configuracao do modulo lida do ambiente.
type ServiceConfig struct {
	BaseDomain     string // CARDAPIO_BASE_DOMAIN (resolve por subdominio)
	DevDefaultSlug string // CARDAPIO_DEV_DEFAULT_SLUG (localhost em dev)
	PublicBaseURL  string // PUBLIC_API_BASE_URL (absolutiza /uploads/* no publico)
	Media          MediaStorage
}

// Service orquestra as regras do modulo cardapio (painel + publico). Depende de
// dataStore (interface) para permitir fakes nos testes de recalculo/resolve.
type Service struct {
	store dataStore
	cfg   ServiceConfig
}

// NewService cria o service. pool e mantido na assinatura para alinhar com o
// padrao dos demais modulos (Build passa deps.Pool); o Service usa o Store.
func NewService(store *Store, _ *pgxpool.Pool, cfg ServiceConfig) *Service {
	return &Service{store: store, cfg: cfg}
}

// newServiceWithStore injeta um dataStore arbitrario (usado nos testes).
func newServiceWithStore(store dataStore, cfg ServiceConfig) *Service {
	return &Service{store: store, cfg: cfg}
}

// mapStoreErr traduz erros de banco para erros de dominio do modulo.
func mapStoreErr(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return ErrSlugConflict
	}
	return err
}

func normalizeSlug(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

// ============================================================================
// Restaurants (painel)
// ============================================================================

// ListRestaurants devolve a listagem lean. accountID e o filtro JA validado
// contra o Principal (vazio so para platform_admin sem filtro).
func (s *Service) ListRestaurants(ctx context.Context, accountID, query string) ([]RestaurantLean, error) {
	return s.store.ListRestaurantsLean(ctx, accountID, strings.TrimSpace(query))
}

// CreateRestaurant cria um restaurante na account informada (ja validada).
func (s *Service) CreateRestaurant(ctx context.Context, accountID, slug, name string) (Restaurant, error) {
	slug = normalizeSlug(slug)
	name = strings.TrimSpace(name)
	if slug == "" || name == "" {
		return Restaurant{}, ErrValidation
	}
	r, err := s.store.CreateRestaurant(ctx, accountID, slug, name)
	return r, mapStoreErr(err)
}

// GetRestaurant busca por id dentro da account.
func (s *Service) GetRestaurant(ctx context.Context, accountID, id string) (Restaurant, error) {
	r, err := s.store.GetRestaurant(ctx, accountID, id)
	return r, mapStoreErr(err)
}

// UpdateRestaurant aplica o PATCH parcial. in.AccountID (mover de conta) ja chega
// zerado para nao-admin (gate no handler); aqui resolvemos o ponteiro do move
// espelhando a bio: vazio/conta atual => nao move; conta destino inexistente =>
// ErrNotFound (404 limpo antes do update, alem da protecao da FK).
func (s *Service) UpdateRestaurant(ctx context.Context, accountID, id string, in UpdateRestaurantInput) (Restaurant, error) {
	if in.Name != nil {
		trimmed := strings.TrimSpace(*in.Name)
		if trimmed == "" {
			return Restaurant{}, ErrValidation
		}
		in.Name = &trimmed
	}
	movePtr, err := s.resolveMoveAccount(ctx, in.AccountID, accountID)
	if err != nil {
		return Restaurant{}, err
	}
	in.AccountID = movePtr
	r, err := s.store.UpdateRestaurant(ctx, accountID, id, in)
	return r, mapStoreErr(err)
}

// resolveMoveAccount decide o ponteiro de account_id do PATCH (mover de conta).
// nil/vazio/igual a conta atual => nil (nao move). Caso contrario, valida que a
// conta destino existe (ErrNotFound se nao) e devolve o ponteiro normalizado.
func (s *Service) resolveMoveAccount(ctx context.Context, accountID *string, currentAccountID string) (*string, error) {
	if accountID == nil {
		return nil, nil
	}
	target := strings.TrimSpace(*accountID)
	if target == "" || target == strings.TrimSpace(currentAccountID) {
		return nil, nil
	}
	exists, err := s.store.AccountExists(ctx, target)
	if err != nil {
		return nil, mapStoreErr(err)
	}
	if !exists {
		return nil, ErrNotFound
	}
	return &target, nil
}

// DeleteRestaurant remove o restaurante.
func (s *Service) DeleteRestaurant(ctx context.Context, accountID, id string) error {
	return s.store.DeleteRestaurant(ctx, accountID, id)
}

// ============================================================================
// Domains (painel)
// ============================================================================

// ListDomains lista os dominios de um restaurante (valida posse do restaurante).
func (s *Service) ListDomains(ctx context.Context, accountID, restaurantID string) ([]Domain, error) {
	if _, err := s.store.GetRestaurant(ctx, accountID, restaurantID); err != nil {
		return nil, mapStoreErr(err)
	}
	return s.store.ListDomains(ctx, accountID, restaurantID)
}

// CreateDomain valida e normaliza o host antes de inserir.
func (s *Service) CreateDomain(ctx context.Context, accountID, restaurantID string, in DomainInput) (Domain, error) {
	if _, err := s.store.GetRestaurant(ctx, accountID, restaurantID); err != nil {
		return Domain{}, mapStoreErr(err)
	}
	host := normalizeHost(in.Host)
	if host == "" {
		return Domain{}, ErrValidation
	}
	d, err := s.store.CreateDomain(ctx, accountID, restaurantID, host, in.IsPrimary)
	return d, mapStoreErr(err)
}

// DeleteDomain remove um dominio por host.
func (s *Service) DeleteDomain(ctx context.Context, accountID, host string) error {
	return s.store.DeleteDomain(ctx, accountID, normalizeHost(host))
}

// ============================================================================
// Delivery zones (painel) — WS-A
// ============================================================================

// ListDeliveryZones lista as zonas de um restaurante (valida posse).
func (s *Service) ListDeliveryZones(ctx context.Context, accountID, restaurantID string) ([]DeliveryZone, error) {
	if _, err := s.store.GetRestaurant(ctx, accountID, restaurantID); err != nil {
		return nil, mapStoreErr(err)
	}
	return s.store.ListZones(ctx, accountID, restaurantID)
}

// CreateDeliveryZone cria uma zona no restaurante (valida posse; nome obrigatorio).
func (s *Service) CreateDeliveryZone(ctx context.Context, accountID, restaurantID string, in DeliveryZoneInput) (DeliveryZone, error) {
	if _, err := s.store.GetRestaurant(ctx, accountID, restaurantID); err != nil {
		return DeliveryZone{}, mapStoreErr(err)
	}
	in.Name = strings.TrimSpace(in.Name)
	if in.Name == "" || in.FeeCents < 0 {
		return DeliveryZone{}, ErrValidation
	}
	z, err := s.store.CreateZone(ctx, accountID, restaurantID, in)
	return z, mapStoreErr(err)
}

// UpdateDeliveryZone aplica o PATCH parcial de uma zona (escopo pela account; 404
// fora do escopo). Valida apenas os campos enviados.
func (s *Service) UpdateDeliveryZone(ctx context.Context, accountID, id string, in UpdateDeliveryZoneInput) (DeliveryZone, error) {
	if in.Name != nil {
		trimmed := strings.TrimSpace(*in.Name)
		if trimmed == "" {
			return DeliveryZone{}, ErrValidation
		}
		in.Name = &trimmed
	}
	if in.FeeCents != nil && *in.FeeCents < 0 {
		return DeliveryZone{}, ErrValidation
	}
	z, err := s.store.UpdateZone(ctx, accountID, id, in)
	return z, mapStoreErr(err)
}

// DeleteDeliveryZone remove uma zona por id na account.
func (s *Service) DeleteDeliveryZone(ctx context.Context, accountID, id string) error {
	return s.store.DeleteZone(ctx, accountID, id)
}

// ============================================================================
// Categories (painel)
// ============================================================================

func (s *Service) ListCategories(ctx context.Context, accountID, restaurantID string) ([]Category, error) {
	if _, err := s.store.GetRestaurant(ctx, accountID, restaurantID); err != nil {
		return nil, mapStoreErr(err)
	}
	return s.store.ListCategories(ctx, accountID, restaurantID, false)
}

func (s *Service) CreateCategory(ctx context.Context, accountID, restaurantID string, in CategoryInput) (Category, error) {
	if _, err := s.store.GetRestaurant(ctx, accountID, restaurantID); err != nil {
		return Category{}, mapStoreErr(err)
	}
	in.Slug = normalizeSlug(in.Slug)
	in.Name = strings.TrimSpace(in.Name)
	if in.Slug == "" || in.Name == "" {
		return Category{}, ErrValidation
	}
	c, err := s.store.CreateCategory(ctx, accountID, restaurantID, in)
	return c, mapStoreErr(err)
}

func (s *Service) UpdateCategory(ctx context.Context, accountID, id string, in CategoryInput) (Category, error) {
	in.Slug = normalizeSlug(in.Slug)
	in.Name = strings.TrimSpace(in.Name)
	if in.Slug == "" || in.Name == "" {
		return Category{}, ErrValidation
	}
	c, err := s.store.UpdateCategory(ctx, accountID, id, in)
	return c, mapStoreErr(err)
}

func (s *Service) DeleteCategory(ctx context.Context, accountID, id string) error {
	return s.store.DeleteCategory(ctx, accountID, id)
}

// ============================================================================
// Products (painel)
// ============================================================================

func (s *Service) ListProducts(ctx context.Context, accountID, restaurantID string) ([]ProductLean, error) {
	if _, err := s.store.GetRestaurant(ctx, accountID, restaurantID); err != nil {
		return nil, mapStoreErr(err)
	}
	return s.store.ListProductsLean(ctx, accountID, restaurantID)
}

func (s *Service) GetProduct(ctx context.Context, accountID, id string) (Product, error) {
	p, err := s.store.GetProduct(ctx, accountID, id)
	return p, mapStoreErr(err)
}

func (s *Service) CreateProduct(ctx context.Context, accountID, restaurantID string, in ProductInput) (Product, error) {
	if _, err := s.store.GetRestaurant(ctx, accountID, restaurantID); err != nil {
		return Product{}, mapStoreErr(err)
	}
	if err := validateProductInput(&in); err != nil {
		return Product{}, err
	}
	p, err := s.store.CreateProduct(ctx, accountID, restaurantID, in)
	return p, mapStoreErr(err)
}

func (s *Service) UpdateProduct(ctx context.Context, accountID, id string, in ProductInput) (Product, error) {
	if err := validateProductInput(&in); err != nil {
		return Product{}, err
	}
	p, err := s.store.UpdateProduct(ctx, accountID, id, in)
	return p, mapStoreErr(err)
}

func validateProductInput(in *ProductInput) error {
	in.Slug = normalizeSlug(in.Slug)
	in.Name = strings.TrimSpace(in.Name)
	if in.Slug == "" || in.Name == "" {
		return ErrValidation
	}
	if in.PriceCents < 0 {
		return ErrValidation
	}
	return nil
}

func (s *Service) DeleteProduct(ctx context.Context, accountID, id string) error {
	return s.store.DeleteProduct(ctx, accountID, id)
}

// ============================================================================
// Reviews (painel)
// ============================================================================

func (s *Service) ListReviews(ctx context.Context, accountID, productID string) ([]Review, error) {
	if _, err := s.store.GetProduct(ctx, accountID, productID); err != nil {
		return nil, mapStoreErr(err)
	}
	return s.store.ListReviewsByProduct(ctx, accountID, productID)
}

func (s *Service) CreateReview(ctx context.Context, accountID, productID string, in ReviewInput) (Review, error) {
	product, err := s.store.GetProduct(ctx, accountID, productID)
	if err != nil {
		return Review{}, mapStoreErr(err)
	}
	in.ProductID = productID
	in.AuthorName = strings.TrimSpace(in.AuthorName)
	if in.AuthorName == "" || in.Rating < 1 || in.Rating > 5 {
		return Review{}, ErrValidation
	}
	r, err := s.store.CreateReview(ctx, accountID, product.RestaurantID, in)
	return r, mapStoreErr(err)
}

func (s *Service) UpdateReview(ctx context.Context, accountID, id string, in ReviewInput) (Review, error) {
	in.AuthorName = strings.TrimSpace(in.AuthorName)
	if in.AuthorName == "" || in.Rating < 1 || in.Rating > 5 {
		return Review{}, ErrValidation
	}
	r, err := s.store.UpdateReview(ctx, accountID, id, in)
	return r, mapStoreErr(err)
}

func (s *Service) DeleteReview(ctx context.Context, accountID, id string) error {
	return s.store.DeleteReview(ctx, accountID, id)
}

// ============================================================================
// Orders + Events (painel)
// ============================================================================

func (s *Service) ListOrders(ctx context.Context, accountID, restaurantID, status string, page, perPage int) ([]Order, int, error) {
	if _, err := s.store.GetRestaurant(ctx, accountID, restaurantID); err != nil {
		return nil, 0, mapStoreErr(err)
	}
	limit, offset := paginate(page, perPage)
	return s.store.ListOrders(ctx, accountID, restaurantID, strings.TrimSpace(status), limit, offset)
}

// UpdateOrderStatus valida o status e persiste.
func (s *Service) UpdateOrderStatus(ctx context.Context, accountID, id, status string) (Order, error) {
	if !isValidOrderStatus(status) {
		return Order{}, ErrValidation
	}
	o, err := s.store.UpdateOrderStatus(ctx, accountID, id, status)
	return o, mapStoreErr(err)
}

func (s *Service) ListEvents(ctx context.Context, accountID, restaurantID string, page, perPage int) ([]EventView, int, error) {
	if _, err := s.store.GetRestaurant(ctx, accountID, restaurantID); err != nil {
		return nil, 0, mapStoreErr(err)
	}
	limit, offset := paginate(page, perPage)
	return s.store.ListEvents(ctx, accountID, restaurantID, limit, offset)
}

// ============================================================================
// Media (painel)
// ============================================================================

// SaveMedia grava uma imagem do restaurante e devolve o caminho relativo.
func (s *Service) SaveMedia(ctx context.Context, accountID, restaurantID, fileName, contentType string, content []byte) (string, error) {
	if _, err := s.store.GetRestaurant(ctx, accountID, restaurantID); err != nil {
		return "", mapStoreErr(err)
	}
	if s.cfg.Media == nil {
		return "", ErrInvalidMedia
	}
	return s.cfg.Media.Save(accountID, fileName, contentType, content)
}

// ============================================================================
// Helpers
// ============================================================================

func isValidOrderStatus(status string) bool {
	switch status {
	case OrderStatusReceived, OrderStatusPreparing, OrderStatusReady,
		OrderStatusOnRoute, OrderStatusDelivered, OrderStatusCanceled:
		return true
	default:
		return false
	}
}

func paginate(page, perPage int) (limit, offset int) {
	if page < 1 {
		page = 1
	}
	if perPage < 1 {
		perPage = 50
	}
	if perPage > 200 {
		perPage = 200
	}
	return perPage, (page - 1) * perPage
}

// normalizeHost normaliza um host: lowercase, sem espacos, remove a porta e o
// prefixo www.
func normalizeHost(host string) string {
	h := strings.ToLower(strings.TrimSpace(host))
	if h == "" {
		return ""
	}
	if idx := strings.IndexByte(h, '/'); idx >= 0 {
		h = h[:idx]
	}
	if idx := strings.IndexByte(h, ':'); idx >= 0 {
		h = h[:idx]
	}
	h = strings.TrimPrefix(h, "www.")
	return h
}
