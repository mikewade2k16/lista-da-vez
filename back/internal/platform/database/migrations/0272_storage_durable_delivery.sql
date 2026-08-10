alter table storage.multipart_uploads
    add column if not exists creation_attempts integer not null default 0;

create table if not exists storage.multipart_deliveries (
    upload_id text primary key references storage.multipart_uploads(id) on delete cascade,
    account_id uuid not null references core.accounts(id) on delete restrict,
    source_module text not null,
    created_by uuid not null references core.users(id) on delete restrict,
    staging_path text not null,
    status text not null default 'queued'
        check (status in ('queued', 'uploading', 'retry', 'completed', 'failed')),
    attempts integer not null default 0 check (attempts >= 0),
    next_attempt_at timestamptz not null default now(),
    locked_at timestamptz,
    last_error text not null default '',
    created_at timestamptz not null default now(),
    updated_at timestamptz not null default now(),
    check (source_module ~ '^[a-z][a-z0-9_]{0,62}$'),
    check (staging_path <> '')
);

create index if not exists storage_multipart_deliveries_worker_idx
    on storage.multipart_deliveries (next_attempt_at, created_at)
    where status in ('queued', 'retry', 'uploading');

comment on table storage.multipart_deliveries is
    'Fila autoritativa de entrega assíncrona: o original permanece no staging durável até o R2 confirmar todas as partes.';
