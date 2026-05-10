-- Reestruturacao multi-tenant — seed inicial do core a partir do legado
-- Branch: refactor/multi-tenant-core
-- Plano: docs/SCHEMA_TARGET.md secao 5 (Estado apos Fase 1)
--
-- Copia idempotente (usa ON CONFLICT DO NOTHING) de:
--   public.tenants          -> core.accounts          (mesmo id)
--   public.users            -> core.users             (mesmo id)
--   public.user_tenant_roles -> core.account_users    (membership)
--   public.user_store_roles  -> core.account_users    (membership via store.tenant_id)
--   public.user_platform_roles -> core.users.is_platform_admin
--
-- Como roles atuais sao hardcoded (Fase 3 cria RBAC dinamico), por enquanto
-- nao geramos core.roles nem core.user_role_assignments. Isso vem na Fase 3
-- a partir do mapeamento role legado -> template do modulo correspondente.

-- ----------------------------------------------------------------------------
-- accounts (a partir de public.tenants)
-- ----------------------------------------------------------------------------
insert into core.accounts (id, organization_id, slug, name, is_active, plan_code, created_at, updated_at)
select
    t.id,
    null::uuid,
    t.slug,
    t.name,
    t.is_active,
    'standard',
    t.created_at,
    t.updated_at
from public.tenants t
on conflict (id) do nothing;

-- ----------------------------------------------------------------------------
-- users globais (a partir de public.users)
-- ----------------------------------------------------------------------------
insert into core.users (
    id,
    email,
    display_name,
    password_hash,
    must_change_password,
    avatar_path,
    is_platform_admin,
    is_active,
    created_at,
    updated_at
)
select
    u.id,
    u.email,
    u.display_name,
    u.password_hash,
    coalesce(u.must_change_password, false),
    coalesce(u.avatar_path, ''),
    exists (
        select 1 from public.user_platform_roles upr where upr.user_id = u.id
    ),
    u.is_active,
    u.created_at,
    u.updated_at
from public.users u
on conflict (id) do nothing;

-- ----------------------------------------------------------------------------
-- account_users — membership a partir de user_tenant_roles
-- ----------------------------------------------------------------------------
insert into core.account_users (account_id, user_id, is_active, joined_at)
select distinct
    utr.tenant_id,
    utr.user_id,
    true,
    utr.created_at
from public.user_tenant_roles utr
where exists (select 1 from core.accounts a where a.id = utr.tenant_id)
  and exists (select 1 from core.users u where u.id = utr.user_id)
on conflict (account_id, user_id) do nothing;

-- ----------------------------------------------------------------------------
-- account_users — membership a partir de user_store_roles (via store.tenant_id)
-- ----------------------------------------------------------------------------
insert into core.account_users (account_id, user_id, is_active, joined_at)
select distinct
    s.tenant_id,
    usr.user_id,
    true,
    usr.created_at
from public.user_store_roles usr
join public.stores s on s.id = usr.store_id
where exists (select 1 from core.accounts a where a.id = s.tenant_id)
  and exists (select 1 from core.users u where u.id = usr.user_id)
on conflict (account_id, user_id) do nothing;

-- Nota: platform admins NAO recebem account_users automatico. Eles operam
-- globalmente. Se precisarem operar dentro de uma account, recebem
-- membership explicito via UI administrativa (Fase 3 em diante).
