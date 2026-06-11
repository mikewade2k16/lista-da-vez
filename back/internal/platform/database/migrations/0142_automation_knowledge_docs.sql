-- 0142 — knowledge_documents por automacao (M3+)
-- Documentos de conhecimento editaveis no painel; concatenados no systemMessage
-- apos as instrucoes da persona (antes dos guardrails). Idempotente: IF NOT EXISTS.

CREATE TABLE IF NOT EXISTS automation.knowledge_documents (
    id            uuid        NOT NULL DEFAULT gen_random_uuid(),
    automation_id uuid        NOT NULL REFERENCES automation.automations(id) ON DELETE CASCADE,
    account_id    uuid        NOT NULL REFERENCES core.accounts(id) ON DELETE CASCADE,
    title         text        NOT NULL DEFAULT '',
    body          text        NOT NULL DEFAULT '',
    sort_order    int         NOT NULL DEFAULT 0,
    enabled       bool        NOT NULL DEFAULT true,
    created_at    timestamptz NOT NULL DEFAULT now(),
    updated_at    timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (id)
);

CREATE INDEX IF NOT EXISTS knowledge_documents_automation_idx
    ON automation.knowledge_documents (automation_id, sort_order);
