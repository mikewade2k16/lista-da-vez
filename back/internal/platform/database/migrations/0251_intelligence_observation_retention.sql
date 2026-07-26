-- Customer Intelligence: closed, versioned retention for source observations.
--
-- Retention never deletes an observation row because facts/claims may still
-- reference it as provenance. Expiry removes the snapshot in a bounded worker,
-- leaves a tombstone and writes metadata-only audit.

alter table intelligence.retention_policy_versions
    add column if not exists snapshot_ttl_seconds integer not null default 7776000;
alter table intelligence.retention_policy_versions
    add column if not exists on_expiry text not null default 'tombstone';
alter table intelligence.retention_policy_versions
    add column if not exists legal_hold_behavior text not null default 'preserve';
alter table intelligence.retention_policy_versions
    add column if not exists block_reingestion boolean not null default true;

alter table intelligence.retention_policy_versions
    drop constraint if exists intelligence_retention_snapshot_ttl_check;
alter table intelligence.retention_policy_versions
    add constraint intelligence_retention_snapshot_ttl_check
    check (snapshot_ttl_seconds between 86400 and 315360000);

alter table intelligence.retention_policy_versions
    drop constraint if exists intelligence_retention_on_expiry_check;
alter table intelligence.retention_policy_versions
    add constraint intelligence_retention_on_expiry_check
    check (on_expiry in ('tombstone', 'crypto_shred'));

alter table intelligence.retention_policy_versions
    drop constraint if exists intelligence_retention_legal_hold_check;
alter table intelligence.retention_policy_versions
    add constraint intelligence_retention_legal_hold_check
    check (legal_hold_behavior = 'preserve');

-- Every configured source receives a published policy. Existing non-empty
-- policy keys are preserved; legacy empty bindings receive the conservative
-- 90-day default.
insert into intelligence.retention_policy_versions (
    account_id,
    policy_key,
    version,
    status,
    category_rules,
    snapshot_ttl_seconds,
    on_expiry,
    legal_hold_behavior,
    block_reingestion,
    published_at
)
select
    needed.account_id,
    needed.policy_key,
    coalesce((
        select max(existing.version) + 1
        from intelligence.retention_policy_versions existing
        where existing.account_id = needed.account_id
          and existing.policy_key = needed.policy_key
    ), 1),
    'published',
    jsonb_build_object(
        'snapshotTtlSeconds', 7776000,
        'onExpiry', 'tombstone'
    ),
    7776000,
    'tombstone',
    'preserve',
    true,
    now()
from (
    select distinct
        source.account_id,
        coalesce(
            nullif(btrim(source.retention_policy_key), ''),
            'customer_profile.default'
        ) as policy_key
    from intelligence.source_configs source
) needed
where not exists (
    select 1
    from intelligence.retention_policy_versions published
    where published.account_id = needed.account_id
      and published.policy_key = needed.policy_key
      and published.status = 'published'
)
on conflict (account_id, policy_key, version) do nothing;

-- If a legacy row already referenced a policy version, its canonical key wins.
update intelligence.source_configs source
set retention_policy_key = policy.policy_key
from intelligence.retention_policy_versions policy
where source.account_id = policy.account_id
  and source.retention_policy_version_id = policy.id
  and btrim(source.retention_policy_key) = '';

update intelligence.source_configs source
set
    retention_policy_key = coalesce(
        nullif(btrim(source.retention_policy_key), ''),
        'customer_profile.default'
    ),
    retention_policy_version_id = (
        select policy.id
        from intelligence.retention_policy_versions policy
        where policy.account_id = source.account_id
          and policy.policy_key = coalesce(
              nullif(btrim(source.retention_policy_key), ''),
              'customer_profile.default'
          )
          and policy.status = 'published'
        order by policy.version desc, policy.id desc
        limit 1
    )
where source.retention_policy_version_id is null
   or btrim(source.retention_policy_key) = '';

create unique index if not exists intelligence_retention_policy_binding_uidx
    on intelligence.retention_policy_versions (account_id, policy_key, id);

alter table intelligence.source_configs
    alter column retention_policy_key set default 'customer_profile.default';
alter table intelligence.source_configs
    alter column retention_policy_version_id set not null;
alter table intelligence.source_configs
    drop constraint if exists intelligence_source_retention_policy_key_check;
alter table intelligence.source_configs
    add constraint intelligence_source_retention_policy_key_check
    check (length(btrim(retention_policy_key)) between 1 and 160);
alter table intelligence.source_configs
    drop constraint if exists intelligence_source_retention_policy_binding_fk;
alter table intelligence.source_configs
    add constraint intelligence_source_retention_policy_binding_fk
    foreign key (account_id, retention_policy_key, retention_policy_version_id)
    references intelligence.retention_policy_versions(account_id, policy_key, id)
    on delete restrict;

-- Runs freeze the policy that was bound when the run started.
update intelligence.source_ingestion_runs run
set retention_policy_version_id = source.retention_policy_version_id
from intelligence.source_configs source
where run.account_id = source.account_id
  and run.client_account_id = source.client_account_id
  and run.source_config_id = source.id
  and run.retention_policy_version_id is null;

alter table intelligence.source_ingestion_runs
    alter column retention_policy_version_id set not null;

alter table intelligence.source_observations
    add column if not exists retention_state text not null default 'active';
alter table intelligence.source_observations
    add column if not exists retention_applied_at timestamptz;
alter table intelligence.source_observations
    add column if not exists retention_reason_code text not null default '';

-- Observations freeze the run policy. Rows created before a run was persisted
-- fall back to the source binding, but never to an unbounded/null policy.
update intelligence.source_observations observation
set retention_policy_version_id = coalesce(
    observation.retention_policy_version_id,
    (
        select run.retention_policy_version_id
        from intelligence.source_ingestion_runs run
        where run.account_id = observation.account_id
          and run.client_account_id = observation.client_account_id
          and run.id = observation.ingestion_run_id
    ),
    source.retention_policy_version_id
)
from intelligence.source_configs source
where observation.account_id = source.account_id
  and observation.client_account_id = source.client_account_id
  and observation.source_config_id = source.id
  and observation.retention_policy_version_id is null;

update intelligence.source_observations observation
set expires_at = least(
    coalesce(
        observation.expires_at,
        observation.observed_at
            + make_interval(secs => policy.snapshot_ttl_seconds)
    ),
    observation.observed_at
        + make_interval(secs => policy.snapshot_ttl_seconds)
)
from intelligence.retention_policy_versions policy
where observation.account_id = policy.account_id
  and observation.retention_policy_version_id = policy.id;

alter table intelligence.source_observations
    alter column retention_policy_version_id set not null;
alter table intelligence.source_observations
    alter column expires_at set not null;

alter table intelligence.source_observations
    drop constraint if exists intelligence_observations_payload_ck;
alter table intelligence.source_observations
    add constraint intelligence_observations_payload_ck
    check (
        (
            retention_state = 'active'
            and (
                (snapshot_json is not null and snapshot_ciphertext is null)
                or (snapshot_json is null and snapshot_ciphertext is not null)
            )
            and retention_applied_at is null
            and retention_reason_code = ''
        )
        or (
            retention_state in ('tombstoned', 'crypto_shredded')
            and snapshot_json is null
            and snapshot_ciphertext is null
            and retention_applied_at is not null
            and retention_reason_code <> ''
        )
    );

alter table intelligence.source_observations
    drop constraint if exists intelligence_observations_retention_state_check;
alter table intelligence.source_observations
    add constraint intelligence_observations_retention_state_check
    check (retention_state in ('active', 'tombstoned', 'crypto_shredded'));

create index if not exists intelligence_observations_retention_claim_idx
    on intelligence.source_observations (
        account_id,
        client_account_id,
        expires_at,
        id
    )
    where retention_state = 'active';

-- A source can bind only a published policy. This is a database invariant;
-- prompt content and adapters cannot bypass it.
create or replace function intelligence.enforce_published_source_retention_policy()
returns trigger
language plpgsql
as $$
begin
    if not exists (
        select 1
        from intelligence.retention_policy_versions policy
        where policy.account_id = new.account_id
          and policy.id = new.retention_policy_version_id
          and policy.policy_key = new.retention_policy_key
          and policy.status = 'published'
    ) then
        raise check_violation
            using constraint = 'intelligence_source_retention_policy_published_check',
                  message = 'source retention policy must be published';
    end if;
    return new;
end;
$$;

drop trigger if exists enforce_published_source_retention_policy
    on intelligence.source_configs;
create trigger enforce_published_source_retention_policy
before insert or update of retention_policy_key, retention_policy_version_id
on intelligence.source_configs
for each row
execute function intelligence.enforce_published_source_retention_policy();

-- Published policy content is immutable. A changed policy is a new version.
create or replace function intelligence.prevent_published_retention_policy_mutation()
returns trigger
language plpgsql
as $$
begin
    if old.status = 'published' then
        raise check_violation
            using constraint = 'intelligence_published_retention_policy_immutable',
                  message = 'published retention policy is immutable';
    end if;
    return new;
end;
$$;

drop trigger if exists prevent_published_retention_policy_mutation
    on intelligence.retention_policy_versions;
create trigger prevent_published_retention_policy_mutation
before update on intelligence.retention_policy_versions
for each row
execute function intelligence.prevent_published_retention_policy_mutation();
