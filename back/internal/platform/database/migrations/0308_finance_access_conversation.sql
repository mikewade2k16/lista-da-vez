create table if not exists finance.portal_access_checks (
 account_id uuid not null references core.accounts(id) on delete cascade,
 user_id uuid not null references core.users(id) on delete cascade,
 profile_version text not null,
 status text not null check (status in ('authenticated','rejected','manual_required','unavailable','unsupported')),
 checked_at timestamptz not null default clock_timestamp(),
 primary key(account_id,user_id)
);
create table if not exists finance.access_messages (
 id uuid primary key default gen_random_uuid(),
 account_id uuid not null references core.accounts(id) on delete cascade,
 user_id uuid not null references core.users(id) on delete cascade,
 request_id uuid not null,
 role text not null check(role in ('user','assistant')),
 text text not null check(length(text)<=4000),
 action text not null default '',
 created_at timestamptz not null default clock_timestamp(),
 unique(account_id,user_id,request_id,role)
);
create index if not exists finance_access_messages_owner_created_idx on finance.access_messages(account_id,user_id,created_at desc,id desc);
