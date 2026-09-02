-- P0: ocultacao logica do historico por instancia de WhatsApp.
-- O cutoff preserva mensagens, conversas, contatos e auditoria; nenhuma linha e apagada.

alter table messaging.whatsapp_instances
    add column if not exists history_visible_from timestamptz;

alter table messaging.whatsapp_instances
    add column if not exists history_reset_revision bigint not null default 0;

-- O vocabulario de auditoria e fechado. Preserve integralmente a uniao vigente da 0225
-- e acrescente somente o evento do reset logico.
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
        'AI_TOOL_APPROVAL_REQUESTED', 'AI_TOOL_APPROVED', 'AI_TOOL_REJECTED',
        'WHATSAPP_INSTANCE_HISTORY_RESET'
    ));
