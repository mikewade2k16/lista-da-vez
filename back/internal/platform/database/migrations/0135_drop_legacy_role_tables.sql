-- 0135 — U4c: DROP das tabelas legadas de papel.
--
-- Substituidas 100% por core.* (core.account_users + core.user_role_assignments
-- + core.user_module_settings). Caminho:
--   - 0133: backfill legado -> core.
--   - U4a: leitores de escopo (crm/erp, settings, stores, tenants) leem core.
--   - U4b: writers (users, stores, consultants, admin, bootstrap) gravam so core.
--   - auth: AUTH_ROLES_SOURCE=core (sem fallback legado).
--
-- Num DB novo estas tabelas sao criadas/seedadas pelas migrations historicas
-- (0001/0002/0012/0015/0036) e backfillas para core pela 0133 ANTES deste drop,
-- entao a ordem se mantem consistente.
--
-- Backup pre-drop: C:\tmp\backup_legacy_roles_pre_drop.sql (+ omni_full_pre_drop.dump).
-- Idempotente (IF EXISTS). Sao tabelas-folha (nenhuma outra referencia FK a elas).

drop table if exists public.user_store_roles;
drop table if exists public.user_tenant_roles;
drop table if exists public.user_platform_roles;
