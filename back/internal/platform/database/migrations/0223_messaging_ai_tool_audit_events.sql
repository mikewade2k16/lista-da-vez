-- E6-BE-03: amplia a trilha autoritativa com eventos de execução de tools.
-- Não altera dados existentes nem toca schemas de outros módulos.
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
end $$;

alter table messaging.audit_events
    add constraint messaging_audit_events_type_ck check (event_type in (
        'MESSAGE_OUTBOUND_QUEUED', 'MESSAGE_OUTBOUND_SENT', 'MESSAGE_OUTBOUND_FAILED',
        'CONVERSATION_STATUS_CHANGED', 'CONVERSATION_ASSIGNED',
        'HANDOFF_REQUESTED', 'HANDOFF_ACCEPTED', 'CONVERSATION_RELEASED',
        'CONVERSATION_TRANSFERRED', 'SLA_UPDATED',
        'AI_TOOL_REQUESTED', 'AI_TOOL_COMPLETED', 'AI_TOOL_DENIED',
        'AI_TOOL_FAILED', 'AI_TOOL_TIMEOUT'
    ));
