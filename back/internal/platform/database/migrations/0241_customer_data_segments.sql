create table customer_data.segments (
    id uuid primary key default gen_random_uuid(),
    account_id uuid not null references core.accounts(id) on delete restrict,
    client_account_id uuid not null references core.accounts(id) on delete restrict,
    segment_key text not null,
    name text not null,
    description text,
    status text not null default 'active',
    idempotency_key text not null,
    active_version_id uuid,
    current_materialization_id uuid,
    revision bigint not null default 1,
    owner_user_id uuid references core.users(id) on delete set null,
    created_by_user_id uuid references core.users(id) on delete set null,
    updated_by_user_id uuid references core.users(id) on delete set null,
    archived_at timestamptz,
    created_at timestamptz not null default now(),
    updated_at timestamptz not null default now(),
    unique (account_id, client_account_id, id),
    unique (account_id, client_account_id, segment_key),
    unique (account_id, client_account_id, idempotency_key),
    constraint customer_data_segment_key_check
        check (segment_key ~ '^[a-z0-9][a-z0-9_-]{1,79}$'),
    constraint customer_data_segment_name_check check (char_length(name) between 1 and 160),
    constraint customer_data_segment_description_check
        check (description is null or char_length(description) <= 2000),
    constraint customer_data_segment_status_check check (status in ('active', 'archived')),
    constraint customer_data_segment_revision_check check (revision > 0)
);

create index customer_data_segments_list_idx
    on customer_data.segments
    (account_id, client_account_id, status, updated_at desc, id);

create table customer_data.segment_versions (
    id uuid primary key default gen_random_uuid(),
    account_id uuid not null,
    client_account_id uuid not null,
    segment_id uuid not null,
    version_number integer not null,
    status text not null default 'draft',
    filter_schema_version text not null,
    field_catalog_version text not null,
    filter_ast jsonb not null,
    evaluation_policy jsonb not null default '{}'::jsonb,
    definition_hash text not null,
    idempotency_key text not null,
    validation_hash text,
    validation_reason_codes jsonb not null default '[]'::jsonb,
    revision bigint not null default 1,
    change_summary text,
    created_by_user_id uuid references core.users(id) on delete set null,
    validated_by_user_id uuid references core.users(id) on delete set null,
    published_by_user_id uuid references core.users(id) on delete set null,
    created_at timestamptz not null default now(),
    updated_at timestamptz not null default now(),
    validated_at timestamptz,
    published_at timestamptz,
    archived_at timestamptz,
    unique (account_id, client_account_id, id),
    unique (account_id, client_account_id, segment_id, id),
    unique (account_id, client_account_id, segment_id, version_number),
    unique (account_id, client_account_id, idempotency_key),
    constraint customer_data_segment_version_segment_fk
        foreign key (account_id, client_account_id, segment_id)
        references customer_data.segments(account_id, client_account_id, id)
        on delete restrict,
    constraint customer_data_segment_version_number_check check (version_number > 0),
    constraint customer_data_segment_version_status_check
        check (status in ('draft', 'validated', 'published', 'archived')),
    constraint customer_data_segment_version_schema_check
        check (filter_schema_version = 'segment.filter.v1'),
    constraint customer_data_segment_version_ast_check check (jsonb_typeof(filter_ast) = 'object'),
    constraint customer_data_segment_version_policy_check
        check (jsonb_typeof(evaluation_policy) = 'object'),
    constraint customer_data_segment_version_reasons_check
        check (jsonb_typeof(validation_reason_codes) = 'array'),
    constraint customer_data_segment_version_revision_check check (revision > 0),
    constraint customer_data_segment_version_lifecycle_check
        check (
            (status = 'draft' and published_at is null)
            or (status = 'validated' and validated_at is not null and validation_hash is not null and published_at is null)
            or (status = 'published' and validated_at is not null and validation_hash is not null and published_at is not null)
            or status = 'archived'
        )
);

create unique index customer_data_segment_versions_open_draft_uidx
    on customer_data.segment_versions (account_id, client_account_id, segment_id)
    where status in ('draft', 'validated');
create index customer_data_segment_versions_history_idx
    on customer_data.segment_versions
    (account_id, client_account_id, segment_id, version_number desc, id);

alter table customer_data.segments
    add constraint customer_data_segment_active_version_fk
    foreign key (account_id, client_account_id, id, active_version_id)
    references customer_data.segment_versions(account_id, client_account_id, segment_id, id)
    on delete restrict
    deferrable initially deferred;

create table customer_data.segment_evaluation_runs (
    id uuid primary key default gen_random_uuid(),
    account_id uuid not null,
    client_account_id uuid not null,
    segment_id uuid not null,
    version_id uuid not null,
    mode text not null,
    trigger_kind text not null,
    status text not null default 'queued',
    as_of timestamptz not null,
    definition_hash text not null,
    input_fingerprint text not null,
    field_catalog_version text not null,
    source_snapshot_refs jsonb not null default '[]'::jsonb,
    matched_count bigint,
    excluded_count bigint,
    error_count bigint,
    sample_count integer,
    reason_codes jsonb not null default '[]'::jsonb,
    idempotency_key text not null,
    budget_class text not null default 'small',
    attempts integer not null default 0,
    lease_owner text,
    lease_expires_at timestamptz,
    run_after timestamptz not null default now(),
    cancel_requested_at timestamptz,
    requested_by_user_id uuid references core.users(id) on delete set null,
    correlation_id text,
    causation_id text,
    requested_at timestamptz not null default now(),
    started_at timestamptz,
    finished_at timestamptz,
    unique (account_id, client_account_id, id),
    unique (account_id, client_account_id, idempotency_key),
    constraint customer_data_segment_run_version_fk
        foreign key (account_id, client_account_id, segment_id, version_id)
        references customer_data.segment_versions(account_id, client_account_id, segment_id, id)
        on delete restrict,
    constraint customer_data_segment_run_mode_check
        check (mode in ('preview', 'materialize', 'recompute')),
    constraint customer_data_segment_run_trigger_check
        check (trigger_kind in ('manual', 'schedule', 'source_change', 'backfill')),
    constraint customer_data_segment_run_status_check
        check (status in ('queued', 'running', 'completed', 'partial', 'failed', 'cancelled')),
    constraint customer_data_segment_run_sources_check
        check (jsonb_typeof(source_snapshot_refs) = 'array'),
    constraint customer_data_segment_run_reasons_check
        check (jsonb_typeof(reason_codes) = 'array'),
    constraint customer_data_segment_run_counts_check
        check (
            (matched_count is null or matched_count >= 0)
            and (excluded_count is null or excluded_count >= 0)
            and (error_count is null or error_count >= 0)
            and (sample_count is null or sample_count >= 0)
            and attempts >= 0
        )
);

create index customer_data_segment_runs_list_idx
    on customer_data.segment_evaluation_runs
    (account_id, client_account_id, segment_id, requested_at desc, id);
create index customer_data_segment_runs_claim_idx
    on customer_data.segment_evaluation_runs (status, run_after, requested_at, id)
    where status in ('queued', 'running');

create table customer_data.segment_materializations (
    id uuid primary key default gen_random_uuid(),
    account_id uuid not null,
    client_account_id uuid not null,
    segment_id uuid not null,
    version_id uuid not null,
    evaluation_run_id uuid not null,
    as_of timestamptz not null,
    definition_hash text not null,
    input_fingerprint text not null,
    field_catalog_version text not null,
    source_snapshot_refs jsonb not null default '[]'::jsonb,
    status text not null default 'building',
    member_count bigint not null default 0,
    fresh_until timestamptz,
    expires_at timestamptz,
    created_at timestamptz not null default now(),
    completed_at timestamptz,
    unique (account_id, client_account_id, id),
    unique (account_id, client_account_id, segment_id, id),
    unique (account_id, client_account_id, evaluation_run_id),
    unique (account_id, client_account_id, segment_id, version_id, input_fingerprint),
    constraint customer_data_materialization_version_fk
        foreign key (account_id, client_account_id, segment_id, version_id)
        references customer_data.segment_versions(account_id, client_account_id, segment_id, id)
        on delete restrict,
    constraint customer_data_materialization_run_fk
        foreign key (account_id, client_account_id, evaluation_run_id)
        references customer_data.segment_evaluation_runs(account_id, client_account_id, id)
        on delete restrict,
    constraint customer_data_materialization_status_check
        check (status in ('building', 'current', 'superseded', 'expired', 'failed')),
    constraint customer_data_materialization_sources_check
        check (jsonb_typeof(source_snapshot_refs) = 'array'),
    constraint customer_data_materialization_member_count_check check (member_count >= 0),
    constraint customer_data_materialization_expiry_check
        check (expires_at is null or expires_at > as_of)
);

create index customer_data_materializations_list_idx
    on customer_data.segment_materializations
    (account_id, client_account_id, segment_id, created_at desc, id);

alter table customer_data.segments
    add constraint customer_data_segment_current_materialization_fk
    foreign key (account_id, client_account_id, id, current_materialization_id)
    references customer_data.segment_materializations(account_id, client_account_id, segment_id, id)
    on delete restrict
    deferrable initially deferred;

create table customer_data.segment_memberships (
    account_id uuid not null,
    client_account_id uuid not null,
    materialization_id uuid not null,
    segment_id uuid not null,
    version_id uuid not null,
    relationship_id uuid not null,
    subject_id uuid not null,
    match_fingerprint text not null,
    matched_at timestamptz not null,
    primary key (account_id, client_account_id, materialization_id, relationship_id),
    constraint customer_data_membership_materialization_fk
        foreign key (account_id, client_account_id, segment_id, materialization_id)
        references customer_data.segment_materializations(account_id, client_account_id, segment_id, id)
        on delete restrict,
    constraint customer_data_membership_version_fk
        foreign key (account_id, client_account_id, segment_id, version_id)
        references customer_data.segment_versions(account_id, client_account_id, segment_id, id)
        on delete restrict,
    constraint customer_data_membership_relationship_fk
        foreign key (account_id, client_account_id, subject_id, relationship_id)
        references customer_data.relationships(account_id, client_account_id, subject_id, id)
        on delete restrict
        deferrable initially deferred
);

create index customer_data_segment_memberships_cursor_idx
    on customer_data.segment_memberships
    (account_id, client_account_id, materialization_id, relationship_id);
