-- Omnichannel: identidade multicanal, atribuicao e CRM automatico.
-- O contato e a entidade unica; telefone passa a ser opcional para permitir perfis do
-- Instagram. Identidades externas e touchpoints ficam normalizados e tenant-scoped.

alter table messaging.contacts
    alter column phone drop not null;

drop index if exists messaging.messaging_contacts_account_phone_uidx;
drop index if exists messaging_contacts_account_phone_uidx;
create unique index if not exists messaging_contacts_account_phone_uidx
    on messaging.contacts (account_id, phone)
    where phone is not null and phone <> '';

alter table messaging.contacts
    add column if not exists first_seen_at timestamptz,
    add column if not exists last_seen_at timestamptz,
    add column if not exists first_channel text,
    add column if not exists last_channel text,
    add column if not exists relationship_status text not null default 'lead',
    add column if not exists tags jsonb not null default '[]'::jsonb,
    add column if not exists custom_fields jsonb not null default '{}'::jsonb;

update messaging.contacts
set first_seen_at = coalesce(first_seen_at, created_at),
    last_seen_at = coalesce(last_seen_at, updated_at)
where first_seen_at is null or last_seen_at is null;

do $$
begin
    if not exists (
        select 1 from pg_constraint where conname = 'messaging_contacts_relationship_status_ck'
    ) then
        alter table messaging.contacts
            add constraint messaging_contacts_relationship_status_ck
            check (relationship_status in ('lead', 'prospect', 'customer', 'inactive'));
    end if;
end $$;

create table if not exists messaging.contact_identities (
    id                 uuid primary key default gen_random_uuid(),
    account_id         uuid not null references core.accounts(id) on delete cascade,
    contact_id         uuid not null references messaging.contacts(id) on delete cascade,
    channel            text not null check (channel in ('WHATSAPP', 'INSTAGRAM')),
    provider           text not null,
    instance_scope_key text not null default '',
    external_id        text not null,
    display_name       text,
    avatar_url         text,
    metadata           jsonb not null default '{}'::jsonb,
    first_seen_at      timestamptz not null default now(),
    last_seen_at       timestamptz not null default now(),
    created_at         timestamptz not null default now(),
    updated_at         timestamptz not null default now()
);
create unique index if not exists messaging_contact_identities_external_uidx
    on messaging.contact_identities
       (account_id, channel, provider, instance_scope_key, external_id);
create index if not exists messaging_contact_identities_contact_idx
    on messaging.contact_identities (account_id, contact_id);

create table if not exists messaging.contact_touchpoints (
    id                 uuid primary key default gen_random_uuid(),
    account_id         uuid not null references core.accounts(id) on delete cascade,
    contact_id         uuid not null references messaging.contacts(id) on delete cascade,
    conversation_id    uuid references messaging.conversations(id) on delete set null,
    message_id         uuid references messaging.messages(id) on delete set null,
    channel            text not null check (channel in ('WHATSAPP', 'INSTAGRAM')),
    provider           text not null,
    external_event_id  text,
    source_kind        text not null,
    source_ref         text,
    landing_page_id    text,
    campaign_id        text,
    metadata           jsonb not null default '{}'::jsonb,
    occurred_at        timestamptz not null,
    created_at         timestamptz not null default now()
);
create unique index if not exists messaging_contact_touchpoints_event_uidx
    on messaging.contact_touchpoints (account_id, provider, external_event_id)
    where external_event_id is not null and external_event_id <> '';
create index if not exists messaging_contact_touchpoints_contact_time_idx
    on messaging.contact_touchpoints (account_id, contact_id, occurred_at desc);
create index if not exists messaging_contact_touchpoints_landing_idx
    on messaging.contact_touchpoints (account_id, landing_page_id, occurred_at desc)
    where landing_page_id is not null;

create table if not exists messaging.contact_notes (
    id              uuid primary key default gen_random_uuid(),
    account_id      uuid not null references core.accounts(id) on delete cascade,
    contact_id      uuid not null references messaging.contacts(id) on delete cascade,
    conversation_id uuid references messaging.conversations(id) on delete set null,
    author_user_id  uuid references core.users(id) on delete set null,
    content         text not null,
    created_at      timestamptz not null default now(),
    updated_at      timestamptz not null default now()
);
create index if not exists messaging_contact_notes_contact_time_idx
    on messaging.contact_notes (account_id, contact_id, created_at desc);

