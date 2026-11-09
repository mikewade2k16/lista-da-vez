-- CT-12: opt-in monitoring only. No source subscription or publication seeded.
create table if not exists messaging.knowledge_source_subscriptions (
 id uuid primary key default gen_random_uuid(),account_id uuid not null,issuer_id uuid not null,base_id uuid not null,
 actor_id uuid not null references core.users(id),source_id text not null,source_revision text not null,
 version bigint not null default 1,enabled boolean not null default false,interval_hours integer not null check(interval_hours between 6 and 168),
 next_check_at timestamptz not null default now(),last_checksum text,last_document_id uuid,
 created_at timestamptz not null default now(),updated_at timestamptz not null default now(),
 foreign key(account_id,issuer_id,base_id) references accounting.knowledge_bindings(account_id,issuer_id,knowledge_base_id),
 foreign key(account_id,last_document_id) references messaging.knowledge_documents(account_id,id),
 unique(account_id,id),unique(account_id,issuer_id,base_id,source_id)
);
create index if not exists knowledge_source_due_idx on messaging.knowledge_source_subscriptions(next_check_at,id) where enabled;
create table if not exists messaging.knowledge_ingestion_runs (
 id uuid primary key default gen_random_uuid(),account_id uuid not null,issuer_id uuid not null,base_id uuid not null,
 subscription_id uuid not null,subscription_version bigint not null,actor_id uuid not null references core.users(id),
 source_id text not null,source_revision text not null,state text not null check(state in ('queued','fetching','retry_wait','review_pending','unchanged','failed','cancelled')),
 attempts integer not null default 0,recoveries integer not null default 0,fence bigint not null default 0,lease_until timestamptz,
 document_id uuid,checksum text,error_code text,created_at timestamptz not null default now(),updated_at timestamptz not null default now(),
 foreign key(account_id,subscription_id) references messaging.knowledge_source_subscriptions(account_id,id),
 foreign key(account_id,document_id) references messaging.knowledge_documents(account_id,id),unique(account_id,id)
);
create unique index if not exists knowledge_ingestion_active_uidx on messaging.knowledge_ingestion_runs(account_id,subscription_id) where state in ('queued','fetching','retry_wait');
create index if not exists knowledge_ingestion_scope_idx on messaging.knowledge_ingestion_runs(account_id,issuer_id,created_at desc,id);
create index if not exists knowledge_ingestion_stale_idx on messaging.knowledge_ingestion_runs(lease_until,id) where state='fetching';
create table if not exists messaging.knowledge_ingestion_receipts (
 account_id uuid not null,issuer_id uuid not null,actor_id uuid not null,operation text not null,request_key text not null,
 request_hash text not null,result jsonb not null,created_at timestamptz not null default now(),
 primary key(account_id,issuer_id,actor_id,operation,request_key),
 foreign key(account_id,issuer_id) references accounting.issuers(account_id,id),foreign key(actor_id) references core.users(id)
);
