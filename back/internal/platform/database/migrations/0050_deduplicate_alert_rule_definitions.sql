-- 0050_deduplicate_alert_rule_definitions.sql
-- Cleans duplicate dynamic alert rules created by earlier local backfills and
-- locks the default tenant/trigger/name identity going forward.

with ranked as (
    select
        id,
        row_number() over (
            partition by tenant_id, trigger_type, name
            order by is_active desc, updated_at desc, created_at desc, id
        ) as row_rank
    from alert_rule_definitions
)
delete from alert_rule_definitions rule_definitions
using ranked
where rule_definitions.id = ranked.id
  and ranked.row_rank > 1;

create unique index if not exists alert_rule_definitions_tenant_trigger_name_uidx
    on alert_rule_definitions (tenant_id, trigger_type, name);
