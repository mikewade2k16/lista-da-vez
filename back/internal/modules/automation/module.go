package automation

import (
	"net/http"
	"os"
	"strings"

	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/events"
	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/modules"
)

// defaultWAHAURL e a base interna da WAHA na rede do compose. Sobrescrita por
// AUTOMATION_WAHA_INTERNAL_URL.
const defaultWAHAURL = "http://waha:3000"

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
	svc := NewService(NewStore(deps.Pool), NewWAHAClient(wahaURL))

	m.handle = &handle{
		service:        svc,
		authMiddleware: deps.AuthMiddleware,
		runtimeToken:   strings.TrimSpace(os.Getenv("AUTOMATION_RUNTIME_TOKEN")),
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
}

func (h *handle) ID() string { return "automation" }

// RegisterRoutes monta as rotas do painel (/v1/automation*, gateadas por modulo no
// Chain) e a rota de runtime consumida pelo n8n (/v1/runtime/automation/config, auth
// por token de servico, fora do gating).
func (h *handle) RegisterRoutes(mux *http.ServeMux) {
	RegisterRoutes(mux, h.service, h.authMiddleware)
	RegisterRuntimeRoutes(mux, h.service, h.runtimeToken)
}

// RegisterEventHandlers — automation nao consome eventos por enquanto.
func (h *handle) RegisterEventHandlers(_ events.Bus) {}

func (h *handle) Close() error { return nil }
