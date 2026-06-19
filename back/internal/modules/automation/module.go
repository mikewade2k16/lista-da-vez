package automation

import (
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/events"
	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/modules"
)

// defaultWAHAURL e a base interna da WAHA na rede do compose. Sobrescrita por
// AUTOMATION_WAHA_INTERNAL_URL.
const defaultWAHAURL = "http://waha:3000"

// defaultWAHASession e a sessao fisica usada nas chamadas a WAHA. A WAHA Core so
// aceita a sessao "default" (1 numero por instancia), entao todas as contas
// compartilham essa sessao unica. Sobrescrita por AUTOMATION_WAHA_SESSION; o valor
// especial "@channel" volta ao modo por-conta (1 sessao por automation), valido
// apenas com WAHA Plus (multi-sessao).
const defaultWAHASession = "default"

// defaultN8NURL e a base interna do n8n na rede do compose (webhook do Omni
// Chat). Sobrescrita por AUTOMATION_N8N_INTERNAL_URL.
const defaultN8NURL = "http://n8n:5678"

// omniContextTokenTTL e a janela de validade do context token do Omni Chat
// (Fase 2). Curta: cobre o salto /ask -> webhook n8n -> tool de dados, limitando
// a janela de uso de um token vazado.
const omniContextTokenTTL = 300 * time.Second

// Module e o adaptador do modulo `automation` para o Module Registry.
//
// Painel de automacao WhatsApp/IA (n8n + WAHA) dentro do Omni. M1: Status +
// Conectar (QR via proxy WAHA) + liga/desliga. Fases seguintes (runtime-config,
// personas/RAG, BYOK) em docs/automation/PLATAFORMA_AUTOMACAO.md.
type Module struct {
	handle *handle
}

// New cria um Module pronto para registrar no Registry.
func New() *Module {
	return &Module{}
}

func (m *Module) ID() string { return "automation" }

func (m *Module) Metadata() modules.Metadata {
	return modules.Metadata{
		SchemaName:  "automation",
		Label:       "Automacao",
		Description: "Assistente de WhatsApp/IA (n8n + WAHA) por account. Conectar numero, ligar/desligar e (em breve) personas, modelos e RAG.",
		IsCore:      false,
		SortOrder:   50,
	}
}

func (m *Module) Permissions() []modules.PermissionDef {
	return []modules.PermissionDef{
		{Key: "automation.view", Label: "Ver automacao", Scope: "account"},
		{Key: "automation.manage", Label: "Gerenciar automacao", Scope: "account"},
		{Key: "automation.whatsapp.manage", Label: "Conectar/desconectar WhatsApp", Scope: "account"},
	}
}

func (m *Module) RoleTemplates() []modules.RoleTemplateDef {
	return []modules.RoleTemplateDef{
		{
			ID:          "automation.manager",
			Label:       "Gestor da Automacao",
			Description: "Conecta o WhatsApp, liga/desliga e configura a automacao.",
			SortOrder:   100,
			Permissions: []string{"automation.view", "automation.manage", "automation.whatsapp.manage"},
		},
	}
}

func (m *Module) Build(deps modules.Dependencies) (modules.Handle, error) {
	wahaURL := strings.TrimSpace(os.Getenv("AUTOMATION_WAHA_INTERNAL_URL"))
	if wahaURL == "" {
		wahaURL = defaultWAHAURL
	}
	n8nURL := strings.TrimSpace(os.Getenv("AUTOMATION_N8N_INTERNAL_URL"))
	if n8nURL == "" {
		n8nURL = defaultN8NURL
	}
	// Sessao fisica da WAHA. Default "default" (WAHA Core, 1 numero compartilhado);
	// "@channel" volta ao modo por-conta (WAHA Plus, 1 sessao por automation).
	wahaSession := strings.TrimSpace(os.Getenv("AUTOMATION_WAHA_SESSION"))
	if wahaSession == "" {
		wahaSession = defaultWAHASession
	}
	if wahaSession == "@channel" {
		wahaSession = ""
	}
	// O Omni Chat reusa AUTOMATION_RUNTIME_TOKEN (mesmo token de servico do
	// runtime-config) para o Bearer Go->n8n. Sem token = nao configurado (503).
	runtimeToken := strings.TrimSpace(os.Getenv("AUTOMATION_RUNTIME_TOKEN"))

	// Context token (Fase 2): secret HMAC dedicado quando
	// AUTOMATION_CONTEXT_TOKEN_SECRET esta setado; senao reusa o runtime token
	// como secret (MVP — token e secret HMAC sao valores distintos: o runtime
	// token assina o context token, nao e' o context token). Sem nenhum dos dois,
	// o manager fica sem secret e Issue/Parse falham (chat sem tools de dados).
	ctxSecret := strings.TrimSpace(os.Getenv("AUTOMATION_CONTEXT_TOKEN_SECRET"))
	if ctxSecret == "" {
		ctxSecret = runtimeToken
	}
	ctxMgr := NewContextTokenManager([]byte(ctxSecret), omniContextTokenTTL)

	svc := NewService(NewStore(deps.Pool), NewWAHAClient(wahaURL), NewN8NClient(n8nURL, runtimeToken), ctxMgr, wahaSession)

	m.handle = &handle{
		service:        svc,
		authMiddleware: deps.AuthMiddleware,
		runtimeToken:   runtimeToken,
		ctxMgr:         ctxMgr,
	}
	return m.handle, nil
}

// ============================================================================
// Handle
// ============================================================================

type handle struct {
	service        *Service
	authMiddleware *auth.Middleware
	runtimeToken   string
	ctxMgr         *ContextTokenManager
}

func (h *handle) ID() string { return "automation" }

// RegisterRoutes monta as rotas do painel (/v1/automation*, gateadas por modulo no
// Chain) e a rota de runtime consumida pelo n8n (/v1/runtime/automation/config, auth
// por token de servico, fora do gating).
func (h *handle) RegisterRoutes(mux *http.ServeMux) {
	RegisterRoutes(mux, h.service, h.authMiddleware)
	RegisterRuntimeRoutes(mux, h.service, h.runtimeToken, h.ctxMgr)
}

// RegisterEventHandlers — automation nao consome eventos por enquanto.
func (h *handle) RegisterEventHandlers(_ events.Bus) {}

func (h *handle) Close() error { return nil }
