-- Multi-Tenant Completion (C1.3)
-- Branch: refactor/multi-tenant-complete
-- Plano: docs/MULTITENANT_COMPLETION_PLAN.md secao C1
--
-- PRE-REQUISITO: executar apos a aplicacao ter bootado pelo menos uma vez com
-- CORE_V2_ENABLED=true, para que core.role_templates / core.permissions /
-- core.role_template_permissions estejam populados pelo SyncCatalog.
--
-- POR QUE ESSA MIGRATION (RE-)EXISTE: a migration 0103 ja executa esse
-- backfill, mas em producao ela rodou ANTES do primeiro boot bem-sucedido
-- do SyncCatalog (catalogo vazio -> CROSS JOIN vazio -> no-op silencioso).
-- Resultado: core.user_role_assignments hoje tem 0 linhas em prod.
-- Essa migration repete o backfill agora que o catalogo esta vivo.
-- Tudo aqui e idempotente (ON CONFLICT DO NOTHING), entao re-executar e seguro.
--
-- DIFERENCAS vs 0103:
--   - 0103 clonava apenas templates do modulo 'core'
--   - 0125 clona templates de TODOS os modulos no catalogo
--     (incluindo queue, crm, tasks, etc.) para que cada account ja tenha
--     a matriz completa de roles disponivel quando o painel C10 abrir.

-- ============================================================================
-- 1. Clonar TODOS os role_templates em core.roles para cada account ativa
-- ============================================================================

insert into core.roles (
    account_id, cloned_from_template_id, code, label, description, is_locked
)
select
    a.id,
    rt.id,
    rt.id,
    rt.label,
    rt.description,
    rt.is_locked
from core.accounts a
cross join core.role_templates rt
where a.is_active = true
on conflict (account_id, code) do nothing;

-- ============================================================================
-- 2. Seed de core.role_permissions a partir do template clonado
--    (idempotente — ON CONFLICT cobre roles ja com permissoes)
-- ============================================================================

insert into core.role_permissions (role_id, permission_key)
select r.id, rtp.permission_key
from core.roles r
join core.role_template_permissions rtp
    on rtp.role_template_id = r.cloned_from_template_id
where r.cloned_from_template_id is not null
on conflict do nothing;

-- ============================================================================
-- 3. Migracao public.user_tenant_roles -> core.user_role_assignments
--    Mapeamento:
--      owner    -> core.owner
--      director -> core.admin
--      else     -> core.member
-- ============================================================================

insert into core.user_role_assignments (account_id, user_id, role_id)
select
    utr.tenant_id  as account_id,
    utr.user_id,
    r.id           as role_id
from public.user_tenant_roles utr
join core.roles r
    on  r.account_id = utr.tenant_id
    and r.code = case utr.role
                    when 'owner'    then 'core.owner'
                    when 'director' then 'core.admin'
                    else                 'core.member'
                 end
where exists (
    select 1 from core.account_users au
    where au.account_id = utr.tenant_id
      and au.user_id    = utr.user_id
      and au.is_active  = true
)
on conflict (account_id, user_id, role_id) do nothing;

-- ============================================================================
-- 4. Migracao public.user_store_roles -> core.user_role_assignments
--    Usuarios com role de loja (consultant, manager, store_terminal) recebem
--    core.member na account do tenant daquela loja.
-- ============================================================================

insert into core.user_role_assignments (account_id, user_id, role_id)
select distinct
    s.tenant_id    as account_id,
    usr.user_id,
    r.id           as role_id
from public.user_store_roles usr
join public.stores s
    on s.id = usr.store_id
join core.roles r
    on  r.account_id = s.tenant_id
    and r.code       = 'core.member'
where exists (
    select 1 from core.account_users au
    where au.account_id = s.tenant_id
      and au.user_id    = usr.user_id
      and au.is_active  = true
)
on conflict (account_id, user_id, role_id) do nothing;
