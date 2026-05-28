create table if not exists tenant_operational_alert_rules (
	tenant_id uuid primary key references tenants(id) on delete cascade,
	long_open_service_minutes integer not null default 25 check (long_open_service_minutes > 0),
	idle_store_minutes integer not null default 20 check (idle_store_minutes > 0),
	after_closing_grace_minutes integer not null default 15 check (after_closing_grace_minutes >= 0),
	notify_dashboard boolean not null default true,
	notify_operation_context boolean not null default true,
	notify_external boolean not null default false,
	updated_by uuid references users(id) on delete set null,
	updated_at timestamptz not null default now()
);

create table if not exists alert_instances (
	id uuid primary key default gen_random_uuid(),
	tenant_id uuid not null references tenants(id) on delete cascade,
	store_id uuid not null references stores(id) on delete cascade,
	service_id text not null default '',
	consultant_id uuid references consultants(id) on delete set null,
	type text not null,
	category text not null,
	severity text not null,
	status text not null check (status in ('active', 'acknowledged', 'resolved', 'closed_by_admin')),
	source_module text not null default 'operations',
	dedupe_key text not null,
	headline text not null,
	body text not null default '',
	metadata jsonb not null default '{}'::jsonb,
	opened_at timestamptz not null,
	last_triggered_at timestamptz not null,
	acknowledged_at timestamptz null,
	resolved_at timestamptz null,
	created_at timestamptz not null default now(),
	updated_at timestamptz not null default now()
);

create unique index if not exists alert_instances_open_dedupe_key_idx
	on alert_instances (dedupe_key)
	where status in ('active', 'acknowledged');

create index if not exists alert_instances_tenant_status_idx
	on alert_instances (tenant_id, status, last_triggered_at desc);

create index if not exists alert_instances_store_status_idx
	on alert_instances (store_id, status, last_triggered_at desc);

create index if not exists alert_instances_service_type_idx
	on alert_instances (service_id, type);

create table if not exists alert_actions (
	id uuid primary key default gen_random_uuid(),
	alert_id uuid not null references alert_instances(id) on delete cascade,
	tenant_id uuid not null references tenants(id) on delete cascade,
	store_id uuid not null references stores(id) on delete cascade,
	action text not null,
	actor_user_id uuid references users(id) on delete set null,
	actor_name text not null default '',
	note text not null default '',
	metadata jsonb not null default '{}'::jsonb,
	created_at timestamptz not null default now()
);

create index if not exists alert_actions_alert_id_idx on alert_actions (alert_id, created_at desc);
create index if not exists alert_actions_tenant_id_idx on alert_actions (tenant_id, created_at desc);