-- Metas por SEMANA: dimensao de semana em operation_goal_targets.
-- week = 0 -> meta MENSAL (mes inteiro); 1..4 -> semana do mes
-- (S1=1-7, S2=8-14, S3=15-21, S4=22-fim). Linhas existentes viram week=0 (mensal).
-- SQL plano e idempotente (o migrator roda o arquivo inteiro).

alter table queue.operation_goal_targets
	add column if not exists week smallint not null default 0;

alter table queue.operation_goal_targets
	drop constraint if exists operation_goal_targets_week_range;
alter table queue.operation_goal_targets
	add constraint operation_goal_targets_week_range check (week >= 0 and week <= 4);

-- Reindice unico incluindo a semana: um registro por escopo por mes por semana.
drop index if exists queue.operation_goal_targets_store_scope_uidx;
drop index if exists queue.operation_goal_targets_consultant_scope_uidx;

create unique index if not exists operation_goal_targets_store_scope_uidx
	on queue.operation_goal_targets (tenant_id, store_id, target_month, week)
	where consultant_id is null;

create unique index if not exists operation_goal_targets_consultant_scope_uidx
	on queue.operation_goal_targets (tenant_id, store_id, consultant_id, target_month, week)
	where consultant_id is not null;

-- View espelho pega a coluna nova (append no fim, compativel com create or replace).
create or replace view public.operation_goal_targets as
	select * from queue.operation_goal_targets;
