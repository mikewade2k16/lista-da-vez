-- 0178_backfill_perola_user_roles.sql
--
-- Backfill de core.user_role_assignments para usuarios ATIVOS da conta-cliente
-- Perola que estao SEM nenhum papel (SEM_PAPEL) — eram invisiveis ao modulo legado
-- access/users (que exige role_code <> '') e davam 404 "Usuario nao encontrado" na
-- aba Paginas, e nao resolviam escopo de Fila. Deriva o papel do `job_title` do
-- usuario (o "Perfil"/subtitulo que o painel "Usuarios da fila" mostra), atribuindo
-- o core.roles queue.<papel> correspondente (que a 0176 garantiu existir por conta).
--
-- ADITIVO e idempotente: SO atribui a quem NAO tem nenhum assignment na conta
-- (guard NOT EXISTS) — nao remove, nao sobrescreve papel existente, nao toca quem ja
-- tem papel (inclusive um eventual job_title divergente do papel atual fica como
-- esta). Rodar 2x = no-op. ON CONFLICT (account_id, user_id, role_id) DO NOTHING.
--
-- Data-driven por slug (lower(slug)='perola'), NAO hardcode de uuid — funciona em
-- qualquer ambiente. job_title que nao casar com o mapa => usuario fica sem papel
-- (nada errado e atribuido). O migrator roda o arquivo INTEIRO no boot (sem goose);
-- schema sempre qualificado (core.*).
--
-- Mapa job_title -> papel-coarse (codes que existem em core.roles via 0176):
--   Consultor de Atendimento  -> queue.consultant
--   Gerente de Loja           -> queue.manager
--   Gerente ERP Loja 184      -> queue.manager
--   Diretoria                 -> queue.director
--   Terminal da Loja          -> queue.store_terminal
--   Gerente de Marketing      -> queue.marketing
--   Proprietario              -> queue.owner

with job_to_code(job_title, code) as (
  values
    ('Consultor de Atendimento', 'queue.consultant'),
    ('Gerente de Loja',          'queue.manager'),
    ('Gerente ERP Loja 184',     'queue.manager'),
    ('Diretoria',                'queue.director'),
    ('Terminal da Loja',         'queue.store_terminal'),
    ('Gerente de Marketing',     'queue.marketing'),
    ('Proprietario',             'queue.owner')
)
insert into core.user_role_assignments (account_id, user_id, role_id)
select a.id, au.user_id, r.id
from core.account_users au
join core.accounts a
  on a.id = au.account_id
  and lower(a.slug) = 'perola'
  and a.is_active = true
join core.users u on u.id = au.user_id
join job_to_code jc on jc.job_title = u.job_title
join core.roles r
  on r.account_id = a.id
  and lower(r.code) = jc.code
where au.is_active = true
  and not exists (
    select 1 from core.user_role_assignments ura
    where ura.account_id = a.id and ura.user_id = au.user_id
  )
on conflict (account_id, user_id, role_id) do nothing;
