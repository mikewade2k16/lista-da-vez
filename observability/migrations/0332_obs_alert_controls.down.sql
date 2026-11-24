-- Reversão da migration 0332_obs_alert_controls.sql.
-- Remove SOMENTE as colunas de reconhecimento/silêncio de obs.alerts.
--   psql "$DATABASE_URL" -f observability/migrations/0332_obs_alert_controls.down.sql
begin;
alter table obs.alerts drop column if exists acknowledged_at;
alter table obs.alerts drop column if exists acknowledged_by;
alter table obs.alerts drop column if exists silenced_until;
alter table obs.alerts drop column if exists silenced_by;
delete from public.schema_migrations where version = '0332_obs_alert_controls.sql';
commit;
