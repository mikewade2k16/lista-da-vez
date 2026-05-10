-- Fase 3 — RBAC dinâmico: seed de roles por account e migração de atribuições
--
-- PRÉ-REQUISITO: executar após a aplicação ter bootado pelo menos uma vez com
-- CORE_V2_ENABLED=true, para que core.role_templates esteja populado pelo
-- SyncCatalog. Se os templates ainda não existirem, os INSERTs produzem zero
-- linhas (CROSS JOIN vazio) e a migration passa sem erro — re-execute depois.

-- ============================================================================
-- 1. Seed de core.roles para todas as accounts ativas
--    Clona cada role_template do módulo 'core' em cada account que ainda
--    não tem o role (ON CONFLICT DO NOTHING = idempotente).
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
where rt.module_id = 'core'
  and a.is_active = true
on conflict (account_id, code) do nothing;

-- ============================================================================
-- 2. Seed de core.role_permissions para os roles recém-criados
--    (roles sem permissões = acabaram de ser inseridos no passo 1)
-- ============================================================================

insert into core.role_permissions (role_id, permission_key)
select r.id, rtp.permission_key
from core.roles r
join core.role_template_permissions rtp
    on rtp.role_template_id = r.cloned_from_template_id
where r.cloned_from_template_id is not null
on conflict do nothing;

-- ============================================================================
-- 3. Migração de public.user_tenant_roles → core.user_role_assignments
--    Mapeamento de roles legados para roles core:
--      owner    → core.owner  (dono da account)
--      director → core.admin  (gestão executiva)
--      marketing → core.member (acesso básico)
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
-- 4. Migração de public.user_store_roles → core.user_role_assignments
--    Usuários com role de loja (consultant, manager, store_terminal) recebem
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
