-- 0131 — Re-backfill idempotente public.users -> core.users (+ memberships).
-- Branch: refactor/multi-tenant-complete (workstream de unificacao de usuarios).
--
-- POR QUE: a 0101 copiou public.users -> core.users UMA vez, no boot da fundacao
-- multi-tenant. Usuarios criados DEPOIS pela Fila (consultores/terminais, que
-- escrevem em public.users) nunca foram re-sincronizados -> sumiam do
-- /manage/users (le core.users) e nao tinham membership em core.account_users
-- (logo /v2/me/accounts vinha vazio e o menu nao filtrava por modulo).
--
-- Esta migration re-roda os 3 inserts da 0101 (idempotentes via ON CONFLICT),
-- capturando o drift acumulado. ADITIVO e seguro — nao apaga nada. O sync
-- continuo (para nao divergir de novo) e a etapa estrutural (public.users vira
-- VIEW sobre core.users) vivem no script manual:
--   back/internal/platform/database/manual/unify_users_view.sql
-- aplicado deliberadamente apos backup.

-- ----------------------------------------------------------------------------
-- 1. core.users — identidade global a partir de public.users (drift)
-- ----------------------------------------------------------------------------
insert into core.users (
    id, email, display_name, password_hash, must_change_password,
    avatar_path, is_platform_admin, is_active, nick, created_at, updated_at
)
select
    u.id,
    u.email,
    u.display_name,
    u.password_hash,
    coalesce(u.must_change_password, false),
    coalesce(u.avatar_path, ''),
    exists (select 1 from public.user_platform_roles upr where upr.user_id = u.id),
    u.is_active,
    coalesce(u.nick, ''),
    u.created_at,
    u.updated_at
from public.users u
on conflict (id) do nothing;

-- ----------------------------------------------------------------------------
-- 2. core.account_users — membership a partir de user_tenant_roles
--    (account_id = tenant_id, pois core.accounts.id == public.tenants.id na 0101)
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
-- 3. core.account_users — membership a partir de user_store_roles (via store.tenant_id)
--    Cobre consultores/terminais vinculados a loja sem tenant_role explicito.
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

-- ----------------------------------------------------------------------------
-- 4. core.account_users — membership a partir de consultants.tenant_id
--    Consultores vinculados DIRETO ao tenant (coluna consultants.tenant_id),
--    sem user_store_roles/user_tenant_roles, eram perdidos pelos inserts 2/3.
--    Cobre consultores reais de uma conta (ex.: 6 da Pérola no diagnostico 2026-06).
-- ----------------------------------------------------------------------------
insert into core.account_users (account_id, user_id, is_active, joined_at)
select distinct
    c.tenant_id,
    c.user_id,
    true,
    coalesce(c.created_at, now())
from public.consultants c
where c.user_id  is not null
  and c.tenant_id is not null
  and exists (select 1 from core.accounts a where a.id = c.tenant_id)
  and exists (select 1 from core.users   u where u.id = c.user_id)
on conflict (account_id, user_id) do nothing;
