package site

import (
	"net/http"

	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/events"
	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/modules"
)

// Module e o adaptador do modulo `site` para o Module Registry.
//
// Responsavel por leads + products + webhook sources do site publico do
// tenant. Dados podem entrar via API admin (manual) ou via POST webhook
// autenticado por HMAC.
type Module struct {
	handle *handle
}

// New cria um Module pronto para registrar no Registry.
func New() *Module {
	return &Module{}
}

func (m *Module) ID() string { return "site" }

func (m *Module) Metadata() modules.Metadata {
	return modules.Metadata{
		SchemaName:  "site",
		Label:       "Site",
		Description: "Leads, produtos e tracking do site publico do tenant. Ingest via webhook + admin CRUD.",
		IsCore:      false,
		SortOrder:   40,
	}
}

func (m *Module) Permissions() []modules.PermissionDef {
	return []modules.PermissionDef{
		{Key: "site.leads.view", Label: "Ver leads", Scope: "account"},
		{Key: "site.leads.manage", Label: "Gerenciar leads", Scope: "account"},
		{Key: "site.products.view", Label: "Ver produtos do site", Scope: "account"},
		{Key: "site.products.manage", Label: "Gerenciar produtos do site", Scope: "account"},
		{Key: "site.tracking.view", Label: "Ver tracking do site", Scope: "account"},
		{Key: "site.webhooks.manage", Label: "Gerenciar webhook sources", Scope: "account"},
	}
}

func (m *Module) RoleTemplates() []modules.RoleTemplateDef {
	return []modules.RoleTemplateDef{
		{
			ID:          "site.manager",
			Label:       "Gestor do Site",
			Description: "CRUD completo de leads, produtos, tracking e webhook sources do site.",
			SortOrder:   100,
			Permissions: []string{
				"site.leads.view",
				"site.leads.manage",
				"site.products.view",
				"site.products.manage",
				"site.tracking.view",
				"site.webhooks.manage",
			},
		},
		{
			ID:          "site.viewer",
			Label:       "Leitor do Site",
			Description: "Apenas leitura de leads, produtos e tracking.",
			SortOrder:   110,
			Permissions: []string{
				"site.leads.view",
				"site.products.view",
				"site.tracking.view",
			},
		},
	}
}

func (m *Module) Build(deps modules.Dependencies) (modules.Handle, error) {
	leads := NewPostgresLeadRepository(deps.Pool)
	products := NewPostgresProductRepository(deps.Pool)
	sources := NewPostgresWebhookSourceRepository(deps.Pool)
	tracking := NewPostgresTrackingRepository(deps.Pool)
	svc := NewService(leads, products, sources, tracking)

	m.handle = &handle{
		service:        svc,
		authMiddleware: deps.AuthMiddleware,
	}
	return m.handle, nil
}

// ============================================================================
// Handle
// ============================================================================

type handle struct {
	service        *Service
	authMiddleware *auth.Middleware
}

func (h *handle) ID() string { return "site" }

// RegisterRoutes monta admin (JWT) + ingest (HMAC).
func (h *handle) RegisterRoutes(mux *http.ServeMux) {
	RegisterAdminRoutes(mux, h.service, h.authMiddleware)
	RegisterIngestRoutes(mux, h.service)
}

// RegisterEventHandlers — site nao consome eventos por enquanto.
func (h *handle) RegisterEventHandlers(_ events.Bus) {}

func (h *handle) Close() error { return nil }
