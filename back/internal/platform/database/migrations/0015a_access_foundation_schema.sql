alter table if exists users
	add column if not exists employee_code text not null default '';

alter table if exists users
	add column if not exists job_title text not null default '';

create unique index if not exists users_employee_code_uidx
	on users (employee_code)
	where trim(employee_code) <> '';

alter table if exists user_tenant_roles
	drop constraint if exists user_tenant_roles_role_check;

alter table if exists user_tenant_roles
	add constraint user_tenant_roles_role_check
	check (role in ('marketing', 'director', 'owner'));

create table if not exists access_permissions (
	key text primary key,
	scope text not null check (scope in ('store', 'tenant', 'platform')),
	description text not null,
	created_at timestamptz not null default now()
);

create table if not exists access_role_permissions (
	role text not null check (role in ('consultant', 'store_terminal', 'manager', 'marketing', 'director', 'owner', 'platform_admin')),
	permission_key text not null references access_permissions(key) on delete cascade,
	created_at timestamptz not null default now(),
	primary key (role, permission_key)
);

create table if not exists user_access_overrides (
	id uuid primary key default gen_random_uuid(),
	user_id uuid not null references users(id) on delete cascade,
	permission_key text not null references access_permissions(key) on delete cascade,
	effect text not null check (effect in ('allow', 'deny')),
	tenant_id uuid references tenants(id) on delete cascade,
	store_id uuid references stores(id) on delete cascade,
	created_by_user_id uuid references users(id) on delete set null,
	note text not null default '',
	is_active boolean not null default true,
	created_at timestamptz not null default now(),
	updated_at timestamptz not null default now()
);

create index if not exists user_access_overrides_user_idx
	on user_access_overrides (user_id, is_active, permission_key);

create index if not exists user_access_overrides_tenant_idx
	on user_access_overrides (tenant_id, permission_key)
	where tenant_id is not null;

create index if not exists user_access_overrides_store_idx
	on user_access_overrides (store_id, permission_key)
	where store_id is not null;

create table if not exists store_terminals (
	id uuid primary key default gen_random_uuid(),
	tenant_id uuid not null references tenants(id) on delete cascade,
	store_id uuid not null unique references stores(id) on delete cascade,
	user_id uuid not null unique references users(id) on delete cascade,
	code text not null,
	device_label text not null,
	device_slug text not null,
	access_mode text not null default 'operations_primary' check (access_mode in ('operations_primary', 'operations_readonly', 'kiosk')),
	is_active boolean not null default true,
	last_seen_at timestamptz,
	created_at timestamptz not null default now(),
	updated_at timestamptz not null default now(),
	unique (tenant_id, code),
	unique (tenant_id, device_slug)
);

insert into access_permissions (key, scope, description)
values
	('operations.read.store', 'store', 'Leitura da fila e dos atendimentos da propria loja.'),
	('operations.read.cross_store', 'tenant', 'Leitura integrada da operacao de todas as lojas acessiveis.'),
	('operations.mutate.store', 'store', 'Comandos operacionais de fila, pausa e encerramento na loja.'),
	('realtime.operations.connect', 'store', 'Conexao ao websocket operacional por loja.'),
	('realtime.context.connect', 'tenant', 'Conexao ao websocket administrativo de contexto do tenant.'),
	('reports.read.tenant', 'tenant', 'Leitura consolidada de relatorios do tenant.'),
	('analytics.read.tenant', 'tenant', 'Leitura consolidada de analytics do tenant.'),
	('campaigns.manage.tenant', 'tenant', 'Gestao de campanhas no tenant.'),
	('settings.manage.tenant', 'tenant', 'Gestao de configuracoes operacionais do tenant.'),
	('stores.manage.tenant', 'tenant', 'Gestao de lojas do tenant.'),
	('users.manage.tenant', 'tenant', 'Gestao de usuarios do tenant.'),
	('platform.manage', 'platform', 'Gestao interna cross-tenant da plataforma.')
on conflict (key) do update
set
	scope = excluded.scope,
	description = excluded.description;

insert into access_role_permissions (role, permission_key)
values
	('consultant', 'operations.read.store'),
	('consultant', 'operations.mutate.store'),
	('consultant', 'realtime.operations.connect'),
	('store_terminal', 'operations.read.store'),
	('store_terminal', 'realtime.operations.connect'),
	('manager', 'operations.read.store'),
	('manager', 'operations.mutate.store'),
	('manager', 'realtime.operations.connect'),
	('marketing', 'operations.read.store'),
	('marketing', 'operations.read.cross_store'),
	('marketing', 'realtime.operations.connect'),
	('marketing', 'realtime.context.connect'),
	('marketing', 'reports.read.tenant'),
	('marketing', 'analytics.read.tenant'),
	('marketing', 'campaigns.manage.tenant'),
	('director', 'operations.read.store'),
	('director', 'operations.read.cross_store'),
	('director', 'realtime.operations.connect'),
	('director', 'realtime.context.connect'),
	('director', 'reports.read.tenant'),
	('director', 'analytics.read.tenant'),
	('owner', 'operations.read.store'),
	('owner', 'operations.read.cross_store'),
	('owner', 'operations.mutate.store'),
	('owner', 'realtime.operations.connect'),
	('owner', 'realtime.context.connect'),
	('owner', 'reports.read.tenant'),
	('owner', 'analytics.read.tenant'),
	('owner', 'campaigns.manage.tenant'),
	('owner', 'settings.manage.tenant'),
	('owner', 'stores.manage.tenant'),
	('owner', 'users.manage.tenant'),
	('platform_admin', 'operations.read.store'),
	('platform_admin', 'operations.read.cross_store'),
	('platform_admin', 'operations.mutate.store'),
	('platform_admin', 'realtime.operations.connect'),
	('platform_admin', 'realtime.context.connect'),
	('platform_admin', 'reports.read.tenant'),
	('platform_admin', 'analytics.read.tenant'),
	('platform_admin', 'campaigns.manage.tenant'),
	('platform_admin', 'settings.manage.tenant'),
	('platform_admin', 'stores.manage.tenant'),
	('platform_admin', 'users.manage.tenant'),
	('platform_admin', 'platform.manage')
on conflict (role, permission_key) do nothing;