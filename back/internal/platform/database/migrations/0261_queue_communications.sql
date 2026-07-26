-- Comunicados exibidos no painel da Fila.
--
-- O registro pertence a uma account. Quando targets_all_stores=false, os
-- destinos ficam em queue.communication_stores e a FK composta impede que uma
-- loja de outra account seja vinculada por engano.

create unique index if not exists queue_stores_tenant_id_id_uidx
    on queue.stores (tenant_id, id);

create table if not exists queue.communications (
    id uuid primary key default gen_random_uuid(),
    account_id uuid not null,
    title text not null,
    excerpt text not null default '',
    body text not null,
    starts_at timestamptz,
    ends_at timestamptz,
    is_published boolean not null default true,
    display_order integer not null default 0,
    targets_all_stores boolean not null default true,
    created_by uuid not null,
    updated_by uuid not null,
    archived_at timestamptz,
    created_at timestamptz not null default now(),
    updated_at timestamptz not null default now(),
    unique (account_id, id),
    check (char_length(btrim(title)) between 1 and 160),
    check (char_length(excerpt) <= 300),
    check (char_length(btrim(body)) between 1 and 20000),
    check (display_order between -10000 and 10000),
    check (ends_at is null or starts_at is null or ends_at > starts_at)
);

comment on column queue.communications.account_id is
    'Escopo da account. Sem FK cross-schema por contrato do modulo queue.';
comment on column queue.communications.archived_at is
    'Exclusao logica: comunicados arquivados deixam de ser listados e exibidos.';

create table if not exists queue.communication_stores (
    account_id uuid not null,
    communication_id uuid not null,
    store_id uuid not null,
    created_at timestamptz not null default now(),
    primary key (account_id, communication_id, store_id),
    foreign key (account_id, communication_id)
        references queue.communications (account_id, id) on delete cascade,
    foreign key (account_id, store_id)
        references queue.stores (tenant_id, id) on delete cascade
);

create index if not exists queue_communications_account_list_idx
    on queue.communications (
        account_id,
        is_published,
        display_order,
        updated_at desc
    )
    where archived_at is null;

create index if not exists queue_communication_stores_lookup_idx
    on queue.communication_stores (account_id, store_id, communication_id);
