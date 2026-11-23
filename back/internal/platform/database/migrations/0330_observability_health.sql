-- Observabilidade, fase "saúde do sistema": amostras de runtime/pool/Postgres,
-- containers e disco, presença de usuários, alertas e status de segurança.
-- Só cria tabelas novas no schema obs (aditiva). Telemetria, sem dado de negócio.
-- Reversão: observability/migrations/0330_observability_health.down.sql.

-- Amostra periódica do coletor da API (source = 'api': runtime Go, pool pgx,
-- Postgres, filas de jobs) e do agente de host (source = 'agent': disco, docker).
create table if not exists obs.system_samples (
  id bigserial primary key,
  occurred_at timestamptz not null default now(),
  source text not null check (source in ('api', 'agent')),
  host text not null default '',
  metrics jsonb not null default '{}'::jsonb
);
create index if not exists system_samples_source_idx on obs.system_samples (source, occurred_at desc);
create index if not exists system_samples_occurred_at_idx on obs.system_samples (occurred_at);

-- Estado de cada container a cada coleta do agente.
create table if not exists obs.container_samples (
  id bigserial primary key,
  occurred_at timestamptz not null default now(),
  host text not null default '',
  container text not null,
  service text not null default '',
  state text not null default '',
  health text not null default '',
  restart_count integer not null default 0,
  -- politica de restart do Docker e exit code: container com restart 'no' parado
  -- de proposito (one-shot, teste antigo) nao e "fora do ar".
  restart_policy text not null default '',
  exit_code integer,
  started_at timestamptz,
  cpu_pct double precision,
  mem_bytes bigint,
  mem_limit_bytes bigint
);
create index if not exists container_samples_container_idx on obs.container_samples (container, occurred_at desc);
create index if not exists container_samples_occurred_at_idx on obs.container_samples (occurred_at);

-- Presença: uma linha por aba/pageview, atualizada pelo heartbeat do front.
create table if not exists obs.presence (
  page_view_id text primary key,
  user_id text not null,
  page text not null default '',
  started_at timestamptz not null default now(),
  last_seen timestamptz not null default now(),
  user_agent text not null default ''
);
create index if not exists presence_last_seen_idx on obs.presence (last_seen desc);

-- Alertas avaliados pela API. Um alerta aberto por fingerprint (regra + alvo).
create table if not exists obs.alerts (
  id bigserial primary key,
  rule text not null,
  fingerprint text not null,
  severity text not null check (severity in ('critical', 'high', 'medium', 'low')),
  area text not null,
  title text not null,
  detail jsonb not null default '{}'::jsonb,
  status text not null default 'open' check (status in ('open', 'resolved')),
  first_seen timestamptz not null default now(),
  last_seen timestamptz not null default now(),
  resolved_at timestamptz,
  notified_at timestamptz,
  notify_count integer not null default 0,
  last_notify_error text
);
create unique index if not exists alerts_open_fingerprint_idx on obs.alerts (fingerprint) where status = 'open';
create index if not exists alerts_last_seen_idx on obs.alerts (last_seen desc);

-- Catálogo de ataques/controles e o resultado do último teste de cada um.
-- 'not_tested' é estado legítimo e deve aparecer no painel.
create table if not exists obs.security_checks (
  key text primary key,
  category text not null,
  title text not null,
  standard text not null default '',
  status text not null default 'not_tested' check (status in ('pass', 'fail', 'partial', 'not_tested')),
  detail text not null default '',
  evidence jsonb not null default '{}'::jsonb,
  source text not null default 'catalog',
  checked_at timestamptz,
  updated_at timestamptz not null default now()
);

-- Achados da auditoria (observability/reports/AUDITORIA.md) com status.
create table if not exists obs.findings (
  id text primary key,
  severity text not null check (severity in ('critical', 'high', 'medium', 'low')),
  title text not null,
  area text not null default '',
  location text not null default '',
  status text not null default 'open' check (status in ('open', 'fixed', 'accepted')),
  source text not null default 'auditoria',
  updated_at timestamptz not null default now()
);

-- Histórico de deploys (scripts/deploy): tempo de ponta a ponta medido na máquina
-- que disparou (relógio monotônico), do início do launcher até os smoke tests na VPS.
-- id = nome da execução em .tmp/deploy/<id> (idempotente na importação).
create table if not exists obs.deploys (
  id text primary key,
  command text not null,
  environment text not null default '',
  started_at timestamptz not null,
  finished_at timestamptz,
  duration_ms double precision,
  exit_code integer,
  status text not null default 'unknown' check (status in ('success', 'failed', 'interrupted', 'unknown')),
  git_sha text not null default '',
  git_branch text not null default '',
  image_tag text not null default '',
  triggered_by text not null default '',
  stages jsonb not null default '[]'::jsonb,
  recorded_at timestamptz not null default now()
);
create index if not exists deploys_started_at_idx on obs.deploys (started_at desc);
