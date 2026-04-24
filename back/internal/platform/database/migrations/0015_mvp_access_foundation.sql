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

update stores
set
	code = 'RIO',
	name = 'Perola Riomar',
	city = 'Aracaju',
	is_active = true,
	updated_at = now()
where id = 'bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbb001';

update stores
set
	code = 'JAR',
	name = 'Perola Jardins',
	city = 'Aracaju',
	is_active = true,
	updated_at = now()
where id = 'bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbb002';

insert into stores (id, tenant_id, code, name, city, is_active)
values
	('bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbb003', 'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa', 'GAR', 'Perola Garcia', 'Aracaju', true),
	('bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbb004', 'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa', 'TRE', 'Perola Treze', 'Aracaju', true)
on conflict (id) do update
set
	tenant_id = excluded.tenant_id,
	code = excluded.code,
	name = excluded.name,
	city = excluded.city,
	is_active = excluded.is_active,
	updated_at = now();

insert into store_operation_settings (store_id)
values
	('bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbb001'),
	('bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbb002'),
	('bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbb003'),
	('bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbb004')
on conflict (store_id) do nothing;

create temp table tmp_old_consultant_users on commit drop as
select distinct c.user_id
from consultants c
where c.tenant_id = 'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa'::uuid
	and c.user_id is not null;

delete from consultants
where tenant_id = 'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa'::uuid;

delete from users
where id in (select user_id from tmp_old_consultant_users);

create temp table tmp_seed_users (
	user_id uuid not null,
	email text not null,
	display_name text not null,
	password_plain text not null,
	must_change_password boolean not null,
	is_active boolean not null,
	role text not null,
	tenant_id uuid,
	store_id uuid,
	employee_code text not null,
	job_title text not null
) on commit drop;

insert into tmp_seed_users (
	user_id,
	email,
	display_name,
	password_plain,
	must_change_password,
	is_active,
	role,
	tenant_id,
	store_id,
	employee_code,
	job_title
)
values
	('cccccccc-cccc-cccc-cccc-ccccccccc001', 'betaniaconceicao681@gmail.com', 'Maria Betania da Conceicao', 'Mvp@2026!', true, true, 'manager', 'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa', 'bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbb003', '204', 'Gerente de Loja'),
	('cccccccc-cccc-cccc-cccc-ccccccccc002', 'lane.olivieravcxz@gmail.com', 'Adelane Sousa Oliveira', 'Mvp@2026!', true, true, 'manager', 'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa', 'bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbb001', '206', 'Gerente de Loja'),
	('cccccccc-cccc-cccc-cccc-ccccccccc003', 'tonyw.right@outlook.com', 'Tony Prado', 'Mvp@2026!', true, true, 'marketing', 'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa', null, '', 'Gerente de Marketing'),
	('cccccccc-cccc-cccc-cccc-ccccccccc004', 'days.matos@gmail.com', 'Dayanne Barbosa de Souza Matos', 'Mvp@2026!', true, true, 'director', 'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa', null, '301', 'Diretoria'),
	('cccccccc-cccc-cccc-cccc-ccccccccc005', 'mikewade2k16@gmail.com', 'Mike Wade', 'Mvp@2026!', false, true, 'platform_admin', null, null, '', 'Developer da Plataforma'),
	('eeeeeeee-eeee-eeee-eeee-eeeeeeeee001', 'talia.sts10@hotmail.com', 'Barbara Talia dos Santos Morais', 'Mvp@2026!', true, true, 'manager', 'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa', 'bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbb004', '155', 'Gerente de Loja'),
	('eeeeeeee-eeee-eeee-eeee-eeeeeeeee002', 'alexsandrapaz@gmail.com.br', 'Alexsandra Paz Ferreira', 'Mvp@2026!', true, true, 'manager', 'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa', 'bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbb002', '227', 'Gerente de Loja'),
	('ffffffff-ffff-ffff-ffff-ffffffff0001', 'terminal.riomar@acesso.omni.local', 'Terminal Perola Riomar', 'Terminal@2026!', false, true, 'store_terminal', 'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa', 'bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbb001', '', 'Terminal da Loja'),
	('ffffffff-ffff-ffff-ffff-ffffffff0002', 'terminal.jardins@acesso.omni.local', 'Terminal Perola Jardins', 'Terminal@2026!', false, true, 'store_terminal', 'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa', 'bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbb002', '', 'Terminal da Loja'),
	('ffffffff-ffff-ffff-ffff-ffffffff0003', 'terminal.garcia@acesso.omni.local', 'Terminal Perola Garcia', 'Terminal@2026!', false, true, 'store_terminal', 'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa', 'bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbb003', '', 'Terminal da Loja'),
	('ffffffff-ffff-ffff-ffff-ffffffff0004', 'terminal.treze@acesso.omni.local', 'Terminal Perola Treze', 'Terminal@2026!', false, true, 'store_terminal', 'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa', 'bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbb004', '', 'Terminal da Loja'),
	('11111111-1111-1111-1111-111111111001', 'roseli.a.paixao@gmail.com', 'Roseli de Andrade Paixao', 'Mvp@2026!', true, true, 'consultant', 'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa', 'bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbb003', '259', 'Consultor de Atendimento'),
	('11111111-1111-1111-1111-111111111002', 'diancampos638@gmail.com', 'Diana Nicory Gomes', 'Mvp@2026!', true, true, 'consultant', 'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa', 'bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbb003', '321', 'Consultor de Atendimento'),
	('11111111-1111-1111-1111-111111111003', 'caroline17silva@gmail.com', 'Caroline Aragao Silva', 'Mvp@2026!', true, true, 'consultant', 'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa', 'bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbb003', '329', 'Consultor de Atendimento'),
	('11111111-1111-1111-1111-111111111004', 'nielaoliveira@hotmail.com', 'Daniella de Morais Oliveira', 'Mvp@2026!', true, true, 'consultant', 'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa', 'bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbb001', '183', 'Consultor de Atendimento'),
	('11111111-1111-1111-1111-111111111005', 'daysepaiva.sp@hotmail.com', 'Dayse Paiva', 'Mvp@2026!', true, true, 'consultant', 'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa', 'bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbb001', '317', 'Consultor de Atendimento'),
	('11111111-1111-1111-1111-111111111006', 'rafialmengo01@gmail.com', 'Iris Rafaela da Silva', 'Mvp@2026!', true, true, 'consultant', 'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa', 'bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbb001', '333', 'Consultor de Atendimento'),
	('11111111-1111-1111-1111-111111111007', 'ray.tsaraujo@gmail.com', 'Rayane Tavares Santos Araujo', 'Mvp@2026!', true, true, 'consultant', 'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa', 'bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbb001', '251', 'Consultor de Atendimento'),
	('11111111-1111-1111-1111-111111111008', 'hitanabatista1@gmail.com', 'Hitana Batista dos Santos', 'Mvp@2026!', true, true, 'consultant', 'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa', 'bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbb001', '215', 'Consultor de Atendimento'),
	('11111111-1111-1111-1111-111111111009', 'nutrilarad@gmail.com', 'Lara Dantas Souza', 'Mvp@2026!', true, true, 'consultant', 'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa', 'bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbb001', '289', 'Consultor de Atendimento'),
	('11111111-1111-1111-1111-111111111010', 'fabiomenezes80@hotmail.com', 'Fabio dos Santos Menezes', 'Mvp@2026!', true, true, 'consultant', 'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa', 'bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbb004', '56', 'Consultor de Atendimento'),
	('11111111-1111-1111-1111-111111111011', 'daianecaroline340@gmail.com', 'Daiane Caroline dos Santos', 'Mvp@2026!', true, true, 'consultant', 'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa', 'bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbb004', '281', 'Consultor de Atendimento'),
	('11111111-1111-1111-1111-111111111012', 'ritadamaris1@gmail.com', 'Rita Damaris Melo da Silva', 'Mvp@2026!', true, true, 'consultant', 'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa', 'bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbb004', '312', 'Consultor de Atendimento'),
	('11111111-1111-1111-1111-111111111013', 'tauvaniyassemin@gmail.com', 'Tauvani Missielly Oliveira', 'Mvp@2026!', true, true, 'consultant', 'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa', 'bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbb002', '268', 'Consultor de Atendimento'),
	('11111111-1111-1111-1111-111111111014', 'everlandalves38@gmail.com', 'Everland Alves dos Santos', 'Mvp@2026!', true, true, 'consultant', 'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa', 'bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbb002', '36', 'Consultor de Atendimento'),
	('11111111-1111-1111-1111-111111111015', 'fabianarafaellaviana2@gmail.com', 'Fabiana Rafaella Viana Santos', 'Mvp@2026!', true, true, 'consultant', 'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa', 'bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbb002', '330', 'Consultor de Atendimento'),
	('11111111-1111-1111-1111-111111111016', 'acilenejeronimo1@hotmail.com', 'Acilene dos Santos', 'Mvp@2026!', true, true, 'consultant', 'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa', 'bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbb002', '334', 'Consultor de Atendimento'),
	('11111111-1111-1111-1111-111111111017', 'gardenia.lobo@hotmail.com', 'Gardenia Lobo do Nascimento', 'Mvp@2026!', true, true, 'consultant', 'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa', 'bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbb002', '318', 'Consultor de Atendimento'),
	('11111111-1111-1111-1111-111111111018', 'mirelamirelasilvarodrigues@gmail.com', 'Mirela da Silva Rodrigues', 'Mvp@2026!', true, true, 'consultant', 'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa', 'bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbb002', '315', 'Consultor de Atendimento');

insert into users (
	id,
	email,
	display_name,
	password_hash,
	must_change_password,
	is_active,
	employee_code,
	job_title
)
select
	user_id,
	lower(email),
	display_name,
	crypt(password_plain, gen_salt('bf', 10)),
	must_change_password,
	is_active,
	employee_code,
	job_title
from tmp_seed_users
on conflict (id) do update
set
	email = excluded.email,
	display_name = excluded.display_name,
	password_hash = excluded.password_hash,
	must_change_password = excluded.must_change_password,
	is_active = excluded.is_active,
	employee_code = excluded.employee_code,
	job_title = excluded.job_title,
	updated_at = now();

delete from user_platform_roles
where user_id in (select user_id from tmp_seed_users);

delete from user_tenant_roles
where user_id in (select user_id from tmp_seed_users);

delete from user_store_roles
where user_id in (select user_id from tmp_seed_users);

insert into user_platform_roles (user_id, role)
select user_id, role
from tmp_seed_users
where role = 'platform_admin'
on conflict (user_id) do update
set role = excluded.role;

insert into user_tenant_roles (user_id, tenant_id, role)
select user_id, tenant_id, role
from tmp_seed_users
where role in ('marketing', 'director', 'owner')
on conflict (user_id, tenant_id, role) do nothing;

insert into user_store_roles (user_id, store_id, role)
select user_id, store_id, role
from tmp_seed_users
where role in ('consultant', 'manager', 'store_terminal')
on conflict (user_id, store_id, role) do nothing;

create temp table tmp_seed_consultants (
	consultant_id uuid not null,
	user_id uuid not null,
	store_id uuid not null,
	name text not null,
	initials text not null,
	color text not null
) on commit drop;

insert into tmp_seed_consultants (consultant_id, user_id, store_id, name, initials, color)
values
	('dddddddd-dddd-dddd-dddd-ddddddddd101', '11111111-1111-1111-1111-111111111001', 'bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbb003', 'Roseli de Andrade Paixao', 'RO', '#168aad'),
	('dddddddd-dddd-dddd-dddd-ddddddddd102', '11111111-1111-1111-1111-111111111002', 'bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbb003', 'Diana Nicory Gomes', 'DI', '#7a6ff0'),
	('dddddddd-dddd-dddd-dddd-ddddddddd103', '11111111-1111-1111-1111-111111111003', 'bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbb003', 'Caroline Aragao Silva', 'CA', '#d17a96'),
	('dddddddd-dddd-dddd-dddd-ddddddddd104', '11111111-1111-1111-1111-111111111004', 'bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbb001', 'Daniella de Morais Oliveira', 'DA', '#168aad'),
	('dddddddd-dddd-dddd-dddd-ddddddddd105', '11111111-1111-1111-1111-111111111005', 'bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbb001', 'Dayse Paiva', 'DP', '#7a6ff0'),
	('dddddddd-dddd-dddd-dddd-ddddddddd106', '11111111-1111-1111-1111-111111111006', 'bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbb001', 'Iris Rafaela da Silva', 'IR', '#d17a96'),
	('dddddddd-dddd-dddd-dddd-ddddddddd107', '11111111-1111-1111-1111-111111111007', 'bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbb001', 'Rayane Tavares Santos Araujo', 'RA', '#e09f3e'),
	('dddddddd-dddd-dddd-dddd-ddddddddd108', '11111111-1111-1111-1111-111111111008', 'bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbb001', 'Hitana Batista dos Santos', 'HI', '#355070'),
	('dddddddd-dddd-dddd-dddd-ddddddddd109', '11111111-1111-1111-1111-111111111009', 'bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbb001', 'Lara Dantas Souza', 'LA', '#23a26d'),
	('dddddddd-dddd-dddd-dddd-ddddddddd110', '11111111-1111-1111-1111-111111111010', 'bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbb004', 'Fabio dos Santos Menezes', 'FA', '#d90429'),
	('dddddddd-dddd-dddd-dddd-ddddddddd111', '11111111-1111-1111-1111-111111111011', 'bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbb004', 'Daiane Caroline dos Santos', 'DC', '#4361ee'),
	('dddddddd-dddd-dddd-dddd-ddddddddd112', '11111111-1111-1111-1111-111111111012', 'bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbb004', 'Rita Damaris Melo da Silva', 'RI', '#8f5bd4'),
	('dddddddd-dddd-dddd-dddd-ddddddddd113', '11111111-1111-1111-1111-111111111013', 'bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbb002', 'Tauvani Missielly Oliveira', 'TA', '#168aad'),
	('dddddddd-dddd-dddd-dddd-ddddddddd114', '11111111-1111-1111-1111-111111111014', 'bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbb002', 'Everland Alves dos Santos', 'EV', '#7a6ff0'),
	('dddddddd-dddd-dddd-dddd-ddddddddd115', '11111111-1111-1111-1111-111111111015', 'bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbb002', 'Fabiana Rafaella Viana Santos', 'FV', '#d17a96'),
	('dddddddd-dddd-dddd-dddd-ddddddddd116', '11111111-1111-1111-1111-111111111016', 'bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbb002', 'Acilene dos Santos', 'AC', '#e09f3e'),
	('dddddddd-dddd-dddd-dddd-ddddddddd117', '11111111-1111-1111-1111-111111111017', 'bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbb002', 'Gardenia Lobo do Nascimento', 'GA', '#355070'),
	('dddddddd-dddd-dddd-dddd-ddddddddd118', '11111111-1111-1111-1111-111111111018', 'bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbb002', 'Mirela da Silva Rodrigues', 'MI', '#23a26d');

insert into consultants (
	id,
	tenant_id,
	store_id,
	user_id,
	name,
	role_label,
	initials,
	color,
	monthly_goal,
	commission_rate,
	conversion_goal,
	avg_ticket_goal,
	pa_goal,
	is_active
)
select
	consultant_id,
	'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa'::uuid,
	store_id,
	user_id,
	name,
	'Atendimento',
	initials,
	color,
	0,
	0,
	0,
	0,
	0,
	true
from tmp_seed_consultants
on conflict (id) do update
set
	tenant_id = excluded.tenant_id,
	store_id = excluded.store_id,
	user_id = excluded.user_id,
	name = excluded.name,
	role_label = excluded.role_label,
	initials = excluded.initials,
	color = excluded.color,
	monthly_goal = excluded.monthly_goal,
	commission_rate = excluded.commission_rate,
	conversion_goal = excluded.conversion_goal,
	avg_ticket_goal = excluded.avg_ticket_goal,
	pa_goal = excluded.pa_goal,
	is_active = excluded.is_active,
	updated_at = now();

insert into store_terminals (
	tenant_id,
	store_id,
	user_id,
	code,
	device_label,
	device_slug,
	access_mode,
	is_active
)
select
	tenant_id,
	store_id,
	user_id,
	case store_id::text
		when 'bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbb001' then 'TERM-RIO'
		when 'bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbb002' then 'TERM-JAR'
		when 'bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbb003' then 'TERM-GAR'
		when 'bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbb004' then 'TERM-TRE'
		else 'TERM-STORE'
	end,
	display_name,
	case store_id::text
		when 'bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbb001' then 'riomar'
		when 'bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbb002' then 'jardins'
		when 'bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbb003' then 'garcia'
		when 'bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbb004' then 'treze'
		else 'store-terminal'
	end,
	'operations_primary',
	true
from tmp_seed_users
where role = 'store_terminal'
on conflict (store_id) do update
set
	tenant_id = excluded.tenant_id,
	user_id = excluded.user_id,
	code = excluded.code,
	device_label = excluded.device_label,
	device_slug = excluded.device_slug,
	access_mode = excluded.access_mode,
	is_active = excluded.is_active,
	updated_at = now();