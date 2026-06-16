-- 0147 — correcao: recriar automation.contacts (idempotente, sem goose).
-- Por que: a 0143 usa marcadores "-- +goose Up / -- +goose Down", mas o migrator
-- (internal/platform/database/migrator.go) executa o ARQUIVO INTEIRO como um unico
-- script (tx.Exec(migration.SQL)). Os marcadores sao so comentarios "--"; o
-- "DROP TABLE automation.contacts" do bloco Down rodava logo apos o CREATE,
-- dropando a tabela no mesmo boot. Esta migration recria a tabela sem DROP.
-- (A 0143 tem o mesmo padrao — esta migration corrige o efeito; a 0144 antiga do
-- settings tambem tinha, ja removida. A 0135 foi auditada: OK, DROP intencional sem goose.)
CREATE TABLE IF NOT EXISTS automation.contacts (
    id            uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    automation_id uuid        NOT NULL REFERENCES automation.automations(id) ON DELETE CASCADE,
    account_id    uuid        NOT NULL REFERENCES core.accounts(id) ON DELETE CASCADE,
    chat_id       text        NOT NULL,
    seg           integer     NOT NULL DEFAULT 0,
    last_msg      text        NOT NULL DEFAULT '',
    last_msg_ts   bigint      NOT NULL DEFAULT 0,
    long_memory   text        NOT NULL DEFAULT '',
    updated_at    timestamptz NOT NULL DEFAULT now(),
    UNIQUE (automation_id, chat_id)
);
