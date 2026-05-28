-- 0058_erp_raw_fidelity.sql
-- Makes ERP raw tables preserve the source CSV row shape in addition to typed helper columns.

alter table erp_item_raw
    add column if not exists raw_values jsonb not null default '[]'::jsonb,
    add column if not exists raw_payload jsonb not null default '{}'::jsonb;

alter table erp_customer_raw
    add column if not exists store_id_raw text not null default '',
    add column if not exists raw_values jsonb not null default '[]'::jsonb,
    add column if not exists raw_payload jsonb not null default '{}'::jsonb;

alter table erp_employee_raw
    add column if not exists store_id_raw text not null default '',
    add column if not exists raw_values jsonb not null default '[]'::jsonb,
    add column if not exists raw_payload jsonb not null default '{}'::jsonb;

alter table erp_order_raw
    add column if not exists store_id_raw text not null default '',
    add column if not exists raw_values jsonb not null default '[]'::jsonb,
    add column if not exists raw_payload jsonb not null default '{}'::jsonb;

alter table erp_order_canceled_raw
    add column if not exists store_id_raw text not null default '',
    add column if not exists raw_values jsonb not null default '[]'::jsonb,
    add column if not exists raw_payload jsonb not null default '{}'::jsonb;

create index if not exists erp_customer_raw_store_id_raw_idx
    on erp_customer_raw (tenant_id, store_id, store_id_raw)
    where store_id_raw <> '';

create index if not exists erp_employee_raw_store_id_raw_idx
    on erp_employee_raw (tenant_id, store_id, store_id_raw)
    where store_id_raw <> '';

create index if not exists erp_order_raw_store_id_raw_idx
    on erp_order_raw (tenant_id, store_id, store_id_raw)
    where store_id_raw <> '';

create index if not exists erp_order_canceled_raw_store_id_raw_idx
    on erp_order_canceled_raw (tenant_id, store_id, store_id_raw)
    where store_id_raw <> '';

-- Rollback (manual, if ever needed):
-- drop index if exists erp_order_canceled_raw_store_id_raw_idx;
-- drop index if exists erp_order_raw_store_id_raw_idx;
-- drop index if exists erp_employee_raw_store_id_raw_idx;
-- drop index if exists erp_customer_raw_store_id_raw_idx;
-- alter table erp_order_canceled_raw drop column if exists raw_payload, drop column if exists raw_values, drop column if exists store_id_raw;
-- alter table erp_order_raw drop column if exists raw_payload, drop column if exists raw_values, drop column if exists store_id_raw;
-- alter table erp_employee_raw drop column if exists raw_payload, drop column if exists raw_values, drop column if exists store_id_raw;
-- alter table erp_customer_raw drop column if exists raw_payload, drop column if exists raw_values, drop column if exists store_id_raw;
-- alter table erp_item_raw drop column if exists raw_payload, drop column if exists raw_values;
