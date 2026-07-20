-- Modulo Omnichannel — schema messaging.* (atendimento WhatsApp).
-- Plano canonico: docs/omnichannel/PLANO_ATENDIMENTO.md (§7). Spec: docs/omnichannel/specs/OMNI-F2.md.
--
-- As 8 tabelas do port (fonte coluna a coluna: o Prisma do legado
-- whats-test/apps/atendimento-online-api/prisma/schema.prisma, camelCase -> snake_case,
-- tenantId -> account_id) + messaging.outbox (a 9a; envio duravel — a TABELA nasce aqui,
-- o engine e a F3 em platform/jobs).
--
-- Decisoes que este arquivo materializa (canonico §2, todas de 2026-07-17):
--   D-E: conversations.state nasce com os 7 valores (pending incluido). A F8 NAO faz ALTER.
--   D-G: outbox usa unique (account_id, idempotency_key) — NUNCA UNIQUE global: a chave vem
--        do cliente e um UNIQUE global deixaria a conta A suprimir o envio da conta B.
--
-- conversations.status NAO e coluna (canonico §7.3): `state` e a verdade e `status` e
-- projecao derivada na serializacao. Coluna + projecao = duas verdades (principio 1).
--
-- department_id/queue_id nascem SEM FK: messaging.departments/queues so existem na F8 —
-- declarar a FK aqui quebraria a migration. A F8 adiciona a constraint.
--
-- Multi-tenant dia 1: account_id uuid not null references core.accounts(id) em todas.
-- account_id vem SEMPRE do Principal, nunca do body; fora de escopo -> 404, nunca 403.
--
-- Idempotente, schema qualificado, SEM `-- +goose Down` (o migrator roda o arquivo
-- inteiro; um Down aqui se auto-destruiria — falha real em 0147_automation_contacts_fix.sql).
-- Ver 0197_tools_module.sql / 0187_finance_module.sql.

create schema if not exists messaging;

-- ============================================================================
-- Config do atendimento por conta (Prisma AtendimentoTenantConfig:48)
-- ============================================================================
-- Defaults 15/500 sao os do legado. A retencao POR CLASSE do canonico §10 e F13 —
-- nao confundir com retention_days daqui.
create table if not exists messaging.account_config (
    account_id     uuid primary key references core.accounts(id) on delete cascade,
    retention_days int not null default 15,
    max_upload_mb  int not null default 500,
    created_at     timestamptz not null default now(),
    updated_at     timestamptz not null default now()
);

-- ============================================================================
-- Instancias de WhatsApp (Prisma WhatsAppInstance:56)
-- ============================================================================
-- provider/provider_config/credentials_ciphertext sao NOVOS desta fase (canonico §7.2,
-- D-A multi-provider): nascem junto, nao viram ALTER depois.
-- credentials_ciphertext fica VAZIA aqui — quem cifra e platform/secretbox (F3), prefixo
-- 'v1:'. Gravar chave crua nela repete o gap do calendar/secrets.go.
create table if not exists messaging.whatsapp_instances (
    id                     uuid primary key default gen_random_uuid(),
    account_id             uuid not null references core.accounts(id) on delete cascade,
    instance_name          text not null,
    display_name           text,
    phone_number           text,
    queue_label            text,
    is_default             boolean not null default false,
    is_active              boolean not null default true,
    created_by_user_id     uuid references core.users(id) on delete set null,
    responsible_user_id    uuid references core.users(id) on delete set null,
    provider               text not null default 'evolution'
        check (provider in ('meta_whatsapp_cloud', 'evolution', 'waha', 'mock')),
    provider_config        jsonb not null default '{}'::jsonb,
    credentials_ciphertext text,
    created_at             timestamptz not null default now(),
    updated_at             timestamptz not null default now()
);

create unique index if not exists messaging_whatsapp_instances_account_name_uidx
    on messaging.whatsapp_instances (account_id, instance_name);
create index if not exists messaging_whatsapp_instances_account_active_default_idx
    on messaging.whatsapp_instances (account_id, is_active, is_default);
create index if not exists messaging_whatsapp_instances_account_responsible_idx
    on messaging.whatsapp_instances (account_id, responsible_user_id);

-- ============================================================================
-- Contatos (Prisma Contact:174)
-- ============================================================================
create table if not exists messaging.contacts (
    id         uuid primary key default gen_random_uuid(),
    account_id uuid not null references core.accounts(id) on delete cascade,
    name       text not null,
    phone      text not null,
    avatar_url text,
    source     text not null default 'MANUAL',
    created_at timestamptz not null default now(),
    updated_at timestamptz not null default now()
);

create unique index if not exists messaging_contacts_account_phone_uidx
    on messaging.contacts (account_id, phone);
create index if not exists messaging_contacts_account_name_idx
    on messaging.contacts (account_id, name);
create index if not exists messaging_contacts_account_created_idx
    on messaging.contacts (account_id, created_at desc);

-- ============================================================================
-- Conversas (Prisma Conversation:92)
-- ============================================================================
-- instance_scope_key = o instance_name (NAO o id) — e a chave real de particionamento.
--
-- state (canonico §7.2/§7.3, D-E): a VERDADE do ciclo de vida, com os 7 valores ja no
-- CHECK. `pending` e rotulo manual do operador (12o evento human.pending, escrito pela F8);
-- a F2 so garante que a coluna o aceita e o projeta como status=PENDING.
--
-- assigned_user_id (novo, -> core.users) COEXISTE com assigned_to_id (Prisma assignedToId,
-- texto, servido ao front com esse nome). Nao fundir aqui: o front verbatim le assignedToId;
-- quem reconcilia e a F7/F8 via maquina de estados.
create table if not exists messaging.conversations (
    id                 uuid primary key default gen_random_uuid(),
    account_id         uuid not null references core.accounts(id) on delete cascade,
    instance_id        uuid references messaging.whatsapp_instances(id) on delete set null,
    instance_scope_key text not null default 'default',
    assigned_to_id     text,
    contact_id         uuid references messaging.contacts(id) on delete set null,
    channel            text not null check (channel in ('WHATSAPP', 'INSTAGRAM')),
    external_id        text not null,
    contact_name       text,
    contact_avatar_url text,
    contact_phone      text,
    state              text not null default 'new'
        check (state in ('new', 'ai_active', 'routing', 'queued', 'human_active', 'pending', 'closed')),
    department_id      uuid,
    queue_id           uuid,
    assigned_user_id   uuid references core.users(id) on delete set null,
    extracted_fields   jsonb not null default '{}'::jsonb,
    last_message_at    timestamptz not null default now(),
    created_at         timestamptz not null default now(),
    updated_at         timestamptz not null default now()
);

create unique index if not exists messaging_conversations_dedupe_uidx
    on messaging.conversations (account_id, external_id, channel, instance_scope_key);
create index if not exists messaging_conversations_account_last_message_idx
    on messaging.conversations (account_id, last_message_at desc);
create index if not exists messaging_conversations_account_scope_last_message_idx
    on messaging.conversations (account_id, instance_scope_key, last_message_at desc);
create index if not exists messaging_conversations_assigned_to_idx
    on messaging.conversations (assigned_to_id);
create index if not exists messaging_conversations_contact_idx
    on messaging.conversations (contact_id);
create index if not exists messaging_conversations_instance_idx
    on messaging.conversations (instance_id);

-- ============================================================================
-- Mensagens (Prisma Message:121)
-- ============================================================================
-- message_status e so PENDING|SENT|FAILED: DELIVERED/READ NAO existem no legado
-- (nao ha tracking de ACK). Se um dia quisermos, e feature nova, nao port (canonico §15).
create table if not exists messaging.messages (
    id                     uuid primary key default gen_random_uuid(),
    account_id             uuid not null references core.accounts(id) on delete cascade,
    conversation_id        uuid not null references messaging.conversations(id) on delete cascade,
    instance_id            uuid references messaging.whatsapp_instances(id) on delete set null,
    instance_scope_key     text not null default 'default',
    sender_user_id         uuid references core.users(id) on delete set null,
    direction              text not null check (direction in ('INBOUND', 'OUTBOUND')),
    message_type           text not null default 'TEXT'
        check (message_type in ('TEXT', 'IMAGE', 'AUDIO', 'VIDEO', 'DOCUMENT')),
    sender_name            text,
    sender_avatar_url      text,
    content                text not null,
    media_url              text,
    media_mime_type        text,
    media_file_name        text,
    media_file_size_bytes  int,
    media_caption          text,
    media_duration_seconds int,
    metadata_json          jsonb,
    external_message_id    text,
    status                 text not null default 'PENDING'
        check (status in ('PENDING', 'SENT', 'FAILED')),
    created_at             timestamptz not null default now(),
    updated_at             timestamptz not null default now()
);

create index if not exists messaging_messages_account_created_idx
    on messaging.messages (account_id, created_at);
create index if not exists messaging_messages_account_scope_created_idx
    on messaging.messages (account_id, instance_scope_key, created_at);
create index if not exists messaging_messages_conversation_created_idx
    on messaging.messages (conversation_id, created_at);
create index if not exists messaging_messages_sender_idx
    on messaging.messages (sender_user_id);
create index if not exists messaging_messages_instance_idx
    on messaging.messages (instance_id);

-- ============================================================================
-- Stickers salvos (Prisma SavedSticker:77)
-- ============================================================================
-- Poda FIFO acima de 200/conta e regra de service (F12), nao do schema.
create table if not exists messaging.saved_stickers (
    id                 uuid primary key default gen_random_uuid(),
    account_id         uuid not null references core.accounts(id) on delete cascade,
    created_by_user_id uuid references core.users(id) on delete set null,
    name               text not null,
    data_url           text not null,
    mime_type          text not null,
    size_bytes         int not null,
    created_at         timestamptz not null default now(),
    updated_at         timestamptz not null default now()
);

create index if not exists messaging_saved_stickers_account_created_idx
    on messaging.saved_stickers (account_id, created_at desc);
create index if not exists messaging_saved_stickers_created_by_idx
    on messaging.saved_stickers (created_by_user_id);

-- ============================================================================
-- Trilha de auditoria (Prisma AuditEvent:156)
-- ============================================================================
create table if not exists messaging.audit_events (
    id              uuid primary key default gen_random_uuid(),
    account_id      uuid not null references core.accounts(id) on delete cascade,
    actor_user_id   uuid references core.users(id) on delete set null,
    conversation_id uuid references messaging.conversations(id) on delete set null,
    message_id      uuid references messaging.messages(id) on delete set null,
    event_type      text not null check (event_type in (
        'MESSAGE_OUTBOUND_QUEUED', 'MESSAGE_OUTBOUND_SENT', 'MESSAGE_OUTBOUND_FAILED',
        'CONVERSATION_STATUS_CHANGED', 'CONVERSATION_ASSIGNED')),
    payload_json    jsonb,
    created_at      timestamptz not null default now()
);

create index if not exists messaging_audit_events_account_created_idx
    on messaging.audit_events (account_id, created_at desc);
create index if not exists messaging_audit_events_conversation_created_idx
    on messaging.audit_events (conversation_id, created_at desc);
create index if not exists messaging_audit_events_message_created_idx
    on messaging.audit_events (message_id, created_at desc);
create index if not exists messaging_audit_events_type_created_idx
    on messaging.audit_events (event_type, created_at desc);

-- ============================================================================
-- "Apagar para mim" (Prisma HiddenMessageForUser:190)
-- ============================================================================
create table if not exists messaging.hidden_messages (
    id         uuid primary key default gen_random_uuid(),
    account_id uuid not null references core.accounts(id) on delete cascade,
    user_id    uuid not null references core.users(id) on delete cascade,
    message_id uuid not null references messaging.messages(id) on delete cascade,
    created_at timestamptz not null default now()
);

create unique index if not exists messaging_hidden_messages_user_message_uidx
    on messaging.hidden_messages (user_id, message_id);
create index if not exists messaging_hidden_messages_account_user_created_idx
    on messaging.hidden_messages (account_id, user_id, created_at desc);
create index if not exists messaging_hidden_messages_account_message_idx
    on messaging.hidden_messages (account_id, message_id);

-- ============================================================================
-- Outbox — envio duravel (a 9a tabela; NAO vem do port)
-- ============================================================================
-- Contrato coluna a coluna: docs/omnichannel/specs/OMNI-F3.md §F3.2 ("Contrato da tabela").
-- A TABELA e da F2; o ENGINE (claim FOR UPDATE SKIP LOCKED, retry classificado, worker,
-- dead-letter, monitor de presas) e da F3 em platform/jobs. O PRODUTOR de job e a F6.
--
-- ordering_key = FIFO. No omnichannel = conversation_id (canonico §12 risco 5).
-- idempotency_key: unique (account_id, idempotency_key) — POR CONTA (D-G). Como o UNIQUE
-- global saiu, prefixar a chave com o account_id deixou de fazer sentido: a unicidade ja
-- e por conta e a chave vai crua.
-- payload: sem PII crua. last_error: mascarado.
create table if not exists messaging.outbox (
    id              uuid primary key default gen_random_uuid(),
    account_id      uuid not null references core.accounts(id) on delete cascade,
    ordering_key    text not null,
    idempotency_key text not null,
    kind            text not null,
    payload         jsonb not null default '{}'::jsonb,
    status          text not null default 'pending'
        check (status in ('pending', 'processing', 'done', 'failed', 'dead')),
    attempts        int not null default 0,
    max_attempts    int not null default 3,
    run_after       timestamptz not null default now(),
    locked_at       timestamptz,
    locked_by       text not null default '',
    last_error      text not null default '',
    created_at      timestamptz not null default now(),
    updated_at      timestamptz not null default now()
);

create unique index if not exists messaging_outbox_account_idempotency_uidx
    on messaging.outbox (account_id, idempotency_key);
-- Claim head-of-line: so e elegivel o job mais antigo nao finalizado da chave (F3).
create index if not exists messaging_outbox_ordering_idx
    on messaging.outbox (account_id, ordering_key, created_at, id)
    where status in ('pending', 'processing');
create index if not exists messaging_outbox_status_run_after_idx
    on messaging.outbox (status, run_after);
