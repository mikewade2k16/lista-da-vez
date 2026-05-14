-- Fase 7A — Performance do hot-path de autenticacao
-- Branch: refactor/multi-tenant-core
--
-- Contexto: depois das Fases 4A-4C que moveram tabelas para queue.*, as
-- estatisticas do planner ficaram defasadas para as views compat e para
-- as tabelas core.* (que foram populadas em 0101/0103). Atualizar
-- estatisticas ajuda o planner a escolher os indices certos no hot-path
-- de cada request autenticado.
--
-- Os indices criticos do hot-path ja foram criados em migrations anteriores:
--   * user_tenant_roles_user_id_idx, user_store_roles_user_id_idx (0001)
--   * user_platform_roles PK em user_id (0001)
--   * user_access_overrides_user_idx (user_id, is_active, permission_key) (0015a)
--   * core_user_role_assignments_account_user_idx (account_id, user_id) (0100)
--   * core_user_permission_overrides_lookup_idx (user_id, account_id) WHERE is_active (0100)
--   * core_user_sessions_active_idx (user_id) WHERE revoked_at IS NULL (0100)
--   * access_role_permissions PK (role, permission_key) (0015a)
--
-- Esta migration:
--   1. Adiciona um indice de cobertura em core.user_sessions para futuro lookup
--      por session_id com check de revoked_at (preparacao para Fase 7B).
--   2. Roda ANALYZE nas tabelas core.* e legadas que estao no hot-path.

-- ============================================================================
-- Indice auxiliar para Fase 7B (lookup de sessao por id com revogacao)
-- ============================================================================

create index if not exists core_user_sessions_id_active_idx
    on core.user_sessions (id)
    where revoked_at is null;

-- ============================================================================
-- ANALYZE para refrescar estatisticas do planner
-- ============================================================================

analyze core.users;
analyze core.accounts;
analyze core.account_users;
analyze core.user_role_assignments;
analyze core.role_permissions;
analyze core.user_permission_overrides;
analyze core.account_modules;
analyze core.permissions;
analyze core.roles;
analyze core.user_sessions;

analyze users;
analyze user_platform_roles;
analyze user_tenant_roles;
analyze user_store_roles;
analyze access_role_permissions;
analyze user_access_overrides;
