-- Add Score 360 weights to the tenant-wide operation settings source of truth.
-- A tabela vive em queue.* desde 0105_queue_operations.sql; public.* e uma view de compat.

alter table queue.tenant_operation_core_settings
add column if not exists score_weight_conversion numeric(8, 2) not null default 35;

alter table queue.tenant_operation_core_settings
add column if not exists score_weight_sold_value numeric(8, 2) not null default 25;

alter table queue.tenant_operation_core_settings
add column if not exists score_weight_quality numeric(8, 2) not null default 20;

alter table queue.tenant_operation_core_settings
add column if not exists score_weight_pa numeric(8, 2) not null default 15;

alter table queue.tenant_operation_core_settings
add column if not exists score_weight_queue_discipline numeric(8, 2) not null default 5;

-- View public.* nao replica automaticamente colunas novas (SELECT * e congelado em CREATE VIEW).
-- Recria a view para expor as colunas novas a consumidores que ainda usam o schema public.
create or replace view public.tenant_operation_core_settings as
    select * from queue.tenant_operation_core_settings;
