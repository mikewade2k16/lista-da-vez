package socialpublishing

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/core"
	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/events"
	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/jobs"
	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/modules"
	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/secretbox"
)

const (
	publishOutboxTable    = ModuleID + ".outbox"
	analyticsOutboxTable  = ModuleID + ".analytics_outbox"
	publishWorkerPoolSize = 3
	analyticsJobTimeout   = 3 * time.Minute
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
	publishOutbox, err := jobs.NewPostgresStore(deps.Pool, publishOutboxTable)
	if err != nil {
		return nil, err
	}
	analyticsOutbox, err := jobs.NewPostgresStore(deps.Pool, analyticsOutboxTable)
	if err != nil {
		return nil, err
	}
	store := NewStore(deps.Pool)
	permissions := core.NewRBACService(core.NewPostgresRBACRepository(deps.Pool))
	service := NewService(
		store,
		provider,
		m.secrets,
		analyticsOutbox,
		WithServicePermissionChecker(permissions),
	)
	gate := newPermissionGate(permissions)
	publishHandler := NewPublishHandler(store, provider, m.secrets, deps.ModulesGuard, deps.Logger)
	analyticsHandler := NewAnalyticsHandler(store, provider, m.secrets, deps.ModulesGuard)
	legacyAnalyticsHandler := NewAnalyticsForwardHandler(analyticsOutbox)
	publishWorkers := make([]*jobs.Worker, 0, publishWorkerPoolSize)
	for _, config := range publishWorkerConfigs(deps.Logger) {
		worker := jobs.New(publishOutbox, config)
		for kind, handler := range publishLaneHandlers(publishHandler, legacyAnalyticsHandler) {
			worker.Register(kind, handler)
		}
		publishWorkers = append(publishWorkers, worker)
	}
	analyticsWorker := jobs.New(
		analyticsOutbox,
		analyticsWorkerConfig(deps.Logger),
	)
	analyticsWorker.Register(
		AnalyticsJobKind,
		analyticsHandler,
	)
	workerContext, cancelWorker := context.WithCancel(context.Background())
	for _, worker := range publishWorkers {
		worker.Start(workerContext)
	}
	analyticsWorker.Start(workerContext)

	m.handle = &handle{
		service:         service,
		gate:            gate,
		authMiddleware:  deps.AuthMiddleware,
		publishWorkers:  publishWorkers,
		analyticsWorker: analyticsWorker,
		cancelWorker:    cancelWorker,
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
	service         *Service
	gate            *permissionGate
	authMiddleware  *auth.Middleware
	publishWorkers  []*jobs.Worker
	analyticsWorker *jobs.Worker
	cancelWorker    context.CancelFunc
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
	var err error
	for _, worker := range h.publishWorkers {
		if worker != nil {
			err = errors.Join(err, worker.Close())
		}
	}
	if h.analyticsWorker != nil {
		err = errors.Join(err, h.analyticsWorker.Close())
	}
	return err
}

func publishWorkerConfigs(logger *slog.Logger) []jobs.Config {
	configs := make([]jobs.Config, 0, publishWorkerPoolSize)
	for index := 1; index <= publishWorkerPoolSize; index++ {
		configs = append(configs, jobs.Config{
			WorkerID: fmt.Sprintf("%s-publish-%d", ModuleID, index),
			Batch:    1,
			Logger:   logger,
		})
	}
	return configs
}

func publishLaneHandlers(
	publishHandler jobs.Handler,
	analyticsHandler jobs.Handler,
) map[string]jobs.Handler {
	return map[string]jobs.Handler{
		PublishJobKind:   publishHandler,
		AnalyticsJobKind: analyticsHandler,
	}
}

func analyticsWorkerConfig(logger *slog.Logger) jobs.Config {
	return jobs.Config{
		WorkerID:   ModuleID + "-analytics",
		Batch:      1,
		JobTimeout: analyticsJobTimeout,
		Logger:     logger,
	}
}
