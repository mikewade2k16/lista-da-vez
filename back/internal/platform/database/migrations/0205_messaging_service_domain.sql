-- Modulo Omnichannel — F8: dominio de atendimento (setores/filas/roteamento).
-- Plano canonico: docs/omnichannel/PLANO_ATENDIMENTO.md (§7.2, §7.3, §12 risco 4).
-- Spec: docs/omnichannel/specs/OMNI-F8.md (Contrato 1).
--
-- Numero: 0205 pinado pela orquestracao (a F4 reivindica 0201-0204). A ultima no disco na
-- hora de escrever era 0200_messaging_schema.sql (a F2). Gap de numeracao e inofensivo.
--
-- As 5 tabelas novas do dominio + as FKs de conversations.department_id/queue_id (que ja
-- nascem na F2 como uuid SEM FK — as tabelas so existem aqui). "add constraint if not
-- exists" NAO existe no Postgres: FK idempotente via bloco do $$ ... pg_constraint $$.
--
-- Multi-tenant dia 1: account_id uuid not null references core.accounts(id) em todas.
-- account_id vem SEMPRE do Principal, nunca do body; fora de escopo -> 404, nunca 403.
--
-- Idempotente, schema qualificado, SEM `-- +goose Down` (o migrator roda o arquivo
-- inteiro; um Down aqui se auto-destruiria). Modelo de estilo: 0200_messaging_schema.sql.

create schema if not exists messaging;

-- ============================================================================
-- Setores (departments) — UNIQUE(account_id, slug)
-- ============================================================================
create table if not exists messaging.departments (
    id         uuid primary key default gen_random_uuid(),
    account_id uuid not null references core.accounts(id) on delete cascade,
    slug       text not null,
    name       text not null,
    is_default boolean not null default false,
    is_active  boolean not null default true,
    created_at timestamptz not null default now(),
    updated_at timestamptz not null default now()
);
create unique index if not exists messaging_departments_slug_uk
    on messaging.departments (account_id, slug);
-- No maximo um setor default por conta (indice parcial).
create unique index if not exists messaging_departments_default_uk
    on messaging.departments (account_id) where is_default;

-- ============================================================================
-- Filas (queues) — UNIQUE(account_id, department_id, slug)
-- ============================================================================
create table if not exists messaging.queues (
    id            uuid primary key default gen_random_uuid(),
    account_id    uuid not null references core.accounts(id) on delete cascade,
    department_id uuid not null references messaging.departments(id) on delete cascade,
    slug          text not null,
    name          text not null,
    is_default    boolean not null default false,
    is_active     boolean not null default true,
    created_at    timestamptz not null default now(),
    updated_at    timestamptz not null default now()
);
create unique index if not exists messaging_queues_slug_uk
    on messaging.queues (account_id, department_id, slug);
-- No maximo uma fila default por setor.
create unique index if not exists messaging_queues_default_uk
    on messaging.queues (account_id, department_id) where is_default;
create index if not exists messaging_queues_account_department_idx
    on messaging.queues (account_id, department_id) where is_active;

-- ============================================================================
-- Membros da fila — O GATE DE DADO (canonico §5.2): quem nao esta aqui nao ve a conversa.
-- ============================================================================
create table if not exists messaging.queue_members (
    id         uuid primary key default gen_random_uuid(),
    account_id uuid not null references core.accounts(id) on delete cascade,
    queue_id   uuid not null references messaging.queues(id) on delete cascade,
    user_id    uuid not null references core.users(id) on delete cascade,
    is_active  boolean not null default true,
    created_at timestamptz not null default now()
);
create unique index if not exists messaging_queue_members_uk
    on messaging.queue_members (queue_id, user_id);
-- Indice do hot path: o filtro de visibilidade roda em TODA leitura de conversa.
create index if not exists messaging_queue_members_user_idx
    on messaging.queue_members (user_id, account_id) where is_active;

-- ============================================================================
-- Regras de roteamento — ordenadas por prioridade (priority asc, id asc)
-- ============================================================================
create table if not exists messaging.routing_rules (
    id              uuid primary key default gen_random_uuid(),
    account_id      uuid not null references core.accounts(id) on delete cascade,
    name            text not null,
    priority        integer not null default 100,
    is_active       boolean not null default true,
    conditions      jsonb not null default '[]'::jsonb,
    target_queue_id uuid not null references messaging.queues(id) on delete cascade,
    created_at      timestamptz not null default now(),
    updated_at      timestamptz not null default now()
);
-- Ordem de avaliacao: priority asc, id asc (desempate estavel => decisao reproduzivel).
create index if not exists messaging_routing_rules_eval_idx
    on messaging.routing_rules (account_id, priority, id) where is_active;

-- ============================================================================
-- Auditoria de roteamento — CADA decisao (entrada, regra que casou, saida)
-- ============================================================================
create table if not exists messaging.routing_decisions (
    id                   uuid primary key default gen_random_uuid(),
    account_id           uuid not null references core.accounts(id) on delete cascade,
    conversation_id      uuid not null references messaging.conversations(id) on delete cascade,
    rule_id              uuid references messaging.routing_rules(id) on delete set null,
    outcome              text not null,
    reason               text not null default '',
    input                jsonb not null default '{}'::jsonb,
    target_department_id uuid references messaging.departments(id) on delete set null,
    target_queue_id      uuid references messaging.queues(id) on delete set null,
    -- ai_run_id fica SEM FK aqui: ai_runs nasce na F9, que adiciona a constraint.
    ai_run_id            uuid,
    decided_at           timestamptz not null default now(),
    constraint messaging_routing_decisions_outcome_ck
        check (outcome in ('matched', 'default_queue', 'unrouted', 'manual_transfer', 'ai_failed'))
);
create index if not exists messaging_routing_decisions_conv_idx
    on messaging.routing_decisions (conversation_id, decided_at desc);

-- ============================================================================
-- FKs de conversations.department_id / queue_id (as colunas nasceram na F2 sem FK)
-- ============================================================================
-- "add constraint if not exists" NAO existe no Postgres: bloco do $$ idempotente que so
-- adiciona a FK se ela ainda nao estiver em pg_constraint. on delete set null: apagar um
-- setor/fila nao deve derrubar a conversa (o DELETE das rotas e soft; isto e backstop).
do $$
begin
    if not exists (
        select 1 from pg_constraint where conname = 'messaging_conversations_department_fk'
    ) then
        alter table messaging.conversations
            add constraint messaging_conversations_department_fk
            foreign key (department_id) references messaging.departments(id) on delete set null;
    end if;

    if not exists (
        select 1 from pg_constraint where conname = 'messaging_conversations_queue_fk'
    ) then
        alter table messaging.conversations
            add constraint messaging_conversations_queue_fk
            foreign key (queue_id) references messaging.queues(id) on delete set null;
    end if;
end $$;

create index if not exists messaging_conversations_queue_idx
    on messaging.conversations (account_id, queue_id) where queue_id is not null;
create index if not exists messaging_conversations_assigned_user_idx
    on messaging.conversations (account_id, assigned_user_id) where assigned_user_id is not null;
