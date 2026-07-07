-- Modulo Calendario — override de IA por cliente (WAVE 3.1, contrato SEC+)
-- Plano: docs/CALENDARIO_SPECS3.md (SPEC-B3).
--
-- A config de IA da conta (calendar.config -> ai) e o default GERAL. Este override
-- por cliente muda SO o COMPORTAMENTO da IA (enabled/provider/model/baseUrl/
-- systemPrompt/temperature) — a API key NUNCA vive aqui (segue no nivel conta/global
-- da WAVE 3.0). Home natural: a tabela 1:1 por (account, cliente) da migration 0185,
-- PK composta (account_id, client_id) — conta A nunca le/escreve o override de B.
-- Idempotente, schema qualificado, sem `-- +goose Down` (o migrator roda o arquivo inteiro).

create schema if not exists calendar;

alter table calendar.client_profiles
    add column if not exists ai_config jsonb not null default '{}'::jsonb;
