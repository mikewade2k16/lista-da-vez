package cardapio

import (
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/events"
	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/modules"
)

// Module e o adaptador do modulo `cardapio` para o Module Registry.
//
// Cardapios online (restaurantes) gerenciados no painel Omni e servidos por um
// front Nuxt estatico no host do cliente, que consome a API publica
// (/v1/public/*) direto do browser do visitante. O painel (/v1/cardapio*) e o
// CRUD multitenant; o publico recalcula precos e isola por host. Plano:
// docs/cardapio/PLANO_MODULO_CARDAPIO.md.
type Module struct {
	handle *handle
}

// New cria um Module pronto para registrar no Registry.
func New() *Module {
	return &Module{}
}

func (m *Module) ID() string { return "cardapio" }

func (m *Module) Metadata() modules.Metadata {
	return modules.Metadata{
		SchemaName:  "cardapio",
		Label:       "Cardapio Online",
		Description: "Cardapios online (restaurantes) por account. CRUD de restaurante, categorias, produtos, avaliacoes, dominios e pedidos; API publica por host para o front estatico.",
		IsCore:      false,
		SortOrder:   60,
	}
}

func (m *Module) Permissions() []modules.PermissionDef {
	return []modules.PermissionDef{
		{Key: "cardapio.view", Label: "Ver cardapios", Scope: "account"},
		{Key: "cardapio.manage", Label: "Gerenciar cardapios", Scope: "account"},
		{Key: "cardapio.orders.manage", Label: "Gerenciar pedidos do cardapio", Scope: "account"},
	}
}

func (m *Module) RoleTemplates() []modules.RoleTemplateDef {
	return []modules.RoleTemplateDef{
		{
			ID:          "cardapio.manager",
			Label:       "Gestor do Cardapio",
			Description: "Gerencia restaurantes, catalogo, dominios e pedidos do cardapio online.",
			SortOrder:   100,
			Permissions: []string{"cardapio.view", "cardapio.manage", "cardapio.orders.manage"},
		},
		{
			ID:          "cardapio.viewer",
			Label:       "Leitor do Cardapio",
			Description: "Acompanha restaurantes e pedidos sem editar.",
			SortOrder:   110,
			Permissions: []string{"cardapio.view"},
		},
	}
}

func (m *Module) Build(deps modules.Dependencies) (modules.Handle, error) {
	store := NewStore(deps.Pool)
	media := NewDiskMediaStorage(strings.TrimSpace(os.Getenv("UPLOADS_DIR")))
	telemetrySalt := strings.TrimSpace(os.Getenv("CARDAPIO_TELEMETRY_SALT"))
	// Sem salt, o ip_hash da telemetria fica vazio (ipHashHex retorna "") — sem
	// fail-open silencioso de IP cru, mas em prod o salt e obrigatorio (LGPD). Em dev
	// apenas avisamos no boot, sem derrubar o modulo.
	if telemetrySalt == "" && deps.Logger != nil {
		deps.Logger.Warn("CARDAPIO_TELEMETRY_SALT vazio: ip_hash da telemetria sera vazio (obrigatorio em producao)")
	}
	svc := NewService(store, deps.Pool, ServiceConfig{
		BaseDomain:     strings.TrimSpace(os.Getenv("CARDAPIO_BASE_DOMAIN")),
		DevDefaultSlug: strings.TrimSpace(os.Getenv("CARDAPIO_DEV_DEFAULT_SLUG")),
		PublicBaseURL:  strings.TrimSpace(os.Getenv("PUBLIC_API_BASE_URL")),
		TelemetrySalt:  telemetrySalt,
		Media:          media,
	})

	// Poda diaria da telemetria (LGPD). CARDAPIO_TELEMETRY_RETENTION_DAYS sobrescreve o
	// default de 90 dias; <= 0 desliga a poda automatica. A goroutine para no Close.
	retentionDays := defaultRetentionDays
	if v := strings.TrimSpace(os.Getenv("CARDAPIO_TELEMETRY_RETENTION_DAYS")); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			retentionDays = n
		}
	}
	stopPrune := make(chan struct{})
	svc.startRetentionLoop(stopPrune, retentionDays)

	m.handle = &handle{
		service:        svc,
		authMiddleware: deps.AuthMiddleware,
		limiter:        newRateLimiter(),
		stopPrune:      stopPrune,
	}
	return m.handle, nil
}

// ============================================================================
// Handle
// ============================================================================

type handle struct {
	service        *Service
	authMiddleware *auth.Middleware
	limiter        *rateLimiter
	stopPrune      chan struct{}
}

func (h *handle) ID() string { return "cardapio" }

// RegisterRoutes monta as rotas do painel (/v1/cardapio*, gateadas por modulo no
// Chain) e as rotas publicas (/v1/public/*, sem JWT e sem gating; CORS wildcard
// aplicado no middleware da plataforma).
func (h *handle) RegisterRoutes(mux *http.ServeMux) {
	RegisterRoutes(mux, h.service, h.authMiddleware)
	RegisterCatalogRoutes(mux, h.service, h.authMiddleware)
	RegisterOrderRoutes(mux, h.service, h.authMiddleware)
	RegisterLayoutRoutes(mux, h.service, h.authMiddleware)
	RegisterAnalyticsRoutes(mux, h.service, h.authMiddleware)
	RegisterPublicRoutes(mux, h.service, h.limiter)
}

// RegisterEventHandlers — cardapio nao consome eventos por enquanto.
func (h *handle) RegisterEventHandlers(_ events.Bus) {}

func (h *handle) Close() error {
	if h.stopPrune != nil {
		close(h.stopPrune)
	}
	return nil
}
