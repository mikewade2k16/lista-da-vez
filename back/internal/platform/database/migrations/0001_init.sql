create extension if not exists pgcrypto;

create table if not exists users (
	id uuid primary key default gen_random_uuid(),
	email text not null,
	display_name text not null,
	password_hash text not null,
	is_active boolean not null default true,
	created_at timestamptz not null default now(),
	updated_at timestamptz not null default now()
);

create unique index if not exists users_email_lower_uidx on users (lower(email));

create table if not exists tenants (
	id uuid primary key default gen_random_uuid(),
	slug text not null,
	name text not null,
	is_active boolean not null default true,
	created_at timestamptz not null default now(),
	updated_at timestamptz not null default now()
);

create unique index if not exists tenants_slug_uidx on tenants (slug);

create table if not exists stores (
	id uuid primary key default gen_random_uuid(),
	tenant_id uuid not null references tenants(id) on delete cascade,
	code text not null,
	name text not null,
	city text not null default '',
	is_active boolean not null default true,
	created_at timestamptz not null default now(),
	updated_at timestamptz not null default now()
);

create unique index if not exists stores_tenant_code_uidx on stores (tenant_id, code);
create index if not exists stores_tenant_id_idx on stores (tenant_id);

create table if not exists user_platform_roles (
	user_id uuid primary key references users(id) on delete cascade,
	role text not null check (role in ('platform_admin')),
	created_at timestamptz not null default now()
);

create table if not exists user_tenant_roles (
	id uuid primary key default gen_random_uuid(),
	user_id uuid not null references users(id) on delete cascade,
	tenant_id uuid not null references tenants(id) on delete cascade,
	role text not null check (role in ('marketing', 'owner')),
	created_at timestamptz not null default now(),
	unique (user_id, tenant_id, role)
);

create index if not exists user_tenant_roles_user_id_idx on user_tenant_roles (user_id);
create index if not exists user_tenant_roles_tenant_id_idx on user_tenant_roles (tenant_id);

create table if not exists user_store_roles (
	id uuid primary key default gen_random_uuid(),
	user_id uuid not null references users(id) on delete cascade,
	store_id uuid not null references stores(id) on delete cascade,
	role text not null check (role in ('consultant', 'manager')),
	created_at timestamptz not null default now(),
	unique (user_id, store_id, role)
);

create index if not exists user_store_roles_user_id_idx on user_store_roles (user_id);
create index if not exists user_store_roles_store_id_idx on user_store_roles (store_id);
