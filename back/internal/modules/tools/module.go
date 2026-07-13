package tools

import (
	"net/http"
	"os"
	"strings"

	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/events"
	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/modules"
)

// Module e o adaptador do modulo `tools` para o Module Registry.
//
// Duas ferramentas: encurtador de link (short_links) e gerador de QR Code
// (qr_codes). Reconstroi como back Go real o que no projeto antigo era mock
// (globalThis no BFF Nitro eliminado). Painel em /v1/tools (gating no Chain);
// redirects publicos /s/{slug} e /q/{slug} fora do gate.
// Plano: docs/tools/PLANO_MODULO_TOOLS.md.
type Module struct {
	handle *handle
}

// New cria um Module pronto para registrar no Registry.
func New() *Module {
	return &Module{}
}

func (m *Module) ID() string { return "tools" }

func (m *Module) Metadata() modules.Metadata {
	return modules.Metadata{
		SchemaName:  "tools",
		Label:       "Tools",
		Description: "Encurtador de link e gerador de QR Code customizavel, por conta, com rastreamento de cliques/scans.",
		IsCore:      false,
		SortOrder:   55,
	}
}

func (m *Module) Permissions() []modules.PermissionDef {
	return []modules.PermissionDef{
		{Key: "tools.shortlinks.view", Label: "Ver links curtos", Scope: "account"},
		{Key: "tools.shortlinks.manage", Label: "Gerenciar links curtos", Scope: "account"},
		{Key: "tools.qr.view", Label: "Ver QR Codes", Scope: "account"},
		{Key: "tools.qr.manage", Label: "Gerenciar QR Codes", Scope: "account"},
	}
}

func (m *Module) RoleTemplates() []modules.RoleTemplateDef {
	return []modules.RoleTemplateDef{
		{
			ID:          "tools.manager",
			Label:       "Gestor de Tools",
			Description: "CRUD completo de links curtos e QR Codes.",
			SortOrder:   100,
			Permissions: []string{
				"tools.shortlinks.view",
				"tools.shortlinks.manage",
				"tools.qr.view",
				"tools.qr.manage",
			},
		},
		{
			ID:          "tools.viewer",
			Label:       "Leitor de Tools",
			Description: "Apenas leitura de links curtos e QR Codes.",
			SortOrder:   110,
			Permissions: []string{
				"tools.shortlinks.view",
				"tools.qr.view",
			},
		},
	}
}

// toolsPublicBase resolve a base absoluta dos redirects /s e /q. Override
// dedicado TOOLS_PUBLIC_BASE_URL; senao a base publica da api (PUBLIC_API_BASE_URL,
// ja usada por bio/cardapio); vazio = URL relativa (o front prefixa com apiBase).
func toolsPublicBase() string {
	if v := strings.TrimSpace(os.Getenv("TOOLS_PUBLIC_BASE_URL")); v != "" {
		return v
	}
	return strings.TrimSpace(os.Getenv("PUBLIC_API_BASE_URL"))
}

func (m *Module) Build(deps modules.Dependencies) (modules.Handle, error) {
	svc := NewService(
		NewPostgresShortLinkStore(deps.Pool),
		NewPostgresQrCodeStore(deps.Pool),
		toolsPublicBase(),
	)
	m.handle = &handle{
		service:        svc,
		authMiddleware: deps.AuthMiddleware,
		accountChecker: auth.NewPostgresAccountMemberChecker(deps.Pool),
	}
	return m.handle, nil
}

// ============================================================================
// Handle
// ============================================================================

type handle struct {
	service        *Service
	authMiddleware *auth.Middleware
	accountChecker auth.AccountMemberChecker
}

func (h *handle) ID() string { return "tools" }

// RegisterRoutes monta o painel (/v1/tools/*, gateado por modulo no Chain) e os
// redirects publicos (/s/{slug}, /q/{slug}, sem JWT e fora do gating).
func (h *handle) RegisterRoutes(mux *http.ServeMux) {
	RegisterRoutes(mux, h.service, h.authMiddleware, h.accountChecker)
	RegisterPublicRoutes(mux, h.service)
}

// RegisterEventHandlers — tools nao consome eventos.
func (h *handle) RegisterEventHandlers(_ events.Bus) {}

func (h *handle) Close() error { return nil }
