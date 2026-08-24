-- Promocao segura de um post real do Instagram. A arvore Campaign -> AdSet ->
-- Creative -> Ad nasce pausada e cada POST externo recebe um recibo local
-- at-most-once. Uma etapa executing/unknown nunca e repetida automaticamente.

alter table meta_ads.action_proposals
    drop constraint if exists action_proposals_action_check;
alter table meta_ads.action_proposals
    add constraint action_proposals_action_check check (action in (
        'create_campaign', 'duplicate_campaign', 'update_campaign',
        'pause_campaign', 'resume_campaign', 'promote_instagram_post'
    ));

alter table meta_ads.action_proposals
    drop constraint if exists meta_ads_action_proposals_target_ck;
alter table meta_ads.action_proposals
    add constraint meta_ads_action_proposals_target_ck check (
        (action in ('create_campaign', 'promote_instagram_post')
            and target_campaign_id is null and target_meta_campaign_id = '')
        or (action not in ('create_campaign', 'promote_instagram_post')
            and target_campaign_id is not null and target_meta_campaign_id <> '')
    );

alter table meta_ads.action_proposals
    drop constraint if exists meta_ads_action_proposals_budget_currency_ck;
alter table meta_ads.action_proposals
    add constraint meta_ads_action_proposals_budget_currency_ck check (
        guard_snapshot_version = 0
        or action not in ('create_campaign', 'update_campaign', 'promote_instagram_post')
        or not (payload ? 'budget')
        or (currency = 'BRL'
            and policy_configured_snapshot
            and policy_currency_snapshot = 'BRL')
    );

create table if not exists meta_ads.action_proposal_steps (
    id uuid primary key default gen_random_uuid(),
    account_id uuid not null references core.accounts(id) on delete cascade,
    proposal_id uuid not null,
    step text not null check (step in ('campaign', 'ad_set', 'creative', 'ad')),
    request_hash text not null check (request_hash ~ '^[0-9a-f]{64}$'),
    status text not null check (status in ('executing', 'succeeded', 'failed', 'unknown')),
    external_entity_id text not null default '' check (length(external_entity_id) <= 160),
    result_snapshot jsonb not null default '{}'::jsonb check (
        jsonb_typeof(result_snapshot) = 'object'
        and octet_length(result_snapshot::text) <= 16384
    ),
    error_code text not null default '' check (length(error_code) <= 100),
    error_message text not null default '' check (length(error_message) <= 500),
    started_at timestamptz not null default now(),
    completed_at timestamptz,
    updated_at timestamptz not null default now(),
    constraint meta_ads_action_proposal_steps_proposal_fk
        foreign key (account_id, proposal_id)
        references meta_ads.action_proposals(account_id, id) on delete cascade,
    constraint meta_ads_action_proposal_steps_unique unique (account_id, proposal_id, step),
    constraint meta_ads_action_proposal_steps_result_ck check (
        (status = 'executing' and external_entity_id = '' and completed_at is null)
        or (status = 'succeeded' and external_entity_id <> '' and completed_at is not null)
        or (status in ('failed', 'unknown') and completed_at is not null)
    )
);

create index if not exists meta_ads_action_proposal_steps_proposal_idx
    on meta_ads.action_proposal_steps (account_id, proposal_id, started_at, id);

comment on table meta_ads.action_proposal_steps is
    'Recibos at-most-once por POST Graph da arvore de anuncio; executing/unknown bloqueiam retry cego.';
