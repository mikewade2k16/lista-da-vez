-- Modulo automation (fase M2): personas por automacao. O comportamento da IA
-- (instrucoes + conhecimento) deixa de ficar cravado no n8n e passa a vir do banco,
-- montado no runtime-config (persona ativa + guardrails). Tenant-aware.

create table if not exists automation.personas (
    id            uuid        primary key default gen_random_uuid(),
    automation_id uuid        not null references automation.automations(id) on delete cascade,
    account_id    uuid        not null references core.accounts(id) on delete cascade,
    name          text        not null,
    system_prompt text        not null,
    is_active     boolean     not null default false,
    created_at    timestamptz not null default now(),
    updated_at    timestamptz not null default now()
);
create index if not exists automation_personas_automation_idx on automation.personas(automation_id);
-- so 1 persona ativa por automacao:
create unique index if not exists automation_personas_one_active_idx
    on automation.personas(automation_id) where is_active;
