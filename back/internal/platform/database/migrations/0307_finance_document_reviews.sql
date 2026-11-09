create table if not exists finance.document_reviews (
 id uuid primary key default gen_random_uuid(),
 account_id uuid not null references core.accounts(id) on delete cascade,
 user_id uuid not null references core.users(id) on delete cascade,
 draft_id uuid not null references finance.document_drafts(id) on delete cascade,
 review_hash text not null,
 draft_version timestamptz not null,
 profile_version timestamptz not null,
 snapshot jsonb not null,
 approved_at timestamptz not null default clock_timestamp(),
 revoked_at timestamptz
);
create unique index if not exists finance_document_reviews_active_idx on finance.document_reviews(account_id,user_id,draft_id,review_hash) where revoked_at is null;
