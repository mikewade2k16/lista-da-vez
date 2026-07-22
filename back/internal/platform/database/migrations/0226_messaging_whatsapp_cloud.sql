-- E7: WhatsApp Cloud API metadata. The existing messages/conversations/outbox remain
-- authoritative; this migration only adds provider-specific configuration and policy state.
create unique index if not exists messaging_whatsapp_instances_account_id_id_uidx
    on messaging.whatsapp_instances (account_id, id);
create unique index if not exists messaging_conversations_account_id_id_uidx
    on messaging.conversations (account_id, id);

create table if not exists messaging.whatsapp_templates (
    id                  uuid primary key default gen_random_uuid(),
    account_id          uuid not null references core.accounts(id) on delete cascade,
    instance_id         uuid not null references messaging.whatsapp_instances(id) on delete cascade,
    meta_template_id    text not null,
    name                text not null,
    language            text not null,
    category            text not null default 'UTILITY',
    status              text not null default 'PENDING',
    components          jsonb not null default '[]'::jsonb,
    quality_rating      text,
    last_synced_at      timestamptz,
    created_at          timestamptz not null default now(),
    updated_at          timestamptz not null default now(),
    constraint whatsapp_templates_status_ck check (status in ('APPROVED','PENDING','REJECTED','PAUSED','DISABLED','UNKNOWN')),
    constraint whatsapp_templates_account_instance_fk foreign key (account_id, instance_id)
        references messaging.whatsapp_instances(account_id, id) on delete cascade
);

create unique index if not exists messaging_whatsapp_templates_account_instance_name_lang_uidx
    on messaging.whatsapp_templates (account_id, instance_id, name, language);
create unique index if not exists messaging_whatsapp_templates_account_meta_uidx
    on messaging.whatsapp_templates (account_id, instance_id, meta_template_id);
create index if not exists messaging_whatsapp_templates_account_status_idx
    on messaging.whatsapp_templates (account_id, instance_id, status, name);

create table if not exists messaging.channel_windows (
    id                  uuid primary key default gen_random_uuid(),
    account_id          uuid not null references core.accounts(id) on delete cascade,
    conversation_id     uuid not null references messaging.conversations(id) on delete cascade,
    provider            text not null,
    window_kind         text not null default 'customer_service',
    opened_at           timestamptz not null,
    expires_at          timestamptz not null,
    source_message_id   uuid references messaging.messages(id) on delete set null,
    updated_at          timestamptz not null default now(),
    constraint channel_windows_expiry_ck check (expires_at > opened_at),
    constraint channel_windows_account_conversation_fk foreign key (account_id, conversation_id)
        references messaging.conversations(account_id, id) on delete cascade
);

create unique index if not exists messaging_channel_windows_account_conv_provider_kind_uidx
    on messaging.channel_windows (account_id, conversation_id, provider, window_kind);
create index if not exists messaging_channel_windows_account_expiry_idx
    on messaging.channel_windows (account_id, provider, expires_at);

comment on table messaging.whatsapp_templates is 'Meta Cloud templates mirrored from provider; status is never manually approved.';
comment on table messaging.channel_windows is 'Provider customer-service windows; Go revalidates on every outbox claim.';
