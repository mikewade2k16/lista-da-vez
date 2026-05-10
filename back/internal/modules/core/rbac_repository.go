package core

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// RBACRepository abstrai o acesso às tabelas de RBAC do schema core.
type RBACRepository interface {
	// ListTemplatesForModules retorna todos os role_templates dos módulos
	// informados, ordenados por sort_order asc. Retorna lista vazia se
	// moduleIDs for vazio.
	ListTemplatesForModules(ctx context.Context, moduleIDs []string) ([]RoleTemplate, error)

	// ListTemplatePermissionKeys retorna as permission_keys associadas ao template.
	ListTemplatePermissionKeys(ctx context.Context, templateID string) ([]string, error)

	// CloneTemplate insere um core.roles clonado do template informado.
	// O code do role é igual ao template.ID (ex: "core.owner").
	// Idempotente via ON CONFLICT (account_id, code) DO NOTHING.
	// Retorna (roleID, created=true) se inserido; ("", created=false) se já existia.
	CloneTemplate(ctx context.Context, accountID string, tmpl RoleTemplate) (roleID string, created bool, err error)

	// SetRolePermissions insere as permissões para um role recém-criado.
	// Idempotente (ON CONFLICT DO NOTHING). Deve ser chamado apenas quando
	// CloneTemplate retornar created=true.
	SetRolePermissions(ctx context.Context, roleID string, permKeys []string) error
}

// PostgresRBACRepository implementa RBACRepository contra o schema core.
type PostgresRBACRepository struct {
	pool *pgxpool.Pool
}

// NewPostgresRBACRepository cria a implementação Postgres do RBACRepository.
func NewPostgresRBACRepository(pool *pgxpool.Pool) *PostgresRBACRepository {
	return &PostgresRBACRepository{pool: pool}
}

func (r *PostgresRBACRepository) ListTemplatesForModules(
	ctx context.Context,
	moduleIDs []string,
) ([]RoleTemplate, error) {
	if len(moduleIDs) == 0 {
		return []RoleTemplate{}, nil
	}

	const query = `
		select id, module_id, label, description, is_system, is_locked, sort_order
		from core.role_templates
		where module_id = any($1::text[])
		order by sort_order asc, id asc
	`

	rows, err := r.pool.Query(ctx, query, moduleIDs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	templates := make([]RoleTemplate, 0)
	for rows.Next() {
		var t RoleTemplate
		if err := rows.Scan(
			&t.ID, &t.ModuleID, &t.Label, &t.Description,
			&t.IsSystem, &t.IsLocked, &t.SortOrder,
		); err != nil {
			return nil, err
		}
		templates = append(templates, t)
	}
	return templates, rows.Err()
}

func (r *PostgresRBACRepository) ListTemplatePermissionKeys(
	ctx context.Context,
	templateID string,
) ([]string, error) {
	const query = `
		select permission_key
		from core.role_template_permissions
		where role_template_id = $1
		order by permission_key asc
	`

	rows, err := r.pool.Query(ctx, query, templateID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	keys := make([]string, 0)
	for rows.Next() {
		var key string
		if err := rows.Scan(&key); err != nil {
			return nil, err
		}
		keys = append(keys, key)
	}
	return keys, rows.Err()
}

func (r *PostgresRBACRepository) CloneTemplate(
	ctx context.Context,
	accountID string,
	tmpl RoleTemplate,
) (string, bool, error) {
	// code = tmpl.ID (ex: "core.owner") — identificador estável e único por account.
	const query = `
		insert into core.roles (
			account_id, cloned_from_template_id, code, label, description, is_locked
		)
		values ($1::uuid, $2, $3, $4, $5, $6)
		on conflict (account_id, code) do nothing
		returning id
	`

	var roleID string
	err := r.pool.QueryRow(ctx, query,
		accountID, tmpl.ID, tmpl.ID, tmpl.Label, tmpl.Description, tmpl.IsLocked,
	).Scan(&roleID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", false, nil
		}
		return "", false, fmt.Errorf("clone template %q for account %s: %w", tmpl.ID, accountID, err)
	}
	return roleID, true, nil
}

func (r *PostgresRBACRepository) SetRolePermissions(
	ctx context.Context,
	roleID string,
	permKeys []string,
) error {
	if len(permKeys) == 0 {
		return nil
	}

	const query = `
		insert into core.role_permissions (role_id, permission_key)
		select $1::uuid, key from unnest($2::text[]) as t(key)
		on conflict do nothing
	`
	if _, err := r.pool.Exec(ctx, query, roleID, permKeys); err != nil {
		return fmt.Errorf("set permissions for role %s: %w", roleID, err)
	}
	return nil
}
