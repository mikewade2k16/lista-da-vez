create table if not exists user_feedback (
    id uuid primary key default gen_random_uuid(),
    tenant_id uuid not null references tenants(id),
    store_id uuid not null references stores(id),
    user_id uuid not null references users(id),
    user_name text not null default '',
    kind text not null check (kind in ('suggestion', 'question', 'problem')),
    status text not null default 'open'
        check (status in ('open', 'in_progress', 'resolved', 'closed')),
    subject text not null,
    body text not null,
    admin_note text not null default '',
    created_at timestamptz not null default now(),
    updated_at timestamptz not null default now()
);

create index if not exists user_feedback_tenant_id_idx on user_feedback (tenant_id);
create index if not exists user_feedback_store_id_idx on user_feedback (store_id);
create index if not exists user_feedback_status_idx on user_feedback (status);
create index if not exists user_feedback_kind_idx on user_feedback (kind);
create index if not exists user_feedback_created_at_idx on user_feedback (created_at desc);
