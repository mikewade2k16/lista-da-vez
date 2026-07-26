-- Customer Intelligence context snapshot retention.
--
-- Runtime/result rows keep their context_snapshot_id for reproducibility, so
-- expiry crypto-shreds the payload in place instead of deleting the snapshot.
-- Active legal holds serialize with the worker and preserve the ciphertext.

alter table intelligence.context_snapshots
    alter column payload_ciphertext drop not null;
alter table intelligence.context_snapshots
    add column if not exists retention_state text not null default 'active';
alter table intelligence.context_snapshots
    add column if not exists tombstoned_at timestamptz;
alter table intelligence.context_snapshots
    add column if not exists retention_reason_code text not null default '';

alter table intelligence.context_snapshots
    drop constraint if exists intelligence_context_snapshots_retention_state_check;
alter table intelligence.context_snapshots
    add constraint intelligence_context_snapshots_retention_state_check
    check (retention_state in ('active', 'crypto_shredded'));

alter table intelligence.context_snapshots
    drop constraint if exists intelligence_context_snapshots_retention_payload_check;
alter table intelligence.context_snapshots
    add constraint intelligence_context_snapshots_retention_payload_check
    check (
        (
            retention_state = 'active'
            and payload_ciphertext is not null
            and cipher_key_version <> ''
            and payload_hash <> ''
            and tombstoned_at is null
            and retention_reason_code = ''
        )
        or (
            retention_state = 'crypto_shredded'
            and payload_ciphertext is null
            and cipher_key_version = ''
            and payload_hash = ''
            and tombstoned_at is not null
            and retention_reason_code <> ''
        )
    );

create unique index if not exists intelligence_context_snapshots_scope_id_uidx
    on intelligence.context_snapshots (account_id, client_account_id, id);

create index if not exists intelligence_context_snapshots_retention_claim_idx
    on intelligence.context_snapshots (
        account_id,
        client_account_id,
        expires_at,
        id
    )
    where retention_state = 'active';

create table if not exists intelligence.context_snapshot_legal_holds (
    id uuid primary key default gen_random_uuid(),
    account_id uuid not null,
    client_account_id uuid not null,
    context_snapshot_id uuid not null,
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
    constraint intelligence_context_snapshot_legal_hold_scope_fk
        foreign key (account_id, client_account_id, context_snapshot_id)
        references intelligence.context_snapshots(account_id, client_account_id, id)
        on delete cascade,
    constraint intelligence_context_snapshot_legal_hold_lifecycle_check
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

create unique index if not exists intelligence_context_snapshot_active_hold_uidx
    on intelligence.context_snapshot_legal_holds (
        account_id,
        client_account_id,
        context_snapshot_id
    )
    where status = 'active';

create index if not exists intelligence_context_snapshot_legal_hold_active_idx
    on intelligence.context_snapshot_legal_holds (
        account_id,
        client_account_id,
        context_snapshot_id,
        created_at
    )
    where status = 'active';

create or replace function intelligence.guard_context_snapshot_legal_hold_mutation()
returns trigger
language plpgsql
as $$
declare
    lock_account_id uuid;
    lock_client_account_id uuid;
begin
    if tg_op = 'DELETE' then
        lock_account_id := old.account_id;
        lock_client_account_id := old.client_account_id;
    else
        lock_account_id := new.account_id;
        lock_client_account_id := new.client_account_id;
    end if;

    perform pg_advisory_xact_lock(hashtextextended(
        'customer_intelligence:context_retention:'
            || lock_account_id::text
            || ':'
            || lock_client_account_id::text,
        0
    ));

    if tg_op = 'DELETE' then
        if old.status = 'active' then
            raise check_violation
                using constraint = 'intelligence_context_snapshot_active_hold_immutable',
                      message = 'active context snapshot legal hold cannot be deleted';
        end if;
        return old;
    end if;

    if tg_op = 'INSERT' then
        if new.status <> 'active'
           or new.released_by_user_id is not null
           or new.released_at is not null then
            raise check_violation
                using constraint = 'intelligence_context_snapshot_hold_active_first',
                      message = 'context snapshot legal hold must be created active';
        end if;

        perform 1
        from intelligence.context_snapshots snapshot
        where snapshot.account_id = new.account_id
          and snapshot.client_account_id = new.client_account_id
          and snapshot.id = new.context_snapshot_id
          and snapshot.retention_state = 'active'
        for update;
        if not found then
            raise check_violation
                using constraint = 'intelligence_context_snapshot_hold_active_snapshot',
                      message = 'legal hold requires an active context snapshot';
        end if;
        return new;
    end if;

    if old.account_id is distinct from new.account_id
       or old.client_account_id is distinct from new.client_account_id
       or old.context_snapshot_id is distinct from new.context_snapshot_id
       or old.reason_code is distinct from new.reason_code
       or old.hold_reference is distinct from new.hold_reference
       or old.created_by_user_id is distinct from new.created_by_user_id
       or old.created_at is distinct from new.created_at
       or old.status <> 'active'
       or new.status <> 'released'
       or new.released_by_user_id is null
       or new.released_at is null then
        raise check_violation
            using constraint = 'intelligence_context_snapshot_legal_hold_immutable',
                  message = 'legal hold only supports an audited active to released transition';
    end if;
    return new;
end;
$$;

drop trigger if exists guard_context_snapshot_legal_hold_mutation
    on intelligence.context_snapshot_legal_holds;
create trigger guard_context_snapshot_legal_hold_mutation
before insert or update or delete
on intelligence.context_snapshot_legal_holds
for each row
execute function intelligence.guard_context_snapshot_legal_hold_mutation();

create or replace function intelligence.audit_context_snapshot_legal_hold_mutation()
returns trigger
language plpgsql
as $$
declare
    event_name text;
    actor_id uuid;
begin
    if tg_op = 'INSERT' then
        event_name := 'context.snapshot_legal_hold_created';
        actor_id := new.created_by_user_id;
    else
        event_name := 'context.snapshot_legal_hold_released';
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
        'context_snapshot_legal_hold',
        new.id::text,
        new.reason_code,
        jsonb_build_object(
            'contextSnapshotId', new.context_snapshot_id,
            'holdReference', new.hold_reference,
            'status', new.status
        )
    );
    return new;
end;
$$;

drop trigger if exists audit_context_snapshot_legal_hold_mutation
    on intelligence.context_snapshot_legal_holds;
create trigger audit_context_snapshot_legal_hold_mutation
after insert or update of status
on intelligence.context_snapshot_legal_holds
for each row
execute function intelligence.audit_context_snapshot_legal_hold_mutation();

-- An observation hold also applies to context derived for the same subject or
-- relationship. Business-context observations conservatively protect every
-- context snapshot of the client because historical snapshots do not expose a
-- normalized input-to-observation link.
create or replace function intelligence.context_snapshot_has_active_legal_hold(
    target_account_id uuid,
    target_client_account_id uuid,
    target_snapshot_id uuid,
    target_subject_id uuid,
    target_relationship_id uuid
)
returns boolean
language sql
stable
as $$
    select
        exists (
            select 1
            from intelligence.context_snapshot_legal_holds snapshot_hold
            where snapshot_hold.account_id = target_account_id
              and snapshot_hold.client_account_id = target_client_account_id
              and snapshot_hold.context_snapshot_id = target_snapshot_id
              and snapshot_hold.status = 'active'
        )
        or exists (
            select 1
            from intelligence.observation_legal_holds observation_hold
            join intelligence.source_observations observation
              on observation.account_id = observation_hold.account_id
             and observation.client_account_id = observation_hold.client_account_id
             and observation.id = observation_hold.observation_id
            where observation_hold.account_id = target_account_id
              and observation_hold.client_account_id = target_client_account_id
              and observation_hold.status = 'active'
              and (
                  observation.classification = 'client_business_context'
                  or (
                      target_relationship_id is not null
                      and observation.relationship_id = target_relationship_id
                  )
                  or (
                      target_subject_id is not null
                      and observation.subject_id = target_subject_id
                  )
                  or (
                      target_subject_id is null
                      and target_relationship_id is null
                  )
              )
        );
$$;

-- Serialize changes to observation holds with context retention for the same
-- account/client. The repository acquires this lock in a separate statement,
-- so its retention query sees the hold state committed by the lock winner.
create or replace function intelligence.lock_context_retention_for_observation_hold()
returns trigger
language plpgsql
as $$
declare
    lock_account_id uuid;
    lock_client_account_id uuid;
begin
    if tg_op = 'DELETE' then
        lock_account_id := old.account_id;
        lock_client_account_id := old.client_account_id;
    else
        lock_account_id := new.account_id;
        lock_client_account_id := new.client_account_id;
    end if;

    perform pg_advisory_xact_lock(hashtextextended(
        'customer_intelligence:context_retention:'
            || lock_account_id::text
            || ':'
            || lock_client_account_id::text,
        0
    ));
    if tg_op = 'DELETE' then
        return old;
    end if;
    return new;
end;
$$;

drop trigger if exists context_retention_lock_observation_hold
    on intelligence.observation_legal_holds;
create trigger context_retention_lock_observation_hold
before insert or update or delete
on intelligence.observation_legal_holds
for each row
execute function intelligence.lock_context_retention_for_observation_hold();

create or replace function intelligence.prevent_held_context_snapshot_retention()
returns trigger
language plpgsql
as $$
begin
    if intelligence.context_snapshot_has_active_legal_hold(
           old.account_id,
           old.client_account_id,
           old.id,
           old.subject_id,
           old.relationship_id
       )
       and (
           old.retention_state is distinct from new.retention_state
           or old.payload_ciphertext is distinct from new.payload_ciphertext
           or old.cipher_key_version is distinct from new.cipher_key_version
           or old.payload_hash is distinct from new.payload_hash
           or old.tombstoned_at is distinct from new.tombstoned_at
           or old.retention_reason_code is distinct from new.retention_reason_code
       ) then
        raise check_violation
            using constraint = 'intelligence_context_snapshot_active_legal_hold',
                  message = 'active legal hold blocks context snapshot retention';
    end if;
    return new;
end;
$$;

drop trigger if exists prevent_held_context_snapshot_retention
    on intelligence.context_snapshots;
create trigger prevent_held_context_snapshot_retention
before update of retention_state, payload_ciphertext, cipher_key_version,
    payload_hash, tombstoned_at, retention_reason_code
on intelligence.context_snapshots
for each row
execute function intelligence.prevent_held_context_snapshot_retention();
