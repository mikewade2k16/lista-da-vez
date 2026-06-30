package core

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// admin_role_templates_repository.go — persistencia do CRUD de role templates
// (core.role_templates + core.role_template_permissions). SQL 100% parametrizado
// e schema-qualificado (core.*). Catalogo GLOBAL: NAO ha filtro por account
// (template e molde de plataforma). O gate platform_admin fica na borda HTTP.

// SQL exposto como constante para teste de contrato (fragmentos), espelhando o
// padrao de store_postgres_test.go (sem infra de integracao no modulo core).
const (
	// listRoleTemplatesQuery lista TODOS os templates com suas permission_keys
	// agregadas (array vazio quando o template nao tem permissao). Ordenado por
	// sort_order, id — estavel para a UI.
	listRoleTemplatesQuery = `
		select rt.id, rt.module_id, rt.label, rt.description,
		       rt.is_system, rt.is_locked, rt.sort_order,
		       coalesce(array_agg(rtp.permission_key order by rtp.permission_key)
		                filter (where rtp.permission_key is not null), '{}') as permission_keys
		from core.role_templates rt
		left join core.role_template_permissions rtp on rtp.role_template_id = rt.id
		group by rt.id, rt.module_id, rt.label, rt.description,
		         rt.is_system, rt.is_locked, rt.sort_order
		order by rt.sort_order asc, rt.id asc
	`

	// listAvailablePermissionsQuery e o catalogo para montar a matriz: todas as
	// permissoes vivas (deprecated_at is null) que NAO sao de escopo plataforma
	// (template e global por account; scope='platform' nao entra em papel de conta).
	listAvailablePermissionsQuery = `
		select key, label, module_id, scope
		from core.permissions
		where deprecated_at is null and scope <> 'platform'
		order by module_id asc, key asc
	`

	// findRoleTemplateQuery resolve um template por id (sem permissoes — o caller
	// reusa listRolePermissions do template via ListTemplatePermissionKeys).
	findRoleTemplateQuery = `
		select id, module_id, label, description, is_system, is_locked, sort_order
		from core.role_templates
		where id = $1
	`

	// invalidTemplatePermissionKeysQuery retorna, dentre as keys informadas, as
	// que NAO existem no catalogo, estao deprecated, ou tem scope='platform'.
	// Lista vazia = todas validas. Mesmo criterio de listAvailablePermissionsQuery.
	invalidTemplatePermissionKeysQuery = `
		select pk.key
		from unnest($1::text[]) as pk(key)
		where not exists (
			select 1 from core.permissions p
			where p.key = pk.key
			  and p.deprecated_at is null
			  and p.scope <> 'platform'
		)
		order by pk.key asc
	`
)

// RoleTemplateAdminRepository abstrai a persistencia do CRUD de templates.
// Interface focada (sem metodos a mais) — facilita fake em teste de service.
type RoleTemplateAdminRepository interface {
	ListRoleTemplates(ctx context.Context) ([]RoleTemplate, error)
	ListAvailablePermissions(ctx context.Context) ([]AvailablePermission, error)
	FindRoleTemplate(ctx context.Context, id string) (RoleTemplate, error)
	InvalidPermissionKeys(ctx context.Context, keys []string) ([]string, error)
	CreateRoleTemplate(ctx context.Context, in CreateRoleTemplateInput) (RoleTemplate, error)
	PatchRoleTemplate(ctx context.Context, id string, in PatchRoleTemplateInput) (RoleTemplate, error)
	ReplaceTemplatePermissions(ctx context.Context, id string, keys []string) (RoleTemplate, error)
	DeleteRoleTemplate(ctx context.Context, id string) error
}

// PostgresRoleTemplateAdminRepository implementa o CRUD contra o schema core.
type PostgresRoleTemplateAdminRepository struct {
	pool *pgxpool.Pool
}

// NewPostgresRoleTemplateAdminRepository injeta o pool compartilhado do core.
func NewPostgresRoleTemplateAdminRepository(pool *pgxpool.Pool) *PostgresRoleTemplateAdminRepository {
	return &PostgresRoleTemplateAdminRepository{pool: pool}
}

func (r *PostgresRoleTemplateAdminRepository) ListRoleTemplates(ctx context.Context) ([]RoleTemplate, error) {
	rows, err := r.pool.Query(ctx, listRoleTemplatesQuery)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	templates := make([]RoleTemplate, 0)
	for rows.Next() {
		var t RoleTemplate
		if err := rows.Scan(&t.ID, &t.ModuleID, &t.Label, &t.Description,
			&t.IsSystem, &t.IsLocked, &t.SortOrder, &t.PermissionKeys); err != nil {
			return nil, err
		}
		if t.PermissionKeys == nil {
			t.PermissionKeys = []string{}
		}
		templates = append(templates, t)
	}
	return templates, rows.Err()
}

func (r *PostgresRoleTemplateAdminRepository) ListAvailablePermissions(ctx context.Context) ([]AvailablePermission, error) {
	rows, err := r.pool.Query(ctx, listAvailablePermissionsQuery)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	perms := make([]AvailablePermission, 0)
	for rows.Next() {
		var p AvailablePermission
		if err := rows.Scan(&p.Key, &p.Label, &p.ModuleID, &p.Scope); err != nil {
			return nil, err
		}
		perms = append(perms, p)
	}
	return perms, rows.Err()
}

func (r *PostgresRoleTemplateAdminRepository) FindRoleTemplate(ctx context.Context, id string) (RoleTemplate, error) {
	return findRoleTemplateRow(ctx, r.pool, id)
}

// findRoleTemplateRow resolve um template por id usando qualquer querier (pool ou
// tx). PermissionKeys vem vazio aqui (carregado a parte quando o caller precisa).
func findRoleTemplateRow(ctx context.Context, q pgxQuerier, id string) (RoleTemplate, error) {
	var t RoleTemplate
	t.PermissionKeys = []string{}
	err := q.QueryRow(ctx, findRoleTemplateQuery, id).Scan(
		&t.ID, &t.ModuleID, &t.Label, &t.Description,
		&t.IsSystem, &t.IsLocked, &t.SortOrder,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return RoleTemplate{}, ErrTemplateNotFound
		}
		return RoleTemplate{}, err
	}
	return t, nil
}

func (r *PostgresRoleTemplateAdminRepository) InvalidPermissionKeys(ctx context.Context, keys []string) ([]string, error) {
	if len(keys) == 0 {
		return []string{}, nil
	}
	rows, err := r.pool.Query(ctx, invalidTemplatePermissionKeysQuery, keys)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	invalid := make([]string, 0)
	for rows.Next() {
		var k string
		if err := rows.Scan(&k); err != nil {
			return nil, err
		}
		invalid = append(invalid, k)
	}
	return invalid, rows.Err()
}

// CreateRoleTemplate insere o template (is_system=false, module_id=core,
// sort_order=200, is_locked=false) + suas permissoes numa unica transacao.
// Conflito de id (23505) -> ErrRoleTemplateConflict.
func (r *PostgresRoleTemplateAdminRepository) CreateRoleTemplate(ctx context.Context, in CreateRoleTemplateInput) (RoleTemplate, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return RoleTemplate{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	const insertTemplate = `
		insert into core.role_templates (id, module_id, label, description, is_system, is_locked, sort_order)
		values ($1, $2, $3, $4, false, false, $5)
	`
	if _, err := tx.Exec(ctx, insertTemplate,
		in.ID, roleTemplateCustomModuleID, in.Label, in.Description, roleTemplateCustomSortOrder,
	); err != nil {
		if isUniqueViolation(err) {
			return RoleTemplate{}, ErrRoleTemplateConflict
		}
		return RoleTemplate{}, fmt.Errorf("insert role template %q: %w", in.ID, err)
	}

	if err := replaceTemplatePermissionsTx(ctx, tx, in.ID, in.PermissionKeys); err != nil {
		return RoleTemplate{}, err
	}

	t, err := findRoleTemplateRow(ctx, tx, in.ID)
	if err != nil {
		return RoleTemplate{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return RoleTemplate{}, err
	}
	t.PermissionKeys = sortedCopy(in.PermissionKeys)
	return t, nil
}

// PatchRoleTemplate atualiza label/description/sort_order (so os campos enviados,
// via SET dinamico parametrizado). Nao toca permissoes. ErrTemplateNotFound se o
// id sumiu. is_system NUNCA e alterado aqui (e o gate fica no service).
func (r *PostgresRoleTemplateAdminRepository) PatchRoleTemplate(ctx context.Context, id string, in PatchRoleTemplateInput) (RoleTemplate, error) {
	sets := []string{}
	args := []any{id}

	// addSet acrescenta `coluna = $N` (N = posicao do arg recem-anexado) — o
	// indice deriva de len(args), sem contador manual (evita assignment ineficaz).
	addSet := func(column string, value any) {
		args = append(args, value)
		sets = append(sets, fmt.Sprintf("%s = $%d", column, len(args)))
	}

	if in.Label != nil {
		addSet("label", *in.Label)
	}
	if in.Description != nil {
		addSet("description", *in.Description)
	}
	if in.SortOrder != nil {
		addSet("sort_order", *in.SortOrder)
	}

	if len(sets) > 0 {
		query := "update core.role_templates set " + strings.Join(sets, ", ") +
			", updated_at = now() where id = $1"
		tag, err := r.pool.Exec(ctx, query, args...)
		if err != nil {
			return RoleTemplate{}, err
		}
		if tag.RowsAffected() == 0 {
			return RoleTemplate{}, ErrTemplateNotFound
		}
	}

	return r.findWithPermissions(ctx, id)
}

// ReplaceTemplatePermissions troca TODAS as permissoes do template (delete +
// insert) numa transacao. Validacao das keys e do is_system fica no service.
func (r *PostgresRoleTemplateAdminRepository) ReplaceTemplatePermissions(ctx context.Context, id string, keys []string) (RoleTemplate, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return RoleTemplate{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if err := replaceTemplatePermissionsTx(ctx, tx, id, keys); err != nil {
		return RoleTemplate{}, err
	}
	t, err := findRoleTemplateRow(ctx, tx, id)
	if err != nil {
		return RoleTemplate{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return RoleTemplate{}, err
	}
	t.PermissionKeys = sortedCopy(keys)
	return t, nil
}

// DeleteRoleTemplate remove o template; role_template_permissions cai por cascade
// (FK on delete cascade na migration 0100). ErrTemplateNotFound se nao existia.
func (r *PostgresRoleTemplateAdminRepository) DeleteRoleTemplate(ctx context.Context, id string) error {
	const query = `delete from core.role_templates where id = $1`
	tag, err := r.pool.Exec(ctx, query, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrTemplateNotFound
	}
	return nil
}

// findWithPermissions resolve o template + suas permission_keys atuais (apos um
// patch que nao toca permissoes). Reusa a query de listagem filtrando por id.
func (r *PostgresRoleTemplateAdminRepository) findWithPermissions(ctx context.Context, id string) (RoleTemplate, error) {
	t, err := findRoleTemplateRow(ctx, r.pool, id)
	if err != nil {
		return RoleTemplate{}, err
	}
	keys, err := r.listTemplatePermissionKeys(ctx, id)
	if err != nil {
		return RoleTemplate{}, err
	}
	t.PermissionKeys = keys
	return t, nil
}

func (r *PostgresRoleTemplateAdminRepository) listTemplatePermissionKeys(ctx context.Context, id string) ([]string, error) {
	const query = `
		select permission_key from core.role_template_permissions
		where role_template_id = $1 order by permission_key asc
	`
	rows, err := r.pool.Query(ctx, query, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	keys := make([]string, 0)
	for rows.Next() {
		var k string
		if err := rows.Scan(&k); err != nil {
			return nil, err
		}
		keys = append(keys, k)
	}
	return keys, rows.Err()
}

// replaceTemplatePermissionsTx limpa e reinsere as permissoes de um template
// dentro de uma transacao ja aberta (batch via unnest, sem N round-trips).
func replaceTemplatePermissionsTx(ctx context.Context, tx pgx.Tx, templateID string, keys []string) error {
	if _, err := tx.Exec(ctx,
		`delete from core.role_template_permissions where role_template_id = $1`, templateID,
	); err != nil {
		return fmt.Errorf("clear template permissions: %w", err)
	}
	if len(keys) > 0 {
		const insert = `
			insert into core.role_template_permissions (role_template_id, permission_key)
			select $1, key from unnest($2::text[]) as t(key)
			on conflict do nothing
		`
		if _, err := tx.Exec(ctx, insert, templateID, keys); err != nil {
			return fmt.Errorf("insert template permissions: %w", err)
		}
	}
	return nil
}

// findRoleTemplateRow usa pgxQuerier (definido em admin_users_links_repository.go:
// QueryRow, satisfeito por *pgxpool.Pool e pgx.Tx) para reusar a query tanto no
// pool quanto dentro de transacao, sem duplicar.

// sortedCopy devolve uma copia ordenada e nao-nula das keys, para o JSON de
// resposta espelhar a ordem persistida (order by permission_key) sem reler.
func sortedCopy(keys []string) []string {
	out := make([]string, len(keys))
	copy(out, keys)
	sort.Strings(out)
	return out
}
