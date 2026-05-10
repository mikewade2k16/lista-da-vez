-- 0059_erp_sync_guardrails.sql
-- Prevents overlapping CSV sync/backfill runs for the same ERP store scope.

create unique index if not exists erp_sync_runs_one_running_csv_ftp_per_store_idx
    on erp_sync_runs (tenant_id, store_id)
    where mode = 'csv_ftp'
      and status = 'running';

-- Rollback (manual, if ever needed):
-- drop index if exists erp_sync_runs_one_running_csv_ftp_per_store_idx;
