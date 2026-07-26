-- Durable, encrypted and idempotent projections produced by the headless
-- Customer Intelligence runtime. The LLM output remains encrypted; the
-- columns below contain only operational metadata safe for filtering/audit.

alter table intelligence.source_suggestions
    add column if not exists subject_id uuid,
    add column if not exists rationale_ciphertext text not null default '',
    add column if not exists cipher_key_version text not null default '',
    add column if not exists expires_at timestamptz,
    add column if not exists prompt_binding_id uuid,
    add column if not exists context_snapshot_id uuid,
    add column if not exists output_hash text not null default '',
    add column if not exists review_reason_code text not null default '';

create unique index if not exists intelligence_summary_runtime_run_uidx
    on intelligence.summary_versions (account_id, runtime_run_id)
    where runtime_run_id is not null;

create unique index if not exists intelligence_recommendations_runtime_run_uidx
    on intelligence.recommendations (
        account_id,
        runtime_run_id,
        recommendation_type
    )
    where runtime_run_id is not null;

create unique index if not exists intelligence_source_suggestions_runtime_run_uidx
    on intelligence.source_suggestions (
        account_id,
        runtime_run_id,
        source_key
    )
    where runtime_run_id is not null;

create index if not exists intelligence_source_suggestions_relationship_idx
    on intelligence.source_suggestions (
        account_id,
        client_account_id,
        relationship_id,
        status,
        created_at desc
    );

do $$
begin
    if not exists (
        select 1
          from pg_constraint
         where conname = 'intelligence_source_suggestions_runtime_run_fk'
    ) then
        alter table intelligence.source_suggestions
            add constraint intelligence_source_suggestions_runtime_run_fk
            foreign key (account_id, runtime_run_id)
            references intelligence.runtime_runs(account_id, id)
            on delete restrict;
    end if;

    if not exists (
        select 1
          from pg_constraint
         where conname = 'intelligence_source_suggestions_prompt_binding_fk'
    ) then
        alter table intelligence.source_suggestions
            add constraint intelligence_source_suggestions_prompt_binding_fk
            foreign key (account_id, prompt_binding_id)
            references intelligence.prompt_bindings(account_id, id)
            on delete restrict;
    end if;

    if not exists (
        select 1
          from pg_constraint
         where conname = 'intelligence_source_suggestions_context_snapshot_fk'
    ) then
        alter table intelligence.source_suggestions
            add constraint intelligence_source_suggestions_context_snapshot_fk
            foreign key (account_id, context_snapshot_id)
            references intelligence.context_snapshots(account_id, id)
            on delete restrict;
    end if;
end $$;
