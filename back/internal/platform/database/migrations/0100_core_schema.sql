-- Reestruturacao multi-tenant — schema core
-- Branch: refactor/multi-tenant-core
-- Plano: docs/SCHEMA_TARGET.md secao 3
--
-- Esta migration cria o schema `core` paralelo ao legado em `public`.
-- Ate a Fase 4, public.* continua intocado e em uso ativo. A Fase 1
-- so adiciona as estruturas novas; a copia de dados acontece em 0101.

create schema if not exists core;

-- ============================================================================
-- Identidade e tenancy
-- ============================================================================

-- Agencia (opcional). Agrupa accounts.
create table if not exists core.organizations (
    id uuid primary key default gen_random_uuid(),
    slug text not null,
    name text not null,
    is_active boolean not null default true,
    created_at timestamptz not null default now(),
    updated_at timestamptz not null default now()
);

create unique index if not exists core_organizations_slug_uidx on core.organizations (lower(slug));

-- Cliente do SaaS. Substitui public.tenants.
create table if not exists core.accounts (
    id uuid primary key default gen_random_uuid(),
    organization_id uuid references core.organizations(id) on delete set null,
    slug text not null,
    name text not null,
    is_active boolean not null default true,
    plan_code text not null default 'standard',
    created_at timestamptz not null default now(),
    updated_at timestamptz not null default now()
);

create unique index if not exists core_accounts_slug_uidx on core.accounts (lower(slug));
create index if not exists core_accounts_organization_id_idx on core.accounts (organization_id);

-- Identidade global. 1 e-mail = 1 user. Sem account_id.
create table if not exists core.users (
    id uuid primary key default gen_random_uuid(),
    email text not null,
    display_name text not null,
    password_hash text,
    must_change_password boolean not null default false,
    avatar_path text not null default '',
    is_platform_admin boolean not null default false,
    is_active boolean not null default true,
    created_at timestamptz not null default now(),
    updated_at timestamptz not null default now()
);

create unique index if not exists core_users_email_lower_uidx on core.users (lower(email));

-- Membership user <-> account.
create table if not exists core.account_users (
    account_id uuid not null references core.accounts(id) on delete cascade,
    user_id uuid not null references core.users(id) on delete cascade,
    is_active boolean not null default true,
    invited_by_user_id uuid references core.users(id) on delete set null,
    joined_at timestamptz not null default now(),
    primary key (account_id, user_id)
);

create index if not exists core_account_users_user_id_idx on core.account_users (user_id);

-- Membership user <-> organization (modo agencia).
create table if not exists core.organization_users (
    organization_id uuid not null references core.organizations(id) on delete cascade,
    user_id uuid not null references core.users(id) on delete cascade,
    org_role text not null check (org_role in ('agency_owner', 'agency_member')),
    joined_at timestamptz not null default now(),
    primary key (organization_id, user_id)
);

create index if not exists core_organization_users_user_id_idx on core.organization_users (user_id);

-- Sessoes ativas. JWT carrega sessionId; revoked_at faz logout funcionar.
create table if not exists core.user_sessions (
    id uuid primary key default gen_random_uuid(),
    user_id uuid not null references core.users(id) on delete cascade,
    revoked_at timestamptz,
    last_seen_at timestamptz not null default now(),
    user_agent text not null default '',
    ip text not null default '',
    created_at timestamptz not null default now()
);

create index if not exists core_user_sessions_user_id_idx on core.user_sessions (user_id);
create index if not exists core_user_sessions_active_idx on core.user_sessions (user_id) where revoked_at is null;

-- ============================================================================
-- Module Registry
-- ============================================================================

-- Catalogo de modulos disponiveis na plataforma. Populado pelo SyncCatalog
-- no boot a partir do Module Registry (Fase 2).
create table if not exists core.modules (
    id text primary key,
    schema_name text not null,
    label text not null,
    description text not null default '',
    is_core boolean not null default false,
    requires_modules text[] not null default '{}',
    optional_modules text[] not null default '{}',
    sort_order integer not null default 100,
    created_at timestamptz not null default now(),
    updated_at timestamptz not null default now()
);

-- Quais modulos cada Account tem habilitados. Controla rotas e menu.
create table if not exists core.account_modules (
    account_id uuid not null references core.accounts(id) on delete cascade,
    module_id text not null references core.modules(id) on delete restrict,
    enabled boolean not null default true,
    enabled_at timestamptz not null default now(),
    config jsonb not null default '{}'::jsonb,
    primary key (account_id, module_id)
);

create index if not exists core_account_modules_module_id_idx on core.account_modules (module_id);

-- ============================================================================
-- RBAC declarativo
-- ============================================================================

-- Catalogo de permissoes declaradas pelos modulos. SyncCatalog popula no boot.
-- deprecated_at marca remocoes (nunca DELETE automatico).
create table if not exists core.permissions (
    key text primary key,
    module_id text not null references core.modules(id) on delete cascade,
    label text not null,
    description text not null default '',
    scope text not null check (scope in ('account', 'store', 'platform')) default 'account',
    deprecated_at timestamptz,
    created_at timestamptz not null default now(),
    updated_at timestamptz not null default now()
);

create index if not exists core_permissions_module_id_idx on core.permissions (module_id);

-- Templates de cargo declarados pelos modulos (Owner, Admin, Operacional, ...).
create table if not exists core.role_templates (
    id text primary key,
    module_id text not null references core.modules(id) on delete cascade,
    label text not null,
    description text not null default '',
    is_system boolean not null default true,
    sort_order integer not null default 100,
    created_at timestamptz not null default now(),
    updated_at timestamptz not null default now()
);

create index if not exists core_role_templates_module_id_idx on core.role_templates (module_id);

-- Matriz template -> permissoes. SyncCatalog so popula em template novo;
-- nunca sobrescreve template ja existente.
create table if not exists core.role_template_permissions (
    role_template_id text not null references core.role_templates(id) on delete cascade,
    permission_key text not null references core.permissions(key) on delete cascade,
    primary key (role_template_id, permission_key)
);

-- Cargos efetivos da Account. Clones editaveis dos templates.
create table if not exists core.roles (
    id uuid primary key default gen_random_uuid(),
    account_id uuid not null references core.accounts(id) on delete cascade,
    cloned_from_template_id text references core.role_templates(id) on delete set null,
    code text not null,
    label text not null,
    description text not null default '',
    is_default boolean not null default false,
    is_locked boolean not null default false,
    created_at timestamptz not null default now(),
    updated_at timestamptz not null default now(),
    unique (account_id, code)
);

create index if not exists core_roles_account_id_idx on core.roles (account_id);

-- Permissoes efetivas por role. Editado pelo cliente (validado contra catalogo).
create table if not exists core.role_permissions (
    role_id uuid not null references core.roles(id) on delete cascade,
    permission_key text not null references core.permissions(key) on delete cascade,
    primary key (role_id, permission_key)
);

-- Atribuicao de cargo a usuario, sempre dentro de uma account.
create table if not exists core.user_role_assignments (
    id uuid primary key default gen_random_uuid(),
    account_id uuid not null references core.accounts(id) on delete cascade,
    user_id uuid not null references core.users(id) on delete cascade,
    role_id uuid not null references core.roles(id) on delete cascade,
    created_at timestamptz not null default now(),
    unique (account_id, user_id, role_id)
);

create index if not exists core_user_role_assignments_user_idx on core.user_role_assignments (user_id);
create index if not exists core_user_role_assignments_account_user_idx on core.user_role_assignments (account_id, user_id);

-- Overrides allow/deny por usuario. Convive com role.
create table if not exists core.user_permission_overrides (
    id uuid primary key default gen_random_uuid(),
    account_id uuid not null references core.accounts(id) on delete cascade,
    user_id uuid not null references core.users(id) on delete cascade,
    permission_key text not null references core.permissions(key) on delete cascade,
    effect text not null check (effect in ('allow', 'deny')),
    note text not null default '',
    is_active boolean not null default true,
    created_by_user_id uuid references core.users(id) on delete set null,
    created_at timestamptz not null default now(),
    updated_at timestamptz not null default now()
);

create index if not exists core_user_permission_overrides_lookup_idx
    on core.user_permission_overrides (user_id, account_id)
    where is_active = true;

create unique index if not exists core_user_permission_overrides_unique_active_uidx
    on core.user_permission_overrides (account_id, user_id, permission_key)
    where is_active = true;
