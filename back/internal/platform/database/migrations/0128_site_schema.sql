-- 0128 — Schema `site` (leads + products via webhook/API + admin CRUD).
--
-- Motivacao: as telas /manage/leads-web e /manage/produtos-web vinham do BFF
-- mock. Decisao 2026-05-29: viram features reais com ingest via webhook +
-- admin CRUD. Backend dedicado em back/internal/modules/site/.
--
-- Convencao multitenant: toda tabela tem account_id NOT NULL com FK para
-- core.accounts (regra do AGENT_RULES.md).

create schema if not exists site;

-- ============================================================================
-- site.webhook_sources — fontes externas cadastradas por account
-- ============================================================================
-- Cada source guarda um secret_hash (sha256 do secret HMAC). O secret e
-- mostrado uma unica vez na criacao/rotate; o cliente passa o secret no
-- header X-Signature do POST /v1/webhooks/<entity>/<slug> para autenticar.
-- payload_mapping: regras de mapeamento de campos do payload para a tabela
-- destino (jsonb com chaves padronizadas, default null = mapeamento direto
-- por nome). Permite que sites diferentes mandem shape diferente sem mudar
-- codigo.

create table if not exists site.webhook_sources (
    id uuid primary key default gen_random_uuid(),
    account_id uuid not null references core.accounts(id) on delete cascade,
    slug text not null,
    name text not null,
    entity_type text not null check (entity_type in ('leads', 'products')),
    -- Secret armazenado em claro: HMAC do webhook usa o secret como chave
    -- (nao da pra validar a partir do hash). Tratado como dado sensivel
    -- equivalente a password_hash: nao logar, so retornar na criacao/rotate.
    secret text not null,
    payload_mapping jsonb,
    is_active boolean not null default true,
    created_at timestamptz not null default now(),
    updated_at timestamptz not null default now(),
    constraint site_webhook_sources_slug_unique unique (slug)
);

create index if not exists site_webhook_sources_account_idx
    on site.webhook_sources (account_id, is_active);

-- ============================================================================
-- site.leads — leads captados (manual ou via webhook)
-- ============================================================================

create table if not exists site.leads (
    id uuid primary key default gen_random_uuid(),
    account_id uuid not null references core.accounts(id) on delete cascade,
    source_id uuid references site.webhook_sources(id) on delete set null,
    source_label text not null default '',
    nome text not null default '',
    email text not null default '',
    telefone text not null default '',
    page text not null default '',
    cupom text not null default '',
    consent boolean not null default false,
    consent_label text not null default '',
    tracking_data jsonb,
    payload_raw jsonb,
    status text not null default 'new' check (status in ('new', 'contacted', 'qualified', 'lost')),
    notes text not null default '',
    is_active boolean not null default true,
    created_at timestamptz not null default now(),
    updated_at timestamptz not null default now()
);

create index if not exists site_leads_account_created_idx
    on site.leads (account_id, created_at desc) where is_active = true;
create index if not exists site_leads_email_lower_idx
    on site.leads (account_id, lower(email)) where is_active = true and email <> '';
create index if not exists site_leads_source_idx
    on site.leads (source_id) where source_id is not null;

-- ============================================================================
-- site.products — catalogo de produtos do site (separado do crm.products)
-- ============================================================================
-- Diferente de crm.products (que e raw do ERP): site.products e o catalogo
-- exposto no site publico, podendo ser preenchido manualmente, via webhook
-- ou (no futuro) puxado de um layout enriquecido sobre crm.products com
-- imagens/extras adicionados pelo admin.

create table if not exists site.products (
    id uuid primary key default gen_random_uuid(),
    account_id uuid not null references core.accounts(id) on delete cascade,
    source_id uuid references site.webhook_sources(id) on delete set null,
    source_label text not null default '',
    name text not null default '',
    code text not null default '',
    description text not null default '',
    image text not null default '',
    categories jsonb,
    campaigns jsonb,
    price numeric(14,2) not null default 0,
    fator numeric(8,4) not null default 1,
    tipo text not null default '',
    stock integer not null default 0,
    status text not null default 'active' check (status in ('active', 'inactive')),
    payload_raw jsonb,
    is_active boolean not null default true,
    created_at timestamptz not null default now(),
    updated_at timestamptz not null default now()
);

create index if not exists site_products_account_idx
    on site.products (account_id, status, name) where is_active = true;
create index if not exists site_products_account_code_idx
    on site.products (account_id, code) where is_active = true and code <> '';
create index if not exists site_products_source_idx
    on site.products (source_id) where source_id is not null;

-- Views public.* para legacy (codigo antigo que ainda apontava para o nome
-- "leads"/"products" sem schema). Inertes se ja existirem.
create or replace view public.site_leads as select * from site.leads;
create or replace view public.site_products as select * from site.products;
