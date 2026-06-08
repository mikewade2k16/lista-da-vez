-- Dev-only seed: provision the Perola site tracking source with a fixed
-- local secret so the painel Perola webhook can post into localhost.
-- This is skipped in production by migrator.go.

insert into site.webhook_sources (
	id,
	account_id,
	slug,
	name,
	entity_type,
	secret,
	is_active,
	created_at,
	updated_at
)
select
	'dddddddd-dddd-dddd-dddd-dddddddd0130'::uuid,
	a.id,
	'perola-site',
	'Perola Site Tracking',
	'tracking',
	'dev-perola-site-tracking-secret',
	true,
	now(),
	now()
from core.accounts a
where a.id = 'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa'::uuid
on conflict (slug) do update
set
	account_id = excluded.account_id,
	name = excluded.name,
	entity_type = excluded.entity_type,
	secret = excluded.secret,
	is_active = excluded.is_active,
	updated_at = now();