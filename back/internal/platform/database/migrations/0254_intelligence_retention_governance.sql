-- Customer Intelligence retention governance.
--
-- New policies must start as drafts and can become published only through an
-- explicit, revision-checked approval transition. Active legal holds preserve
-- observation payloads even after their retention expiry.

alter table intelligence.retention_policy_versions
    add column if not exists publication_reason_code text not null default '';
alter table intelligence.retention_policy_versions
    add column if not exists approval_reference text not null default '';

alter table intelligence.retention_policy_versions
    drop constraint if exists intelligence_retention_publication_reason_check;
alter table intelligence.retention_policy_versions
    add constraint intelligence_retention_publication_reason_check
    check (
        length(publication_reason_code) <= 80
        and (
            publication_reason_code = ''
            or publication_reason_code ~ '^[a-z][a-z0-9._-]{0,79}$'
        )
    );

alter table intelligence.retention_policy_versions
    drop constraint if exists intelligence_retention_approval_reference_check;
alter table intelligence.retention_policy_versions
    add constraint intelligence_retention_approval_reference_check
    check (
        length(approval_reference) <= 128
        and (
            approval_reference = ''
            or approval_reference ~ '^[A-Za-z0-9][A-Za-z0-9._:/-]{0,127}$'
        )
    );

create or replace function intelligence.enforce_retention_policy_publication_approval()
returns trigger
language plpgsql
as $$
begin
    if tg_op = 'INSERT' then
        if new.status = 'published' then
            raise check_violation
                using constraint = 'intelligence_retention_policy_draft_first',
                      message = 'retention policy must be created as draft';
        end if;
        return new;
    end if;

    if new.status = 'published' and old.status is distinct from 'published' then
        if old.status <> 'draft'
           or new.revision <> old.revision + 1
           or new.published_by_user_id is null
           or new.published_at is null
           or new.publication_reason_code = ''
           or new.approval_reference = '' then
            raise check_violation
                using constraint = 'intelligence_retention_policy_publication_approval',
                      message = 'retention policy publication requires draft, revision and approval metadata';
        end if;
    end if;
    return new;
end;
$$;

drop trigger if exists enforce_retention_policy_publication_approval
    on intelligence.retention_policy_versions;
create trigger enforce_retention_policy_publication_approval
before insert or update of status, revision, published_by_user_id, published_at,
    publication_reason_code, approval_reference
on intelligence.retention_policy_versions
for each row
execute function intelligence.enforce_retention_policy_publication_approval();

create unique index if not exists intelligence_observations_scope_id_uidx
    on intelligence.source_observations (account_id, client_account_id, id);

create table if not exists intelligence.observation_legal_holds (
    id uuid primary key default gen_random_uuid(),
    account_id uuid not null,
    client_account_id uuid not null,
    observation_id uuid not null,
    status text not null default 'active'
        check (status in ('active', 'released')),
    reason_code text not null
        check (
            length(reason_code) between 1 and 80
            and reason_code ~ '^[a-z][a-z0-9._-]{0,79}$'
        ),
    hold_reference text not null
        check (
            length(hold_reference) between 1 and 128
            and hold_reference ~ '^[A-Za-z0-9][A-Za-z0-9._:/-]{0,127}$'
        ),
    created_by_user_id uuid not null
        references core.users(id) on delete restrict,
    released_by_user_id uuid
        references core.users(id) on delete restrict,
    created_at timestamptz not null default now(),
    released_at timestamptz,
    unique (account_id, client_account_id, id),
    constraint intelligence_observation_legal_hold_scope_fk
        foreign key (account_id, client_account_id, observation_id)
        references intelligence.source_observations(account_id, client_account_id, id)
        on delete cascade,
    constraint intelligence_observation_legal_hold_lifecycle_check
        check (
            (
                status = 'active'
                and released_by_user_id is null
                and released_at is null
            )
            or (
                status = 'released'
                and released_by_user_id is not null
                and released_at is not null
            )
        )
);

create unique index if not exists intelligence_observation_active_legal_hold_uidx
    on intelligence.observation_legal_holds (
        account_id,
        client_account_id,
        observation_id
    )
    where status = 'active';

create index if not exists intelligence_observation_legal_hold_active_idx
    on intelligence.observation_legal_holds (
        account_id,
        client_account_id,
        observation_id,
        created_at
    )
    where status = 'active';

create or replace function intelligence.guard_observation_legal_hold_mutation()
returns trigger
language plpgsql
as $$
begin
    if tg_op = 'DELETE' then
        if old.status = 'active' then
            raise check_violation
                using constraint = 'intelligence_observation_active_legal_hold_immutable',
                      message = 'active legal hold cannot be deleted';
        end if;
        return old;
    end if;

    if tg_op = 'INSERT' then
        if new.status <> 'active'
           or new.released_by_user_id is not null
           or new.released_at is not null then
            raise check_violation
                using constraint = 'intelligence_observation_legal_hold_active_first',
                      message = 'legal hold must be created active';
        end if;

        -- Serialize legal-hold creation with the retention worker, which also
        -- locks this observation before clearing its payload. If retention won
        -- the lock first, the hold is rejected instead of recording a promise
        -- that arrived after the payload was already removed.
        perform 1
        from intelligence.source_observations observation
        where observation.account_id = new.account_id
          and observation.client_account_id = new.client_account_id
          and observation.id = new.observation_id
          and observation.retention_state = 'active'
        for update;
        if not found then
            raise check_violation
                using constraint = 'intelligence_observation_legal_hold_active_observation',
                      message = 'legal hold requires an active observation';
        end if;
        return new;
    end if;

    if old.account_id is distinct from new.account_id
       or old.client_account_id is distinct from new.client_account_id
       or old.observation_id is distinct from new.observation_id
       or old.reason_code is distinct from new.reason_code
       or old.hold_reference is distinct from new.hold_reference
       or old.created_by_user_id is distinct from new.created_by_user_id
       or old.created_at is distinct from new.created_at
       or old.status <> 'active'
       or new.status <> 'released'
       or new.released_by_user_id is null
       or new.released_at is null then
        raise check_violation
            using constraint = 'intelligence_observation_legal_hold_immutable',
                  message = 'legal hold only supports an audited active to released transition';
    end if;
    return new;
end;
$$;

drop trigger if exists guard_observation_legal_hold_mutation
    on intelligence.observation_legal_holds;
create trigger guard_observation_legal_hold_mutation
before insert or update or delete
on intelligence.observation_legal_holds
for each row
execute function intelligence.guard_observation_legal_hold_mutation();

create or replace function intelligence.audit_observation_legal_hold_mutation()
returns trigger
language plpgsql
as $$
declare
    event_name text;
    actor_id uuid;
begin
    if tg_op = 'INSERT' then
        event_name := 'source.observation_legal_hold_created';
        actor_id := new.created_by_user_id;
    else
        event_name := 'source.observation_legal_hold_released';
        actor_id := new.released_by_user_id;
    end if;

    insert into intelligence.audit_events (
        account_id,
        client_account_id,
        actor_user_id,
        event_type,
        aggregate_type,
        aggregate_id,
        reason_code,
        metadata
    )
    values (
        new.account_id,
        new.client_account_id,
        actor_id,
        event_name,
        'observation_legal_hold',
        new.id::text,
        new.reason_code,
        jsonb_build_object(
            'observationId', new.observation_id,
            'holdReference', new.hold_reference,
            'status', new.status
        )
    );
    return new;
end;
$$;

drop trigger if exists audit_observation_legal_hold_mutation
    on intelligence.observation_legal_holds;
create trigger audit_observation_legal_hold_mutation
after insert or update of status
on intelligence.observation_legal_holds
for each row
execute function intelligence.audit_observation_legal_hold_mutation();

create or replace function intelligence.prevent_held_observation_retention()
returns trigger
language plpgsql
as $$
begin
    if old.retention_state = 'active'
       and new.retention_state in ('tombstoned', 'crypto_shredded')
       and exists (
           select 1
           from intelligence.observation_legal_holds legal_hold
           where legal_hold.account_id = old.account_id
             and legal_hold.client_account_id = old.client_account_id
             and legal_hold.observation_id = old.id
             and legal_hold.status = 'active'
       ) then
        raise check_violation
            using constraint = 'intelligence_observation_active_legal_hold',
                  message = 'active legal hold blocks observation retention';
    end if;
    return new;
end;
$$;

drop trigger if exists prevent_held_observation_retention
    on intelligence.source_observations;
create trigger prevent_held_observation_retention
before update of retention_state, snapshot_json, snapshot_ciphertext
on intelligence.source_observations
for each row
execute function intelligence.prevent_held_observation_retention();
