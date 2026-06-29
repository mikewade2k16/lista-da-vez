-- Modulo cardapio (Fase 10 / F1 — ingestao de telemetria): desnormaliza colunas de
-- enriquecimento server-side em cardapio.events (device/UA/UTM/referrer/ip_hash/dwell)
-- e cria a tabela agregada cardapio.sessions (upsert por (restaurant_id, session_id)),
-- alem dos indices de leitura/analytics e do indice parcial de dedupe por event_id.
-- Idempotente, schema-qualificada, sem -- +goose Down (o migrator roda o arquivo
-- inteiro; um Down com DROP se auto-destruiria). Todas as colunas novas tem default,
-- entao o backfill das linhas antigas e implicito. Plano:
-- docs/cardapio/PLANO_CARDAPIO_TRACKING_ANALYTICS.md (secao 8.2).

-- Colunas desnormalizadas de enriquecimento em events (todas com default).
alter table cardapio.events
    add column if not exists occurred_at   timestamptz not null default now(),
    add column if not exists event_id      text        not null default '',
    add column if not exists device_id     text        not null default '',
    add column if not exists page_path     text        not null default '',
    add column if not exists product_slug  text        not null default '',
    add column if not exists device_type   text        not null default '',
    add column if not exists browser       text        not null default '',
    add column if not exists os            text        not null default '',
    add column if not exists referrer_host text        not null default '',
    add column if not exists utm_source    text        not null default '',
    add column if not exists utm_medium    text        not null default '',
    add column if not exists utm_campaign  text        not null default '',
    add column if not exists ip_hash       text        not null default '',
    add column if not exists dwell_ms      integer     not null default 0;

-- Sessoes agregadas (upsert na ingestao). Overview/sources/devices do analytics leem
-- daqui; o detalhe (funil/dwell/top-produtos/cliques) cai em events.
create table if not exists cardapio.sessions (
    id            uuid        primary key default gen_random_uuid(),
    account_id    uuid        not null references core.accounts(id) on delete cascade,
    restaurant_id uuid        not null references cardapio.restaurants(id) on delete cascade,
    session_id    text        not null,
    device_id     text        not null default '',
    first_seen_at timestamptz not null default now(),
    last_seen_at  timestamptz not null default now(),
    duration_ms   bigint      not null default 0,
    pageviews     int         not null default 0,
    events        int         not null default 0,
    utm_source    text        not null default '',
    utm_medium    text        not null default '',
    utm_campaign  text        not null default '',
    referrer_host text        not null default '',
    device_type   text        not null default '',
    landing_path  text        not null default '',
    had_order     boolean     not null default false,
    created_at    timestamptz not null default now(),
    updated_at    timestamptz not null default now()
);

create unique index if not exists cardapio_sessions_restaurant_session_uidx
    on cardapio.sessions (restaurant_id, session_id);
create index if not exists cardapio_sessions_account_idx
    on cardapio.sessions (account_id);
create index if not exists cardapio_sessions_restaurant_last_seen_idx
    on cardapio.sessions (restaurant_id, last_seen_at);

-- Indices de leitura para o analytics (Fase 2) + dedupe de eventos.
create index if not exists cardapio_events_restaurant_name_created_idx
    on cardapio.events (restaurant_id, name, created_at);
create index if not exists cardapio_events_restaurant_session_idx
    on cardapio.events (restaurant_id, session_id);
create index if not exists cardapio_events_restaurant_product_created_idx
    on cardapio.events (restaurant_id, product_slug, created_at)
    where product_slug <> '';
-- Dedupe: o mesmo eventId (uuid do cliente) so entra uma vez por restaurante. O
-- INSERT da ingestao faz ON CONFLICT DO NOTHING contra este indice parcial.
create unique index if not exists cardapio_events_restaurant_event_uidx
    on cardapio.events (restaurant_id, event_id)
    where event_id <> '';

-- Casamento evento<->pedido pela sessao (funil/conversao).
create index if not exists cardapio_orders_restaurant_session_idx
    on cardapio.orders (restaurant_id, session_id)
    where session_id <> '';
