-- Omnichannel MVP-02: auditoria duravel da proposta de encerramento da IA.
-- A linha registra a avaliacao do Go; nunca concede ao n8n/modelo autoridade
-- para fechar a conversa. Nao persiste prompt nem conteudo das mensagens.

create schema if not exists messaging;

create table if not exists messaging.ai_close_evaluations (
    id uuid primary key default gen_random_uuid(),
    account_id uuid not null references core.accounts(id) on delete cascade,
    conversation_id uuid not null references messaging.conversations(id) on delete cascade,
    automation_profile_id uuid references messaging.automation_profiles(id) on delete set null,
    ai_run_id uuid references messaging.ai_runs(id) on delete set null,
    idempotency_key text not null,

    requested boolean not null,
    accepted boolean not null,
    reason_codes text[] not null default '{}'::text[],
    confidence numeric(5,4) not null default 0
        check (confidence >= 0 and confidence <= 1),
    minimum_confidence numeric(5,4) not null default 0
        check (minimum_confidence >= 0 and minimum_confidence <= 1),
    required_fields text[] not null default '{}'::text[],
    missing_fields text[] not null default '{}'::text[],
    human_requested boolean not null default false,
    sensitive_topic boolean not null default false,
    source_state text not null,
    captured_generation bigint not null check (captured_generation >= 0),
    current_generation bigint not null check (current_generation >= 0),
    policy_snapshot jsonb not null default '{}'::jsonb,
    created_at timestamptz not null default now(),

    unique (account_id, idempotency_key)
);

create index if not exists messaging_ai_close_evaluations_conversation_idx
    on messaging.ai_close_evaluations (account_id, conversation_id, created_at desc);

create index if not exists messaging_ai_close_evaluations_result_idx
    on messaging.ai_close_evaluations (account_id, accepted, created_at desc);
