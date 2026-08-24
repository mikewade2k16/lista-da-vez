-- Meta Ads: snapshots autoritativos e guardas atomicos para o claim de uma
-- proposta. A migration 0286 permanece imutavel; propostas anteriores recebem
-- version=0 e nunca ficam elegiveis para Graph por inferencia/backfill.

alter table meta_ads.action_proposals
    add column if not exists guard_snapshot_version smallint not null default 0,
    add column if not exists guard_snapshot_hash text not null default '',
    add column if not exists connection_id_snapshot uuid,
    add column if not exists connection_revision_snapshot uuid,
    add column if not exists ad_account_client_account_id_snapshot uuid,
    add column if not exists ad_account_updated_at_snapshot timestamptz,
    add column if not exists ad_account_hash_snapshot text not null default '',
    add column if not exists policy_configured_snapshot boolean not null default false,
    add column if not exists policy_id_snapshot uuid,
    add column if not exists policy_updated_at_snapshot timestamptz,
    add column if not exists policy_hash_snapshot text not null default '',
    add column if not exists policy_currency_snapshot text not null default '',
    add column if not exists policy_max_daily_budget_snapshot numeric(15,2),
    add column if not exists policy_max_lifetime_budget_snapshot numeric(15,2),
    add column if not exists policy_allow_create_snapshot boolean not null default false,
    add column if not exists policy_allow_duplicate_snapshot boolean not null default false,
    add column if not exists policy_allow_resume_snapshot boolean not null default false,
    add column if not exists campaign_synced_at_snapshot timestamptz,
    add column if not exists campaign_hash_snapshot text not null default '',
    add column if not exists campaign_name_snapshot text not null default '',
    add column if not exists campaign_status_snapshot text not null default '',
    add column if not exists campaign_daily_budget_snapshot numeric(15,2),
    add column if not exists campaign_lifetime_budget_snapshot numeric(15,2),
    add column if not exists claimed_connection_id uuid,
    add column if not exists claimed_connection_revision uuid;

alter table meta_ads.action_proposals
    drop constraint if exists meta_ads_action_proposals_guard_snapshot_version_ck;
alter table meta_ads.action_proposals
    add constraint meta_ads_action_proposals_guard_snapshot_version_ck check (
        guard_snapshot_version between 0 and 1
    );

alter table meta_ads.action_proposals
    drop constraint if exists meta_ads_action_proposals_guard_snapshot_ck;
alter table meta_ads.action_proposals
    add constraint meta_ads_action_proposals_guard_snapshot_ck check (
        guard_snapshot_version = 0
        or (
            guard_snapshot_hash ~ '^[0-9a-f]{64}$'
            and connection_id_snapshot is not null
            and connection_revision_snapshot is not null
            and ad_account_updated_at_snapshot is not null
            and ad_account_hash_snapshot ~ '^[0-9a-f]{64}$'
            and policy_hash_snapshot ~ '^[0-9a-f]{64}$'
            and campaign_hash_snapshot ~ '^[0-9a-f]{64}$'
            and (
                (policy_configured_snapshot
                    and policy_id_snapshot is not null
                    and policy_updated_at_snapshot is not null
                    and policy_currency_snapshot ~ '^[A-Z]{3}$')
                or (not policy_configured_snapshot
                    and policy_id_snapshot is null
                    and policy_updated_at_snapshot is null
                    and policy_currency_snapshot = ''
                    and policy_max_daily_budget_snapshot is null
                    and policy_max_lifetime_budget_snapshot is null
                    and not policy_allow_create_snapshot
                    and not policy_allow_duplicate_snapshot
                    and not policy_allow_resume_snapshot)
            )
            and (
                (target_campaign_id is null
                    and campaign_synced_at_snapshot is null
                    and campaign_name_snapshot = ''
                    and campaign_status_snapshot = ''
                    and campaign_daily_budget_snapshot is null
                    and campaign_lifetime_budget_snapshot is null)
                or (target_campaign_id is not null
                    and campaign_synced_at_snapshot is not null)
            )
        )
    );

alter table meta_ads.action_proposals
    drop constraint if exists meta_ads_action_proposals_claimed_connection_ck;
alter table meta_ads.action_proposals
    add constraint meta_ads_action_proposals_claimed_connection_ck check (
        (claimed_connection_id is null and claimed_connection_revision is null)
        or (guard_snapshot_version = 1
            and claimed_connection_id is not null
            and claimed_connection_revision is not null
			and claimed_connection_id = connection_id_snapshot
			and claimed_connection_revision = connection_revision_snapshot
            and attempt_count = 1)
    );

-- Nesta fase o conversor canônico do executor usa duas casas/minor units. Um
-- budget novo so e elegivel quando a moeda snapshot da ad account e BRL.
alter table meta_ads.action_proposals
    drop constraint if exists meta_ads_action_proposals_budget_currency_ck;
alter table meta_ads.action_proposals
    add constraint meta_ads_action_proposals_budget_currency_ck check (
        guard_snapshot_version = 0
        or action <> 'update_campaign'
        or not (payload ? 'budget')
		or (currency = 'BRL'
			and policy_configured_snapshot
			and policy_currency_snapshot = 'BRL')
    );

create index if not exists meta_ads_action_proposals_claimed_revision_idx
    on meta_ads.action_proposals (
        resource_account_id, claimed_connection_id, claimed_connection_revision, updated_at desc
    )
    where claimed_connection_revision is not null;

comment on column meta_ads.action_proposals.guard_snapshot_version is
    '0 = legado fail-closed; 1 = snapshot autoritativo completo capturado pelo Store';
comment on column meta_ads.action_proposals.guard_snapshot_hash is
    'SHA-256 canonico de connection/ad account/mapping/policy/campaign + request hash';
comment on column meta_ads.action_proposals.claimed_connection_revision is
    'Revision do token persistida atomicamente no claim; o executor exige exatamente esta revision';
