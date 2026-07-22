-- Omnichannel E3: configuração multimodal versionada e análises de mídia.
--
-- A mídia binária continua no storage privado do módulo. Esta migration guarda somente
-- metadados, resultado estruturado limitado pelo contrato e custo da análise. O worker,
-- policy, token assinado e persistência de resultados pertencem ao pacote E3-BE-03.
-- SQL idempotente, schema-qualified e sem Down.

create schema if not exists messaging;

-- A configuração é parte da versão imutável do agente. A API E3 valida o shape fechado;
-- o banco garante ao menos que o valor raiz seja um objeto JSON e nunca aceita segredo cru.
alter table messaging.ai_agent_versions
    add column if not exists media_config jsonb not null default '{}'::jsonb;

do $$
begin
    if not exists (
        select 1
        from pg_constraint
        where conname = 'messaging_ai_versions_media_config_object_ck'
          and conrelid = 'messaging.ai_agent_versions'::regclass
    ) then
        alter table messaging.ai_agent_versions
            add constraint messaging_ai_versions_media_config_object_ck
            check (jsonb_typeof(media_config) = 'object')
            not valid;
    end if;
end
$$;

alter table messaging.ai_agent_versions
    validate constraint messaging_ai_versions_media_config_object_ck;

-- Uma análise é sempre escopada pela conta, mensagem e conversa. As FKs compostas impedem
-- que um ID de outra conta seja associado por acidente, mesmo que o UUID exista no banco.
create table if not exists messaging.media_analyses (
    id                 uuid primary key default gen_random_uuid(),
    account_id         uuid not null references core.accounts(id) on delete cascade,
    message_id         uuid not null,
    conversation_id    uuid not null,
    analysis_kind      text not null
        check (analysis_kind in ('transcription', 'vision', 'document_text')),
    content_hash       text not null
        check (content_hash ~ '^[0-9a-f]{64}$'),
    status             text not null default 'queued'
        check (status in ('queued', 'processing', 'completed', 'failed', 'blocked')),
    provider           text not null default '',
    model              text not null default '',
    agent_version_id   uuid not null,
    result_text        text,
    result_json        jsonb not null default '{}'::jsonb
        check (jsonb_typeof(result_json) = 'object'),
    prompt_tokens      integer not null default 0 check (prompt_tokens >= 0),
    completion_tokens  integer not null default 0 check (completion_tokens >= 0),
    cost_usd           numeric(12, 6) not null default 0 check (cost_usd >= 0),
    latency_ms         integer not null default 0 check (latency_ms >= 0),
    attempts           integer not null default 0 check (attempts >= 0),
    last_error         text not null default '',
    created_at         timestamptz not null default now(),
    completed_at       timestamptz,
    expires_at         timestamptz,
    constraint messaging_media_analyses_message_fk
        foreign key (account_id, message_id)
        references messaging.messages(account_id, id) on delete cascade,
    constraint messaging_media_analyses_conversation_fk
        foreign key (account_id, conversation_id)
        references messaging.conversations(account_id, id) on delete cascade,
    constraint messaging_media_analyses_agent_version_fk
        foreign key (account_id, agent_version_id)
        references messaging.ai_agent_versions(account_id, id) on delete restrict
);

create unique index if not exists messaging_media_analyses_dedupe_uidx
    on messaging.media_analyses (
        account_id, message_id, analysis_kind, content_hash, provider, model, agent_version_id
    );

create index if not exists messaging_media_analyses_message_idx
    on messaging.media_analyses (account_id, message_id, created_at desc);

create index if not exists messaging_media_analyses_conversation_idx
    on messaging.media_analyses (account_id, conversation_id, created_at desc);

create index if not exists messaging_media_analyses_status_created_idx
    on messaging.media_analyses (account_id, status, created_at, id);

create index if not exists messaging_media_analyses_expires_idx
    on messaging.media_analyses (account_id, expires_at)
    where expires_at is not null;
