-- Backfill das conversas que existiam antes do CRM automatico da 0211.

with ranked as (
    select c.account_id,
           regexp_replace(coalesce(c.contact_phone, ''), '[^0-9]', '', 'g') as phone,
           coalesce(nullif(max(c.contact_name), ''),
                    regexp_replace(coalesce(c.contact_phone, ''), '[^0-9]', '', 'g')) as name,
           max(c.contact_avatar_url) as avatar_url,
           min(c.created_at) as first_seen_at,
           max(c.last_message_at) as last_seen_at,
           min(c.channel) as first_channel,
           max(c.channel) as last_channel
    from messaging.conversations c
    where regexp_replace(coalesce(c.contact_phone, ''), '[^0-9]', '', 'g') <> ''
    group by c.account_id, regexp_replace(coalesce(c.contact_phone, ''), '[^0-9]', '', 'g')
)
insert into messaging.contacts
    (account_id, name, phone, avatar_url, source, first_seen_at, last_seen_at,
     first_channel, last_channel, relationship_status)
select account_id, name, phone, avatar_url, first_channel || '_BACKFILL',
       first_seen_at, last_seen_at, first_channel, last_channel, 'lead'
from ranked
on conflict (account_id, phone) where phone is not null and phone <> '' do update
set last_seen_at = greatest(contacts.last_seen_at, excluded.last_seen_at),
    last_channel = excluded.last_channel,
    avatar_url = coalesce(contacts.avatar_url, excluded.avatar_url),
    updated_at = now();

update messaging.conversations c
set contact_id = ct.id,
    contact_phone = ct.phone,
    extracted_fields = coalesce(c.extracted_fields, '{}'::jsonb) || jsonb_build_object(
        'crm_contact_status', 'known_contact',
        'source_channel', lower(c.channel),
        'source_provider', coalesce(wi.provider, 'unknown'),
        'source_kind', 'legacy_backfill'
    ),
    updated_at = now()
from messaging.contacts ct
left join messaging.whatsapp_instances wi
  on wi.account_id = ct.account_id
where c.account_id = ct.account_id
  and ct.phone = regexp_replace(coalesce(c.contact_phone, ''), '[^0-9]', '', 'g')
  and (c.instance_id is null or wi.id = c.instance_id);

insert into messaging.contact_identities
    (account_id, contact_id, channel, provider, instance_scope_key, external_id,
     display_name, avatar_url, first_seen_at, last_seen_at)
select c.account_id, c.contact_id, c.channel, coalesce(wi.provider, 'unknown'),
       c.instance_scope_key, c.external_id, c.contact_name, c.contact_avatar_url,
       c.created_at, c.last_message_at
from messaging.conversations c
left join messaging.whatsapp_instances wi
  on wi.account_id = c.account_id and wi.id = c.instance_id
where c.contact_id is not null and c.external_id <> ''
on conflict (account_id, channel, provider, instance_scope_key, external_id) do update
set contact_id = excluded.contact_id,
    display_name = coalesce(excluded.display_name, contact_identities.display_name),
    avatar_url = coalesce(excluded.avatar_url, contact_identities.avatar_url),
    last_seen_at = greatest(contact_identities.last_seen_at, excluded.last_seen_at),
    updated_at = now();

create unique index if not exists messaging_contact_touchpoints_backfill_uidx
    on messaging.contact_touchpoints (account_id, conversation_id, source_kind)
    where source_kind = 'legacy_backfill';

insert into messaging.contact_touchpoints
    (account_id, contact_id, conversation_id, message_id, channel, provider,
     source_kind, occurred_at)
select distinct on (c.account_id, c.id)
       c.account_id, c.contact_id, c.id, m.id, c.channel, coalesce(wi.provider, 'unknown'),
       'legacy_backfill', m.created_at
from messaging.conversations c
join messaging.messages m
  on m.account_id = c.account_id and m.conversation_id = c.id and m.direction = 'INBOUND'
left join messaging.whatsapp_instances wi
  on wi.account_id = c.account_id and wi.id = c.instance_id
where c.contact_id is not null
order by c.account_id, c.id, m.created_at, m.id
on conflict (account_id, conversation_id, source_kind)
    where source_kind = 'legacy_backfill' do nothing;

