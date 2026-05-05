-- Production-safe ERP root store bootstrap.
--
-- The dev seed that originally created store 184 is intentionally skipped in
-- production. The ERP MVP, however, is anchored on store code 184, so existing
-- production tenants need this root store to resolve /v1/erp/* requests.

insert into stores (
	tenant_id,
	code,
	name,
	city,
	is_active
)
select
	t.id,
	'184',
	'Loja 184',
	'',
	true
from tenants t
where t.is_active = true
on conflict (tenant_id, code) do update
set
	name = excluded.name,
	city = excluded.city,
	is_active = true,
	updated_at = now();
