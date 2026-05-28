-- Add max_concurrent_services_per_consultant to the tenant-wide settings source of truth.
-- Store-level rows remain as legacy fallback, but the app now persists settings per tenant.

alter table tenant_operation_settings
add column if not exists max_concurrent_services_per_consultant integer not null default 1;

with primary_store as (
	select distinct on (s.tenant_id)
		s.tenant_id,
		s.id as store_id
	from stores s
	join store_operation_settings sos on sos.store_id = s.id
	order by s.tenant_id, s.created_at asc, s.id asc
)
update tenant_operation_settings tos
set max_concurrent_services_per_consultant = greatest(
	1,
	least(
		tos.max_concurrent_services,
		coalesce(sos.max_concurrent_services_per_consultant, 1)
	)
)
from primary_store ps
join store_operation_settings sos on sos.store_id = ps.store_id
where tos.tenant_id = ps.tenant_id;

update tenant_operation_settings
set max_concurrent_services_per_consultant = greatest(
	1,
	least(max_concurrent_services, coalesce(max_concurrent_services_per_consultant, 1))
);

do $$
begin
	if not exists (
		select 1
		from pg_constraint
		where conname = 'tenant_max_concurrent_per_consultant_vs_store_check'
	) then
		alter table tenant_operation_settings
		add constraint tenant_max_concurrent_per_consultant_vs_store_check
			check (
				max_concurrent_services_per_consultant >= 1
				and max_concurrent_services_per_consultant <= max_concurrent_services
			);
	end if;
end $$;