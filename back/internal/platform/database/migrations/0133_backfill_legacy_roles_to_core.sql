-- 0133 - U2: backfill additive dos papeis legados para core.*
--
-- Idempotente e transitivo: nao remove tabelas legadas nem apaga dados.
-- O auth passa a poder reconstruir o papel coarse a partir de core.roles.code.
--
-- Mapeamento legado -> core role:
--   owner          -> queue.owner          (template queue.supervisor)
--   director       -> queue.director       (template queue.supervisor)
--   marketing      -> queue.marketing      (template queue.consultant)
--   manager        -> queue.manager        (template queue.supervisor)
--   consultant     -> queue.consultant     (template queue.consultant)
--   store_terminal -> queue.store_terminal (template queue.supervisor)

-- Roles core especificos da compatibilidade U2, clonados dos templates queue.
with legacy_role_map (legacy_role, core_code, template_id, label, description) as (
    values
        ('owner', 'queue.owner', 'queue.supervisor', 'Proprietario da Fila', 'Compatibilidade U2 para role legado owner.'),
        ('director', 'queue.director', 'queue.supervisor', 'Diretoria da Fila', 'Compatibilidade U2 para role legado director.'),
        ('marketing', 'queue.marketing', 'queue.consultant', 'Marketing da Fila', 'Compatibilidade U2 para role legado marketing.'),
        ('manager', 'queue.manager', 'queue.supervisor', 'Gerente da Fila', 'Compatibilidade U2 para role legado manager.'),
        ('consultant', 'queue.consultant', 'queue.consultant', 'Consultor', 'Acesso ao dashboard e operacoes da propria fila.'),
        ('store_terminal', 'queue.store_terminal', 'queue.supervisor', 'Terminal de Loja', 'Compatibilidade U2 para role legado store_terminal.')
)
insert into core.roles (
    account_id,
    cloned_from_template_id,
    code,
    label,
    description,
    is_locked
)
select
    a.id,
    rt.id,
    m.core_code,
    m.label,
    m.description,
    rt.is_locked
from core.accounts a
join legacy_role_map m on true
join core.role_templates rt on rt.id = m.template_id
where a.is_active = true
on conflict (account_id, code) do nothing;

-- Permissoes dos templates para os roles inseridos acima.
with legacy_role_map (core_code, template_id) as (
    values
        ('queue.owner', 'queue.supervisor'),
        ('queue.director', 'queue.supervisor'),
        ('queue.marketing', 'queue.consultant'),
        ('queue.manager', 'queue.supervisor'),
        ('queue.consultant', 'queue.consultant'),
        ('queue.store_terminal', 'queue.supervisor')
)
insert into core.role_permissions (role_id, permission_key)
select
    r.id,
    rtp.permission_key
from core.roles r
join legacy_role_map m on m.core_code = r.code
join core.role_template_permissions rtp on rtp.role_template_id = m.template_id
on conflict do nothing;

-- Memberships por roles tenant-scoped legados.
insert into core.account_users (account_id, user_id, is_active, joined_at)
select distinct
    utr.tenant_id,
    utr.user_id,
    true,
    min(utr.created_at)
from public.user_tenant_roles utr
where exists (select 1 from core.accounts a where a.id = utr.tenant_id)
  and exists (select 1 from core.users u where u.id = utr.user_id)
group by utr.tenant_id, utr.user_id
on conflict (account_id, user_id) do nothing;

-- Memberships por roles store-scoped legados.
insert into core.account_users (account_id, user_id, is_active, joined_at)
select distinct
    s.tenant_id,
    usr.user_id,
    true,
    min(usr.created_at)
from public.user_store_roles usr
join queue.stores s on s.id = usr.store_id
where exists (select 1 from core.accounts a where a.id = s.tenant_id)
  and exists (select 1 from core.users u where u.id = usr.user_id)
group by s.tenant_id, usr.user_id
on conflict (account_id, user_id) do nothing;

-- Platform admins legados reconciliados no core.
update core.users u
set
    is_platform_admin = true,
    updated_at = now()
where exists (
    select 1
    from public.user_platform_roles upr
    where upr.user_id = u.id
)
  and u.is_platform_admin = false;

-- Role assignments tenant-scoped.
with legacy_role_map (legacy_role, core_code) as (
    values
        ('owner', 'queue.owner'),
        ('director', 'queue.director'),
        ('marketing', 'queue.marketing')
)
insert into core.user_role_assignments (account_id, user_id, role_id)
select
    utr.tenant_id,
    utr.user_id,
    r.id
from public.user_tenant_roles utr
join legacy_role_map m on m.legacy_role = utr.role
join core.roles r on r.account_id = utr.tenant_id and r.code = m.core_code
where exists (
    select 1
    from core.account_users au
    where au.account_id = utr.tenant_id
      and au.user_id = utr.user_id
      and au.is_active = true
)
on conflict (account_id, user_id, role_id) do nothing;

-- Role assignments store-scoped.
with legacy_role_map (legacy_role, core_code) as (
    values
        ('manager', 'queue.manager'),
        ('consultant', 'queue.consultant'),
        ('store_terminal', 'queue.store_terminal')
)
insert into core.user_role_assignments (account_id, user_id, role_id)
select distinct
    s.tenant_id,
    usr.user_id,
    r.id
from public.user_store_roles usr
join queue.stores s on s.id = usr.store_id
join legacy_role_map m on m.legacy_role = usr.role
join core.roles r on r.account_id = s.tenant_id and r.code = m.core_code
where exists (
    select 1
    from core.account_users au
    where au.account_id = s.tenant_id
      and au.user_id = usr.user_id
      and au.is_active = true
)
on conflict (account_id, user_id, role_id) do nothing;

-- Escopo de lojas store-scoped em config core do modulo queue.
-- Formato: {"storeIdsByAccount": {"<account_id>": ["<store_id>", ...]}}
with store_scope as (
    select
        usr.user_id,
        s.tenant_id as account_id,
        to_jsonb(array_agg(distinct usr.store_id::text order by usr.store_id::text)) as store_ids
    from public.user_store_roles usr
    join queue.stores s on s.id = usr.store_id
    where exists (select 1 from core.users u where u.id = usr.user_id)
    group by usr.user_id, s.tenant_id
),
user_config as (
    select
        user_id,
        jsonb_build_object(
            'storeIdsByAccount',
            jsonb_object_agg(account_id::text, store_ids)
        ) as config
    from store_scope
    group by user_id
)
insert into core.user_module_settings (user_id, module_id, config, created_at, updated_at)
select
    user_id,
    'queue',
    config,
    now(),
    now()
from user_config
on conflict (user_id, module_id) do update
set
    config = jsonb_set(
        core.user_module_settings.config,
        '{storeIdsByAccount}',
        coalesce(core.user_module_settings.config->'storeIdsByAccount', '{}'::jsonb)
            || coalesce(excluded.config->'storeIdsByAccount', '{}'::jsonb),
        true
    ),
    updated_at = now()
where not (
    coalesce(core.user_module_settings.config->'storeIdsByAccount', '{}'::jsonb)
        @> coalesce(excluded.config->'storeIdsByAccount', '{}'::jsonb)
);
