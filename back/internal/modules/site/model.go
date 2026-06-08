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
	ID          string    `json:"id"`
	AccountID   string    `json:"accountId"`
	SourceID    string    `json:"sourceId,omitempty"`
	SourceLabel string    `json:"sourceLabel"`
	Name        string    `json:"name"`
	Code        string    `json:"code"`
	Description string    `json:"description"`
	Image       string    `json:"image"`
	Categories  []string  `json:"categories"`
	Campaigns   []string  `json:"campaigns"`
	Price       float64   `json:"price"`
	Fator       float64   `json:"fator"`
	Tipo        string    `json:"tipo"`
	Stock       int       `json:"stock"`
	Status      string    `json:"status"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

// ProductListFilter parametriza GET /v1/admin/products.
type ProductListFilter struct {
	AccountID string
	Q         string
	Status    string
	Category  string
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
