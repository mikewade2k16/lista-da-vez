// Package modules define o Module Registry da plataforma.
//
// Cada modulo plugavel da reestruturacao multi-tenant implementa a interface
// Module e e registrado no Registry no boot. O Registry:
//   - Sincroniza catalogos declarativos (modulos, permissoes, role templates)
//     no schema core via SyncCatalog.
//   - Constroi cada modulo (Build) com Dependencies resolvidas (incluindo
//     dependencias opcionais entre modulos).
//   - Devolve handles para registro de rotas HTTP e handlers de eventos.
//
// Plano: docs/SCHEMA_TARGET.md secao 3 e plano mestre Fase 2.
package modules

import (
	"log/slog"
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/events"
)

// Module e a unidade plugavel da plataforma.
//
// Modulos atuais (auth, tenants, stores, operations, etc.) continuam
// registrados pelo wiring legado em app.go. Modulos novos (Fase 2 em diante)
// passam pelo Registry — comecando pelo proprio "core".
type Module interface {
	// ID e o identificador estavel do modulo. Vira chave em core.modules e
	// referencia em core.account_modules. Ex: "core", "queue", "finance".
	ID() string

	// Metadata descreve schema, label, dependencias. Sincronizado em
	// core.modules via SyncCatalog.
	Metadata() Metadata

	// Permissions retorna o catalogo declarativo de permissoes do modulo.
	// SyncCatalog faz upsert em core.permissions; chaves removidas sao
	// marcadas com deprecated_at — nunca DELETE automatico.
	Permissions() []PermissionDef

	// RoleTemplates retorna os templates de cargo declarados pelo modulo.
	// SyncCatalog faz upsert em core.role_templates e popula
	// core.role_template_permissions APENAS para templates novos (nao
	// sobrescreve template ja existente).
	RoleTemplates() []RoleTemplateDef

	// Build constroi o modulo com dependencias resolvidas. Retorna o Handle
	// usado para registrar rotas e handlers de eventos.
	Build(deps Dependencies) (Handle, error)
}

// Handle e o resultado de Module.Build. Encapsula tudo que o modulo precisa
// expor para o resto da aplicacao depois de construido.
type Handle interface {
	// ID repete Module.ID — facilita logs sem acoplamento ao modulo original.
	ID() string

	// RegisterRoutes monta os endpoints HTTP do modulo no mux compartilhado.
	// Modulos satelites (Fase 6) devem aplicar o middleware accountModulesGuard
	// nas suas rotas. O modulo "core" e exceto: suas rotas de descoberta nao
	// podem ser bloqueadas pelo proprio guard que dependem.
	RegisterRoutes(mux *http.ServeMux)

	// RegisterEventHandlers conecta handlers do modulo no event bus.
	RegisterEventHandlers(bus events.Bus)

	// Close libera recursos no shutdown da aplicacao.
	Close() error
}

// Metadata descreve um modulo declarativamente.
type Metadata struct {
	// SchemaName e o schema Postgres alocado ao modulo. Para "core" e "core";
	// para "queue" sera "queue"; etc.
	SchemaName string

	// Label e o nome legivel mostrado em UI administrativa.
	Label string

	// Description e opcional; ajuda admins a decidir quando habilitar/desabilitar.
	Description string

	// IsCore indica modulos da plataforma (core, contacts) versus satelites
	// (queue, finance, tasks, omni, etc). Nao bloqueia nada hoje, mas pode ser
	// usado para regras como "core nao desabilita".
	IsCore bool

	// RequiresModules lista IDs de modulos obrigatorios para o funcionamento
	// deste. Validado por Registry.Build (falha se algum faltar). Ex: omni
	// poderia requerer "contacts".
	RequiresModules []string

	// OptionalModules lista IDs cujo recurso este modulo aproveita se estiver
	// habilitado para o account, mas nao bloqueia se ausente. Ex: finance
	// declara optional "contacts" — se ativo, usa como fonte de verdade; senao
	// cai em entidade local.
	OptionalModules []string

	// SortOrder define a ordem em UIs e logs. Menor primeiro. core = 0.
	SortOrder int
}

// Dependencies sao as dependencias passadas para Module.Build.
//
// Struct extensivel: novas deps opcionais entram aqui sem quebrar modulos
// existentes (basta nao consumir o campo). Para evitar amarras, cada campo
// e nilable conceitualmente — modulos checam o que precisam.
type Dependencies struct {
	// Pool e o pool PostgreSQL da plataforma. Compartilhado entre modulos
	// (todos usam o mesmo banco com schemas separados).
	Pool *pgxpool.Pool

	// Logger contextualizado com app_name. Modulos podem adicionar atributos
	// proprios via slog.With.
	Logger *slog.Logger

	// Bus e o event bus in-process. Use para publicar eventos do modulo e
	// se inscrever em topicos de outros modulos.
	Bus events.Bus

	// AuthMiddleware e o middleware de autenticacao legado. Continua sendo
	// fonte de Principal nos endpoints v1 e v2 ate que JWT v3 com sessionId
	// (Fase futura) o substitua.
	AuthMiddleware *auth.Middleware
}

// PermissionDef declara uma permissao do catalogo do modulo.
//
// Key segue o formato "<module>.<entity>.<verb>" (ex: "finance.invoices.read").
// SyncCatalog usa Key como primary key em core.permissions.
type PermissionDef struct {
	Key         string
	Label       string
	Description string

	// Scope e um de "account", "store" ou "platform".
	// "account": permissao avaliada dentro de uma account.
	// "store": permissao avaliada dentro de uma loja (modulo queue principalmente).
	// "platform": permissao global (so para platform_admin tipico).
	Scope string
}

// RoleTemplateDef declara um cargo-template do modulo.
//
// ID segue o formato "<module>.<role>" (ex: "core.owner", "finance.financial").
// Quando uma account e criada, o sistema clona estes templates em core.roles.
type RoleTemplateDef struct {
	ID          string
	Label       string
	Description string
	IsSystem    bool
	SortOrder   int

	// Permissions lista as Keys de PermissionDef que o template concede.
	// SyncCatalog valida que todas as keys existam em core.permissions antes
	// de inserir em core.role_template_permissions.
	Permissions []string
}

// (catalog sync interface vive em registry.go — CatalogRepository.)
