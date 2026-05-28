-- Fase 3 da refatoracao de settings: tabela de thresholds e alertas operacionais do tenant.
-- Separa os alertas do tenant_operation_settings para evitar crescimento da tabela central.
-- tenant_operation_settings continua existindo e nao e alterada por esta migration.
--
-- Rollback:
--   drop table if exists tenant_alert_settings;

create table if not exists tenant_alert_settings (
    tenant_id                 uuid primary key references tenants(id) on delete cascade,
    alert_min_conversion_rate numeric(8, 2) not null default 0,
    alert_max_queue_jump_rate numeric(8, 2) not null default 0,
    alert_min_pa_score        numeric(8, 2) not null default 0,
    alert_min_ticket_average  numeric(14, 2) not null default 0,
    updated_by                uuid null,
    updated_at                timestamptz not null default now()
);

-- Backfill a partir de tenant_operation_settings.
-- on conflict do nothing: seguro para reexecutar sem risco de sobrescrita.
insert into tenant_alert_settings (
    tenant_id,
    alert_min_conversion_rate,
    alert_max_queue_jump_rate,
    alert_min_pa_score,
    alert_min_ticket_average,
    updated_at
)
select
    tenant_id,
    alert_min_conversion_rate,
    alert_max_queue_jump_rate,
    alert_min_pa_score,
    alert_min_ticket_average,
    updated_at
from tenant_operation_settings
on conflict (tenant_id) do nothing;
