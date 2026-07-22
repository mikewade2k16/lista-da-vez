-- Omnichannel E1: audita o ciclo duravel de midia inbound.
--
-- A constraint anterior (0208) aceita somente envio, atribuicao e acoes humanas.
-- O handler E1 ja grava READY/FAILED e o endpoint de retry grava RETRY; sem ampliar
-- o vocabulario, o efeito principal conclui mas a auditoria falha e gera ERROR falso.
--
-- A nova constraint e adicionada/validada antes de remover a antiga. Assim nao ha
-- janela sem validacao. SQL idempotente, schema-qualified e sem bloco Down.

do $$
begin
    if not exists (
        select 1
        from pg_constraint
        where conname = 'messaging_audit_events_event_type_e1_check'
          and conrelid = 'messaging.audit_events'::regclass
    ) then
        alter table messaging.audit_events
            add constraint messaging_audit_events_event_type_e1_check
            check (event_type in (
                'MESSAGE_OUTBOUND_QUEUED', 'MESSAGE_OUTBOUND_SENT', 'MESSAGE_OUTBOUND_FAILED',
                'CONVERSATION_STATUS_CHANGED', 'CONVERSATION_ASSIGNED',
                'MESSAGE_FORWARDED', 'MESSAGE_DELETED_FOR_ALL',
                'MESSAGE_MEDIA_READY', 'MESSAGE_MEDIA_FAILED', 'MESSAGE_MEDIA_RETRY'
            ))
            not valid;
    end if;
end
$$;

alter table messaging.audit_events
    validate constraint messaging_audit_events_event_type_e1_check;

alter table messaging.audit_events
    drop constraint if exists audit_events_event_type_check;
