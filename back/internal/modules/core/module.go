package core

import (
	"net/http"

	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/events"
	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/modules"
)

// Module e o adaptador do core para o Module Registry da plataforma.
//
// Construido em app.go quando CORE_V2_ENABLED=true. Sincroniza permissoes e
// templates de cargo no boot via Registry.SyncCatalog. Quando montado, expoe
// /v2/me/accounts e /v2/me/context via RegisterRoutes.
type Module struct {
	handle *handle
}

// New cria um Module pronto para registrar no Registry.
func New() *Module {
	return &Module{}
}

// ID identifica o modulo no Registry e em core.modules.
func (m *Module) ID() string {
	return "core"
}

// Metadata descreve o modulo no catalogo.
func (m *Module) Metadata() modules.Metadata {
	return modules.Metadata{
		SchemaName:  "core",
		Label:       "Plataforma core",
		Description: "Identidade global, accounts, organizations, RBAC e Module Registry. Modulo obrigatorio.",
		IsCore:      true,
		SortOrder:   0,
	}
}

// Permissions declara o catalogo de permissoes do core.
//
// Estas chaves sao validadas em Registry.SyncCatalog antes de qualquer
// role_template referenciar. Adicionar/renomear aqui exige planejamento:
// removidas viram deprecated_at em core.permissions (nunca DELETE auto).
func (m *Module) Permissions() []modules.PermissionDef {
	return []modules.PermissionDef{
		{
			Key:         "core.account.view",
			Label:       "Visualizar dados da account",
			Description: "Acesso de leitura ao perfil da account, modulos habilitados e usuarios.",
			Scope:       "account",
		},
		{
			Key:         "core.account.manage",
			Label:       "Gerenciar dados da account",
			Description: "Editar nome, slug e metadados da account.",
			Scope:       "account",
		},
		{
			Key:         "core.users.view",
			Label:       "Visualizar usuarios da account",
			Description: "Listar membros da account e suas roles.",
			Scope:       "account",
		},
		{
			Key:         "core.users.manage",
			Label:       "Gerenciar usuarios da account",
			Description: "Convidar, remover e atualizar membership de usuarios.",
			Scope:       "account",
		},
		{
			Key:         "core.roles.view",
			Label:       "Visualizar cargos",
			Description: "Listar cargos clonados e templates disponiveis.",
			Scope:       "account",
		},
		{
			Key:         "core.roles.manage",
			Label:       "Gerenciar cargos",
			Description: "Clonar templates, editar permissoes de cargos da account, atribuir cargos a usuarios.",
			Scope:       "account",
		},
		{
			Key:         "core.modules.manage",
			Label:       "Habilitar e desabilitar modulos",
			Description: "Controlar quais modulos estao ativos para a account (impacta menu e rotas).",
			Scope:       "account",
		},
		{
			Key:         "core.organization.consolidated_read",
			Label:       "Ver dados consolidados da organization",
			Description: "Modo agencia: visualizar dados agregados de todas as accounts da organization.",
			Scope:       "platform",
		},
	}
}

// RoleTemplates declara os cargos-template que cada Account recebe quando
// criada (clonados em core.roles).
//
// Templates nao podem ser editados depois de criados (regra do SyncCatalog).
// Para evoluir um template, criar uma versao nova.
func (m *Module) RoleTemplates() []modules.RoleTemplateDef {
	return []modules.RoleTemplateDef{
		{
			ID:          "core.owner",
			Label:       "Proprietario",
			Description: "Dono da account. Acesso total a configuracao, usuarios, cargos e modulos.",
			IsSystem:    true,
			IsLocked:    true,
			SortOrder:   0,
			Permissions: []string{
				"core.account.view",
				"core.account.manage",
				"core.users.view",
				"core.users.manage",
				"core.roles.view",
				"core.roles.manage",
				"core.modules.manage",
			},
		},
		{
			ID:          "core.admin",
			Label:       "Administrador",
			Description: "Gerencia usuarios, cargos e configuracao da account. Nao toca em modulos.",
			IsSystem:    true,
			SortOrder:   10,
			Permissions: []string{
				"core.account.view",
				"core.users.view",
				"core.users.manage",
				"core.roles.view",
				"core.roles.manage",
			},
		},
		{
			ID:          "core.member",
			Label:       "Membro",
			Description: "Acesso basico a account. Permissoes adicionais vem dos modulos satelites.",
			IsSystem:    true,
			SortOrder:   100,
			Permissions: []string{
				"core.account.view",
				"core.users.view",
				"core.roles.view",
			},
		},
	}
}

// Build conecta o Service e o RBACService do core ao Handle do Registry.
func (m *Module) Build(deps modules.Dependencies) (modules.Handle, error) {
	repo := NewPostgresRepository(deps.Pool)
	rbacRepo := NewPostgresRBACRepository(deps.Pool)
	rbacSvc := NewRBACService(rbacRepo)

	svc := NewService(repo)
	svc.WithRBAC(rbacSvc)

	m.handle = &handle{
		service:        svc,
		rbacService:    rbacSvc,
		authMiddleware: deps.AuthMiddleware,
	}
	return m.handle, nil
}

// ============================================================================
// Handle interno
// ============================================================================

type handle struct {
	service        *Service
	rbacService    *RBACService
	authMiddleware *auth.Middleware
}

func (h *handle) ID() string { return "core" }

// RegisterRoutes monta /v2/me/accounts, /v2/me/context e os endpoints RBAC.
func (h *handle) RegisterRoutes(mux *http.ServeMux) {
	RegisterRoutes(mux, h.service, h.authMiddleware)
	RegisterRBACRoutes(mux, h.rbacService, h.authMiddleware)
}

// RegisterEventHandlers — core nao consome eventos por enquanto (publica
// account.modules.changed quando UI de habilitacao aterrissar, na Fase 3).
func (h *handle) RegisterEventHandlers(_ events.Bus) {}

func (h *handle) Close() error { return nil }
