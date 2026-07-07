-- Modulo Calendario — persistencia do chat com memoria (WAVE 4, contrato D1)
-- Plano: docs/CALENDARIO_SPECS4.md (SPEC-B10).
--
-- O chat do calendario (Crow Assistant) passa a PERSISTIR conversas e mensagens
-- (antes era stateless: a memoria vivia so no n8n por sessionKey). Duas tabelas no
-- schema calendar:
--   - chat_conversations: uma conversa por (account, usuario), com escopo do chat
--     (scope_mode 'client'|'all' + scope_client_id quando 'client'). Soft-delete via
--     deleted_at. created_by_user_id = dono (cliente-side ve so as suas; agencia ve todas).
--   - chat_messages: as mensagens (role 'user'|'assistant'), tambem account-scoped
--     (defesa em profundidade) e amarradas a conversa (FK on delete cascade).
-- account_id em AMBAS as tabelas isola por tenant (conta A nunca le conversa/mensagem de B).
-- Idempotente, schema qualificado, sem `-- +goose Down` (o migrator roda o arquivo inteiro).

create schema if not exists calendar;

create table if not exists calendar.chat_conversations (
    id                 uuid primary key default gen_random_uuid(),
    account_id         uuid not null references core.accounts(id) on delete cascade,
    created_by_user_id uuid not null references core.users(id) on delete cascade,
    title              text not null default '',
    scope_mode         text not null default 'client',   -- 'client' | 'all'
    scope_client_id    uuid,                              -- preenchido quando scope_mode='client'
    created_at         timestamptz not null default now(),
    updated_at         timestamptz not null default now(),
    deleted_at         timestamptz
);
-- Indice do list: por account + dono, ordenado por updated_at desc (conversas vivas).
create index if not exists calendar_chat_conv_idx
    on calendar.chat_conversations (account_id, created_by_user_id, updated_at desc)
    where deleted_at is null;

create table if not exists calendar.chat_messages (
    id              uuid primary key default gen_random_uuid(),
    conversation_id uuid not null references calendar.chat_conversations(id) on delete cascade,
    account_id      uuid not null references core.accounts(id) on delete cascade,
    role            text not null,   -- 'user' | 'assistant'
    content         text not null,
    created_at      timestamptz not null default now()
);
-- Indice da memoria: mensagens de uma conversa em ordem cronologica (ultimas N).
create index if not exists calendar_chat_msg_idx
    on calendar.chat_messages (conversation_id, created_at);
