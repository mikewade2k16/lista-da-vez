-- Enable concurrent services per consultant (atendimentos paralelos)
-- Refactor operation_active_services PK from (store_id, consultant_id) to (store_id, service_id)
-- This allows one consultant to have multiple active services simultaneously

-- Step 1: Create new table with updated schema
create table operation_active_services_v2 (
	store_id uuid not null references stores(id) on delete cascade,
	consultant_id uuid not null references consultants(id) on delete cascade,
	service_id text not null,
	service_started_at bigint not null,
	queue_joined_at bigint not null,
	queue_wait_ms bigint not null default 0,
	queue_position_at_start integer,
	start_mode text not null check (start_mode in ('queue', 'queue-jump', 'parallel')),
	skipped_people_json jsonb not null default '[]'::jsonb,
	parallel_group_id text,
	parallel_start_index integer,
	sibling_service_ids_json jsonb not null default '[]'::jsonb,
	start_offset_ms bigint not null default 0,
	created_at timestamptz not null default now(),
	updated_at timestamptz not null default now(),
	primary key (store_id, service_id),
	unique (store_id, service_id)
);

-- Step 2: Copy data from old table (preserve existing services as 'queue' or 'queue-jump' mode, not parallel)
insert into operation_active_services_v2 (
	store_id,
	consultant_id,
	service_id,
	service_started_at,
	queue_joined_at,
	queue_wait_ms,
	queue_position_at_start,
	start_mode,
	skipped_people_json,
	parallel_group_id,
	parallel_start_index,
	sibling_service_ids_json,
	start_offset_ms,
	created_at,
	updated_at
)
select
	store_id,
	consultant_id,
	service_id,
	service_started_at,
	queue_joined_at,
	queue_wait_ms,
	queue_position_at_start,
	start_mode,
	skipped_people_json,
	null as parallel_group_id,
	null as parallel_start_index,
	'[]'::jsonb as sibling_service_ids_json,
	0 as start_offset_ms,
	created_at,
	updated_at
from operation_active_services;

-- Step 3: Drop old table and rename new one
drop table operation_active_services;
alter table operation_active_services_v2 rename to operation_active_services;

-- Step 4: Recreate indexes (adjusted for new PK)
create index if not exists operation_active_services_store_idx
	on operation_active_services (store_id, service_started_at);

create index if not exists operation_active_services_consultant_idx
	on operation_active_services (store_id, consultant_id, service_started_at);

-- Step 5: Update operation_service_history to include parallel metadata
alter table operation_service_history
add column if not exists parallel_group_id text,
add column if not exists parallel_start_index integer,
add column if not exists sibling_service_ids_json jsonb not null default '[]'::jsonb,
add column if not exists start_offset_ms bigint not null default 0;

-- Update check constraint to include 'parallel' mode
alter table operation_service_history drop constraint if exists operation_service_history_start_mode_check;
alter table operation_service_history
add constraint operation_service_history_start_mode_check
	check (start_mode in ('queue', 'queue-jump', 'parallel'));
