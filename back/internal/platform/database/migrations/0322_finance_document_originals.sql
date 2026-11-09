-- CT-15 (P15-05/P15-33): original privado dos comprovantes e política de retenção.
-- O binário fica no storage privado (R2 ou disco privado local), nunca no PostgreSQL.
-- Os dados extraídos (finance.document_drafts), vínculos e históricos continuam no
-- banco; a retenção remove somente o arquivo original e registra quando isso ocorreu.

create table if not exists finance.document_originals (
    document_id uuid primary key references finance.document_drafts(id) on delete cascade,
    account_id uuid not null references core.accounts(id) on delete cascade,
    user_id uuid not null references core.users(id) on delete cascade,
    storage_ref text check (storage_ref is null or storage_ref ~ '^(r2|disk):[0-9a-f-]{36}$'),
    file_name text not null check (length(file_name) between 1 and 200),
    content_type text not null check (length(content_type) between 1 and 120),
    size_bytes bigint not null check (size_bytes > 0),
    sha256 text not null check (sha256 ~ '^[0-9a-f]{64}$'),
    -- stored | not_stored (storage indisponível/cota) | purged (prazo vencido) | removed (exclusão manual)
    status text not null check (status in ('stored', 'not_stored', 'purged', 'removed')),
    not_stored_reason text check (not_stored_reason is null or length(not_stored_reason) <= 80),
    stored_at timestamptz,
    purged_at timestamptz,
    purge_attempts integer not null default 0 check (purge_attempts >= 0),
    next_purge_attempt_at timestamptz,
    created_at timestamptz not null default clock_timestamp(),
    updated_at timestamptz not null default clock_timestamp(),
    check (status <> 'stored' or (storage_ref is not null and stored_at is not null and purged_at is null)),
    check (status <> 'purged' or purged_at is not null)
);

create index if not exists finance_document_originals_scope_idx
    on finance.document_originals(account_id, user_id);
create index if not exists finance_document_originals_purge_idx
    on finance.document_originals(account_id, stored_at) where status = 'stored';

-- Política por conta. Ausência de linha = padrão de 90 dias (3 meses), exibido como tal.
create table if not exists finance.retention_settings (
    account_id uuid primary key references core.accounts(id) on delete cascade,
    receipt_original_days integer not null check (receipt_original_days between 1 and 3650),
    version integer not null default 1 check (version >= 1),
    updated_by uuid not null references core.users(id) on delete restrict,
    updated_at timestamptz not null default clock_timestamp()
);

create table if not exists finance.retention_setting_events (
    id bigint generated always as identity primary key,
    account_id uuid not null references core.accounts(id) on delete cascade,
    version integer not null check (version >= 1),
    previous_days integer,
    receipt_original_days integer not null,
    actor_id uuid not null references core.users(id) on delete restrict,
    created_at timestamptz not null default clock_timestamp(),
    unique (account_id, version)
);

create or replace function finance.retention_setting_events_immutable() returns trigger
language plpgsql as $$
begin
    raise exception 'finance.retention_setting_events is append-only';
end;
$$;

drop trigger if exists retention_setting_events_immutable on finance.retention_setting_events;
create trigger retention_setting_events_immutable
    before update on finance.retention_setting_events
    for each row execute function finance.retention_setting_events_immutable();
