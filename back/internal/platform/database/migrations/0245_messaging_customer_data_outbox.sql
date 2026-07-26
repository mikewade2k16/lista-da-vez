-- CI-01/CI-05: entrega duravel e ID-only de um novo inbound resolvido ao
-- Customer Data. Esta lane e separada de messaging.outbox (envio ao canal) e
-- de messaging.intelligence_outbox (resultado aceito da IA), portanto uma
-- indisponibilidade do Customer Data nunca disputa FIFO com o sender.

create table if not exists messaging.customer_data_outbox (
    id                        uuid primary key default gen_random_uuid(),
    account_id                uuid not null references core.accounts(id) on delete restrict,
    client_account_id         uuid not null references core.accounts(id) on delete restrict,
    contact_id                uuid not null,
    conversation_id           uuid not null,
    message_id                uuid not null,
    channel_client_binding_id uuid not null,
    channel                   text not null check (channel in ('WHATSAPP', 'INSTAGRAM')),
    provider                  text not null check (char_length(btrim(provider)) between 1 and 80),
    ordering_key              text not null,
    idempotency_key           text not null,
    kind                      text not null default 'omnichannel.customer_data.relationship.resolve',
    topic                     text not null default 'omnichannel.customer_data.relationship.resolve',
    schema_version            text not null default 'omnichannel.customer_data.inbound.v1',
    payload                   jsonb not null,
    status                    text not null default 'pending',
    attempts                  integer not null default 0,
    max_attempts              integer not null default 8,
    run_after                 timestamptz not null default now(),
    locked_at                 timestamptz,
    locked_by                 text not null default '',
    last_error                text not null default '',
    occurred_at               timestamptz not null,
    published_at              timestamptz,
    created_at                timestamptz not null default now(),
    updated_at                timestamptz not null default now(),
    constraint messaging_customer_data_outbox_account_contact_fk
        foreign key (account_id, contact_id)
        references messaging.contacts(account_id, id)
        on delete restrict,
    constraint messaging_customer_data_outbox_account_conversation_fk
        foreign key (account_id, conversation_id)
        references messaging.conversations(account_id, id)
        on delete restrict,
    constraint messaging_customer_data_outbox_account_message_fk
        foreign key (account_id, message_id)
        references messaging.messages(account_id, id)
        on delete restrict,
    constraint messaging_customer_data_outbox_binding_fk
        foreign key (account_id, client_account_id, channel_client_binding_id)
        references messaging.channel_client_bindings(account_id, client_account_id, id)
        on delete restrict,
    constraint messaging_customer_data_outbox_idempotency_uidx
        unique (account_id, idempotency_key),
    constraint messaging_customer_data_outbox_message_uidx
        unique (account_id, message_id),
    constraint messaging_customer_data_outbox_status_check
        check (status in ('pending', 'processing', 'done', 'failed', 'dead')),
    constraint messaging_customer_data_outbox_attempts_check
        check (attempts >= 0 and max_attempts between 1 and 50),
    constraint messaging_customer_data_outbox_kind_check
        check (kind = 'omnichannel.customer_data.relationship.resolve'),
    constraint messaging_customer_data_outbox_topic_check
        check (topic = 'omnichannel.customer_data.relationship.resolve'),
    constraint messaging_customer_data_outbox_schema_check
        check (schema_version = 'omnichannel.customer_data.inbound.v1'),
    constraint messaging_customer_data_outbox_payload_id_only_check
        check (
            jsonb_typeof(payload) = 'object'
            and payload ?& array[
                'schemaVersion',
                'eventId',
                'accountId',
                'clientAccountId',
                'contactId',
                'conversationId',
                'messageId',
                'channelClientBindingId',
                'channel',
                'provider',
                'occurredAt'
            ]
            and (
                payload - array[
                    'schemaVersion',
                    'eventId',
                    'accountId',
                    'clientAccountId',
                    'contactId',
                    'conversationId',
                    'messageId',
                    'channelClientBindingId',
                    'channel',
                    'provider',
                    'occurredAt'
                ]::text[]
            ) = '{}'::jsonb
            and payload->>'schemaVersion' = schema_version
            and payload->>'eventId' = id::text
            and payload->>'accountId' = account_id::text
            and payload->>'clientAccountId' = client_account_id::text
            and payload->>'contactId' = contact_id::text
            and payload->>'conversationId' = conversation_id::text
            and payload->>'messageId' = message_id::text
            and payload->>'channelClientBindingId' = channel_client_binding_id::text
            and payload->>'channel' = channel
            and payload->>'provider' = provider
            and (payload->>'occurredAt')::timestamptz = occurred_at
        )
);

create index if not exists messaging_customer_data_outbox_claim_idx
    on messaging.customer_data_outbox (status, run_after, created_at, id)
    where status = 'pending';

create index if not exists messaging_customer_data_outbox_fifo_idx
    on messaging.customer_data_outbox
    (account_id, ordering_key, status, created_at, id);

create index if not exists messaging_customer_data_outbox_client_idx
    on messaging.customer_data_outbox
    (account_id, client_account_id, occurred_at desc, id);

create index if not exists messaging_customer_data_outbox_contact_idx
    on messaging.customer_data_outbox
    (account_id, client_account_id, contact_id, occurred_at desc, id);

create or replace function messaging.mark_customer_data_outbox_published()
returns trigger
language plpgsql
as $$
begin
    if new.status = 'done' and old.status is distinct from 'done' then
        new.published_at = coalesce(new.published_at, now());
    end if;
    return new;
end;
$$;

drop trigger if exists customer_data_outbox_published_at
    on messaging.customer_data_outbox;
create trigger customer_data_outbox_published_at
before update of status on messaging.customer_data_outbox
for each row
execute function messaging.mark_customer_data_outbox_published();
