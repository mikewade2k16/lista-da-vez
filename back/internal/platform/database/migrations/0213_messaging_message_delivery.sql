-- E1: reply references, message origin and provider delivery state.
-- Additive and rolling-compatible: the previous application version keeps using
-- the original columns/status values while the E1 backend starts populating these.
-- Plain SQL only; the project migrator runs the entire file in one transaction.

-- Monotonic lease used to invalidate an LLM dispatch when a human takes over.
-- Human transitions increment it under the same conversation row lock; merge and
-- AI message/outbox creation compare the generation captured before the model call.
alter table messaging.conversations
    add column if not exists ai_generation bigint not null default 0;

alter table messaging.messages
    add column if not exists reply_to_message_id uuid;

alter table messaging.messages
    add column if not exists reply_to_external_message_id text;

alter table messaging.messages
    add column if not exists origin text not null default 'contact';

alter table messaging.messages
    add column if not exists provider_status_at timestamptz;

alter table messaging.messages
    add column if not exists provider_error_code text not null default '';

-- Classify the history before enforcing the origin vocabulary. This update is
-- deterministic and therefore safe to repeat: inbound always belongs to the
-- contact; outbound prefers the explicit legacy AI source, then an authenticated
-- sender, and otherwise represents a message mirrored from the provider device.
update messaging.messages
set origin = case
    when direction = 'INBOUND' then 'contact'
    when metadata_json ->> 'source' = 'ai' then 'ai'
    when sender_user_id is not null then 'human'
    else 'provider_device'
end
where origin = 'contact';

-- PostgreSQL 16 has no ADD CONSTRAINT IF NOT EXISTS. Check both the relation and
-- the constraint name so an unrelated schema cannot suppress this self-reference.
do $$
begin
    if not exists (
        select 1
        from pg_constraint
        where conname = 'messaging_messages_reply_to_message_fk'
          and conrelid = 'messaging.messages'::regclass
    ) then
        alter table messaging.messages
            add constraint messaging_messages_reply_to_message_fk
            foreign key (reply_to_message_id)
            references messaging.messages(id)
            on delete set null
            not valid;
    end if;
end
$$;

alter table messaging.messages
    validate constraint messaging_messages_reply_to_message_fk;

do $$
begin
    if not exists (
        select 1
        from pg_constraint
        where conname = 'messaging_messages_origin_check'
          and conrelid = 'messaging.messages'::regclass
    ) then
        alter table messaging.messages
            add constraint messaging_messages_origin_check
            check (origin in ('contact', 'human', 'ai', 'provider_device', 'system'))
            not valid;
    end if;
end
$$;

alter table messaging.messages
    validate constraint messaging_messages_origin_check;

-- Replace the original PENDING|SENT|FAILED constraint with the E1 provider ACK
-- states. The old values remain valid, so old and new application versions can
-- run during the rollout. Delivery transition monotonicity remains a Go/store rule.
alter table messaging.messages
    drop constraint if exists messages_status_check;

alter table messaging.messages
    add constraint messages_status_check
    check (status in ('PENDING', 'SENT', 'DELIVERED', 'READ', 'FAILED', 'DELETED'))
    not valid;

alter table messaging.messages
    validate constraint messages_status_check;

create unique index if not exists messaging_messages_external_id_uidx
    on messaging.messages (account_id, instance_scope_key, external_message_id)
    where external_message_id is not null
      and btrim(external_message_id) <> '';

create index if not exists messaging_messages_conversation_cursor_idx
    on messaging.messages (account_id, conversation_id, created_at desc, id desc);

create index if not exists messaging_messages_reply_external_idx
    on messaging.messages (account_id, reply_to_external_message_id)
    where reply_to_external_message_id is not null
      and btrim(reply_to_external_message_id) <> '';
