-- P1A: acesso relacional por instancia de WhatsApp.
--
-- A autorizacao nasce fechada (RESTRICTED). provider_config continua armazenado por uma
-- versao apenas para compatibilidade/rollback, mas os grants abaixo sao a nova fonte
-- relacional preparada para o resolver canonico do P1.

alter table messaging.whatsapp_instances
    add column if not exists access_policy text not null default 'RESTRICTED';

alter table messaging.whatsapp_instances
    add column if not exists access_revision bigint not null default 0;

do $$
begin
    if not exists (
        select 1 from pg_constraint
        where conname = 'messaging_whatsapp_instances_access_policy_ck'
          and conrelid = 'messaging.whatsapp_instances'::regclass
    ) then
        alter table messaging.whatsapp_instances
            add constraint messaging_whatsapp_instances_access_policy_ck
            check (access_policy in ('ACCOUNT_SHARED', 'RESTRICTED'));
    end if;
    if not exists (
        select 1 from pg_constraint
        where conname = 'messaging_whatsapp_instances_access_revision_ck'
          and conrelid = 'messaging.whatsapp_instances'::regclass
    ) then
        alter table messaging.whatsapp_instances
            add constraint messaging_whatsapp_instances_access_revision_ck
            check (access_revision >= 0);
    end if;
end $$;

create table if not exists messaging.whatsapp_instance_user_grants (
    account_id          uuid not null references core.accounts(id) on delete cascade,
    instance_id         uuid not null,
    user_id             uuid not null,
    access_level        text not null,
    is_active           boolean not null default true,
    revision            bigint not null default 1,
    granted_by_user_id  uuid references core.users(id) on delete set null,
    updated_by_user_id  uuid references core.users(id) on delete set null,
    revoked_by_user_id  uuid references core.users(id) on delete set null,
    revoked_at          timestamptz,
    created_at          timestamptz not null default now(),
    updated_at          timestamptz not null default now(),
    constraint messaging_whatsapp_instance_user_grants_pk
        primary key (account_id, instance_id, user_id),
    constraint messaging_whatsapp_instance_user_grants_instance_fk
        foreign key (account_id, instance_id)
        references messaging.whatsapp_instances(account_id, id) on delete cascade,
    constraint messaging_whatsapp_instance_user_grants_membership_fk
        foreign key (account_id, user_id)
        references core.account_users(account_id, user_id) on delete cascade,
    constraint messaging_whatsapp_instance_user_grants_level_ck
        check (access_level in ('view', 'reply', 'manage')),
    constraint messaging_whatsapp_instance_user_grants_revision_ck
        check (revision >= 1),
    constraint messaging_whatsapp_instance_user_grants_revocation_ck check (
        (is_active = true and revoked_by_user_id is null and revoked_at is null)
        or
        (is_active = false and revoked_at is not null)
    )
);

create index if not exists messaging_whatsapp_instance_grants_account_user_active_idx
    on messaging.whatsapp_instance_user_grants (account_id, user_id)
    where is_active = true;

create index if not exists messaging_whatsapp_instance_grants_account_instance_active_idx
    on messaging.whatsapp_instance_user_grants (account_id, instance_id)
    where is_active = true;

create index if not exists messaging_whatsapp_instance_grants_account_instance_level_idx
    on messaging.whatsapp_instance_user_grants (account_id, instance_id, access_level);

-- Backfill idempotente. A ordem impede que assignedUserIds rebaixe um manage valido.
-- Usuario fora da account, membership inativa e UUID invalido nao entram; o relatorio do
-- repository os classifica a partir do JSON legado que permanece nesta versao.
insert into messaging.whatsapp_instance_user_grants
    (account_id, instance_id, user_id, access_level)
select wi.account_id, wi.id, wi.responsible_user_id, 'manage'
from messaging.whatsapp_instances wi
join core.account_users au
  on au.account_id = wi.account_id
 and au.user_id = wi.responsible_user_id
 and au.is_active = true
where wi.responsible_user_id is not null
on conflict (account_id, instance_id, user_id) do nothing;

insert into messaging.whatsapp_instance_user_grants
    (account_id, instance_id, user_id, access_level)
select wi.account_id, wi.id, wi.created_by_user_id, 'manage'
from messaging.whatsapp_instances wi
join core.account_users creator
  on creator.account_id = wi.account_id
 and creator.user_id = wi.created_by_user_id
 and creator.is_active = true
where wi.created_by_user_id is not null
  and not exists (
      select 1
      from core.account_users responsible
      where responsible.account_id = wi.account_id
        and responsible.user_id = wi.responsible_user_id
        and responsible.is_active = true
  )
on conflict (account_id, instance_id, user_id) do nothing;

insert into messaging.whatsapp_instance_user_grants
    (account_id, instance_id, user_id, access_level)
select wi.account_id, wi.id, member.user_id, 'reply'
from messaging.whatsapp_instances wi
cross join lateral jsonb_array_elements_text(
    case
        when jsonb_typeof(wi.provider_config -> 'assignedUserIds') = 'array'
            then wi.provider_config -> 'assignedUserIds'
        else '[]'::jsonb
    end
) assigned(raw_user_id)
join core.account_users member
  on member.account_id = wi.account_id
 and member.user_id::text = btrim(assigned.raw_user_id)
 and member.is_active = true
on conflict (account_id, instance_id, user_id) do nothing;

-- O vocabulario de auditoria e fechado. Preserve a uniao vigente da 0297 e acrescente
-- somente o evento de acesso por instancia.
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
        'WHATSAPP_INSTANCE_HISTORY_RESET',
        'WHATSAPP_INSTANCE_ACCESS_CHANGED'
    ));

-- Uma unica trilha por instancia documenta o corte do metadata legado para a fonte
-- relacional, inclusive quando a instancia ficou sem manage e exige correcao manual.
insert into messaging.audit_events (account_id, event_type, payload_json)
select wi.account_id, 'WHATSAPP_INSTANCE_ACCESS_CHANGED', jsonb_build_object(
    'source', 'p1_access_backfill',
    'instanceId', wi.id,
    'before', jsonb_build_object('authorizationSource', 'legacy_metadata'),
    'after', jsonb_build_object(
        'accessPolicy', wi.access_policy,
        'accessRevision', wi.access_revision,
        'activeManageCount', count(grant_row.user_id) filter (
            where grant_row.is_active and grant_row.access_level = 'manage'
        ),
        'activeReplyCount', count(grant_row.user_id) filter (
            where grant_row.is_active and grant_row.access_level = 'reply'
        )
    )
)
from messaging.whatsapp_instances wi
left join messaging.whatsapp_instance_user_grants grant_row
  on grant_row.account_id = wi.account_id and grant_row.instance_id = wi.id
where not exists (
    select 1 from messaging.audit_events existing
    where existing.account_id = wi.account_id
      and existing.event_type = 'WHATSAPP_INSTANCE_ACCESS_CHANGED'
      and existing.payload_json ->> 'source' = 'p1_access_backfill'
      and existing.payload_json ->> 'instanceId' = wi.id::text
)
group by wi.account_id, wi.id, wi.access_policy, wi.access_revision;
