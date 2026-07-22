-- E4 CRM: audit-friendly, reversible contact merge metadata.
alter table messaging.contact_merge_events
    add column if not exists undone_at timestamptz,
    add column if not exists undo_actor_user_id uuid references core.users(id) on delete set null;

alter table messaging.audit_events
    drop constraint if exists audit_events_event_type_check;
alter table messaging.audit_events
    add constraint audit_events_event_type_check check (event_type in (
        'MESSAGE_OUTBOUND_QUEUED', 'MESSAGE_OUTBOUND_SENT', 'MESSAGE_OUTBOUND_FAILED',
        'CONVERSATION_STATUS_CHANGED', 'CONVERSATION_ASSIGNED',
        'MESSAGE_FORWARDED', 'MESSAGE_DELETED_FOR_ALL', 'CONTACT_MERGED', 'CONTACT_MERGE_UNDONE'
    ));

alter table messaging.contact_touchpoints
    drop constraint if exists contact_touchpoints_channel_check;
alter table messaging.contact_touchpoints
    add constraint messaging_contact_touchpoints_channel_ck
    check (channel in ('WHATSAPP', 'INSTAGRAM', 'LANDING_PAGE'));
