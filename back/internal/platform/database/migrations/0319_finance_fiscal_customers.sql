-- CT-15 pacote 4 (P15-03/P15-04): clientes (tomadores) recorrentes do emitente pessoal.
-- Mesmo escopo do perfil fiscal e dos templates: conta + usuário da sessão.
-- Versões são imutáveis; a linha atual guarda somente o último estado confirmado.
create table if not exists finance.fiscal_customers (
    id uuid primary key default gen_random_uuid(),
    account_id uuid not null references core.accounts(id) on delete cascade,
    user_id uuid not null references core.users(id) on delete cascade,
    name text not null check (length(name) between 1 and 300),
    document text not null check (document ~ '^(?:[0-9]{11}|[A-Z0-9]{12}[0-9]{2})$'),
    service_description text not null default '' check (length(service_description) <= 4000),
    -- Template used last time; validated against the user's templates on reuse (templates may be deleted).
    template_id uuid,
    amount numeric(14, 2) check (amount is null or amount > 0),
    version integer not null default 1 check (version >= 1),
    last_used_at timestamptz not null default clock_timestamp(),
    created_at timestamptz not null default clock_timestamp(),
    updated_at timestamptz not null default clock_timestamp()
);

create unique index if not exists finance_fiscal_customers_document_idx
    on finance.fiscal_customers(account_id, user_id, document);
create index if not exists finance_fiscal_customers_name_idx
    on finance.fiscal_customers(account_id, user_id, lower(name));

create table if not exists finance.fiscal_customer_versions (
    customer_id uuid not null references finance.fiscal_customers(id) on delete cascade,
    account_id uuid not null references core.accounts(id) on delete cascade,
    user_id uuid not null references core.users(id) on delete cascade,
    version integer not null check (version >= 1),
    data jsonb not null,
    source_draft_id uuid,
    created_at timestamptz not null default clock_timestamp(),
    primary key (customer_id, version)
);

create index if not exists finance_fiscal_customer_versions_scope_idx
    on finance.fiscal_customer_versions(account_id, user_id, customer_id);

create or replace function finance.fiscal_customer_versions_immutable() returns trigger
language plpgsql as $$
begin
    raise exception 'finance.fiscal_customer_versions is append-only';
end;
$$;

drop trigger if exists fiscal_customer_versions_immutable on finance.fiscal_customer_versions;
create trigger fiscal_customer_versions_immutable
    before update on finance.fiscal_customer_versions
    for each row execute function finance.fiscal_customer_versions_immutable();
