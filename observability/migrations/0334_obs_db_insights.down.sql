-- Reversão da migration 0334_obs_db_insights.sql.
--   psql "$DATABASE_URL" -f observability/migrations/0334_obs_db_insights.down.sql
begin;
drop table if exists obs.table_stats;
drop table if exists obs.query_plans;
delete from public.schema_migrations where version = '0334_obs_db_insights.sql';
commit;
