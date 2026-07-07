package calendar

import (
	"net/http"
	"os"
	"strings"

	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/tenants"
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
	// tasksProvider resolve o Service do modulo tasks de forma LAZY (contrato C10).
	// Injetado via WithTasksService no app.go; nil = integracao calendario<->tasks
	// desligada. Ver task_link.go.
	tasksProvider TasksServiceProvider
	// publisher e o transporte realtime do calendario (contrato C11). Injetado via
	// WithPublisher no app.go (o realtimeService); nil = canal realtime desligado.
	// Ver publisher.go.
	publisher Publisher
}

// Option configura o Module no construtor (padrao functional options).
type Option func(*Module)

// New cria um Module pronto para registrar no Registry. storage grava os anexos
// (imagem/video) em disco (uploads/calendar/{account}/); pode ser nil sem upload.
// opts injeta dependencias opcionais (ex.: WithTasksService para a integracao C10).
func New(storage MediaStorage, opts ...Option) *Module {
	m := &Module{storage: storage}
	for _, opt := range opts {
		if opt != nil {
			opt(m)
		}
	}
	return m
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
	// Envs do disparo de IA lidos no Build (mesmo padrao do modulo cardapio, que le
	// os proprios envs via os.Getenv). WebhookURL vazio => POST /ai/plan devolve
	// ai_not_configured (503); ServiceToken vazio => o callback publico devolve 503.
	// As CHAVES de API dos provedores NAO ficam aqui — vivem nas credentials do n8n.
	aiCfg := AIDispatchConfig{
		WebhookURL:   strings.TrimSpace(os.Getenv("CALENDAR_AI_WEBHOOK_URL")),
		ServiceToken: strings.TrimSpace(os.Getenv("CALENDAR_AI_SERVICE_TOKEN")),
		CallbackBase: strings.TrimSpace(os.Getenv("CALENDAR_AI_CALLBACK_BASE")),
	}
	// Runtime de contexto compartilhado (C9): reusa AUTOMATION_RUNTIME_TOKEN — o
	// MESMO token de servico do runtime do automation — para autenticar chamadas
	// service-to-service em /v1/runtime/calendar/context. Vazio => a rota responde
	// 503 runtime_not_configured (nao aceita chamada anonima).
	runtimeToken := strings.TrimSpace(os.Getenv("AUTOMATION_RUNTIME_TOKEN"))
	// Chat de IA do calendario (C7/C8): webhooks do n8n lidos do ambiente. Vazios =>
	// /chat/ask responde 503 chat_not_configured e /chat/transcribe responde 503
	// transcribe_not_configured (o proxy nunca inventa resposta). As CHAVES de API
	// dos provedores vivem nas credentials do n8n, nao aqui.
	chatWebhook := strings.TrimSpace(os.Getenv("CALENDAR_CHAT_WEBHOOK_URL"))
	transcribeWebhook := strings.TrimSpace(os.Getenv("CALENDAR_TRANSCRIBE_WEBHOOK_URL"))
	// Resolvedor de clientes visiveis do chat (contrato D2, Wave 4): reusa a MESMA lista
	// permission-scoped de /v1/tenants (tenants.Service -> scope_queries.go), sem duplicar
	// a query org-aware. Stateless (so o pool), construido aqui a partir de deps.Pool.
	clientScope := tenants.NewService(tenants.NewPostgresRepository(deps.Pool))
	svc := NewService(NewStore(deps.Pool), m.storage).
		WithAI(aiCfg, deps.Logger).
		WithChat(chatWebhook, transcribeWebhook).
		WithTasks(m.tasksProvider).
		WithPublisher(m.publisher).
		WithClientScope(clientScope)
	m.handle = &handle{
		service:        svc,
		authMiddleware: deps.AuthMiddleware,
		runtimeToken:   runtimeToken,
	}
	return m.handle, nil
}

// Service devolve o Service construido no Build (nil antes do Build). Usado pelo handler de
// sync invertido do WAVE 5 (task->evento) como provider LAZY: calendar.NewTaskSyncHandler(
// func() *calendar.Service { return calendarModule.Service() }) — imune a ordem de Build.
func (m *Module) Service() *Service {
	if m.handle == nil {
		return nil
	}
	return m.handle.service
}

// ============================================================================
// Handle
// ============================================================================

type handle struct {
	service        *Service
	authMiddleware *auth.Middleware
	// runtimeToken autentica /v1/runtime/calendar/context (token de servico, fora
	// do gate de modulo). Vazio => a rota responde 503 runtime_not_configured.
	runtimeToken string
}

func (h *handle) ID() string { return "calendar" }

func (h *handle) RegisterRoutes(mux *http.ServeMux) {
	RegisterRoutes(mux, h.service, h.authMiddleware)
	RegisterMediaRoutes(mux, h.service, h.authMiddleware)
	RegisterProfileRoutes(mux, h.service, h.authMiddleware)
	// Chat de IA do calendario /v1/calendar/chat/* (RequireAuth + accountScope;
	// proxy fino ao n8n, contratos C7/C8). Fica sob /v1/calendar (gate de modulo).
	RegisterChatRoutes(mux, h.service, h.authMiddleware)
	// Keys de IA /v1/calendar/ai-keys[/global] (Wave 3, SEC; RequireAuth; global exige
	// platform_admin). Status mascarado {set,last4} — a key crua nunca sai do server.
	RegisterSecretRoutes(mux, h.service, h.authMiddleware)
	// Override de IA por cliente /v1/calendar/ai-config/client (Wave 3.1, SEC+;
	// RequireAuthWithAccount — account-scoped e sensivel). So COMPORTAMENTO; key nunca sai.
	RegisterClientAIRoutes(mux, h.service, h.authMiddleware)
	// Painel /v1/calendar/ai/* (RequireAuth) + callback publico /v1/public/calendar-ai/*
	// (sem JWT, autenticado por X-Service-Token; fora do gate de modulo — o prefixo
	// /v1/public nao esta em moduleGatingRules).
	RegisterAIPlanRoutes(mux, h.service, h.authMiddleware)
	// Runtime de contexto compartilhado /v1/runtime/calendar/context (contrato C9;
	// sem JWT, autenticado por AUTOMATION_RUNTIME_TOKEN; fora do gate de modulo — o
	// prefixo /v1/runtime nao esta em moduleGatingRules).
	RegisterRuntimeRoutes(mux, h.service, h.runtimeToken)
}

func (h *handle) RegisterEventHandlers(_ events.Bus) {}

func (h *handle) Close() error { return nil }
