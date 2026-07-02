-- Modulo Calendario — config por conta (schema `calendar`)
-- Plano: docs/CALENDARIO_PLAN.md secoes 3.3/3.5.
--
-- Guarda, por account, a config do calendario em jsonb: quais usuarios aparecem
-- como responsaveis e quais conjuntos de feriados/datas estao ligados. Shape do
-- jsonb e do front/back (ver model.go CalendarConfig).
-- Idempotente, schema qualificado, sem `-- +goose Down`.

create table if not exists calendar.config (
    account_id uuid primary key references core.accounts(id) on delete cascade,
    config jsonb not null default '{}'::jsonb,
    updated_at timestamptz not null default now()
);
