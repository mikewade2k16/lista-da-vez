-- 0151 — Configuracoes do assistente meta_ads por account (modelo + system prompt
-- editaveis no painel). 1 linha por account (a agencia). Vazio = usa o default do
-- runner. Plano: docs/meta-ads/PLANO_INTEGRACAO_META_ADS.md (assistente MCP).

create table if not exists meta_ads.assistant_settings (
    account_id uuid primary key references core.accounts(id) on delete cascade,
    model text not null default '',
    system_prompt text not null default '',
    updated_at timestamptz not null default now()
);
