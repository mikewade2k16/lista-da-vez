-- Fase 12 / Tasks T1 — schema tasks
--
-- Cria a fundacao multi-tenant do orquestrador de tarefas. O frontend ainda
-- usa localStorage nesta fase, mas o schema ja nasce completo para API, RBAC,
-- tracking server-side, shares com cliente, relations cross-module, audit e
-- preparacao futura para documentos colaborativos.

create schema if not exists tasks;

-- ============================================================================
-- Boards / configuracao da base
-- ============================================================================

create table if not exists tasks.boards (
    id uuid primary key default gen_random_uuid(),
    account_id uuid not null references core.accounts(id) on delete cascade,
    organization_id uuid references core.organizations(id) on delete set null,
    slug text not null,
    name text not null,
    description text not null default '',
    icon text not null default '',
    archived boolean not null default false,
    created_by_user_id uuid not null references core.users(id),
    created_at timestamptz not null default now(),
    updated_at timestamptz not null default now(),
    constraint tasks_boards_slug_unique unique (account_id, slug),
    constraint tasks_boards_id_account_unique unique (id, account_id)
);

create index if not exists tasks_boards_account_idx
    on tasks.boards (account_id, archived, updated_at desc);

create table if not exists tasks.columns (
    id uuid primary key default gen_random_uuid(),
    board_id uuid not null references tasks.boards(id) on delete cascade,
    label text not null,
    color text not null default 'slate',
    sort_order integer not null default 100,
    created_at timestamptz not null default now()
);

create index if not exists tasks_columns_board_idx
    on tasks.columns (board_id, sort_order, created_at);

create table if not exists tasks.fields (
    id uuid primary key default gen_random_uuid(),
    board_id uuid not null references tasks.boards(id) on delete cascade,
    key text not null,
    label text not null,
    type text not null check (type in (
        'title', 'text', 'select', 'multiSelect', 'status', 'person', 'client',
        'date', 'priority', 'number', 'checkbox', 'image', 'location',
        'url', 'email', 'phone'
    )),
    required boolean not null default false,
    hidden boolean not null default false,
    sort_order integer not null default 100,
    config jsonb not null default '{}'::jsonb,
    constraint tasks_fields_key_unique unique (board_id, key)
);

create index if not exists tasks_fields_board_idx
    on tasks.fields (board_id, sort_order);

create table if not exists tasks.field_options (
    id uuid primary key default gen_random_uuid(),
    field_id uuid not null references tasks.fields(id) on delete cascade,
    value text not null,
    label text not null,
    color text not null default 'slate',
    sort_order integer not null default 100,
    constraint tasks_field_options_value_unique unique (field_id, value)
);

create index if not exists tasks_field_options_field_idx
    on tasks.field_options (field_id, sort_order);

-- ============================================================================
-- Views e layouts
-- ============================================================================

create table if not exists tasks.views (
    id uuid primary key default gen_random_uuid(),
    board_id uuid not null references tasks.boards(id) on delete cascade,
    name text not null,
    type text not null check (type in (
        'board', 'table', 'timeline', 'calendar', 'list', 'gallery',
        'chart', 'feed', 'map', 'dashboard', 'custom'
    )),
    scope text not null default 'board' check (scope in ('board', 'user')),
    owner_user_id uuid references core.users(id) on delete cascade,
    config jsonb not null default '{}'::jsonb,
    sort_order integer not null default 100,
    created_at timestamptz not null default now(),
    updated_at timestamptz not null default now()
);

create index if not exists tasks_views_board_idx
    on tasks.views (board_id, scope, sort_order);

create table if not exists tasks.view_widgets (
    id uuid primary key default gen_random_uuid(),
    view_id uuid not null references tasks.views(id) on delete cascade,
    widget_type text not null check (widget_type in (
        'count', 'chart', 'list', 'text', 'progress', 'calendar_mini', 'feed_mini'
    )),
    title text not null default '',
    position jsonb not null default '{"x":0,"y":0,"w":4,"h":3}'::jsonb,
    config jsonb not null default '{}'::jsonb
);

create index if not exists tasks_view_widgets_view_idx
    on tasks.view_widgets (view_id);

-- ============================================================================
-- Tasks / valores custom
-- ============================================================================

create table if not exists tasks.tasks (
    id uuid primary key default gen_random_uuid(),
    account_id uuid not null references core.accounts(id) on delete cascade,
    board_id uuid not null,
    column_id uuid references tasks.columns(id) on delete set null,
    title text not null,
    content_html text not null default '',
    status text,
    priority text not null default 'media' check (priority in ('baixa', 'media', 'alta')),
    due_date timestamptz,
    start_date timestamptz,
    archived boolean not null default false,
    sort_order numeric(20,5) not null default 0,
    created_by_user_id uuid not null references core.users(id),
    responsible_user_id uuid references core.users(id),
    client_account_id uuid references core.accounts(id) on delete set null,
    version integer not null default 0,
    created_at timestamptz not null default now(),
    updated_at timestamptz not null default now(),
    constraint tasks_tasks_board_account_fk
        foreign key (board_id, account_id)
        references tasks.boards(id, account_id)
        on delete cascade
);

create index if not exists tasks_tasks_account_board_idx
    on tasks.tasks (account_id, board_id, archived, updated_at desc);

create index if not exists tasks_tasks_due_date_idx
    on tasks.tasks (account_id, due_date)
    where archived = false;

create index if not exists tasks_tasks_responsible_idx
    on tasks.tasks (responsible_user_id);

create index if not exists tasks_tasks_client_idx
    on tasks.tasks (client_account_id);

create index if not exists tasks_tasks_column_order_idx
    on tasks.tasks (board_id, column_id, sort_order, created_at);

create table if not exists tasks.field_values (
    task_id uuid not null references tasks.tasks(id) on delete cascade,
    field_id uuid not null references tasks.fields(id) on delete cascade,
    value_text text,
    value_number numeric,
    value_date timestamptz,
    value_bool boolean,
    value_json jsonb,
    primary key (task_id, field_id)
);

create index if not exists tasks_field_values_field_idx
    on tasks.field_values (field_id);

create table if not exists tasks.task_assignees (
    task_id uuid not null references tasks.tasks(id) on delete cascade,
    user_id uuid not null references core.users(id) on delete cascade,
    share_with_client boolean not null default false,
    primary key (task_id, user_id)
);

create index if not exists tasks_task_assignees_user_idx
    on tasks.task_assignees (user_id);

create table if not exists tasks.task_subscribers (
    task_id uuid not null references tasks.tasks(id) on delete cascade,
    user_id uuid not null references core.users(id) on delete cascade,
    primary key (task_id, user_id)
);

create index if not exists tasks_task_subscribers_user_idx
    on tasks.task_subscribers (user_id);

-- ============================================================================
-- Comentarios / mentions
-- ============================================================================

create table if not exists tasks.task_comments (
    id uuid primary key default gen_random_uuid(),
    task_id uuid not null references tasks.tasks(id) on delete cascade,
    author_user_id uuid not null references core.users(id),
    body_html text not null,
    created_at timestamptz not null default now(),
    updated_at timestamptz not null default now(),
    deleted_at timestamptz
);

create index if not exists tasks_task_comments_task_idx
    on tasks.task_comments (task_id, created_at);

create table if not exists tasks.task_mentions (
    id uuid primary key default gen_random_uuid(),
    task_id uuid not null references tasks.tasks(id) on delete cascade,
    comment_id uuid references tasks.task_comments(id) on delete cascade,
    mentioned_user_id uuid not null references core.users(id),
    created_at timestamptz not null default now()
);

create index if not exists tasks_task_mentions_user_idx
    on tasks.task_mentions (mentioned_user_id, created_at desc);

-- ============================================================================
-- Tracking server-side
-- ============================================================================

create table if not exists tasks.task_time_entries (
    id uuid primary key default gen_random_uuid(),
    task_id uuid not null references tasks.tasks(id) on delete cascade,
    user_id uuid not null references core.users(id),
    account_id uuid not null references core.accounts(id) on delete cascade,
    started_at timestamptz not null,
    paused_at timestamptz,
    resumed_at timestamptz,
    stopped_at timestamptz,
    duration_ms bigint not null default 0,
    notes text not null default '',
    version integer not null default 0,
    created_at timestamptz not null default now(),
    updated_at timestamptz not null default now()
);

create unique index if not exists tasks_time_entries_one_active_per_user_task
    on tasks.task_time_entries (user_id, task_id)
    where stopped_at is null;

create index if not exists tasks_time_entries_user_idx
    on tasks.task_time_entries (user_id, stopped_at);

create index if not exists tasks_time_entries_task_idx
    on tasks.task_time_entries (task_id, started_at desc);

create index if not exists tasks_time_entries_account_idx
    on tasks.task_time_entries (account_id, started_at desc);

-- ============================================================================
-- Relations / shares / audit
-- ============================================================================

create table if not exists tasks.task_relations (
    id uuid primary key default gen_random_uuid(),
    task_id uuid not null references tasks.tasks(id) on delete cascade,
    module text not null,
    resource_type text not null,
    resource_id text not null,
    label_cache text not null default '',
    metadata_cache jsonb not null default '{}'::jsonb,
    refreshed_at timestamptz not null default now(),
    constraint tasks_task_relations_unique unique (task_id, module, resource_type, resource_id)
);

create index if not exists tasks_task_relations_module_idx
    on tasks.task_relations (module, resource_type, resource_id);

create table if not exists tasks.task_shares (
    id uuid primary key default gen_random_uuid(),
    task_id uuid not null references tasks.tasks(id) on delete cascade,
    client_account_id uuid not null references core.accounts(id) on delete cascade,
    permission text not null check (permission in ('view', 'comment', 'edit')),
    shared_by_user_id uuid not null references core.users(id),
    created_at timestamptz not null default now(),
    revoked_at timestamptz
);

create unique index if not exists tasks_task_shares_active_unique
    on tasks.task_shares (task_id, client_account_id)
    where revoked_at is null;

create index if not exists tasks_task_shares_client_idx
    on tasks.task_shares (client_account_id)
    where revoked_at is null;

create table if not exists tasks.audit_log (
    id bigserial primary key,
    account_id uuid not null references core.accounts(id) on delete cascade,
    user_id uuid references core.users(id),
    action text not null,
    resource_type text not null,
    resource_id text not null,
    before jsonb,
    after jsonb,
    at timestamptz not null default now()
);

create index if not exists tasks_audit_account_at_idx
    on tasks.audit_log (account_id, at desc);

create index if not exists tasks_audit_resource_idx
    on tasks.audit_log (resource_type, resource_id, at desc);

-- Preparacao para Y.js/Tiptap colaborativo em fase futura.
create table if not exists tasks.task_doc_snapshots (
    task_id uuid not null references tasks.tasks(id) on delete cascade,
    snapshot bytea not null,
    version integer not null,
    saved_at timestamptz not null default now(),
    primary key (task_id, version)
);
