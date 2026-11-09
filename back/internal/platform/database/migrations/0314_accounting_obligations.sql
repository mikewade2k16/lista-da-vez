-- CT-11: structure only; no fiscal rule or rate is seeded/activated.
create table if not exists accounting.obligation_rules (
 id uuid primary key default gen_random_uuid(), account_id uuid not null,issuer_id uuid not null,
 code text not null,version bigint not null check(version>0),regime text not null,jurisdiction text not null,
 valid_from date not null,valid_to date not null,parameters jsonb not null,
 source_ref text not null,technical_review_ref text not null,reviewed_by uuid not null references core.users(id),
 created_at timestamptz not null default now(),
 check(valid_to>=valid_from),check(jsonb_typeof(parameters)='object'),
 foreign key(account_id,issuer_id) references accounting.issuers(account_id,id),
 unique(account_id,issuer_id,id),unique(account_id,issuer_id,code,version)
);
create index if not exists accounting_obligation_rules_scope_idx on accounting.obligation_rules(account_id,issuer_id,code,version desc);
create table if not exists accounting.obligations (
 id uuid primary key default gen_random_uuid(),account_id uuid not null,issuer_id uuid not null,
 code text not null,period text not null,version bigint not null default 1,profile_version bigint not null,
 state text not null check(state in ('pending_data','calculated','reviewed','payment_reported')),
 base_amount numeric(20,2),amount numeric(20,2),due_date date,rule_id uuid,
 pending jsonb not null default '[]',memory jsonb not null default '{}',
 reviewed_by uuid references core.users(id),payment_date date,payment_ref text,
 created_at timestamptz not null default now(),updated_at timestamptz not null default now(),
 foreign key(account_id,issuer_id) references accounting.issuers(account_id,id),
 foreign key(account_id,issuer_id,rule_id) references accounting.obligation_rules(account_id,issuer_id,id),
 unique(account_id,issuer_id,id),unique(account_id,issuer_id,code,period),
 check(amount is null or amount>=0),check(base_amount is null or base_amount>=0)
);
create index if not exists accounting_obligations_scope_idx on accounting.obligations(account_id,issuer_id,period,id);
create table if not exists accounting.obligation_events (
 id uuid primary key default gen_random_uuid(),account_id uuid not null,issuer_id uuid not null,obligation_id uuid not null,
 version bigint not null,actor_id uuid not null references core.users(id),operation text not null,snapshot jsonb not null,
 created_at timestamptz not null default now(),
 foreign key(account_id,issuer_id,obligation_id) references accounting.obligations(account_id,issuer_id,id),
 unique(account_id,issuer_id,obligation_id,version)
);
-- Finance owns its projection. The logical line reference survives full-replace
-- autosave; a missing/deleted/modified line is a conflict, never silently recreated.
create table if not exists finance.obligation_links (
 account_id uuid not null references core.accounts(id),issuer_id uuid not null,obligation_id uuid not null,
 obligation_version bigint not null,sheet_id uuid not null,line_id uuid not null,
 amount numeric(14,2) not null,created_by uuid not null references core.users(id),created_at timestamptz not null default now(),
 primary key(account_id,issuer_id,obligation_id),unique(account_id,sheet_id,line_id),
 foreign key(account_id,issuer_id,obligation_id) references accounting.obligations(account_id,issuer_id,id)
);
