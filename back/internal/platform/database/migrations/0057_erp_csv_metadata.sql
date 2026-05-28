-- 0057_erp_csv_metadata.sql
-- Adds CSV-native sync metadata for ERP ingestion.

do $$
begin
    if not exists (
        select 1
        from information_schema.columns
        where table_schema = 'public'
          and table_name = 'erp_sync_runs'
          and column_name = 'triggered_by'
    ) then
        alter table erp_sync_runs
            add column triggered_by text not null default 'manual';
    end if;
end
$$;

do $$
begin
    if exists (
        select 1
        from pg_constraint
        where conrelid = 'erp_sync_runs'::regclass
          and conname = 'erp_sync_runs_mode_check'
    ) then
        alter table erp_sync_runs
            drop constraint erp_sync_runs_mode_check;
    end if;

    alter table erp_sync_runs
        add constraint erp_sync_runs_mode_check
        check (mode in ('bootstrap_markdown', 'csv_ftp'));
exception
    when duplicate_object then
        null;
end
$$;

do $$
begin
    if exists (
        select 1
        from pg_constraint
        where conrelid = 'erp_sync_runs'::regclass
          and conname = 'erp_sync_runs_triggered_by_check'
    ) then
        alter table erp_sync_runs
            drop constraint erp_sync_runs_triggered_by_check;
    end if;

    alter table erp_sync_runs
        add constraint erp_sync_runs_triggered_by_check
        check (triggered_by in ('manual', 'cron', 'backfill'));
exception
    when duplicate_object then
        null;
end
$$;

alter table erp_sync_files
    add column if not exists source_extracted_at timestamptz,
    add column if not exists source_data_reference timestamptz,
    add column if not exists source_size_bytes bigint,
    add column if not exists error_message text;

alter table erp_item_current
    add column if not exists source_extracted_at timestamptz;

-- Rollback (manual, if ever needed):
-- alter table erp_item_current drop column if exists source_extracted_at;
-- alter table erp_sync_files
--     drop column if exists error_message,
--     drop column if exists source_size_bytes,
--     drop column if exists source_data_reference,
--     drop column if exists source_extracted_at;
-- alter table erp_sync_runs drop constraint if exists erp_sync_runs_triggered_by_check;
-- alter table erp_sync_runs drop constraint if exists erp_sync_runs_mode_check;
-- alter table erp_sync_runs add constraint erp_sync_runs_mode_check check (mode in ('bootstrap_markdown'));
-- alter table erp_sync_runs drop column if exists triggered_by;