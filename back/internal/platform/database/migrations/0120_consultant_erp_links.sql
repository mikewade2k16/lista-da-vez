create table if not exists queue.consultant_erp_links (
	id uuid primary key default gen_random_uuid(),
	tenant_id uuid not null references tenants(id) on delete cascade,
	store_id uuid null references queue.stores(id) on delete set null,
	consultant_id uuid not null references queue.consultants(id) on delete cascade,
	erp_store_code text not null default '',
	erp_employee_id text not null,
	erp_employee_name text not null default '',
	note text not null default '',
	is_active boolean not null default true,
	created_by_user_id uuid null references users(id) on delete set null,
	updated_by_user_id uuid null references users(id) on delete set null,
	created_at timestamptz not null default now(),
	updated_at timestamptz not null default now(),
	constraint consultant_erp_links_employee_id_not_blank check (trim(erp_employee_id) <> '')
);

create unique index if not exists consultant_erp_links_active_employee_uidx
	on queue.consultant_erp_links (
		tenant_id,
		lower(trim(erp_store_code)),
		lower(trim(erp_employee_id))
	)
	where is_active = true;

create index if not exists consultant_erp_links_consultant_idx
	on queue.consultant_erp_links (consultant_id)
	where is_active = true;

create index if not exists consultant_erp_links_tenant_store_idx
	on queue.consultant_erp_links (tenant_id, store_id)
	where is_active = true;

create or replace view public.consultant_erp_links as
	select * from queue.consultant_erp_links;
