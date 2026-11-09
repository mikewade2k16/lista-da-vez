-- CT-03: additive foundation; no runtime connector or automatic module activation.
create schema if not exists accounting;
create table if not exists accounting.issuers (
 id uuid primary key default gen_random_uuid(),
 account_id uuid not null references core.accounts(id),
 status text not null default 'active' check(status in ('active','inactive')),
 version bigint not null default 1, access_version bigint not null default 1,
 created_at timestamptz not null default now(), updated_at timestamptz not null default now(),
 unique(account_id,id)
);
create index if not exists accounting_issuers_status_idx on accounting.issuers(account_id,status,id);
create table if not exists accounting.issuer_members (
 id uuid primary key default gen_random_uuid(), account_id uuid not null references core.accounts(id),
 issuer_id uuid not null, user_id uuid not null references core.users(id),
 grants text[] not null, status text not null default 'active' check(status in ('active','inactive')),
 version bigint not null default 1, created_at timestamptz not null default now(), updated_at timestamptz not null default now(),
 unique(account_id,issuer_id,user_id),
 foreign key(account_id,issuer_id) references accounting.issuers(account_id,id)
);
create index if not exists accounting_members_actor_idx on accounting.issuer_members(account_id,user_id,status);
create table if not exists accounting.issuer_profiles (
 id uuid primary key default gen_random_uuid(), account_id uuid not null references core.accounts(id), issuer_id uuid not null,
 version bigint not null, data jsonb not null, valid_from date, valid_to date,
 created_by uuid not null references core.users(id), created_at timestamptz not null default now(),
 unique(account_id,issuer_id,version), foreign key(account_id,issuer_id) references accounting.issuers(account_id,id),
 check(valid_to is null or valid_from < valid_to)
);
create table if not exists accounting.diagnoses (
 id uuid primary key default gen_random_uuid(), account_id uuid not null references core.accounts(id), issuer_id uuid not null,
 version bigint not null default 1, profile_version bigint not null, intent text not null check(intent in ('service','product','mixed')),
 competence_date date, answers jsonb not null, pending jsonb not null,
 created_at timestamptz not null default now(), updated_at timestamptz not null default now(),
 unique(account_id,issuer_id,id), foreign key(account_id,issuer_id,profile_version) references accounting.issuer_profiles(account_id,issuer_id,version)
);
create index if not exists accounting_diagnoses_updated_idx on accounting.diagnoses(account_id,issuer_id,updated_at desc,id);
create table if not exists accounting.documents (
 id uuid primary key default gen_random_uuid(), account_id uuid not null references core.accounts(id), issuer_id uuid not null,
 version bigint not null default 1, profile_version bigint not null, diagnosis_id uuid not null, diagnosis_version bigint not null,
 content_version bigint not null default 1, validated_connector text not null default '', validated_environment text not null default '',
 kind text not null check(kind in ('nfse','nfe')), state text not null default 'draft'
 check(state in ('draft','validated','reviewed','authorized_for_submission','submitting','authorized','rejected','unknown')),
 payload jsonb not null, amount numeric(20,2), source_ref jsonb, deleted_at timestamptz,
 created_at timestamptz not null default now(), updated_at timestamptz not null default now(),
 unique(account_id,issuer_id,id),
 foreign key(account_id,issuer_id,diagnosis_id) references accounting.diagnoses(account_id,issuer_id,id),
 foreign key(account_id,issuer_id,profile_version) references accounting.issuer_profiles(account_id,issuer_id,version),
 check(amount is null or amount>=0)
);
create index if not exists accounting_documents_state_idx on accounting.documents(account_id,issuer_id,state,updated_at desc,id);
create table if not exists accounting.document_reviews (
 id uuid primary key default gen_random_uuid(), account_id uuid not null references core.accounts(id), issuer_id uuid not null, document_id uuid not null,
 document_version bigint not null, profile_version bigint not null, snapshot jsonb not null, snapshot_hash text not null,
 actor_id uuid not null references core.users(id), revoked_at timestamptz, created_at timestamptz not null default now(),
 unique(account_id,issuer_id,id), unique(account_id,issuer_id,document_id,id), unique(account_id,issuer_id,document_id,snapshot_hash,actor_id),
 foreign key(account_id,issuer_id,document_id) references accounting.documents(account_id,issuer_id,id)
);
create table if not exists accounting.submission_authorizations (
 id uuid primary key default gen_random_uuid(), account_id uuid not null references core.accounts(id), issuer_id uuid not null, document_id uuid not null,
 review_id uuid not null, actor_id uuid not null references core.users(id), snapshot_hash text not null,
 connector_id text not null, environment text not null check(environment in ('fake_local','homologation','production')),
 access_version bigint not null, version bigint not null default 1, expires_at timestamptz not null,
 revoked_at timestamptz, consumed_at timestamptz, created_at timestamptz not null default now(),
 unique(account_id,issuer_id,id), unique(account_id,issuer_id,document_id,id),
 foreign key(account_id,issuer_id,document_id) references accounting.documents(account_id,issuer_id,id),
 foreign key(account_id,issuer_id,document_id,review_id) references accounting.document_reviews(account_id,issuer_id,document_id,id)
);
create index if not exists accounting_authorizations_document_idx on accounting.submission_authorizations(account_id,issuer_id,document_id,expires_at);
create table if not exists accounting.executions (
 id uuid primary key default gen_random_uuid(), account_id uuid not null references core.accounts(id), issuer_id uuid not null, document_id uuid not null,
 authorization_id uuid not null, actor_id uuid not null references core.users(id), job_id uuid,
 state text not null default 'queued' check(state in ('queued','processing','unknown','authorized','rejected','cancelled')),
 operation text not null check(operation='issue'), environment text not null, connector_id text not null,
 request_hash text not null, business_key text not null, remote_ref text, protocol text, safe_error_code text,
 remote_identity text not null default '', lease_owner text not null default '', lease_until timestamptz, fence bigint not null default 0,
 version bigint not null default 1, artifacts_pending boolean not null default false,
 created_at timestamptz not null default now(), updated_at timestamptz not null default now(),
 unique(account_id,issuer_id,id), unique(account_id,issuer_id,operation,environment,business_key),
 foreign key(account_id,issuer_id,document_id) references accounting.documents(account_id,issuer_id,id),
 foreign key(account_id,issuer_id,document_id,authorization_id) references accounting.submission_authorizations(account_id,issuer_id,document_id,id)
);
create unique index if not exists accounting_execution_active_idx on accounting.executions(account_id,issuer_id,document_id)
 where state in ('queued','processing','unknown');
create index if not exists accounting_execution_lease_idx on accounting.executions(account_id,state,lease_until);
create table if not exists accounting.execution_events (
 id uuid primary key default gen_random_uuid(), account_id uuid not null references core.accounts(id), issuer_id uuid not null, execution_id uuid not null,
 sequence bigint not null, event_code text not null, actor_id uuid references core.users(id), metadata jsonb not null default '{}',
 created_at timestamptz not null default now(), unique(account_id,issuer_id,execution_id,sequence),
 foreign key(account_id,issuer_id,execution_id) references accounting.executions(account_id,issuer_id,id)
);
create table if not exists accounting.artifacts (
 id uuid primary key default gen_random_uuid(), account_id uuid not null references core.accounts(id), issuer_id uuid not null, execution_id uuid not null,
 storage_object_id uuid not null, kind text not null, sha256 text not null, size bigint not null, protocol text,
 created_at timestamptz not null default now(), unique(account_id,issuer_id,execution_id,kind,sha256),
 foreign key(account_id,issuer_id,execution_id) references accounting.executions(account_id,issuer_id,id)
);
create table if not exists accounting.request_receipts (
 id uuid primary key default gen_random_uuid(), account_id uuid not null references core.accounts(id), issuer_id uuid,
 actor_id uuid not null references core.users(id), operation text not null, idempotency_key text not null,
 request_hash text not null, result jsonb not null, created_at timestamptz not null default now(),
 foreign key(account_id,issuer_id) references accounting.issuers(account_id,id)
);
create unique index if not exists accounting_receipt_key_idx on accounting.request_receipts(account_id,actor_id,coalesce(issuer_id,'00000000-0000-0000-0000-000000000000'::uuid),operation,idempotency_key);
create table if not exists accounting.outbox (
 id uuid primary key default gen_random_uuid(), account_id uuid not null references core.accounts(id),
 ordering_key text not null, idempotency_key text not null, kind text not null, payload jsonb not null,
 status text not null default 'pending' check(status in ('pending','processing','done','failed','dead')),
 attempts int not null default 0, max_attempts int not null default 3, run_after timestamptz not null default now(),
 locked_at timestamptz, locked_by text not null default '', last_error text not null default '',
 created_at timestamptz not null default now(), updated_at timestamptz not null default now(), unique(account_id,idempotency_key)
);
create index if not exists accounting_outbox_claim_idx on accounting.outbox(status,run_after,created_at,id);
create index if not exists accounting_outbox_fifo_idx on accounting.outbox(account_id,ordering_key,status,created_at,id);
