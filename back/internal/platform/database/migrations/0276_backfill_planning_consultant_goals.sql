-- Reconstroi metas individuais derivadas para meses que ja tinham escalas geradas
-- antes de o Planejamento passar a persistir os quatro periodos e o consolidado mensal.

with source_schedule as (
	select distinct on (schedule.tenant_id, schedule.store_id, schedule.target_month)
		schedule.tenant_id,
		schedule.store_id,
		schedule.target_month,
		schedule.goal_allocations,
		coalesce(schedule.updated_by_user_id, schedule.created_by_user_id) as user_id
	from queue.planning_schedules schedule
	where jsonb_array_length(schedule.goal_allocations) > 0
	order by
		schedule.tenant_id,
		schedule.store_id,
		schedule.target_month,
		jsonb_array_length(schedule.goal_allocations) desc,
		schedule.updated_at desc
), source_allocation as (
	select
		source.tenant_id,
		source.store_id,
		source.target_month,
		(allocation->>'staffId')::uuid as consultant_id,
		greatest(coalesce((allocation->>'share')::numeric, 0), 0) as share,
		source.user_id
	from source_schedule source
	cross join lateral jsonb_array_elements(source.goal_allocations) allocation
	join queue.consultants consultant
		on consultant.id = (allocation->>'staffId')::uuid
		and consultant.tenant_id = source.tenant_id
		and consultant.store_id = source.store_id
		and consultant.is_active = true
), weekly_raw as (
	select
		source.tenant_id,
		source.store_id,
		source.target_month,
		store_goal.week,
		store_goal.monthly_goal as store_target,
		store_goal.avg_ticket_goal,
		store_goal.conversion_goal,
		store_goal.pa_goal,
		source.consultant_id,
		source.share,
		source.user_id,
		round(store_goal.monthly_goal * source.share, 2) as raw_target
	from source_allocation source
	join queue.operation_goal_targets store_goal
		on store_goal.tenant_id = source.tenant_id
		and store_goal.store_id = source.store_id
		and store_goal.target_month = source.target_month
		and store_goal.consultant_id is null
		and store_goal.week between 1 and 4
), weekly_adjusted as (
	select
		weekly_raw.*,
		raw_target + case
			when consultant_id::text = max(consultant_id::text) filter (where share > 0)
				over (partition by tenant_id, store_id, target_month, week)
			then store_target - sum(raw_target)
				over (partition by tenant_id, store_id, target_month, week)
			else 0
		end as target
	from weekly_raw
)
insert into queue.operation_goal_targets (
	tenant_id,
	store_id,
	consultant_id,
	target_month,
	week,
	monthly_goal,
	avg_ticket_goal,
	conversion_goal,
	pa_goal,
	created_by_user_id,
	updated_by_user_id
)
select
	tenant_id,
	store_id,
	consultant_id,
	target_month,
	week,
	target,
	avg_ticket_goal,
	conversion_goal,
	pa_goal,
	user_id,
	user_id
from weekly_adjusted
where store_target > 0
on conflict (tenant_id, store_id, consultant_id, target_month, week)
	where consultant_id is not null
do update set
	monthly_goal = excluded.monthly_goal,
	avg_ticket_goal = excluded.avg_ticket_goal,
	conversion_goal = excluded.conversion_goal,
	pa_goal = excluded.pa_goal,
	updated_by_user_id = excluded.updated_by_user_id,
	updated_at = now()
where queue.operation_goal_targets.monthly_goal = 0;

delete from queue.operation_goal_targets consultant_goal
using (
	select distinct tenant_id, store_id, target_month
	from queue.planning_schedules
) planned_month
where consultant_goal.tenant_id = planned_month.tenant_id
	and consultant_goal.store_id = planned_month.store_id
	and consultant_goal.target_month = planned_month.target_month
	and consultant_goal.consultant_id is not null
	and consultant_goal.week = 0
	and exists (
		select 1
		from queue.operation_goal_targets weekly_goal
		where weekly_goal.tenant_id = planned_month.tenant_id
			and weekly_goal.store_id = planned_month.store_id
			and weekly_goal.target_month = planned_month.target_month
			and weekly_goal.consultant_id is not null
			and weekly_goal.week between 1 and 4
			and weekly_goal.monthly_goal > 0
	);

with monthly_store as (
	select
		goal.tenant_id,
		goal.store_id,
		goal.target_month,
		goal.monthly_goal as store_target,
		goal.avg_ticket_goal,
		goal.conversion_goal,
		goal.pa_goal,
		goal.updated_by_user_id
	from queue.operation_goal_targets goal
	where goal.consultant_id is null
		and goal.week = 0
		and exists (
			select 1
			from queue.planning_schedules schedule
			where schedule.tenant_id = goal.tenant_id
				and schedule.store_id = goal.store_id
				and schedule.target_month = goal.target_month
		)
), weekly_total as (
	select
		goal.tenant_id,
		goal.store_id,
		goal.target_month,
		goal.consultant_id,
		sum(goal.monthly_goal) as consultant_total
	from queue.operation_goal_targets goal
	join queue.consultants consultant
		on consultant.id = goal.consultant_id
		and consultant.tenant_id = goal.tenant_id
		and consultant.store_id = goal.store_id
		and consultant.is_active = true
	where goal.week between 1 and 4
	group by goal.tenant_id, goal.store_id, goal.target_month, goal.consultant_id
	having sum(goal.monthly_goal) > 0
), monthly_raw as (
	select
		weekly_total.*,
		monthly_store.store_target,
		monthly_store.avg_ticket_goal,
		monthly_store.conversion_goal,
		monthly_store.pa_goal,
		monthly_store.updated_by_user_id,
		round(
			monthly_store.store_target * weekly_total.consultant_total
			/ sum(weekly_total.consultant_total)
				over (partition by weekly_total.tenant_id, weekly_total.store_id, weekly_total.target_month),
			2
		) as raw_target
	from weekly_total
	join monthly_store using (tenant_id, store_id, target_month)
), monthly_adjusted as (
	select
		monthly_raw.*,
		raw_target + case
			when consultant_id::text = max(consultant_id::text)
				over (partition by tenant_id, store_id, target_month)
			then store_target - sum(raw_target)
				over (partition by tenant_id, store_id, target_month)
			else 0
		end as target
	from monthly_raw
)
insert into queue.operation_goal_targets (
	tenant_id,
	store_id,
	consultant_id,
	target_month,
	week,
	monthly_goal,
	avg_ticket_goal,
	conversion_goal,
	pa_goal,
	created_by_user_id,
	updated_by_user_id
)
select
	tenant_id,
	store_id,
	consultant_id,
	target_month,
	0,
	target,
	avg_ticket_goal,
	conversion_goal,
	pa_goal,
	updated_by_user_id,
	updated_by_user_id
from monthly_adjusted;
