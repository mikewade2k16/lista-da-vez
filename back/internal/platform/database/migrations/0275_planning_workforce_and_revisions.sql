-- Contratos de jornada normalizados, metas calculadas e historico da escala.

create table if not exists queue.planning_staff_contracts (
	id uuid primary key default gen_random_uuid(),
	tenant_id uuid not null references core.accounts(id) on delete cascade,
	store_id uuid not null references queue.stores(id) on delete cascade,
	consultant_id uuid not null references queue.consultants(id) on delete cascade,
	weekly_hours numeric(5, 2) not null default 44,
	max_daily_hours numeric(4, 2) not null default 8,
	target_weight numeric(5, 2) not null default 1,
	available_weekdays text[] not null default array['mon','tue','wed','thu','fri','sat','sun']::text[],
	version bigint not null default 1,
	created_by_user_id uuid null references core.users(id) on delete set null,
	updated_by_user_id uuid null references core.users(id) on delete set null,
	created_at timestamptz not null default now(),
	updated_at timestamptz not null default now(),
	constraint planning_staff_contracts_weekly_hours_check check (weekly_hours > 0 and weekly_hours <= 60),
	constraint planning_staff_contracts_daily_hours_check check (max_daily_hours > 0 and max_daily_hours <= 12),
	constraint planning_staff_contracts_target_weight_check check (target_weight >= 0 and target_weight <= 3),
	constraint planning_staff_contracts_weekdays_check check (
		available_weekdays <@ array['mon','tue','wed','thu','fri','sat','sun']::text[]
	),
	unique (tenant_id, store_id, consultant_id)
);

create index if not exists planning_staff_contracts_tenant_store_idx
	on queue.planning_staff_contracts (tenant_id, store_id, consultant_id);

insert into queue.planning_staff_contracts (
	tenant_id,
	store_id,
	consultant_id,
	weekly_hours,
	max_daily_hours,
	target_weight,
	available_weekdays,
	created_by_user_id,
	updated_by_user_id
)
select
	config.tenant_id,
	config.store_id,
	(member->>'id')::uuid,
	coalesce(nullif(member->>'weeklyHours', '')::numeric, 44),
	coalesce(nullif(member->>'maxDailyHours', '')::numeric, 8),
	coalesce(nullif(member->>'targetWeight', '')::numeric, 1),
	coalesce(
		array(select jsonb_array_elements_text(coalesce(member->'availableDays', '[]'::jsonb))),
		array['mon','tue','wed','thu','fri','sat','sun']::text[]
	),
	config.created_by_user_id,
	config.updated_by_user_id
from queue.planning_store_configs config
cross join lateral jsonb_array_elements(coalesce(config.configuration->'staff', '[]'::jsonb)) member
where member->>'id' ~* '^[0-9a-f]{8}-[0-9a-f]{4}-[1-5][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$'
on conflict (tenant_id, store_id, consultant_id) do nothing;

alter table queue.planning_schedules
	add column if not exists target_month date null,
	add column if not exists goal_week smallint null,
	add column if not exists goal_allocations jsonb not null default '[]'::jsonb,
	add column if not exists published_at timestamptz null,
	add column if not exists published_by_user_id uuid null references core.users(id) on delete set null;

update queue.planning_schedules
set
	target_month = date_trunc('month', week_start + 3)::date,
	goal_week = least(4, ((extract(day from week_start + 3)::integer - 1) / 7) + 1)
where target_month is null or goal_week is null;

alter table queue.planning_schedules
	alter column target_month set not null,
	alter column goal_week set not null;

alter table queue.planning_schedules
	drop constraint if exists planning_schedules_goal_week_check;

alter table queue.planning_schedules
	add constraint planning_schedules_goal_week_check check (goal_week between 1 and 4),
	add constraint planning_schedules_goal_allocations_array_check check (jsonb_typeof(goal_allocations) = 'array');

create index if not exists planning_schedules_goal_period_idx
	on queue.planning_schedules (tenant_id, store_id, target_month, goal_week);

create table if not exists queue.planning_schedule_revisions (
	id uuid primary key default gen_random_uuid(),
	tenant_id uuid not null references core.accounts(id) on delete cascade,
	store_id uuid not null references queue.stores(id) on delete cascade,
	schedule_id uuid not null references queue.planning_schedules(id) on delete cascade,
	version bigint not null,
	status text not null,
	shifts jsonb not null,
	goal_allocations jsonb not null default '[]'::jsonb,
	changed_by_user_id uuid null references core.users(id) on delete set null,
	created_at timestamptz not null default now(),
	constraint planning_schedule_revisions_status_check check (status in ('saved', 'published')),
	constraint planning_schedule_revisions_shifts_array_check check (jsonb_typeof(shifts) = 'array'),
	constraint planning_schedule_revisions_allocations_array_check check (jsonb_typeof(goal_allocations) = 'array'),
	unique (schedule_id, version)
);

create index if not exists planning_schedule_revisions_schedule_idx
	on queue.planning_schedule_revisions (tenant_id, store_id, schedule_id, version desc);
