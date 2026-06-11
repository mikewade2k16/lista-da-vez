-- +goose Up
CREATE TABLE IF NOT EXISTS automation.contacts (
    id           uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    automation_id uuid       NOT NULL REFERENCES automation.automations(id) ON DELETE CASCADE,
    account_id   uuid        NOT NULL REFERENCES core.accounts(id) ON DELETE CASCADE,
    chat_id      text        NOT NULL,
    seg          integer     NOT NULL DEFAULT 0,
    last_msg     text        NOT NULL DEFAULT '',
    last_msg_ts  bigint      NOT NULL DEFAULT 0,
    long_memory  text        NOT NULL DEFAULT '',
    updated_at   timestamptz NOT NULL DEFAULT now(),
    UNIQUE (automation_id, chat_id)
);

-- +goose Down
DROP TABLE IF EXISTS automation.contacts;
