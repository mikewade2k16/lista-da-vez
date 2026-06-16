package bio

import (
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/events"
	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/modules"
)

// envBioMaxBytes le uma env de tamanho em MB e devolve bytes; 0 (= default no
// MediaStorage) se ausente ou invalida.
func envBioMaxBytes(key string) int64 {
	mb, err := strconv.Atoi(strings.TrimSpace(os.Getenv(key)))
	if err != nil || mb <= 0 {
		return 0
	}
	return int64(mb) * 1024 * 1024
}

// Module e o adaptador do modulo `bio` (link-in-bio) para o Module Registry.
//
// O painel Omni e o CRUD das paginas de bio; o front Nuxt separado consome
// GET /v1/public/bio/{slug} server-to-server. Conteudo em jsonb (data_draft +
// data_published) com merge sobre bio.defaults global na hora de servir.
type Module struct {
	handle *handle
}

// New cria um Module pronto para registrar no Registry.
func New() *Module {
	return &Module{}
}

func (m *Module) ID() string { return "bio" }

func (m *Module) Metadata() modules.Metadata {
	return modules.Metadata{
		SchemaName:  "bio",
		Label:       "Bio",
		Description: "Paginas de link-in-bio por account. Painel faz o CRUD; o front bio consome o endpoint publico.",
		IsCore:      false,
		SortOrder:   45,
	}
}

func (m *Module) Permissions() []modules.PermissionDef {
	return []modules.PermissionDef{
		{Key: "bio.view", Label: "Ver bios", Scope: "account"},
		{Key: "bio.manage", Label: "Gerenciar bios", Scope: "account"},
		{Key: "bio.publish", Label: "Publicar/despublicar bios", Scope: "account"},
	}
}

func (m *Module) RoleTemplates() []modules.RoleTemplateDef {
	return []modules.RoleTemplateDef{
		{
			ID:          "bio.manager",
			Label:       "Gestor de Bio",
			Description: "Cria, edita, publica e despublica as bios da account.",
			SortOrder:   100,
			Permissions: []string{"bio.view", "bio.manage", "bio.publish"},
		},
		{
			ID:          "bio.editor",
			Label:       "Editor de Bio",
			Description: "Cria e edita bios, sem publicar.",
			SortOrder:   110,
			Permissions: []string{"bio.view", "bio.manage"},
		},
		{
			ID:          "bio.viewer",
			Label:       "Leitor de Bio",
			Description: "Apenas leitura das bios da account.",
			SortOrder:   120,
			Permissions: []string{"bio.view"},
		},
	}
}

func (m *Module) Build(deps modules.Dependencies) (modules.Handle, error) {
	publicBase := strings.TrimSpace(os.Getenv("PUBLIC_API_BASE_URL"))
	uploadsDir := strings.TrimSpace(os.Getenv("UPLOADS_DIR"))

	svc := NewService(NewStore(deps.Pool), publicBase)
	// B7: fonte de produtos plugavel. site_products le o schema site.products
	// (cross-schema, mesmo pool, apenas SELECT). Novas fontes (ERP/API do
	// cliente) entram aqui sem mexer no resto.
	svc.RegisterSource(SourceTypeSiteProducts, NewSiteProductsSource(deps.Pool))

	m.handle = &handle{
		service:        svc,
		storage:        NewMediaStorage(uploadsDir, envBioMaxBytes("BIO_MAX_VIDEO_MB"), envBioMaxBytes("BIO_MAX_IMAGE_MB")),
		authMiddleware: deps.AuthMiddleware,
		publicToken:    strings.TrimSpace(os.Getenv("BIO_PUBLIC_TOKEN")),
		broker:         newSSEBroker(),
	}
	return m.handle, nil
}

// ============================================================================
// Handle
// ============================================================================

type handle struct {
	service        *Service
	storage        *MediaStorage
	authMiddleware *auth.Middleware
	publicToken    string
	broker         *sseBroker
}

func (h *handle) ID() string { return "bio" }

// RegisterRoutes monta as rotas do painel (/v1/bio*, gateadas por modulo no
// Chain via RequireModuleByPath) E a rota publica (/v1/public/bio/{slug}, sem
// JWT e fora do gating).
func (h *handle) RegisterRoutes(mux *http.ServeMux) {
	RegisterRoutes(mux, h.service, h.storage, h.authMiddleware, h.broker)
	RegisterSourceRoutes(mux, h.service, h.authMiddleware)
	RegisterPublicRoutes(mux, h.service, h.publicToken, h.broker)
}

// RegisterEventHandlers — bio nao consome eventos por enquanto.
func (h *handle) RegisterEventHandlers(_ events.Bus) {}

func (h *handle) Close() error { return nil }
