package tasks

import (
	"net/http"

	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/notifications"
	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/events"
	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/modules"
)

type Module struct {
	handle           *handle
	publisher        Publisher
	notifier         notifications.Notifier
	relationRegistry *modules.RelationRegistry
	videoStorage     TaskVideoStorage
	// syncRegistry avisa os modulos donos de recursos espelhados (calendar) quando uma task
	// muda (WAVE 5, E2). Injetado via WithRelationSync no app.go; nil = sync desligado.
	syncRegistry *modules.RelationSyncRegistry
}

func New(publisher Publisher, notifier notifications.Notifier, relationRegistry *modules.RelationRegistry, videoStorage TaskVideoStorage) *Module {
	return &Module{
		publisher:        publisher,
		notifier:         notifier,
		relationRegistry: relationRegistry,
		videoStorage:     videoStorage,
	}
}

// WithRelationSync injeta o registry de sync invertido (WAVE 5, E2), encadeavel no app.go
// apos o New. Feito por setter (nao no New) para nao mexer nos callers de teste do modulo.
func (module *Module) WithRelationSync(registry *modules.RelationSyncRegistry) *Module {
	module.syncRegistry = registry
	return module
}

func (module *Module) ID() string {
	return "tasks"
}

func (module *Module) Metadata() modules.Metadata {
	return modules.Metadata{
		SchemaName:      "tasks",
		Label:           "Tasks",
		Description:     "Orquestrador notion-like de tarefas, boards, views, tracking e compartilhamento com clientes.",
		RequiresModules: []string{"core"},
		OptionalModules: []string{"notifications"},
		SortOrder:       50,
	}
}

func (module *Module) Permissions() []modules.PermissionDef {
	return []modules.PermissionDef{
		{Key: PermBoardsView, Label: "Visualizar boards", Description: "Listar e abrir boards de tasks.", Scope: "account"},
		{Key: PermBoardsManage, Label: "Gerenciar boards", Description: "Criar e editar boards, colunas, campos e views.", Scope: "account"},
		{Key: PermTasksView, Label: "Visualizar tasks", Description: "Listar e abrir tasks da account.", Scope: "account"},
		{Key: PermTasksCreate, Label: "Criar tasks", Description: "Criar novas tasks dentro dos boards.", Scope: "account"},
		{Key: PermTasksEdit, Label: "Editar tasks", Description: "Editar campos e mover tasks entre colunas.", Scope: "account"},
		{Key: PermTasksDelete, Label: "Arquivar tasks", Description: "Arquivar tasks.", Scope: "account"},
		{Key: PermTasksAssign, Label: "Atribuir tasks", Description: "Atribuir responsaveis e participantes.", Scope: "account"},
		{Key: PermTasksComment, Label: "Comentar tasks", Description: "Adicionar comentarios em tasks acessiveis.", Scope: "account"},
		{Key: PermTrackingUse, Label: "Usar tracking", Description: "Iniciar, pausar, retomar e parar tracking proprio.", Scope: "account"},
		{Key: PermTrackingViewAll, Label: "Ver tracking geral", Description: "Ver tracking de outros usuarios e metricas agregadas.", Scope: "account"},
		{Key: PermRelationsManage, Label: "Gerenciar relations", Description: "Vincular tasks a recursos de outros modulos.", Scope: "account"},
		{Key: PermSharesManage, Label: "Gerenciar shares", Description: "Compartilhar tasks com contas cliente.", Scope: "account"},
		{Key: PermClientView, Label: "Visao cliente", Description: "Acesso externo limitado por shares explicitas.", Scope: "account"},
	}
}

func (module *Module) RoleTemplates() []modules.RoleTemplateDef {
	return []modules.RoleTemplateDef{
		{
			ID:          "tasks.admin",
			Label:       "Tasks - Admin",
			Description: "Administra boards, tasks, shares, tracking e relations do modulo Tasks.",
			IsSystem:    true,
			SortOrder:   50,
			Permissions: adminPermissions,
		},
		{
			ID:          "tasks.member",
			Label:       "Tasks - Membro",
			Description: "Trabalha em boards e tasks sem gerenciar estrutura ou shares.",
			IsSystem:    true,
			SortOrder:   60,
			Permissions: memberPermissions,
		},
		{
			ID:          "tasks.client_viewer",
			Label:       "Tasks - Cliente",
			Description: "Acompanha tasks compartilhadas de forma limitada.",
			IsSystem:    true,
			SortOrder:   70,
			Permissions: clientViewerPermissions,
		},
	}
}

func (module *Module) Build(deps modules.Dependencies) (modules.Handle, error) {
	repository := NewPostgresRepository(deps.Pool)
	service := NewService(repository, module.publisher, module.notifier, module.relationRegistry, module.videoStorage)
	service.SetLogger(deps.Logger)
	service.syncRegistry = module.syncRegistry // WAVE 5 (E2): sync invertido calendario<->tasks

	module.handle = &handle{
		service:        service,
		authMiddleware: deps.AuthMiddleware,
	}
	return module.handle, nil
}

// Service devolve o Service construido no Build (nil antes do Build). Usado por
// outros modulos que precisam do tasks como provider LAZY (ex.: calendar injeta
// via closure `func() *tasks.Service { return tasksModule.Service() }`, resolvida
// no primeiro uso e imune a ordem de Build no Registry).
func (module *Module) Service() *Service {
	if module.handle == nil {
		return nil
	}
	return module.handle.service
}

type handle struct {
	service        *Service
	authMiddleware *auth.Middleware
}

func (handle *handle) ID() string {
	return "tasks"
}

func (handle *handle) RegisterRoutes(mux *http.ServeMux) {
	RegisterRoutes(mux, handle.service, handle.authMiddleware)
}

func (handle *handle) RegisterEventHandlers(_ events.Bus) {}

func (handle *handle) Close() error {
	return nil
}
