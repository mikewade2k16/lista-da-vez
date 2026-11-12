create table if not exists tools.scripts (
    id uuid primary key default gen_random_uuid(),
    account_id uuid not null references core.accounts(id) on delete cascade,
    title text not null,
    rows jsonb not null default '[]'::jsonb,
    description text not null default '',
    created_at timestamptz not null default now(),
    updated_at timestamptz not null default now()
);
create index if not exists tools_scripts_account_updated_idx on tools.scripts (account_id, updated_at desc, id desc);
