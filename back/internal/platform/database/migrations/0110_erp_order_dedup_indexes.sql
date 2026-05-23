-- 0110_erp_order_dedup_indexes.sql
-- Speeds up the ERP purchases view, which collapses repeated raw order rows by purchase.

create index if not exists erp_order_raw_purchase_dedup_idx
    on erp_order_raw (
        tenant_id,
        store_id,
        order_id,
        source_batch_date desc,
        created_at_imported desc,
        source_file_name desc,
        file_id desc
    )
    where order_id <> '';

create index if not exists erp_order_raw_purchase_file_idx
    on erp_order_raw (
        tenant_id,
        store_id,
        file_id,
        order_id
    )
    where order_id <> '';

create index if not exists erp_order_canceled_raw_purchase_dedup_idx
    on erp_order_canceled_raw (
        tenant_id,
        store_id,
        order_id,
        source_batch_date desc,
        created_at_imported desc,
        source_file_name desc,
        file_id desc
    )
    where order_id <> '';

create index if not exists erp_order_canceled_raw_purchase_file_idx
    on erp_order_canceled_raw (
        tenant_id,
        store_id,
        file_id,
        order_id
    )
    where order_id <> '';

-- Rollback (manual, if ever needed):
-- drop index if exists erp_order_canceled_raw_purchase_file_idx;
-- drop index if exists erp_order_canceled_raw_purchase_dedup_idx;
-- drop index if exists erp_order_raw_purchase_file_idx;
-- drop index if exists erp_order_raw_purchase_dedup_idx;
