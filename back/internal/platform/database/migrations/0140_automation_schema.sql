-- Modulo automation (fase P1, fatia M1): schema automation-centric minimo para o
-- painel de Status/Conectar do WhatsApp. Tenant-aware: toda tabela tem account_id
-- NOT NULL com FK para core.accounts. Demais tabelas (personas, provider_credentials,
-- knowledge/RAG, model_catalog, ...) entram em migrations posteriores conforme as
-- fases. Desenho completo: docs/automation/schema_automation_sketch.sql.

create schema if not exists automation;

-- Automations: o "robo". N por account. Entidade central do modulo.
create table if not exists automation.automations (
    id          uuid primary key default gen_random_uuid(),
    account_id  uuid        not null references core.accounts(id) on delete cascade,
    type        text        not null default 'atendimento',
    name        text        not null,
    slug        text        not null,
    status      text        not null default 'draft',          -- draft | active | paused (active = ligado)
    settings    jsonb       not null default '{}'::jsonb,
    created_at  timestamptz not null default now(),
    updated_at  timestamptz not null default now(),
    unique (account_id, slug)
);
create index if not exists automation_automations_account_idx on automation.automations(account_id);

-- Channels: conexao do canal (WAHA session = 1 numero). provider pluggable
-- (waha | evolution | cloud_api). session_name unico global resolve a automacao.
create table if not exists automation.channels (
    id              uuid        primary key default gen_random_uuid(),
    automation_id   uuid        not null references automation.automations(id) on delete cascade,
    account_id      uuid        not null references core.accounts(id) on delete cascade,
    provider        text        not null default 'waha',
    session_name    text        not null,
    status          text        not null default 'STOPPED',
    connected_phone text,
    updated_at      timestamptz not null default now(),
    unique (session_name)
);
create index if not exists automation_channels_automation_idx on automation.channels(automation_id);
create index if not exists automation_channels_account_idx on automation.channels(account_id);
