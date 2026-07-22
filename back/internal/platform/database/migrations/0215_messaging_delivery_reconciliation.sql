-- Omnichannel E1-R1: durable provider ACK metadata and indexed message search.
--
-- Delivery notifications may arrive before the corresponding message/echo. Persist the
-- canonical, sanitized ACK fields beside the webhook dedupe row so the domain write can
-- replay them later in deterministic (provider_status_at, id) order. The raw provider
-- payload and free-form provider error remain forbidden here.
--
-- pg_trgm is already installed by 0034_erp_ftp_foundation.sql. This migration only adds
-- the expression index that matches the existing lower(content) LIKE '%...%' hot path.
-- Plain, schema-qualified, additive SQL; the project migrator wraps this file in one tx.

alter table messaging.webhook_events
    add column if not exists external_message_id text;

alter table messaging.webhook_events
    add column if not exists provider_status text;

alter table messaging.webhook_events
    add column if not exists provider_status_at timestamptz;

alter table messaging.webhook_events
    add column if not exists provider_error_code text not null default '';

-- Non-status webhook rows keep all delivery fields empty. Status rows must be complete,
-- use the closed E1 vocabulary and may retain only a short sanitized error token.
do $$
begin
    if not exists (
        select 1
        from pg_constraint
        where conname = 'messaging_webhook_events_delivery_check'
          and conrelid = 'messaging.webhook_events'::regclass
    ) then
        alter table messaging.webhook_events
            add constraint messaging_webhook_events_delivery_check
            check (
                (
                    provider_status is null
                    and external_message_id is null
                    and provider_status_at is null
                    and provider_error_code = ''
                )
                or
                (
                    event_kind = 'message_status'
                    and provider_status in ('SENT', 'DELIVERED', 'READ', 'FAILED', 'DELETED')
                    and instance_name is not null
                    and btrim(instance_name) <> ''
                    and external_message_id is not null
                    and btrim(external_message_id) <> ''
                    and provider_status_at is not null
                    and (
                        provider_error_code = ''
                        or (
                            provider_status = 'FAILED'
                            and provider_error_code ~ '^[A-Z0-9_-]{1,64}$'
                        )
                    )
                )
            )
            not valid;
    end if;
end
$$;

alter table messaging.webhook_events
    validate constraint messaging_webhook_events_delivery_check;

-- Equality on tenant/provider/instance/message plus this trailing order lets replay scan
-- ACKs without sorting. Included fields keep the replay projection covered.
create index if not exists messaging_webhook_events_delivery_replay_idx
    on messaging.webhook_events
        (account_id, provider, instance_name, external_message_id, provider_status_at, id)
    include (provider_status, provider_error_code)
    where provider_status is not null;

-- Exact expression used by Store.ListConversations for message-content search.
create index if not exists messaging_messages_content_trgm_idx
    on messaging.messages using gin (lower(content) gin_trgm_ops);

comment on column messaging.webhook_events.external_message_id is
    'Canonical provider message id for durable delivery-status replay; null on non-status events.';
comment on column messaging.webhook_events.provider_status is
    'Canonical delivery status: SENT, DELIVERED, READ, FAILED or DELETED.';
comment on column messaging.webhook_events.provider_status_at is
    'Provider event time used with webhook_events.id for deterministic ACK replay.';
comment on column messaging.webhook_events.provider_error_code is
    'Sanitized provider error token only; never a raw message, payload or PII.';
