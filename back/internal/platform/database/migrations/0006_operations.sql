create table if not exists operation_queue_entries (
	store_id uuid not null references stores(id) on delete cascade,
	consultant_id uuid not null references consultants(id) on delete cascade,
	queue_joined_at bigint not null,
	sort_order integer not null default 0,
	created_at timestamptz not null default now(),
	updated_at timestamptz not null default now(),
	primary key (store_id, consultant_id)
);

create index if not exists operation_queue_entries_store_idx
	on operation_queue_entries (store_id, sort_order, queue_joined_at);

create table if not exists operation_active_services (
	store_id uuid not null references stores(id) on delete cascade,
	consultant_id uuid not null references consultants(id) on delete cascade,
	service_id text not null,
	service_started_at bigint not null,
	queue_joined_at bigint not null,
	queue_wait_ms bigint not null default 0,
	queue_position_at_start integer not null default 1,
	start_mode text not null check (start_mode in ('queue', 'queue-jump')),
	skipped_people_json jsonb not null default '[]'::jsonb,
	created_at timestamptz not null default now(),
	updated_at timestamptz not null default now(),
	primary key (store_id, consultant_id),
	unique (store_id, service_id)
);

create index if not exists operation_active_services_store_idx
	on operation_active_services (store_id, service_started_at);

create table if not exists operation_paused_consultants (
	store_id uuid not null references stores(id) on delete cascade,
	consultant_id uuid not null references consultants(id) on delete cascade,
	reason text not null,
	started_at bigint not null,
	created_at timestamptz not null default now(),
	updated_at timestamptz not null default now(),
	primary key (store_id, consultant_id)
);

create index if not exists operation_paused_consultants_store_idx
	on operation_paused_consultants (store_id, started_at);

create table if not exists operation_current_status (
	store_id uuid not null references stores(id) on delete cascade,
	consultant_id uuid not null references consultants(id) on delete cascade,
	status text not null check (status in ('available', 'queue', 'service', 'paused')),
	started_at bigint not null,
	created_at timestamptz not null default now(),
	updated_at timestamptz not null default now(),
	primary key (store_id, consultant_id)
);

create index if not exists operation_current_status_store_idx
	on operation_current_status (store_id, status, started_at);

create table if not exists operation_status_sessions (
	id uuid primary key default gen_random_uuid(),
	store_id uuid not null references stores(id) on delete cascade,
	consultant_id uuid not null references consultants(id) on delete cascade,
	status text not null check (status in ('available', 'queue', 'service', 'paused')),
	started_at bigint not null,
	ended_at bigint not null,
	duration_ms bigint not null default 0,
	created_at timestamptz not null default now()
);

create index if not exists operation_status_sessions_store_idx
	on operation_status_sessions (store_id, started_at);

create index if not exists operation_status_sessions_consultant_idx
	on operation_status_sessions (consultant_id, started_at);

create table if not exists operation_service_history (
	id uuid primary key default gen_random_uuid(),
	store_id uuid not null references stores(id) on delete cascade,
	service_id text not null,
	person_id uuid not null references consultants(id) on delete cascade,
	person_name text not null default '',
	started_at bigint not null,
	finished_at bigint not null,
	duration_ms bigint not null default 0,
	finish_outcome text not null check (finish_outcome in ('reserva', 'compra', 'nao-compra')),
	start_mode text not null check (start_mode in ('queue', 'queue-jump')),
	queue_position_at_start integer not null default 1,
	queue_wait_ms bigint not null default 0,
	skipped_people_json jsonb not null default '[]'::jsonb,
	skipped_count integer not null default 0,
	is_window_service boolean not null default false,
	is_gift boolean not null default false,
	product_seen text not null default '',
	product_closed text not null default '',
	product_details text not null default '',
	products_seen_json jsonb not null default '[]'::jsonb,
	products_closed_json jsonb not null default '[]'::jsonb,
	products_seen_none boolean not null default false,
	visit_reasons_not_informed boolean not null default false,
	customer_sources_not_informed boolean not null default false,
	customer_name text not null default '',
	customer_phone text not null default '',
	customer_email text not null default '',
	is_existing_customer boolean not null default false,
	visit_reasons_json jsonb not null default '[]'::jsonb,
	visit_reason_details_json jsonb not null default '{}'::jsonb,
	customer_sources_json jsonb not null default '[]'::jsonb,
	customer_source_details_json jsonb not null default '{}'::jsonb,
	loss_reasons_json jsonb not null default '[]'::jsonb,
	loss_reason_details_json jsonb not null default '{}'::jsonb,
	loss_reason_id text not null default '',
	loss_reason text not null default '',
	sale_amount numeric(14, 2) not null default 0,
	customer_profession text not null default '',
	queue_jump_reason text not null default '',
	notes text not null default '',
	campaign_matches_json jsonb not null default '[]'::jsonb,
	campaign_bonus_total numeric(14, 2) not null default 0,
	created_at timestamptz not null default now(),
	unique (store_id, service_id)
);

create index if not exists operation_service_history_store_idx
	on operation_service_history (store_id, started_at desc);

create index if not exists operation_service_history_person_idx
	on operation_service_history (person_id, started_at desc);
