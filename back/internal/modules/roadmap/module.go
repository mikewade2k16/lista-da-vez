package roadmap

import (
	"net/http"

	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/events"
	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/modules"
)

type Module struct {
	handle  *handle
	service *Service
}

func New() *Module {
	return &Module{}
}

func (module *Module) ID() string {
	return "roadmap"
}

func (module *Module) Metadata() modules.Metadata {
	return modules.Metadata{
		SchemaName:      "roadmap",
		Label:           "Roadmap",
		Description:     "Inventario editavel de modulos/paginas pendentes e regras canonicas para agentes (AGENT_RULES.md).",
		RequiresModules: []string{"core"},
		SortOrder:       90,
	}
}

func (module *Module) Permissions() []modules.PermissionDef {
	return []modules.PermissionDef{
		{
			Key:         PermRoadmapView,
			Label:       "Visualizar roadmap",
			Description: "Ver modulos e regras na aba /roadmap.",
			Scope:       "account",
		},
		{
			Key:         PermRoadmapManage,
			Label:       "Gerenciar roadmap",
			Description: "Criar, editar e remover modulos/regras (override por account).",
			Scope:       "account",
		},
	}
}

func (module *Module) RoleTemplates() []modules.RoleTemplateDef {
	return []modules.RoleTemplateDef{
		{
			ID:          "roadmap.viewer",
			Label:       "Roadmap - Leitor",
			Description: "Acessa aba Roadmap em modo leitura.",
			IsSystem:    true,
			SortOrder:   90,
			Permissions: []string{PermRoadmapView},
		},
		{
			ID:          "roadmap.admin",
			Label:       "Roadmap - Admin",
			Description: "Edita modulos e regras do roadmap.",
			IsSystem:    true,
			SortOrder:   91,
			Permissions: []string{PermRoadmapView, PermRoadmapManage},
		},
	}
}

func (module *Module) Build(deps modules.Dependencies) (modules.Handle, error) {
	if module.service == nil {
		module.service = NewService(NewPostgresRepository(deps.Pool))
	}
	module.handle = &handle{
		service:        module.service,
		authMiddleware: deps.AuthMiddleware,
	}
	return module.handle, nil
}

type handle struct {
	service        *Service
	authMiddleware *auth.Middleware
}

func (handle *handle) ID() string {
	return "roadmap"
}

func (handle *handle) RegisterRoutes(mux *http.ServeMux) {
	RegisterRoutes(mux, handle.service, handle.authMiddleware)
}

func (handle *handle) RegisterEventHandlers(_ events.Bus) {}

func (handle *handle) Close() error {
	return nil
}
