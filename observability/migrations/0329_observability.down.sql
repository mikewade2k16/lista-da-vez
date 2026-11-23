-- Reversão da migration 0329_observability.sql.
-- Apaga SOMENTE o schema obs (telemetria); nenhum dado de negócio é tocado.
-- Rodar manualmente com a role privilegiada e depois remover a linha do controle:
--   psql "$DATABASE_URL" -f observability/migrations/0329_observability.down.sql
-- O nome da tabela de controle vem do migrator (back/internal/platform/database/migrator.go).
begin;
drop schema if exists obs cascade;
delete from public.schema_migrations where version = '0329_observability.sql';
commit;
