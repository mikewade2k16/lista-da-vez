-- Modulo Calendario — secrets de IA por conta (Wave 3, contrato SEC)
-- Plano: docs/CALENDARIO_SPECS3.md (SPEC-B1).
--
-- Guarda, por account e provider, a API key crua da IA do calendario. A key SO
-- existe server-side (resolver/dispatch); o front recebe apenas o status mascarado
-- {set,last4} via GET /v1/calendar/ai-keys — NUNCA a key crua. A chave GLOBAL da
-- plataforma vive em core.platform_settings (key 'calendar_ai_secrets'), fora desta
-- tabela. account_id + provider = PK composta: conta A nunca le/escreve a de B.
-- Idempotente, schema qualificado, sem `-- +goose Down` (o migrator roda o arquivo inteiro).

create schema if not exists calendar;

create table if not exists calendar.ai_secrets (
    account_id uuid not null references core.accounts(id) on delete cascade,
    provider text not null,
    api_key text not null default '',
    updated_by text not null default '',
    updated_at timestamptz not null default now(),
    primary key (account_id, provider)
);
