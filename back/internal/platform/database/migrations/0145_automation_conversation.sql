-- 0145 — persistencia de conversa (A7 / P7)
-- messages: historico de mensagens (in/out) por contato, gravado pelo n8n via runtime.
-- lead_state: estado do lead (status do funil, ultima interacao, contagem de follow-up).
-- Ambas scoped por account_id + automation_id (padrao das migrations 0140-0144).
-- Idempotente: CREATE TABLE/INDEX IF NOT EXISTS.

-- 1. Mensagens trocadas com o contato. -----------------------------------------
CREATE TABLE IF NOT EXISTS automation.messages (
    id            uuid        NOT NULL DEFAULT gen_random_uuid(),
    automation_id uuid        NOT NULL REFERENCES automation.automations(id) ON DELETE CASCADE,
    account_id    uuid        NOT NULL REFERENCES core.accounts(id) ON DELETE CASCADE,
    contact_id    text        NOT NULL,                  -- chatId do WhatsApp (igual contacts.chat_id)
    direction     text        NOT NULL,                  -- in | out
    type          text        NOT NULL DEFAULT 'text',   -- text | audio | image
    content       text        NOT NULL DEFAULT '',
    media_url     text        NOT NULL DEFAULT '',
    segment       text        NOT NULL DEFAULT '',       -- classificacao da etapa (ex.: triagem)
    created_at    timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (id)
);

CREATE INDEX IF NOT EXISTS automation_messages_account_idx
    ON automation.messages (account_id);
CREATE INDEX IF NOT EXISTS automation_messages_contact_idx
    ON automation.messages (automation_id, contact_id, created_at);

-- 2. Estado do lead por contato + automacao. -----------------------------------
CREATE TABLE IF NOT EXISTS automation.lead_state (
    contact_id       text        NOT NULL,               -- chatId do WhatsApp
    automation_id    uuid        NOT NULL REFERENCES automation.automations(id) ON DELETE CASCADE,
    account_id       uuid        NOT NULL REFERENCES core.accounts(id) ON DELETE CASCADE,
    status           text        NOT NULL DEFAULT 'new', -- new | engaged | qualified | won | lost ...
    last_interaction timestamptz NOT NULL DEFAULT now(),
    follow_up_count  integer     NOT NULL DEFAULT 0,
    updated_at       timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (automation_id, contact_id)
);

CREATE INDEX IF NOT EXISTS automation_lead_state_account_idx
    ON automation.lead_state (account_id);
