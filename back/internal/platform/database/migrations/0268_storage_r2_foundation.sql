-- Fundacao do storage de objetos compartilhado da plataforma.
--
-- O PostgreSQL e a fonte autoritativa de metadados, reservas e consumo. Os
-- bytes ficam exclusivamente no bucket privado Cloudflare R2 Standard.

create schema if not exists storage;

create table if not exists storage.provider_state (
    provider text primary key,
    account_identifier text not null,
    bucket_name text not null,
    initialized_at timestamptz not null default now(),
    checked_at timestamptz not null default now(),
    check (provider = 'cloudflare_r2'),
    check (account_identifier <> ''),
    check (bucket_name <> '')
);

create table if not exists storage.monthly_usage (
    billing_month date primary key,
    class_a_requests bigint not null default 0 check (class_a_requests >= 0),
    class_b_requests bigint not null default 0 check (class_b_requests >= 0),
    uploaded_bytes bigint not null default 0 check (uploaded_bytes >= 0),
    created_at timestamptz not null default now(),
    updated_at timestamptz not null default now(),
    check (billing_month = date_trunc('month', billing_month)::date)
);

create table if not exists storage.settings (
    id smallint primary key default 1 check (id = 1),
    storage_limit_bytes bigint not null default 9000000000
        check (storage_limit_bytes > 0 and storage_limit_bytes <= 10000000000),
    class_a_limit bigint not null default 900000
        check (class_a_limit > 0 and class_a_limit <= 1000000),
    class_b_limit bigint not null default 9000000
        check (class_b_limit > 0 and class_b_limit <= 10000000),
    max_object_bytes bigint not null default 26214400
        check (max_object_bytes > 0 and max_object_bytes <= 536870912),
    updated_by uuid references core.users(id) on delete set null,
    updated_at timestamptz not null default now(),
    check (max_object_bytes <= storage_limit_bytes)
);

insert into storage.settings (id)
values (1)
on conflict (id) do nothing;

create table if not exists storage.objects (
    id text primary key,
    account_id uuid not null references core.accounts(id) on delete restrict,
    source_module text not null,
    idempotency_key text not null,
    object_key text not null unique,
    file_name text not null,
    content_type text not null,
    size_bytes bigint not null check (size_bytes > 0),
    etag text not null default '',
    status text not null default 'pending'
        check (status in ('pending', 'available', 'failed', 'deleted')),
    created_by uuid not null references core.users(id) on delete restrict,
    created_at timestamptz not null default now(),
    available_at timestamptz,
    failed_at timestamptz,
    deleted_at timestamptz,
    unique (account_id, source_module, idempotency_key),
    check (source_module ~ '^[a-z][a-z0-9_]{0,62}$'),
    check (idempotency_key <> ''),
    check (object_key <> ''),
    check (file_name <> ''),
    check (content_type <> '')
);

create index if not exists storage_objects_account_status_created_idx
    on storage.objects (account_id, status, created_at desc, id desc);

create index if not exists storage_objects_pending_idx
    on storage.objects (created_at, id)
    where status = 'pending';

comment on table storage.monthly_usage is
    'Ledger global conservador das operacoes R2 mediadas pelo Omni; tentativas contam mesmo quando o provider falha.';
comment on table storage.settings is
    'Configuracao global autoritativa dos limites R2 editaveis por platform_admin; tetos absolutos preservam o free tier.';
comment on table storage.objects is
    'Metadados tenant-scoped dos objetos; bytes permanecem no bucket privado R2 Standard.';
comment on column storage.objects.idempotency_key is
    'Chave do chamador, unica por account e modulo, que impede upload duplicado por retry.';
