-- Customer Intelligence: provenance audit for minimized observations.
-- The audit event contains metadata only. Snapshot, ciphertext, upstream entity
-- identifier and idempotency key are intentionally excluded.

-- 0242 allowed business-context observations to rely on the original
-- customer_relationship default while both subject references were null.
-- Normalize that unambiguous legacy shape before enforcing the stronger
-- contract. A partially scoped row is ambiguous and must stop the upgrade
-- instead of being silently reclassified.
update intelligence.source_observations
set classification = 'client_business_context'
where subject_id is null
  and relationship_id is null
  and classification = 'customer_relationship';

do $$
begin
    if exists (
        select 1
        from intelligence.source_observations
        where (subject_id is null) <> (relationship_id is null)
    ) then
        raise exception using
            errcode = 'check_violation',
            message = 'source_observations contains partially scoped legacy rows';
    end if;
end;
$$;

alter table intelligence.source_observations
    drop constraint if exists intelligence_source_observations_classification_check;
alter table intelligence.source_observations
    add constraint intelligence_source_observations_classification_check
    check (classification in ('customer_relationship', 'client_business_context'));

alter table intelligence.source_observations
    drop constraint if exists intelligence_source_observations_scope_check;
alter table intelligence.source_observations
    add constraint intelligence_source_observations_scope_check
    check (
        (
            classification = 'customer_relationship'
            and subject_id is not null
            and relationship_id is not null
        )
        or (
            classification = 'client_business_context'
            and subject_id is null
            and relationship_id is null
        )
    );

create or replace function intelligence.audit_source_observation_insert()
returns trigger
language plpgsql
as $$
begin
    insert into intelligence.audit_events (
        account_id,
        client_account_id,
        event_type,
        aggregate_type,
        aggregate_id,
        correlation_id,
        reason_code,
        metadata
    )
    values (
        new.account_id,
        new.client_account_id,
        'source.observation_ingested',
        'source_observation',
        new.id::text,
        coalesce(new.ingestion_run_id::text, ''),
        'allowlisted_snapshot_ingested',
        jsonb_build_object(
            'sourceKey', new.source_key,
            'entityType', new.source_entity_type,
            'sensitivity', new.sensitivity,
            'purposeKey', new.purpose_key
        )
    );
    return new;
end;
$$;

drop trigger if exists audit_source_observation_insert
    on intelligence.source_observations;

create trigger audit_source_observation_insert
after insert on intelligence.source_observations
for each row
execute function intelligence.audit_source_observation_insert();
