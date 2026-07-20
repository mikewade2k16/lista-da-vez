-- Modulo Omnichannel — F13 (LGPD): evidencia de que a politica de retencao RODOU.
-- Plano canonico: docs/omnichannel/PLANO_ATENDIMENTO.md (§9.2 F13, §10).
-- Spec: docs/omnichannel/specs/OMNI-F13.md (C3).
--
-- Numero: 0209 (a ultima no disco na hora de escrever era 0208_messaging_audit_actions.sql).
--
-- messaging.purge_runs e a TRILHA da poda: uma linha por (conta, classe) por execucao, com
-- as contagens do que caiu (banco e disco). Sem registro, "temos purge" e' afirmacao sem
-- prova — e e' o que um DPO/auditor pede primeiro. A tabela e classe `audit` e PODA A SI
-- MESMA aos 365 dias (o purge da classe audit apaga purge_runs velhas): sem isso a tabela
-- de evidencia cresceria para sempre.
--
-- DECISAO desta fase (registrada na spec, Divergencia): NAO se adiciona ai_runs.cost_priced.
-- A F9 (0206) ja calcula e CONGELA cost_usd no dispatch a partir de core.platform_settings
-- key 'ai_model_pricing'. O status "preco cadastrado?" e derivado no READ direto dessa
-- tabela de preco (autoritativa), sem heuristica e sem coluna que nasceria `false` em toda
-- linha antiga (default que mentiria ate um retrofit da F9). Ver OMNI-F13 C7 / http_cost.go.
--
-- Idempotente, schema qualificado, SEM `-- +goose Down` (o migrator roda o arquivo inteiro;
-- um Down aqui se auto-destruiria no mesmo boot). Modelo de estilo: 0200/0206.

create schema if not exists messaging;

create table if not exists messaging.purge_runs (
    id            uuid primary key default gen_random_uuid(),
    account_id    uuid not null references core.accounts(id) on delete cascade,
    class         text not null,
    mode          text not null default 'delete',   -- delete | dry_run
    cutoff_at     timestamptz not null,
    rows_deleted  bigint not null default 0,
    rows_scrubbed bigint not null default 0,
    files_deleted bigint not null default 0,
    bytes_freed   bigint not null default 0,
    started_at    timestamptz not null default now(),
    finished_at   timestamptz,
    error         text not null default '',          -- MASCARADO (C6): so a classe do erro
    constraint messaging_purge_runs_class_ck
        check (class in ('audit', 'conversation', 'ai_io', 'ephemeral', 'media_orphan')),
    constraint messaging_purge_runs_mode_ck
        check (mode in ('delete', 'dry_run'))
);

create index if not exists messaging_purge_runs_account_idx
    on messaging.purge_runs (account_id, started_at desc);
