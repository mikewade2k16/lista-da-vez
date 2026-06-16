-- 0157 — Agência: platform_admins viram membros + agency_owner da org Crow Visuals.
--
-- Plano canônico: docs/AGENCY_TENANT_ARCHITECTURE.md (Etapa 4, parte de acesso).
-- Contexto: descobriu-se (2026-06-15) que os platform_admins NÃO tinham membership
-- real em conta nenhuma (eram admins "flutuantes"). Sem uma conta-casa, o default
-- de conta ativa caía na 1ª por nome (am-malls) em vez do board da agência. Esta
-- migration dá aos admins a casa correta: a conta-agência "crow".
--
-- Efeitos (ambos idempotentes, data-driven por slug):
--   1. Cada platform_admin ativo vira MEMBRO da conta-agência "crow"
--      (core.account_users) — com isso o ListAccountsForUser membership-first passa
--      a defaultar o admin para a conta crow (onde mora o board geral de Tasks).
--   2. Cada platform_admin ativo vira agency_owner da org "crow-visuals"
--      (core.organization_users) — fecha o seed que a 0156 não pôde fazer (a conta
--      crow não tinha membros na época).
--
-- REGRAS: SQL plano e idempotente (ON CONFLICT DO NOTHING), schema sempre
-- qualificado (core.*), sem +goose. NÃO toca em password_hash nem cria usuários.
-- Defensivo: se a conta "crow" ou a org "crow-visuals" não existir (outro ambiente),
-- o SELECT não retorna linhas e nada é inserido (sem erro).

-- ----------------------------------------------------------------------------
-- 1. platform_admins viram membros da conta-agência "crow"
-- ----------------------------------------------------------------------------
insert into core.account_users (account_id, user_id, is_active)
select a.id, u.id, true
from core.accounts a
cross join core.users u
where lower(a.slug) = 'crow'
  and u.is_platform_admin = true
  and u.is_active = true
on conflict (account_id, user_id) do nothing;

-- ----------------------------------------------------------------------------
-- 2. platform_admins viram agency_owner da org "crow-visuals"
-- ----------------------------------------------------------------------------
insert into core.organization_users (organization_id, user_id, org_role)
select o.id, u.id, 'agency_owner'
from core.organizations o
cross join core.users u
where lower(o.slug) = 'crow-visuals'
  and u.is_platform_admin = true
  and u.is_active = true
on conflict (organization_id, user_id) do nothing;
