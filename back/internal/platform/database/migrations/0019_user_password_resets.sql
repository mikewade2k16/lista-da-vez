create table if not exists user_password_resets (
	id uuid primary key default gen_random_uuid(),
	user_id uuid not null references users(id) on delete cascade,
	email text not null,
	code_hash text not null,
	status text not null check (status in ('pending', 'consumed', 'revoked')),
	expires_at timestamptz not null,
	consumed_at timestamptz,
	revoked_at timestamptz,
	created_at timestamptz not null default now(),
	updated_at timestamptz not null default now()
);

create unique index if not exists user_password_resets_pending_user_uidx on user_password_resets (user_id) where status = 'pending';
create index if not exists user_password_resets_email_idx on user_password_resets (lower(email), created_at desc);
create index if not exists user_password_resets_lookup_idx on user_password_resets (lower(email), code_hash, created_at desc);