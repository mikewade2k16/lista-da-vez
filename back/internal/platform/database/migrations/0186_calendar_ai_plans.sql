-- Modulo Calendario — planos de conteudo gerados por IA (Fase 6)
-- Plano: docs/CALENDARIO_PLAN.md (secao 3.8); contrato C4 em docs/CALENDARIO_SPECS.md.
--
-- Cada linha e uma solicitacao de plano do mes: o painel dispara o n8n ("Calendar
-- Omni"), que gera o conteudo (resumo + pilares + ideias por cliente/dia) e chama
-- de volta o callback publico. account_id = dona do calendario (Principal). status
-- transita pending -> done | error; done -> applied. content jsonb = shape C4.content
-- (preenchido no callback). client_ids jsonb = uuids escolhidos na solicitacao.
-- provider/model = snapshot da config da conta no momento do disparo.
-- Idempotente, schema qualificado, sem `-- +goose Down` (o migrator roda o arquivo inteiro).

create schema if not exists calendar;

create table if not exists calendar.ai_plans (
    id uuid not null default gen_random_uuid(),
    account_id uuid not null references core.accounts(id) on delete cascade,
    month_key text not null,
    client_ids jsonb not null default '[]'::jsonb,
    status text not null default 'pending',
    provider text not null default '',
    model text not null default '',
    content jsonb not null default '{}'::jsonb,
    error text not null default '',
    created_by text not null default '',
    created_at timestamptz not null default now(),
    updated_at timestamptz not null default now(),
    primary key (id)
);
create index if not exists calendar_ai_plans_account_month_idx
    on calendar.ai_plans (account_id, month_key);
create index if not exists calendar_ai_plans_account_created_idx
    on calendar.ai_plans (account_id, created_at desc);
