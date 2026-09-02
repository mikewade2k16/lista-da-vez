-- E9: estado efemero compartilhado entre replicas da API.
--
-- UNLOGGED evita WAL/backup para QR e buckets de rate limit. Nenhuma tabela contem IP,
-- telefone ou payload de webhook: bucket_key e SHA-256 de escopo+IP. O QR expira em 120s
-- e e removido ao conectar/logout; perda em crash apenas exige novo pareamento.

create unlogged table if not exists messaging.runtime_rate_limit_buckets (
    bucket_key     bytea primary key,
    hits           integer not null,
    window_started timestamptz not null,
    expires_at     timestamptz not null,
    constraint messaging_runtime_rate_limit_hits_ck check (hits >= 0)
);

create index if not exists messaging_runtime_rate_limit_expiry_idx
    on messaging.runtime_rate_limit_buckets (expires_at);

create unlogged table if not exists messaging.runtime_qr_cache (
    account_id    uuid not null,
    instance_name text not null,
    data_url      text not null,
    expires_at    timestamptz not null,
    updated_at    timestamptz not null default now(),
    primary key (account_id, instance_name),
    constraint messaging_runtime_qr_cache_size_ck check (octet_length(data_url) <= 1048576)
);

create index if not exists messaging_runtime_qr_cache_expiry_idx
    on messaging.runtime_qr_cache (expires_at);
