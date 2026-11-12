create table if not exists tools.script_orders (
 account_id uuid not null references core.accounts(id) on delete cascade,
 user_id uuid not null references core.users(id) on delete cascade,
 ids uuid[] not null default '{}',
 primary key (account_id,user_id)
);
create table if not exists tools.script_images (
 id uuid primary key,
 account_id uuid not null references core.accounts(id) on delete cascade,
 mime_type text not null,
 size_bytes bigint not null,
 created_at timestamptz not null default now()
);
create index if not exists tools_script_images_account_idx on tools.script_images(account_id);
