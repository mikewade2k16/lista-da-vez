-- Omnichannel F4 — dedupe do webhook inbound + backstop "um numero, uma instancia".
-- Plano: docs/omnichannel/PLANO_ATENDIMENTO.md (§7.1/§9.2-F4). Spec: OMNI-F4 C4/C5/C6.
--
-- messaging.webhook_events e a IDEMPOTENCIA POR TABELA (o canonico vence o Redis do
-- legado): a linha de dedupe e a escrita de dominio caem na MESMA transacao (exactly-once
-- sem lock distribuido). UNIQUE (account_id, provider, external_event_id) — POR CONTA, nunca
-- global: o external_event_id embute {instanceName}:{msgId}, mas o nome da instancia so e
-- unico DENTRO da conta, entao um UNIQUE global deixaria o evento da conta B sumir como
-- "duplicado" do da conta A (colisao cross-tenant, principio 2). O account_id no indice fecha isso.
--
-- payload_masked = copia MASCARADA para triagem (telefone -> ultimos 4; corpo omitido).
-- NUNCA o body cru (canonico §10). Retencao/purge desta tabela e F13.
--
-- Idempotente, schema qualificado, SEM `-- +goose Down` (o migrator roda o arquivo
-- inteiro; um Down aqui se auto-destruiria — falha real em 0147). Ver 0200_messaging_schema.

create table if not exists messaging.webhook_events (
    id                uuid        primary key default gen_random_uuid(),
    account_id        uuid        not null references core.accounts(id) on delete cascade,
    provider          text        not null,
    external_event_id text        not null,
    event_kind        text        not null default 'unknown',
    instance_name     text,
    payload_masked    jsonb       not null default '{}'::jsonb,
    received_at       timestamptz not null default now()
);

create unique index if not exists messaging_webhook_events_provider_event_uidx
    on messaging.webhook_events (account_id, provider, external_event_id);

create index if not exists messaging_webhook_events_account_received_idx
    on messaging.webhook_events (account_id, received_at desc);

-- Backstop da C6: o mesmo numero nao fica em duas instancias da mesma conta. PARCIAL de
-- proposito — phone_number e nullable (so resolve depois de conectar) e em Postgres NULLs
-- nao colidem; sem o filtro o indice nao diria nada de util. A coluna phone_number ja
-- existe (F2, migration 0200); esta fase so acrescenta o indice.
create unique index if not exists messaging_whatsapp_instances_account_phone_uidx
    on messaging.whatsapp_instances (account_id, phone_number)
    where phone_number is not null;
