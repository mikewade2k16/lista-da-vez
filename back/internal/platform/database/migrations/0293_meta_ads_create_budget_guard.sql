-- Mantem a barreira de minor units da 0290 tambem para create_campaign.
-- A 0290 ja pode ter sido aplicada; por isso a evolucao e aditiva e recebe
-- uma nova migration, sem reescrever historico.

alter table meta_ads.action_proposals
    drop constraint if exists meta_ads_action_proposals_budget_currency_ck;
alter table meta_ads.action_proposals
    add constraint meta_ads_action_proposals_budget_currency_ck check (
        guard_snapshot_version = 0
        or action not in ('create_campaign', 'update_campaign')
        or not (payload ? 'budget')
        or (currency = 'BRL'
            and policy_configured_snapshot
            and policy_currency_snapshot = 'BRL')
    );

comment on constraint meta_ads_action_proposals_budget_currency_ck
    on meta_ads.action_proposals is
    'Create/update com budget so executam em BRL com policy snapshot autoritativa.';
