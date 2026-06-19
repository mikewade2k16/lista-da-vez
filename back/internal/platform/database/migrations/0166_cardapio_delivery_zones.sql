-- Modulo cardapio (Fase 2, WS-A): zonas de entrega (bairros + valor de entrega).
-- Cada zona pertence a um restaurante e a sua account (defesa em profundidade,
-- igual as demais tabelas do schema cardapio.*). O frete do pedido publico passa
-- a poder vir da zona escolhida (fee_cents) em vez do valor fixo das settings.
-- Idempotente, schema-qualificado, sem -- +goose Down (o migrator roda o arquivo
-- inteiro). Plano canonico: docs/cardapio/PLANO_CARDAPIO_FASE2.md secao WS-A.

create table if not exists cardapio.delivery_zones (
    id            uuid        primary key default gen_random_uuid(),
    account_id    uuid        not null references core.accounts(id) on delete cascade,
    restaurant_id uuid        not null references cardapio.restaurants(id) on delete cascade,
    name          text        not null,
    fee_cents     bigint      not null default 0,
    is_active     boolean     not null default true,
    sort_order    integer     not null default 0,
    created_at    timestamptz not null default now()
);
create unique index if not exists cardapio_delivery_zones_name_uidx
    on cardapio.delivery_zones (restaurant_id, lower(name));
create index if not exists cardapio_delivery_zones_account_idx
    on cardapio.delivery_zones (account_id);
create index if not exists cardapio_delivery_zones_restaurant_sort_idx
    on cardapio.delivery_zones (restaurant_id, sort_order);
