create table if not exists finance.user_fiscal_profiles (
    account_id uuid not null references core.accounts(id) on delete cascade,
    user_id uuid not null references core.users(id) on delete cascade,
    data jsonb not null default '{}'::jsonb,
    updated_at timestamptz not null default clock_timestamp(),
    primary key (account_id, user_id)
);

create table if not exists finance.user_fiscal_credentials (
    account_id uuid not null,
    user_id uuid not null,
    ciphertext text not null,
    updated_at timestamptz not null default clock_timestamp(),
    primary key (account_id, user_id),
    foreign key (account_id, user_id) references finance.user_fiscal_profiles(account_id, user_id) on delete cascade
);

create table if not exists finance.user_fiscal_templates (
    id uuid primary key default gen_random_uuid(),
    account_id uuid not null references core.accounts(id) on delete cascade,
    user_id uuid not null references core.users(id) on delete cascade,
    name text not null,
    data jsonb not null default '{}'::jsonb,
    is_default boolean not null default false,
    updated_at timestamptz not null default clock_timestamp()
);

create index if not exists finance_user_fiscal_templates_scope_idx
    on finance.user_fiscal_templates(account_id, user_id);
create unique index if not exists finance_user_fiscal_templates_default_idx
    on finance.user_fiscal_templates(account_id, user_id) where is_default;
