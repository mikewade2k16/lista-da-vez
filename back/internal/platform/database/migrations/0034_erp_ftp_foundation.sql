create extension if not exists pg_trgm;

create table if not exists erp_sync_runs (
	id uuid primary key default gen_random_uuid(),
	tenant_id uuid not null references tenants(id) on delete cascade,
	store_id uuid not null references stores(id) on delete cascade,
	store_code text not null,
	store_cnpj text,
	data_type text not null check (data_type in ('item', 'customer', 'employee', 'order', 'ordercanceled')),
	mode text not null,
	source_path text not null default '',
	status text not null check (status in ('running', 'succeeded', 'failed')),
	files_seen integer not null default 0,
	files_imported integer not null default 0,
	files_skipped integer not null default 0,
	rows_read integer not null default 0,
	raw_rows_imported integer not null default 0,
	error_message text,
	started_at timestamptz not null default now(),
	finished_at timestamptz,
	created_at timestamptz not null default now(),
	updated_at timestamptz not null default now()
);

create index if not exists erp_sync_runs_tenant_store_idx on erp_sync_runs (tenant_id, store_id, data_type, started_at desc);

create table if not exists erp_sync_files (
	id uuid primary key default gen_random_uuid(),
	run_id uuid not null references erp_sync_runs(id) on delete cascade,
	tenant_id uuid not null references tenants(id) on delete cascade,
	store_id uuid not null references stores(id) on delete cascade,
	store_code text not null,
	store_cnpj text,
	data_type text not null check (data_type in ('item', 'customer', 'employee', 'order', 'ordercanceled')),
	source_name text not null,
	source_path text not null default '',
	source_kind text not null,
	source_batch_date date,
	checksum_sha256 text not null,
	record_count integer not null default 0,
	status text not null default 'pending',
	imported_at timestamptz not null default now(),
	created_at timestamptz not null default now(),
	updated_at timestamptz not null default now(),
	unique (tenant_id, store_id, data_type, source_name, checksum_sha256)
);

create index if not exists erp_sync_files_tenant_store_idx on erp_sync_files (tenant_id, store_id, data_type, imported_at desc);

create table if not exists erp_item_raw (
	id uuid primary key default gen_random_uuid(),
	run_id uuid not null references erp_sync_runs(id) on delete cascade,
	file_id uuid not null references erp_sync_files(id) on delete cascade,
	tenant_id uuid not null references tenants(id) on delete cascade,
	store_id uuid not null references stores(id) on delete cascade,
	store_code text not null,
	store_cnpj text not null default '',
	source_file_name text not null,
	source_batch_date date not null,
	source_line_number integer not null,
	sku text not null,
	name text not null default '',
	description text not null default '',
	supplierreference text not null default '',
	brandname text not null default '',
	seasonname text not null default '',
	category1 text not null default '',
	category2 text not null default '',
	category3 text not null default '',
	size text not null default '',
	color text not null default '',
	unit text not null default '',
	price_raw text not null default '',
	price_cents bigint,
	identifier text not null default '',
	created_at_raw text not null default '',
	updated_at_raw text not null default '',
	created_at timestamptz,
	updated_at timestamptz,
	created_at_imported timestamptz not null default now()
);

create index if not exists erp_item_raw_tenant_store_idx on erp_item_raw (tenant_id, store_id, source_batch_date desc);
create index if not exists erp_item_raw_sku_idx on erp_item_raw (tenant_id, store_id, sku);

create table if not exists erp_customer_raw (
	id uuid primary key default gen_random_uuid(),
	run_id uuid not null references erp_sync_runs(id) on delete cascade,
	file_id uuid not null references erp_sync_files(id) on delete cascade,
	tenant_id uuid not null references tenants(id) on delete cascade,
	store_id uuid not null references stores(id) on delete cascade,
	store_code text not null,
	store_cnpj text not null default '',
	source_file_name text not null,
	source_batch_date date not null,
	source_line_number integer not null,
	name text not null default '',
	nickname text not null default '',
	cpf text not null default '',
	email text not null default '',
	phone text not null default '',
	mobile text not null default '',
	gender text not null default '',
	birthday_raw text not null default '',
	street text not null default '',
	number text not null default '',
	complement text not null default '',
	neighborhood text not null default '',
	city text not null default '',
	uf text not null default '',
	country text not null default '',
	zipcode text not null default '',
	employee_id text not null default '',
	registered_at_raw text not null default '',
	original_id text not null default '',
	identifier text not null default '',
	tags text not null default '',
	created_at_imported timestamptz not null default now()
);

create index if not exists erp_customer_raw_tenant_store_idx on erp_customer_raw (tenant_id, store_id, source_batch_date desc);

create table if not exists erp_employee_raw (
	id uuid primary key default gen_random_uuid(),
	run_id uuid not null references erp_sync_runs(id) on delete cascade,
	file_id uuid not null references erp_sync_files(id) on delete cascade,
	tenant_id uuid not null references tenants(id) on delete cascade,
	store_id uuid not null references stores(id) on delete cascade,
	store_code text not null,
	store_cnpj text not null default '',
	source_file_name text not null,
	source_batch_date date not null,
	source_line_number integer not null,
	name text not null default '',
	original_id text not null default '',
	street text not null default '',
	complement text not null default '',
	city text not null default '',
	uf text not null default '',
	zipcode text not null default '',
	is_active_raw text not null default '',
	created_at_imported timestamptz not null default now()
);

create index if not exists erp_employee_raw_tenant_store_idx on erp_employee_raw (tenant_id, store_id, source_batch_date desc);

create table if not exists erp_order_raw (
	id uuid primary key default gen_random_uuid(),
	run_id uuid not null references erp_sync_runs(id) on delete cascade,
	file_id uuid not null references erp_sync_files(id) on delete cascade,
	tenant_id uuid not null references tenants(id) on delete cascade,
	store_id uuid not null references stores(id) on delete cascade,
	store_code text not null,
	store_cnpj text not null default '',
	source_file_name text not null,
	source_batch_date date not null,
	source_line_number integer not null,
	order_id text not null default '',
	identifier text not null default '',
	customer_id text not null default '',
	order_date_raw text not null default '',
	order_date timestamptz,
	total_amount_raw text not null default '',
	total_amount_cents bigint,
	product_return_raw text not null default '',
	product_return_cents bigint,
	sku text not null default '',
	amount_raw text not null default '',
	amount_cents bigint,
	quantity_raw text not null default '',
	quantity integer,
	employee_id text not null default '',
	payment_type text not null default '',
	total_exclusion_raw text not null default '',
	total_exclusion_cents bigint,
	total_debit_raw text not null default '',
	total_debit_cents bigint,
	created_at_imported timestamptz not null default now()
);

create index if not exists erp_order_raw_tenant_store_idx on erp_order_raw (tenant_id, store_id, source_batch_date desc);

create table if not exists erp_order_canceled_raw (
	id uuid primary key default gen_random_uuid(),
	run_id uuid not null references erp_sync_runs(id) on delete cascade,
	file_id uuid not null references erp_sync_files(id) on delete cascade,
	tenant_id uuid not null references tenants(id) on delete cascade,
	store_id uuid not null references stores(id) on delete cascade,
	store_code text not null,
	store_cnpj text not null default '',
	source_file_name text not null,
	source_batch_date date not null,
	source_line_number integer not null,
	order_id text not null default '',
	identifier text not null default '',
	customer_id text not null default '',
	order_date_raw text not null default '',
	order_date timestamptz,
	total_amount_raw text not null default '',
	total_amount_cents bigint,
	product_return_raw text not null default '',
	product_return_cents bigint,
	sku text not null default '',
	amount_raw text not null default '',
	amount_cents bigint,
	quantity_raw text not null default '',
	quantity integer,
	employee_id text not null default '',
	payment_type text not null default '',
	total_exclusion_raw text not null default '',
	total_exclusion_cents bigint,
	total_debit_raw text not null default '',
	total_debit_cents bigint,
	created_at_imported timestamptz not null default now()
);

create index if not exists erp_order_canceled_raw_tenant_store_idx on erp_order_canceled_raw (tenant_id, store_id, source_batch_date desc);

create table if not exists erp_item_current (
	tenant_id uuid not null references tenants(id) on delete cascade,
	store_id uuid not null references stores(id) on delete cascade,
	store_code text not null,
	store_cnpj text not null default '',
	sku text not null,
	identifier text not null default '',
	name text not null default '',
	description text not null default '',
	supplierreference text not null default '',
	brandname text not null default '',
	seasonname text not null default '',
	category1 text not null default '',
	category2 text not null default '',
	category3 text not null default '',
	size text not null default '',
	color text not null default '',
	unit text not null default '',
	price_raw text not null default '',
	price_cents bigint,
	source_file_name text not null default '',
	source_batch_date date not null,
	source_line_number integer not null,
	source_created_at_raw text not null default '',
	source_updated_at_raw text not null default '',
	source_created_at timestamptz,
	source_updated_at timestamptz,
	run_id uuid not null references erp_sync_runs(id) on delete cascade,
	file_id uuid not null references erp_sync_files(id) on delete cascade,
	created_at timestamptz not null default now(),
	updated_at timestamptz not null default now(),
	primary key (tenant_id, store_id, sku)
);

create index if not exists erp_item_current_tenant_store_idx on erp_item_current (tenant_id, store_id, updated_at desc);
create index if not exists erp_item_current_identifier_idx on erp_item_current (tenant_id, store_id, identifier);
create index if not exists erp_item_current_search_trgm_idx on erp_item_current using gin ((coalesce(sku, '') || ' ' || coalesce(identifier, '') || ' ' || coalesce(name, '')) gin_trgm_ops);

create table if not exists erp_export_outbox (
	id uuid primary key default gen_random_uuid(),
	tenant_id uuid not null references tenants(id) on delete cascade,
	store_id uuid references stores(id) on delete set null,
	data_type text not null check (data_type in ('item', 'customer', 'employee', 'order', 'ordercanceled')),
	stream_key text not null,
	payload jsonb not null default '{}'::jsonb,
	status text not null default 'pending',
	attempt_count integer not null default 0,
	available_at timestamptz not null default now(),
	last_attempt_at timestamptz,
	created_at timestamptz not null default now(),
	updated_at timestamptz not null default now()
);

create index if not exists erp_export_outbox_status_idx on erp_export_outbox (status, available_at, created_at);