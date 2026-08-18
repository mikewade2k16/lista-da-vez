package contentoperations

import (
	"net/http"

	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/events"
	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/modules"
)

type Module struct {
	scope   ScopeProvider
	access  AccessProvider
	handle  *handle
	service *Service
}

func New(scope ScopeProvider, access AccessProvider) *Module {
	return &Module{scope: scope, access: access}
}
func (m *Module) ID() string { return "content_operations" }
func (m *Module) Metadata() modules.Metadata {
	return modules.Metadata{SchemaName: "core", Label: "Operação de conteúdo", Description: "Alertas proativos calculados a partir de Tasks e Calendário.", RequiresModules: []string{"core", "tasks", "calendar"}, SortOrder: 47}
}
func (m *Module) Permissions() []modules.PermissionDef     { return nil }
func (m *Module) RoleTemplates() []modules.RoleTemplateDef { return nil }
func (m *Module) Build(deps modules.Dependencies) (modules.Handle, error) {
	m.service = NewService(NewPostgresRepository(deps.Pool), m.scope, m.access)
	m.handle = &handle{service: m.service, auth: deps.AuthMiddleware}
	return m.handle, nil
}
func (m *Module) Service() *Service { return m.service }

type handle struct {
	service *Service
	auth    *auth.Middleware
}

func (h *handle) ID() string                        { return "content_operations" }
func (h *handle) RegisterRoutes(mux *http.ServeMux) { RegisterRoutes(mux, h.service, h.auth) }
func (h *handle) RegisterEventHandlers(events.Bus)  {}
func (h *handle) Close() error                      { return nil }
