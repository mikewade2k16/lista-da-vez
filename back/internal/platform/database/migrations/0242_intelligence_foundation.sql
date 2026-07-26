-- 0242_intelligence_foundation.sql
--
-- Fundacao aditiva do modulo Customer Intelligence. O dado bruto continua no
-- modulo de origem; este schema guarda observacoes allowlisted, evidencia,
-- claims, fatos resolvidos, sinteses, recomendacoes e contexto temporario.
--
-- Capabilities nascem sem linhas e, portanto, OFF. Nenhuma migration habilita
-- fonte, runtime, portfolio ou writer novo. Toda entidade individual carrega
-- account_id + client_account_id e toda FK interna relevante repete account_id.

create schema if not exists intelligence;

insert into core.modules (
    id, schema_name, label, description, is_core, sort_order
)
values (
    'customer_intelligence',
    'intelligence',
    'Inteligencia de Clientes',
    'Contexto, fatos, prompts, agentes e recomendacoes com proveniencia por cliente.',
    false,
    49
)
on conflict (id) do update set
    schema_name = excluded.schema_name,
    label = excluded.label,
    description = excluded.description,
    is_core = excluded.is_core,
    sort_order = excluded.sort_order,
    updated_at = now();

insert into core.permissions (key, module_id, label, description, scope)
values
    ('customer_intelligence.profile.view', 'customer_intelligence', 'Ver inteligencia do cliente', 'Consultar fatos, sinteses e recomendacoes autorizadas.', 'account'),
    ('customer_intelligence.profile.manage', 'customer_intelligence', 'Gerenciar inteligencia do cliente', 'Revisar claims, fatos, sinteses e recomendacoes.', 'account'),
    ('customer_intelligence.sources.view', 'customer_intelligence', 'Ver fontes de inteligencia', 'Consultar catalogo, configuracao, health e runs sem segredos.', 'account'),
    ('customer_intelligence.sources.manage', 'customer_intelligence', 'Gerenciar fontes de inteligencia', 'Configurar e disparar fontes allowlisted.', 'account'),
    ('customer_intelligence.agents.manage', 'customer_intelligence', 'Gerenciar agentes de inteligencia', 'Gerenciar agentes, modelos e credenciais write-only.', 'account'),
    ('customer_intelligence.prompts.view', 'customer_intelligence', 'Ver prompts de inteligencia', 'Consultar processos, versoes, bindings e avaliacoes autorizadas.', 'account'),
    ('customer_intelligence.prompts.manage', 'customer_intelligence', 'Editar prompts de inteligencia', 'Criar, editar, validar e testar drafts.', 'account'),
    ('customer_intelligence.prompts.publish', 'customer_intelligence', 'Publicar prompts de inteligencia', 'Publicar, ativar, pausar e fazer rollback de bindings.', 'account'),
    ('customer_intelligence.prompts.platform_manage', 'customer_intelligence', 'Gerenciar guardrails globais', 'Administrar guardrails de plataforma que tenants nao sobrescrevem.', 'platform'),
    ('customer_intelligence.runs.view', 'customer_intelligence', 'Ver execucoes de inteligencia', 'Consultar custo, latencia, versoes e erros sanitizados.', 'account'),
    ('customer_intelligence.audit.view', 'customer_intelligence', 'Ver auditoria de inteligencia', 'Consultar eventos de auditoria do modulo.', 'account'),
    ('customer_intelligence.portfolio.view', 'customer_intelligence', 'Ver oportunidades agregadas', 'Consultar somente oportunidades agregadas e suprimidas conforme policy.', 'account'),
    ('customer_intelligence.portfolio.manage', 'customer_intelligence', 'Gerenciar oportunidades agregadas', 'Gerar e revisar oportunidades agregadas conforme policy.', 'account'),
    ('customer_intelligence.portfolio.platform_manage', 'customer_intelligence', 'Gerenciar policy global de portfolio', 'Administrar coorte e supressao globais de portfolio.', 'platform')
on conflict (key) do update set
    module_id = excluded.module_id,
    label = excluded.label,
    description = excluded.description,
    scope = excluded.scope,
    deprecated_at = null,
    updated_at = now();

create table if not exists intelligence.capabilities (
    id uuid primary key default gen_random_uuid(),
    account_id uuid not null references core.accounts(id) on delete cascade,
    client_account_id uuid not null references core.accounts(id) on delete cascade,
    capability_key text not null,
    scope_key text not null default '',
    mode text not null default 'off'
        check (mode in ('off', 'shadow', 'canary', 'on')),
    config jsonb not null default '{}'::jsonb
        check (jsonb_typeof(config) = 'object'),
    revision bigint not null default 1 check (revision > 0),
    updated_by_user_id uuid references core.users(id) on delete set null,
    created_at timestamptz not null default now(),
    updated_at timestamptz not null default now(),
    unique (account_id, client_account_id, capability_key, scope_key)
);

create index if not exists intelligence_capabilities_scope_idx
    on intelligence.capabilities (account_id, client_account_id, capability_key, scope_key, mode);

create table if not exists intelligence.fact_definitions (
    id uuid primary key default gen_random_uuid(),
    account_id uuid not null references core.accounts(id) on delete cascade,
    fact_key text not null check (length(btrim(fact_key)) between 1 and 160),
    catalog_status text not null default 'registered'
        check (catalog_status in ('registered', 'deprecated')),
    active_version_id uuid,
    revision bigint not null default 1 check (revision > 0),
    created_by_user_id uuid references core.users(id) on delete set null,
    updated_by_user_id uuid references core.users(id) on delete set null,
    created_at timestamptz not null default now(),
    updated_at timestamptz not null default now(),
    deprecated_at timestamptz,
    unique (account_id, fact_key),
    unique (account_id, id)
);

create table if not exists intelligence.fact_definition_versions (
    id uuid primary key default gen_random_uuid(),
    account_id uuid not null references core.accounts(id) on delete cascade,
    fact_definition_id uuid not null,
    version integer not null check (version > 0),
    status text not null default 'draft'
        check (status in ('draft', 'validated', 'published', 'archived')),
    label text not null check (length(btrim(label)) between 1 and 200),
    value_type text not null
        check (value_type in ('string', 'integer', 'decimal', 'boolean', 'date', 'timestamp', 'enum', 'string_list', 'object_closed')),
    value_schema jsonb not null default '{}'::jsonb
        check (jsonb_typeof(value_schema) = 'object'),
    sensitivity text not null default 'personal'
        check (sensitivity in ('public', 'internal', 'personal', 'sensitive', 'restricted')),
    relationship_scoped boolean not null default true,
    context_allowed boolean not null default false,
    cross_client_allowed boolean not null default false,
    manual_verification_allowed boolean not null default true,
    revision bigint not null default 1 check (revision > 0),
    based_on_version_id uuid,
    created_by_user_id uuid references core.users(id) on delete set null,
    validated_by_user_id uuid references core.users(id) on delete set null,
    published_by_user_id uuid references core.users(id) on delete set null,
    created_at timestamptz not null default now(),
    validated_at timestamptz,
    published_at timestamptz,
    unique (account_id, fact_definition_id, version),
    unique (account_id, id),
    constraint intelligence_fact_definition_versions_definition_fk
        foreign key (account_id, fact_definition_id)
        references intelligence.fact_definitions(account_id, id) on delete cascade,
    constraint intelligence_fact_definition_versions_based_on_fk
        foreign key (account_id, based_on_version_id)
        references intelligence.fact_definition_versions(account_id, id) on delete set null
);

do $$
begin
    if not exists (
        select 1 from pg_constraint
        where conname = 'intelligence_fact_definitions_active_version_fk'
          and conrelid = 'intelligence.fact_definitions'::regclass
    ) then
        alter table intelligence.fact_definitions
            add constraint intelligence_fact_definitions_active_version_fk
            foreign key (account_id, active_version_id)
            references intelligence.fact_definition_versions(account_id, id)
            on delete set null;
    end if;
end $$;

create index if not exists intelligence_fact_definition_versions_status_idx
    on intelligence.fact_definition_versions (account_id, fact_definition_id, status, version desc);

create table if not exists intelligence.retention_policy_versions (
    id uuid primary key default gen_random_uuid(),
    account_id uuid not null references core.accounts(id) on delete cascade,
    policy_key text not null check (length(btrim(policy_key)) between 1 and 160),
    version integer not null check (version > 0),
    status text not null default 'draft'
        check (status in ('draft', 'validated', 'published', 'archived')),
    category_rules jsonb not null default '{}'::jsonb
        check (jsonb_typeof(category_rules) = 'object'),
    revision bigint not null default 1 check (revision > 0),
    created_by_user_id uuid references core.users(id) on delete set null,
    published_by_user_id uuid references core.users(id) on delete set null,
    created_at timestamptz not null default now(),
    published_at timestamptz,
    unique (account_id, policy_key, version),
    unique (account_id, id)
);

create table if not exists intelligence.authority_policy_versions (
    id uuid primary key default gen_random_uuid(),
    account_id uuid not null references core.accounts(id) on delete cascade,
    fact_definition_id uuid not null,
    fact_definition_version_id uuid not null,
    version integer not null check (version > 0),
    status text not null default 'draft'
        check (status in ('draft', 'published', 'archived')),
    rules jsonb not null default '[]'::jsonb
        check (jsonb_typeof(rules) = 'array'),
    conflict_strategy text not null default 'manual_review'
        check (conflict_strategy in ('keep_current', 'highest_authority', 'newest_valid', 'manual_review')),
    published_by_user_id uuid references core.users(id) on delete set null,
    created_at timestamptz not null default now(),
    published_at timestamptz,
    unique (account_id, fact_definition_id, version),
    unique (account_id, id),
    constraint intelligence_authority_policy_definition_fk
        foreign key (account_id, fact_definition_id)
        references intelligence.fact_definitions(account_id, id) on delete cascade,
    constraint intelligence_authority_policy_definition_version_fk
        foreign key (account_id, fact_definition_version_id)
        references intelligence.fact_definition_versions(account_id, id) on delete restrict
);

create table if not exists intelligence.authority_policy_bindings (
    id uuid primary key default gen_random_uuid(),
    account_id uuid not null references core.accounts(id) on delete cascade,
    client_account_id uuid references core.accounts(id) on delete cascade,
    fact_definition_id uuid not null,
    fact_definition_version_id uuid not null,
    authority_policy_version_id uuid not null,
    status text not null default 'draft'
        check (status in ('draft', 'published', 'archived')),
    revision bigint not null default 1 check (revision > 0),
    created_by_user_id uuid references core.users(id) on delete set null,
    updated_by_user_id uuid references core.users(id) on delete set null,
    created_at timestamptz not null default now(),
    updated_at timestamptz not null default now(),
    unique nulls not distinct (account_id, client_account_id, fact_definition_id),
    unique (account_id, id),
    constraint intelligence_authority_binding_definition_fk
        foreign key (account_id, fact_definition_id)
        references intelligence.fact_definitions(account_id, id) on delete cascade,
    constraint intelligence_authority_binding_definition_version_fk
        foreign key (account_id, fact_definition_version_id)
        references intelligence.fact_definition_versions(account_id, id) on delete restrict,
    constraint intelligence_authority_binding_policy_fk
        foreign key (account_id, authority_policy_version_id)
        references intelligence.authority_policy_versions(account_id, id) on delete restrict
);

create table if not exists intelligence.source_configs (
    id uuid primary key default gen_random_uuid(),
    account_id uuid not null references core.accounts(id) on delete cascade,
    client_account_id uuid not null references core.accounts(id) on delete cascade,
    source_key text not null check (length(btrim(source_key)) between 1 and 120),
    connection_key text not null default 'default'
        check (length(btrim(connection_key)) between 1 and 120),
    status text not null default 'disabled'
        check (status in ('draft', 'enabled', 'disabled', 'error')),
    mode text not null
        check (mode in ('event', 'scheduled', 'on_demand', 'manual')),
    purpose_key text not null check (length(btrim(purpose_key)) between 1 and 120),
    field_allowlist jsonb not null default '[]'::jsonb
        check (jsonb_typeof(field_allowlist) = 'array'),
    freshness_seconds integer not null default 3600 check (freshness_seconds between 0 and 31536000),
    retention_policy_key text not null default '',
    retention_policy_version_id uuid,
    config jsonb not null default '{}'::jsonb
        check (jsonb_typeof(config) = 'object'),
    cursor jsonb not null default '{}'::jsonb
        check (jsonb_typeof(cursor) = 'object'),
    revision bigint not null default 1 check (revision > 0),
    last_health_status text not null default 'unknown'
        check (last_health_status in ('unknown', 'ok', 'degraded', 'error', 'disabled')),
    last_health_at timestamptz,
    created_by_user_id uuid references core.users(id) on delete set null,
    updated_by_user_id uuid references core.users(id) on delete set null,
    created_at timestamptz not null default now(),
    updated_at timestamptz not null default now(),
    unique (account_id, client_account_id, source_key, connection_key),
    unique (account_id, id),
    constraint intelligence_source_retention_policy_fk
        foreign key (account_id, retention_policy_version_id)
        references intelligence.retention_policy_versions(account_id, id) on delete restrict
);

create index if not exists intelligence_source_configs_enabled_idx
    on intelligence.source_configs (account_id, client_account_id, source_key, status);

create table if not exists intelligence.source_ingestion_runs (
    id uuid primary key default gen_random_uuid(),
    account_id uuid not null references core.accounts(id) on delete cascade,
    client_account_id uuid not null references core.accounts(id) on delete cascade,
    source_config_id uuid not null,
    source_key text not null,
    source_config_revision bigint not null check (source_config_revision > 0),
    retention_policy_version_id uuid,
    trigger text not null
        check (trigger in ('event', 'schedule', 'manual', 'replay', 'backfill', 'on_demand')),
    status text not null default 'queued'
        check (status in ('queued', 'running', 'completed', 'partial', 'failed', 'dead')),
    idempotency_key text not null,
    cursor_before jsonb not null default '{}'::jsonb,
    cursor_after jsonb not null default '{}'::jsonb,
    observed_count integer not null default 0 check (observed_count >= 0),
    accepted_count integer not null default 0 check (accepted_count >= 0),
    rejected_count integer not null default 0 check (rejected_count >= 0),
    error_code text not null default '',
    started_at timestamptz,
    completed_at timestamptz,
    created_at timestamptz not null default now(),
    unique (account_id, idempotency_key),
    unique (account_id, id),
    constraint intelligence_ingestion_runs_source_fk
        foreign key (account_id, source_config_id)
        references intelligence.source_configs(account_id, id) on delete restrict
);

create table if not exists intelligence.source_ingestion_jobs (
    id uuid primary key default gen_random_uuid(),
    account_id uuid not null references core.accounts(id) on delete cascade,
    ordering_key text not null,
    idempotency_key text not null,
    kind text not null,
    payload jsonb not null default '{}'::jsonb
        check (jsonb_typeof(payload) = 'object'),
    status text not null default 'pending'
        check (status in ('pending', 'processing', 'done', 'dead')),
    attempts integer not null default 0 check (attempts >= 0),
    max_attempts integer not null default 5 check (max_attempts between 1 and 20),
    run_after timestamptz not null default now(),
    locked_at timestamptz,
    locked_by text not null default '',
    last_error text not null default '',
    created_at timestamptz not null default now(),
    updated_at timestamptz not null default now(),
    unique (account_id, idempotency_key)
);

create index if not exists intelligence_source_ingestion_jobs_claim_idx
    on intelligence.source_ingestion_jobs (status, run_after, created_at)
    where status = 'pending';
create index if not exists intelligence_source_ingestion_jobs_order_idx
    on intelligence.source_ingestion_jobs (account_id, ordering_key, created_at, id);

create table if not exists intelligence.source_observations (
    id uuid primary key default gen_random_uuid(),
    account_id uuid not null references core.accounts(id) on delete cascade,
    client_account_id uuid not null references core.accounts(id) on delete cascade,
    subject_id uuid,
    relationship_id uuid,
    ingestion_run_id uuid,
    source_config_id uuid not null,
    source_key text not null,
    source_entity_type text not null check (length(btrim(source_entity_type)) between 1 and 120),
    source_entity_id text not null check (length(btrim(source_entity_id)) between 1 and 500),
    idempotency_key text not null check (length(btrim(idempotency_key)) between 1 and 500),
    source_version text not null default '',
    source_occurred_at timestamptz,
    observed_at timestamptz not null default now(),
    payload_hash text not null check (length(payload_hash) between 32 and 128),
    snapshot_json jsonb,
    snapshot_ciphertext text,
    cipher_key_version text not null default '',
    sensitivity text not null default 'personal'
        check (sensitivity in ('public', 'internal', 'personal', 'sensitive', 'restricted')),
    classification text not null default 'customer_relationship',
    purpose_key text not null,
    retention_policy_version_id uuid,
    expires_at timestamptz,
    supersedes_observation_id uuid,
    created_at timestamptz not null default now(),
    unique (account_id, source_config_id, source_entity_type, source_entity_id, source_version, payload_hash),
    unique (account_id, source_config_id, idempotency_key),
    unique (account_id, id),
    constraint intelligence_observations_payload_ck check (
        (snapshot_json is not null and snapshot_ciphertext is null)
        or (snapshot_json is null and snapshot_ciphertext is not null)
    ),
    constraint intelligence_observations_snapshot_json_ck check (
        snapshot_json is null or jsonb_typeof(snapshot_json) = 'object'
    ),
    constraint intelligence_observations_run_fk
        foreign key (account_id, ingestion_run_id)
        references intelligence.source_ingestion_runs(account_id, id) on delete set null,
    constraint intelligence_observations_source_fk
        foreign key (account_id, source_config_id)
        references intelligence.source_configs(account_id, id) on delete restrict,
    constraint intelligence_observations_supersedes_fk
        foreign key (account_id, supersedes_observation_id)
        references intelligence.source_observations(account_id, id) on delete set null,
    constraint intelligence_observations_retention_fk
        foreign key (account_id, retention_policy_version_id)
        references intelligence.retention_policy_versions(account_id, id) on delete restrict
);

create index if not exists intelligence_observations_relationship_idx
    on intelligence.source_observations (account_id, client_account_id, relationship_id, observed_at desc, id);
create index if not exists intelligence_observations_expiry_idx
    on intelligence.source_observations (account_id, expires_at)
    where expires_at is not null;

create table if not exists intelligence.claims (
    id uuid primary key default gen_random_uuid(),
    account_id uuid not null references core.accounts(id) on delete cascade,
    client_account_id uuid not null references core.accounts(id) on delete cascade,
    subject_id uuid not null,
    relationship_id uuid not null,
    fact_definition_id uuid not null,
    fact_definition_version_id uuid not null,
    fact_key text not null,
    value_type text not null,
    value_normalized jsonb,
    value_ciphertext text,
    cipher_key_version text not null default '',
    value_fingerprint text not null default '',
    extraction_method text not null
        check (extraction_method in ('source_direct', 'rule', 'manual', 'llm')),
    extractor_key text not null default '',
    extractor_version text not null default '',
    prompt_binding_id uuid,
    runtime_run_id uuid,
    confidence numeric(6,5) not null default 1 check (confidence between 0 and 1),
    verification_state text not null default 'unverified'
        check (verification_state in ('unverified', 'verified', 'rejected', 'contested')),
    valid_from timestamptz,
    valid_until timestamptz,
    sensitivity text not null default 'personal'
        check (sensitivity in ('public', 'internal', 'personal', 'sensitive', 'restricted')),
    status text not null default 'candidate'
        check (status in ('candidate', 'accepted', 'superseded', 'invalidated', 'rejected')),
    superseded_by_claim_id uuid,
    created_by_user_id uuid references core.users(id) on delete set null,
    created_at timestamptz not null default now(),
    updated_at timestamptz not null default now(),
    unique (account_id, id),
    constraint intelligence_claims_value_ck check (
        (value_normalized is not null and value_ciphertext is null)
        or (value_normalized is null and value_ciphertext is not null)
    ),
    constraint intelligence_claims_definition_fk
        foreign key (account_id, fact_definition_id)
        references intelligence.fact_definitions(account_id, id) on delete restrict,
    constraint intelligence_claims_definition_version_fk
        foreign key (account_id, fact_definition_version_id)
        references intelligence.fact_definition_versions(account_id, id) on delete restrict,
    constraint intelligence_claims_superseded_fk
        foreign key (account_id, superseded_by_claim_id)
        references intelligence.claims(account_id, id) on delete set null
);

create index if not exists intelligence_claims_relationship_idx
    on intelligence.claims (account_id, client_account_id, relationship_id, fact_key, status, created_at desc);

create table if not exists intelligence.claim_evidence (
    account_id uuid not null references core.accounts(id) on delete cascade,
    claim_id uuid not null,
    observation_id uuid not null,
    role text not null check (role in ('supports', 'contradicts', 'context')),
    created_at timestamptz not null default now(),
    primary key (account_id, claim_id, observation_id),
    constraint intelligence_claim_evidence_claim_fk
        foreign key (account_id, claim_id)
        references intelligence.claims(account_id, id) on delete cascade,
    constraint intelligence_claim_evidence_observation_fk
        foreign key (account_id, observation_id)
        references intelligence.source_observations(account_id, id) on delete restrict
);

create table if not exists intelligence.facts (
    id uuid primary key default gen_random_uuid(),
    account_id uuid not null references core.accounts(id) on delete cascade,
    client_account_id uuid not null references core.accounts(id) on delete cascade,
    subject_id uuid not null,
    relationship_id uuid not null,
    fact_definition_id uuid not null,
    fact_definition_version_id uuid not null,
    fact_key text not null,
    version integer not null check (version > 0),
    value_type text not null,
    value_resolved jsonb,
    value_ciphertext text,
    cipher_key_version text not null default '',
    winning_claim_id uuid,
    authority_policy_version_id uuid,
    confidence numeric(6,5) not null default 1 check (confidence between 0 and 1),
    resolution_state text not null
        check (resolution_state in ('resolved', 'verified', 'contested', 'invalidated', 'superseded')),
    resolution_reason_code text not null,
    valid_from timestamptz,
    valid_until timestamptz,
    effective_at timestamptz not null default now(),
    superseded_by_fact_id uuid,
    resolved_by_user_id uuid references core.users(id) on delete set null,
    created_at timestamptz not null default now(),
    unique (account_id, relationship_id, fact_definition_id, version),
    unique (account_id, id),
    constraint intelligence_facts_value_ck check (
        (value_resolved is not null and value_ciphertext is null)
        or (value_resolved is null and value_ciphertext is not null)
    ),
    constraint intelligence_facts_definition_fk
        foreign key (account_id, fact_definition_id)
        references intelligence.fact_definitions(account_id, id) on delete restrict,
    constraint intelligence_facts_definition_version_fk
        foreign key (account_id, fact_definition_version_id)
        references intelligence.fact_definition_versions(account_id, id) on delete restrict,
    constraint intelligence_facts_winning_claim_fk
        foreign key (account_id, winning_claim_id)
        references intelligence.claims(account_id, id) on delete set null,
    constraint intelligence_facts_policy_fk
        foreign key (account_id, authority_policy_version_id)
        references intelligence.authority_policy_versions(account_id, id) on delete set null,
    constraint intelligence_facts_superseded_fk
        foreign key (account_id, superseded_by_fact_id)
        references intelligence.facts(account_id, id) on delete set null
);

create unique index if not exists intelligence_facts_current_uidx
    on intelligence.facts (account_id, relationship_id, fact_definition_id)
    where resolution_state in ('resolved', 'verified', 'contested');
create index if not exists intelligence_facts_timeline_idx
    on intelligence.facts (account_id, client_account_id, relationship_id, effective_at desc, id desc);

create table if not exists intelligence.fact_evidence (
    account_id uuid not null references core.accounts(id) on delete cascade,
    fact_id uuid not null,
    observation_id uuid not null,
    claim_id uuid,
    role text not null check (role in ('winning', 'supporting', 'conflicting')),
    created_at timestamptz not null default now(),
    primary key (account_id, fact_id, observation_id),
    constraint intelligence_fact_evidence_fact_fk
        foreign key (account_id, fact_id)
        references intelligence.facts(account_id, id) on delete cascade,
    constraint intelligence_fact_evidence_observation_fk
        foreign key (account_id, observation_id)
        references intelligence.source_observations(account_id, id) on delete restrict,
    constraint intelligence_fact_evidence_claim_fk
        foreign key (account_id, claim_id)
        references intelligence.claims(account_id, id) on delete set null
);

create table if not exists intelligence.summary_versions (
    id uuid primary key default gen_random_uuid(),
    account_id uuid not null references core.accounts(id) on delete cascade,
    client_account_id uuid not null references core.accounts(id) on delete cascade,
    subject_id uuid not null,
    relationship_id uuid not null,
    summary_type text not null
        check (summary_type in ('relationship_profile', 'handoff', 'conversation')),
    version integer not null check (version > 0),
    status text not null default 'draft'
        check (status in ('draft', 'published', 'superseded', 'invalidated')),
    content_ciphertext text not null,
    sections_ciphertext text not null default '',
    content_hash text not null,
    cipher_key_version text not null default 'v1',
    input_fingerprint text not null,
    as_of timestamptz not null,
    prompt_binding_id uuid,
    runtime_run_id uuid,
    confidence numeric(6,5) check (confidence between 0 and 1),
    expires_at timestamptz,
    superseded_by_summary_id uuid,
    created_at timestamptz not null default now(),
    published_at timestamptz,
    unique (account_id, relationship_id, summary_type, version),
    unique (account_id, id),
    constraint intelligence_summary_superseded_fk
        foreign key (account_id, superseded_by_summary_id)
        references intelligence.summary_versions(account_id, id) on delete set null
);

create unique index if not exists intelligence_summary_current_uidx
    on intelligence.summary_versions (account_id, relationship_id, summary_type)
    where status = 'published';

create table if not exists intelligence.summary_evidence (
    account_id uuid not null references core.accounts(id) on delete cascade,
    summary_id uuid not null,
    fact_id uuid not null,
    created_at timestamptz not null default now(),
    primary key (account_id, summary_id, fact_id),
    constraint intelligence_summary_evidence_summary_fk
        foreign key (account_id, summary_id)
        references intelligence.summary_versions(account_id, id) on delete cascade,
    constraint intelligence_summary_evidence_fact_fk
        foreign key (account_id, fact_id)
        references intelligence.facts(account_id, id) on delete restrict
);

create table if not exists intelligence.recommendations (
    id uuid primary key default gen_random_uuid(),
    account_id uuid not null references core.accounts(id) on delete cascade,
    client_account_id uuid not null references core.accounts(id) on delete cascade,
    subject_id uuid not null,
    relationship_id uuid not null,
    recommendation_type text not null
        check (recommendation_type in ('follow_up', 'offer', 'important_date', 'next_best_action')),
    status text not null default 'proposed'
        check (status in ('proposed', 'accepted', 'rejected', 'expired', 'superseded')),
    payload_json jsonb,
    payload_ciphertext text,
    cipher_key_version text not null default '',
    rationale_code text not null default '',
    confidence numeric(6,5) not null default 0 check (confidence between 0 and 1),
    valid_from timestamptz,
    expires_at timestamptz,
    prompt_binding_id uuid,
    runtime_run_id uuid,
    context_snapshot_id uuid,
    feedback_code text not null default '',
    reviewed_by_user_id uuid references core.users(id) on delete set null,
    reviewed_at timestamptz,
    created_at timestamptz not null default now(),
    updated_at timestamptz not null default now(),
    unique (account_id, id),
    constraint intelligence_recommendations_payload_ck check (
        (payload_json is not null and payload_ciphertext is null)
        or (payload_json is null and payload_ciphertext is not null)
    )
);

create index if not exists intelligence_recommendations_relationship_idx
    on intelligence.recommendations (account_id, client_account_id, relationship_id, status, created_at desc);

create table if not exists intelligence.context_snapshots (
    id uuid primary key default gen_random_uuid(),
    account_id uuid not null references core.accounts(id) on delete cascade,
    client_account_id uuid not null references core.accounts(id) on delete cascade,
    subject_id uuid,
    relationship_id uuid,
    conversation_id uuid,
    process_keys jsonb not null default '[]'::jsonb
        check (jsonb_typeof(process_keys) = 'array'),
    purpose_key text not null,
    as_of timestamptz not null,
    payload_ciphertext text not null,
    cipher_key_version text not null default 'v1',
    payload_hash text not null,
    prompt_binding_id uuid,
    item_count integer not null default 0 check (item_count >= 0),
    token_estimate integer not null default 0 check (token_estimate >= 0),
    omission_codes jsonb not null default '[]'::jsonb
        check (jsonb_typeof(omission_codes) = 'array'),
    expires_at timestamptz not null,
    created_at timestamptz not null default now(),
    unique (account_id, id)
);

create index if not exists intelligence_context_snapshots_expiry_idx
    on intelligence.context_snapshots (account_id, expires_at);

create table if not exists intelligence.source_suggestions (
    id uuid primary key default gen_random_uuid(),
    account_id uuid not null references core.accounts(id) on delete cascade,
    client_account_id uuid not null references core.accounts(id) on delete cascade,
    relationship_id uuid,
    source_key text not null,
    gap_codes jsonb not null default '[]'::jsonb
        check (jsonb_typeof(gap_codes) = 'array'),
    rationale_code text not null,
    confidence numeric(6,5) not null default 0 check (confidence between 0 and 1),
    status text not null default 'proposed'
        check (status in ('proposed', 'accepted', 'rejected', 'expired')),
    runtime_run_id uuid,
    reviewed_by_user_id uuid references core.users(id) on delete set null,
    reviewed_at timestamptz,
    created_at timestamptz not null default now(),
    unique (account_id, id)
);

create table if not exists intelligence.accepted_outcomes (
    id uuid primary key default gen_random_uuid(),
    event_id uuid not null,
    account_id uuid not null references core.accounts(id) on delete cascade,
    client_account_id uuid not null references core.accounts(id) on delete cascade,
    interaction_id text not null,
    decision_id text not null,
    conversation_id uuid,
    subject_id uuid,
    relationship_id uuid,
    outcome text not null check (outcome in ('reply', 'handoff', 'no_reply')),
    reason_code text not null,
    process_run_refs jsonb not null default '[]'::jsonb
        check (jsonb_typeof(process_run_refs) = 'array'),
    occurred_at timestamptz not null,
    created_at timestamptz not null default now(),
    unique (account_id, event_id),
    unique (account_id, decision_id)
);

create table if not exists intelligence.portfolio_opportunities (
    id uuid primary key default gen_random_uuid(),
    account_id uuid not null references core.accounts(id) on delete cascade,
    target_client_account_id uuid not null references core.accounts(id) on delete cascade,
    organization_id uuid,
    segment_key text not null,
    cohort_size integer not null check (cohort_size > 0),
    suppression_threshold integer not null check (suppression_threshold > 0),
    aggregate_metrics jsonb not null
        check (jsonb_typeof(aggregate_metrics) = 'object'),
    opportunity_type text not null,
    rationale_code text not null,
    confidence numeric(6,5) not null default 0 check (confidence between 0 and 1),
    status text not null default 'proposed'
        check (status in ('proposed', 'accepted', 'rejected', 'expired')),
    expires_at timestamptz,
    runtime_run_id uuid,
    reviewed_by_user_id uuid references core.users(id) on delete set null,
    reviewed_at timestamptz,
    created_at timestamptz not null default now(),
    unique (account_id, id),
    constraint intelligence_portfolio_suppression_ck
        check (cohort_size >= suppression_threshold)
);

create index if not exists intelligence_portfolio_target_idx
    on intelligence.portfolio_opportunities (account_id, target_client_account_id, status, created_at desc);

create table if not exists intelligence.audit_events (
    id uuid primary key default gen_random_uuid(),
    account_id uuid not null references core.accounts(id) on delete cascade,
    client_account_id uuid,
    actor_user_id uuid references core.users(id) on delete set null,
    event_type text not null,
    aggregate_type text not null,
    aggregate_id text not null,
    correlation_id text not null default '',
    reason_code text not null default '',
    metadata jsonb not null default '{}'::jsonb
        check (jsonb_typeof(metadata) = 'object'),
    occurred_at timestamptz not null default now()
);

create index if not exists intelligence_audit_events_account_idx
    on intelligence.audit_events (account_id, occurred_at desc, id desc);

-- Auditoria administrativa transacional. O trigger persiste somente metadados
-- allowlisted; valores, snapshots, prompts e segredos nunca entram no audit log.
create or replace function intelligence.audit_admin_mutation()
returns trigger
language plpgsql
as $$
declare
    row_data jsonb;
    actor_text text;
    client_text text;
    entity_id text;
    audit_metadata jsonb;
begin
    if tg_op = 'DELETE' then
        row_data := to_jsonb(old);
    else
        row_data := to_jsonb(new);
    end if;

    -- Catálogos platform-wide sem account_id usam auditoria de plataforma,
    -- fora deste log tenant-scoped.
    if nullif(row_data ->> 'account_id', '') is null then
        if tg_op = 'DELETE' then
            return old;
        end if;
        return new;
    end if;

    actor_text := coalesce(
        nullif(row_data ->> 'updated_by_user_id', ''),
        nullif(row_data ->> 'published_by_user_id', ''),
        nullif(row_data ->> 'validated_by_user_id', ''),
        nullif(row_data ->> 'reviewed_by_user_id', ''),
        nullif(row_data ->> 'created_by_user_id', '')
    );
    client_text := nullif(row_data ->> 'client_account_id', '');
    entity_id := coalesce(nullif(row_data ->> 'id', ''), 'unknown');
    audit_metadata := jsonb_strip_nulls(jsonb_build_object(
        'revision', row_data -> 'revision',
        'status', row_data -> 'status',
        'mode', row_data -> 'mode',
        'key', coalesce(
            row_data -> 'capability_key',
            row_data -> 'source_key',
            row_data -> 'fact_key',
            row_data -> 'policy_key',
            row_data -> 'process_key',
            row_data -> 'slug'
        ),
        'version', row_data -> 'version'
    ));

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
        (row_data ->> 'account_id')::uuid,
        client_text::uuid,
        actor_text::uuid,
        tg_table_schema || '.' || tg_table_name || '.' || lower(tg_op),
        tg_table_name,
        entity_id,
        'administrative_mutation',
        audit_metadata
    );
    if tg_op = 'DELETE' then
        return old;
    end if;
    return new;
end;
$$;

do $$
declare
    table_name text;
    trigger_name text;
begin
    foreach table_name in array array[
        'capabilities',
        'fact_definitions',
        'fact_definition_versions',
        'retention_policy_versions',
        'authority_policy_versions',
        'authority_policy_bindings',
        'source_configs',
        'recommendations',
        'portfolio_opportunities'
    ]
    loop
        trigger_name := 'audit_' || table_name || '_mutation';
        if not exists (
            select 1
            from pg_trigger
            where tgname = trigger_name
              and tgrelid = format('intelligence.%I', table_name)::regclass
              and not tgisinternal
        ) then
            execute format(
                'create trigger %I after insert or update or delete on intelligence.%I '
                'for each row execute function intelligence.audit_admin_mutation()',
                trigger_name,
                table_name
            );
        end if;
    end loop;
end $$;
