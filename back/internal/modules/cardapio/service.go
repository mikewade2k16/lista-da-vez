package cardapio

import (
	"context"
	"errors"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/stringsx"
)

// ServiceConfig agrupa a configuracao do modulo lida do ambiente.
type ServiceConfig struct {
	BaseDomain     string // CARDAPIO_BASE_DOMAIN (resolve por subdominio)
	DevDefaultSlug string // CARDAPIO_DEV_DEFAULT_SLUG (localhost em dev)
	PublicBaseURL  string // PUBLIC_API_BASE_URL (absolutiza /uploads/* no publico)
	TelemetrySalt  string // CARDAPIO_TELEMETRY_SALT (hash do IP na ingestao de telemetria)
	Media          MediaStorage
}

// Service orquestra as regras do modulo cardapio (painel + publico). Depende de
// dataStore (interface) para permitir fakes nos testes de recalculo/resolve.
type Service struct {
	store dataStore
	cfg   ServiceConfig
	gate  *cardapioGate // permissao fina do painel; nil nos testes (gate fail-closed)
}

// NewService cria o service. O gate de permissao e injetado depois via WithGate
// (Build do modulo monta o RBACService); ate la o gate fica nil (so os handlers
// do painel o consultam, e a falta dele e fail-closed, nunca fail-open).
func NewService(store *Store, cfg ServiceConfig) *Service {
	return &Service{store: store, cfg: cfg}
}

// WithGate injeta o gate de permissao fina do painel (curto-circuito de
// platform_admin/agency_owner + cardapio.view/manage/orders.manage). Retorna o
// proprio Service para encadear no Build.
func (s *Service) WithGate(gate *cardapioGate) *Service {
	s.gate = gate
	return s
}

// authorize delega ao gate a checagem de permKey na account para o Principal.
// Sem gate (testes), so platform_admin passa — fail-closed.
func (s *Service) authorize(ctx context.Context, principal auth.Principal, accountID, permKey string) error {
	if s.gate == nil {
		if principal.Role == auth.RolePlatformAdmin {
			return nil
		}
		return ErrForbidden
	}
	return s.gate.Authorize(ctx, principal, accountID, permKey)
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

// normalizeSlug aplica a regra canonica de slug ao valor recebido do cliente.
// Antes era so ToLower+Trim; agora normaliza acentos tambem via stringsx.Slugify
// (mudanca deliberada — slugs NOVOS podem diferir; os gravados no banco nao sao
// re-gerados). Valores ja em formato ^[a-z0-9-]+$ nao mudam.
func normalizeSlug(value string) string {
	return stringsx.Slugify(value)
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

// DuplicateRestaurant copia o restaurante sourceID (catalogo + zonas + layout)
// para um novo restaurante (is_active=false) na MESMA account (ja validada). name
// e slug sao obrigatorios; o slug precisa estar livre globalmente (a unique do
// banco vira ErrSlugConflict via mapStoreErr). O gate de platform_admin e do
// handler (403); aqui o source e escopado pela account (404 fora de escopo). A
// copia inteira e transacional no store.
func (s *Service) DuplicateRestaurant(ctx context.Context, accountID, sourceID string, in DuplicateRestaurantInput) (Restaurant, error) {
	slug := normalizeSlug(in.Slug)
	name := strings.TrimSpace(in.Name)
	if slug == "" || name == "" {
		return Restaurant{}, ErrValidation
	}
	r, err := s.store.DuplicateRestaurant(ctx, accountID, sourceID, slug, name)
	return r, mapStoreErr(err)
}

// UpdateRestaurant aplica o PATCH parcial. in.AccountID (mover de conta) ja chega
// zerado para nao-admin (gate no handler); aqui resolvemos o ponteiro do move
// espelhando a bio: vazio/conta atual => nao move; conta destino inexistente =>
// ErrNotFound (404 limpo antes do update, alem da protecao da FK).
//
// Mover de conta (move) NAO e um simples update de coluna: a subarvore inteira do
// restaurante (categorias, produtos, variacoes/adicionais, avaliacoes, zonas,
// pedidos/itens, eventos, dominios, layout) precisa mudar de account_id na MESMA
// transacao, senao o cardapio fica orfao na conta nova (filhas com account_id
// antigo) e o site publico cai (o publico exige core.account_modules habilitado
// na conta nova). Por isso o move usa MoveRestaurantToAccount, que tambem
// auto-habilita o modulo cardapio no destino (decisao de negocio). Quando ha
// move, os demais campos do MESMO PATCH sao ignorados: o painel dispara o move
// como edicao isolada da coluna Cliente (sem outros campos no corpo); aplica-se
// apenas o move para manter o comportamento simples e seguro.
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
	if movePtr != nil {
		// Move da subarvore inteira + auto-habilita o modulo no destino (atomico).
		r, err := s.store.MoveRestaurantToAccount(ctx, accountID, id, *movePtr)
		return r, mapStoreErr(err)
	}
	in.AccountID = nil
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

// ListEstablishmentReviews lista as avaliacoes do estabelecimento (F2): reviews
// proprias (product_id NULL) + reviews de produto marcadas para a vitrine. Valida
// posse do restaurante (404 fora de escopo).
func (s *Service) ListEstablishmentReviews(ctx context.Context, accountID, restaurantID string) ([]Review, error) {
	if _, err := s.store.GetRestaurant(ctx, accountID, restaurantID); err != nil {
		return nil, mapStoreErr(err)
	}
	return s.store.ListEstablishmentReviews(ctx, accountID, restaurantID)
}

// CreateEstablishmentReview cria uma avaliacao do estabelecimento (product_id
// NULL). Valida posse do restaurante e rating 1-5 (como o create de produto).
func (s *Service) CreateEstablishmentReview(ctx context.Context, accountID, restaurantID string, in ReviewInput) (Review, error) {
	if _, err := s.store.GetRestaurant(ctx, accountID, restaurantID); err != nil {
		return Review{}, mapStoreErr(err)
	}
	in.ProductID = "" // review do estabelecimento: sem produto associado (NULL).
	in.AuthorName = strings.TrimSpace(in.AuthorName)
	if in.AuthorName == "" || in.Rating < 1 || in.Rating > 5 {
		return Review{}, ErrValidation
	}
	r, err := s.store.CreateReview(ctx, accountID, restaurantID, in)
	return r, mapStoreErr(err)
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

// SaveMedia grava uma midia do restaurante (imagem ou video) e devolve o caminho
// relativo. O teto de tamanho e o mime aceito sao validados no MediaStorage.
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

// normalizeHost normaliza um host: lowercase, sem espacos, remove o esquema
// (http:// ou https://), a porta, o path e o prefixo www. Aceitar a URL colada
// inteira (ex.: "https://loja.com/") evita salvar lixo como "https" — o "://" e
// removido ANTES dos cortes em "/" e ":" para o esquema nao virar o host.
func normalizeHost(host string) string {
	h := strings.ToLower(strings.TrimSpace(host))
	if h == "" {
		return ""
	}
	if idx := strings.Index(h, "://"); idx >= 0 {
		h = h[idx+3:]
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
