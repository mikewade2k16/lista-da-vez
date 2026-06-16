-- 0146 — settings de gamificacao (badges) por account (C6).
-- badge_rules: lista de regras de badge (jsonb), editavel no painel de settings.
-- tenant_id == id da account (FK core.accounts), seguindo a convencao do modulo
-- settings (public.*, tenant_id). Idempotente: CREATE TABLE IF NOT EXISTS.
-- SEM marcadores goose: o migrator (migrator.go) roda o arquivo INTEIRO como um
-- script, entao um bloco "-- +goose Down ... DROP TABLE" dropa o que acabou de criar.
CREATE TABLE IF NOT EXISTS public.tenant_gamification_settings (
    tenant_id   uuid        PRIMARY KEY REFERENCES core.accounts(id) ON DELETE CASCADE,
    badge_rules jsonb       NOT NULL DEFAULT '[]'::jsonb,
    updated_at  timestamptz NOT NULL DEFAULT now()
);
