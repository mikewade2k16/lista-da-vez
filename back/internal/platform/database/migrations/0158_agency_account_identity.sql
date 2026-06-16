-- 0158 — Identidade da conta-agência: "crow" vira "Crow Visuals", ganha is_agency
-- e TODOS os módulos do catálogo; sai da lista de clientes (filtro na camada admin).
--
-- Plano canônico: docs/AGENCY_TENANT_ARCHITECTURE.md (Trilho 2 — identidade da
-- conta-agência). Decisão do dono do produto:
--   A conta-agência hoje se chama "crow" (slug 'crow') e aparece como se fosse um
--   cliente na lista de /manage/clientes — confunde. Ela é o WORKSPACE da agência
--   (dona do board geral de Tasks), não um cliente. Esta migration:
--     1. Adiciona a coluna core.accounts.is_agency (marca a conta-workspace).
--     2. Renomeia 'crow' -> 'Crow Visuals' (mantém o slug 'crow').
--     3. Marca is_agency=true na conta 'crow'.
--     4. Habilita TODOS os módulos do catálogo na conta 'crow'.
--   A organização (core.organizations slug 'crow-visuals', criada na 0156) continua
--   existindo e não é tocada aqui.
--
-- REGRAS (AGENT_RULES.md + memória do projeto):
--   - O migrator roda o arquivo .sql INTEIRO no boot. SEM marcadores -- +goose;
--     SQL plano e idempotente (IF NOT EXISTS / WHERE por slug / ON CONFLICT DO UPDATE),
--     seguro para rodar 2x sem efeito colateral.
--   - Schema sempre qualificado (core.*). Data-driven por lower(slug) — sem hardcode de UUID.
--   - NÃO toca em password_hash nem cria/edita usuários.
-- Defensivo: se a conta 'crow' não existir (outro ambiente), os UPDATEs afetam 0
-- linhas sem erro e o INSERT...SELECT não gera linhas (sem erro).

-- ----------------------------------------------------------------------------
-- 1. Coluna is_agency em core.accounts (idempotente: IF NOT EXISTS)
-- ----------------------------------------------------------------------------
alter table core.accounts
    add column if not exists is_agency boolean not null default false;

-- ----------------------------------------------------------------------------
-- 2. Renomear a conta-agência para "Crow Visuals" (mantém o slug 'crow')
-- ----------------------------------------------------------------------------
-- Idempotente: rodar 2x reescreve o mesmo nome (no-op de valor) e bumpa updated_at.
update core.accounts
set name = 'Crow Visuals',
    updated_at = now()
where lower(slug) = 'crow';

-- ----------------------------------------------------------------------------
-- 3. Marcar a conta-agência como is_agency
-- ----------------------------------------------------------------------------
-- Idempotente por valor; só toca a linha cujo slug é 'crow'.
update core.accounts
set is_agency = true,
    updated_at = now()
where lower(slug) = 'crow';

-- ----------------------------------------------------------------------------
-- 4. Habilitar TODOS os módulos do catálogo na conta-agência
-- ----------------------------------------------------------------------------
-- A conta-workspace da agência precisa de todos os módulos (é a casa dos admins).
-- CROSS JOIN de 'crow' x core.modules. PK de core.account_modules é
-- (account_id, module_id); ON CONFLICT DO UPDATE reativa entradas existentes e
-- atualiza enabled_at. Idempotente: rodar 2x mantém todos enabled=true.
-- Se a conta 'crow' não existir, o SELECT não retorna linhas (sem erro).
insert into core.account_modules (account_id, module_id, enabled)
select a.id, m.id, true
from core.accounts a
cross join core.modules m
where lower(a.slug) = 'crow'
on conflict (account_id, module_id) do update
    set enabled = true,
        enabled_at = now();
