-- Add max_concurrent_services_per_consultant setting to store_operation_settings
-- Default 1 = one consultant can handle max 1 concurrent service (backward compatible, current behavior)
-- Can be increased per store to enable parallel services (2, 3, etc)

alter table store_operation_settings
add column if not exists max_concurrent_services_per_consultant integer not null default 1;

-- Add constraint: per_consultant limit must be <= store limit
alter table store_operation_settings
add constraint max_concurrent_per_consultant_vs_store_check
	check (max_concurrent_services_per_consultant >= 1 and max_concurrent_services_per_consultant <= max_concurrent_services);
