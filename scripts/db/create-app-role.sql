-- DESDE O AC-04b o `migrate up` auto-provisiona esta role no boot (cria +
-- converge senha/atributos + grant connect, a partir de DATABASE_APP_URL).
-- Este script permanece como FALLBACK MANUAL e como caminho do initdb
-- (scripts/db/postgres-init/10-app-role.sh) para volumes novos.
-- scripts/db/create-app-role.sql — AC-04: role de RUNTIME least-privilege da api.
-- Idempotente. NAO e migration (role e cluster-level; senha vem de env).
-- Uso: psql -v ON_ERROR_STOP=1 -U <superuser> -d <db> -v role=omni_app -v pw='<senha>' -f scripts/db/create-app-role.sql
-- Os GRANTs de tabelas/sequences sao aplicados pelo `migrate up` (SyncAppRoleGrants)
-- a cada boot da api — este arquivo cuida apenas de: existencia, senha, atributos, CONNECT.

\set ON_ERROR_STOP on

select format('create role %I login', :'role')
where not exists (select 1 from pg_roles where rolname = :'role')
\gexec

select format(
  'alter role %I with login password %L nosuperuser nocreatedb nocreaterole nobypassrls noreplication',
  :'role', :'pw')
\gexec

select format('grant connect on database %I to %I', current_database(), :'role')
\gexec
