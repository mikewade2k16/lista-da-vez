package users

import (
	"context"
	"strings"

	"github.com/jackc/pgx/v5"

	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
)

func upsertCoreAssignmentsTx(ctx context.Context, tx pgx.Tx, user User) error {
	if _, err := tx.Exec(ctx, `
		update core.users
		set is_platform_admin = $2, updated_at = now()
		where id = $1::uuid;
	`, user.ID, user.Role == auth.RolePlatformAdmin); err != nil {
		return err
	}

	if user.Role == auth.RolePlatformAdmin {
		return deleteCoreQueueAssignmentsTx(ctx, tx, "", user.ID)
	}

	accountID := strings.TrimSpace(user.TenantID)
	if accountID == "" {
		return ErrValidation
	}

	if err := ensureCoreQueueRoleTx(ctx, tx, accountID, user.Role); err != nil {
		return err
	}

	if _, err := tx.Exec(ctx, `
		insert into core.account_users (account_id, user_id, is_active, joined_at)
		values ($1::uuid, $2::uuid, true, now())
		on conflict (account_id, user_id) do update
		set is_active = true;
	`, accountID, user.ID); err != nil {
		return err
	}

	if err := deleteCoreQueueAssignmentsTx(ctx, tx, accountID, user.ID); err != nil {
		return err
	}

	coreRoleCode := auth.QueueCoreRoleCodeForCoarse(user.Role)
	if coreRoleCode == "" {
		return ErrValidation
	}

	if _, err := tx.Exec(ctx, `
		insert into core.user_role_assignments (account_id, user_id, role_id)
		select $1::uuid, $2::uuid, r.id
		from core.roles r
		where r.account_id = $1::uuid
			and r.code = $3
		on conflict (account_id, user_id, role_id) do nothing;
	`, accountID, user.ID, coreRoleCode); err != nil {
		return err
	}

	return upsertQueueUserSettingsTx(ctx, tx, accountID, user)
}

func ensureCoreQueueRoleTx(ctx context.Context, tx pgx.Tx, accountID string, role auth.Role) error {
	coreRoleCode := auth.QueueCoreRoleCodeForCoarse(role)
	templateID := auth.QueueCoreRoleTemplateForCoarse(role)
	if coreRoleCode == "" || templateID == "" {
		return ErrValidation
	}

	if _, err := tx.Exec(ctx, `
		insert into core.roles (
			account_id,
			cloned_from_template_id,
			code,
			label,
			description,
			is_locked
		)
		select
			$1::uuid,
			rt.id,
			$2,
			$3,
			$4,
			rt.is_locked
		from core.role_templates rt
		where rt.id = $5
		on conflict (account_id, code) do nothing;
	`, accountID, coreRoleCode, coreRoleLabel(role), coreRoleDescription(role), templateID); err != nil {
		return err
	}

	_, err := tx.Exec(ctx, `
		insert into core.role_permissions (role_id, permission_key)
		select r.id, rtp.permission_key
		from core.roles r
		join core.role_template_permissions rtp on rtp.role_template_id = $3
		where r.account_id = $1::uuid
			and r.code = $2
		on conflict do nothing;
	`, accountID, coreRoleCode, templateID)
	return err
}

func deleteCoreQueueAssignmentsTx(ctx context.Context, tx pgx.Tx, accountID string, userID string) error {
	_, err := tx.Exec(ctx, `
		delete from core.user_role_assignments ura
		using core.roles r
		where ura.role_id = r.id
			and ura.user_id = $1::uuid
			and lower(r.code) = any($2::text[])
			and (nullif($3, '') is null or ura.account_id = $3::uuid);
	`, strings.TrimSpace(userID), auth.QueueCompatibilityCoreRoleCodes(), strings.TrimSpace(accountID))
	return err
}

func upsertQueueUserSettingsTx(ctx context.Context, tx pgx.Tx, accountID string, user User) error {
	storeIDs := []string{}
	switch user.Role {
	case auth.RoleManager, auth.RoleConsultant, auth.RoleStoreTerminal:
		storeIDs = cloneStringSlice(user.StoreIDs)
	}

	_, err := tx.Exec(ctx, `
		insert into core.user_module_settings (user_id, module_id, config, created_at, updated_at)
		values (
			$1::uuid,
			'queue',
			jsonb_build_object(
				'storeIdsByAccount',
				jsonb_build_object($2::text, to_jsonb($3::text[]))
			),
			now(),
			now()
		)
		on conflict (user_id, module_id) do update
		set
			config = jsonb_set(
				jsonb_set(
					core.user_module_settings.config,
					array['storeIdsByAccount'],
					coalesce(core.user_module_settings.config->'storeIdsByAccount', '{}'::jsonb),
					true
				),
				array['storeIdsByAccount', $2::text],
				to_jsonb($3::text[]),
				true
			),
			updated_at = now();
	`, user.ID, accountID, storeIDs)
	return err
}

func coreRoleLabel(role auth.Role) string {
	switch role {
	case auth.RoleOwner:
		return "Proprietario da Fila"
	case auth.RoleDirector:
		return "Diretoria da Fila"
	case auth.RoleMarketing:
		return "Marketing da Fila"
	case auth.RoleManager:
		return "Gerente da Fila"
	case auth.RoleConsultant:
		return "Consultor"
	case auth.RoleStoreTerminal:
		return "Terminal de Loja"
	default:
		return string(role)
	}
}

func coreRoleDescription(role auth.Role) string {
	return "Compatibilidade U3 para role coarse " + string(role) + "."
}
