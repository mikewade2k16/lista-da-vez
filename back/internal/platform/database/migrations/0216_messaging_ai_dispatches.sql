-- Omnichannel E2: contrato brain.v2, configuração durável e dispatches de IA.
-- O worker/n8n ainda não é ligado por esta migration: ela somente cria a fonte autoritativa
-- que os pacotes E2-BE-03/E2-BE-05 consumirão.
--
-- Idempotente, schema-qualified, sem Down. Nenhuma chave de provider ou PII é armazenada aqui.

create schema if not exists messaging;

-- Configuração publicada do agente. Versões já existentes recebem defaults compatíveis com
-- brain.v2; editar uma versão publicada continua proibido pelo service (nova versão/rollback).
alter table messaging.ai_agent_versions
    add column if not exists debounce_ms integer not null default 2500,
    add column if not exists max_context_messages integer not null default 30,
    add column if not exists max_ai_turns integer not null default 6,
    add column if not exists min_confidence numeric(4,3) not null default 0.650,
    add column if not exists handoff_on_error boolean not null default true,
    add column if not exists handoff_on_limit boolean not null default true,
    add column if not exists workflow_contract_version text not null default 'brain.v2';

do $$
begin
    if not exists (select 1 from pg_constraint where conname = 'messaging_ai_versions_debounce_ck') then
        alter table messaging.ai_agent_versions
            add constraint messaging_ai_versions_debounce_ck
            check (debounce_ms between 500 and 15000);
    end if;
    if not exists (select 1 from pg_constraint where conname = 'messaging_ai_versions_context_ck') then
        alter table messaging.ai_agent_versions
            add constraint messaging_ai_versions_context_ck
            check (max_context_messages between 1 and 100);
    end if;
    if not exists (select 1 from pg_constraint where conname = 'messaging_ai_versions_turns_ck') then
        alter table messaging.ai_agent_versions
            add constraint messaging_ai_versions_turns_ck
            check (max_ai_turns between 1 and 20);
    end if;
    if not exists (select 1 from pg_constraint where conname = 'messaging_ai_versions_confidence_ck') then
        alter table messaging.ai_agent_versions
            add constraint messaging_ai_versions_confidence_ck
            check (min_confidence between 0 and 1);
    end if;
    if not exists (select 1 from pg_constraint where conname = 'messaging_ai_versions_contract_ck') then
        alter table messaging.ai_agent_versions
            add constraint messaging_ai_versions_contract_ck
            check (workflow_contract_version <> '' and length(workflow_contract_version) <= 64);
    end if;
end $$;

-- Composite uniqueness permits tenant-safe foreign keys below. The primary key remains the
-- public UUID, but an id from another account can no longer satisfy a dispatch FK.
create unique index if not exists messaging_conversations_account_id_uidx
    on messaging.conversations (account_id, id);
create unique index if not exists messaging_ai_agent_versions_account_id_uidx
    on messaging.ai_agent_versions (account_id, id);
create unique index if not exists messaging_ai_runs_account_id_uidx
    on messaging.ai_runs (account_id, id);

create table if not exists messaging.ai_dispatches (
    id                  uuid primary key default gen_random_uuid(),
    account_id          uuid not null references core.accounts(id) on delete cascade,
    conversation_id     uuid not null,
    agent_version_id    uuid not null,
    generation          bigint not null check (generation >= 0),
    status              text not null default 'buffering'
        check (status in ('buffering', 'queued', 'processing', 'completed', 'cancelled', 'failed')),
    message_ids         uuid[] not null,
    run_after           timestamptz not null,
    locked_at           timestamptz,
    completed_at        timestamptz,
    idempotency_key     text not null,
    result_run_id       uuid,
    last_error          text not null default '',
    created_at          timestamptz not null default now(),
    updated_at          timestamptz not null default now(),
    constraint messaging_ai_dispatches_message_ids_ck check (cardinality(message_ids) > 0),
    constraint messaging_ai_dispatches_conversation_fk
        foreign key (account_id, conversation_id)
        references messaging.conversations(account_id, id) on delete cascade,
    constraint messaging_ai_dispatches_agent_version_fk
        foreign key (account_id, agent_version_id)
        references messaging.ai_agent_versions(account_id, id) on delete restrict,
    constraint messaging_ai_dispatches_result_run_fk
        foreign key (account_id, result_run_id)
        references messaging.ai_runs(account_id, id) on delete set null
);

create unique index if not exists messaging_ai_dispatches_account_idempotency_uidx
    on messaging.ai_dispatches (account_id, idempotency_key);
create unique index if not exists messaging_ai_dispatches_generation_uidx
    on messaging.ai_dispatches (account_id, conversation_id, generation);
create unique index if not exists messaging_ai_dispatches_active_conversation_uidx
    on messaging.ai_dispatches (account_id, conversation_id)
    where status in ('buffering', 'queued', 'processing');
create index if not exists messaging_ai_dispatches_claim_idx
    on messaging.ai_dispatches (status, run_after, created_at)
    where status in ('buffering', 'queued', 'processing');
create index if not exists messaging_ai_dispatches_account_created_idx
    on messaging.ai_dispatches (account_id, created_at desc);
