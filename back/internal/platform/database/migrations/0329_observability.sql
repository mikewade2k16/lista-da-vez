-- Observabilidade (feat/observabilidade): armazenamento próprio no schema obs.
-- Dados de telemetria, sem dado de negócio nem de tenant além de ids opacos.
-- Reversão: observability/migrations/0329_observability.down.sql (fora do glob do migrator).
create schema if not exists obs;

create or replace function obs.add_bigint_arrays(a bigint[], b bigint[])
returns bigint[]
language sql
immutable
as $$
  select coalesce(array_agg(coalesce(x, 0) + coalesce(y, 0) order by ord), '{}'::bigint[])
  from unnest(a, b) with ordinality as t(x, y, ord)
$$;

create table if not exists obs.http_requests (
  id bigserial primary key,
  occurred_at timestamptz not null default now(),
  correlation_id text not null,
  method text not null,
  route text not null,
  path_module text not null default '',
  status integer not null,
  duration_ms double precision not null,
  response_bytes bigint not null default 0,
  user_id text,
  account_id text,
  query_count integer not null default 0,
  query_ms double precision not null default 0,
  external_count integer not null default 0,
  error_code text,
  error_message text,
  error_stack text,
  source text not null default 'live' check (source in ('live', 'runner'))
);
create index if not exists http_requests_occurred_at_idx on obs.http_requests (occurred_at);
create index if not exists http_requests_route_idx on obs.http_requests (route, occurred_at desc);
create index if not exists http_requests_correlation_idx on obs.http_requests (correlation_id);
create index if not exists http_requests_errors_idx on obs.http_requests (occurred_at desc) where status >= 500;

create table if not exists obs.http_route_hourly (
  bucket timestamptz not null,
  route text not null,
  path_module text not null default '',
  calls bigint not null default 0,
  errors_4xx bigint not null default 0,
  errors_5xx bigint not null default 0,
  total_ms double precision not null default 0,
  max_ms double precision not null default 0,
  queries_total bigint not null default 0,
  queries_max integer not null default 0,
  buckets bigint[] not null default '{}',
  primary key (bucket, route)
);
create index if not exists http_route_hourly_route_idx on obs.http_route_hourly (route, bucket desc);

create table if not exists obs.db_query_stats (
  bucket timestamptz not null,
  fingerprint text not null,
  route text not null,
  statement text not null,
  verb text not null default '',
  calls bigint not null default 0,
  errors bigint not null default 0,
  total_ms double precision not null default 0,
  max_ms double precision not null default 0,
  primary key (bucket, fingerprint, route)
);
create index if not exists db_query_stats_fingerprint_idx on obs.db_query_stats (fingerprint, bucket desc);

create table if not exists obs.db_slow_queries (
  id bigserial primary key,
  occurred_at timestamptz not null default now(),
  correlation_id text,
  route text not null,
  fingerprint text not null,
  statement text not null,
  duration_ms double precision not null,
  error text
);
create index if not exists db_slow_queries_occurred_at_idx on obs.db_slow_queries (occurred_at);
create index if not exists db_slow_queries_correlation_idx on obs.db_slow_queries (correlation_id);

create table if not exists obs.db_n_plus_one (
  id bigserial primary key,
  occurred_at timestamptz not null default now(),
  correlation_id text not null,
  route text not null,
  fingerprint text not null,
  statement text not null,
  repetitions integer not null
);
create index if not exists db_n_plus_one_occurred_at_idx on obs.db_n_plus_one (occurred_at);
create index if not exists db_n_plus_one_correlation_idx on obs.db_n_plus_one (correlation_id);

create table if not exists obs.external_calls (
  id bigserial primary key,
  occurred_at timestamptz not null default now(),
  correlation_id text,
  route text not null,
  integration text not null,
  method text not null,
  path text not null,
  status integer not null default 0,
  duration_ms double precision not null,
  timeout boolean not null default false,
  error text
);
create index if not exists external_calls_occurred_at_idx on obs.external_calls (occurred_at);
create index if not exists external_calls_correlation_idx on obs.external_calls (correlation_id);

create table if not exists obs.frontend_events (
  id bigserial primary key,
  occurred_at timestamptz not null default now(),
  page_view_id text not null,
  page text not null,
  kind text not null check (kind in ('vital', 'pageload', 'js_error', 'api_call', 'api_failure')),
  name text not null default '',
  value double precision,
  status integer,
  correlation_id text,
  detail jsonb,
  user_id text,
  user_agent text
);
create index if not exists frontend_events_occurred_at_idx on obs.frontend_events (occurred_at);
create index if not exists frontend_events_page_idx on obs.frontend_events (page, kind, occurred_at desc);
create index if not exists frontend_events_correlation_idx on obs.frontend_events (correlation_id);
create index if not exists frontend_events_page_view_idx on obs.frontend_events (page_view_id);

create table if not exists obs.inventory_items (
  kind text not null check (kind in ('endpoint', 'page', 'table', 'job', 'integration', 'module')),
  key text not null,
  module text not null default '',
  data jsonb not null default '{}'::jsonb,
  updated_at timestamptz not null default now(),
  primary key (kind, key)
);

create table if not exists obs.runs (
  id text primary key,
  started_at timestamptz not null,
  finished_at timestamptz,
  git_sha text,
  branch text,
  env text not null default '',
  base_url text not null default '',
  filters jsonb not null default '{}'::jsonb,
  summary jsonb not null default '{}'::jsonb,
  coverage jsonb not null default '{}'::jsonb,
  scores jsonb not null default '{}'::jsonb,
  regressions jsonb not null default '[]'::jsonb,
  exit_code integer
);
create index if not exists runs_started_at_idx on obs.runs (started_at desc);

create table if not exists obs.run_results (
  id bigserial primary key,
  run_id text not null references obs.runs (id) on delete cascade,
  kind text not null check (kind in ('api', 'page', 'db', 'load', 'health', 'authz')),
  module text not null default '',
  target text not null,
  status text not null check (status in ('pass', 'fail', 'warn', 'skip')),
  reason text,
  metrics jsonb not null default '{}'::jsonb
);
create index if not exists run_results_run_idx on obs.run_results (run_id, kind);
create index if not exists run_results_target_idx on obs.run_results (kind, target);

create table if not exists obs.run_requests (
  id bigserial primary key,
  requested_at timestamptz not null default now(),
  requested_by text not null,
  module text,
  only_kinds text,
  status text not null default 'queued' check (status in ('queued', 'running', 'done', 'failed')),
  run_id text references obs.runs (id) on delete set null,
  started_at timestamptz,
  finished_at timestamptz,
  log text
);
create index if not exists run_requests_status_idx on obs.run_requests (status, requested_at);
create index if not exists run_requests_run_idx on obs.run_requests (run_id);

create table if not exists obs.workers (
  id text primary key,
  host text not null default '',
  last_seen timestamptz not null default now()
);
