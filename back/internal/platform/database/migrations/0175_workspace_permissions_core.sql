-- 0175_workspace_permissions_core.sql
--
-- FASE ADITIVA do access -> core (gating de PAGINA). Traz as permissoes
-- workspace.*.view/.edit (visibilidade/edicao de pagina do menu) do modulo legado
-- `access` (access_permissions + access_role_permissions + user_access_overrides)
-- para o core (core.permissions + core.role_permissions + core.user_permission_overrides),
-- SEM trocar o caminho de leitura: o gating do menu continua resolvendo do access
-- (auth.permissionKeys <- /v1/me/context) ate o SWITCH (fase seguinte, com canary).
-- Zero blast radius: nada no front le core.workspace.* via has() hoje.
--
-- Idempotente e additivo (ON CONFLICT/NOT EXISTS). SQL plano: o migrator roda o
-- arquivo inteiro (sem marcadores goose). Ver docs/LEGADO.md item 5.

-- 0) Garante o modulo `core` em core.modules. Em DB fresh as migrations rodam ANTES
--    do SyncCatalog (que so popula core.modules no boot), entao o FK de core.permissions
--    (module_id -> core.modules) falharia. SyncCatalog faz UPSERT depois (idempotente).
insert into core.modules (id, schema_name, label, is_core)
values ('core', 'core', 'Plataforma core', true)
on conflict (id) do nothing;

-- 1) Catalogo: workspace.*.view/.edit em core.permissions (modulo core, sempre
--    habilitado, scope account). Declaradas tambem no registry do core (module.go)
--    para o SyncCatalog NAO marcar deprecated_at depois.
insert into core.permissions (key, module_id, label, description, scope)
values
  ('workspace.operacao.view', 'core', 'Ver pagina Operacao', 'Visibilidade da pagina Operacao no menu.', 'account'),
  ('workspace.operacao.edit', 'core', 'Editar na pagina Operacao', 'Executar comandos na pagina Operacao.', 'account'),
  ('workspace.consultor.view', 'core', 'Ver pagina Consultor', 'Visibilidade da pagina Consultor no menu.', 'account'),
  ('workspace.ranking.view', 'core', 'Ver pagina Ranking', 'Visibilidade da pagina Ranking no menu.', 'account'),
  ('workspace.dados.view', 'core', 'Ver pagina Dados', 'Visibilidade da pagina Dados no menu.', 'account'),
  ('workspace.inteligencia.view', 'core', 'Ver pagina Inteligencia', 'Visibilidade da pagina Inteligencia no menu.', 'account'),
  ('workspace.relatorios.view', 'core', 'Ver pagina Relatorios', 'Visibilidade da pagina Relatorios no menu.', 'account'),
  ('workspace.campanhas.view', 'core', 'Ver pagina Campanhas', 'Visibilidade da pagina Campanhas no menu.', 'account'),
  ('workspace.campanhas.edit', 'core', 'Editar na pagina Campanhas', 'Editar regras/campanhas na pagina Campanhas.', 'account'),
  ('workspace.clientes.view', 'core', 'Ver pagina Clientes', 'Visibilidade da pagina Clientes no menu.', 'account'),
  ('workspace.clientes.edit', 'core', 'Editar na pagina Clientes', 'Editar clientes e grupos na pagina Clientes.', 'account'),
  ('workspace.multiloja.view', 'core', 'Ver pagina Multi-loja', 'Visibilidade da pagina Multi-loja no menu.', 'account'),
  ('workspace.multiloja.edit', 'core', 'Editar na pagina Multi-loja', 'Editar lojas/config na pagina Multi-loja.', 'account'),
  ('workspace.usuarios.view', 'core', 'Ver pagina Usuarios', 'Visibilidade da pagina Usuarios no menu.', 'account'),
  ('workspace.usuarios.edit', 'core', 'Editar na pagina Usuarios', 'Editar usuarios/overrides na pagina Usuarios.', 'account'),
  ('workspace.manage.view', 'core', 'Ver paginas de Manage', 'Visibilidade das rotas agrupadas em Manage.', 'account'),
  ('workspace.configuracoes.view', 'core', 'Ver pagina Configuracoes', 'Visibilidade da pagina Configuracoes no menu.', 'account'),
  ('workspace.configuracoes.edit', 'core', 'Editar na pagina Configuracoes', 'Editar configuracoes operacionais.', 'account'),
  ('workspace.themes.view', 'core', 'Ver pagina Temas', 'Visibilidade da pagina Temas no menu.', 'account'),
  ('workspace.alertas.view', 'core', 'Ver pagina Alertas', 'Visibilidade da pagina Alertas no menu.', 'account'),
  ('workspace.alertas.edit', 'core', 'Editar na pagina Alertas', 'Gerenciar a pagina Alertas.', 'account'),
  ('workspace.feedback.view', 'core', 'Ver pagina Feedback', 'Visibilidade da pagina Feedback no menu.', 'account'),
  ('workspace.feedback.edit', 'core', 'Editar na pagina Feedback', 'Editar feedback e notas na pagina Feedback.', 'account'),
  ('workspace.tools.view', 'core', 'Ver pagina Tools', 'Visibilidade da pagina Tools no menu.', 'account'),
  ('workspace.erp.view', 'core', 'Ver pagina ERP', 'Visibilidade da pagina ERP no menu.', 'account'),
  ('workspace.erp.edit', 'core', 'Editar na pagina ERP', 'Sync manual e administracao na pagina ERP.', 'account')
on conflict (key) do nothing;

-- 2) Backfill core.role_permissions: para cada role efetiva de cada account, concede
--    as workspace.* conforme o papel coarse (mesmo mapa do access defaultRolePermissionMap).
--    code_to_coarse cobre os aliases de codigo usados no resolvedor (queue.*, core.*, plano).
with code_to_coarse(code, coarse) as (
  values
    ('queue.owner', 'owner'), ('core.owner', 'owner'), ('owner', 'owner'),
    ('queue.director', 'director'), ('core.admin', 'director'), ('director', 'director'), ('queue.supervisor', 'director'),
    ('queue.marketing', 'marketing'), ('marketing', 'marketing'),
    ('queue.manager', 'manager'), ('manager', 'manager'),
    ('queue.consultant', 'consultant'), ('core.member', 'consultant'), ('consultant', 'consultant'),
    ('queue.store_terminal', 'store_terminal'), ('store_terminal', 'store_terminal')
),
coarse_to_perm(coarse, permission_key) as (
  values
    ('consultant', 'workspace.operacao.view'), ('consultant', 'workspace.operacao.edit'),
    ('store_terminal', 'workspace.operacao.view'), ('store_terminal', 'workspace.operacao.edit'),
    ('store_terminal', 'workspace.consultor.view'), ('store_terminal', 'workspace.ranking.view'),
    ('store_terminal', 'workspace.dados.view'), ('store_terminal', 'workspace.inteligencia.view'),
    ('store_terminal', 'workspace.relatorios.view'), ('store_terminal', 'workspace.alertas.view'),
    ('manager', 'workspace.operacao.view'), ('manager', 'workspace.operacao.edit'),
    ('manager', 'workspace.alertas.view'), ('manager', 'workspace.erp.view'),
    ('manager', 'workspace.feedback.view'), ('manager', 'workspace.feedback.edit'),
    ('manager', 'workspace.multiloja.view'),
    ('marketing', 'workspace.operacao.view'), ('marketing', 'workspace.erp.view'),
    ('marketing', 'workspace.multiloja.view'), ('marketing', 'workspace.campanhas.view'),
    ('marketing', 'workspace.campanhas.edit'),
    ('director', 'workspace.operacao.view'), ('director', 'workspace.erp.view'),
    ('director', 'workspace.multiloja.view'),
    ('owner', 'workspace.operacao.view'), ('owner', 'workspace.operacao.edit'),
    ('owner', 'workspace.consultor.view'), ('owner', 'workspace.ranking.view'),
    ('owner', 'workspace.dados.view'), ('owner', 'workspace.inteligencia.view'),
    ('owner', 'workspace.relatorios.view'), ('owner', 'workspace.campanhas.view'),
    ('owner', 'workspace.campanhas.edit'), ('owner', 'workspace.clientes.view'),
    ('owner', 'workspace.clientes.edit'), ('owner', 'workspace.multiloja.view'),
    ('owner', 'workspace.multiloja.edit'), ('owner', 'workspace.usuarios.view'),
    ('owner', 'workspace.usuarios.edit'), ('owner', 'workspace.manage.view'),
    ('owner', 'workspace.configuracoes.view'), ('owner', 'workspace.configuracoes.edit'),
    ('owner', 'workspace.themes.view'), ('owner', 'workspace.alertas.view'),
    ('owner', 'workspace.alertas.edit'), ('owner', 'workspace.feedback.view'),
    ('owner', 'workspace.feedback.edit'), ('owner', 'workspace.tools.view'),
    ('owner', 'workspace.erp.view'), ('owner', 'workspace.erp.edit')
)
insert into core.role_permissions (role_id, permission_key)
select r.id, ctp.permission_key
from core.roles r
join code_to_coarse ctc on lower(r.code) = ctc.code
join coarse_to_perm ctp on ctp.coarse = ctc.coarse
on conflict (role_id, permission_key) do nothing;

-- 3) Backfill core.user_permission_overrides a partir do user_access_overrides legado
--    (so chaves workspace.*; tenant_id == account_id apos a repontagem de FKs da 0136).
--    Preserva allow/deny; anti-join contra override ativo ja existente (indice parcial).
insert into core.user_permission_overrides (account_id, user_id, permission_key, effect, note, is_active)
select uao.tenant_id, uao.user_id, uao.permission_key, uao.effect, coalesce(uao.note, ''), true
from user_access_overrides uao
where uao.is_active = true
  and uao.permission_key like 'workspace.%'
  and uao.tenant_id is not null
  and exists (select 1 from core.accounts a where a.id = uao.tenant_id)
  and exists (select 1 from core.users u where u.id = uao.user_id)
  and exists (select 1 from core.permissions p where p.key = uao.permission_key)
  and not exists (
    select 1 from core.user_permission_overrides existing
    where existing.account_id = uao.tenant_id
      and existing.user_id = uao.user_id
      and existing.permission_key = uao.permission_key
      and existing.is_active = true
  );
