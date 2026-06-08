-- Multi-Tenant Completion (C1.2)
-- Branch: refactor/multi-tenant-complete
-- Plano: docs/MULTITENANT_COMPLETION_PLAN.md secao C1
--
-- PRE-REQUISITO: executar apos a aplicacao ter bootado pelo menos uma vez com
-- CORE_V2_ENABLED=true, para que core.modules esteja populado pelo SyncCatalog.
-- Se os modulos ainda nao existirem no catalogo, os INSERTs produzem zero
-- linhas (JOIN vazio) e a migration passa sem erro. Re-executar depois.
--
-- Seed idempotente de core.account_modules para todas as accounts ativas:
--   - queue: sempre habilitado (modulo central da fila de atendimento)
--   - crm:   habilitado por default; pode ser desligado depois via painel
--   - tasks: habilitado (cuide das automacoes)
-- Outros modulos satelites (site, finance, omni, ...) ficam de fora aqui.
-- Esses sao habilitados sob demanda pelo painel admin (C3 + C10).

-- ============================================================================
-- 1. queue (modulo central — sempre habilitado para accounts ativas)
-- ============================================================================

insert into core.account_modules (account_id, module_id, enabled, config)
select
    a.id,
    m.id,
    true,
    '{}'::jsonb
from core.accounts a
cross join core.modules m
where a.is_active = true
  and m.id = 'queue'
on conflict (account_id, module_id) do nothing;

-- ============================================================================
-- 2. tasks (modulo central de automacoes)
-- ============================================================================

insert into core.account_modules (account_id, module_id, enabled, config)
select
    a.id,
    m.id,
    true,
    '{}'::jsonb
from core.accounts a
cross join core.modules m
where a.is_active = true
  and m.id = 'tasks'
on conflict (account_id, module_id) do nothing;

-- ============================================================================
-- 3. crm (habilitado por default; pode ser desligado via painel)
-- ============================================================================

insert into core.account_modules (account_id, module_id, enabled, config)
select
    a.id,
    m.id,
    true,
    '{}'::jsonb
from core.accounts a
cross join core.modules m
where a.is_active = true
  and m.id = 'crm'
on conflict (account_id, module_id) do nothing;
