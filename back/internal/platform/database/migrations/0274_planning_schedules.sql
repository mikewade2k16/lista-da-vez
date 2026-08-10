-- Planejamento autoritativo por loja: configuracao permanente e escala semanal.

create table if not exists queue.planning_store_configs (
	id uuid primary key default gen_random_uuid(),
	tenant_id uuid not null references core.accounts(id) on delete cascade,
	store_id uuid not null references queue.stores(id) on delete cascade,
	configuration jsonb not null default '{}'::jsonb,
	version bigint not null default 1,
	created_by_user_id uuid null references core.users(id) on delete set null,
	updated_by_user_id uuid null references core.users(id) on delete set null,
	created_at timestamptz not null default now(),
	updated_at timestamptz not null default now(),
	constraint planning_store_configs_configuration_object_check
		check (jsonb_typeof(configuration) = 'object'),
	unique (tenant_id, store_id)
);

create index if not exists planning_store_configs_tenant_store_idx
	on queue.planning_store_configs (tenant_id, store_id);

create table if not exists queue.planning_schedules (
	id uuid primary key default gen_random_uuid(),
	tenant_id uuid not null references core.accounts(id) on delete cascade,
	store_id uuid not null references queue.stores(id) on delete cascade,
	week_start date not null,
	status text not null default 'saved',
	shifts jsonb not null default '[]'::jsonb,
	version bigint not null default 1,
	created_by_user_id uuid null references core.users(id) on delete set null,
	updated_by_user_id uuid null references core.users(id) on delete set null,
	created_at timestamptz not null default now(),
	updated_at timestamptz not null default now(),
	constraint planning_schedules_status_check check (status in ('saved', 'published')),
	constraint planning_schedules_shifts_array_check check (jsonb_typeof(shifts) = 'array'),
	unique (tenant_id, store_id, week_start)
);

create index if not exists planning_schedules_tenant_store_week_idx
	on queue.planning_schedules (tenant_id, store_id, week_start desc);
