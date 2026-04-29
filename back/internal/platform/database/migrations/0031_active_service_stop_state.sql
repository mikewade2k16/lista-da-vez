alter table operation_active_services
	add column if not exists stopped_at bigint not null default 0,
	add column if not exists stop_reason text not null default '';
