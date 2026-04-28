package access

import (
	"context"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
)

type PostgresRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresRepository(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{pool: pool}
}

func (repository *PostgresRepository) ListRolePermissions(ctx context.Context, role auth.Role) ([]string, error) {
	rows, err := repository.pool.Query(ctx, `
		select permission_key
		from access_role_permissions
		where role = $1;
	`, role)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	permissionKeys := make([]string, 0)
	for rows.Next() {
		var permissionKey string
		if err := rows.Scan(&permissionKey); err != nil {
			return nil, err
		}

		permissionKeys = append(permissionKeys, permissionKey)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return RecognizedPermissionKeys(permissionKeys), nil
}

func (repository *PostgresRepository) ListAllRolePermissions(ctx context.Context) ([]RoleGrant, error) {
	rows, err := repository.pool.Query(ctx, `
		select role, permission_key
		from access_role_permissions;
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	grants := make([]RoleGrant, 0)
	for rows.Next() {
		var role string
		var permissionKey string
		if err := rows.Scan(&role, &permissionKey); err != nil {
			return nil, err
		}

		if _, ok := PermissionDefinitionForKey(permissionKey); !ok {
			continue
		}

		grants = append(grants, RoleGrant{
			Role:          auth.Role(strings.TrimSpace(role)),
			PermissionKey: strings.TrimSpace(permissionKey),
		})
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return grants, nil
}

func (repository *PostgresRepository) ListActiveTenantIDs(ctx context.Context) ([]string, error) {
	rows, err := repository.pool.Query(ctx, `
		select id::text
		from tenants
		where is_active = true
		order by name asc;
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	tenantIDs := make([]string, 0)
	for rows.Next() {
		var tenantID string
		if err := rows.Scan(&tenantID); err != nil {
			return nil, err
		}

		tenantIDs = append(tenantIDs, strings.TrimSpace(tenantID))
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return tenantIDs, nil
}

func (repository *PostgresRepository) ReplaceRolePermissions(ctx context.Context, role auth.Role, permissionKeys []string) error {
	keys := RecognizedPermissionKeys(permissionKeys)
	tx, err := repository.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	if _, err := tx.Exec(ctx, `
		delete from access_role_permissions
		where role = $1
		  and permission_key = any($2::text[]);
	`, role, PermissionCatalogKeys()); err != nil {
		return err
	}

	for _, permissionKey := range keys {
		if _, err := tx.Exec(ctx, `
			insert into access_role_permissions (role, permission_key)
			values ($1, $2)
			on conflict (role, permission_key) do nothing;
		`, role, permissionKey); err != nil {
			return err
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return err
	}

	return nil
}

func (repository *PostgresRepository) ListUserOverrides(ctx context.Context, userID string) ([]UserOverride, error) {
	rows, err := repository.pool.Query(ctx, `
		select
			id::text,
			user_id::text,
			permission_key,
			effect,
			coalesce(tenant_id::text, ''),
			coalesce(store_id::text, ''),
			note,
			is_active
		from user_access_overrides
		where user_id = $1::uuid;
	`, strings.TrimSpace(userID))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	overrides := make([]UserOverride, 0)
	for rows.Next() {
		var override UserOverride
		if err := rows.Scan(
			&override.ID,
			&override.UserID,
			&override.PermissionKey,
			&override.Effect,
			&override.TenantID,
			&override.StoreID,
			&override.Note,
			&override.IsActive,
		); err != nil {
			return nil, err
		}

		if _, ok := PermissionDefinitionForKey(override.PermissionKey); !ok {
			continue
		}

		overrides = append(overrides, override)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	sort.Slice(overrides, func(left, right int) bool {
		return overrides[left].PermissionKey < overrides[right].PermissionKey
	})

	return overrides, nil
}

func (repository *PostgresRepository) ReplaceUserOverrides(ctx context.Context, userID string, overrides []UserOverride, createdByUserID string) ([]UserOverride, error) {
	tx, err := repository.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	if _, err := tx.Exec(ctx, `
		delete from user_access_overrides
		where user_id = $1::uuid
		  and permission_key = any($2::text[]);
	`, strings.TrimSpace(userID), PermissionCatalogKeys()); err != nil {
		return nil, err
	}

	saved := make([]UserOverride, 0, len(overrides))
	for _, override := range overrides {
		if !override.IsActive {
			continue
		}

		var savedOverride UserOverride
		if err := tx.QueryRow(ctx, `
			insert into user_access_overrides (
				user_id,
				permission_key,
				effect,
				tenant_id,
				store_id,
				created_by_user_id,
				note,
				is_active
			)
			values (
				$1::uuid,
				$2,
				$3,
				nullif($4, '')::uuid,
				nullif($5, '')::uuid,
				nullif($6, '')::uuid,
				$7,
				$8
			)
			returning
				id::text,
				user_id::text,
				permission_key,
				effect,
				coalesce(tenant_id::text, ''),
				coalesce(store_id::text, ''),
				note,
				is_active;
		`,
			strings.TrimSpace(userID),
			override.PermissionKey,
			override.Effect,
			override.TenantID,
			override.StoreID,
			strings.TrimSpace(createdByUserID),
			override.Note,
			true,
		).Scan(
			&savedOverride.ID,
			&savedOverride.UserID,
			&savedOverride.PermissionKey,
			&savedOverride.Effect,
			&savedOverride.TenantID,
			&savedOverride.StoreID,
			&savedOverride.Note,
			&savedOverride.IsActive,
		); err != nil {
			return nil, err
		}

		saved = append(saved, savedOverride)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	sort.Slice(saved, func(left, right int) bool {
		return saved[left].PermissionKey < saved[right].PermissionKey
	})

	return saved, nil
}
