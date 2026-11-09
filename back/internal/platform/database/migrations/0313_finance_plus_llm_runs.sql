-- CT-07: usage ledger only. No prompts, answers, secrets or duplicate evidence bodies.
create table if not exists finance.plus_llm_runs (
 id uuid primary key default gen_random_uuid(),
 account_id uuid not null references core.accounts(id),
 actor_id uuid not null references core.users(id),
 issuer_id uuid not null,
 request_id uuid not null,
 provider text not null,
 model text not null,
 task_class text not null,
 prompt_version integer not null,
 policy_version integer not null,
 config_version text not null,
 evidence_refs jsonb not null default '[]'::jsonb check(jsonb_typeof(evidence_refs)='array'),
 status text not null default 'reserved',
 prompt_tokens integer,
 completion_tokens integer,
 total_tokens integer,
 latency_ms integer,
 cost_usd numeric(20,6),
 reserved_cost_usd numeric(20,6),
 pricing_version text,
 created_at timestamptz not null default now(),
 completed_at timestamptz,
 expires_at timestamptz not null default now()+interval '30 days',
 unique(account_id,actor_id,request_id)
);
create index if not exists finance_plus_llm_runs_budget_idx on finance.plus_llm_runs(account_id,created_at desc);
create index if not exists finance_plus_llm_runs_retention_idx on finance.plus_llm_runs(expires_at);
alter table finance.plus_llm_runs add column if not exists schema_version integer not null default 1;
alter table finance.plus_llm_runs add column if not exists pricing_snapshot jsonb;
