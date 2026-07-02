-- Modulo Calendario — anexos avulsos por dia (Fase 3)
-- Plano: docs/CALENDARIO_PLAN.md (secao 3.6)
--
-- Anexos (imagem/video) soltos num dia, sem vinculo com um evento especifico
-- (referencias/moodboard do dia). A midia dos eventos vive em calendar.events.media;
-- esta tabela e a lista avulsa por (account, dia). `media` e um array jsonb de
-- MediaItem { id, url, name, type, contentType, sizeBytes }.
-- Idempotente, schema qualificado, sem `-- +goose Down` (o migrator roda o arquivo inteiro).

create schema if not exists calendar;

create table if not exists calendar.day_media (
    account_id uuid not null references core.accounts(id) on delete cascade,
    event_date date not null,
    media jsonb not null default '[]'::jsonb,
    updated_at timestamptz not null default now(),
    primary key (account_id, event_date)
);
