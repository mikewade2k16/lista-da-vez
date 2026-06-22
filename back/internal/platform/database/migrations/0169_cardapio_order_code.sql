-- Modulo cardapio (Fase 2, WS-G): codigo curto e legivel do pedido, mostrado ao
-- cliente na confirmacao (e no WhatsApp). O order_number sequencial continua
-- existindo para o painel; o code e o identificador voltado ao cliente.
-- Idempotente, schema-qualificado, sem -- +goose Down (o migrator roda o arquivo
-- inteiro). Unique PARCIAL (ignora pedidos antigos com code = '') garante
-- unicidade por restaurante sem colidir com o backfill vazio.
-- Plano: docs/cardapio/PLANO_CARDAPIO_FASE2.md (WS-G).

alter table cardapio.orders
    add column if not exists code text not null default '';

create unique index if not exists cardapio_orders_code_uidx
    on cardapio.orders (restaurant_id, code) where code <> '';
