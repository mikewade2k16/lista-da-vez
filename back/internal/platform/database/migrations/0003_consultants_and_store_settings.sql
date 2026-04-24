create table if not exists consultants (
	id uuid primary key default gen_random_uuid(),
	tenant_id uuid not null references tenants(id) on delete cascade,
	store_id uuid not null references stores(id) on delete cascade,
	name text not null,
	role_label text not null default 'Atendimento',
	initials text not null,
	color text not null,
	monthly_goal numeric(14, 2) not null default 0,
	commission_rate numeric(8, 4) not null default 0,
	conversion_goal numeric(6, 2) not null default 0,
	avg_ticket_goal numeric(14, 2) not null default 0,
	pa_goal numeric(8, 2) not null default 0,
	is_active boolean not null default true,
	created_at timestamptz not null default now(),
	updated_at timestamptz not null default now()
);

create index if not exists consultants_store_id_idx on consultants (store_id);
create index if not exists consultants_tenant_id_idx on consultants (tenant_id);
create unique index if not exists consultants_store_name_active_uidx
	on consultants (store_id, lower(name))
	where is_active = true;

create table if not exists store_settings (
	store_id uuid primary key references stores(id) on delete cascade,
	selected_operation_template_id text not null default 'joalheria-padrao',
	settings_json jsonb not null default '{}'::jsonb,
	modal_config_json jsonb not null default '{}'::jsonb,
	visit_reason_options_json jsonb not null default '[]'::jsonb,
	customer_source_options_json jsonb not null default '[]'::jsonb,
	queue_jump_reason_options_json jsonb not null default '[]'::jsonb,
	loss_reason_options_json jsonb not null default '[]'::jsonb,
	profession_options_json jsonb not null default '[]'::jsonb,
	product_catalog_json jsonb not null default '[]'::jsonb,
	created_at timestamptz not null default now(),
	updated_at timestamptz not null default now()
);
