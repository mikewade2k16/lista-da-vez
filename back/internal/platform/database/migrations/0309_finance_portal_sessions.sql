create table if not exists finance.portal_sessions (
 account_id uuid not null,
 user_id uuid not null,
 credentials_version text not null,
 ciphertext text not null,
 expires_at timestamptz not null,
 verified_at timestamptz not null default clock_timestamp(),
 primary key(account_id,user_id),
 foreign key(account_id,user_id) references finance.user_fiscal_profiles(account_id,user_id) on delete cascade
);
alter table finance.access_messages add column if not exists draft_id uuid references finance.document_drafts(id) on delete set null;
