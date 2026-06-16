-- 0156 — Hierarquia agencia -> cliente: organizacao "Crow Visuals" (Etapa 2).
--
-- Plano canonico: docs/AGENCY_TENANT_ARCHITECTURE.md (Onda 1, Trilho C).
-- Modelo-alvo (§1, definido pelo dono do produto):
--   Organizacao "Crow Visuals" (agencia) e o nivel mais alto; TODAS as
--   contas-cliente (core.accounts) pertencem a ela. A conta-agencia "Crow"
--   e dona do board geral de Tasks. Os donos da conta-agencia sao
--   agency_owner da organizacao.
--
-- Esta migration faz 3 coisas, todas idempotentes e data-driven (sem hardcode
-- de UUID; resolve por slug/name com lower()):
--   1. Cria a org "Crow Visuals" em core.organizations (se nao existir).
--   2. Vincula TODAS as contas soltas (organization_id IS NULL) a essa org.
--   3. Promove os membros ativos da conta-agencia "Crow" a agency_owner.
--
-- REGRAS (AGENT_RULES.md + memoria do projeto):
--   - O migrator roda o arquivo .sql INTEIRO no boot. SEM marcadores -- +goose;
--     SQL plano e idempotente (IF NOT EXISTS / WHERE NOT EXISTS / ON CONFLICT
--     DO NOTHING / WHERE ... IS NULL), seguro para rodar 2x sem efeito colateral.
--   - Schema sempre qualificado (core.*).
--   - NAO toca em password_hash nem cria/edita usuarios.

-- ----------------------------------------------------------------------------
-- 1. Org "Crow Visuals" (idempotente por lower(slug))
-- ----------------------------------------------------------------------------
-- Usa WHERE NOT EXISTS (em vez de ON CONFLICT) para nao depender do nome do
-- indice de expressao core_organizations_slug_uidx on (lower(slug)). Rodar 2x:
-- na 2a vez a org ja existe, o SELECT do EXISTS casa e nada e inserido.
insert into core.organizations (slug, name)
select 'crow-visuals', 'Crow Visuals'
where not exists (
    select 1 from core.organizations o where lower(o.slug) = 'crow-visuals'
);

-- ----------------------------------------------------------------------------
-- 2. Vincular contas soltas a org Crow
-- ----------------------------------------------------------------------------
-- ASSUNCAO (modelo do dono do produto, doc §1): toda conta atualmente sem org
-- pertence a agencia Crow. Vincula apenas as contas com organization_id IS NULL
-- (data-driven; nao hardcode de ids). Idempotente: na 2a execucao nenhuma conta
-- tem organization_id NULL, entao o UPDATE afeta 0 linhas. Contas ja vinculadas
-- a OUTRA org no futuro nao sao tocadas (filtro IS NULL).
update core.accounts
set organization_id = (
        select o.id from core.organizations o where lower(o.slug) = 'crow-visuals'
    ),
    updated_at = now()
where organization_id is null
  and exists (
      select 1 from core.organizations o where lower(o.slug) = 'crow-visuals'
  );

-- ----------------------------------------------------------------------------
-- 3. Seed agency_owner — membros ativos da conta-agencia "Crow"
-- ----------------------------------------------------------------------------
-- Resolve a conta-agencia por slug OU name = 'crow' (lower), seguindo o padrao
-- de 0154 (perola por slug/name). Pega os usuarios com membership ativa nessa
-- conta (core.account_users.is_active) e os promove a agency_owner da org Crow.
--
-- INCERTEZA DE DADOS (reverificar no banco vivo antes de aplicar): o slug/name
-- exato da conta-agencia. O doc cita "Crow (80caf5d5)" mas e dado de 2026-06-10.
-- Esta condicao e DEFENSIVA: se nenhuma conta casar com 'crow', o SELECT nao
-- retorna linhas e NADA e inserido (sem erro). Se o slug real for outro
-- (ex.: 'crow-visuals', 'agencia-crow'), o supervisor ajusta a condicao abaixo.
--
-- Idempotente: ON CONFLICT (organization_id, user_id) DO NOTHING (PK da tabela).
-- Rodar 2x nao duplica nem altera org_role de quem ja esta la.
insert into core.organization_users (organization_id, user_id, org_role)
select
    org.id,
    au.user_id,
    'agency_owner'
from core.account_users au
join core.accounts a on a.id = au.account_id
cross join lateral (
    select o.id from core.organizations o where lower(o.slug) = 'crow-visuals'
) org
where au.is_active = true
  and (lower(a.slug) = 'crow' or lower(a.name) = 'crow')
on conflict (organization_id, user_id) do nothing;
