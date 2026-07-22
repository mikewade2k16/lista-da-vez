-- E6-FE-06: propostas de tools mutáveis com aprovação humana explícita.
-- Os argumentos originais ficam cifrados no Go; a tela e a auditoria só recebem a projeção mascarada.
-- Não há credencial, URL ou SQL nesta tabela. A execução continua exclusivamente no registry Go.

create unique index if not exists messaging_ai_tool_runs_account_id_uidx
    on messaging.ai_tool_runs (account_id, id);
create unique index if not exists messaging_conversations_account_id_uidx
    on messaging.conversations (account_id, id);

create table if not exists messaging.ai_tool_approvals (
    id                   uuid primary key default gen_random_uuid(),
    account_id           uuid not null references core.accounts(id) on delete cascade,
    tool_run_id          uuid not null,
    binding_id           uuid not null,
    agent_id             uuid not null,
    conversation_id      uuid,
    dispatch_id          uuid,
    call_id              text not null check (length(call_id) between 1 and 160),
    operation            text not null check (length(operation) between 1 and 160),
    arguments_ciphertext text not null check (length(arguments_ciphertext) between 1 and 262144),
    status               text not null default 'pending'
        check (status in ('pending','approved','rejected','expired')),
    reason               text not null default '' check (length(reason) <= 500),
    decided_by           uuid references core.users(id) on delete set null,
    requested_at         timestamptz not null default now(),
    decided_at           timestamptz,
    expires_at           timestamptz not null default (now() + interval '15 minutes'),
    constraint messaging_ai_tool_approvals_run_fk
        foreign key (account_id, tool_run_id)
        references messaging.ai_tool_runs(account_id, id) on delete cascade,
    constraint messaging_ai_tool_approvals_binding_fk
        foreign key (account_id, binding_id)
        references messaging.ai_tool_bindings(account_id, id) on delete restrict,
    constraint messaging_ai_tool_approvals_agent_fk
        foreign key (account_id, agent_id)
        references messaging.ai_agents(account_id, id) on delete cascade,
    constraint messaging_ai_tool_approvals_conversation_fk
        foreign key (account_id, conversation_id)
        references messaging.conversations(account_id, id) on delete cascade
);

create unique index if not exists messaging_ai_tool_approvals_run_uidx
    on messaging.ai_tool_approvals (account_id, tool_run_id);
create index if not exists messaging_ai_tool_approvals_agent_status_idx
    on messaging.ai_tool_approvals (account_id, agent_id, status, requested_at desc, id desc);

do $$
begin
    if exists (select 1 from pg_constraint
        where conname = 'messaging_audit_events_type_ck'
          and conrelid = 'messaging.audit_events'::regclass) then
        alter table messaging.audit_events drop constraint messaging_audit_events_type_ck;
    end if;
    if exists (select 1 from pg_constraint
        where conname = 'audit_events_event_type_check'
          and conrelid = 'messaging.audit_events'::regclass) then
        alter table messaging.audit_events drop constraint audit_events_event_type_check;
    end if;
    if exists (select 1 from pg_constraint
        where conname = 'messaging_audit_events_event_type_e1_check'
          and conrelid = 'messaging.audit_events'::regclass) then
        alter table messaging.audit_events drop constraint messaging_audit_events_event_type_e1_check;
    end if;
end $$;

alter table messaging.audit_events
    add constraint messaging_audit_events_type_ck check (event_type in (
        'MESSAGE_OUTBOUND_QUEUED', 'MESSAGE_OUTBOUND_SENT', 'MESSAGE_OUTBOUND_FAILED',
        'CONVERSATION_STATUS_CHANGED', 'CONVERSATION_ASSIGNED',
        'MESSAGE_FORWARDED', 'MESSAGE_DELETED_FOR_ALL',
        'MESSAGE_MEDIA_READY', 'MESSAGE_MEDIA_FAILED', 'MESSAGE_MEDIA_RETRY',
        'CONTACT_MERGED', 'CONTACT_MERGE_UNDONE',
        'HANDOFF_REQUESTED', 'HANDOFF_ACCEPTED', 'CONVERSATION_RELEASED',
        'CONVERSATION_TRANSFERRED', 'SLA_UPDATED',
        'AI_TOOL_REQUESTED', 'AI_TOOL_COMPLETED', 'AI_TOOL_DENIED',
        'AI_TOOL_FAILED', 'AI_TOOL_TIMEOUT',
        'AI_TOOL_APPROVAL_REQUESTED', 'AI_TOOL_APPROVED', 'AI_TOOL_REJECTED'
    ));
