package metaads

import (
	"net/http"
	"os"
	"strings"

	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/events"
	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/modules"
)

// defaultGraphBase e a base da Graph/Marketing API. Sobrescrita por
// META_ADS_GRAPH_BASE (ex.: bump de versao da API).
const defaultGraphBase = "https://graph.facebook.com/v24.0"

// Module e o adaptador do modulo `meta_ads` para o Module Registry.
//
// Painel de Meta/Facebook Ads dentro do Omni: OAuth/token cifrado, sync/cache,
// relatorios, contexto do Assistente 360 e executor first-party limitado. O estado
// vigente fica em docs/meta-ads/ASSISTENTE_360_STATUS_E_ROADMAP.md.
type Module struct {
	handle                         *handle
	assistantActionSourceValidator AssistantActionSourceValidator
}

// New cria um Module pronto para registrar no Registry.
func New() *Module {
	return &Module{}
}

func (m *Module) WithAssistantActionSourceValidator(validator AssistantActionSourceValidator) *Module {
	m.assistantActionSourceValidator = validator
	return m
}

func (m *Module) ID() string { return "meta_ads" }

func (m *Module) Metadata() modules.Metadata {
	return modules.Metadata{
		SchemaName:  "meta_ads",
		Label:       "Meta Ads",
		Description: "Relatorios e gestao de campanhas de Meta/Facebook Ads por account. Conectar conta, sincronizar dados e (em breve) criar/editar campanhas e IA.",
		IsCore:      false,
		SortOrder:   60,
	}
}

func (m *Module) Permissions() []modules.PermissionDef {
	return []modules.PermissionDef{
		{Key: "meta_ads.view", Label: "Ver Meta Ads", Description: "Ver dashboards e relatorios de campanhas.", Scope: "account"},
		{Key: "meta_ads.manage", Label: "Gerenciar campanhas", Description: "Criar, editar e pausar campanhas.", Scope: "account"},
		{Key: "meta_ads.connect", Label: "Conectar Meta Ads", Description: "Conectar/desconectar a conta Meta (token).", Scope: "account"},
	}
}

func (m *Module) RoleTemplates() []modules.RoleTemplateDef {
	return []modules.RoleTemplateDef{
		{
			ID:          "meta_ads.manager",
			Label:       "Gestor de Trafego",
			Description: "Conecta a conta Meta, sincroniza dados e gerencia campanhas.",
			SortOrder:   100,
			Permissions: []string{"meta_ads.view", "meta_ads.manage", "meta_ads.connect"},
		},
	}
}

func (m *Module) Build(deps modules.Dependencies) (modules.Handle, error) {
	graphBase := strings.TrimSpace(os.Getenv("META_ADS_GRAPH_BASE"))
	if graphBase == "" {
		graphBase = defaultGraphBase
	}
	cryptoKey := strings.TrimSpace(os.Getenv("META_ADS_CRYPTO_KEY"))

	// Runner legado/read-only de compatibilidade interna. Nao e o chat nem o
	// executor canonico; valores vazios desabilitam somente esse cliente legado.
	runnerURL := strings.TrimSpace(os.Getenv("META_ADS_ASSISTANT_RUNNER_URL"))
	runnerToken := strings.TrimSpace(os.Getenv("META_ADS_ASSISTANT_TOKEN"))

	// Bearer de servico do bridge legado (/internal/meta-ads/runner/*) que o runner
	// read-only consome para buscar postagens do Instagram. Sem default no Go:
	// vazio = bridge nao configurado (503 bridge_not_configured).
	bridgeToken := strings.TrimSpace(os.Getenv("META_ADS_RUNNER_BRIDGE_TOKEN"))

	store := NewStore(deps.Pool, cryptoKey)
	metaClient := NewMetaClient(graphBase)
	svc := NewService(
		store,
		metaClient,
		NewRunnerClient(runnerURL, runnerToken),
	)
	svc.SetBridgeToken(bridgeToken)
	svc.SetAssistantActionSourceValidator(m.assistantActionSourceValidator)
	oauthService := NewOAuthService(
		store,
		svc,
		NewMetaOAuthClient(graphBase),
		OAuthConfig{
			AppID:       os.Getenv("META_ADS_APP_ID"),
			AppSecret:   os.Getenv("META_ADS_APP_SECRET"),
			RedirectURI: os.Getenv("META_ADS_OAUTH_REDIRECT_URL"),
		},
	)

	m.handle = &handle{
		service:        svc,
		oauthService:   oauthService,
		authMiddleware: deps.AuthMiddleware,
		bridgeToken:    bridgeToken,
	}
	return m.handle, nil
}

// Service devolve a instancia construida para injecao lazy no assistente 360.
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
	oauthService   *OAuthService
	authMiddleware *auth.Middleware
	bridgeToken    string
}

func (h *handle) ID() string { return "meta_ads" }

// RegisterRoutes monta as rotas do painel (/v1/meta-ads*, JWT) e o bridge legado
// do runner (/internal/meta-ads/*, bearer de servico, SEM JWT). O gating
// por modulo (account_modules) e aplicado no Chain via moduleGatingRules
// (/v1/meta-ads -> meta_ads) e cobre so o painel; o bridge fica fora desse
// prefixo de proposito (rede interna + bearer).
func (h *handle) RegisterRoutes(mux *http.ServeMux) {
	RegisterRoutes(mux, h.service, h.authMiddleware)
	RegisterOAuthRoutes(mux, h.oauthService, h.authMiddleware)
	registerBridgeRoutes(mux, h.service, h.bridgeToken)
}

// RegisterEventHandlers — meta_ads nao consome eventos no MVP.
func (h *handle) RegisterEventHandlers(_ events.Bus) {}

func (h *handle) Close() error { return nil }
