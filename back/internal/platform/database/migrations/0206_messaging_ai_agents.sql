-- Modulo Omnichannel — F9: triagem por IA no Go (agentes/versoes/runs/campos).
-- Plano canonico: docs/omnichannel/PLANO_ATENDIMENTO.md (§5.2, §6, §7.1, §9.2 F9, §10).
-- Spec: docs/omnichannel/specs/OMNI-F9.md (Contrato C9.1).
--
-- Numero: 0206 pinado pela orquestracao (0200/0201/0205 ocupadas; a ultima no disco na hora
-- de escrever era 0205_messaging_service_domain.sql, a F8). Gap de numeracao e inofensivo.
--
-- 4 tabelas novas + a FK routing_decisions.ai_run_id -> ai_runs (a F8 deixou a coluna SEM FK
-- de proposito porque ai_runs nasce aqui). "add constraint if not exists" NAO existe no
-- Postgres: FK idempotente via bloco do $$ ... pg_constraint $$ (modelo: 0205).
--
-- Multi-tenant dia 1: account_id uuid not null references core.accounts(id) em todas.
-- account_id vem SEMPRE do Principal, nunca do body; fora de escopo -> 404, nunca 403.
--
-- Chave do provider LLM: gravada CIFRADA (secretbox/AES-GCM) na coluna provider_key_ciphertext
-- de ai_agents; provider_key_last4 e o unico pedaco que sai ao front ({set,last4}). NUNCA em
-- coluna crua, nunca em log. (Modelo do calendar/secrets.go, agora com cifragem em repouso.)
--
-- Idempotente, schema qualificado, SEM `-- +goose Down` (o migrator roda o arquivo inteiro;
-- um Down aqui se auto-destruiria no mesmo boot). Modelo de estilo: 0205.

create schema if not exists messaging;

-- ============================================================================
-- Agentes de IA — UNIQUE(account_id, slug). active_version_id repontado no publish/rollback.
-- ============================================================================
create table if not exists messaging.ai_agents (
    id uuid not null default gen_random_uuid(),
    account_id uuid not null references core.accounts(id) on delete cascade,
    slug text not null,
    name text not null default '',
    enabled boolean not null default false,
    active_version_id uuid,
    -- Chave do provider LLM cifrada em repouso (secretbox "v1:..."); vazio = sem chave.
    -- last4 e o unico fragmento exposto ao painel; a chave crua so existe server-side.
    provider_key_ciphertext text not null default '',
    provider_key_last4 text not null default '',
    created_by text not null default '',
    created_at timestamptz not null default now(),
    updated_at timestamptz not null default now(),
    primary key (id)
);
create unique index if not exists messaging_ai_agents_account_slug_uidx
    on messaging.ai_agents (account_id, slug);

-- ============================================================================
-- Versoes do agente — PUBLICADA e IMUTAVEL: editar = criar version nova. Rollback = repontar
-- ai_agents.active_version_id para uma version anterior (nunca reescrever a linha publicada).
-- ============================================================================
create table if not exists messaging.ai_agent_versions (
    id uuid not null default gen_random_uuid(),
    account_id uuid not null references core.accounts(id) on delete cascade,
    agent_id uuid not null references messaging.ai_agents(id) on delete cascade,
    version integer not null,
    status text not null default 'draft', -- draft | published | archived
    provider text not null default '',     -- do painel; NUNCA supor
    model text not null default '',
    temperature numeric(3,2) not null default 0.20,
    layers jsonb not null default '{}'::jsonb,         -- as camadas editaveis do prompt
    output_schema jsonb not null default '{}'::jsonb,  -- JSON Schema da saida
    schema_version text not null default 'v1',
    published_at timestamptz,
    published_by text not null default '',
    created_at timestamptz not null default now(),
    primary key (id)
);
create unique index if not exists messaging_ai_agent_versions_agent_version_uidx
    on messaging.ai_agent_versions (agent_id, version);
create index if not exists messaging_ai_agent_versions_account_idx
    on messaging.ai_agent_versions (account_id, agent_id);

-- ============================================================================
-- Campos que a IA extrai — UNIQUE(account_id, agent_id, key).
-- ============================================================================
create table if not exists messaging.collect_field_defs (
    id uuid not null default gen_random_uuid(),
    account_id uuid not null references core.accounts(id) on delete cascade,
    agent_id uuid not null references messaging.ai_agents(id) on delete cascade,
    key text not null,
    label text not null default '',
    field_type text not null default 'text', -- text|number|email|phone|date|enum
    enum_options jsonb not null default '[]'::jsonb,
    required boolean not null default false,
    sort_order integer not null default 0,
    primary key (id)
);
create unique index if not exists messaging_collect_field_defs_agent_key_uidx
    on messaging.collect_field_defs (account_id, agent_id, key);

-- ============================================================================
-- ai_runs — uma linha por TENTATIVA de triagem, inclusive as que NAO chamaram o modelo
-- (blocked/limit_exceeded/provider_error): a trilha precisa explicar o silencio da IA.
-- input e MASCARADO antes de persistir (§10). usage/custo = base do custo por conta (F13).
--
-- conversation_id e NULLABLE (deviacao consciente do C9.1, que pedia not null): o `simulate`
-- grava um run REAL com custo real mas "NUNCA cria conversa" (C9.7) — sem conversa, sem id.
-- ============================================================================
create table if not exists messaging.ai_runs (
    id uuid not null default gen_random_uuid(),
    account_id uuid not null references core.accounts(id) on delete cascade,
    conversation_id uuid,
    agent_id uuid,
    agent_version_id uuid,
    message_id uuid,
    -- ok|schema_invalid|provider_error|blocked|limit_exceeded
    status text not null default 'ok',
    provider text not null default '',
    model text not null default '',
    schema_version text not null default '',
    input jsonb not null default '{}'::jsonb,  -- MASCARADO (§10)
    output jsonb not null default '{}'::jsonb,
    prompt_tokens integer not null default 0,
    completion_tokens integer not null default 0,
    total_tokens integer not null default 0,
    cost_usd numeric(12,6) not null default 0,
    latency_ms integer not null default 0,
    error text not null default '',
    created_at timestamptz not null default now(),
    primary key (id)
);
create index if not exists messaging_ai_runs_account_created_idx
    on messaging.ai_runs (account_id, created_at desc);
create index if not exists messaging_ai_runs_conversation_idx
    on messaging.ai_runs (conversation_id, created_at desc);
create index if not exists messaging_ai_runs_agent_idx
    on messaging.ai_runs (account_id, agent_id, created_at desc);

-- ============================================================================
-- FK routing_decisions.ai_run_id -> ai_runs (a coluna nasceu na F8 sem FK). on delete set
-- null: apagar um run nao derruba a decisao de roteamento (a decisao e a verdade da fila).
-- ============================================================================
do $$
begin
    if not exists (
        select 1 from pg_constraint where conname = 'messaging_routing_decisions_ai_run_fk'
    ) then
        alter table messaging.routing_decisions
            add constraint messaging_routing_decisions_ai_run_fk
            foreign key (ai_run_id) references messaging.ai_runs(id) on delete set null;
    end if;
end $$;
