-- 0150 — Assistente MCP do Meta Ads: historico do chat (meta_ads.assistant_messages).
--
-- Motivacao: a pagina /meta-ads ganha um chat que cria/edita campanhas via o
-- MCP oficial da Meta (Claude headless no agent-runner, fase MA2). Cada mensagem
-- (usuario e assistente) e persistida por account para historico e auditoria;
-- `actions` jsonb registra as tools executadas pelo runner ({tool, summary,
-- status}) — NULL quando a resposta nao executou acao nenhuma.
-- Plano canonico: docs/meta-ads/PLANO_INTEGRACAO_META_ADS.md (secao 12).
--
-- Convencao multitenant (AGENT_RULES.md): account_id NOT NULL com FK para
-- core.accounts; toda leitura/escrita filtra por account_id.

create schema if not exists meta_ads;

create table if not exists meta_ads.assistant_messages (
    id uuid primary key default gen_random_uuid(),
    account_id uuid not null references core.accounts(id) on delete cascade,
    role text not null check (role in ('user','assistant')),
    content text not null default '',
    actions jsonb,
    created_at timestamptz not null default now()
);

create index if not exists meta_ads_assistant_messages_account_idx
    on meta_ads.assistant_messages (account_id, created_at desc);
