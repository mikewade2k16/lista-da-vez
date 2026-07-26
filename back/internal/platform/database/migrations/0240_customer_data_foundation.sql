create schema if not exists customer_data;

create table customer_data.capability_states (
    account_id uuid not null references core.accounts(id) on delete restrict,
    client_account_id uuid not null references core.accounts(id) on delete restrict,
    capability_key text not null,
    mode text not null default 'off',
    revision bigint not null default 1,
    last_idempotency_key text,
    updated_by_user_id uuid references core.users(id) on delete set null,
    created_at timestamptz not null default now(),
    updated_at timestamptz not null default now(),
    primary key (account_id, client_account_id, capability_key),
    unique (account_id, client_account_id, last_idempotency_key),
    constraint customer_data_capability_key_check
        check (capability_key in ('core', 'identity_resolution', 'matching_merge', 'offline_interactions', 'segmentation', 'segment_exports')),
    constraint customer_data_capability_mode_check
        check (mode in ('off', 'shadow', 'on')),
    constraint customer_data_capability_revision_check check (revision > 0)
);

create table customer_data.subjects (
    id uuid primary key default gen_random_uuid(),
    account_id uuid not null references core.accounts(id) on delete restrict,
    subject_type text not null,
    status text not null default 'active',
    merged_into_subject_id uuid,
    idempotency_key text not null,
    revision bigint not null default 1,
    created_by_user_id uuid references core.users(id) on delete set null,
    updated_by_user_id uuid references core.users(id) on delete set null,
    created_at timestamptz not null default now(),
    updated_at timestamptz not null default now(),
    unique (account_id, id),
    unique (account_id, idempotency_key),
    constraint customer_data_subject_type_check
        check (subject_type in ('person', 'organization')),
    constraint customer_data_subject_status_check
        check (status in ('active', 'merged', 'anonymized')),
    constraint customer_data_subject_merge_state_check
        check (
            (status = 'merged' and merged_into_subject_id is not null)
            or (status <> 'merged' and merged_into_subject_id is null)
        ),
    constraint customer_data_subject_merge_self_check
        check (merged_into_subject_id is null or merged_into_subject_id <> id),
    constraint customer_data_subject_revision_check check (revision > 0),
    constraint customer_data_subject_merge_target_fk
        foreign key (account_id, merged_into_subject_id)
        references customer_data.subjects(account_id, id)
        on delete restrict
        deferrable initially deferred
);

create index customer_data_subjects_status_idx
    on customer_data.subjects (account_id, status, updated_at desc, id);
create index customer_data_subjects_type_status_idx
    on customer_data.subjects (account_id, subject_type, status, updated_at desc, id);
create index customer_data_subjects_merge_target_idx
    on customer_data.subjects (account_id, merged_into_subject_id)
    where merged_into_subject_id is not null;

create table customer_data.subject_person_profiles (
    account_id uuid not null,
    subject_id uuid not null,
    legal_name text,
    preferred_name text,
    birth_date date,
    locale text,
    timezone text,
    verified_at timestamptz,
    verification_source_ref text,
    revision bigint not null default 1,
    created_at timestamptz not null default now(),
    updated_at timestamptz not null default now(),
    primary key (account_id, subject_id),
    constraint customer_data_person_profile_subject_fk
        foreign key (account_id, subject_id)
        references customer_data.subjects(account_id, id)
        on delete restrict,
    constraint customer_data_person_profile_revision_check check (revision > 0),
    constraint customer_data_person_profile_name_check
        check (
            (legal_name is null or char_length(legal_name) between 1 and 200)
            and (preferred_name is null or char_length(preferred_name) between 1 and 120)
        )
);

create table customer_data.subject_organization_profiles (
    account_id uuid not null,
    subject_id uuid not null,
    legal_name text,
    trade_name text,
    registration_country text,
    registration_id_ciphertext text,
    registration_id_fingerprint text,
    key_version text,
    verified_at timestamptz,
    verification_source_ref text,
    revision bigint not null default 1,
    created_at timestamptz not null default now(),
    updated_at timestamptz not null default now(),
    primary key (account_id, subject_id),
    constraint customer_data_organization_profile_subject_fk
        foreign key (account_id, subject_id)
        references customer_data.subjects(account_id, id)
        on delete restrict,
    constraint customer_data_organization_profile_revision_check check (revision > 0),
    constraint customer_data_organization_profile_names_check
        check (
            (legal_name is null or char_length(legal_name) between 1 and 200)
            and (trade_name is null or char_length(trade_name) between 1 and 200)
        ),
    constraint customer_data_organization_profile_registration_check
        check (
            (registration_id_ciphertext is null and registration_id_fingerprint is null and key_version is null)
            or (registration_id_ciphertext is not null and registration_id_fingerprint is not null and key_version is not null)
        )
);

create table customer_data.relationships (
    id uuid primary key default gen_random_uuid(),
    account_id uuid not null references core.accounts(id) on delete restrict,
    client_account_id uuid not null references core.accounts(id) on delete restrict,
    subject_id uuid not null,
    display_name text not null,
    preferred_name text,
    lifecycle_status text not null default 'lead',
    classification_source text not null default 'manual',
    classification_confidence numeric(5,4),
    owner_user_id uuid references core.users(id) on delete set null,
    tags jsonb not null default '[]'::jsonb,
    custom_fields jsonb not null default '{}'::jsonb,
    first_seen_at timestamptz,
    last_seen_at timestamptz,
    last_qualified_at timestamptz,
    archived_at timestamptz,
    revision bigint not null default 1,
    created_by_user_id uuid references core.users(id) on delete set null,
    updated_by_user_id uuid references core.users(id) on delete set null,
    created_at timestamptz not null default now(),
    updated_at timestamptz not null default now(),
    unique (account_id, client_account_id, subject_id),
    unique (account_id, client_account_id, id),
    unique (account_id, client_account_id, subject_id, id),
    constraint customer_data_relationship_subject_fk
        foreign key (account_id, subject_id)
        references customer_data.subjects(account_id, id)
        on delete restrict,
    constraint customer_data_relationship_display_name_check
        check (char_length(display_name) between 1 and 200),
    constraint customer_data_relationship_preferred_name_check
        check (preferred_name is null or char_length(preferred_name) between 1 and 120),
    constraint customer_data_relationship_lifecycle_check
        check (lifecycle_status in ('lead', 'prospect', 'customer', 'inactive')),
    constraint customer_data_relationship_classification_source_check
        check (classification_source in ('manual', 'erp', 'rule', 'backfill')),
    constraint customer_data_relationship_confidence_check
        check (classification_confidence is null or classification_confidence between 0 and 1),
    constraint customer_data_relationship_tags_check
        check (jsonb_typeof(tags) = 'array' and jsonb_array_length(tags) <= 50),
    constraint customer_data_relationship_custom_fields_check
        check (jsonb_typeof(custom_fields) = 'object'),
    constraint customer_data_relationship_seen_check
        check (first_seen_at is null or last_seen_at is null or first_seen_at <= last_seen_at),
    constraint customer_data_relationship_revision_check check (revision > 0)
);

create index customer_data_relationships_list_idx
    on customer_data.relationships
    (account_id, client_account_id, archived_at, updated_at desc, id);
create index customer_data_relationships_lifecycle_idx
    on customer_data.relationships
    (account_id, client_account_id, lifecycle_status, last_seen_at desc, id);

create table customer_data.subject_identities (
    id uuid primary key default gen_random_uuid(),
    account_id uuid not null,
    client_account_id uuid not null,
    relationship_id uuid not null,
    subject_id uuid not null,
    identity_kind text not null,
    issuer text not null,
    value_ciphertext text,
    value_fingerprint text not null,
    key_version text not null,
    masked_value text not null,
    verification_status text not null default 'unverified',
    verification_method text,
    source_ref_type text,
    source_ref_id text,
    metadata jsonb not null default '{}'::jsonb,
    first_seen_at timestamptz not null,
    last_seen_at timestamptz not null,
    verified_at timestamptz,
    revoked_at timestamptz,
    idempotency_key text not null,
    revision bigint not null default 1,
    created_by_user_id uuid references core.users(id) on delete set null,
    updated_by_user_id uuid references core.users(id) on delete set null,
    created_at timestamptz not null default now(),
    updated_at timestamptz not null default now(),
    unique (account_id, client_account_id, id),
    unique (account_id, client_account_id, idempotency_key),
    constraint customer_data_identity_relationship_fk
        foreign key (account_id, client_account_id, subject_id, relationship_id)
        references customer_data.relationships(account_id, client_account_id, subject_id, id)
        on delete restrict
        deferrable initially deferred,
    constraint customer_data_identity_kind_check
        check (identity_kind in ('phone', 'email', 'whatsapp', 'instagram', 'erp_customer', 'site_visitor', 'document', 'other')),
    constraint customer_data_identity_issuer_check
        check (char_length(issuer) between 1 and 120),
    constraint customer_data_identity_masked_check
        check (char_length(masked_value) between 1 and 200),
    constraint customer_data_identity_verification_check
        check (verification_status in ('unverified', 'verified', 'revoked')),
    constraint customer_data_identity_metadata_check check (jsonb_typeof(metadata) = 'object'),
    constraint customer_data_identity_seen_check check (first_seen_at <= last_seen_at),
    constraint customer_data_identity_revision_check check (revision > 0)
);

create unique index customer_data_identities_active_fingerprint_uidx
    on customer_data.subject_identities
    (account_id, client_account_id, identity_kind, issuer, value_fingerprint)
    where verification_status <> 'revoked';
create index customer_data_identities_relationship_idx
    on customer_data.subject_identities
    (account_id, client_account_id, relationship_id, updated_at desc, id);
create index customer_data_identities_subject_idx
    on customer_data.subject_identities
    (account_id, subject_id, verification_status, id);

create table customer_data.subject_source_links (
    id uuid primary key default gen_random_uuid(),
    account_id uuid not null,
    client_account_id uuid not null,
    subject_id uuid not null,
    relationship_id uuid not null,
    source_module text not null,
    source_key text not null,
    source_entity_type text not null,
    source_entity_id text not null,
    source_version text,
    source_hash text,
    link_method text not null,
    match_confidence numeric(5,4),
    status text not null default 'active',
    idempotency_key text not null,
    linked_by_user_id uuid references core.users(id) on delete set null,
    reviewed_by_user_id uuid references core.users(id) on delete set null,
    reviewed_at timestamptz,
    created_at timestamptz not null default now(),
    updated_at timestamptz not null default now(),
    unique (account_id, client_account_id, source_module, source_entity_type, source_entity_id),
    unique (account_id, idempotency_key),
    constraint customer_data_source_link_relationship_fk
        foreign key (account_id, client_account_id, subject_id, relationship_id)
        references customer_data.relationships(account_id, client_account_id, subject_id, id)
        on delete restrict
        deferrable initially deferred,
    constraint customer_data_source_link_method_check
        check (link_method in ('verified_exact', 'manual', 'backfill', 'reviewed_candidate')),
    constraint customer_data_source_link_status_check
        check (status in ('active', 'superseded', 'quarantined')),
    constraint customer_data_source_link_confidence_check
        check (match_confidence is null or match_confidence between 0 and 1)
);

create table customer_data.relationship_notes (
    id uuid primary key default gen_random_uuid(),
    account_id uuid not null,
    client_account_id uuid not null,
    relationship_id uuid not null,
    content text not null,
    context_source_module text,
    context_entity_type text,
    context_entity_id text,
    author_user_id uuid references core.users(id) on delete set null,
    idempotency_key text not null,
    revision bigint not null default 1,
    archived_at timestamptz,
    created_at timestamptz not null default now(),
    updated_at timestamptz not null default now(),
    unique (account_id, client_account_id, id),
    unique (account_id, client_account_id, idempotency_key),
    constraint customer_data_note_relationship_fk
        foreign key (account_id, client_account_id, relationship_id)
        references customer_data.relationships(account_id, client_account_id, id)
        on delete restrict,
    constraint customer_data_note_content_check check (char_length(content) between 1 and 10000),
    constraint customer_data_note_revision_check check (revision > 0)
);

create index customer_data_notes_timeline_idx
    on customer_data.relationship_notes
    (account_id, client_account_id, relationship_id, created_at desc, id);

create table customer_data.offline_interactions (
    id uuid primary key default gen_random_uuid(),
    account_id uuid not null,
    client_account_id uuid not null,
    relationship_id uuid not null,
    interaction_type text not null,
    occurred_at timestamptz not null,
    timezone text not null,
    duration_seconds integer,
    title text not null,
    content_sanitized text,
    content_ciphertext text,
    cipher_key_version text,
    sensitivity text not null default 'internal',
    purpose_key text not null,
    source_external_ref text,
    status text not null default 'active',
    revision bigint not null default 1,
    idempotency_key text not null,
    created_by_user_id uuid references core.users(id) on delete set null,
    updated_by_user_id uuid references core.users(id) on delete set null,
    created_at timestamptz not null default now(),
    updated_at timestamptz not null default now(),
    unique (account_id, client_account_id, id),
    unique (account_id, client_account_id, idempotency_key),
    constraint customer_data_offline_relationship_fk
        foreign key (account_id, client_account_id, relationship_id)
        references customer_data.relationships(account_id, client_account_id, id)
        on delete restrict,
    constraint customer_data_offline_type_check
        check (interaction_type in ('meeting', 'call', 'offline_chat', 'visit', 'note', 'other')),
    constraint customer_data_offline_duration_check
        check (duration_seconds is null or duration_seconds between 0 and 86400),
    constraint customer_data_offline_title_check check (char_length(title) between 1 and 240),
    constraint customer_data_offline_content_check
        check (
            (sensitivity in ('public', 'internal') and content_ciphertext is null and cipher_key_version is null)
            or (sensitivity in ('personal', 'sensitive', 'restricted') and content_sanitized is null and content_ciphertext is not null and cipher_key_version is not null)
        ),
    constraint customer_data_offline_sensitivity_check
        check (sensitivity in ('public', 'internal', 'personal', 'sensitive', 'restricted')),
    constraint customer_data_offline_status_check check (status in ('active', 'archived')),
    constraint customer_data_offline_revision_check check (revision > 0)
);

create index customer_data_offline_timeline_idx
    on customer_data.offline_interactions
    (account_id, client_account_id, relationship_id, occurred_at desc, id);

create table customer_data.relationship_consents (
    id uuid primary key default gen_random_uuid(),
    account_id uuid not null,
    client_account_id uuid not null,
    relationship_id uuid not null,
    purpose text not null,
    channel text not null,
    status text not null,
    source_module text not null,
    source_ref text,
    evidence_hash text,
    effective_at timestamptz not null,
    expires_at timestamptz,
    actor_user_id uuid references core.users(id) on delete set null,
    idempotency_key text not null,
    created_at timestamptz not null default now(),
    unique (account_id, client_account_id, id),
    unique (account_id, client_account_id, idempotency_key),
    constraint customer_data_consent_relationship_fk
        foreign key (account_id, client_account_id, relationship_id)
        references customer_data.relationships(account_id, client_account_id, id)
        on delete restrict,
    constraint customer_data_consent_status_check
        check (status in ('granted', 'revoked', 'unknown')),
    constraint customer_data_consent_expiry_check
        check (expires_at is null or expires_at > effective_at)
);

create index customer_data_consents_timeline_idx
    on customer_data.relationship_consents
    (account_id, client_account_id, relationship_id, purpose, channel, effective_at desc, id);

create table customer_data.match_candidates (
    id uuid primary key default gen_random_uuid(),
    account_id uuid not null references core.accounts(id) on delete restrict,
    client_account_id uuid not null references core.accounts(id) on delete restrict,
    incoming_source_key text not null,
    incoming_source_type text not null,
    incoming_source_id text not null,
    incoming_source_version text,
    candidate_subject_id uuid,
    candidate_relationship_id uuid,
    match_method text not null,
    match_confidence numeric(5,4) not null,
    evidence_refs jsonb not null default '[]'::jsonb,
    risk_flags jsonb not null default '[]'::jsonb,
    status text not null default 'pending',
    decision_reason text,
    reviewed_by_user_id uuid references core.users(id) on delete set null,
    reviewed_at timestamptz,
    expires_at timestamptz,
    revision bigint not null default 1,
    idempotency_key text not null,
    created_at timestamptz not null default now(),
    updated_at timestamptz not null default now(),
    unique (account_id, client_account_id, id),
    unique (account_id, idempotency_key),
    constraint customer_data_match_candidate_subject_fk
        foreign key (account_id, candidate_subject_id)
        references customer_data.subjects(account_id, id)
        on delete restrict,
    constraint customer_data_match_candidate_relationship_fk
        foreign key (account_id, client_account_id, candidate_relationship_id)
        references customer_data.relationships(account_id, client_account_id, id)
        on delete restrict,
    constraint customer_data_match_candidate_confidence_check
        check (match_confidence between 0 and 1),
    constraint customer_data_match_candidate_evidence_check
        check (jsonb_typeof(evidence_refs) = 'array' and jsonb_array_length(evidence_refs) <= 50),
    constraint customer_data_match_candidate_risk_check
        check (jsonb_typeof(risk_flags) = 'array' and jsonb_array_length(risk_flags) <= 20),
    constraint customer_data_match_candidate_status_check
        check (status in ('pending', 'accepted', 'rejected', 'expired')),
    constraint customer_data_match_candidate_decision_check
        check (
            (status = 'pending' and decision_reason is null and reviewed_at is null)
            or (status <> 'pending' and decision_reason is not null)
        ),
    constraint customer_data_match_candidate_revision_check check (revision > 0)
);

create index customer_data_match_candidates_queue_idx
    on customer_data.match_candidates
    (account_id, client_account_id, status, created_at, id);
create index customer_data_match_candidates_subject_idx
    on customer_data.match_candidates
    (account_id, candidate_subject_id, status, id);

create table customer_data.merge_events (
    id uuid primary key default gen_random_uuid(),
    account_id uuid not null references core.accounts(id) on delete restrict,
    client_account_id uuid not null references core.accounts(id) on delete restrict,
    source_subject_id uuid not null,
    target_subject_id uuid not null,
    affected_relationship_ids jsonb not null,
    reason text not null,
    actor_user_id uuid references core.users(id) on delete set null,
    idempotency_key text not null,
    event_kind text not null,
    reverses_event_id uuid,
    snapshot jsonb not null,
    created_at timestamptz not null default now(),
    unique (account_id, client_account_id, id),
    unique (account_id, idempotency_key),
    constraint customer_data_merge_source_fk
        foreign key (account_id, source_subject_id)
        references customer_data.subjects(account_id, id)
        on delete restrict,
    constraint customer_data_merge_target_fk
        foreign key (account_id, target_subject_id)
        references customer_data.subjects(account_id, id)
        on delete restrict,
    constraint customer_data_merge_reverse_fk
        foreign key (account_id, client_account_id, reverses_event_id)
        references customer_data.merge_events(account_id, client_account_id, id)
        on delete restrict,
    constraint customer_data_merge_subject_check check (source_subject_id <> target_subject_id),
    constraint customer_data_merge_relationships_check
        check (jsonb_typeof(affected_relationship_ids) = 'array'),
    constraint customer_data_merge_snapshot_check check (jsonb_typeof(snapshot) = 'object'),
    constraint customer_data_merge_kind_check check (event_kind in ('merge', 'undo')),
    constraint customer_data_merge_reverse_check
        check (
            (event_kind = 'merge' and reverses_event_id is null)
            or (event_kind = 'undo' and reverses_event_id is not null)
        )
);

create index customer_data_merge_events_subject_idx
    on customer_data.merge_events (account_id, source_subject_id, created_at desc, id);

create table customer_data.writer_states (
    account_id uuid not null references core.accounts(id) on delete restrict,
    client_account_id uuid not null references core.accounts(id) on delete restrict,
    entity_key text not null,
    mode text not null default 'legacy',
    watermark text,
    source_checksum text,
    target_checksum text,
    approved_by_user_id uuid references core.users(id) on delete set null,
    approved_at timestamptz,
    revision bigint not null default 1,
    last_idempotency_key text,
    created_at timestamptz not null default now(),
    updated_at timestamptz not null default now(),
    primary key (account_id, client_account_id, entity_key),
    unique (account_id, client_account_id, last_idempotency_key),
    constraint customer_data_writer_entity_check
        check (entity_key in ('relationship', 'identity', 'note', 'consent', 'merge', 'segment_definition')),
    constraint customer_data_writer_mode_check check (mode in ('legacy', 'shadow', 'new')),
    constraint customer_data_writer_approval_check
        check (mode <> 'new' or (approved_by_user_id is not null and approved_at is not null)),
    constraint customer_data_writer_revision_check check (revision > 0)
);

create table customer_data.outbox_events (
    id uuid primary key default gen_random_uuid(),
    account_id uuid not null references core.accounts(id) on delete restrict,
    client_account_id uuid not null references core.accounts(id) on delete restrict,
    aggregate_type text not null,
    aggregate_id uuid not null,
    topic text not null,
    schema_version text not null,
    payload jsonb not null,
    correlation_id text,
    causation_id text,
    idempotency_key text not null,
    status text not null default 'pending',
    attempts integer not null default 0,
    lease_owner text,
    lease_expires_at timestamptz,
    run_after timestamptz not null default now(),
    error_code text,
    created_at timestamptz not null default now(),
    processed_at timestamptz,
    unique (account_id, idempotency_key),
    constraint customer_data_outbox_payload_check check (jsonb_typeof(payload) = 'object'),
    constraint customer_data_outbox_status_check
        check (status in ('pending', 'processing', 'done', 'failed', 'dead')),
    constraint customer_data_outbox_attempts_check check (attempts >= 0)
);

create index customer_data_outbox_claim_idx
    on customer_data.outbox_events (status, run_after, created_at, id)
    where status in ('pending', 'failed');

create table customer_data.audit_events (
    id uuid primary key default gen_random_uuid(),
    account_id uuid not null references core.accounts(id) on delete restrict,
    client_account_id uuid not null references core.accounts(id) on delete restrict,
    subject_id uuid,
    relationship_id uuid,
    actor_type text not null,
    actor_id text,
    action text not null,
    entity_type text not null,
    entity_id text not null,
    old_hash text,
    new_hash text,
    reason text,
    correlation_id text,
    created_at timestamptz not null default now(),
    constraint customer_data_audit_actor_check
        check (actor_type in ('user', 'service', 'job', 'system'))
);

create index customer_data_audit_scope_idx
    on customer_data.audit_events
    (account_id, client_account_id, created_at desc, id);
create index customer_data_audit_relationship_idx
    on customer_data.audit_events
    (account_id, client_account_id, relationship_id, created_at desc, id);
