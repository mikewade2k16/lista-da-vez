-- =============================================================================
-- ESBOCO DE SCHEMA — modulo automation (NAO E MIGRATION; so para revisao)
-- =============================================================================
-- Design da fase A1 do PLANO_INTEGRACAO_OMNI.md. NAO aplicar: a migration real,
-- numerada (back/internal/platform/database/migrations/####_automation_schema.sql),
-- so e escrita quando a branch refactor/multi-tenant-complete fechar.
-- Convencoes seguidas: schema-per-modulo, account_id NOT NULL com FK core.accounts,
-- IF NOT EXISTS (idempotente), indices por account_id, sem SQL dinamico.
-- IDs uuid (gen_random_uuid) como em core.*/site.*; no Go sao tratados como string.
-- =============================================================================

create schema if not exists automation;

-- 1. Config global do bot por account (1 linha por account) -------------------
create table if not exists automation.settings (
  account_id              uuid primary key references core.accounts(id) on delete cascade,
  enabled                 boolean      not null default false,
  active_persona_id       uuid,            -- FK logica para automation.personas (set via service)
  models                  jsonb        not null default '{}'::jsonb,  -- {chat,vision,audio,classifier}
  temp_context            text,
  temp_context_expires_at timestamptz,
  debounce_seconds        integer      not null default 7,
  created_at              timestamptz  not null default now(),
  updated_at              timestamptz  not null default now()
);

-- 2. Personas / prompts (Tony, Perola, ...) -----------------------------------
create table if not exists automation.personas (
  id            uuid primary key default gen_random_uuid(),
  account_id    uuid        not null references core.accounts(id) on delete cascade,
  name          text        not null,
  slug          text        not null,
  system_prompt text        not null,
  is_active     boolean     not null default false,
  created_at    timestamptz not null default now(),
  updated_at    timestamptz not null default now(),
  unique (account_id, slug)
);
create index if not exists automation_personas_account_idx on automation.personas(account_id);
-- so 1 persona ativa por account:
create unique index if not exists automation_personas_one_active_idx
  on automation.personas(account_id) where is_active;

-- 3. Guardrails (regras anexadas a QUALQUER persona) --------------------------
-- account_id NULL = guardrail global (default do produto); por account = override.
create table if not exists automation.guardrails (
  id         uuid primary key default gen_random_uuid(),
  account_id uuid references core.accounts(id) on delete cascade,
  body       text not null,
  updated_at timestamptz not null default now()
);
create unique index if not exists automation_guardrails_global_idx
  on automation.guardrails((account_id is null)) where account_id is null;
create unique index if not exists automation_guardrails_account_idx
  on automation.guardrails(account_id) where account_id is not null;

-- 4. Catalogo de modelos (global; regras do MODELOS.md) -----------------------
create table if not exists automation.model_catalog (
  id                    text    not null,             -- ex.: 'gpt-4o'
  kind                  text    not null,             -- chat | vision | audio | classifier
  label                 text    not null,
  requires_responses_api boolean not null default false,
  accepts_temperature   boolean not null default true,
  vision_ok             boolean not null default false,
  enabled               boolean not null default true,
  sort_order            integer not null default 100,
  primary key (id, kind)                              -- mesmo modelo serve a kinds diferentes
);

-- 5. Sessao WAHA -> account ---------------------------------------------------
create table if not exists automation.waha_sessions (
  account_id      uuid        not null references core.accounts(id) on delete cascade,
  session_name    text        not null,               -- ex.: 'default'
  status          text        not null default 'STOPPED',
  connected_phone text,
  updated_at      timestamptz not null default now(),
  primary key (account_id, session_name),
  unique (session_name)                               -- 1 sessao WAHA mapeia 1 account (V1)
);

-- 6. Token de servico do n8n (resolve a account no runtime) -------------------
create table if not exists automation.service_tokens (
  id           uuid primary key default gen_random_uuid(),
  account_id   uuid        not null references core.accounts(id) on delete cascade,
  token_hash   text        not null,                  -- nunca guardar o token em claro
  name         text        not null default 'n8n',
  last_used_at timestamptz,
  revoked_at   timestamptz,
  created_at   timestamptz not null default now()
);
create index if not exists automation_service_tokens_account_idx on automation.service_tokens(account_id);
create unique index if not exists automation_service_tokens_hash_idx on automation.service_tokens(token_hash);

-- 7. Mini-CRM: contatos -------------------------------------------------------
create table if not exists automation.contacts (
  id         uuid primary key default gen_random_uuid(),
  account_id uuid        not null references core.accounts(id) on delete cascade,
  channel    text        not null default 'whatsapp',
  remote_id  text        not null,                    -- chatId da WAHA (ex.: <num>@c.us / @lid)
  push_name  text,
  phone      text,
  first_seen timestamptz not null default now(),
  last_seen  timestamptz not null default now(),
  unique (account_id, channel, remote_id)
);
create index if not exists automation_contacts_account_idx on automation.contacts(account_id);

-- 8. Mensagens (historico) ----------------------------------------------------
create table if not exists automation.messages (
  id         uuid primary key default gen_random_uuid(),
  account_id uuid        not null references core.accounts(id) on delete cascade,
  contact_id uuid        not null references automation.contacts(id) on delete cascade,
  direction  text        not null,                    -- in | out
  type       text        not null default 'text',     -- text | audio | image
  content    text,
  media_url  text,
  segment    text,                                    -- segmento de contexto (classificador)
  created_at timestamptz not null default now()
);
create index if not exists automation_messages_account_created_idx on automation.messages(account_id, created_at);
create index if not exists automation_messages_contact_idx on automation.messages(contact_id);

-- 9. Estado do lead -----------------------------------------------------------
create table if not exists automation.lead_state (
  contact_id       uuid primary key references automation.contacts(id) on delete cascade,
  account_id       uuid        not null references core.accounts(id) on delete cascade,
  status           text        not null default 'novo',
  last_interaction timestamptz,
  follow_up_count  integer     not null default 0,
  updated_at       timestamptz not null default now()
);
create index if not exists automation_lead_state_account_idx on automation.lead_state(account_id);

-- 10. Memoria longa por contato (substitui o staticData "lite") ---------------
create table if not exists automation.long_memory (
  contact_id uuid primary key references automation.contacts(id) on delete cascade,
  account_id uuid        not null references core.accounts(id) on delete cascade,
  summary    text        not null default '',
  updated_at timestamptz not null default now()
);
create index if not exists automation_long_memory_account_idx on automation.long_memory(account_id);

-- 11. Follow-ups (motor proativo / Etapa 3) -----------------------------------
create table if not exists automation.follow_ups (
  id         uuid primary key default gen_random_uuid(),
  account_id uuid        not null references core.accounts(id) on delete cascade,
  contact_id uuid        not null references automation.contacts(id) on delete cascade,
  kind       text        not null,                    -- sem_resposta | pos_venda | nurture
  due_at     timestamptz not null,
  status     text        not null default 'pendente', -- pendente | enviado | cancelado
  attempts   integer     not null default 0,
  created_at timestamptz not null default now()
);
create index if not exists automation_follow_ups_due_idx on automation.follow_ups(account_id, status, due_at);

-- 12. Compras / pos-venda -----------------------------------------------------
create table if not exists automation.purchases (
  id         uuid primary key default gen_random_uuid(),
  account_id uuid        not null references core.accounts(id) on delete cascade,
  contact_id uuid        not null references automation.contacts(id) on delete cascade,
  amount     numeric(12,2),
  detail     text,
  created_at timestamptz not null default now()
);
create index if not exists automation_purchases_account_idx on automation.purchases(account_id);

-- =============================================================================
-- SEED do catalogo de modelos (global) — baseline das regras do MODELOS.md.
-- requires_responses_api/accepts_temperature/vision_ok conforme aprendido na pratica.
-- (gpt-5.3-chat-latest: confirmar flags na fase A3 antes de fechar.)
-- =============================================================================
insert into automation.model_catalog (id, kind, label, requires_responses_api, accepts_temperature, vision_ok, sort_order) values
  ('gpt-4o-mini',        'chat',   'GPT-4o mini',        false, true,  true,  10),
  ('gpt-4o',             'chat',   'GPT-4o',             false, true,  true,  20),
  ('gpt-4.1',            'chat',   'GPT-4.1',            false, true,  true,  30),
  ('gpt-5.3-chat-latest','chat',   'GPT-5.3 chat',       true,  false, false, 40),
  ('gpt-5.5-pro',        'chat',   'GPT-5.5 pro (reasoning)', true, false, false, 50),
  ('gpt-4o',             'vision', 'GPT-4o (visao)',     false, false, true,  20),
  ('gpt-4o-mini',        'vision', 'GPT-4o mini (visao)',false, false, true,  10),
  ('whisper-1',          'audio',  'Whisper',            false, false, false, 10),
  ('gpt-4o-mini',        'classifier', 'GPT-4o mini (classificador/resumidor)', false, true, false, 10)
on conflict (id, kind) do nothing;
