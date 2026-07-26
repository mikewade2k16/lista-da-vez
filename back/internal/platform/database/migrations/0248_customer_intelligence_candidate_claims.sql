-- 0248_customer_intelligence_candidate_claims.sql
--
-- Claims extraidas de uma decisao operacional aceita permanecem candidatas.
-- A outbox do Omnichannel transporta somente descritores e referencias; o
-- Customer Intelligence reidrata o valor do output cifrado do runtime.
-- Nenhuma linha criada por esta migration promove claim para fact.

alter table intelligence.accepted_outcomes
    drop constraint if exists accepted_outcomes_account_id_event_id_key,
    drop constraint if exists accepted_outcomes_account_id_decision_id_key;

do $$
begin
    if not exists (
        select 1
        from pg_constraint
        where conname = 'intelligence_accepted_outcomes_client_event'
          and conrelid = 'intelligence.accepted_outcomes'::regclass
    ) then
        alter table intelligence.accepted_outcomes
            add constraint intelligence_accepted_outcomes_client_event
            unique (account_id, client_account_id, event_id);
    end if;
    if not exists (
        select 1
        from pg_constraint
        where conname = 'intelligence_accepted_outcomes_client_decision'
          and conrelid = 'intelligence.accepted_outcomes'::regclass
    ) then
        alter table intelligence.accepted_outcomes
            add constraint intelligence_accepted_outcomes_client_decision
            unique (account_id, client_account_id, decision_id);
    end if;
end $$;

alter table intelligence.claims
    add column if not exists source_outcome_event_id uuid,
    add column if not exists source_claim_ordinal integer,
    add column if not exists revision bigint not null default 1,
    add column if not exists reviewed_by_user_id uuid,
    add column if not exists reviewed_at timestamptz,
    add column if not exists review_reason_code text not null default '';

do $$
begin
    if not exists (
        select 1
        from pg_constraint
        where conname = 'intelligence_claims_source_ordinal_ck'
          and conrelid = 'intelligence.claims'::regclass
    ) then
        alter table intelligence.claims
            add constraint intelligence_claims_source_ordinal_ck
            check (
                (source_outcome_event_id is null and source_claim_ordinal is null)
                or (
                    source_outcome_event_id is not null
                    and source_claim_ordinal between 0 and 99
                )
            );
    end if;
    if not exists (
        select 1
        from pg_constraint
        where conname = 'intelligence_claims_revision_ck'
          and conrelid = 'intelligence.claims'::regclass
    ) then
        alter table intelligence.claims
            add constraint intelligence_claims_revision_ck
            check (revision > 0);
    end if;
    if not exists (
        select 1
        from pg_constraint
        where conname = 'intelligence_claims_review_reason_ck'
          and conrelid = 'intelligence.claims'::regclass
    ) then
        alter table intelligence.claims
            add constraint intelligence_claims_review_reason_ck
            check (char_length(review_reason_code) <= 160);
    end if;
    if not exists (
        select 1
        from pg_constraint
        where conname = 'intelligence_claims_reviewer_fk'
          and conrelid = 'intelligence.claims'::regclass
    ) then
        alter table intelligence.claims
            add constraint intelligence_claims_reviewer_fk
            foreign key (reviewed_by_user_id)
            references core.users(id)
            on delete set null;
    end if;
    if not exists (
        select 1
        from pg_constraint
        where conname = 'intelligence_claims_outcome_fk'
          and conrelid = 'intelligence.claims'::regclass
    ) then
        alter table intelligence.claims
            add constraint intelligence_claims_outcome_fk
            foreign key (
                account_id,
                client_account_id,
                source_outcome_event_id
            )
            references intelligence.accepted_outcomes(
                account_id,
                client_account_id,
                event_id
            )
            on delete restrict;
    end if;
    if not exists (
        select 1
        from pg_constraint
        where conname = 'intelligence_claims_prompt_binding_fk'
          and conrelid = 'intelligence.claims'::regclass
    ) then
        alter table intelligence.claims
            add constraint intelligence_claims_prompt_binding_fk
            foreign key (account_id, prompt_binding_id)
            references intelligence.prompt_bindings(account_id, id)
            on delete restrict;
    end if;
    if not exists (
        select 1
        from pg_constraint
        where conname = 'intelligence_claims_runtime_run_fk'
          and conrelid = 'intelligence.claims'::regclass
    ) then
        alter table intelligence.claims
            add constraint intelligence_claims_runtime_run_fk
            foreign key (account_id, runtime_run_id)
            references intelligence.runtime_runs(account_id, id)
            on delete restrict;
    end if;
end $$;

create unique index if not exists intelligence_claims_client_outcome_ordinal_uidx
    on intelligence.claims (
        account_id,
        client_account_id,
        source_outcome_event_id,
        source_claim_ordinal
    )
    where source_outcome_event_id is not null;

create index if not exists intelligence_claims_runtime_idx
    on intelligence.claims (
        account_id,
        client_account_id,
        runtime_run_id,
        created_at desc
    )
    where runtime_run_id is not null;

create index if not exists intelligence_claim_evidence_observation_idx
    on intelligence.claim_evidence (account_id, observation_id, claim_id);
