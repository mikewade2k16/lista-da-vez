-- 0155 — Cruzamento de produtos do site com itens do ERP (erp_item_current).
--
-- Motivacao: cada site.products.code pode conter varios codigos de produto
-- separados por '_' (ex.: "368252_360856"); cada segmento casa com um sku do
-- ERP (erp_item_current.sku == identifier == codigo). O ERP esta escopado por
-- tenant_id (== account_id do site) + store_id. Esta tabela materializa o
-- cruzamento (produto x sku do ERP) para enriquecer o GET de produtos com
-- nome/descricao vindos do ERP, sem sobrescrever os campos proprios do produto.
--
-- Convencao multitenant: account_id NOT NULL com FK para core.accounts.
-- Idempotente, schema-qualificado, sem -- +goose Down.

create table if not exists site.product_erp_links (
    id              uuid        primary key default gen_random_uuid(),
    account_id      uuid        not null references core.accounts(id),
    product_id      uuid        not null references site.products(id) on delete cascade,
    erp_sku         text        not null,
    erp_name        text,
    erp_description text,
    matched_at      timestamptz not null default now(),
    unique (product_id, erp_sku)
);

create index if not exists site_product_erp_links_account_idx
    on site.product_erp_links (account_id);

create index if not exists site_product_erp_links_product_idx
    on site.product_erp_links (product_id);

create index if not exists site_product_erp_links_sku_idx
    on site.product_erp_links (erp_sku);
