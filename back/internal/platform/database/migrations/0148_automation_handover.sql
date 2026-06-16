-- 0148 — M4: trava de handover humano (idempotente, sem marcadores goose).
-- Por que: o migrator (internal/platform/database/migrator.go) executa o ARQUIVO
-- INTEIRO como um unico script; marcadores "-- +goose Up/Down" sao so comentarios e
-- um bloco Down rodaria no mesmo boot. Esta migration so adiciona a coluna.
--
-- paused_until: quando preenchido e > now(), o bot deve ficar em silencio (atendente
-- humano assumiu). O runtime (n8n) le esse estado pelo GET /v1/runtime/automation/memory
-- (campos paused/pausedUntil) e nao responde enquanto a pausa nao expira.
ALTER TABLE automation.contacts
    ADD COLUMN IF NOT EXISTS paused_until timestamptz;
