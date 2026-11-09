create table if not exists finance.document_drafts (
 id uuid primary key default gen_random_uuid(),
 account_id uuid not null references core.accounts(id) on delete cascade,
 user_id uuid not null references core.users(id) on delete cascade,
 source_hash text not null,
 source_name text not null,
 extracted_text text not null,
 data jsonb not null,
 created_at timestamptz not null default clock_timestamp(),
 updated_at timestamptz not null default clock_timestamp(),
 unique(account_id,user_id,source_hash)
);
create index if not exists finance_document_drafts_owner_updated_idx on finance.document_drafts(account_id,user_id,updated_at desc);
