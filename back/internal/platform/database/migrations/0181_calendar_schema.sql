-- Modulo Calendario (agenda de conteudo por cliente) — schema `calendar`
-- Plano: docs/CALENDARIO_PLAN.md
--
-- Painel Omni faz o CRUD dos eventos (por account) e das notas por mes. `client_id`
-- referencia a account do cliente (tenant) a que o evento se destina; pode ser null
-- (evento interno/sem cliente).
-- Idempotente, schema qualificado, sem `-- +goose Down` (o migrator roda o arquivo inteiro).

create schema if not exists calendar;

create table if not exists calendar.events (
    id uuid primary key default gen_random_uuid(),
    account_id uuid not null references core.accounts(id) on delete cascade,
    client_id uuid references core.accounts(id) on delete set null,
    event_date date not null,
    event_time text not null default '',       -- 'HH:mm' ('' = dia inteiro)
    type text not null default 'post',         -- post|story|reels|reuniao|gravacao|evento
    title text not null,
    status text not null default 'planejado',
    priority text not null default 'media',    -- alta|media|baixa
    responsible_id text not null default '',
    involved_ids jsonb not null default '[]'::jsonb,
    media jsonb not null default '[]'::jsonb,
    description text not null default '',
    created_at timestamptz not null default now(),
    updated_at timestamptz not null default now()
);
create index if not exists calendar_events_account_date_idx on calendar.events (account_id, event_date);
create index if not exists calendar_events_client_idx on calendar.events (client_id);

-- Notas por mes (uma linha por account + 'YYYY-MM'). Rich text (HTML) do editor.
create table if not exists calendar.notes (
    account_id uuid not null references core.accounts(id) on delete cascade,
    month_key text not null,                   -- 'YYYY-MM'
    content text not null default '',
    updated_by text not null default '',
    updated_at timestamptz not null default now(),
    primary key (account_id, month_key)
);
