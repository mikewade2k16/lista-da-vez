-- Omnichannel E5: handoff humano, SLA e policy determinística.
-- Somente o módulo omnichannel usa estas tabelas. Não altera automação, WAHA,
-- calendário, operação ou workflows n8n de outros módulos.
-- Idempotente, schema-qualified e sem Down.

create schema if not exists messaging;

create table if not exists messaging.handoffs (
    id                    uuid primary key default gen_random_uuid(),
    account_id            uuid not null references core.accounts(id) on delete cascade,
    conversation_id       uuid not null,
    ai_run_id             uuid,
    routing_decision_id   uuid,
    reason_code           text not null default 'requested'
        check (reason_code in ('requested','low_confidence','max_turns','tool_failed','policy','error')),
    summary               text not null default '' check (length(summary) <= 12000),
    collected_fields      jsonb not null default '{}'::jsonb
        check (jsonb_typeof(collected_fields) = 'object'),
    source_state          text not null default '',
    target_queue_id       uuid,
    status                text not null default 'requested'
        check (status in ('requested','queued','accepted','cancelled','closed')),
    idempotency_key       text not null default '',
    accepted_by_user_id   uuid references core.users(id) on delete set null,
    requested_at          timestamptz not null default now(),
    queued_at             timestamptz,
    accepted_at           timestamptz,
    closed_at             timestamptz,
    created_at            timestamptz not null default now(),
    updated_at            timestamptz not null default now(),
    constraint messaging_handoffs_conversation_fk
        foreign key (account_id, conversation_id)
        references messaging.conversations(account_id, id) on delete cascade,
    constraint messaging_handoffs_ai_run_fk
        foreign key (ai_run_id) references messaging.ai_runs(id) on delete set null,
    constraint messaging_handoffs_routing_decision_fk
        foreign key (routing_decision_id) references messaging.routing_decisions(id) on delete set null,
    constraint messaging_handoffs_queue_fk
        foreign key (target_queue_id) references messaging.queues(id) on delete set null
);

create unique index if not exists messaging_handoffs_account_idempotency_uidx
    on messaging.handoffs (account_id, idempotency_key)
    where idempotency_key <> '';
create unique index if not exists messaging_handoffs_open_conversation_uidx
    on messaging.handoffs (account_id, conversation_id)
    where status in ('requested','queued','accepted');
create index if not exists messaging_handoffs_conversation_created_idx
    on messaging.handoffs (account_id, conversation_id, created_at desc);
create index if not exists messaging_handoffs_queue_status_idx
    on messaging.handoffs (account_id, target_queue_id, status, requested_at);

create table if not exists messaging.queue_sla_policies (
    id                    uuid primary key default gen_random_uuid(),
    account_id            uuid not null references core.accounts(id) on delete cascade,
    queue_id              uuid not null,
    first_response_seconds integer not null default 900 check (first_response_seconds between 1 and 2592000),
    resolution_seconds    integer not null default 86400 check (resolution_seconds between 1 and 7776000),
    business_hours_only   boolean not null default true,
    is_active             boolean not null default true,
    created_at            timestamptz not null default now(),
    updated_at            timestamptz not null default now(),
    constraint messaging_queue_sla_policy_queue_fk
        foreign key (queue_id) references messaging.queues(id) on delete cascade
);
create unique index if not exists messaging_queue_sla_policies_queue_uidx
    on messaging.queue_sla_policies (account_id, queue_id);

create table if not exists messaging.sla_events (
    id                    uuid primary key default gen_random_uuid(),
    account_id            uuid not null references core.accounts(id) on delete cascade,
    conversation_id       uuid not null,
    handoff_id            uuid,
    event_type            text not null
        check (event_type in ('started','warning','breached','paused','resumed','satisfied')),
    idempotency_key       text not null,
    occurred_at           timestamptz not null default now(),
    metadata              jsonb not null default '{}'::jsonb
        check (jsonb_typeof(metadata) = 'object'),
    constraint messaging_sla_events_conversation_fk
        foreign key (account_id, conversation_id)
        references messaging.conversations(account_id, id) on delete cascade,
    constraint messaging_sla_events_handoff_fk
        foreign key (handoff_id) references messaging.handoffs(id) on delete set null
);
create unique index if not exists messaging_sla_events_idempotency_uidx
    on messaging.sla_events (account_id, idempotency_key);
create index if not exists messaging_sla_events_conversation_occurred_idx
    on messaging.sla_events (account_id, conversation_id, occurred_at desc);

create table if not exists messaging.handoff_policies (
    id                    uuid primary key default gen_random_uuid(),
    account_id            uuid not null references core.accounts(id) on delete cascade,
    name                  text not null,
    priority              integer not null default 100 check (priority >= 0 and priority <= 100000),
    is_active              boolean not null default true,
    conditions             jsonb not null default '{}'::jsonb
        check (jsonb_typeof(conditions) = 'object'),
    target_queue_id        uuid,
    fallback_queue_id      uuid,
    customer_notice_template text not null default '' check (length(customer_notice_template) <= 2000),
    created_at             timestamptz not null default now(),
    updated_at             timestamptz not null default now(),
    constraint messaging_handoff_policies_target_queue_fk
        foreign key (target_queue_id) references messaging.queues(id) on delete set null,
    constraint messaging_handoff_policies_fallback_queue_fk
        foreign key (fallback_queue_id) references messaging.queues(id) on delete set null
);
create unique index if not exists messaging_handoff_policies_name_uidx
    on messaging.handoff_policies (account_id, name);
create index if not exists messaging_handoff_policies_eval_idx
    on messaging.handoff_policies (account_id, priority, id) where is_active;

-- A trilha existente tinha um CHECK fechado antes do E5. Ampliamos apenas com os
-- eventos do handoff/SLA; valores antigos permanecem válidos.
do $$
begin
    if exists (select 1 from pg_constraint where conname = 'audit_events_event_type_check'
        and conrelid = 'messaging.audit_events'::regclass) then
        alter table messaging.audit_events drop constraint audit_events_event_type_check;
    end if;
    if exists (select 1 from pg_constraint where conname = 'messaging_audit_events_type_ck'
        and conrelid = 'messaging.audit_events'::regclass) then
        alter table messaging.audit_events drop constraint messaging_audit_events_type_ck;
    end if;
end $$;

alter table messaging.audit_events
    add constraint messaging_audit_events_type_ck check (event_type in (
        'MESSAGE_OUTBOUND_QUEUED', 'MESSAGE_OUTBOUND_SENT', 'MESSAGE_OUTBOUND_FAILED',
        'CONVERSATION_STATUS_CHANGED', 'CONVERSATION_ASSIGNED',
        'HANDOFF_REQUESTED', 'HANDOFF_ACCEPTED', 'CONVERSATION_RELEASED',
        'CONVERSATION_TRANSFERRED', 'SLA_UPDATED'
    ));
