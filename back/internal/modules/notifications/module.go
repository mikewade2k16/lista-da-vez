package notifications

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

func New(service *Service) *Module {
	return &Module{service: service}
}

func (module *Module) ID() string {
	return "notifications"
}

func (module *Module) Metadata() modules.Metadata {
	return modules.Metadata{
		SchemaName:      "notifications",
		Label:           "Notifications",
		Description:     "Notificacoes in-app com preferencias, mutes e adapters externos stubados.",
		RequiresModules: []string{"core"},
		SortOrder:       55,
	}
}

func (module *Module) Permissions() []modules.PermissionDef {
	return []modules.PermissionDef{
		{
			Key:         PermNotificationsRead,
			Label:       "Ler notificacoes",
			Description: "Listar e marcar notificacoes pessoais como lidas.",
			Scope:       "account",
		},
		{
			Key:         PermNotificationsPreferencesManage,
			Label:       "Gerenciar preferencias de notificacao",
			Description: "Editar preferencias pessoais de canal e silenciar recursos.",
			Scope:       "account",
		},
	}
}

func (module *Module) RoleTemplates() []modules.RoleTemplateDef {
	return []modules.RoleTemplateDef{
		{
			ID:          "notifications.member",
			Label:       "Notifications - Membro",
			Description: "Consulta a propria caixa de notificacoes e ajusta preferencias pessoais.",
			IsSystem:    true,
			SortOrder:   80,
			Permissions: []string{PermNotificationsRead, PermNotificationsPreferencesManage},
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
	return "notifications"
}

func (handle *handle) RegisterRoutes(mux *http.ServeMux) {
	RegisterRoutes(mux, handle.service, handle.authMiddleware)
}

func (handle *handle) RegisterEventHandlers(_ events.Bus) {}

func (handle *handle) Close() error {
	return nil
}
