-- Observabilidade, etapa 3 (banco): estatísticas por tabela ao longo do tempo e
-- planos de execução das consultas lentas. Aditiva. Reversão:
-- observability/migrations/0334_obs_db_insights.down.sql.

-- Uma linha por tabela por hora (coletor da API, só tabelas de usuário).
create table if not exists obs.table_stats (
  bucket timestamptz not null,
  schema_name text not null,
  table_name text not null,
  total_bytes bigint not null default 0,
  table_bytes bigint not null default 0,
  index_bytes bigint not null default 0,
  live_tuples bigint not null default 0,
  dead_tuples bigint not null default 0,
  mods_since_analyze bigint not null default 0,
  seq_scan bigint not null default 0,
  seq_tup_read bigint not null default 0,
  idx_scan bigint not null default 0,
  last_vacuum timestamptz,
  last_autovacuum timestamptz,
  last_analyze timestamptz,
  last_autoanalyze timestamptz,
  primary key (bucket, schema_name, table_name)
);
create index if not exists table_stats_table_idx on obs.table_stats (schema_name, table_name, bucket desc);

-- Plano genérico (EXPLAIN (GENERIC_PLAN, FORMAT JSON), sem executar) por fingerprint
-- lento, com a recomendação calculada a partir do plano.
create table if not exists obs.query_plans (
  fingerprint text primary key,
  statement text not null,
  plan jsonb,
  total_cost double precision,
  findings jsonb not null default '[]'::jsonb,
  error text not null default '',
  captured_at timestamptz not null default now()
);
