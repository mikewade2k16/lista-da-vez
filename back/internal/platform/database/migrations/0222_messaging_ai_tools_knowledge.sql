-- Omnichannel E6-DB-02: bindings/run auditados e conhecimento com FTS.
-- Não cria catálogo paralelo do módulo Tools, não guarda credenciais e não instala pgvector.
-- n8n continua apenas orquestrador; execução/autorização permanecem no Go.

create schema if not exists messaging;

create unique index if not exists messaging_ai_agents_account_id_uidx
    on messaging.ai_agents (account_id, id);
create unique index if not exists messaging_ai_dispatches_account_id_uidx
    on messaging.ai_dispatches (account_id, id);

create table if not exists messaging.ai_tool_bindings (
    id                     uuid primary key default gen_random_uuid(),
    account_id             uuid not null references core.accounts(id) on delete cascade,
    agent_id               uuid not null,
    tool_id                text not null check (length(tool_id) between 1 and 160),
    is_enabled             boolean not null default false,
    mode                   text not null default 'read'
        check (mode in ('read','propose_write','approved_write')),
    allowed_operations     jsonb not null default '[]'::jsonb
        check (jsonb_typeof(allowed_operations) = 'array' and jsonb_array_length(allowed_operations) <= 32),
    input_schema           jsonb not null default '{}'::jsonb
        check (jsonb_typeof(input_schema) = 'object'),
    output_schema          jsonb not null default '{}'::jsonb
        check (jsonb_typeof(output_schema) = 'object'),
    timeout_ms             integer not null default 5000 check (timeout_ms between 100 and 30000),
    max_calls_per_dispatch integer not null default 4 check (max_calls_per_dispatch between 1 and 20),
    config                 jsonb not null default '{}'::jsonb
        check (jsonb_typeof(config) = 'object'),
    created_at             timestamptz not null default now(),
    updated_at             timestamptz not null default now(),
    constraint messaging_ai_tool_bindings_agent_fk
        foreign key (account_id, agent_id)
        references messaging.ai_agents(account_id, id) on delete cascade
);
create unique index if not exists messaging_ai_tool_bindings_identity_uidx
    on messaging.ai_tool_bindings (account_id, agent_id, tool_id);
create unique index if not exists messaging_ai_tool_bindings_account_id_uidx
    on messaging.ai_tool_bindings (account_id, id);
create index if not exists messaging_ai_tool_bindings_enabled_idx
    on messaging.ai_tool_bindings (account_id, agent_id, is_enabled);

create table if not exists messaging.ai_tool_runs (
    id              uuid primary key default gen_random_uuid(),
    account_id      uuid not null references core.accounts(id) on delete cascade,
    conversation_id uuid,
    dispatch_id     uuid,
    ai_run_id       uuid,
    binding_id      uuid not null,
    call_id         text not null check (length(call_id) between 1 and 160),
    status          text not null default 'requested'
        check (status in ('requested','approved','denied','running','completed','failed','timeout')),
    operation       text not null check (length(operation) between 1 and 160),
    input_masked    jsonb not null default '{}'::jsonb
        check (jsonb_typeof(input_masked) = 'object'),
    output_masked   jsonb not null default '{}'::jsonb
        check (jsonb_typeof(output_masked) = 'object'),
    latency_ms      integer not null default 0 check (latency_ms between 0 and 600000),
    error           text not null default '' check (length(error) <= 2000),
    created_at      timestamptz not null default now(),
    completed_at    timestamptz,
    constraint messaging_ai_tool_runs_binding_fk
        foreign key (account_id, binding_id)
        references messaging.ai_tool_bindings(account_id, id) on delete restrict,
    constraint messaging_ai_tool_runs_conversation_fk
        foreign key (account_id, conversation_id)
        references messaging.conversations(account_id, id) on delete cascade,
    constraint messaging_ai_tool_runs_dispatch_fk
        foreign key (account_id, dispatch_id)
        references messaging.ai_dispatches(account_id, id) on delete set null,
    constraint messaging_ai_tool_runs_ai_run_fk
        foreign key (account_id, ai_run_id)
        references messaging.ai_runs(account_id, id) on delete set null
);
create unique index if not exists messaging_ai_tool_runs_call_uidx
    on messaging.ai_tool_runs (account_id, dispatch_id, call_id)
    where dispatch_id is not null;
create index if not exists messaging_ai_tool_runs_conversation_created_idx
    on messaging.ai_tool_runs (account_id, conversation_id, created_at desc);

create table if not exists messaging.knowledge_bases (
    id            uuid primary key default gen_random_uuid(),
    account_id    uuid not null references core.accounts(id) on delete cascade,
    name          text not null check (length(name) between 1 and 200),
    is_enabled    boolean not null default false,
    search_config jsonb not null default '{}'::jsonb
        check (jsonb_typeof(search_config) = 'object'),
    created_at    timestamptz not null default now(),
    updated_at    timestamptz not null default now()
);
create unique index if not exists messaging_knowledge_bases_name_uidx
    on messaging.knowledge_bases (account_id, name);
create unique index if not exists messaging_knowledge_bases_account_id_uidx
    on messaging.knowledge_bases (account_id, id);

create table if not exists messaging.knowledge_documents (
    id           uuid primary key default gen_random_uuid(),
    account_id   uuid not null references core.accounts(id) on delete cascade,
    knowledge_base_id uuid not null,
    source_ref   text not null check (length(source_ref) between 1 and 1000),
    title        text not null default '' check (length(title) <= 500),
    checksum     text not null check (length(checksum) between 1 and 128),
    status       text not null default 'draft'
        check (status in ('draft','processing','published','failed','archived')),
    version      integer not null default 1 check (version > 0),
    metadata     jsonb not null default '{}'::jsonb
        check (jsonb_typeof(metadata) = 'object'),
    error        text not null default '' check (length(error) <= 2000),
    created_at   timestamptz not null default now(),
    updated_at   timestamptz not null default now(),
    constraint messaging_knowledge_documents_base_fk
        foreign key (account_id, knowledge_base_id)
        references messaging.knowledge_bases(account_id, id) on delete cascade
);
create unique index if not exists messaging_knowledge_documents_version_uidx
    on messaging.knowledge_documents (account_id, knowledge_base_id, checksum, version);
create unique index if not exists messaging_knowledge_documents_account_id_uidx
    on messaging.knowledge_documents (account_id, id);
create index if not exists messaging_knowledge_documents_status_idx
    on messaging.knowledge_documents (account_id, knowledge_base_id, status, version desc);

create table if not exists messaging.knowledge_chunks (
    id            uuid primary key default gen_random_uuid(),
    account_id    uuid not null references core.accounts(id) on delete cascade,
    document_id   uuid not null,
    ordinal       integer not null check (ordinal >= 0),
    body_text     text not null check (length(body_text) between 1 and 20000),
    token_count   integer not null default 0 check (token_count >= 0 and token_count <= 10000),
    search_vector tsvector generated always as (to_tsvector('simple'::regconfig, body_text)) stored,
    created_at    timestamptz not null default now(),
    constraint messaging_knowledge_chunks_document_fk
        foreign key (account_id, document_id)
        references messaging.knowledge_documents(account_id, id) on delete cascade
);
create unique index if not exists messaging_knowledge_chunks_ordinal_uidx
    on messaging.knowledge_chunks (account_id, document_id, ordinal);
create index if not exists messaging_knowledge_chunks_search_idx
    on messaging.knowledge_chunks using gin (search_vector);

create table if not exists messaging.ai_knowledge_bindings (
    id               uuid primary key default gen_random_uuid(),
    account_id       uuid not null references core.accounts(id) on delete cascade,
    agent_id         uuid not null,
    knowledge_base_id uuid not null,
    is_enabled       boolean not null default false,
    top_k            integer not null default 5 check (top_k between 1 and 20),
    min_score        real not null default 0 check (min_score between 0 and 1),
    created_at       timestamptz not null default now(),
    updated_at       timestamptz not null default now(),
    constraint messaging_ai_knowledge_bindings_agent_fk
        foreign key (account_id, agent_id)
        references messaging.ai_agents(account_id, id) on delete cascade,
    constraint messaging_ai_knowledge_bindings_base_fk
        foreign key (account_id, knowledge_base_id)
        references messaging.knowledge_bases(account_id, id) on delete cascade
);
create unique index if not exists messaging_ai_knowledge_bindings_identity_uidx
    on messaging.ai_knowledge_bindings (account_id, agent_id, knowledge_base_id);
