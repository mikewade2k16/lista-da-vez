-- Modulo Calendario — perfil estrategico do cliente (Fase 4)
-- Plano: docs/CALENDARIO_PLAN.md (secao 3.7); contrato C3 em docs/CALENDARIO_SPECS.md.
--
-- Tabela DEDICADA 1:1 por (account, cliente): a account e a dona do calendario
-- (Principal); o client_id referencia a account do cliente (tenant) a que o perfil
-- pertence. Insumo estrategico da IA (segmento, posicionamento, historia, objetivos,
-- tom de voz, etc.). Campos livres do brief no jsonb `extra` (audience/offer/pillars/
-- cadence/restrictions/performance/assets). Perfil e OPCIONAL por design.
-- Idempotente, schema qualificado, sem `-- +goose Down` (o migrator roda o arquivo inteiro).

create schema if not exists calendar;

create table if not exists calendar.client_profiles (
    account_id uuid not null references core.accounts(id) on delete cascade,
    client_id uuid not null references core.accounts(id) on delete cascade,
    segment text not null default '',
    positioning text not null default '',
    description text not null default '',
    history text not null default '',
    site_url text not null default '',
    instagram text not null default '',
    address text not null default '',
    objectives text not null default '',
    brand_voice text not null default '',
    extra jsonb not null default '{}'::jsonb,
    updated_by text not null default '',
    created_at timestamptz not null default now(),
    updated_at timestamptz not null default now(),
    primary key (account_id, client_id)
);
create index if not exists calendar_client_profiles_account_idx on calendar.client_profiles (account_id);
