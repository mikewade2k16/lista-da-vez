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
	leads    LeadRepository
	products ProductRepository
	sources  WebhookSourceRepository
	tracking TrackingRepository
}

// NewService cria o service.
func NewService(leads LeadRepository, products ProductRepository, sources WebhookSourceRepository, tracking TrackingRepository) *Service {
	return &Service{leads: leads, products: products, sources: sources, tracking: tracking}
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
