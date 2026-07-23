package socialpublishing

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/core"
	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/events"
	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/jobs"
	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/modules"
	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/secretbox"
)

type Module struct {
	handle   *handle
	secrets  *secretbox.Box
	provider InstagramProvider
}

type Option func(*Module)

func WithSecretBox(box *secretbox.Box) Option {
	return func(module *Module) {
		module.secrets = box
	}
}

func WithInstagramProvider(provider InstagramProvider) Option {
	return func(module *Module) {
		module.provider = provider
	}
}

func New(options ...Option) *Module {
	module := &Module{}
	for _, option := range options {
		if option != nil {
			option(module)
		}
	}
	return module
}

func (m *Module) ID() string {
	return ModuleID
}

func (m *Module) Metadata() modules.Metadata {
	return modules.Metadata{
		SchemaName:  ModuleID,
		Label:       "Agendamento de Postagens",
		Description: "Agenda, publica e acompanha analytics de posts profissionais do Instagram.",
		IsCore:      false,
		SortOrder:   48,
	}
}

func (m *Module) Permissions() []modules.PermissionDef {
	return []modules.PermissionDef{
		{Key: PermissionView, Label: "Ver postagens agendadas", Scope: "account"},
		{Key: PermissionManage, Label: "Gerenciar postagens", Scope: "account"},
		{Key: PermissionConnect, Label: "Conectar Instagram", Scope: "account"},
		{Key: PermissionAnalytics, Label: "Ver e sincronizar analytics", Scope: "account"},
	}
}

func (m *Module) RoleTemplates() []modules.RoleTemplateDef {
	return []modules.RoleTemplateDef{
		{
			ID:          ModuleID + ".manager",
			Label:       "Gestor de Postagens",
			Description: "Conecta o Instagram, agenda publicacoes e acompanha analytics.",
			SortOrder:   100,
			Permissions: []string{
				PermissionView,
				PermissionManage,
				PermissionConnect,
				PermissionAnalytics,
			},
		},
		{
			ID:          ModuleID + ".viewer",
			Label:       "Leitor de Postagens",
			Description: "Acompanha postagens e analytics sem alterar configuracoes.",
			SortOrder:   110,
			Permissions: []string{PermissionView, PermissionAnalytics},
		},
	}
}

func (m *Module) Build(deps modules.Dependencies) (modules.Handle, error) {
	if deps.Pool == nil {
		return nil, fmt.Errorf("social publishing: pool PostgreSQL obrigatorio")
	}
	if deps.AuthMiddleware == nil {
		return nil, fmt.Errorf("social publishing: auth middleware obrigatorio")
	}
	if deps.ModulesGuard == nil {
		return nil, fmt.Errorf("social publishing: modules guard obrigatorio")
	}
	if m.secrets == nil {
		return nil, ErrSecretsUnavailable
	}
	provider := m.provider
	if provider == nil {
		provider = NewInstagramGraphProvider(strings.TrimSpace(os.Getenv(GraphBaseEnv)))
	}
	outbox, err := jobs.NewPostgresStore(deps.Pool, ModuleID+".outbox")
	if err != nil {
		return nil, err
	}
	store := NewStore(deps.Pool)
	service := NewService(store, provider, m.secrets, outbox)
	gate := newPermissionGate(core.NewRBACService(core.NewPostgresRBACRepository(deps.Pool)))
	worker := jobs.New(outbox, jobs.Config{WorkerID: ModuleID, Logger: deps.Logger})
	worker.Register(
		PublishJobKind,
		NewPublishHandler(store, provider, m.secrets, deps.ModulesGuard, deps.Logger),
	)
	worker.Register(
		AnalyticsJobKind,
		NewAnalyticsHandler(store, provider, m.secrets, deps.ModulesGuard),
	)
	workerContext, cancelWorker := context.WithCancel(context.Background())
	worker.Start(workerContext)

	m.handle = &handle{
		service:        service,
		gate:           gate,
		authMiddleware: deps.AuthMiddleware,
		worker:         worker,
		cancelWorker:   cancelWorker,
	}
	return m.handle, nil
}

func (m *Module) Service() *Service {
	if m.handle == nil {
		return nil
	}
	return m.handle.service
}

type handle struct {
	service        *Service
	gate           *permissionGate
	authMiddleware *auth.Middleware
	worker         *jobs.Worker
	cancelWorker   context.CancelFunc
}

func (h *handle) ID() string {
	return ModuleID
}

func (h *handle) RegisterRoutes(mux *http.ServeMux) {
	RegisterRoutes(mux, h.service, h.authMiddleware, h.gate)
}

func (h *handle) RegisterEventHandlers(_ events.Bus) {}

func (h *handle) Close() error {
	if h.cancelWorker != nil {
		h.cancelWorker()
	}
	if h.worker != nil {
		return h.worker.Close()
	}
	return nil
}
