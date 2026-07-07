package finance

import (
	"net/http"

	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/events"
	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/modules"
)

// Module e o adaptador do modulo `finance` para o Module Registry.
//
// Planilhas financeiras mensais por cliente: entradas/saidas, efetivacao,
// ajustes, contas fixas, categorias e recorrencias. Substitui o mock BFF Nitro
// (web/server/*) por back Go real (ADR 0002). Plano: docs/finance/PLANO_MODULO_FINANCE.md.
type Module struct {
	handle *handle
}

// New cria um Module pronto para registrar no Registry.
func New() *Module {
	return &Module{}
}

func (m *Module) ID() string { return "finance" }

func (m *Module) Metadata() modules.Metadata {
	return modules.Metadata{
		SchemaName:  "finance",
		Label:       "Finance",
		Description: "Planilhas financeiras mensais por cliente: entradas/saidas, efetivacao, ajustes, contas fixas, categorias e recorrencias.",
		IsCore:      false,
		SortOrder:   50,
	}
}

func (m *Module) Permissions() []modules.PermissionDef {
	return []modules.PermissionDef{
		{Key: "finance.sheets.view", Label: "Ver planilhas financeiras", Scope: "account"},
		{Key: "finance.sheets.manage", Label: "Gerenciar planilhas financeiras", Scope: "account"},
		{Key: "finance.config.view", Label: "Ver configuracao financeira", Scope: "account"},
		{Key: "finance.config.manage", Label: "Gerenciar configuracao financeira", Scope: "account"},
		{Key: "finance.recurring.manage", Label: "Gerenciar recorrencias", Scope: "account"},
	}
}

func (m *Module) RoleTemplates() []modules.RoleTemplateDef {
	return []modules.RoleTemplateDef{
		{
			ID:          "finance.manager",
			Label:       "Gestor Financeiro",
			Description: "CRUD completo de planilhas, configuracao e recorrencias.",
			SortOrder:   100,
			Permissions: []string{
				"finance.sheets.view",
				"finance.sheets.manage",
				"finance.config.view",
				"finance.config.manage",
				"finance.recurring.manage",
			},
		},
		{
			ID:          "finance.viewer",
			Label:       "Leitor Financeiro",
			Description: "Apenas leitura de planilhas e configuracao.",
			SortOrder:   110,
			Permissions: []string{
				"finance.sheets.view",
				"finance.config.view",
			},
		},
	}
}

func (m *Module) Build(deps modules.Dependencies) (modules.Handle, error) {
	sheets := NewPostgresSheetStore(deps.Pool)
	config := NewPostgresConfigStore(deps.Pool)
	svc := NewService(sheets, config)

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

func (h *handle) ID() string { return "finance" }

// RegisterRoutes monta /v1/finance/* (RequireAuth; gating de modulo no Chain).
func (h *handle) RegisterRoutes(mux *http.ServeMux) {
	RegisterRoutes(mux, h.service, h.authMiddleware)
}

// RegisterEventHandlers — finance nao consome eventos por enquanto.
func (h *handle) RegisterEventHandlers(_ events.Bus) {}

func (h *handle) Close() error { return nil }
