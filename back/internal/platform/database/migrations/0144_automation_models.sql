-- 0144 — catalogo de modelos + selecao por automacao (A6 / P6)
-- model_catalog: global, provider-agnostico (openai|anthropic|...), com as flags do
--   MODELOS.md (requires_responses_api, accepts_temperature, vision_ok).
-- automation_models: o modelo escolhido por automacao + funcao (chat|vision|audio|...),
--   scoped por account_id + automation_id (padrao das migrations 0140-0143).
-- Idempotente: CREATE TABLE/INDEX IF NOT EXISTS; seeds com ON CONFLICT DO NOTHING.

-- 1. Catalogo global de modelos (sem account_id; e a lista de opcoes disponiveis). --
CREATE TABLE IF NOT EXISTS automation.model_catalog (
    id                     text    NOT NULL,                  -- ex.: 'gpt-4o'
    provider               text    NOT NULL DEFAULT 'openai', -- openai | anthropic | ...
    kind                   text    NOT NULL,                  -- chat | vision | audio | classifier | embedding
    label                  text    NOT NULL,
    requires_responses_api boolean NOT NULL DEFAULT false,    -- gpt-5*/o-series exigem Responses API
    accepts_temperature    boolean NOT NULL DEFAULT true,     -- modelos de raciocinio nao aceitam
    vision_ok              boolean NOT NULL DEFAULT false,    -- pode ser usado no no de imagem
    enabled                boolean NOT NULL DEFAULT true,
    sort_order             integer NOT NULL DEFAULT 100,
    created_at             timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (provider, id, kind)
);

-- 2. Modelo escolhido por automacao + funcao ("varios nos com IAs diferentes"). ----
CREATE TABLE IF NOT EXISTS automation.automation_models (
    automation_id uuid  NOT NULL REFERENCES automation.automations(id) ON DELETE CASCADE,
    account_id    uuid  NOT NULL REFERENCES core.accounts(id) ON DELETE CASCADE,
    role          text  NOT NULL,                             -- chat|vision|audio|classifier
    provider      text  NOT NULL DEFAULT 'openai',
    model_id      text  NOT NULL,
    params        jsonb NOT NULL DEFAULT '{}'::jsonb,         -- temperature, max_tokens, etc.
    updated_at    timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (automation_id, role)
);

CREATE INDEX IF NOT EXISTS automation_models_account_idx
    ON automation.automation_models (account_id);
CREATE INDEX IF NOT EXISTS automation_models_automation_idx
    ON automation.automation_models (automation_id);

-- 3. Seed do catalogo. V1: OpenAI + Anthropic (provider-agnostico). Flags conforme
--    MODELOS.md. Anthropic (Claude) nao usa "Responses API" (conceito OpenAI), aceita
--    temperature e tem visao nativa nos modelos de chat.
INSERT INTO automation.model_catalog
    (id, provider, kind, label, requires_responses_api, accepts_temperature, vision_ok, sort_order) VALUES
    -- OpenAI — chat
    ('gpt-4o-mini',               'openai',    'chat',       'GPT-4o mini',                   false, true,  true,  10),
    ('gpt-4o',                    'openai',    'chat',       'GPT-4o',                        false, true,  true,  20),
    ('gpt-4.1',                   'openai',    'chat',       'GPT-4.1',                       false, true,  true,  30),
    ('gpt-5.3-chat-latest',       'openai',    'chat',       'GPT-5.3 chat',                  true,  false, false, 40),
    ('gpt-5.5-pro',               'openai',    'chat',       'GPT-5.5 pro (raciocinio)',      true,  false, false, 50),
    -- OpenAI — visao
    ('gpt-4o-mini',               'openai',    'vision',     'GPT-4o mini (visao)',           false, false, true,  10),
    ('gpt-4o',                    'openai',    'vision',     'GPT-4o (visao)',                false, false, true,  20),
    -- OpenAI — audio (modelo fixo Whisper)
    ('whisper-1',                 'openai',    'audio',      'Whisper',                       false, false, false, 10),
    -- OpenAI — classificador/resumo
    ('gpt-4o-mini',               'openai',    'classifier', 'GPT-4o mini (classif./resumo)', false, true,  false, 10),
    -- Anthropic (Claude) — chat (visao nativa; sem Responses API)
    ('claude-haiku-4-5-20251001', 'anthropic', 'chat',       'Claude Haiku 4.5',              false, true,  true,  60),
    ('claude-sonnet-4-6',         'anthropic', 'chat',       'Claude Sonnet 4.6',             false, true,  true,  70),
    ('claude-opus-4-8',           'anthropic', 'chat',       'Claude Opus 4.8',               false, true,  true,  80),
    -- Anthropic (Claude) — visao
    ('claude-haiku-4-5-20251001', 'anthropic', 'vision',     'Claude Haiku 4.5 (visao)',      false, true,  true,  60),
    ('claude-sonnet-4-6',         'anthropic', 'vision',     'Claude Sonnet 4.6 (visao)',     false, true,  true,  70),
    -- Anthropic (Claude) — classificador/resumo
    ('claude-haiku-4-5-20251001', 'anthropic', 'classifier', 'Claude Haiku 4.5 (classif.)',   false, true,  false, 60)
ON CONFLICT (provider, id, kind) DO NOTHING;
