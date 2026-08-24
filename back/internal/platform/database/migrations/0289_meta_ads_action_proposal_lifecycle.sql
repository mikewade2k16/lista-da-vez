-- Assistente 360 / Meta Ads: vinculo autoritativo do card, cancelamento e
-- expiracao terminal. Esta migration e separada da 0286 porque a base pode ja
-- ter aplicado o contrato inicial durante o desenvolvimento.

alter table meta_ads.action_proposals
    add column if not exists source_bound boolean;

-- Propostas manuais nao dependem de card. Propostas assistant preexistentes
-- ficam deliberadamente sem bind (fail closed), pois nao e seguro inferir que o
-- card correspondente foi persistido apenas pelos IDs armazenados.
update meta_ads.action_proposals
set source_bound = (source = 'manual')
where source_bound is null;

alter table meta_ads.action_proposals
    alter column source_bound set default false,
    alter column source_bound set not null;

alter table meta_ads.action_proposals
    add column if not exists cancellation_idempotency_key text;

alter table meta_ads.action_proposals
    drop constraint if exists meta_ads_action_proposals_cancellation_key_ck;
alter table meta_ads.action_proposals
    add constraint meta_ads_action_proposals_cancellation_key_ck check (
        cancellation_idempotency_key is null
        or length(cancellation_idempotency_key) between 8 and 160
    );

alter table meta_ads.action_proposals
    add column if not exists expires_at timestamptz;

update meta_ads.action_proposals
set expires_at = created_at + interval '30 minutes'
where expires_at is null;

alter table meta_ads.action_proposals
    alter column expires_at set default (now() + interval '30 minutes'),
    alter column expires_at set not null;

alter table meta_ads.action_proposals
    drop constraint if exists action_proposals_status_check;
alter table meta_ads.action_proposals
    add constraint action_proposals_status_check check (status in (
        'pending', 'executing', 'succeeded', 'failed', 'unknown',
        'cancelled', 'expired'
    ));

alter table meta_ads.action_proposals
    drop constraint if exists meta_ads_action_proposals_source_refs_ck;
alter table meta_ads.action_proposals
    add constraint meta_ads_action_proposals_source_refs_ck check (
        (source = 'assistant'
            and source_conversation_id is not null
            and source_message_id is not null)
        or (source = 'manual'
            and source_conversation_id is null
            and source_message_id is null
            and source_bound)
    );

create unique index if not exists meta_ads_action_proposals_cancellation_uidx
    on meta_ads.action_proposals (account_id, cancellation_idempotency_key)
    where cancellation_idempotency_key is not null;

create index if not exists meta_ads_action_proposals_pending_expiry_idx
    on meta_ads.action_proposals (expires_at, account_id, id)
    where status = 'pending';

alter table meta_ads.action_proposal_events
    drop constraint if exists action_proposal_events_event_type_check;
alter table meta_ads.action_proposal_events
    add constraint action_proposal_events_event_type_check check (event_type in (
        'proposed', 'bound', 'confirmed', 'cancelled', 'expired',
        'succeeded', 'failed', 'unknown', 'reconciled'
    ));
