alter table queue.operation_goal_targets
	drop constraint if exists operation_goal_targets_week_range;

alter table queue.operation_goal_targets
	add constraint operation_goal_targets_week_range check (week between 0 and 5);

alter table queue.planning_schedules
	drop constraint if exists planning_schedules_goal_week_check;

alter table queue.planning_schedules
	add constraint planning_schedules_goal_week_check check (goal_week between 1 and 5);

alter table queue.performance_feedback_reviews
	drop constraint if exists performance_feedback_reviews_week_check;

alter table queue.performance_feedback_reviews
	add constraint performance_feedback_reviews_week_check check (week between 0 and 5);
