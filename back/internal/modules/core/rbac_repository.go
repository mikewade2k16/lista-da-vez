package core

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// RBACRepository abstrai o acesso às tabelas de RBAC do schema core.
type RBACRepository interface {
	// ---- CRUD de roles (Item 3) ----

	// ListRolesForAccount lista todos os roles da account, ordenados por is_locked desc, code asc.
	ListRolesForAccount(ctx context.Context, accountID string) ([]Role, error)

	// FindRole busca um role verificando que pertence à account. ErrRoleNotFound se não existe.
	FindRole(ctx context.Context, accountID, roleID string) (Role, error)

	// CreateRole insere um role customizado (não derivado de template). Retorna
	// ErrRoleCodeConflict se o code já existe na account.
	CreateRole(ctx context.Context, accountID, code, label, description string) (Role, error)

	// UpdateRole atualiza label e description de um role.
	UpdateRole(ctx context.Context, accountID, roleID, label, description string) error

	// ReplaceRolePermissions substitui todas as permissões do role em uma transação
	// (DELETE + INSERT). Não valida keys — validação fica no service.
	ReplaceRolePermissions(ctx context.Context, roleID string, permKeys []string) error

	// ListRolePermissions retorna as permission_keys do role.
	ListRolePermissions(ctx context.Context, roleID string) ([]string, error)

	// DeleteRole remove o role. Não verifica is_locked — verificação no service.
	DeleteRole(ctx context.Context, accountID, roleID string) error

	// ---- Atribuição (Item 3) ----

	// AssignRoleToUser cria ou garante a atribuição user→role na account.
	AssignRoleToUser(ctx context.Context, accountID, userID, roleID string) error

	// RemoveRoleFromUser remove a atribuição user→role na account. Não falha se
	// já não existia.
	RemoveRoleFromUser(ctx context.Context, accountID, userID, roleID string) error

	// ---- MeContext (Item 5) ----

	// ListRolesForUser retorna os roles atribuídos ao user na account.
	ListRolesForUser(ctx context.Context, accountID, userID string) ([]Role, error)

	// ListPermissionsForUser resolve as permissões efetivas do user na account:
	// UNION de role_permissions dos roles atribuídos + allow overrides
	// EXCEPT deny overrides.
	ListPermissionsForUser(ctx context.Context, accountID, userID string) ([]string, error)

	// ---- Validação ----

	// InvalidPermissionKeys retorna as keys da lista que NÃO existem em
	// core.permissions (ou estão deprecated) ou cujo módulo não está
	// habilitado para a account. Lista vazia = todas válidas.
	InvalidPermissionKeys(ctx context.Context, accountID string, keys []string) ([]string, error)

	// CheckMembership verifica que o user tem membership ativa na account.
	// ErrAccountNotMember se não tiver.
	CheckMembership(ctx context.Context, accountID, userID string) error

	// PlatformScopedKeys retorna, dentre as keys informadas, as que tem
	// scope='platform' (bloqueadas em papel custom — um core.roles.manage de
	// cliente nao pode conceder permissao de plataforma). Lista vazia = nenhuma.
	PlatformScopedKeys(ctx context.Context, keys []string) ([]string, error)

	// ---- Gate de papeis por cliente (impl. em rbac_repository_assign.go) ----

	HasAccountPermission(ctx context.Context, accountID, userID, permKey string) (bool, error)
	CanAccessAccountRoles(ctx context.Context, accountID, userID string, requireManage bool) (bool, error)
	ReplaceUserRoleAssignments(ctx context.Context, accountID, userID string, roleIDs []string) error
}

// PostgresRBACRepository implementa RBACRepository contra o schema core.
type PostgresRBACRepository struct {
	pool *pgxpool.Pool
}

// NewPostgresRBACRepository cria a implementação Postgres do RBACRepository.
func NewPostgresRBACRepository(pool *pgxpool.Pool) *PostgresRBACRepository {
	return &PostgresRBACRepository{pool: pool}
}

// ============================================================================
// CRUD de roles
// ============================================================================

func (r *PostgresRBACRepository) ListRolesForAccount(
	ctx context.Context,
	accountID string,
) ([]Role, error) {
	const query = `
		select id, account_id, coalesce(cloned_from_template_id,''), code, label,
		       description, is_default, is_locked, created_at, updated_at
		from core.roles
		where account_id = $1::uuid
		order by is_locked desc, code asc
	`

	rows, err := r.pool.Query(ctx, query, accountID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	roles := make([]Role, 0)
	for rows.Next() {
		var ro Role
		if err := rows.Scan(&ro.ID, &ro.AccountID, &ro.ClonedFromTemplateID, &ro.Code,
			&ro.Label, &ro.Description, &ro.IsDefault, &ro.IsLocked,
			&ro.CreatedAt, &ro.UpdatedAt); err != nil {
			return nil, err
		}
		roles = append(roles, ro)
	}
	return roles, rows.Err()
}

func (r *PostgresRBACRepository) FindRole(
	ctx context.Context,
	accountID, roleID string,
) (Role, error) {
	const query = `
		select id, account_id, coalesce(cloned_from_template_id,''), code, label,
		       description, is_default, is_locked, created_at, updated_at
		from core.roles
		where account_id = $1::uuid and id = $2::uuid
	`

	var ro Role
	err := r.pool.QueryRow(ctx, query, accountID, roleID).Scan(
		&ro.ID, &ro.AccountID, &ro.ClonedFromTemplateID, &ro.Code,
		&ro.Label, &ro.Description, &ro.IsDefault, &ro.IsLocked,
		&ro.CreatedAt, &ro.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Role{}, ErrRoleNotFound
		}
		return Role{}, err
	}
	return ro, nil
}

func (r *PostgresRBACRepository) CreateRole(
	ctx context.Context,
	accountID, code, label, description string,
) (Role, error) {
	const query = `
		insert into core.roles (account_id, code, label, description)
		values ($1::uuid, $2, $3, $4)
		returning id, account_id, coalesce(cloned_from_template_id,''), code,
		          label, description, is_default, is_locked, created_at, updated_at
	`

	var ro Role
	err := r.pool.QueryRow(ctx, query, accountID, code, label, description).Scan(
		&ro.ID, &ro.AccountID, &ro.ClonedFromTemplateID, &ro.Code,
		&ro.Label, &ro.Description, &ro.IsDefault, &ro.IsLocked,
		&ro.CreatedAt, &ro.UpdatedAt,
	)
	if err != nil {
		if isUniqueViolation(err) {
			return Role{}, ErrRoleCodeConflict
		}
		return Role{}, fmt.Errorf("create role %q for account %s: %w", code, accountID, err)
	}
	return ro, nil
}

func (r *PostgresRBACRepository) UpdateRole(
	ctx context.Context,
	accountID, roleID, label, description string,
) error {
	const query = `
		update core.roles
		   set label = $3, description = $4, updated_at = now()
		 where account_id = $1::uuid and id = $2::uuid
	`

	tag, err := r.pool.Exec(ctx, query, accountID, roleID, label, description)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrRoleNotFound
	}
	return nil
}

func (r *PostgresRBACRepository) ReplaceRolePermissions(
	ctx context.Context,
	roleID string,
	permKeys []string,
) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err := tx.Exec(ctx,
		`delete from core.role_permissions where role_id = $1::uuid`, roleID,
	); err != nil {
		return fmt.Errorf("clear role permissions: %w", err)
	}

	if len(permKeys) > 0 {
		const insert = `
			insert into core.role_permissions (role_id, permission_key)
			select $1::uuid, key from unnest($2::text[]) as t(key)
			on conflict do nothing
		`
		if _, err := tx.Exec(ctx, insert, roleID, permKeys); err != nil {
			return fmt.Errorf("insert role permissions: %w", err)
		}
	}

	return tx.Commit(ctx)
}

func (r *PostgresRBACRepository) ListRolePermissions(
	ctx context.Context,
	roleID string,
) ([]string, error) {
	const query = `
		select permission_key from core.role_permissions
		where role_id = $1::uuid order by permission_key asc
	`

	rows, err := r.pool.Query(ctx, query, roleID)
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

func (r *PostgresRBACRepository) DeleteRole(
	ctx context.Context,
	accountID, roleID string,
) error {
	const query = `delete from core.roles where account_id = $1::uuid and id = $2::uuid`
	tag, err := r.pool.Exec(ctx, query, accountID, roleID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrRoleNotFound
	}
	return nil
}

// ============================================================================
// Atribuição de roles a usuários
// ============================================================================

func (r *PostgresRBACRepository) AssignRoleToUser(
	ctx context.Context,
	accountID, userID, roleID string,
) error {
	const query = `
		insert into core.user_role_assignments (account_id, user_id, role_id)
		values ($1::uuid, $2::uuid, $3::uuid)
		on conflict (account_id, user_id, role_id) do nothing
	`
	_, err := r.pool.Exec(ctx, query, accountID, userID, roleID)
	return err
}

func (r *PostgresRBACRepository) RemoveRoleFromUser(
	ctx context.Context,
	accountID, userID, roleID string,
) error {
	const query = `
		delete from core.user_role_assignments
		where account_id = $1::uuid and user_id = $2::uuid and role_id = $3::uuid
	`
	_, err := r.pool.Exec(ctx, query, accountID, userID, roleID)
	return err
}

// ============================================================================
// Resolução de permissões (MeContext)
// ============================================================================

func (r *PostgresRBACRepository) ListRolesForUser(
	ctx context.Context,
	accountID, userID string,
) ([]Role, error) {
	const query = `
		select ro.id, ro.account_id, coalesce(ro.cloned_from_template_id,''),
		       ro.code, ro.label, ro.description, ro.is_default, ro.is_locked,
		       ro.created_at, ro.updated_at
		from core.user_role_assignments ura
		join core.roles ro on ro.id = ura.role_id
		where ura.account_id = $1::uuid and ura.user_id = $2::uuid
		order by ro.is_locked desc, ro.code asc
	`

	rows, err := r.pool.Query(ctx, query, accountID, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	roles := make([]Role, 0)
	for rows.Next() {
		var ro Role
		if err := rows.Scan(&ro.ID, &ro.AccountID, &ro.ClonedFromTemplateID, &ro.Code,
			&ro.Label, &ro.Description, &ro.IsDefault, &ro.IsLocked,
			&ro.CreatedAt, &ro.UpdatedAt); err != nil {
			return nil, err
		}
		roles = append(roles, ro)
	}
	return roles, rows.Err()
}

func (r *PostgresRBACRepository) ListPermissionsForUser(
	ctx context.Context,
	accountID, userID string,
) ([]string, error) {
	const query = `
		select rp.permission_key
		from core.user_role_assignments ura
		join core.role_permissions rp on rp.role_id = ura.role_id
		where ura.account_id = $1::uuid and ura.user_id = $2::uuid

		union

		select permission_key
		from core.user_permission_overrides
		where account_id = $1::uuid and user_id = $2::uuid
		  and effect = 'allow' and is_active = true

		except

		select permission_key
		from core.user_permission_overrides
		where account_id = $1::uuid and user_id = $2::uuid
		  and effect = 'deny' and is_active = true

		order by 1 asc
	`

	rows, err := r.pool.Query(ctx, query, accountID, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	perms := make([]string, 0)
	for rows.Next() {
		var p string
		if err := rows.Scan(&p); err != nil {
			return nil, err
		}
		perms = append(perms, p)
	}
	return perms, rows.Err()
}

// ============================================================================
// Validação
// ============================================================================

func (r *PostgresRBACRepository) InvalidPermissionKeys(
	ctx context.Context,
	accountID string,
	keys []string,
) ([]string, error) {
	if len(keys) == 0 {
		return []string{}, nil
	}

	const query = `
		select pk.key
		from unnest($1::text[]) as pk(key)
		where not exists (
			select 1
			from core.permissions p
			join core.account_modules am
			    on am.module_id = p.module_id
			    and am.account_id = $2::uuid
			    and am.enabled = true
			where p.key = pk.key
			  and p.deprecated_at is null
		)
		order by pk.key asc
	`

	rows, err := r.pool.Query(ctx, query, keys, accountID)
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

func (r *PostgresRBACRepository) CheckMembership(
	ctx context.Context,
	accountID, userID string,
) error {
	const query = `
		select 1 from core.account_users
		where account_id = $1::uuid and user_id = $2::uuid and is_active = true
	`
	var dummy int
	err := r.pool.QueryRow(ctx, query, accountID, userID).Scan(&dummy)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrAccountNotMember
	}
	return err
}

// ============================================================================
// Helpers internos
// ============================================================================

func isUniqueViolation(err error) bool {
	var pgerr *pgconn.PgError
	return errors.As(err, &pgerr) && pgerr.Code == "23505"
}
