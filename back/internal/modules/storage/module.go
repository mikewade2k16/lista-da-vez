package storage

import (
	"net/http"

	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/events"
	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/modules"
)

type Module struct {
	config  Config
	handle  *handle
	service *Service
}

func New(cfg Config) *Module {
	return &Module{config: cfg}
}

func (module *Module) ID() string { return "storage" }

func (module *Module) Metadata() modules.Metadata {
	return modules.Metadata{
		SchemaName:      "storage",
		Label:           "Storage",
		Description:     "Storage privado compartilhado com cotas globais e adapter Cloudflare R2.",
		IsCore:          true,
		RequiresModules: []string{"core"},
		SortOrder:       5,
	}
}

func (module *Module) Permissions() []modules.PermissionDef { return nil }

func (module *Module) RoleTemplates() []modules.RoleTemplateDef { return nil }

func (module *Module) Build(deps modules.Dependencies) (modules.Handle, error) {
	if err := ValidateConfig(module.config); err != nil {
		return nil, err
	}
	var client ObjectClient
	var usageClient UsageClient
	if module.config.Enabled {
		r2Client, err := NewR2Client(module.config)
		if err != nil {
			return nil, err
		}
		client = r2Client
		if module.config.AnalyticsToken != "" {
			usageClient = NewCloudflareUsageClient(module.config)
		}
	}
	module.service = NewService(module.config, NewPostgresRepository(deps.Pool), client, usageClient)
	module.handle = &handle{service: module.service, authMiddleware: deps.AuthMiddleware}
	return module.handle, nil
}

func (module *Module) Service() *Service { return module.service }

type handle struct {
	service        *Service
	authMiddleware *auth.Middleware
}

func (handle *handle) ID() string { return "storage" }

func (handle *handle) RegisterRoutes(mux *http.ServeMux) {
	RegisterRoutes(mux, handle.service, handle.authMiddleware)
}

func (handle *handle) RegisterEventHandlers(_ events.Bus) {}

func (handle *handle) Close() error { handle.service.Close(); return nil }
