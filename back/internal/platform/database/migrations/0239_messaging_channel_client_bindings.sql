-- CI-01: ownership historico de canal por cliente.
--
-- O Omnichannel continua dono do recurso de canal, da conversa e do envio. O
-- binding abaixo somente determina a qual cliente um recurso pertencia em um
-- instante. Conversas/touchpoints guardam snapshot; reatribuir um canal nunca
-- move historico silenciosamente.

create unique index if not exists messaging_whatsapp_instances_account_id_id_uidx
    on messaging.whatsapp_instances (account_id, id);

create table if not exists messaging.channel_client_bindings (
    id                       uuid primary key default gen_random_uuid(),
    account_id               uuid not null references core.accounts(id) on delete restrict,
    client_account_id        uuid not null references core.accounts(id) on delete restrict,
    channel                  text not null check (channel in ('WHATSAPP', 'INSTAGRAM')),
    whatsapp_instance_id     uuid,
    instagram_account_id     uuid,
    effective_from           timestamptz not null default now(),
    effective_to             timestamptz,
    source                   text not null default 'manual'
        check (source in ('manual', 'automation_profile_backfill', 'standalone_default')),
    source_ref               text,
    reason                   text not null check (char_length(btrim(reason)) between 1 and 500),
    revision                 bigint not null default 1 check (revision >= 1),
    created_by_user_id       uuid references core.users(id) on delete set null,
    ended_by_user_id         uuid references core.users(id) on delete set null,
    created_at               timestamptz not null default now(),
    updated_at               timestamptz not null default now(),
    constraint messaging_channel_client_bindings_resource_ck check (
        (channel = 'WHATSAPP' and whatsapp_instance_id is not null and instagram_account_id is null)
        or
        (channel = 'INSTAGRAM' and instagram_account_id is not null and whatsapp_instance_id is null)
    ),
    constraint messaging_channel_client_bindings_interval_ck check (
        effective_to is null or effective_to > effective_from
    ),
    constraint messaging_channel_client_bindings_account_whatsapp_fk
        foreign key (account_id, whatsapp_instance_id)
        references messaging.whatsapp_instances(account_id, id)
        on delete restrict,
    constraint messaging_channel_client_bindings_account_instagram_fk
        foreign key (account_id, instagram_account_id)
        references messaging.instagram_accounts(account_id, id)
        on delete restrict,
    constraint messaging_channel_client_bindings_account_id_uidx
        unique (account_id, id),
    constraint messaging_channel_client_bindings_snapshot_uidx
        unique (account_id, client_account_id, id)
);

create unique index if not exists messaging_channel_client_bindings_active_whatsapp_uidx
    on messaging.channel_client_bindings (account_id, whatsapp_instance_id)
    where effective_to is null and channel = 'WHATSAPP';
create unique index if not exists messaging_channel_client_bindings_active_instagram_uidx
    on messaging.channel_client_bindings (account_id, instagram_account_id)
    where effective_to is null and channel = 'INSTAGRAM';
create index if not exists messaging_channel_client_bindings_client_time_idx
    on messaging.channel_client_bindings
        (account_id, client_account_id, effective_to, effective_from desc);
create index if not exists messaging_channel_client_bindings_channel_time_idx
    on messaging.channel_client_bindings
        (account_id, channel, effective_to, updated_at desc);

create table if not exists messaging.channel_client_binding_events (
    id                    uuid primary key default gen_random_uuid(),
    account_id            uuid not null references core.accounts(id) on delete restrict,
    binding_id            uuid not null,
    successor_binding_id  uuid,
    event_type            text not null check (event_type in ('created', 'reassigned', 'ended')),
    reason                text not null check (char_length(btrim(reason)) between 1 and 500),
    idempotency_key       text not null check (char_length(btrim(idempotency_key)) between 1 and 200),
    request_hash          text not null check (char_length(request_hash) = 64),
    actor_user_id         uuid references core.users(id) on delete set null,
    snapshot              jsonb not null default '{}'::jsonb
        check (jsonb_typeof(snapshot) = 'object'),
    occurred_at           timestamptz not null default now(),
    constraint messaging_channel_client_binding_events_binding_fk
        foreign key (account_id, binding_id)
        references messaging.channel_client_bindings(account_id, id)
        on delete restrict,
    constraint messaging_channel_client_binding_events_successor_fk
        foreign key (account_id, successor_binding_id)
        references messaging.channel_client_bindings(account_id, id)
        on delete restrict,
    constraint messaging_channel_client_binding_events_idempotency_uidx
        unique (account_id, idempotency_key)
);

create index if not exists messaging_channel_client_binding_events_binding_time_idx
    on messaging.channel_client_binding_events (account_id, binding_id, occurred_at desc);

alter table messaging.conversations
    add column if not exists client_account_id uuid,
    add column if not exists channel_client_binding_id uuid,
    add column if not exists client_binding_state text not null default 'unresolved',
    add column if not exists client_bound_at timestamptz;

do $$
begin
    if not exists (
        select 1 from pg_constraint
        where conname = 'messaging_conversations_client_binding_state_ck'
    ) then
        alter table messaging.conversations
            add constraint messaging_conversations_client_binding_state_ck
            check (
                client_binding_state in ('resolved', 'unresolved', 'quarantined')
                and (
                    client_binding_state <> 'resolved'
                    or (
                        client_account_id is not null
                        and channel_client_binding_id is not null
                        and client_bound_at is not null
                    )
                )
            );
    end if;
    if not exists (
        select 1 from pg_constraint
        where conname = 'messaging_conversations_client_account_fk'
    ) then
        alter table messaging.conversations
            add constraint messaging_conversations_client_account_fk
            foreign key (client_account_id)
            references core.accounts(id)
            on delete restrict;
    end if;
    if not exists (
        select 1 from pg_constraint
        where conname = 'messaging_conversations_client_binding_fk'
    ) then
        alter table messaging.conversations
            add constraint messaging_conversations_client_binding_fk
            foreign key (account_id, client_account_id, channel_client_binding_id)
            references messaging.channel_client_bindings(account_id, client_account_id, id)
            on delete restrict;
    end if;
end $$;

create index if not exists messaging_conversations_client_last_message_idx
    on messaging.conversations (account_id, client_account_id, last_message_at desc);
create index if not exists messaging_conversations_binding_state_idx
    on messaging.conversations (account_id, client_binding_state, last_message_at desc);
create index if not exists messaging_conversations_binding_idx
    on messaging.conversations (account_id, channel_client_binding_id);

alter table messaging.contact_touchpoints
    add column if not exists client_account_id uuid,
    add column if not exists channel_client_binding_id uuid,
    add column if not exists client_binding_state text not null default 'unresolved';

do $$
begin
    if not exists (
        select 1 from pg_constraint
        where conname = 'messaging_contact_touchpoints_client_binding_state_ck'
    ) then
        alter table messaging.contact_touchpoints
            add constraint messaging_contact_touchpoints_client_binding_state_ck
            check (
                client_binding_state in ('resolved', 'unresolved', 'quarantined')
                and (
                    client_binding_state <> 'resolved'
                    or (client_account_id is not null and channel_client_binding_id is not null)
                )
            );
    end if;
    if not exists (
        select 1 from pg_constraint
        where conname = 'messaging_contact_touchpoints_client_account_fk'
    ) then
        alter table messaging.contact_touchpoints
            add constraint messaging_contact_touchpoints_client_account_fk
            foreign key (client_account_id)
            references core.accounts(id)
            on delete restrict;
    end if;
    if not exists (
        select 1 from pg_constraint
        where conname = 'messaging_contact_touchpoints_client_binding_fk'
    ) then
        alter table messaging.contact_touchpoints
            add constraint messaging_contact_touchpoints_client_binding_fk
            foreign key (account_id, client_account_id, channel_client_binding_id)
            references messaging.channel_client_bindings(account_id, client_account_id, id)
            on delete restrict;
    end if;
end $$;

create index if not exists messaging_contact_touchpoints_client_time_idx
    on messaging.contact_touchpoints (account_id, client_account_id, occurred_at desc);
create index if not exists messaging_contact_touchpoints_binding_state_idx
    on messaging.contact_touchpoints (account_id, client_binding_state, occurred_at desc);

create table if not exists messaging.channel_client_binding_repair_jobs (
    id                       uuid primary key default gen_random_uuid(),
    account_id               uuid not null references core.accounts(id) on delete restrict,
    channel                  text not null check (channel in ('WHATSAPP', 'INSTAGRAM')),
    whatsapp_instance_id     uuid,
    instagram_account_id     uuid,
    client_account_id        uuid not null references core.accounts(id) on delete restrict,
    binding_id               uuid not null,
    mode                     text not null check (mode in ('preview', 'apply')),
    status                   text not null default 'queued'
        check (status in ('queued', 'processing', 'completed', 'partial', 'failed', 'cancelled')),
    filters                  jsonb not null default '{}'::jsonb
        check (jsonb_typeof(filters) = 'object'),
    watermark               timestamptz not null,
    preview_job_id           uuid,
    preview_checksum         text not null default '',
    idempotency_key          text not null check (char_length(btrim(idempotency_key)) between 1 and 200),
    request_hash             text not null check (char_length(request_hash) = 64),
    scanned_count            bigint not null default 0,
    eligible_count           bigint not null default 0,
    repaired_count           bigint not null default 0,
    quarantined_count        bigint not null default 0,
    skipped_count            bigint not null default 0,
    cursor                   text not null default '',
    attempts                 integer not null default 0,
    locked_at                timestamptz,
    locked_by                text not null default '',
    last_error_code          text not null default '',
    report_ref               text not null default '',
    actor_user_id            uuid references core.users(id) on delete set null,
    reason                   text not null check (char_length(btrim(reason)) between 1 and 500),
    created_at               timestamptz not null default now(),
    started_at               timestamptz,
    completed_at             timestamptz,
    updated_at               timestamptz not null default now(),
    constraint messaging_channel_client_binding_repair_resource_ck check (
        (channel = 'WHATSAPP' and whatsapp_instance_id is not null and instagram_account_id is null)
        or
        (channel = 'INSTAGRAM' and instagram_account_id is not null and whatsapp_instance_id is null)
    ),
    constraint messaging_channel_client_binding_repair_binding_fk
        foreign key (account_id, client_account_id, binding_id)
        references messaging.channel_client_bindings(account_id, client_account_id, id)
        on delete restrict,
    constraint messaging_channel_client_binding_repair_preview_fk
        foreign key (preview_job_id)
        references messaging.channel_client_binding_repair_jobs(id)
        on delete restrict,
    constraint messaging_channel_client_binding_repair_idempotency_uidx
        unique (account_id, idempotency_key)
);

create index if not exists messaging_channel_client_binding_repair_status_idx
    on messaging.channel_client_binding_repair_jobs (account_id, status, created_at desc);

-- Capabilities sensiveis nascem explicitamente desligadas. O runtime consulta
-- esta policy; prompt algum pode liga-la.
insert into messaging.account_config (account_id)
select a.id
from core.accounts a
where a.is_active = true
on conflict (account_id) do nothing;

alter table messaging.account_config
    add column if not exists customer_intelligence_mode text not null default 'off'
        check (customer_intelligence_mode in ('off', 'shadow', 'on')),
    add column if not exists channel_binding_mode text not null default 'shadow'
        check (channel_binding_mode in ('legacy', 'shadow', 'enforced')),
    add column if not exists integration_policy_revision bigint not null default 1
        check (integration_policy_revision >= 1);

-- Nenhum DROP/backfill destrutivo. A migration 0244 futura pode endurecer
-- nullability somente depois de preview, reparo e cutover aprovados.
