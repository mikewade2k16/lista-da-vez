package calendar

import (
	"net/http"

	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/events"
	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/modules"
)

// Module e o adaptador do modulo `calendar` para o Module Registry.
//
// Agenda de conteudo por cliente: o painel Omni faz o CRUD dos eventos (por
// account) e das notas por mes. Plano: docs/CALENDARIO_PLAN.md.
type Module struct {
	handle  *handle
	storage MediaStorage
}

// New cria um Module pronto para registrar no Registry. storage grava os anexos
// (imagem/video) em disco (uploads/calendar/{account}/); pode ser nil sem upload.
func New(storage MediaStorage) *Module {
	return &Module{storage: storage}
}

func (m *Module) ID() string { return "calendar" }

func (m *Module) Metadata() modules.Metadata {
	return modules.Metadata{
		SchemaName:  "calendar",
		Label:       "Calendario",
		Description: "Agenda de conteudo por cliente (eventos + notas por mes).",
		IsCore:      false,
		SortOrder:   46,
	}
}

func (m *Module) Permissions() []modules.PermissionDef {
	return []modules.PermissionDef{
		{Key: "calendar.view", Label: "Ver calendario", Scope: "account"},
		{Key: "calendar.manage", Label: "Gerenciar calendario", Scope: "account"},
	}
}

func (m *Module) RoleTemplates() []modules.RoleTemplateDef {
	return []modules.RoleTemplateDef{
		{
			ID:          "calendar.manager",
			Label:       "Gestor de Calendario",
			Description: "Cria e edita eventos e notas do calendario da account.",
			SortOrder:   100,
			Permissions: []string{"calendar.view", "calendar.manage"},
		},
		{
			ID:          "calendar.viewer",
			Label:       "Leitor de Calendario",
			Description: "Apenas leitura do calendario da account.",
			SortOrder:   110,
			Permissions: []string{"calendar.view"},
		},
	}
}

func (m *Module) Build(deps modules.Dependencies) (modules.Handle, error) {
	svc := NewService(NewStore(deps.Pool), m.storage)
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

func (h *handle) ID() string { return "calendar" }

func (h *handle) RegisterRoutes(mux *http.ServeMux) {
	RegisterRoutes(mux, h.service, h.authMiddleware)
	RegisterMediaRoutes(mux, h.service, h.authMiddleware)
}

func (h *handle) RegisterEventHandlers(_ events.Bus) {}

func (h *handle) Close() error { return nil }
