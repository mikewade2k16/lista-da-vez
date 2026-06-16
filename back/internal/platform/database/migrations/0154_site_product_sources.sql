-- 0154 — Fontes externas de produtos + colunas de origem em site.products (B8).
--
-- Motivacao: a bio/site le produtos de site.products. Esta iteracao POPULA
-- site.products puxando da API publica do site do proprio cliente (primeiro
-- cliente = Perola: https://perolajoias.com/api/products/). Arquitetura
-- plugavel: cada account tem zero/mais fontes externas em site.product_sources.
--
-- Convencao multitenant: toda tabela tem account_id NOT NULL com FK para
-- core.accounts. Idempotente, schema-qualificado, sem -- +goose Down.

-- ============================================================================
-- site.product_sources — config de fonte externa de produtos por account
-- ============================================================================
-- type 'external_api': GET {base_url}?page=&limit= seguindo meta.has_more.
-- Sem credencial (GET publico). Plugavel para outros clientes no futuro.

create table if not exists site.product_sources (
    id         uuid        primary key default gen_random_uuid(),
    account_id uuid        not null references core.accounts(id) on delete cascade,
    type       text        not null default 'external_api' check (type in ('external_api')),
    base_url   text        not null,
    enabled    boolean     not null default true,
    created_at timestamptz not null default now(),
    updated_at timestamptz not null default now()
);

create index if not exists site_product_sources_account_idx
    on site.product_sources (account_id, enabled);

-- ============================================================================
-- site.products — colunas de origem para upsert idempotente do sync
-- ============================================================================
-- external_id: id do produto na origem (chave de upsert por account+source).
-- source: identificador da fonte (ex.: 'perola'); '' para itens manuais/webhook.

alter table site.products add column if not exists external_id text not null default '';
alter table site.products add column if not exists source text not null default '';

-- Indice unico para o ON CONFLICT do upsert. Parcial: so vale para itens com
-- origem externa (source/external_id preenchidos), nao trava os manuais (vazios).
create unique index if not exists site_products_account_source_external_uidx
    on site.products (account_id, source, external_id)
    where source <> '' and external_id <> '';

-- ============================================================================
-- Registro idempotente da fonte da Perola (apenas se a account 'perola' existir)
-- ============================================================================
-- Identifica a account Perola por slug/name (lower). ON CONFLICT DO NOTHING
-- evita duplicar caso ja exista uma fonte para a mesma account+base_url.

create unique index if not exists site_product_sources_account_baseurl_uidx
    on site.product_sources (account_id, base_url);

insert into site.product_sources (account_id, type, base_url, enabled)
select a.id, 'external_api', 'https://perolajoias.com/api/products/', true
from core.accounts a
where lower(a.slug) = 'perola' or lower(a.name) = 'perola'
on conflict (account_id, base_url) do nothing;
