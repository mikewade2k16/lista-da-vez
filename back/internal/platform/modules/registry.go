package modules

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sort"
)

// Registry mantem a lista de modulos registrados, sincroniza catalogo no banco
// e constroi cada modulo com suas dependencias.
//
// Uso tipico no bootstrap (app.go):
//
//	registry := modules.NewRegistry(logger)
//	registry.MustRegister(core.New())
//	registry.MustRegister(contacts.New()) // futuro
//	if err := registry.SyncCatalog(ctx, pool); err != nil { ... }
//	handles, err := registry.Build(deps)
//	for _, h := range handles {
//	    h.RegisterRoutes(mux)
//	    h.RegisterEventHandlers(bus)
//	}
type Registry struct {
	logger  *slog.Logger
	modules []Module
	byID    map[string]Module
}

// NewRegistry cria um Registry vazio.
func NewRegistry(logger *slog.Logger) *Registry {
	return &Registry{
		logger:  logger,
		modules: make([]Module, 0),
		byID:    make(map[string]Module),
	}
}

// MustRegister adiciona um modulo. Falha (panic) se ID duplicado ou vazio —
// erro de programacao que precisa ser visto no boot.
func (r *Registry) MustRegister(module Module) {
	if module == nil {
		panic("modules: cannot register nil module")
	}

	id := module.ID()
	if id == "" {
		panic("modules: module must have a non-empty ID")
	}

	if _, exists := r.byID[id]; exists {
		panic(fmt.Sprintf("modules: duplicate module ID %q", id))
	}

	r.byID[id] = module
	r.modules = append(r.modules, module)
}

// Modules retorna a lista registrada na ordem de registro. Util para
// inspecao/log; nao usar para rodar Build (use Build()).
func (r *Registry) Modules() []Module {
	cloned := make([]Module, len(r.modules))
	copy(cloned, r.modules)
	return cloned
}

// SyncCatalog popula core.modules, core.permissions e core.role_templates
// (e role_template_permissions) a partir das declaracoes dos modulos.
//
// Regras inegociaveis (vide CONTRACT_FREEZE.md):
//  1. Cria entradas novas.
//  2. Atualiza apenas label/description/sort_order/dependencias de existentes.
//  3. Marca permissoes removidas com deprecated_at = now(). NUNCA DELETE auto.
//  4. NAO toca em core.roles ou core.role_permissions (pertencem a Account).
//  5. NAO sobrescreve core.role_template_permissions de templates ja existentes
//     (templates sao versionados — para mudar, criar template novo).
func (r *Registry) SyncCatalog(ctx context.Context, repo CatalogRepository) error {
	if repo == nil {
		return errors.New("modules: SyncCatalog requires a non-nil CatalogRepository")
	}

	// 1) Modulos
	for _, module := range r.modules {
		metadata := module.Metadata()
		if err := repo.UpsertModule(ctx, ModuleRow{
			ID:              module.ID(),
			SchemaName:      metadata.SchemaName,
			Label:           metadata.Label,
			Description:     metadata.Description,
			IsCore:          metadata.IsCore,
			RequiresModules: metadata.RequiresModules,
			OptionalModules: metadata.OptionalModules,
			SortOrder:       metadata.SortOrder,
		}); err != nil {
			return fmt.Errorf("modules: upsert module %q: %w", module.ID(), err)
		}
	}

	// 2) Permissoes — coleta keys declaradas
	declared := make(map[string]struct{})
	for _, module := range r.modules {
		for _, perm := range module.Permissions() {
			if err := validatePermissionDef(module.ID(), perm); err != nil {
				return err
			}

			declared[perm.Key] = struct{}{}
			if err := repo.UpsertPermission(ctx, PermissionRow{
				Key:         perm.Key,
				ModuleID:    module.ID(),
				Label:       perm.Label,
				Description: perm.Description,
				Scope:       perm.Scope,
			}); err != nil {
				return fmt.Errorf("modules: upsert permission %q: %w", perm.Key, err)
			}
		}
	}

	// Marca como deprecated as keys que sumiram dos catalogos.
	deprecated, err := repo.MarkDeprecatedPermissions(ctx, declared)
	if err != nil {
		return fmt.Errorf("modules: mark deprecated permissions: %w", err)
	}
	if deprecated > 0 {
		r.logger.Warn(
			"permissions marked as deprecated",
			slog.Int("count", deprecated),
		)
	}

	// 3) Role templates + role_template_permissions (so para template novo)
	for _, module := range r.modules {
		for _, tmpl := range module.RoleTemplates() {
			if err := validateRoleTemplate(module.ID(), tmpl, declared); err != nil {
				return err
			}

			created, err := repo.UpsertRoleTemplate(ctx, RoleTemplateRow{
				ID:          tmpl.ID,
				ModuleID:    module.ID(),
				Label:       tmpl.Label,
				Description: tmpl.Description,
				IsSystem:    tmpl.IsSystem,
				IsLocked:    tmpl.IsLocked,
				SortOrder:   tmpl.SortOrder,
			})
			if err != nil {
				return fmt.Errorf("modules: upsert role template %q: %w", tmpl.ID, err)
			}

			if created {
				if err := repo.SetTemplatePermissions(ctx, tmpl.ID, tmpl.Permissions); err != nil {
					return fmt.Errorf(
						"modules: set permissions for new template %q: %w",
						tmpl.ID, err,
					)
				}
			}
		}
	}

	return nil
}

// Build constroi cada modulo registrado com as dependencias fornecidas.
// Valida RequiresModules antes de chamar Module.Build.
func (r *Registry) Build(deps Dependencies) ([]Handle, error) {
	// Valida dependencies obrigatorias entre modulos primeiro.
	for _, module := range r.modules {
		for _, requiredID := range module.Metadata().RequiresModules {
			if _, ok := r.byID[requiredID]; !ok {
				return nil, fmt.Errorf(
					"modules: %q requires %q which is not registered",
					module.ID(), requiredID,
				)
			}
		}
	}

	// Ordena por SortOrder para boot determinístico.
	ordered := append([]Module(nil), r.modules...)
	sort.SliceStable(ordered, func(i, j int) bool {
		return ordered[i].Metadata().SortOrder < ordered[j].Metadata().SortOrder
	})

	handles := make([]Handle, 0, len(ordered))
	for _, module := range ordered {
		handle, err := module.Build(deps)
		if err != nil {
			return nil, fmt.Errorf("modules: build %q: %w", module.ID(), err)
		}
		if handle == nil {
			return nil, fmt.Errorf("modules: %q returned nil handle", module.ID())
		}
		handles = append(handles, handle)

		r.logger.Info(
			"module built",
			slog.String("module_id", module.ID()),
			slog.String("schema", module.Metadata().SchemaName),
			slog.Bool("is_core", module.Metadata().IsCore),
		)
	}

	return handles, nil
}

// ============================================================================
// Validacoes
// ============================================================================

func validatePermissionDef(moduleID string, perm PermissionDef) error {
	if perm.Key == "" {
		return fmt.Errorf("modules: permission in module %q has empty Key", moduleID)
	}
	switch perm.Scope {
	case "account", "store", "platform":
	default:
		return fmt.Errorf(
			"modules: permission %q has invalid Scope %q (expected account|store|platform)",
			perm.Key, perm.Scope,
		)
	}
	return nil
}

func validateRoleTemplate(moduleID string, tmpl RoleTemplateDef, declared map[string]struct{}) error {
	if tmpl.ID == "" {
		return fmt.Errorf("modules: role template in module %q has empty ID", moduleID)
	}
	for _, key := range tmpl.Permissions {
		if _, ok := declared[key]; !ok {
			return fmt.Errorf(
				"modules: role template %q references undeclared permission %q",
				tmpl.ID, key,
			)
		}
	}
	return nil
}

// ============================================================================
// CatalogRepository — abstracao da persistencia do catalogo
// ============================================================================

// CatalogRepository abstrai a persistencia do catalogo no banco. Implementacao
// padrao em catalog_postgres.go usa o pool PostgreSQL diretamente.
//
// Manter como interface facilita testes do Registry sem precisar de banco.
type CatalogRepository interface {
	UpsertModule(ctx context.Context, row ModuleRow) error
	UpsertPermission(ctx context.Context, row PermissionRow) error
	MarkDeprecatedPermissions(ctx context.Context, declaredKeys map[string]struct{}) (int, error)

	// UpsertRoleTemplate retorna created=true se o template nao existia antes.
	// Quem chama usa esse flag para popular role_template_permissions APENAS
	// na primeira vez (regra: nao sobrescrever template existente).
	UpsertRoleTemplate(ctx context.Context, row RoleTemplateRow) (created bool, err error)

	// SetTemplatePermissions substitui as permissoes do template. So deve ser
	// chamado quando UpsertRoleTemplate retornou created=true.
	SetTemplatePermissions(ctx context.Context, templateID string, permissionKeys []string) error
}

// ModuleRow espelha core.modules.
type ModuleRow struct {
	ID              string
	SchemaName      string
	Label           string
	Description     string
	IsCore          bool
	RequiresModules []string
	OptionalModules []string
	SortOrder       int
}

// PermissionRow espelha core.permissions.
type PermissionRow struct {
	Key         string
	ModuleID    string
	Label       string
	Description string
	Scope       string
}

// RoleTemplateRow espelha core.role_templates.
type RoleTemplateRow struct {
	ID          string
	ModuleID    string
	Label       string
	Description string
	IsSystem    bool
	IsLocked    bool
	SortOrder   int
}
