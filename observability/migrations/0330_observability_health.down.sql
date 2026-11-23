-- Reversão da migration 0330_observability_health.sql.
-- Apaga SOMENTE as tabelas de saúde/alertas/segurança do schema obs.
--   psql "$DATABASE_URL" -f observability/migrations/0330_observability_health.down.sql
begin;
drop table if exists obs.system_samples;
drop table if exists obs.container_samples;
drop table if exists obs.presence;
drop table if exists obs.alerts;
drop table if exists obs.security_checks;
drop table if exists obs.findings;
drop table if exists obs.deploys;
delete from public.schema_migrations where version = '0330_observability_health.sql';
commit;
