-- Omnichannel: memoria estruturada e auditavel por contato.
-- O PostgreSQL continua autoritativo; o n8n apenas sugere fatos no resultado versionado.
-- Nenhum prompt, segredo ou historico bruto e persistido nesta tabela.

create schema if not exists messaging;

create table if not exists messaging.contact_intelligence (
    account_id             uuid not null references core.accounts(id) on delete cascade,
    contact_id             uuid not null,
    summary                text not null default '',
    facts                  jsonb not null default '{}'::jsonb,
    preferences            jsonb not null default '{}'::jsonb,
    interaction_count      integer not null default 0 check (interaction_count >= 0),
    ai_reply_count         integer not null default 0 check (ai_reply_count >= 0),
    handoff_count          integer not null default 0 check (handoff_count >= 0),
    last_intent            text not null default '',
    last_sentiment         text not null default 'unknown'
        check (last_sentiment in ('positive', 'neutral', 'negative', 'unknown')),
    last_confidence        numeric(4,3),
    last_outcome           text not null default '',
    last_conversation_id   uuid,
    last_ai_run_id         uuid,
    last_learned_at        timestamptz,
    created_at             timestamptz not null default now(),
    updated_at             timestamptz not null default now(),
    primary key (account_id, contact_id),
    constraint messaging_contact_intelligence_confidence_ck
        check (last_confidence is null or last_confidence between 0 and 1),
    constraint messaging_contact_intelligence_contact_tenant_fk
        foreign key (account_id, contact_id)
        references messaging.contacts(account_id, id) on delete cascade,
    constraint messaging_contact_intelligence_conversation_tenant_fk
        foreign key (account_id, last_conversation_id)
        references messaging.conversations(account_id, id) on delete set null
);

create index if not exists messaging_contact_intelligence_updated_idx
    on messaging.contact_intelligence (account_id, updated_at desc);

