package consultants

import (
	"context"
	"strings"

	"github.com/jackc/pgx/v5"
)

// syncConsultantCoreScopeTx grava em core.* o vinculo consultor->user->loja, no
// lugar do legado user_store_roles (U4b). Garante membership, o role coarse
// queue.consultant (clonado do template) e o escopo de loja em
// core.user_module_settings(module_id='queue'). No-op se nao houver user/tenant.
func syncConsultantCoreScopeTx(ctx context.Context, tx pgx.Tx, accountID string, userID string, storeID string) error {
	accountID = strings.TrimSpace(accountID)
	userID = strings.TrimSpace(userID)
	if accountID == "" || userID == "" {
		return nil
	}

	if _, err := tx.Exec(ctx, `
		insert into core.account_users (account_id, user_id, is_active, joined_at)
		values ($1::uuid, $2::uuid, true, now())
		on conflict (account_id, user_id) do update set is_active = true;
	`, accountID, userID); err != nil {
		return err
	}

	if _, err := tx.Exec(ctx, `
		insert into core.roles (account_id, cloned_from_template_id, code, label, description, is_locked)
		select $1::uuid, rt.id, 'queue.consultant', rt.label, rt.description, rt.is_locked
		from core.role_templates rt
		where rt.id = 'queue.consultant'
		on conflict (account_id, code) do nothing;
	`, accountID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `
		insert into core.role_permissions (role_id, permission_key)
		select r.id, rtp.permission_key
		from core.roles r
		join core.role_template_permissions rtp on rtp.role_template_id = 'queue.consultant'
		where r.account_id = $1::uuid and r.code = 'queue.consultant'
		on conflict do nothing;
	`, accountID); err != nil {
		return err
	}

	if _, err := tx.Exec(ctx, `
		insert into core.user_role_assignments (account_id, user_id, role_id)
		select $1::uuid, $2::uuid, r.id
		from core.roles r
		where r.account_id = $1::uuid and r.code = 'queue.consultant'
		on conflict (account_id, user_id, role_id) do nothing;
	`, accountID, userID); err != nil {
		return err
	}

	storeIDs := []string{}
	if s := strings.TrimSpace(storeID); s != "" {
		storeIDs = []string{s}
	}
	_, err := tx.Exec(ctx, `
		insert into core.user_module_settings (user_id, module_id, config, created_at, updated_at)
		values (
			$1::uuid,
			'queue',
			jsonb_build_object('storeIdsByAccount', jsonb_build_object($2::text, to_jsonb($3::text[]))),
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
	`, userID, accountID, storeIDs)
	return err
}
