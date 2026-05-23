-- Add Score 360 weights to the tenant-wide operation settings source of truth.

alter table tenant_operation_core_settings
add column if not exists score_weight_conversion numeric(8, 2) not null default 35;

alter table tenant_operation_core_settings
add column if not exists score_weight_sold_value numeric(8, 2) not null default 25;

alter table tenant_operation_core_settings
add column if not exists score_weight_quality numeric(8, 2) not null default 20;

alter table tenant_operation_core_settings
add column if not exists score_weight_pa numeric(8, 2) not null default 15;

alter table tenant_operation_core_settings
add column if not exists score_weight_queue_discipline numeric(8, 2) not null default 5;