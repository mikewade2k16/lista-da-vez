-- 0048_alert_instances_consultant_name.sql
-- Denormalizes consultant_name as snapshot in alert_instances
-- Allows alerts to display consultant name even if consultant record is deleted
-- Backfills existing alerts from consultants table

alter table alert_instances
    add column if not exists consultant_name text not null default '';

-- Backfill: populate consultant_name from consultants table
update alert_instances ai
set consultant_name = c.name
from consultants c
where ai.consultant_id = c.id
    and ai.consultant_name = '';

-- Create index for consultant-based filtering
create index if not exists alert_instances_consultant_name_idx
    on alert_instances (store_id, consultant_id, consultant_name);

-- Rollback:
-- drop index if exists alert_instances_consultant_name_idx;
-- alter table alert_instances
--     drop column if exists consultant_name;
