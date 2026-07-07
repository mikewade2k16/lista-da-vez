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
		// Permissoes de VISIBILIDADE/EDICAO de PAGINA (workspace.*), migradas do
		// modulo legado `access` para o core (fase aditiva 2026-06-26). Gateiam quais
		// paginas do menu cada usuario ve. Declaradas no `core` (modulo sempre
		// habilitado) para o override valer em qualquer account. Backfill de
		// role_permissions/overrides na migration 0175. Ver docs/LEGADO.md item 5.
		{Key: "workspace.operacao.view", Label: "Ver pagina Operacao", Description: "Visibilidade da pagina Operacao no menu.", Scope: "account"},
		{Key: "workspace.operacao.edit", Label: "Editar na pagina Operacao", Description: "Executar comandos na pagina Operacao.", Scope: "account"},
		{Key: "workspace.consultor.view", Label: "Ver pagina Consultor", Description: "Visibilidade da pagina Consultor no menu.", Scope: "account"},
		{Key: "workspace.ranking.view", Label: "Ver pagina Ranking", Description: "Visibilidade da pagina Ranking no menu.", Scope: "account"},
		{Key: "workspace.dados.view", Label: "Ver pagina Dados", Description: "Visibilidade da pagina Dados no menu.", Scope: "account"},
		{Key: "workspace.inteligencia.view", Label: "Ver pagina Inteligencia", Description: "Visibilidade da pagina Inteligencia no menu.", Scope: "account"},
		{Key: "workspace.relatorios.view", Label: "Ver pagina Relatorios", Description: "Visibilidade da pagina Relatorios no menu.", Scope: "account"},
		{Key: "workspace.campanhas.view", Label: "Ver pagina Campanhas", Description: "Visibilidade da pagina Campanhas no menu.", Scope: "account"},
		{Key: "workspace.campanhas.edit", Label: "Editar na pagina Campanhas", Description: "Editar regras/campanhas na pagina Campanhas.", Scope: "account"},
		{Key: "workspace.clientes.view", Label: "Ver pagina Clientes", Description: "Visibilidade da pagina Clientes no menu.", Scope: "account"},
		{Key: "workspace.clientes.edit", Label: "Editar na pagina Clientes", Description: "Editar clientes e grupos na pagina Clientes.", Scope: "account"},
		{Key: "workspace.multiloja.view", Label: "Ver pagina Multi-loja", Description: "Visibilidade da pagina Multi-loja no menu.", Scope: "account"},
		{Key: "workspace.multiloja.edit", Label: "Editar na pagina Multi-loja", Description: "Editar lojas/config na pagina Multi-loja.", Scope: "account"},
		{Key: "workspace.usuarios.view", Label: "Ver pagina Usuarios", Description: "Visibilidade da pagina Usuarios no menu.", Scope: "account"},
		{Key: "workspace.usuarios.edit", Label: "Editar na pagina Usuarios", Description: "Editar usuarios/overrides na pagina Usuarios.", Scope: "account"},
		{Key: "workspace.manage.view", Label: "Ver paginas de Manage", Description: "Visibilidade das rotas agrupadas em Manage.", Scope: "account"},
		{Key: "workspace.configuracoes.view", Label: "Ver pagina Configuracoes", Description: "Visibilidade da pagina Configuracoes no menu.", Scope: "account"},
		{Key: "workspace.configuracoes.edit", Label: "Editar na pagina Configuracoes", Description: "Editar configuracoes operacionais.", Scope: "account"},
		{Key: "workspace.themes.view", Label: "Ver pagina Temas", Description: "Visibilidade da pagina Temas no menu.", Scope: "account"},
		{Key: "workspace.alertas.view", Label: "Ver pagina Alertas", Description: "Visibilidade da pagina Alertas no menu.", Scope: "account"},
		{Key: "workspace.alertas.edit", Label: "Editar na pagina Alertas", Description: "Gerenciar a pagina Alertas.", Scope: "account"},
		{Key: "workspace.feedback.view", Label: "Ver pagina Feedback", Description: "Visibilidade da pagina Feedback no menu.", Scope: "account"},
		{Key: "workspace.feedback.edit", Label: "Editar na pagina Feedback", Description: "Editar feedback e notas na pagina Feedback.", Scope: "account"},
		{Key: "workspace.tools.view", Label: "Ver pagina Tools", Description: "Visibilidade da pagina Tools no menu.", Scope: "account"},
		{Key: "workspace.erp.view", Label: "Ver pagina ERP", Description: "Visibilidade da pagina ERP no menu.", Scope: "account"},
		{Key: "workspace.erp.edit", Label: "Editar na pagina ERP", Description: "Sync manual e administracao na pagina ERP.", Scope: "account"},
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

// Build conecta o Service, RBACService, AdminService, AdminUserService e
// AdminOrganizationService do core ao Handle do Registry.
func (m *Module) Build(deps modules.Dependencies) (modules.Handle, error) {
	repo := NewPostgresRepository(deps.Pool)
	rbacRepo := NewPostgresRBACRepository(deps.Pool)
	rbacSvc := NewRBACService(rbacRepo)

	svc := NewService(repo)
	svc.WithRBAC(rbacSvc)

	adminRepo := NewPostgresAdminRepository(deps.Pool)
	adminSvc := NewAdminService(adminRepo, deps.Bus, deps.ModulesGuard)

	// AdminScopeResolver: decide por-request/por-account se o ator pode administrar
	// (delegacao multi-tenant). NAO confia em Principal.Role/Permissions (resolvidos
	// so no login a partir de UMA conta home); resolve tudo no banco.
	scopeResolver := NewAdminScopeResolver(NewPostgresAdminScopeRepository(deps.Pool))

	adminUserRepo := NewPostgresAdminUserRepository(adminRepo)
	adminUserLinksRepo := NewPostgresAdminUserLinksRepository(adminUserRepo)
	adminUserSvc := NewAdminUserService(adminUserRepo, deps.PasswordHasher, scopeResolver, adminUserLinksRepo)

	// AC-01: invalidacao sincrona do PrincipalCache nas mutacoes de papel (RBAC) e de
	// identidade (AdminUser). nil quando o cache esta desligado (AUTH_PRINCIPAL_CACHE_TTL=0s).
	if deps.PrincipalCache != nil {
		rbacSvc.SetPrincipalCacheInvalidator(deps.PrincipalCache)
		adminUserSvc.SetPrincipalCacheInvalidator(deps.PrincipalCache)
	}

	// Overrides allow/deny por usuario por account (core.user_permission_overrides),
	// validados via InvalidPermissionKeys do RBAC repo (reuso, sem catalogo novo).
	adminOverridesRepo := NewPostgresAdminOverridesRepository(deps.Pool)
	adminOverridesSvc := NewAdminOverridesService(adminOverridesRepo, scopeResolver, rbacRepo)

	adminOrgRepo := NewPostgresAdminOrganizationRepository(adminRepo)
	adminOrgSvc := NewAdminOrganizationService(adminOrgRepo)

	platformSettingsRepo := NewPostgresPlatformSettingsRepository(deps.Pool)
	platformSettingsSvc := NewPlatformSettingsService(platformSettingsRepo)

	// Aparência GLOBAL da plataforma (tema visual). Reusa o mesmo repositório de
	// platform_settings (chave 'appearance'); desacoplado do módulo queue.
	appearanceSvc := NewAppearanceService(platformSettingsRepo)

	// CRUD de role templates (catalogo GLOBAL de papeis-padrao) gerenciado por
	// platform_admin. core.role_templates + core.role_template_permissions.
	roleTemplateRepo := NewPostgresRoleTemplateAdminRepository(deps.Pool)
	roleTemplateSvc := NewRoleTemplateAdminService(roleTemplateRepo)

	m.handle = &handle{
		service:                  svc,
		rbacService:              rbacSvc,
		adminService:             adminSvc,
		adminUserService:         adminUserSvc,
		adminOverridesService:    adminOverridesSvc,
		adminOrganizationService: adminOrgSvc,
		platformSettingsService:  platformSettingsSvc,
		appearanceService:        appearanceSvc,
		roleTemplateService:      roleTemplateSvc,
		authMiddleware:           deps.AuthMiddleware,
	}
	return m.handle, nil
}

// ============================================================================
// Handle interno
// ============================================================================

type handle struct {
	service                  *Service
	rbacService              *RBACService
	adminService             *AdminService
	adminUserService         *AdminUserService
	adminOverridesService    *AdminOverridesService
	adminOrganizationService *AdminOrganizationService
	platformSettingsService  *PlatformSettingsService
	appearanceService        *AppearanceService
	roleTemplateService      *RoleTemplateAdminService
	authMiddleware           *auth.Middleware
}

func (h *handle) ID() string { return "core" }

// RegisterRoutes monta /v2/me/accounts, /v2/me/context, endpoints RBAC,
// admin de accounts, admin de users e admin de organizations.
func (h *handle) RegisterRoutes(mux *http.ServeMux) {
	RegisterRoutes(mux, h.service, h.authMiddleware)
	RegisterRBACRoutes(mux, h.rbacService, h.authMiddleware)
	RegisterAdminRoutes(mux, h.adminService, h.authMiddleware)
	RegisterAdminUsersRoutes(mux, h.adminUserService, h.authMiddleware)
	RegisterAdminOverridesRoutes(mux, h.adminOverridesService, h.adminUserService, h.authMiddleware)
	RegisterAdminOrganizationsRoutes(mux, h.adminOrganizationService, h.authMiddleware)
	RegisterPlatformSettingsRoutes(mux, h.platformSettingsService, h.authMiddleware)
	RegisterAppearanceRoutes(mux, h.appearanceService, h.authMiddleware)
	RegisterRoleTemplatesRoutes(mux, h.roleTemplateService, h.authMiddleware)
}

// RegisterEventHandlers — core nao consome eventos por enquanto (publica
// account.modules.changed quando UI de habilitacao aterrissar, na Fase 3).
func (h *handle) RegisterEventHandlers(_ events.Bus) {}

func (h *handle) Close() error { return nil }
