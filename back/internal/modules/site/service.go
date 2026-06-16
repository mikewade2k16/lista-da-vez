package site

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"strings"
)

// Service orquestra regras do modulo site (leads, products, webhook sources).
type Service struct {
	leads          LeadRepository
	products       ProductRepository
	sources        WebhookSourceRepository
	tracking       TrackingRepository
	productSources ProductSourceRepository
	productErp     ProductErpRepository
	sourceClient   productSourceFetcher
	imageCache     *ImageCache
}

// productSourceFetcher abstrai o cliente HTTP da fonte externa (testavel).
type productSourceFetcher interface {
	FetchAll(ctx context.Context, baseURL string) ([]ProductUpsertItem, error)
}

// NewService cria o service.
func NewService(leads LeadRepository, products ProductRepository, sources WebhookSourceRepository, tracking TrackingRepository) *Service {
	return &Service{leads: leads, products: products, sources: sources, tracking: tracking}
}

// WithProductSync injeta o repo de fontes externas e o cliente HTTP usados por
// SyncProducts. Mantido como setter para nao quebrar callers de NewService.
func (s *Service) WithProductSync(repo ProductSourceRepository, client productSourceFetcher) *Service {
	s.productSources = repo
	s.sourceClient = client
	return s
}

// WithImageCache injeta o cache de imagens (baixa as imagens externas dos
// produtos no sync e serve localmente). Opcional: sem ele, o sync mantem as URLs
// externas (hotlink). Setter para nao quebrar callers de NewService.
func (s *Service) WithImageCache(cache *ImageCache) *Service {
	s.imageCache = cache
	return s
}

// WithProductErp injeta o repo de cruzamento com o ERP (erp_item_current).
// Setter para nao quebrar callers de NewService.
func (s *Service) WithProductErp(repo ProductErpRepository) *Service {
	s.productErp = repo
	return s
}

// ============================================================================
// Leads
// ============================================================================

func (s *Service) ListLeads(ctx context.Context, filter LeadListFilter) (LeadListResponse, error) {
	items, total, err := s.leads.List(ctx, filter)
	if err != nil {
		return LeadListResponse{}, err
	}
	page := filter.Page
	if page < 1 {
		page = 1
	}
	perPage := filter.PerPage
	if perPage < 1 {
		perPage = 50
	}
	if perPage > 200 {
		perPage = 200
	}
	return LeadListResponse{Leads: items, Total: total, Page: page, PerPage: perPage}, nil
}

func (s *Service) GetLead(ctx context.Context, accountID, id string) (LeadView, error) {
	return s.leads.Find(ctx, accountID, id)
}

func (s *Service) CreateLead(ctx context.Context, accountID string, input LeadCreateInput) (LeadView, error) {
	input.Email = strings.ToLower(strings.TrimSpace(input.Email))
	input.Nome = strings.TrimSpace(input.Nome)
	input.Telefone = strings.TrimSpace(input.Telefone)
	if input.Nome == "" && input.Email == "" && input.Telefone == "" {
		return LeadView{}, errors.New("nome, email or telefone is required")
	}
	return s.leads.Create(ctx, accountID, input)
}

func (s *Service) UpdateLead(ctx context.Context, accountID, id string, input LeadUpdateInput) (LeadView, error) {
	if input.Status != nil {
		switch LeadStatus(*input.Status) {
		case LeadStatusNew, LeadStatusContacted, LeadStatusQualified, LeadStatusLost:
		default:
			return LeadView{}, errors.New("invalid lead status")
		}
	}
	return s.leads.Update(ctx, accountID, id, input)
}

func (s *Service) DeleteLead(ctx context.Context, accountID, id string) error {
	return s.leads.SoftDelete(ctx, accountID, id)
}

// ============================================================================
// Products
// ============================================================================

func (s *Service) ListProducts(ctx context.Context, filter ProductListFilter) (ProductListResponse, error) {
	items, total, err := s.products.List(ctx, filter)
	if err != nil {
		return ProductListResponse{}, err
	}
	page := filter.Page
	if page < 1 {
		page = 1
	}
	perPage := filter.PerPage
	if perPage < 1 {
		perPage = 50
	}
	if perPage > 200 {
		perPage = 200
	}
	return ProductListResponse{Products: items, Total: total, Page: page, PerPage: perPage}, nil
}

func (s *Service) GetProduct(ctx context.Context, accountID, id string) (ProductView, error) {
	return s.products.Find(ctx, accountID, id)
}

func (s *Service) CreateProduct(ctx context.Context, accountID string, input ProductCreateInput) (ProductView, error) {
	input.Name = strings.TrimSpace(input.Name)
	if input.Name == "" {
		return ProductView{}, errors.New("name is required")
	}
	if input.Fator == 0 {
		input.Fator = 1
	}
	return s.products.Create(ctx, accountID, input)
}

func (s *Service) UpdateProduct(ctx context.Context, accountID, id string, input ProductUpdateInput) (ProductView, error) {
	if input.Status != nil {
		switch ProductStatus(*input.Status) {
		case ProductStatusActive, ProductStatusInactive:
		default:
			return ProductView{}, errors.New("invalid product status")
		}
	}
	return s.products.Update(ctx, accountID, id, input)
}

func (s *Service) DeleteProduct(ctx context.Context, accountID, id string) error {
	return s.products.SoftDelete(ctx, accountID, id)
}

// UploadProductImage grava a imagem enviada (multipart) em /uploads e atualiza o
// produto para apontar pra ela. Requer o cache de imagens (UPLOADS_DIR) configurado.
func (s *Service) UploadProductImage(ctx context.Context, accountID, productID, filename, contentType string, content []byte) (ProductView, error) {
	if s.imageCache == nil {
		return ProductView{}, errors.New("site: image storage unavailable")
	}
	rel, err := s.imageCache.SaveUpload(accountID, filename, contentType, content)
	if err != nil {
		return ProductView{}, err
	}
	return s.products.Update(ctx, accountID, productID, ProductUpdateInput{Image: &rel})
}

// SyncProducts puxa os produtos das fontes externas habilitadas da account e
// faz upsert em site.products. Retorna {inserted, updated, skipped} agregado.
func (s *Service) SyncProducts(ctx context.Context, accountID string) (ProductSyncResult, error) {
	if s.productSources == nil || s.sourceClient == nil {
		return ProductSyncResult{}, ErrProductSyncUnavailable
	}
	srcs, err := s.productSources.ListByAccount(ctx, accountID)
	if err != nil {
		return ProductSyncResult{}, err
	}

	total := ProductSyncResult{}
	synced := false
	for _, src := range srcs {
		if !src.Enabled || strings.TrimSpace(src.BaseURL) == "" {
			continue
		}
		synced = true
		items, err := s.sourceClient.FetchAll(ctx, src.BaseURL)
		if err != nil {
			return ProductSyncResult{}, err
		}
		// Baixa as imagens externas e reescreve item.Image para o path local
		// (/uploads/site/products/...) ANTES do upsert. Falha => mantem a URL
		// externa (fallback). So roda se o cache estiver configurado.
		if s.imageCache != nil {
			total.ImagesCached += s.imageCache.CacheItems(ctx, accountID, items)
		}
		res, err := s.productSources.UpsertProducts(ctx, accountID, items)
		if err != nil {
			return ProductSyncResult{}, err
		}
		total.Inserted += res.Inserted
		total.Updated += res.Updated
		total.Skipped += res.Skipped
	}
	if !synced {
		return ProductSyncResult{}, ErrNoProductSource
	}
	return total, nil
}

// ============================================================================
// Product source toggle (local XAMPP / online)
// ============================================================================

// productSourceModeFromBaseURL deriva o modo a partir do base_url da fonte:
// contem host.docker.internal -> local; contem o host da Perola -> online;
// senao -> custom.
func productSourceModeFromBaseURL(baseURL string) string {
	switch {
	case strings.Contains(baseURL, dockerInternalHost):
		return productSourceModeLocal
	case strings.Contains(baseURL, "perolajoias.com"):
		return productSourceModeOnline
	default:
		return productSourceModeCustom
	}
}

// GetProductSource le a fonte external_api da account e deriva o modo do
// base_url. Sem fonte configurada => {mode: "online", baseUrl: ""}.
func (s *Service) GetProductSource(ctx context.Context, accountID string) (ProductSourceView, error) {
	if s.productSources == nil {
		return ProductSourceView{}, ErrProductSyncUnavailable
	}
	src, err := s.productSources.GetAccountSource(ctx, accountID)
	if err != nil {
		if errors.Is(err, ErrNoProductSource) {
			return ProductSourceView{Mode: productSourceModeOnline, BaseURL: ""}, nil
		}
		return ProductSourceView{}, err
	}
	return ProductSourceView{
		Mode:    productSourceModeFromBaseURL(src.BaseURL),
		BaseURL: src.BaseURL,
	}, nil
}

// SetProductSourceMode troca o base_url da fonte external_api da account para a
// URL conhecida do modo (local/online). Modo invalido => ErrInvalidProductSourceMode.
func (s *Service) SetProductSourceMode(ctx context.Context, accountID, mode string) (ProductSourceView, error) {
	if s.productSources == nil {
		return ProductSourceView{}, ErrProductSyncUnavailable
	}
	var baseURL string
	switch mode {
	case productSourceModeLocal:
		baseURL = productSourceURLLocal
	case productSourceModeOnline:
		baseURL = productSourceURLOnline
	default:
		return ProductSourceView{}, ErrInvalidProductSourceMode
	}
	if err := s.productSources.SetAccountSourceBaseURL(ctx, accountID, baseURL); err != nil {
		return ProductSourceView{}, err
	}
	return ProductSourceView{Mode: mode, BaseURL: baseURL}, nil
}

// ============================================================================
// ERP cross-match
// ============================================================================

// MatchERP cruza os produtos ativos da account com erp_item_current e
// materializa o resultado em site.product_erp_links. Retorna {matched, products}.
func (s *Service) MatchERP(ctx context.Context, accountID string) (ErpMatchResult, error) {
	if s.productErp == nil {
		return ErpMatchResult{}, ErrProductSyncUnavailable
	}
	return s.productErp.MatchERP(ctx, accountID)
}

// ListUnmatchedErp lista itens do ERP da account que ainda nao casam com nenhum
// segmento de code de produto ativo.
func (s *Service) ListUnmatchedErp(ctx context.Context, filter ErpUnmatchedFilter) (ErpUnmatchedListResponse, error) {
	if s.productErp == nil {
		return ErpUnmatchedListResponse{}, ErrProductSyncUnavailable
	}
	items, total, err := s.productErp.ListUnmatched(ctx, filter)
	if err != nil {
		return ErpUnmatchedListResponse{}, err
	}
	page := filter.Page
	if page < 1 {
		page = 1
	}
	perPage := filter.PerPage
	if perPage < 1 {
		perPage = 50
	}
	if perPage > 200 {
		perPage = 200
	}
	return ErpUnmatchedListResponse{Items: items, Total: total, Page: page, PerPage: perPage}, nil
}

// CreateProductFromErp cria um site.products a partir de um item do ERP (sku) e
// roda o cruzamento para aquele produto, removendo-o do erp-unmatched. O sku tem
// de existir no ERP da account (senao ErrErpItemNotFound).
func (s *Service) CreateProductFromErp(ctx context.Context, accountID string, input ProductFromErpInput) (ProductView, error) {
	if s.productErp == nil {
		return ProductView{}, ErrProductSyncUnavailable
	}
	sku := strings.TrimSpace(input.Sku)
	if sku == "" {
		return ProductView{}, errors.New("sku is required")
	}
	item, err := s.productErp.FindErpItem(ctx, accountID, sku)
	if err != nil {
		return ProductView{}, err
	}
	view, err := s.products.CreateFromErp(ctx, accountID, item)
	if err != nil {
		return ProductView{}, err
	}
	if err := s.productErp.MatchERPForProduct(ctx, accountID, view.ID); err != nil {
		return ProductView{}, err
	}
	return s.products.Find(ctx, accountID, view.ID)
}

// ============================================================================
// Webhook sources
// ============================================================================

func (s *Service) ListSources(ctx context.Context, accountID string) ([]WebhookSourceView, error) {
	return s.sources.List(ctx, accountID)
}

// CreateSource cria a source com um secret gerado aleatoriamente. O secret e
// retornado APENAS uma vez (o banco guarda so o hash).
func (s *Service) CreateSource(ctx context.Context, accountID string, input WebhookSourceCreateInput) (WebhookSourceCreatedResponse, error) {
	input.Slug = strings.ToLower(strings.TrimSpace(input.Slug))
	input.Name = strings.TrimSpace(input.Name)
	if len(input.Slug) < 3 {
		return WebhookSourceCreatedResponse{}, errors.New("slug must have at least 3 chars")
	}
	if input.Name == "" {
		return WebhookSourceCreatedResponse{}, errors.New("name is required")
	}
	if input.EntityType != "leads" && input.EntityType != "products" && input.EntityType != "tracking" {
		return WebhookSourceCreatedResponse{}, ErrInvalidEntityType
	}

	secret, err := generateSecret(32)
	if err != nil {
		return WebhookSourceCreatedResponse{}, err
	}
	view, err := s.sources.Create(ctx, accountID, input, secret)
	if err != nil {
		return WebhookSourceCreatedResponse{}, err
	}
	return WebhookSourceCreatedResponse{Source: view, Secret: secret}, nil
}

// RotateSecret gera novo secret e devolve o secret novo (mostrado uma unica vez).
func (s *Service) RotateSecret(ctx context.Context, accountID, sourceID string) (WebhookSourceRotateResponse, error) {
	_, err := s.sources.Find(ctx, accountID, sourceID)
	if err != nil {
		return WebhookSourceRotateResponse{}, err
	}
	secret, err := generateSecret(32)
	if err != nil {
		return WebhookSourceRotateResponse{}, err
	}
	if err := s.sources.UpdateSecret(ctx, sourceID, secret); err != nil {
		return WebhookSourceRotateResponse{}, err
	}
	return WebhookSourceRotateResponse{Secret: secret}, nil
}

func (s *Service) DeleteSource(ctx context.Context, accountID, sourceID string) error {
	return s.sources.SoftDelete(ctx, accountID, sourceID)
}

// ============================================================================
// Tracking admin
// ============================================================================

func (s *Service) ListTrackingEvents(ctx context.Context, filter TrackingEventListFilter) (TrackingEventListResponse, error) {
	items, total, err := s.tracking.List(ctx, filter)
	if err != nil {
		return TrackingEventListResponse{}, err
	}
	page := filter.Page
	if page < 1 {
		page = 1
	}
	perPage := filter.PerPage
	if perPage < 1 {
		perPage = 50
	}
	if perPage > 200 {
		perPage = 200
	}
	return TrackingEventListResponse{Events: items, Total: total, Page: page, PerPage: perPage}, nil
}

// ============================================================================
// Ingest (webhook)
// ============================================================================

// IngestLead recebe payload de webhook e cria um lead na account dona da source.
// Caller (handler HTTP) deve validar HMAC antes; aqui assumimos que ja foi feito.
func (s *Service) IngestLead(ctx context.Context, source WebhookSourceView, fields map[string]any, raw string) (LeadView, error) {
	if source.EntityType != "leads" {
		return LeadView{}, errors.New("source is not configured for leads")
	}
	return s.leads.CreateFromWebhook(ctx, source.AccountID, source.ID, source.Name, fields, raw)
}

// IngestProduct recebe payload de webhook e cria/atualiza produto.
func (s *Service) IngestProduct(ctx context.Context, source WebhookSourceView, fields map[string]any, raw string) (ProductView, error) {
	if source.EntityType != "products" {
		return ProductView{}, errors.New("source is not configured for products")
	}
	return s.products.CreateFromWebhook(ctx, source.AccountID, source.ID, source.Name, fields, raw)
}

// FindSourceBySlug e usado pelo handler de ingest para resolver slug → source.
func (s *Service) FindSourceBySlug(ctx context.Context, slug string) (WebhookSourceView, string, error) {
	return s.sources.FindBySlug(ctx, slug)
}

// ============================================================================
// Crypto helpers
// ============================================================================

func generateSecret(nBytes int) (string, error) {
	b := make([]byte, nBytes)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
