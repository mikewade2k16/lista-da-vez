-- Reversão da migration 0333_obs_governance.sql.
--   psql "$DATABASE_URL" -f observability/migrations/0333_obs_governance.down.sql
begin;
drop table if exists obs.settings;
alter table obs.findings drop column if exists owner;
alter table obs.findings drop column if exists due_date;
alter table obs.findings drop column if exists note;
alter table obs.findings drop column if exists updated_by;
delete from public.schema_migrations where version = '0333_obs_governance.sql';
commit;
