-- Modulo cardapio: registro das fotos de PRODUTO convertidas (aba Produtos).
-- Cada upload guarda o original e gera um AVIF para o site + miniatura AVIF para
-- a tabela do painel. A listagem de produtos le thumb_url por (account_id, url).
-- original_bytes acima do teto de revisao vira lembrete para o admin; reviewed_at
-- marca o original ja tratado. Nao altera cardapio.products.
-- Idempotente e schema-qualificada.

create table if not exists cardapio.product_photos (
    id             uuid primary key default gen_random_uuid(),
    account_id     uuid not null references core.accounts(id) on delete cascade,
    restaurant_id  uuid references cardapio.restaurants(id) on delete set null,
    url            text not null,
    thumb_url      text not null default '',
    original_url   text not null default '',
    original_bytes bigint not null default 0,
    source         text not null default 'upload',
    reviewed_at    timestamptz,
    created_at     timestamptz not null default now()
);

create unique index if not exists cardapio_product_photos_account_url_uidx
    on cardapio.product_photos (account_id, url);

create index if not exists cardapio_product_photos_restaurant_idx
    on cardapio.product_photos (account_id, restaurant_id, created_at desc);
