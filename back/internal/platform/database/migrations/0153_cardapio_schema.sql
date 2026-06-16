-- Modulo cardapio (C1): schema cardapio para os cardapios online (restaurantes)
-- servidos por um front Nuxt estatico no host do cliente. Multitenant igual a bio:
-- toda tabela tem account_id NOT NULL com FK para core.accounts (defesa em
-- profundidade) alem do FK de agregacao. Idempotente, schema-qualificado, sem
-- -- +goose Down. Plano canonico: docs/cardapio/PLANO_MODULO_CARDAPIO.md secao 3.

create schema if not exists cardapio;

-- Restaurants: a entidade central. N por account. slug unico global (lower).
create table if not exists cardapio.restaurants (
    id                uuid        primary key default gen_random_uuid(),
    account_id        uuid        not null references core.accounts(id) on delete cascade,
    slug              text        not null,
    name              text        not null,
    tagline           text        not null default '',
    description       text        not null default '',
    logo_url          text        not null default '',
    banner_url        text        not null default '',
    whatsapp          text        not null default '',
    phone             text        not null default '',
    email             text        not null default '',
    instagram         text        not null default '',
    address           jsonb       not null default '{}'::jsonb,
    hours             jsonb       not null default '[]'::jsonb,
    settings          jsonb       not null default '{}'::jsonb,
    theme             jsonb       not null default '{}'::jsonb,
    is_active         boolean     not null default false,
    last_order_number integer     not null default 0,
    created_at        timestamptz not null default now(),
    updated_at        timestamptz not null default now()
);
create unique index if not exists cardapio_restaurants_slug_uidx
    on cardapio.restaurants (lower(slug));
create index if not exists cardapio_restaurants_account_idx
    on cardapio.restaurants (account_id);

-- Domains: host -> restaurante. host normalizado (lowercase, sem porta, sem www.).
create table if not exists cardapio.restaurant_domains (
    host          text        primary key,
    restaurant_id uuid        not null references cardapio.restaurants(id) on delete cascade,
    account_id    uuid        not null references core.accounts(id) on delete cascade,
    is_primary    boolean     not null default false,
    created_at    timestamptz not null default now()
);
create index if not exists cardapio_restaurant_domains_restaurant_idx
    on cardapio.restaurant_domains (restaurant_id);
create index if not exists cardapio_restaurant_domains_account_idx
    on cardapio.restaurant_domains (account_id);

-- Categories: agrupam produtos. Unique (restaurant_id, lower(slug)).
create table if not exists cardapio.categories (
    id            uuid        primary key default gen_random_uuid(),
    account_id    uuid        not null references core.accounts(id) on delete cascade,
    restaurant_id uuid        not null references cardapio.restaurants(id) on delete cascade,
    slug          text        not null,
    name          text        not null,
    description   text        not null default '',
    sort_order    integer     not null default 0,
    is_active     boolean     not null default true,
    created_at    timestamptz not null default now()
);
create unique index if not exists cardapio_categories_slug_uidx
    on cardapio.categories (restaurant_id, lower(slug));
create index if not exists cardapio_categories_account_idx
    on cardapio.categories (account_id);
create index if not exists cardapio_categories_restaurant_idx
    on cardapio.categories (restaurant_id);

-- Products: o prato. price_cents inteiro. Unique (restaurant_id, lower(slug)).
create table if not exists cardapio.products (
    id            uuid        primary key default gen_random_uuid(),
    account_id    uuid        not null references core.accounts(id) on delete cascade,
    restaurant_id uuid        not null references cardapio.restaurants(id) on delete cascade,
    category_id   uuid        references cardapio.categories(id) on delete set null,
    slug          text        not null,
    name          text        not null,
    short_desc    text        not null default '',
    description   text        not null default '',
    body          text        not null default '',
    price_cents   bigint      not null default 0,
    image_url     text        not null default '',
    gallery       jsonb       not null default '[]'::jsonb,
    weight        text        not null default '',
    cook_time     text        not null default '',
    diet          jsonb       not null default '[]'::jsonb,
    allergens     jsonb       not null default '[]'::jsonb,
    pairing       jsonb,
    tags          jsonb       not null default '[]'::jsonb,
    is_available  boolean     not null default true,
    is_featured   boolean     not null default false,
    sort_order    integer     not null default 0,
    rating        numeric,
    review_count  integer     not null default 0,
    sold_count    integer     not null default 0,
    created_at    timestamptz not null default now(),
    updated_at    timestamptz not null default now()
);
create unique index if not exists cardapio_products_slug_uidx
    on cardapio.products (restaurant_id, lower(slug));
create index if not exists cardapio_products_account_idx
    on cardapio.products (account_id);
create index if not exists cardapio_products_restaurant_idx
    on cardapio.products (restaurant_id);
create index if not exists cardapio_products_category_idx
    on cardapio.products (category_id);

-- Variations: opcoes mutuamente exclusivas (tamanho etc). price_delta_cents soma.
create table if not exists cardapio.product_variations (
    id                uuid    primary key default gen_random_uuid(),
    account_id        uuid    not null references core.accounts(id) on delete cascade,
    product_id        uuid    not null references cardapio.products(id) on delete cascade,
    name              text    not null,
    price_delta_cents bigint  not null default 0,
    sort_order        integer not null default 0
);
create index if not exists cardapio_product_variations_account_idx
    on cardapio.product_variations (account_id);
create index if not exists cardapio_product_variations_product_idx
    on cardapio.product_variations (product_id);

-- Addons: adicionais cumulativos. price_cents soma.
create table if not exists cardapio.product_addons (
    id          uuid    primary key default gen_random_uuid(),
    account_id  uuid    not null references core.accounts(id) on delete cascade,
    product_id  uuid    not null references cardapio.products(id) on delete cascade,
    name        text    not null,
    price_cents bigint  not null default 0,
    sort_order  integer not null default 0
);
create index if not exists cardapio_product_addons_account_idx
    on cardapio.product_addons (account_id);
create index if not exists cardapio_product_addons_product_idx
    on cardapio.product_addons (product_id);

-- Reviews: avaliacoes curadas por produto. rating 1-5 validado no service.
create table if not exists cardapio.reviews (
    id            uuid        primary key default gen_random_uuid(),
    account_id    uuid        not null references core.accounts(id) on delete cascade,
    restaurant_id uuid        not null references cardapio.restaurants(id) on delete cascade,
    product_id    uuid        not null references cardapio.products(id) on delete cascade,
    author_name   text        not null,
    author_level  text        not null default '',
    rating        integer     not null default 5,
    body          text        not null default '',
    is_highlight  boolean     not null default false,
    date_label    text        not null default '',
    sort_order    integer     not null default 0,
    created_at    timestamptz not null default now()
);
create index if not exists cardapio_reviews_account_idx
    on cardapio.reviews (account_id);
create index if not exists cardapio_reviews_restaurant_idx
    on cardapio.reviews (restaurant_id);
create index if not exists cardapio_reviews_product_idx
    on cardapio.reviews (product_id);

-- Orders: pedidos recebidos pelo cardapio publico. order_number sequencial por
-- restaurante (via last_order_number atomico). Valores sempre em centavos.
create table if not exists cardapio.orders (
    id                 uuid        primary key default gen_random_uuid(),
    account_id         uuid        not null references core.accounts(id) on delete cascade,
    restaurant_id      uuid        not null references cardapio.restaurants(id) on delete cascade,
    customer_id        uuid,
    order_number       integer     not null,
    status             text        not null default 'recebido',
    type               text        not null,
    session_id         text        not null default '',
    customer_name      text        not null default '',
    customer_phone     text        not null default '',
    delivery_address   jsonb       not null default '{}'::jsonb,
    notes              text        not null default '',
    subtotal_cents     bigint      not null default 0,
    delivery_fee_cents bigint      not null default 0,
    discount_cents     bigint      not null default 0,
    total_cents        bigint      not null default 0,
    created_at         timestamptz not null default now(),
    updated_at         timestamptz not null default now()
);
create index if not exists cardapio_orders_account_idx
    on cardapio.orders (account_id);
create index if not exists cardapio_orders_restaurant_status_idx
    on cardapio.orders (restaurant_id, status, created_at);

-- Order items: snapshot do item no momento do pedido. product_id nullable (o
-- snapshot sobrevive a delete do produto). addons jsonb [{name, priceCents}].
create table if not exists cardapio.order_items (
    id               uuid    primary key default gen_random_uuid(),
    account_id       uuid    not null references core.accounts(id) on delete cascade,
    order_id         uuid    not null references cardapio.orders(id) on delete cascade,
    product_id       uuid    references cardapio.products(id) on delete set null,
    product_name     text    not null,
    variation_name   text    not null default '',
    addons           jsonb   not null default '[]'::jsonb,
    quantity         integer not null default 1,
    unit_price_cents bigint  not null default 0,
    total_cents      bigint  not null default 0,
    notes            text    not null default ''
);
create index if not exists cardapio_order_items_account_idx
    on cardapio.order_items (account_id);
create index if not exists cardapio_order_items_order_idx
    on cardapio.order_items (order_id);

-- Events: telemetria do front publico (page_view, add_to_cart, ...). context jsonb.
create table if not exists cardapio.events (
    id            uuid        primary key default gen_random_uuid(),
    account_id    uuid        not null references core.accounts(id) on delete cascade,
    restaurant_id uuid        not null references cardapio.restaurants(id) on delete cascade,
    name          text        not null,
    session_id    text        not null default '',
    context       jsonb       not null default '{}'::jsonb,
    created_at    timestamptz not null default now()
);
create index if not exists cardapio_events_account_idx
    on cardapio.events (account_id);
create index if not exists cardapio_events_restaurant_created_idx
    on cardapio.events (restaurant_id, created_at);
