-- 0266_restrict_transcriptions_and_campaigns.sql
--
-- Separa a captura operacional de audio da consulta sensivel de transcricoes
-- e restringe os defaults de Transcricoes/Campanhas ao dono da account e ao
-- administrador da plataforma. Overrides explicitos continuam soberanos.

insert into access_permissions (key, scope, description)
values
  ('workspace.transcricoes.view', 'tenant', 'Visualizar audios e transcricoes dos atendimentos.'),
  ('workspace.transcricoes.edit', 'tenant', 'Solicitar transcricoes, analises e editar sua configuracao.')
on conflict (key) do update
set
  scope = excluded.scope,
  description = excluded.description;

insert into access_role_permissions (role, permission_key)
values
  ('owner', 'workspace.transcricoes.view'),
  ('owner', 'workspace.transcricoes.edit'),
  ('platform_admin', 'workspace.transcricoes.view'),
  ('platform_admin', 'workspace.transcricoes.edit')
on conflict (role, permission_key) do nothing;

delete from access_role_permissions
where role = 'marketing'
  and permission_key in ('workspace.campanhas.view', 'workspace.campanhas.edit');

insert into core.modules (id, schema_name, label, is_core)
values ('core', 'core', 'Plataforma core', true)
on conflict (id) do nothing;

insert into core.permissions (key, module_id, label, description, scope)
values
  ('workspace.transcricoes.view', 'core', 'Ver pagina Transcricoes', 'Visibilidade de audios e transcricoes dos atendimentos.', 'account'),
  ('workspace.transcricoes.edit', 'core', 'Gerenciar Transcricoes', 'Solicitar transcricoes, analises e editar sua configuracao.', 'account')
on conflict (key) do update
set
  label = excluded.label,
  description = excluded.description,
  scope = excluded.scope,
  deprecated_at = null;

insert into core.role_permissions (role_id, permission_key)
select role.id, permission.permission_key
from core.roles role
cross join (
  values
    ('workspace.transcricoes.view'),
    ('workspace.transcricoes.edit')
) as permission(permission_key)
where lower(role.code) in ('queue.owner', 'core.owner', 'owner')
on conflict (role_id, permission_key) do nothing;

delete from core.role_permissions role_permission
using core.roles role
where role_permission.role_id = role.id
  and lower(role.code) in ('queue.marketing', 'marketing')
  and role_permission.permission_key in ('workspace.campanhas.view', 'workspace.campanhas.edit');

insert into core.role_template_permissions (role_template_id, permission_key)
select role_template.id, permission.permission_key
from core.role_templates role_template
cross join (
  values
    ('workspace.transcricoes.view'),
    ('workspace.transcricoes.edit')
) as permission(permission_key)
where lower(role_template.id) in ('queue.owner', 'core.owner', 'owner')
on conflict (role_template_id, permission_key) do nothing;

delete from core.role_template_permissions
where lower(role_template_id) in ('queue.marketing', 'marketing')
  and permission_key in ('workspace.campanhas.view', 'workspace.campanhas.edit');
