-- Omnichannel E4: complementos aditivos do CRM e atribuição 360°.
-- 0211/0212 permanecem imutáveis; esta migration mantém compatibilidade com os valores legados
-- e não grava token de captura, PII de integração ou regra de tenant no n8n.
-- Sem Down. Nenhum endpoint/worker é ligado por este arquivo.

create schema if not exists messaging;

alter table messaging.contacts
    add column if not exists primary_email text,
    add column if not exists owner_user_id uuid references core.users(id) on delete set null,
    add column if not exists merged_into_contact_id uuid references messaging.contacts(id) on delete set null,
    add column if not exists archived_at timestamptz,
    add column if not exists classification_source text not null default 'backfill',
    add column if not exists classification_confidence numeric(4,3),
    add column if not exists last_qualified_at timestamptz;

-- A limpeza dos valores legados ocorre em pacote de backfill separado. Até lá, o CHECK aceita
-- os dois vocabulários, enquanto o backend novo só escreve os quatro valores canônicos.
alter table messaging.contacts
    drop constraint if exists messaging_contacts_relationship_status_ck;
do $$
begin
    if not exists (select 1 from pg_constraint where conname = 'messaging_contacts_relationship_status_compat_ck') then
        alter table messaging.contacts
            add constraint messaging_contacts_relationship_status_compat_ck
            check (relationship_status in ('lead', 'prospect', 'new_lead', 'known_lead', 'customer', 'inactive'));
    end if;
    if not exists (select 1 from pg_constraint where conname = 'messaging_contacts_classification_source_ck') then
        alter table messaging.contacts
            add constraint messaging_contacts_classification_source_ck
            check (classification_source in ('manual', 'ai', 'erp', 'rule', 'backfill'));
    end if;
    if not exists (select 1 from pg_constraint where conname = 'messaging_contacts_classification_confidence_ck') then
        alter table messaging.contacts
            add constraint messaging_contacts_classification_confidence_ck
            check (classification_confidence is null or classification_confidence between 0 and 1);
    end if;
    if not exists (select 1 from pg_constraint where conname = 'messaging_contacts_merge_target_ck') then
        alter table messaging.contacts
            add constraint messaging_contacts_merge_target_ck
            check (merged_into_contact_id is null or merged_into_contact_id <> id);
    end if;
end $$;

create index if not exists messaging_contacts_email_ci_idx
    on messaging.contacts (account_id, lower(primary_email))
    where primary_email is not null and primary_email <> '';
create index if not exists messaging_contacts_owner_updated_idx
    on messaging.contacts (account_id, owner_user_id, updated_at desc)
    where archived_at is null;
create index if not exists messaging_contacts_merged_idx
    on messaging.contacts (account_id, merged_into_contact_id)
    where merged_into_contact_id is not null;
create unique index if not exists messaging_contacts_account_id_uidx
    on messaging.contacts (account_id, id);
create unique index if not exists messaging_conversations_account_id_uidx
    on messaging.conversations (account_id, id);
create unique index if not exists messaging_messages_account_id_uidx
    on messaging.messages (account_id, id);

create table if not exists messaging.contact_merge_events (
    id                    uuid primary key default gen_random_uuid(),
    account_id            uuid not null references core.accounts(id) on delete cascade,
    source_contact_id     uuid not null,
    target_contact_id     uuid not null,
    actor_user_id         uuid references core.users(id) on delete set null,
    reason                text not null,
    idempotency_key       text not null,
    snapshot               jsonb not null default '{}'::jsonb,
    created_at             timestamptz not null default now(),
    constraint messaging_contact_merge_distinct_ck check (source_contact_id <> target_contact_id)
);
create unique index if not exists messaging_contact_merge_events_idempotency_uidx
    on messaging.contact_merge_events (account_id, idempotency_key);
create index if not exists messaging_contact_merge_events_source_idx
    on messaging.contact_merge_events (account_id, source_contact_id, created_at desc);
create index if not exists messaging_contact_merge_events_target_idx
    on messaging.contact_merge_events (account_id, target_contact_id, created_at desc);

create table if not exists messaging.contact_consents (
    id                uuid primary key default gen_random_uuid(),
    account_id        uuid not null references core.accounts(id) on delete cascade,
    contact_id        uuid not null,
    purpose           text not null,
    channel           text not null,
    status            text not null default 'unknown'
        check (status in ('granted', 'revoked', 'unknown')),
    source            text not null default '',
    evidence_ref      text not null default '',
    effective_at      timestamptz not null default now(),
    expires_at        timestamptz,
    created_at        timestamptz not null default now(),
    updated_at        timestamptz not null default now()
);
create index if not exists messaging_contact_consents_current_idx
    on messaging.contact_consents (account_id, contact_id, purpose, channel, effective_at desc);

create table if not exists messaging.contact_external_refs (
    id                uuid primary key default gen_random_uuid(),
    account_id        uuid not null references core.accounts(id) on delete cascade,
    contact_id        uuid not null,
    system            text not null,
    external_id       text not null,
    metadata          jsonb not null default '{}'::jsonb,
    verified_at       timestamptz,
    created_at        timestamptz not null default now(),
    updated_at        timestamptz not null default now()
);
create unique index if not exists messaging_contact_external_refs_system_uidx
    on messaging.contact_external_refs (account_id, system, external_id);
create index if not exists messaging_contact_external_refs_contact_idx
    on messaging.contact_external_refs (account_id, contact_id, system);

create table if not exists messaging.contact_segments (
    id                uuid primary key default gen_random_uuid(),
    account_id        uuid not null references core.accounts(id) on delete cascade,
    name              text not null,
    filter_json       jsonb not null default '{}'::jsonb,
    version           integer not null default 1 check (version >= 1),
    owner_user_id     uuid references core.users(id) on delete set null,
    is_active         boolean not null default true,
    created_at        timestamptz not null default now(),
    updated_at        timestamptz not null default now()
);
create unique index if not exists messaging_contact_segments_name_uidx
    on messaging.contact_segments (account_id, name);
create index if not exists messaging_contact_segments_active_idx
    on messaging.contact_segments (account_id, is_active, updated_at desc);

create table if not exists messaging.lead_sources (
    id                  uuid primary key default gen_random_uuid(),
    account_id          uuid not null references core.accounts(id) on delete cascade,
    slug                text not null,
    name                text not null,
    domain              text not null default '',
    allowed_origins     text[] not null default '{}'::text[],
    capture_token_hash  text not null default '',
    is_active            boolean not null default true,
    created_at           timestamptz not null default now(),
    updated_at           timestamptz not null default now(),
    constraint messaging_lead_sources_origins_ck check (cardinality(allowed_origins) <= 20)
);
create unique index if not exists messaging_lead_sources_slug_uidx
    on messaging.lead_sources (account_id, slug);
create unique index if not exists messaging_lead_sources_account_id_uidx
    on messaging.lead_sources (account_id, id);
create unique index if not exists messaging_lead_sources_token_hash_uidx
    on messaging.lead_sources (account_id, capture_token_hash)
    where capture_token_hash <> '';
create index if not exists messaging_lead_sources_active_idx
    on messaging.lead_sources (account_id, is_active, slug);

alter table messaging.contact_touchpoints
    add column if not exists landing_source_id uuid,
    add column if not exists utm_source text,
    add column if not exists utm_medium text,
    add column if not exists utm_campaign text,
    add column if not exists utm_term text,
    add column if not exists utm_content text,
    add column if not exists referrer_host text;

alter table messaging.contact_touchpoints
    drop constraint if exists messaging_contact_touchpoints_source_kind_ck;
do $$
begin
    if not exists (select 1 from pg_constraint where conname = 'messaging_contact_touchpoints_source_kind_ck') then
        alter table messaging.contact_touchpoints
            add constraint messaging_contact_touchpoints_source_kind_ck
            check (source_kind in (
                'whatsapp_inbound', 'instagram_dm', 'instagram_comment', 'landing_page',
                'manual', 'import', 'campaign', 'legacy_backfill', 'direct_message', 'lead'
            ));
    end if;
end $$;

create index if not exists messaging_contact_touchpoints_source_idx
    on messaging.contact_touchpoints (account_id, source_kind, occurred_at desc);
create index if not exists messaging_contact_touchpoints_landing_source_idx
    on messaging.contact_touchpoints (account_id, landing_source_id, occurred_at desc)
    where landing_source_id is not null;

-- FKs compostas fecham o tenant no banco, inclusive para as tabelas CRM que já existiam.
-- O índice único de (account_id,id) é criado acima; os blocos tornam a migration idempotente.
do $$
begin
    if not exists (select 1 from pg_constraint where conname = 'messaging_merge_events_source_tenant_fk') then
        alter table messaging.contact_merge_events
            add constraint messaging_merge_events_source_tenant_fk
            foreign key (account_id, source_contact_id)
            references messaging.contacts(account_id, id) on delete restrict;
    end if;
    if not exists (select 1 from pg_constraint where conname = 'messaging_merge_events_target_tenant_fk') then
        alter table messaging.contact_merge_events
            add constraint messaging_merge_events_target_tenant_fk
            foreign key (account_id, target_contact_id)
            references messaging.contacts(account_id, id) on delete restrict;
    end if;
    if not exists (select 1 from pg_constraint where conname = 'messaging_consents_contact_tenant_fk') then
        alter table messaging.contact_consents
            add constraint messaging_consents_contact_tenant_fk
            foreign key (account_id, contact_id)
            references messaging.contacts(account_id, id) on delete cascade;
    end if;
    if not exists (select 1 from pg_constraint where conname = 'messaging_external_refs_contact_tenant_fk') then
        alter table messaging.contact_external_refs
            add constraint messaging_external_refs_contact_tenant_fk
            foreign key (account_id, contact_id)
            references messaging.contacts(account_id, id) on delete cascade;
    end if;
    if not exists (select 1 from pg_constraint where conname = 'messaging_touchpoints_contact_tenant_fk') then
        alter table messaging.contact_touchpoints
            add constraint messaging_touchpoints_contact_tenant_fk
            foreign key (account_id, contact_id)
            references messaging.contacts(account_id, id) on delete cascade;
    end if;
    if not exists (select 1 from pg_constraint where conname = 'messaging_contacts_merge_target_tenant_fk') then
        alter table messaging.contacts
            add constraint messaging_contacts_merge_target_tenant_fk
            foreign key (account_id, merged_into_contact_id)
            references messaging.contacts(account_id, id) on delete set null;
    end if;
    if not exists (select 1 from pg_constraint where conname = 'messaging_identities_contact_tenant_fk') then
        alter table messaging.contact_identities
            add constraint messaging_identities_contact_tenant_fk
            foreign key (account_id, contact_id)
            references messaging.contacts(account_id, id) on delete cascade;
    end if;
    if not exists (select 1 from pg_constraint where conname = 'messaging_notes_contact_tenant_fk') then
        alter table messaging.contact_notes
            add constraint messaging_notes_contact_tenant_fk
            foreign key (account_id, contact_id)
            references messaging.contacts(account_id, id) on delete cascade;
    end if;
    if not exists (select 1 from pg_constraint where conname = 'messaging_conversations_contact_tenant_fk') then
        alter table messaging.conversations
            add constraint messaging_conversations_contact_tenant_fk
            foreign key (account_id, contact_id)
            references messaging.contacts(account_id, id) on delete set null;
    end if;
    if not exists (select 1 from pg_constraint where conname = 'messaging_touchpoints_landing_source_tenant_fk') then
        alter table messaging.contact_touchpoints
            add constraint messaging_touchpoints_landing_source_tenant_fk
            foreign key (account_id, landing_source_id)
            references messaging.lead_sources(account_id, id) on delete set null;
    end if;
    if not exists (select 1 from pg_constraint where conname = 'messaging_touchpoints_conversation_tenant_fk') then
        alter table messaging.contact_touchpoints
            add constraint messaging_touchpoints_conversation_tenant_fk
            foreign key (account_id, conversation_id)
            references messaging.conversations(account_id, id) on delete set null;
    end if;
    if not exists (select 1 from pg_constraint where conname = 'messaging_notes_conversation_tenant_fk') then
        alter table messaging.contact_notes
            add constraint messaging_notes_conversation_tenant_fk
            foreign key (account_id, conversation_id)
            references messaging.conversations(account_id, id) on delete set null;
    end if;
    if not exists (select 1 from pg_constraint where conname = 'messaging_touchpoints_message_tenant_fk') then
        alter table messaging.contact_touchpoints
            add constraint messaging_touchpoints_message_tenant_fk
            foreign key (account_id, message_id)
            references messaging.messages(account_id, id) on delete set null;
    end if;
end $$;
