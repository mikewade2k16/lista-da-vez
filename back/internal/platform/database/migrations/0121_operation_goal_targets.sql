create table if not exists queue.operation_goal_targets (
	id uuid primary key default gen_random_uuid(),
	tenant_id uuid not null references tenants(id) on delete cascade,
	store_id uuid not null references queue.stores(id) on delete cascade,
	consultant_id uuid null references queue.consultants(id) on delete set null,
	target_month date not null,
	monthly_goal numeric(14, 2) not null default 0,
	avg_ticket_goal numeric(14, 2) not null default 0,
	conversion_goal numeric(6, 2) not null default 0,
	pa_goal numeric(8, 2) not null default 0,
	created_by_user_id uuid null references users(id) on delete set null,
	updated_by_user_id uuid null references users(id) on delete set null,
	created_at timestamptz not null default now(),
	updated_at timestamptz not null default now(),
	constraint operation_goal_targets_month_start
		check (target_month = date_trunc('month', target_month)::date),
	constraint operation_goal_targets_non_negative_metrics
		check (
			monthly_goal >= 0
			and avg_ticket_goal >= 0
			and conversion_goal >= 0
			and conversion_goal <= 100
			and pa_goal >= 0
		)
);

create unique index if not exists operation_goal_targets_store_scope_uidx
	on queue.operation_goal_targets (tenant_id, store_id, target_month)
	where consultant_id is null;

create unique index if not exists operation_goal_targets_consultant_scope_uidx
	on queue.operation_goal_targets (tenant_id, store_id, consultant_id, target_month)
	where consultant_id is not null;

create index if not exists operation_goal_targets_month_store_idx
	on queue.operation_goal_targets (target_month, store_id);

create index if not exists operation_goal_targets_consultant_idx
	on queue.operation_goal_targets (consultant_id, target_month)
	where consultant_id is not null;

create or replace view public.operation_goal_targets as
	select * from queue.operation_goal_targets;

insert into access_permissions (key, scope, description)
values
	('workspace.multiloja.view', 'tenant', 'Visualizar a workspace Multi-loja.'),
	('workspace.multiloja.edit', 'tenant', 'Editar lojas e configuracoes administrativas da workspace Multi-loja.')
on conflict (key) do update
set
	scope = excluded.scope,
	description = excluded.description;

insert into access_role_permissions (role, permission_key)
values
	('marketing', 'workspace.multiloja.view'),
	('director', 'workspace.multiloja.view'),
	('manager', 'workspace.multiloja.view')
on conflict (role, permission_key) do nothing;