package finance

import (
	"context"
	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/llm"
	"net/http"

	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/events"
	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/modules"
	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/secretbox"
)

// Module e o adaptador do modulo `finance` para o Module Registry.
//
// Planilhas financeiras mensais por cliente: entradas/saidas, efetivacao,
// ajustes, contas fixas, categorias e recorrencias. Substitui o mock BFF Nitro
// (web/server/*) por back Go real (ADR 0002). Plano: docs/finance/PLANO_MODULO_FINANCE.md.
type Module struct {
	plusPorts         PlusPorts
	handle            *handle
	box               *secretbox.Box
	assistantResolver AssistantCredentialResolver
	originals         OriginalFiles
}

// WithOriginalFiles liga o storage privado dos originais de comprovantes.
func (m *Module) WithOriginalFiles(files OriginalFiles) *Module { m.originals = files; return m }

// New cria um Module pronto para registrar no Registry.
func New(box ...*secretbox.Box) *Module {
	m := &Module{}
	if len(box) > 0 {
		m.box = box[0]
	}
	return m
}

func (m *Module) ID() string { return "finance" }

func (m *Module) Metadata() modules.Metadata {
	return modules.Metadata{
		SchemaName:  "finance",
		Label:       "Finance",
		Description: "Planilhas financeiras mensais por cliente: entradas/saidas, efetivacao, ajustes, contas fixas, categorias e recorrencias.",
		IsCore:      false,
		SortOrder:   50,
	}
}

func (m *Module) Permissions() []modules.PermissionDef {
	return []modules.PermissionDef{
		{Key: "finance.sheets.view", Label: "Ver planilhas financeiras", Scope: "account"},
		{Key: "finance.sheets.manage", Label: "Gerenciar planilhas financeiras", Scope: "account"},
		{Key: "finance.config.view", Label: "Ver configuracao financeira", Scope: "account"},
		{Key: "finance.config.manage", Label: "Gerenciar configuracao financeira", Scope: "account"},
		{Key: "finance.recurring.manage", Label: "Gerenciar recorrencias", Scope: "account"},
	}
}

func (m *Module) RoleTemplates() []modules.RoleTemplateDef {
	return []modules.RoleTemplateDef{
		{
			ID:          "finance.manager",
			Label:       "Gestor Financeiro",
			Description: "CRUD completo de planilhas, configuracao e recorrencias.",
			SortOrder:   100,
			Permissions: []string{
				"finance.sheets.view",
				"finance.sheets.manage",
				"finance.config.view",
				"finance.config.manage",
				"finance.recurring.manage",
			},
		},
		{
			ID:          "finance.viewer",
			Label:       "Leitor Financeiro",
			Description: "Apenas leitura de planilhas e configuracao.",
			SortOrder:   110,
			Permissions: []string{
				"finance.sheets.view",
				"finance.config.view",
			},
		},
	}
}

func (m *Module) Build(deps modules.Dependencies) (modules.Handle, error) {
	sheets := NewPostgresSheetStore(deps.Pool)
	config := NewPostgresConfigStore(deps.Pool)
	svc := NewService(sheets, config)

	fiscal := NewFiscalService(NewPostgresFiscalStore(deps.Pool), m.box)
	documents := newDocumentService(&DocumentStore{pool: deps.Pool}, fiscal)
	documents.originals = m.originals
	retentionCtx, stopRetention := context.WithCancel(context.Background())
	if m.originals != nil {
		go documents.runRetention(retentionCtx, deps.Logger)
	}
	m.handle = &handle{
		documents:      documents,
		accessChat:     &AccessConversationService{pool: deps.Pool, fiscal: fiscal, documents: documents},
		assistant:      &AssistantService{pool: deps.Pool, finance: svc, fiscal: fiscal, resolver: m.assistantResolver, documents: documents},
		service:        svc,
		fiscal:         fiscal,
		authMiddleware: deps.AuthMiddleware,
		stopRetention:  stopRetention,
	}
	m.handle.assistant.access = m.handle.accessChat
	if m.plusPorts.LLM == nil {
		m.plusPorts.LLM = llm.New(deps.Logger)
	}
	m.handle.plus = NewPlusService(m.handle.assistant, m.plusPorts, deps.Logger)
	m.handle.accessChat.orchestrator = &ConversationOrchestrator{plus: m.handle.plus}
	return m.handle, nil
}

// ============================================================================
// Handle
// ============================================================================

type handle struct {
	plus           *PlusService
	accessChat     *AccessConversationService
	documents      *DocumentService
	assistant      *AssistantService
	service        *Service
	fiscal         *FiscalService
	authMiddleware *auth.Middleware
	stopRetention  context.CancelFunc
}

func (h *handle) ID() string { return "finance" }

// RegisterRoutes monta /v1/finance/* (RequireAuth; gating de modulo no Chain).
func (h *handle) RegisterRoutes(mux *http.ServeMux) {
	RegisterRoutes(mux, h.service, h.authMiddleware)
	RegisterFiscalRoutes(mux, h.fiscal, h.authMiddleware)
	RegisterPortalSessionRoutes(mux, h.fiscal, h.authMiddleware)
	RegisterDocumentPreparationRoutes(mux, h.documents, h.authMiddleware)
	RegisterAccessConversationRoutes(mux, h.accessChat, h.authMiddleware)
	RegisterAssistantRoutes(mux, h.assistant, h.authMiddleware)
	RegisterPlusStatusRoute(mux, h.plus, h.authMiddleware)
	RegisterDocumentRoutes(mux, h.documents, h.authMiddleware)
	RegisterDocumentReviewRoutes(mux, h.documents, h.authMiddleware)
	RegisterFiscalCustomerRoutes(mux, h.documents, h.authMiddleware)
	RegisterReceiptLinkRoutes(mux, h.documents, h.authMiddleware)
	RegisterRetentionRoutes(mux, h.documents, h.authMiddleware)
}

// RegisterEventHandlers — finance nao consome eventos por enquanto.
func (h *handle) RegisterEventHandlers(_ events.Bus) {}

func (h *handle) Close() error {
	if h.stopRetention != nil {
		h.stopRetention()
	}
	return nil
}
