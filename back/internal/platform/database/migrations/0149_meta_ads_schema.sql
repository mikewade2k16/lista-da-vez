-- 0149 — Schema `meta_ads` (integracao Meta/Facebook Ads: conexao + cache de
-- contas de anuncio, campanhas e insights diarios).
--
-- Motivacao: a agencia (Crow Visuals) gere trafego pago de Meta hoje FORA do
-- painel. Este modulo puxa os dados da Marketing API para o nosso banco (fonte
-- de verdade dos relatorios) e, em fase seguinte, cria/edita campanhas.
-- Plano canonico: docs/meta-ads/PLANO_INTEGRACAO_META_ADS.md.
--
-- Convencao multitenant (AGENT_RULES.md): toda tabela tem account_id NOT NULL
-- com FK para core.accounts. organization_id/client_account_id entram NULLABLE
-- e RESERVADOS para quando o modelo agencia->cliente
-- (docs/AGENCY_TENANT_ARCHITECTURE.md) for ligado — backfill na fase P5. O MVP
-- NAO depende do modelo de agencia.
--
-- O System User token (sensivel, acesso total a conta de anuncios) e guardado
-- CIFRADO via pgcrypto (pgp_sym_encrypt com chave META_ADS_CRYPTO_KEY). Nunca
-- logar; so existe em claro dentro do processo Go no momento da chamada a Graph.

create extension if not exists pgcrypto;

create schema if not exists meta_ads;

-- ============================================================================
-- meta_ads.connections — 1 conexao Meta por account (System User token cifrado)
-- ============================================================================

create table if not exists meta_ads.connections (
    id uuid primary key default gen_random_uuid(),
    account_id uuid not null references core.accounts(id) on delete cascade,
    -- organization_id: reservado (sem FK no MVP). FK para core.organizations
    -- entra na fase P5, junto com o backfill do modelo de agencia.
    organization_id uuid,
    meta_business_id text not null default '',
    name text not null default '',
    -- token cifrado at-rest (bytea de pgp_sym_encrypt). Tratado como segredo:
    -- nunca retornado ao front, nunca logado.
    encrypted_token bytea not null,
    token_expires_at timestamptz,
    status text not null default 'active' check (status in ('active', 'revoked')),
    created_at timestamptz not null default now(),
    updated_at timestamptz not null default now(),
    constraint meta_ads_connections_account_unique unique (account_id)
);

create index if not exists meta_ads_connections_account_idx
    on meta_ads.connections (account_id);

-- ============================================================================
-- meta_ads.ad_accounts — contas de anuncio (act_...) descobertas na conexao
-- ============================================================================

create table if not exists meta_ads.ad_accounts (
    id uuid primary key default gen_random_uuid(),
    account_id uuid not null references core.accounts(id) on delete cascade,
    connection_id uuid not null references meta_ads.connections(id) on delete cascade,
    meta_ad_account_id text not null,
    -- client_account_id: reservado p/ atribuir a conta de anuncio ao cliente da
    -- agencia que ela atende (fase P5). Nullable no MVP.
    client_account_id uuid references core.accounts(id) on delete set null,
    name text not null default '',
    currency text not null default '',
    status text not null default '',
    created_at timestamptz not null default now(),
    updated_at timestamptz not null default now(),
    constraint meta_ads_ad_accounts_unique unique (account_id, meta_ad_account_id)
);

create index if not exists meta_ads_ad_accounts_account_idx
    on meta_ads.ad_accounts (account_id);

-- ============================================================================
-- meta_ads.campaigns — cache de campanhas (sync da Marketing API)
-- ============================================================================

create table if not exists meta_ads.campaigns (
    id uuid primary key default gen_random_uuid(),
    account_id uuid not null references core.accounts(id) on delete cascade,
    ad_account_id uuid not null references meta_ads.ad_accounts(id) on delete cascade,
    meta_campaign_id text not null,
    name text not null default '',
    objective text not null default '',
    status text not null default '',
    daily_budget numeric(15,2),
    lifetime_budget numeric(15,2),
    synced_at timestamptz not null default now(),
    constraint meta_ads_campaigns_unique unique (ad_account_id, meta_campaign_id)
);

create index if not exists meta_ads_campaigns_account_idx
    on meta_ads.campaigns (account_id);

-- ============================================================================
-- meta_ads.insights_daily — cache de metricas diarias (alimenta graficos)
-- ============================================================================
-- meta_campaign_id = '' (string vazia, NAO null) representa o agregado da conta
-- de anuncio no dia — assim o UNIQUE deduplica (NULLs seriam distintos).

create table if not exists meta_ads.insights_daily (
    id uuid primary key default gen_random_uuid(),
    account_id uuid not null references core.accounts(id) on delete cascade,
    ad_account_id uuid not null references meta_ads.ad_accounts(id) on delete cascade,
    meta_campaign_id text not null default '',
    date date not null,
    impressions bigint not null default 0,
    clicks bigint not null default 0,
    spend numeric(15,2) not null default 0,
    reach bigint not null default 0,
    ctr numeric(8,4) not null default 0,
    cpc numeric(12,4) not null default 0,
    cpm numeric(12,4) not null default 0,
    conversions numeric(15,2) not null default 0,
    synced_at timestamptz not null default now(),
    constraint meta_ads_insights_daily_unique unique (ad_account_id, meta_campaign_id, date)
);

create index if not exists meta_ads_insights_daily_account_date_idx
    on meta_ads.insights_daily (account_id, date desc);
create index if not exists meta_ads_insights_daily_adaccount_date_idx
    on meta_ads.insights_daily (ad_account_id, date desc);
