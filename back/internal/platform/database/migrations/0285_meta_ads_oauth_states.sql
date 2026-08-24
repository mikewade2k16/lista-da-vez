-- OAuth de primeira parte do Meta Ads. O state bruto nunca e persistido:
-- somente SHA-256, com expiracao curta e consumo atomico/single-use.

create table if not exists meta_ads.oauth_states (
    id uuid primary key default gen_random_uuid(),
    account_id uuid not null references core.accounts(id) on delete cascade,
    created_by_user_id uuid not null references core.users(id) on delete cascade,
    state_hash bytea not null,
    redirect_uri text not null,
    expires_at timestamptz not null,
    consumed_at timestamptz,
    created_at timestamptz not null default now(),
    constraint meta_ads_oauth_states_hash_unique unique (state_hash),
    constraint meta_ads_oauth_states_hash_size check (octet_length(state_hash) = 32),
    constraint meta_ads_oauth_states_expiry check (expires_at > created_at)
);

create index if not exists meta_ads_oauth_states_pending_idx
    on meta_ads.oauth_states (expires_at)
    where consumed_at is null;

create index if not exists meta_ads_oauth_states_account_created_idx
    on meta_ads.oauth_states (account_id, created_at desc);
