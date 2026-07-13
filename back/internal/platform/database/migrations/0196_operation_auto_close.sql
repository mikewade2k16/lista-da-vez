-- 0196: Auto-encerramento de atendimento (2h).
--
-- Adiciona o estado de countdown/adiamento no atendimento corrente, o status de
-- validacao + trilha de auditoria no historico, e a config por tenant que o sweep
-- (operations.ProcessTimedAlerts) le via LoadOperationalRules.
--
-- SQL plano idempotente, sem goose Down (o migrator roda o arquivo inteiro).
-- As tabelas reais vivem no schema `queue`; os objetos `public.*` sao VIEWS
-- (select * from queue.*) e NAO expoem colunas novas ate serem recriadas
-- (trap das migrations 0105/0113/0106).

-- ---------------------------------------------------------------------------
-- 1) Estado corrente do atendimento: countdown (grace) e adiamento (snooze).
--    grace_deadline = epoch ms ABSOLUTO em que o countdown vence (0 = sem countdown).
--    snoozed_until  = epoch ms ate quando o "Continuar" adia a reavaliacao.
--    snooze_count   = quantas vezes o operador adiou.
-- ---------------------------------------------------------------------------
alter table queue.operation_active_services
    add column if not exists grace_deadline bigint  not null default 0,
    add column if not exists snoozed_until  bigint  not null default 0,
    add column if not exists snooze_count   integer not null default 0;

-- ---------------------------------------------------------------------------
-- 2) Historico: motivo do fechamento, status de validacao e auditoria.
--    close_reason      = 'manual' (encerrado por pessoa) | 'auto' (sweep).
--    validation_status = 'validated' (default, fluxo manual) | 'pending' (auto,
--                        aguardando gerente) | 'cancelled' (metrica descartada).
--    validated_by/at   = quem e quando validou/cancelou (auditoria; sem FK para
--                        core.* por regra do modulo queue).
--    snooze_count      = adiamentos ate o auto-close (ajuda a distinguir esquecido
--                        de continuado).
--    cancel_reason     = motivo obrigatorio quando o gerente cancela a metrica.
-- ---------------------------------------------------------------------------
alter table queue.operation_service_history
    add column if not exists close_reason      text    not null default 'manual',
    add column if not exists validation_status text    not null default 'validated',
    add column if not exists validated_by      uuid,
    add column if not exists validated_at      bigint  not null default 0,
    add column if not exists snooze_count      integer not null default 0,
    add column if not exists cancel_reason     text    not null default '';

alter table queue.operation_service_history
    drop constraint if exists osh_close_reason_check;
alter table queue.operation_service_history
    add constraint osh_close_reason_check
    check (close_reason in ('manual', 'auto'));

alter table queue.operation_service_history
    drop constraint if exists osh_validation_status_check;
alter table queue.operation_service_history
    add constraint osh_validation_status_check
    check (validation_status in ('pending', 'validated', 'cancelled'));

-- Relaxar o CHECK de finish_outcome para admitir o sentinela 'auto'. O nome foi
-- auto-gerado pelo `create table ... (like ... including all)` da 0105, entao
-- dropamos qualquer check que referencie a coluna antes de recriar com nome fixo.
do $$
declare
    constraint_name text;
begin
    for constraint_name in
        select con.conname
        from pg_constraint con
        join pg_class rel on rel.oid = con.conrelid
        join pg_namespace nsp on nsp.oid = rel.relnamespace
        where nsp.nspname = 'queue'
          and rel.relname = 'operation_service_history'
          and con.contype = 'c'
          and pg_get_constraintdef(con.oid) ilike '%finish_outcome%'
    loop
        execute format(
            'alter table queue.operation_service_history drop constraint %I',
            constraint_name
        );
    end loop;
end $$;

alter table queue.operation_service_history
    add constraint operation_service_history_finish_outcome_check
    check (finish_outcome in ('reserva', 'compra', 'nao-compra', 'auto'));

-- Indice parcial para a caixa de Pendencias (poucas linhas pending por loja).
create index if not exists osh_pending_validation_idx
    on queue.operation_service_history (store_id, finished_at desc)
    where validation_status = 'pending';

-- ---------------------------------------------------------------------------
-- 3) Config por tenant do auto-encerramento (fonte que o sweep le).
-- ---------------------------------------------------------------------------
alter table queue.tenant_operational_alert_rules
    add column if not exists auto_close_enabled       boolean not null default false,
    add column if not exists auto_close_minutes       integer not null default 120,
    add column if not exists auto_close_grace_seconds integer not null default 60,
    add column if not exists snooze_reprompt_minutes  integer not null default 30;

-- ---------------------------------------------------------------------------
-- 4) Recriar as views public.* (select * congela as colunas na criacao).
-- ---------------------------------------------------------------------------
create or replace view public.operation_active_services as
    select * from queue.operation_active_services;
create or replace view public.operation_service_history as
    select * from queue.operation_service_history;
create or replace view public.tenant_operational_alert_rules as
    select * from queue.tenant_operational_alert_rules;
