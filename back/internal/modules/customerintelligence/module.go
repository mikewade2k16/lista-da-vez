package customerintelligence

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/core"
	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/events"
	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/httpapi"
	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/jobs"
	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/llm"
	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/modules"
	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/secretbox"
)

type Module struct {
	handle                 *handle
	secrets                *secretbox.Box
	llmClient              llm.Client
	clientAuthorizer       ClientScopeAuthorizer
	relationshipAuthorizer RelationshipScopeAuthorizer
	portfolioAuthorizer    PortfolioScopeAuthorizer
	sourceAdapters         map[string]SourceAdapter
}

type ModuleOption func(*Module)

func WithSecretBox(box *secretbox.Box) ModuleOption {
	return func(module *Module) { module.secrets = box }
}

func WithLLMClient(client llm.Client) ModuleOption {
	return func(module *Module) { module.llmClient = client }
}

func WithModuleClientScopeAuthorizer(authorizer ClientScopeAuthorizer) ModuleOption {
	return func(module *Module) { module.clientAuthorizer = authorizer }
}

func WithModulePortfolioScopeAuthorizer(authorizer PortfolioScopeAuthorizer) ModuleOption {
	return func(module *Module) { module.portfolioAuthorizer = authorizer }
}

func WithModuleRelationshipScopeAuthorizer(authorizer RelationshipScopeAuthorizer) ModuleOption {
	return func(module *Module) { module.relationshipAuthorizer = authorizer }
}

func WithModuleSourceAdapter(key string, adapter SourceAdapter) ModuleOption {
	return func(module *Module) {
		if validSourceKey(key) && adapter != nil {
			module.sourceAdapters[key] = adapter
		}
	}
}

func New(options ...ModuleOption) *Module {
	module := &Module{sourceAdapters: make(map[string]SourceAdapter)}
	for _, option := range options {
		if option != nil {
			option(module)
		}
	}
	return module
}

func (m *Module) ID() string { return ModuleID }

func (m *Module) Metadata() modules.Metadata {
	return modules.Metadata{
		SchemaName:      "intelligence",
		Label:           "Inteligencia de Clientes",
		Description:     "Contexto, fatos, prompts, agentes e recomendacoes com proveniencia por cliente.",
		IsCore:          false,
		SortOrder:       49,
		RequiresModules: []string{"customer_data"},
		OptionalModules: []string{"omnichannel", "crm", "calendar", "site"},
	}
}

func (m *Module) Permissions() []modules.PermissionDef {
	return []modules.PermissionDef{
		{Key: PermissionProfileView, Label: "Ver inteligencia do cliente", Scope: "account"},
		{Key: PermissionProfileManage, Label: "Gerenciar inteligencia do cliente", Scope: "account"},
		{Key: PermissionSourcesView, Label: "Ver fontes de inteligencia", Scope: "account"},
		{Key: PermissionSourcesManage, Label: "Gerenciar fontes de inteligencia", Scope: "account"},
		{Key: PermissionAgentsManage, Label: "Gerenciar agentes de inteligencia", Scope: "account"},
		{Key: PermissionPromptsView, Label: "Ver prompts de inteligencia", Scope: "account"},
		{Key: PermissionPromptsManage, Label: "Editar prompts de inteligencia", Scope: "account"},
		{Key: PermissionPromptsPublish, Label: "Publicar prompts de inteligencia", Scope: "account"},
		{Key: PermissionPromptsPlatform, Label: "Gerenciar guardrails globais", Scope: "platform"},
		{Key: PermissionRunsView, Label: "Ver execucoes de inteligencia", Scope: "account"},
		{Key: PermissionAuditView, Label: "Ver auditoria de inteligencia", Scope: "account"},
		{Key: PermissionPortfolioView, Label: "Ver oportunidades agregadas", Scope: "account"},
		{Key: PermissionPortfolioManage, Label: "Gerenciar oportunidades agregadas", Scope: "account"},
		{Key: PermissionPortfolioPlatform, Label: "Gerenciar policy global de portfolio", Scope: "platform"},
	}
}

func (m *Module) RoleTemplates() []modules.RoleTemplateDef {
	return []modules.RoleTemplateDef{
		{
			ID: ModuleID + ".manager", Label: "Gestor de Inteligencia",
			Description: "Configura fontes, perfis, agentes e prompts da account.",
			SortOrder:   100,
			Permissions: []string{
				PermissionProfileView, PermissionProfileManage,
				PermissionSourcesView, PermissionSourcesManage,
				PermissionAgentsManage, PermissionPromptsView,
				PermissionPromptsManage, PermissionPromptsPublish,
				PermissionRunsView, PermissionAuditView,
			},
		},
		{
			ID: ModuleID + ".viewer", Label: "Leitor de Inteligencia",
			Description: "Consulta perfis, fontes, prompts e execucoes sem alterar.",
			SortOrder:   110,
			Permissions: []string{
				PermissionProfileView, PermissionSourcesView,
				PermissionPromptsView, PermissionRunsView,
			},
		},
	}
}

func (m *Module) Build(deps modules.Dependencies) (modules.Handle, error) {
	if deps.Pool == nil {
		return nil, fmt.Errorf("customer intelligence: pool PostgreSQL obrigatorio")
	}
	if deps.AuthMiddleware == nil {
		return nil, fmt.Errorf("customer intelligence: auth middleware obrigatorio")
	}
	if deps.ModulesGuard == nil {
		return nil, fmt.Errorf("customer intelligence: modules guard obrigatorio")
	}
	if m.secrets == nil {
		return nil, ErrSecretsUnavailable
	}
	client := m.llmClient
	if client == nil {
		client = llm.New(deps.Logger)
	}
	repository := NewPostgresRepository(deps.Pool)
	runtimeJobStore, err := jobs.NewPostgresStore(deps.Pool, runtimeJobTable)
	if err != nil {
		return nil, err
	}
	serviceOptions := make([]ServiceOption, 0, 4)
	serviceOptions = append(serviceOptions, WithHeadlessJobEnqueuer(runtimeJobStore))
	if m.clientAuthorizer != nil {
		serviceOptions = append(serviceOptions, WithClientScopeAuthorizer(m.clientAuthorizer))
	}
	if m.portfolioAuthorizer != nil {
		serviceOptions = append(serviceOptions, WithPortfolioScopeAuthorizer(m.portfolioAuthorizer))
	}
	if m.relationshipAuthorizer != nil {
		serviceOptions = append(serviceOptions, WithRelationshipScopeAuthorizer(m.relationshipAuthorizer))
	}
	service := NewService(repository, m.secrets, client, serviceOptions...)
	for key, adapter := range m.sourceAdapters {
		_ = service.RegisterSourceAdapter(key, adapter)
	}
	sourceJobStore, err := jobs.NewPostgresStore(deps.Pool, sourceJobTable)
	if err != nil {
		return nil, err
	}
	sourceWorker := jobs.New(sourceJobStore, jobs.Config{
		WorkerID: ModuleID + "-sources", Batch: 5, Logger: deps.Logger,
	})
	sourceWorker.Register(sourceJobKind, NewSourceJobHandler(service))
	sourceWorker.Register(
		observationRetentionJobKind,
		NewObservationRetentionJobHandler(repository),
	)
	runtimeWorker := jobs.New(runtimeJobStore, jobs.Config{
		WorkerID: ModuleID + "-runtime", Batch: 3, Logger: deps.Logger,
	})
	runtimeWorker.Register(
		relationshipRefreshJobKind,
		NewRelationshipRefreshJobHandler(service),
	)
	workerContext, cancelWorker := context.WithCancel(context.Background())
	sourceWorker.Start(workerContext)
	runtimeWorker.Start(workerContext)
	StartObservationRetentionScheduler(
		workerContext,
		repository,
		sourceJobStore,
		deps.Logger,
	)

	m.handle = &handle{
		service:        service,
		gate:           newPermissionGate(core.NewRBACService(core.NewPostgresRBACRepository(deps.Pool))),
		authMiddleware: deps.AuthMiddleware,
		modulesGuard:   deps.ModulesGuard,
		sourceWorker:   sourceWorker,
		runtimeWorker:  runtimeWorker,
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

func (m *Module) Runtime() Runtime {
	if m.handle == nil {
		return nil
	}
	return m.handle.service
}

type handle struct {
	service        *Service
	gate           *permissionGate
	authMiddleware *auth.Middleware
	modulesGuard   *httpapi.AccountModulesGuard
	sourceWorker   *jobs.Worker
	runtimeWorker  *jobs.Worker
	cancelWorker   context.CancelFunc
}

func (h *handle) ID() string { return ModuleID }

func (h *handle) RegisterRoutes(mux *http.ServeMux) {
	RegisterRoutes(mux, h.service, h.authMiddleware, h.modulesGuard, h.gate)
}

func (h *handle) RegisterEventHandlers(_ events.Bus) {}

func (h *handle) Close() error {
	if h.cancelWorker != nil {
		h.cancelWorker()
	}
	var closeErr error
	if h.sourceWorker != nil {
		closeErr = h.sourceWorker.Close()
	}
	if h.runtimeWorker != nil {
		closeErr = errors.Join(closeErr, h.runtimeWorker.Close())
	}
	return closeErr
}

var _ modules.Module = (*Module)(nil)
var _ modules.Handle = (*handle)(nil)
