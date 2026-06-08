package stores

import (
	"context"
	"strings"

	"github.com/jackc/pgx/v5"
)

func deleteCoreStoreScopeTx(ctx context.Context, tx pgx.Tx, storeID string) error {
	_, err := tx.Exec(ctx, `
		with target_store as (
			select
				s.id::text as store_id,
				s.tenant_id::text as account_id
			from queue.stores s
			where s.id = $1::uuid
		),
		affected_settings as (
			select
				queue_settings.user_id,
				target_store.account_id,
				coalesce(
					to_jsonb(array_agg(configured.store_id order by configured.ordinal)
						filter (where configured.store_id <> target_store.store_id)),
					'[]'::jsonb
				) as remaining_store_ids
			from core.user_module_settings queue_settings
			join target_store on true
			join lateral jsonb_array_elements_text(
				coalesce(
					queue_settings.config #> array['storeIdsByAccount', target_store.account_id],
					'[]'::jsonb
				)
			) with ordinality configured(store_id, ordinal) on true
			where queue_settings.module_id = 'queue'
			group by queue_settings.user_id, target_store.account_id, target_store.store_id
			having bool_or(configured.store_id = target_store.store_id)
		)
		update core.user_module_settings queue_settings
		set
			config = jsonb_set(
				jsonb_set(
					queue_settings.config,
					array['storeIdsByAccount'],
					coalesce(queue_settings.config->'storeIdsByAccount', '{}'::jsonb),
					true
				),
				array['storeIdsByAccount', affected_settings.account_id],
				affected_settings.remaining_store_ids,
				true
			),
			updated_at = now()
		from affected_settings
		where queue_settings.user_id = affected_settings.user_id
			and queue_settings.module_id = 'queue';
	`, strings.TrimSpace(storeID))
	return err
}
