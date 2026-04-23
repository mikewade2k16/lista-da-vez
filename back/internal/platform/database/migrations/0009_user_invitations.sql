alter table users
	alter column password_hash drop not null;

create table if not exists user_invitations (
	id uuid primary key default gen_random_uuid(),
	user_id uuid not null references users(id) on delete cascade,
	email text not null,
	invited_by_user_id uuid references users(id) on delete set null,
	token_hash text not null,
	status text not null check (status in ('pending', 'accepted', 'revoked')),
	expires_at timestamptz not null,
	accepted_at timestamptz,
	revoked_at timestamptz,
	created_at timestamptz not null default now(),
	updated_at timestamptz not null default now()
);

create unique index if not exists user_invitations_token_hash_uidx on user_invitations (token_hash);
create unique index if not exists user_invitations_pending_user_uidx on user_invitations (user_id) where status = 'pending';
create index if not exists user_invitations_user_id_idx on user_invitations (user_id, created_at desc);
create index if not exists user_invitations_email_idx on user_invitations (lower(email), created_at desc);
