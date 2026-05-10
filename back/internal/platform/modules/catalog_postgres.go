package modules

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PostgresCatalogRepository implementa CatalogRepository contra core.modules,
// core.permissions, core.role_templates e core.role_template_permissions.
type PostgresCatalogRepository struct {
	pool *pgxpool.Pool
}

// NewPostgresCatalogRepository cria a implementacao Postgres.
func NewPostgresCatalogRepository(pool *pgxpool.Pool) *PostgresCatalogRepository {
	return &PostgresCatalogRepository{pool: pool}
}

// UpsertModule sincroniza uma linha em core.modules.
//
// Insert se id novo; update de label/description/sort_order/dependencias para
// linhas existentes. schema_name e is_core nunca mudam apos criados (proteja-se
// de typo).
func (r *PostgresCatalogRepository) UpsertModule(ctx context.Context, row ModuleRow) error {
	const query = `
		insert into core.modules (
			id, schema_name, label, description, is_core,
			requires_modules, optional_modules, sort_order
		) values (
			$1, $2, $3, $4, $5, $6, $7, $8
		)
		on conflict (id) do update set
			label = excluded.label,
			description = excluded.description,
			requires_modules = excluded.requires_modules,
			optional_modules = excluded.optional_modules,
			sort_order = excluded.sort_order,
			updated_at = now()
	`

	_, err := r.pool.Exec(
		ctx, query,
		row.ID,
		row.SchemaName,
		row.Label,
		row.Description,
		row.IsCore,
		row.RequiresModules,
		row.OptionalModules,
		row.SortOrder,
	)
	return err
}

// UpsertPermission sincroniza uma linha em core.permissions.
//
// Insert quando key novo; update de label/description/scope para existentes.
// Reativa (deprecated_at = null) caso a key tenha sido marcada como
// deprecated antes e agora reaparecer no catalogo.
func (r *PostgresCatalogRepository) UpsertPermission(ctx context.Context, row PermissionRow) error {
	const query = `
		insert into core.permissions (key, module_id, label, description, scope, deprecated_at)
		values ($1, $2, $3, $4, $5, null)
		on conflict (key) do update set
			module_id = excluded.module_id,
			label = excluded.label,
			description = excluded.description,
			scope = excluded.scope,
			deprecated_at = null,
			updated_at = now()
	`

	_, err := r.pool.Exec(
		ctx, query,
		row.Key,
		row.ModuleID,
		row.Label,
		row.Description,
		row.Scope,
	)
	return err
}

// MarkDeprecatedPermissions marca como deprecated todas as keys que NAO estao
// na lista declarada. NUNCA executa DELETE — preserva historico para auditoria
// e permite migration manual depois.
//
// Retorna a quantidade afetada para fim de log.
func (r *PostgresCatalogRepository) MarkDeprecatedPermissions(
	ctx context.Context,
	declaredKeys map[string]struct{},
) (int, error) {
	if len(declaredKeys) == 0 {
		// Sem keys declaradas — nao deprecia nada (modo defensivo: provavelmente
		// foi um boot sem modulos, nao deve mascarar todo o catalogo).
		return 0, nil
	}

	keys := make([]string, 0, len(declaredKeys))
	for key := range declaredKeys {
		keys = append(keys, key)
	}

	const query = `
		update core.permissions
		   set deprecated_at = now(),
		       updated_at = now()
		 where deprecated_at is null
		   and key <> all($1::text[])
	`

	tag, err := r.pool.Exec(ctx, query, keys)
	if err != nil {
		return 0, err
	}
	return int(tag.RowsAffected()), nil
}

// UpsertRoleTemplate sincroniza uma linha em core.role_templates.
// Retorna created=true se a linha era nova (caller decide se popula
// role_template_permissions).
func (r *PostgresCatalogRepository) UpsertRoleTemplate(
	ctx context.Context,
	row RoleTemplateRow,
) (bool, error) {
	const query = `
		insert into core.role_templates (id, module_id, label, description, is_system, is_locked, sort_order)
		values ($1, $2, $3, $4, $5, $6, $7)
		on conflict (id) do update set
			label = excluded.label,
			description = excluded.description,
			is_system = excluded.is_system,
			is_locked = excluded.is_locked,
			sort_order = excluded.sort_order,
			updated_at = now()
		returning (xmax = 0) as inserted
	`

	var inserted bool
	err := r.pool.QueryRow(
		ctx, query,
		row.ID,
		row.ModuleID,
		row.Label,
		row.Description,
		row.IsSystem,
		row.IsLocked,
		row.SortOrder,
	).Scan(&inserted)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return false, fmt.Errorf("upsert role template %q returned no row", row.ID)
		}
		return false, err
	}
	return inserted, nil
}

// SetTemplatePermissions popula core.role_template_permissions para um template
// recém-criado. Usa transacao para garantir atomicidade (limpa + insere).
//
// IMPORTANTE: este metodo nao deve ser chamado para template ja existente.
// Templates sao versionados — para mudar permissoes, criar template novo.
// Quem chama (Registry) garante isso usando o flag created de UpsertRoleTemplate.
func (r *PostgresCatalogRepository) SetTemplatePermissions(
	ctx context.Context,
	templateID string,
	permissionKeys []string,
) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err := tx.Exec(
		ctx,
		`delete from core.role_template_permissions where role_template_id = $1`,
		templateID,
	); err != nil {
		return fmt.Errorf("clear template permissions: %w", err)
	}

	if len(permissionKeys) > 0 {
		// Insert em batch via unnest evita N round-trips.
		const insertQuery = `
			insert into core.role_template_permissions (role_template_id, permission_key)
			select $1, key from unnest($2::text[]) as t(key)
			on conflict do nothing
		`
		if _, err := tx.Exec(ctx, insertQuery, templateID, permissionKeys); err != nil {
			return fmt.Errorf("insert template permissions: %w", err)
		}
	}

	return tx.Commit(ctx)
}
