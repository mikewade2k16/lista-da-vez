-- CI-07: the Omnichannel owns acceptance of intelligent decisions.  This
-- outbox is deliberately separate from messaging.outbox, whose payload is a
-- provider send command.  It carries identifiers and reason codes only.

alter table messaging.ai_dispatches
    add column if not exists intelligence_decision_id text,
    add column if not exists intelligence_run_id uuid;

create table if not exists messaging.intelligence_decision_acceptances (
    id uuid primary key default gen_random_uuid(),
    event_id uuid not null,
    account_id uuid not null references core.accounts(id) on delete restrict,
    client_account_id uuid not null references core.accounts(id) on delete restrict,
    conversation_id uuid not null,
    dispatch_id uuid not null,
    message_id uuid,
    subject_id uuid,
    relationship_id uuid,
    generation bigint not null,
    decision_id text not null,
    intelligence_run_id uuid,
    outcome text not null,
    reason_code text not null,
    created_at timestamptz not null default now(),
    unique (account_id, event_id),
    unique (account_id, dispatch_id, generation),
    constraint intelligence_acceptance_conversation_fk
        foreign key (account_id, conversation_id)
        references messaging.conversations(account_id, id)
        on delete restrict,
    constraint intelligence_acceptance_dispatch_fk
        foreign key (account_id, dispatch_id)
        references messaging.ai_dispatches(account_id, id)
        on delete restrict,
    constraint intelligence_acceptance_message_fk
        foreign key (account_id, message_id)
        references messaging.messages(account_id, id)
        on delete restrict,
    constraint intelligence_acceptance_outcome_check
        check (outcome in ('reply', 'handoff', 'no_reply')),
    constraint intelligence_acceptance_reason_check
        check (char_length(reason_code) between 1 and 120),
    constraint intelligence_acceptance_decision_check
        check (char_length(decision_id) between 1 and 256)
);

create index if not exists intelligence_acceptances_client_created_idx
    on messaging.intelligence_decision_acceptances
    (account_id, client_account_id, created_at desc, id);

create table if not exists messaging.intelligence_outbox (
    id uuid primary key default gen_random_uuid(),
    event_id uuid not null,
    account_id uuid not null references core.accounts(id) on delete restrict,
    client_account_id uuid not null references core.accounts(id) on delete restrict,
    ordering_key text not null,
    idempotency_key text not null,
    kind text not null,
    topic text not null default 'omnichannel.interaction.accepted',
    schema_version text not null default 'omnichannel.interaction.accepted.v1',
    aggregate_id uuid not null,
    causation_id text not null default '',
    correlation_id text not null default '',
    payload jsonb not null,
    status text not null default 'pending',
    attempts integer not null default 0,
    max_attempts integer not null default 8,
    run_after timestamptz not null default now(),
    locked_at timestamptz,
    locked_by text not null default '',
    last_error text not null default '',
    occurred_at timestamptz not null default now(),
    published_at timestamptz,
    created_at timestamptz not null default now(),
    updated_at timestamptz not null default now(),
    unique (account_id, event_id),
    unique (account_id, idempotency_key),
    constraint intelligence_outbox_status_check
        check (status in ('pending', 'processing', 'done', 'failed', 'dead')),
    constraint intelligence_outbox_attempts_check
        check (attempts >= 0 and max_attempts between 1 and 50),
    constraint intelligence_outbox_payload_check
        check (jsonb_typeof(payload) = 'object'),
    constraint intelligence_outbox_topic_check
        check (topic = 'omnichannel.interaction.accepted'),
    constraint intelligence_outbox_schema_check
        check (schema_version = 'omnichannel.interaction.accepted.v1')
);

create index if not exists intelligence_outbox_claim_idx
    on messaging.intelligence_outbox (status, run_after, created_at, id)
    where status = 'pending';

create index if not exists intelligence_outbox_fifo_idx
    on messaging.intelligence_outbox
    (account_id, ordering_key, status, created_at, id);

create index if not exists intelligence_outbox_client_idx
    on messaging.intelligence_outbox
    (account_id, client_account_id, created_at desc, id);

create or replace function messaging.mark_intelligence_outbox_published()
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

drop trigger if exists intelligence_outbox_published_at
    on messaging.intelligence_outbox;
create trigger intelligence_outbox_published_at
before update of status on messaging.intelligence_outbox
for each row
execute function messaging.mark_intelligence_outbox_published();
