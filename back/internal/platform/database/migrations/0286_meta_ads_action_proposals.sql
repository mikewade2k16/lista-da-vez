-- Assistente 360 / Meta Ads: propostas de mutacao duraveis, confirmacao
-- idempotente e trilha append-only. A chamada externa nunca acontece dentro da
-- transacao: primeiro a proposta muda para `executing`; timeout/queda vira
-- `unknown` e exige reconciliacao, sem retry cego.

create schema if not exists meta_ads;

create unique index if not exists meta_ads_ad_accounts_account_id_id_uidx
    on meta_ads.ad_accounts (account_id, id);

create table if not exists meta_ads.action_policies (
    id uuid primary key default gen_random_uuid(),
    account_id uuid not null references core.accounts(id) on delete cascade,
    ad_account_id uuid not null,
    currency text not null check (currency ~ '^[A-Z]{3}$'),
    max_daily_budget numeric(15,2)
        check (max_daily_budget is null or max_daily_budget > 0),
    max_lifetime_budget numeric(15,2)
        check (max_lifetime_budget is null or max_lifetime_budget > 0),
    allow_create boolean not null default false,
    allow_duplicate boolean not null default false,
    allow_resume boolean not null default false,
    updated_by_user_id uuid references core.users(id) on delete set null,
    created_at timestamptz not null default now(),
    updated_at timestamptz not null default now(),
    constraint meta_ads_action_policies_account_ad_account_unique
        unique (account_id, ad_account_id),
    constraint meta_ads_action_policies_ad_account_fk
        foreign key (account_id, ad_account_id)
        references meta_ads.ad_accounts(account_id, id) on delete cascade
);

create index if not exists meta_ads_action_policies_account_idx
    on meta_ads.action_policies (account_id, updated_at desc);

create table if not exists meta_ads.action_proposals (
    id uuid primary key default gen_random_uuid(),
    -- account_id e o escopo autenticado que criou/visualiza a proposta. Em uma
    -- conexao compartilhada, resource_account_id pode ser a conta-agencia.
    account_id uuid not null references core.accounts(id) on delete cascade,
    resource_account_id uuid not null references core.accounts(id) on delete restrict,
    ad_account_id uuid not null,
    meta_ad_account_id text not null check (length(meta_ad_account_id) between 1 and 80),
    ad_account_name text not null default '' check (length(ad_account_name) <= 300),
    currency text not null check (currency ~ '^[A-Z]{3}$'),
    action text not null check (action in (
        'create_campaign', 'duplicate_campaign', 'update_campaign',
        'pause_campaign', 'resume_campaign'
    )),
    source text not null check (source in ('assistant', 'manual')),
    source_conversation_id uuid,
    source_message_id uuid,
    -- IDs interno/externo sao snapshots validados no service contra
    -- resource_account_id + ad_account_id. Nao ha FK para campaigns de
    -- proposito: disconnect apaga o cache, mas nao pode apagar a auditoria.
    target_campaign_id uuid,
    target_meta_campaign_id text not null default '' check (length(target_meta_campaign_id) <= 80),
    payload jsonb not null check (
        jsonb_typeof(payload) = 'object'
        and octet_length(payload::text) <= 32768
    ),
    summary text not null check (length(summary) between 1 and 1000),
    request_hash text not null check (request_hash ~ '^[0-9a-f]{64}$'),
    idempotency_key text not null check (length(idempotency_key) between 8 and 160),
    confirmation_idempotency_key text
        check (confirmation_idempotency_key is null or length(confirmation_idempotency_key) between 8 and 160),
    status text not null default 'pending'
        check (status in ('pending', 'executing', 'succeeded', 'failed', 'unknown')),
    attempt_count smallint not null default 0 check (attempt_count between 0 and 1),
    external_entity_id text not null default '' check (length(external_entity_id) <= 160),
    result_snapshot jsonb not null default '{}'::jsonb check (
        jsonb_typeof(result_snapshot) = 'object'
        and octet_length(result_snapshot::text) <= 32768
    ),
    error_code text not null default '' check (length(error_code) <= 100),
    error_message text not null default '' check (length(error_message) <= 500),
    created_by_user_id uuid references core.users(id) on delete set null,
    confirmed_by_user_id uuid references core.users(id) on delete set null,
    confirmed_at timestamptz,
    execution_started_at timestamptz,
    completed_at timestamptz,
    reconciled_at timestamptz,
    created_at timestamptz not null default now(),
    updated_at timestamptz not null default now(),
    constraint meta_ads_action_proposals_source_refs_ck check (
        (source = 'assistant' and source_conversation_id is not null and source_message_id is not null)
        or (source = 'manual' and source_conversation_id is null and source_message_id is null)
    ),
    constraint meta_ads_action_proposals_target_ck check (
        (action = 'create_campaign' and target_campaign_id is null and target_meta_campaign_id = '')
        or (action <> 'create_campaign' and target_campaign_id is not null and target_meta_campaign_id <> '')
    ),
    constraint meta_ads_action_proposals_account_idempotency_unique
        unique (account_id, idempotency_key)
);

create unique index if not exists meta_ads_action_proposals_account_id_id_uidx
    on meta_ads.action_proposals (account_id, id);
create unique index if not exists meta_ads_action_proposals_confirmation_uidx
    on meta_ads.action_proposals (account_id, confirmation_idempotency_key)
    where confirmation_idempotency_key is not null;
create index if not exists meta_ads_action_proposals_account_created_idx
    on meta_ads.action_proposals (account_id, created_at desc, id desc);
create index if not exists meta_ads_action_proposals_account_status_idx
    on meta_ads.action_proposals (account_id, status, updated_at desc);

create table if not exists meta_ads.action_proposal_events (
    id uuid primary key default gen_random_uuid(),
    account_id uuid not null references core.accounts(id) on delete cascade,
    proposal_id uuid not null,
    event_type text not null check (event_type in (
        'proposed', 'confirmed', 'succeeded', 'failed', 'unknown', 'reconciled'
    )),
    actor_user_id uuid references core.users(id) on delete set null,
    detail jsonb not null default '{}'::jsonb check (
        jsonb_typeof(detail) = 'object'
        and octet_length(detail::text) <= 8192
    ),
    occurred_at timestamptz not null default now(),
    -- Append-only durante toda a vida da account. O cascade existe somente
    -- para a remocao administrativa da account/proposta; nao ha endpoint de
    -- delete nem update para eventos.
    constraint meta_ads_action_proposal_events_proposal_fk
        foreign key (account_id, proposal_id)
        references meta_ads.action_proposals(account_id, id) on delete cascade
);

create index if not exists meta_ads_action_proposal_events_proposal_idx
    on meta_ads.action_proposal_events (account_id, proposal_id, occurred_at, id);
