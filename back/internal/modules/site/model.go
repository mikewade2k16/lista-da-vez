package site

import (
	"context"
	"time"
)

// LeadStatus e o ciclo de vida do lead no painel.
type LeadStatus string

const (
	LeadStatusNew       LeadStatus = "new"
	LeadStatusContacted LeadStatus = "contacted"
	LeadStatusQualified LeadStatus = "qualified"
	LeadStatusLost      LeadStatus = "lost"
)

// LeadView e o DTO completo de um lead retornado por GET /v1/admin/leads.
type LeadView struct {
	ID           string    `json:"id"`
	AccountID    string    `json:"accountId"`
	SourceID     string    `json:"sourceId,omitempty"`
	SourceLabel  string    `json:"sourceLabel"`
	Nome         string    `json:"nome"`
	Email        string    `json:"email"`
	Telefone     string    `json:"telefone"`
	Page         string    `json:"page"`
	Cupom        string    `json:"cupom"`
	Consent      bool      `json:"consent"`
	ConsentLabel string    `json:"consentLabel"`
	TrackingData string    `json:"trackingData,omitempty"`
	PayloadRaw   string    `json:"payloadRaw,omitempty"`
	Status       string    `json:"status"`
	Notes        string    `json:"notes"`
	CreatedAt    time.Time `json:"createdAt"`
	UpdatedAt    time.Time `json:"updatedAt"`
}

// LeadListFilter parametriza GET /v1/admin/leads.
type LeadListFilter struct {
	AccountID string
	Q         string
	Status    string
	SourceID  string
	Page      int
	PerPage   int
}

// LeadListResponse e o body de GET /v1/admin/leads.
type LeadListResponse struct {
	Leads   []LeadView `json:"leads"`
	Total   int        `json:"total"`
	Page    int        `json:"page"`
	PerPage int        `json:"perPage"`
}

// LeadCreateInput e o body de POST /v1/admin/leads (criacao manual).
type LeadCreateInput struct {
	Nome         string `json:"nome"`
	Email        string `json:"email"`
	Telefone     string `json:"telefone"`
	Page         string `json:"page"`
	Cupom        string `json:"cupom"`
	Consent      bool   `json:"consent"`
	ConsentLabel string `json:"consentLabel"`
	SourceLabel  string `json:"sourceLabel"`
	Notes        string `json:"notes"`
}

// LeadUpdateInput e o body de PATCH /v1/admin/leads/:id. Semantica de patch.
type LeadUpdateInput struct {
	Nome     *string `json:"nome"`
	Email    *string `json:"email"`
	Telefone *string `json:"telefone"`
	Status   *string `json:"status"`
	Notes    *string `json:"notes"`
}

// ============================================================================
// Products
// ============================================================================

// ProductStatus e 'active' | 'inactive'.
type ProductStatus string

const (
	ProductStatusActive   ProductStatus = "active"
	ProductStatusInactive ProductStatus = "inactive"
)

// ProductView e o DTO completo de produto.
type ProductView struct {
	ID          string   `json:"id"`
	AccountID   string   `json:"accountId"`
	SourceID    string   `json:"sourceId,omitempty"`
	SourceLabel string   `json:"sourceLabel"`
	Name        string   `json:"name"`
	Code        string   `json:"code"`
	Description string   `json:"description"`
	Image       string   `json:"image"`
	Categories  []string `json:"categories"`
	Campaigns   []string `json:"campaigns"`
	Price       float64  `json:"price"`
	Fator       float64  `json:"fator"`
	Tipo        string   `json:"tipo"`
	Stock       int      `json:"stock"`
	Status      string   `json:"status"`
	// Campos do cruzamento com o ERP (site.product_erp_links). ErpSynced indica
	// se o produto tem ao menos um link; ErpName/ErpDescription trazem o nome/
	// descricao do primeiro link (por erp_sku). NAO sobrescrevem Name/Description.
	ErpSynced      bool      `json:"erpSynced"`
	ErpName        string    `json:"erpName"`
	ErpDescription string    `json:"erpDescription"`
	CreatedAt      time.Time `json:"createdAt"`
	UpdatedAt      time.Time `json:"updatedAt"`
}

// ProductListFilter parametriza GET /v1/admin/products.
type ProductListFilter struct {
	AccountID string
	Q         string
	Status    string
	Category  string
	Campaign  string
	Page      int
	PerPage   int
}

// ProductListResponse e o body de GET /v1/admin/products.
type ProductListResponse struct {
	Products []ProductView `json:"products"`
	Total    int           `json:"total"`
	Page     int           `json:"page"`
	PerPage  int           `json:"perPage"`
}

// ProductCreateInput e o body de POST /v1/admin/products.
type ProductCreateInput struct {
	Name        string   `json:"name"`
	Code        string   `json:"code"`
	Description string   `json:"description"`
	Image       string   `json:"image"`
	Categories  []string `json:"categories"`
	Campaigns   []string `json:"campaigns"`
	Price       float64  `json:"price"`
	Fator       float64  `json:"fator"`
	Tipo        string   `json:"tipo"`
	Stock       int      `json:"stock"`
}

// ProductUpdateInput e o body de PATCH /v1/admin/products/:id.
type ProductUpdateInput struct {
	Name        *string  `json:"name"`
	Code        *string  `json:"code"`
	Description *string  `json:"description"`
	Image       *string  `json:"image"`
	Categories  []string `json:"categories"`
	Campaigns   []string `json:"campaigns"`
	Price       *float64 `json:"price"`
	Fator       *float64 `json:"fator"`
	Tipo        *string  `json:"tipo"`
	Stock       *int     `json:"stock"`
	Status      *string  `json:"status"`
}

// ============================================================================
// Product sources (sync de catalogo externo — B8)
// ============================================================================

// ProductSource e a config de uma fonte externa de produtos de uma account.
type ProductSource struct {
	ID        string `json:"id"`
	AccountID string `json:"accountId"`
	Type      string `json:"type"`
	BaseURL   string `json:"baseUrl"`
	Enabled   bool   `json:"enabled"`
}

// productSourceType e o tipo de fonte alvo do toggle local/online (a fonte de
// catalogo externa puxada pelo sync).
const productSourceType = "external_api"

// URLs conhecidas do toggle de fonte de produtos (reusam perolaBaseHost e
// dockerInternalHost de perola_client.go):
//   - online: API publica do site do cliente.
//   - local:  XAMPP no host, alcancado pelo container via host.docker.internal.
const (
	productSourceURLOnline = perolaBaseHost + "/api/products/"
	productSourceURLLocal  = "http://" + dockerInternalHost + "/painel-perola/api/products/"
)

// Modos derivados/aceitos pelo toggle de fonte de produtos.
const (
	productSourceModeLocal  = "local"
	productSourceModeOnline = "online"
	productSourceModeCustom = "custom"
)

// ProductSourceView e o body de GET /v1/admin/products/source. Mode e derivado
// do base_url da fonte external_api da account.
type ProductSourceView struct {
	Mode    string `json:"mode"`    // 'local' | 'online' | 'custom'
	BaseURL string `json:"baseUrl"` // base_url atual ('' se nao houver fonte)
}

// ProductSourceModeInput e o body de PATCH /v1/admin/products/source.
type ProductSourceModeInput struct {
	Mode string `json:"mode"` // 'local' | 'online'
}

// ProductUpsertItem e o produto ja mapeado da origem, pronto para o upsert em
// site.products. Source/ExternalID formam a chave de upsert por account.
type ProductUpsertItem struct {
	ExternalID  string
	Source      string
	Name        string
	Code        string
	Description string
	Image       string
	// ImageCandidates sao URLs ORDENADAS a tentar no cache de imagens do sync
	// (1a que responder 200 vence). Vazio => o cache usa so Image. Preenchido pela
	// fonte (ex.: perolaImageCandidates) quando a origem devolve so o nome do
	// arquivo e o caminho real precisa de heuristica (segmento + variantes).
	ImageCandidates []string
	Categories      []string
	Campaigns       []string
	Price           float64
	Fator           float64
	Tipo            string
	Stock           int
	Status          string // 'active' | 'inactive'
	Deleted         bool   // true => marcar inativo (deleted_at na origem)
}

// ProductSyncResult e o retorno de SyncProducts / POST products/sync.
type ProductSyncResult struct {
	Inserted     int `json:"inserted"`
	Updated      int `json:"updated"`
	Skipped      int `json:"skipped"`
	ImagesCached int `json:"imagesCached"`
}

// ProductSourceRepository abstrai persistencia das fontes externas e o upsert.
type ProductSourceRepository interface {
	ListByAccount(ctx context.Context, accountID string) ([]ProductSource, error)
	UpsertProducts(ctx context.Context, accountID string, items []ProductUpsertItem) (ProductSyncResult, error)
	// GetAccountSource retorna a fonte external_api da account (a 1a/unica). Se
	// nao houver linha, retorna ErrNoProductSource.
	GetAccountSource(ctx context.Context, accountID string) (ProductSource, error)
	// SetAccountSourceBaseURL atualiza o base_url da fonte external_api da account.
	// Se nao existir linha, retorna ErrNoProductSource.
	SetAccountSourceBaseURL(ctx context.Context, accountID, baseURL string) error
}

// ============================================================================
// ERP cross-match (site.product_erp_links x erp_item_current)
// ============================================================================

// ErpMatchResult e o retorno de MatchERP / POST products/erp-match.
type ErpMatchResult struct {
	Matched  int `json:"matched"`  // numero de links (produto x sku)
	Products int `json:"products"` // numero de produtos com ao menos 1 link
}

// ErpUnmatchedItem e um item do ERP (erp_item_current) que ainda nao casa com
// nenhum segmento de code de produto ativo da account.
type ErpUnmatchedItem struct {
	Sku         string `json:"sku"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

// ErpUnmatchedFilter parametriza GET /v1/admin/products/erp-unmatched.
type ErpUnmatchedFilter struct {
	AccountID string
	Q         string
	Page      int
	PerPage   int
}

// ErpUnmatchedListResponse e o body de GET /v1/admin/products/erp-unmatched.
type ErpUnmatchedListResponse struct {
	Items   []ErpUnmatchedItem `json:"items"`
	Total   int                `json:"total"`
	Page    int                `json:"page"`
	PerPage int                `json:"perPage"`
}

// ProductFromErpInput e o body de POST /v1/admin/products/from-erp.
type ProductFromErpInput struct {
	Sku string `json:"sku"`
}

// ProductErpRepository abstrai o cruzamento de produtos do site com o ERP.
type ProductErpRepository interface {
	// MatchERP faz o upsert/limpeza de site.product_erp_links da account inteira.
	MatchERP(ctx context.Context, accountID string) (ErpMatchResult, error)
	// MatchERPForProduct refaz os links de UM produto especifico (apos criacao).
	MatchERPForProduct(ctx context.Context, accountID, productID string) error
	// ListUnmatched lista itens do ERP sem casamento com code de produto ativo.
	ListUnmatched(ctx context.Context, filter ErpUnmatchedFilter) ([]ErpUnmatchedItem, int, error)
	// FindErpItem busca um item do ERP por sku dentro do tenant (== account).
	FindErpItem(ctx context.Context, accountID, sku string) (ErpUnmatchedItem, error)
}

// ============================================================================
// Webhook sources
// ============================================================================

// WebhookSourceView e o DTO de uma fonte de webhook.
type WebhookSourceView struct {
	ID         string    `json:"id"`
	AccountID  string    `json:"accountId"`
	Slug       string    `json:"slug"`
	Name       string    `json:"name"`
	EntityType string    `json:"entityType"`
	IsActive   bool      `json:"isActive"`
	CreatedAt  time.Time `json:"createdAt"`
	UpdatedAt  time.Time `json:"updatedAt"`
}

// WebhookSourceCreateInput e o body de POST /v1/admin/webhook-sources.
type WebhookSourceCreateInput struct {
	Slug       string `json:"slug"`
	Name       string `json:"name"`
	EntityType string `json:"entityType"` // 'leads' | 'products' | 'tracking'
}

// WebhookSourceCreatedResponse inclui o secret revelado APENAS uma vez na
// criacao (depois fica so o hash no banco).
type WebhookSourceCreatedResponse struct {
	Source WebhookSourceView `json:"source"`
	Secret string            `json:"secret"`
}

// WebhookSourceRotateResponse e o body do POST /v1/admin/webhook-sources/:id/rotate.
type WebhookSourceRotateResponse struct {
	Secret string `json:"secret"`
}

// ============================================================================
// Repositories
// ============================================================================

// LeadRepository abstrai persistencia para leads.
type LeadRepository interface {
	List(ctx context.Context, filter LeadListFilter) ([]LeadView, int, error)
	Find(ctx context.Context, accountID, leadID string) (LeadView, error)
	Create(ctx context.Context, accountID string, input LeadCreateInput) (LeadView, error)
	CreateFromWebhook(ctx context.Context, accountID, sourceID, sourceLabel string, fields map[string]any, raw string) (LeadView, error)
	Update(ctx context.Context, accountID, leadID string, input LeadUpdateInput) (LeadView, error)
	SoftDelete(ctx context.Context, accountID, leadID string) error
}

// ProductRepository abstrai persistencia para produtos do site.
type ProductRepository interface {
	List(ctx context.Context, filter ProductListFilter) ([]ProductView, int, error)
	Find(ctx context.Context, accountID, productID string) (ProductView, error)
	Create(ctx context.Context, accountID string, input ProductCreateInput) (ProductView, error)
	CreateFromErp(ctx context.Context, accountID string, item ErpUnmatchedItem) (ProductView, error)
	CreateFromWebhook(ctx context.Context, accountID, sourceID, sourceLabel string, fields map[string]any, raw string) (ProductView, error)
	Update(ctx context.Context, accountID, productID string, input ProductUpdateInput) (ProductView, error)
	SoftDelete(ctx context.Context, accountID, productID string) error
}

// WebhookSourceRepository abstrai persistencia de fontes.
//
// Sobre o secret: e armazenado em claro porque HMAC do webhook usa o secret
// como chave (nao da pra validar a partir do hash). Tratado como dado sensivel
// equivalente a password_hash: nunca logar, so retornar via Create/Rotate.
type WebhookSourceRepository interface {
	List(ctx context.Context, accountID string) ([]WebhookSourceView, error)
	Find(ctx context.Context, accountID, sourceID string) (WebhookSourceView, error)
	FindBySlug(ctx context.Context, slug string) (WebhookSourceView, string, error) // retorna view + secret em claro
	Create(ctx context.Context, accountID string, input WebhookSourceCreateInput, secret string) (WebhookSourceView, error)
	UpdateSecret(ctx context.Context, sourceID, secret string) error
	SoftDelete(ctx context.Context, accountID, sourceID string) error
}
