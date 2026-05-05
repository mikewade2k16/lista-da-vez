-- Fase 3 da refatoracao de settings: tabela de configuracoes operacionais estaveis do tenant.
-- Separa os campos de operacao core do tenant_operation_settings (tabela larga legacy).
-- tenant_operation_settings continua existindo e nao e alterada por esta migration.
--
-- Rollback:
--   drop table if exists tenant_operation_core_settings;

create table if not exists tenant_operation_core_settings (
    tenant_id                              uuid primary key references tenants(id) on delete cascade,
    selected_operation_template_id         text not null default 'joalheria-padrao',
    max_concurrent_services                integer not null default 10,
    max_concurrent_services_per_consultant integer not null default 1,
    timing_fast_close_minutes              integer not null default 5,
    timing_long_service_minutes            integer not null default 25,
    timing_low_sale_amount                 numeric(14, 2) not null default 1200,
    service_cancel_window_seconds          integer not null default 30,
    test_mode_enabled                      boolean not null default false,
    auto_fill_finish_modal                 boolean not null default false,
    updated_by                             uuid null,
    updated_at                             timestamptz not null default now()
);

-- Backfill a partir de tenant_operation_settings.
-- Usa coalesce nas colunas adicionadas em migrations posteriores que podem ser nulas
-- em ambientes antigos (max_concurrent_services_per_consultant, service_cancel_window_seconds).
-- on conflict do nothing: seguro para reexecutar sem risco de sobrescrita.
insert into tenant_operation_core_settings (
    tenant_id,
    selected_operation_template_id,
    max_concurrent_services,
    max_concurrent_services_per_consultant,
    timing_fast_close_minutes,
    timing_long_service_minutes,
    timing_low_sale_amount,
    service_cancel_window_seconds,
    test_mode_enabled,
    auto_fill_finish_modal,
    updated_at
)
select
    tenant_id,
    selected_operation_template_id,
    max_concurrent_services,
    coalesce(max_concurrent_services_per_consultant, 1),
    timing_fast_close_minutes,
    timing_long_service_minutes,
    timing_low_sale_amount,
    coalesce(service_cancel_window_seconds, 30),
    test_mode_enabled,
    auto_fill_finish_modal,
    updated_at
from tenant_operation_settings
on conflict (tenant_id) do nothing;
