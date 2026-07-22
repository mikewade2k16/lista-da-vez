-- Omnichannel MVP-01: perfil de automacao por cliente/numero/agente.
--
-- Go/PostgreSQL continuam autoritativos. O n8n recebe contexto e devolve sugestoes,
-- mas nunca escolhe o perfil, muda o estado da conversa ou envia ao canal.
--
-- O perfil e provider-neutral: whatsapp_instance_id pode apontar hoje para Evolution
-- e depois para WhatsApp Cloud sem alterar o contrato do cerebro.

create schema if not exists messaging;

-- Necessarios para FKs compostas que impedem referencias cross-tenant no proprio banco.
create unique index if not exists messaging_whatsapp_instances_account_id_id_uidx
    on messaging.whatsapp_instances (account_id, id);

create unique index if not exists messaging_ai_agents_account_id_id_uidx
    on messaging.ai_agents (account_id, id);

create table if not exists messaging.automation_profiles (
    id uuid primary key default gen_random_uuid(),
    account_id uuid not null references core.accounts(id) on delete cascade,
    client_account_id uuid not null references core.accounts(id) on delete cascade,
    whatsapp_instance_id uuid not null,
    ai_agent_id uuid not null,
    enabled boolean not null default false,

    -- A IA apenas sugere o encerramento. O Go avalia esta policy e a lease
    -- messaging.conversations.ai_generation antes de aceitar a transicao.
    auto_close_enabled boolean not null default false,
    auto_close_min_confidence numeric(4,3) not null default 0.900
        check (auto_close_min_confidence >= 0 and auto_close_min_confidence <= 1),
    auto_close_require_all_fields boolean not null default true,
    auto_close_block_human_request boolean not null default true,
    auto_close_block_sensitive boolean not null default true,

    revision bigint not null default 1 check (revision > 0),
    created_by_user_id uuid references core.users(id) on delete set null,
    updated_by_user_id uuid references core.users(id) on delete set null,
    created_at timestamptz not null default now(),
    updated_at timestamptz not null default now(),

    constraint messaging_automation_profiles_instance_fk
        foreign key (account_id, whatsapp_instance_id)
        references messaging.whatsapp_instances(account_id, id) on delete restrict,
    constraint messaging_automation_profiles_agent_fk
        foreign key (account_id, ai_agent_id)
        references messaging.ai_agents(account_id, id) on delete restrict,

    -- Um cliente tem um perfil e um numero pertence a um unico cliente no MVP.
    unique (account_id, client_account_id),
    unique (account_id, whatsapp_instance_id)
);

create index if not exists messaging_automation_profiles_account_enabled_idx
    on messaging.automation_profiles (account_id, enabled, updated_at desc);

create index if not exists messaging_automation_profiles_agent_idx
    on messaging.automation_profiles (account_id, ai_agent_id);
