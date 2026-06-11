```sql
-- =============================================================================
-- ESBOCO DE SCHEMA — modulo automation (NAO E MIGRATION; so para revisao)
-- =============================================================================
-- Design da fase P1 da PLATAFORMA_AUTOMACAO.md (automation-centric, multi-tenant).
-- NAO aplicar: a migration real, numerada
-- (back/internal/platform/database/migrations/####_automation_schema.sql), so e
-- escrita quando a branch refactor/multi-tenant-complete fechar (regra do
-- MULTITENANT_COMPLETION_PLAN: nenhum modulo satelite avanca antes).
--
-- Pivot vs. esboco antigo: a ENTIDADE CENTRAL e `automations` (o robo) — N por
-- account. Tudo (persona, canal, CRM, modelos, knowledge) passa a ter automation_id.
-- Resolucao no runtime: webhook traz `session` -> channels.session_name ->
-- automation_id -> account_id + persona + guardrails + modelos + pipeline + tools.
--
-- Convencoes: schema-per-modulo; account_id NOT NULL com FK core.accounts;
-- IF NOT EXISTS (idempotente); indices por account_id/automation_id; sem SQL
-- dinamico. IDs uuid (gen_random_uuid), tratados como string no Go.
-- Decisoes fechadas (2026-06-08): pgvector p/ RAG; BYOK obrigatorio (chave do
-- cliente criptografada at-rest); so OpenAI na V1 (catalogo provider-aware);
-- channel.provider pluggable (waha|evolution|cloud_api), WAHA Core na V1.
-- =============================================================================

create schema if not exists automation;

-- Extensao para RAG (P8). Precisa estar disponivel no Postgres (imagem com pgvector).
-- A migration real roda isto antes das tabelas de knowledge.
create extension if not exists vector;

-- =============================================================================
-- NUCLEO MULTI-TENANT
-- =============================================================================

-- 1. Automations (o "robo"). ENTIDADE CENTRAL. N por account. ------------------
create table if not exists automation.automations (
  id          uuid primary key default gen_random_uuid(),
  account_id  uuid        not null references core.accounts(id) on delete cascade,
  type        text        not null default 'atendimento',  -- atendimento | super | (futuros)
  name        text        not null,
  slug        text        not null,
  status      text        not null default 'draft',        -- draft | active | paused (active = ligado)
  -- credencial BYOK que esta automacao usa (provider key do cliente); null = sem IA ainda
  provider_credential_id uuid,                              -- FK logica -> provider_credentials
  temp_context            text,                             -- contexto temporario ("em gravacao ate 16h")
  temp_context_expires_at timestamptz,
  created_by  uuid        references core.users(id) on delete set null,
  created_at  timestamptz not null default now(),
  updated_at  timestamptz not null default now(),
  unique (account_id, slug)
);
create index if not exists automation_automations_account_idx on automation.automations(account_id);
create index if not exists automation_automations_status_idx  on automation.automations(account_id, status);

-- 2. Entitlements: quem pode criar automacao, quantas, quais tipos (gating). ---
-- Liberado pelo platform_admin (permissao automation.platform.admin).
create table if not exists automation.entitlements (
  account_id       uuid primary key references core.accounts(id) on delete cascade,
  enabled          boolean     not null default false,
  max_automations  integer     not null default 1,
  allowed_types    text[]      not null default array['atendimento'],
  updated_at       timestamptz not null default now()
);

-- 3. Credenciais de provedor (BYOK). Chave do CLIENTE, criptografada at-rest. --
-- Nunca guardar a chave em claro. key_ciphertext = AES-256-GCM (master key em
-- env AUTOMATION_CRED_ENC_KEY). key_last4 so para exibir no painel.
create table if not exists automation.provider_credentials (
  id             uuid primary key default gen_random_uuid(),
  account_id     uuid        not null references core.accounts(id) on delete cascade,
  provider       text        not null default 'openai',    -- openai | anthropic | ... (V1: openai)
  label          text        not null,
  key_ciphertext bytea       not null,
  key_nonce      bytea       not null,                      -- nonce/IV do AES-GCM
  key_last4      text        not null,
  created_at     timestamptz not null default now(),
  revoked_at     timestamptz
);
create index if not exists automation_provider_credentials_account_idx
  on automation.provider_credentials(account_id);

-- 4. Channels: conexao do canal. provider PLUGGABLE. 1 sessao = 1 numero. ------
-- session_name unico global -> resolve a automacao no webhook.
create table if not exists automation.channels (
  id              uuid primary key default gen_random_uuid(),
  automation_id   uuid        not null references automation.automations(id) on delete cascade,
  account_id      uuid        not null references core.accounts(id) on delete cascade,
  provider        text        not null default 'waha',     -- waha | evolution | cloud_api
  session_name    text        not null,                    -- ex.: 'default' (WAHA) / instance (Evolution)
  status          text        not null default 'STOPPED',
  connected_phone text,
  updated_at      timestamptz not null default now(),
  unique (session_name)                                    -- mapeia 1 sessao -> 1 channel
);
create index if not exists automation_channels_automation_idx on automation.channels(automation_id);
create index if not exists automation_channels_account_idx    on automation.channels(account_id);

-- =============================================================================
-- COMPORTAMENTO (persona / modelos / pipeline / tools)
-- =============================================================================

-- 5. Personas / instrucoes (comportamento estilo GPT) por automacao. ----------
create table if not exists automation.personas (
  id            uuid primary key default gen_random_uuid(),
  automation_id uuid        not null references automation.automations(id) on delete cascade,
  account_id    uuid        not null references core.accounts(id) on delete cascade,
  name          text        not null,
  slug          text        not null,
  system_prompt text        not null,
  is_active     boolean     not null default false,
  created_at    timestamptz not null default now(),
  updated_at    timestamptz not null default now(),
  unique (automation_id, slug)
);
create index if not exists automation_personas_automation_idx on automation.personas(automation_id);
-- so 1 persona ativa por automacao:
create unique index if not exists automation_personas_one_active_idx
  on automation.personas(automation_id) where is_active;

-- 6. Guardrails: global (default do produto) ou override por automacao. --------
-- automation_id NULL = guardrail global; preenchido = override da automacao.
create table if not exists automation.guardrails (
  id            uuid primary key default gen_random_uuid(),
  account_id    uuid references core.accounts(id) on delete cascade,
  automation_id uuid references automation.automations(id) on delete cascade,
  body          text not null,
  updated_at    timestamptz not null default now()
);
create unique index if not exists automation_guardrails_global_idx
  on automation.guardrails((automation_id is null)) where automation_id is null;
create unique index if not exists automation_guardrails_automation_idx
  on automation.guardrails(automation_id) where automation_id is not null;

-- 7. Catalogo de modelos (global; provider-aware; regras do MODELOS.md). -------
create table if not exists automation.model_catalog (
  id                     text    not null,                 -- ex.: 'gpt-4o'
  provider               text    not null default 'openai',
  kind                   text    not null,                 -- chat | vision | audio | classifier | embedding
  label                  text    not null,
  requires_responses_api boolean not null default false,
  accepts_temperature    boolean not null default true,
  vision_ok              boolean not null default false,
  enabled                boolean not null default true,
  sort_order             integer not null default 100,
  primary key (provider, id, kind)
);

-- 8. Modelos por etapa/no, por automacao ("varios nos com IAs diferentes"). ----
create table if not exists automation.automation_models (
  automation_id uuid not null references automation.automations(id) on delete cascade,
  role          text not null,                             -- chat|vision|audio|classifier|summarizer|triage|embedding
  provider      text not null default 'openai',
  model_id      text not null,
  params        jsonb not null default '{}'::jsonb,        -- temperature, max_tokens, etc.
  primary key (automation_id, role)
);

-- 9. Config do pipeline por automacao (toggles/parametros do painel). ----------
-- Ex.: { "debounce_seconds":7, "backlog_cutoff_minutes":5, "triage_enabled":true,
--        "split_balloons":true, "naturalness":true }
create table if not exists automation.pipeline_config (
  automation_id uuid primary key references automation.automations(id) on delete cascade,
  config        jsonb       not null default '{}'::jsonb,
  updated_at    timestamptz not null default now()
);

-- 10. Tools / conexoes (dados Omni ou API externa), por automacao. -------------
create table if not exists automation.tools (
  id            uuid primary key default gen_random_uuid(),
  automation_id uuid        not null references automation.automations(id) on delete cascade,
  account_id    uuid        not null references core.accounts(id) on delete cascade,
  kind          text        not null,                      -- crm | erp | metrics | http
  label         text        not null,
  config        jsonb       not null default '{}'::jsonb,
  enabled       boolean     not null default true,
  created_at    timestamptz not null default now()
);
create index if not exists automation_tools_automation_idx on automation.tools(automation_id);

-- =============================================================================
-- KNOWLEDGE / RAG (pgvector) — P8
-- =============================================================================

-- 11. Knowledge base por automacao (o "Knowledge" do GPT). --------------------
create table if not exists automation.knowledge_bases (
  id            uuid primary key default gen_random_uuid(),
  automation_id uuid        not null references automation.automations(id) on delete cascade,
  account_id    uuid        not null references core.accounts(id) on delete cascade,
  name          text        not null,
  created_at    timestamptz not null default now()
);
create index if not exists automation_kb_automation_idx on automation.knowledge_bases(automation_id);

create table if not exists automation.knowledge_documents (
  id          uuid primary key default gen_random_uuid(),
  kb_id       uuid        not null references automation.knowledge_bases(id) on delete cascade,
  account_id  uuid        not null references core.accounts(id) on delete cascade,
  filename    text        not null,
  status      text        not null default 'pending',      -- pending | indexed | error
  created_at  timestamptz not null default now()
);
create index if not exists automation_kdocs_kb_idx on automation.knowledge_documents(kb_id);

-- Chunks com embedding. dim 1536 = OpenAI text-embedding-3-small (ajustar ao modelo).
create table if not exists automation.knowledge_chunks (
  id           uuid primary key default gen_random_uuid(),
  kb_id        uuid        not null references automation.knowledge_bases(id) on delete cascade,
  account_id   uuid        not null references core.accounts(id) on delete cascade,
  document_id  uuid        references automation.knowledge_documents(id) on delete cascade,
  content      text        not null,
  embedding    vector(1536),
  metadata     jsonb       not null default '{}'::jsonb,
  created_at   timestamptz not null default now()
);
create index if not exists automation_kchunks_kb_idx on automation.knowledge_chunks(kb_id);
-- indice ANN (HNSW, pgvector >= 0.5) por similaridade de cosseno:
create index if not exists automation_kchunks_embedding_idx
  on automation.knowledge_chunks using hnsw (embedding vector_cosine_ops);

-- =============================================================================
-- RUNTIME / AUTH
-- =============================================================================

-- 12. Token de servico do n8n (resolve account/automacao no runtime). ---------
create table if not exists automation.service_tokens (
  id            uuid primary key default gen_random_uuid(),
  account_id    uuid        not null references core.accounts(id) on delete cascade,
  automation_id uuid        references automation.automations(id) on delete cascade,
  token_hash    text        not null,                      -- nunca em claro
  name          text        not null default 'n8n',
  last_used_at  timestamptz,
  revoked_at    timestamptz,
  created_at    timestamptz not null default now()
);
create index if not exists automation_service_tokens_account_idx on automation.service_tokens(account_id);
create unique index if not exists automation_service_tokens_hash_idx on automation.service_tokens(token_hash);

-- =============================================================================
-- MINI-CRM (por automacao) — P7
-- =============================================================================

create table if not exists automation.contacts (
  id            uuid primary key default gen_random_uuid(),
  automation_id uuid        not null references automation.automations(id) on delete cascade,
  account_id    uuid        not null references core.accounts(id) on delete cascade,
  channel       text        not null default 'whatsapp',
  remote_id     text        not null,                      -- chatId (ex.: <num>@c.us / @lid)
  push_name     text,
  phone         text,
  first_seen    timestamptz not null default now(),
  last_seen     timestamptz not null default now(),
  unique (automation_id, channel, remote_id)
);
create index if not exists automation_contacts_account_idx    on automation.contacts(account_id);
create index if not exists automation_contacts_automation_idx on automation.contacts(automation_id);

create table if not exists automation.messages (
  id            uuid primary key default gen_random_uuid(),
  automation_id uuid        not null references automation.automations(id) on delete cascade,
  account_id    uuid        not null references core.accounts(id) on delete cascade,
  contact_id    uuid        not null references automation.contacts(id) on delete cascade,
  direction     text        not null,                      -- in | out
  type          text        not null default 'text',       -- text | audio | image | sticker
  content       text,
  media_url     text,
  segment       text,                                      -- segmento de contexto (classificador)
  created_at    timestamptz not null default now()
);
create index if not exists automation_messages_account_created_idx on automation.messages(account_id, created_at);
create index if not exists automation_messages_contact_idx on automation.messages(contact_id);

create table if not exists automation.lead_state (
  contact_id       uuid primary key references automation.contacts(id) on delete cascade,
  automation_id    uuid        not null references automation.automations(id) on delete cascade,
  account_id       uuid        not null references core.accounts(id) on delete cascade,
  status           text        not null default 'novo',
  last_interaction timestamptz,
  follow_up_count  integer     not null default 0,
  updated_at       timestamptz not null default now()
);
create index if not exists automation_lead_state_automation_idx on automation.lead_state(automation_id);

create table if not exists automation.long_memory (
  contact_id    uuid primary key references automation.contacts(id) on delete cascade,
  automation_id uuid        not null references automation.automations(id) on delete cascade,
  account_id    uuid        not null references core.accounts(id) on delete cascade,
  summary       text        not null default '',
  updated_at    timestamptz not null default now()
);

create table if not exists automation.follow_ups (
  id            uuid primary key default gen_random_uuid(),
  automation_id uuid        not null references automation.automations(id) on delete cascade,
  account_id    uuid        not null references core.accounts(id) on delete cascade,
  contact_id    uuid        not null references automation.contacts(id) on delete cascade,
  kind          text        not null,                      -- sem_resposta | pos_venda | nurture
  due_at        timestamptz not null,
  status        text        not null default 'pendente',   -- pendente | enviado | cancelado
  attempts      integer     not null default 0,
  created_at    timestamptz not null default now()
);
create index if not exists automation_follow_ups_due_idx on automation.follow_ups(account_id, status, due_at);

create table if not exists automation.purchases (
  id            uuid primary key default gen_random_uuid(),
  automation_id uuid        not null references automation.automations(id) on delete cascade,
  account_id    uuid        not null references core.accounts(id) on delete cascade,
  contact_id    uuid        not null references automation.contacts(id) on delete cascade,
  amount        numeric(12,2),
  detail        text,
  created_at    timestamptz not null default now()
);
create index if not exists automation_purchases_automation_idx on automation.purchases(automation_id);

-- =============================================================================
-- METERING (BYOK: cobrado na chave do cliente; a gente so mede) — P13
-- =============================================================================
create table if not exists automation.usage_log (
  id            uuid primary key default gen_random_uuid(),
  account_id    uuid        not null references core.accounts(id) on delete cascade,
  automation_id uuid        not null references automation.automations(id) on delete cascade,
  provider      text        not null,
  model         text        not null,
  role          text,                                      -- chat|vision|audio|classifier|embedding
  tokens_in     integer     not null default 0,
  tokens_out    integer     not null default 0,
  cost_estimate numeric(12,6) not null default 0,
  created_at    timestamptz not null default now()
);
create index if not exists automation_usage_log_idx on automation.usage_log(account_id, automation_id, created_at);

-- =============================================================================
-- SEEDS (globais)
-- =============================================================================

-- Catalogo de modelos — V1 so OpenAI (provider-aware p/ somar outros depois).
-- Flags conforme MODELOS.md; gpt-5.3-chat-latest: confirmar na fase de n8n config.
insert into automation.model_catalog (id, provider, kind, label, requires_responses_api, accepts_temperature, vision_ok, sort_order) values
  ('gpt-4o-mini',            'openai', 'chat',       'GPT-4o mini',                false, true,  true,  10),
  ('gpt-4o',                 'openai', 'chat',       'GPT-4o',                     false, true,  true,  20),
  ('gpt-4.1',                'openai', 'chat',       'GPT-4.1',                    false, true,  true,  30),
  ('gpt-5.3-chat-latest',    'openai', 'chat',       'GPT-5.3 chat',               true,  false, false, 40),
  ('gpt-5.5-pro',            'openai', 'chat',       'GPT-5.5 pro (reasoning)',    true,  false, false, 50),
  ('gpt-4o',                 'openai', 'vision',     'GPT-4o (visao)',             false, false, true,  20),
  ('gpt-4o-mini',            'openai', 'vision',     'GPT-4o mini (visao)',        false, false, true,  10),
  ('whisper-1',              'openai', 'audio',      'Whisper',                    false, false, false, 10),
  ('gpt-4o-mini',            'openai', 'classifier', 'GPT-4o mini (classif./resumo)', false, true, false, 10),
  ('text-embedding-3-small', 'openai', 'embedding',  'Embedding 3 small (RAG)',    false, false, false, 10)
on conflict (provider, id, kind) do nothing;

-- Guardrail global default (anexado a qualquer persona). Corpo real vem do
-- guardrails-resposta.md no momento da migration (PT-BR, texto puro, baloes).
-- insert into automation.guardrails (account_id, automation_id, body) values (null, null, '<guardrails-resposta.md>')
--   on conflict do nothing;  -- (preencher na migration real)
```
