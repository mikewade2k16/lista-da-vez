-- Permissoes proprias do Planejamento. Separa escalas/jornadas da administracao
-- de lojas da workspace Multi-loja e preserva os papeis operacionais existentes.

insert into access_permissions (key, scope, description)
values
  ('workspace.planejamento.view', 'tenant', 'Visualizar escalas, jornadas e metas do Planejamento.'),
  ('workspace.planejamento.edit', 'tenant', 'Editar configuracoes e escalas do Planejamento.')
on conflict (key) do update
set
  scope = excluded.scope,
  description = excluded.description;

insert into access_role_permissions (role, permission_key)
values
  ('marketing', 'workspace.planejamento.view'),
  ('manager', 'workspace.planejamento.view'),
  ('manager', 'workspace.planejamento.edit'),
  ('director', 'workspace.planejamento.view'),
  ('director', 'workspace.planejamento.edit'),
  ('owner', 'workspace.planejamento.view'),
  ('owner', 'workspace.planejamento.edit'),
  ('platform_admin', 'workspace.planejamento.view'),
  ('platform_admin', 'workspace.planejamento.edit')
on conflict (role, permission_key) do nothing;

insert into core.modules (id, schema_name, label, is_core)
values ('core', 'core', 'Plataforma core', true)
on conflict (id) do nothing;

insert into core.permissions (key, module_id, label, description, scope)
values
  ('workspace.planejamento.view', 'core', 'Ver pagina Planejamento', 'Visualizar escalas, jornadas e metas do Planejamento.', 'account'),
  ('workspace.planejamento.edit', 'core', 'Editar Planejamento', 'Editar configuracoes e escalas do Planejamento.', 'account')
on conflict (key) do update
set
  label = excluded.label,
  description = excluded.description,
  scope = excluded.scope,
  deprecated_at = null;

with role_permissions(role_code, permission_key) as (
  values
    ('queue.marketing', 'workspace.planejamento.view'),
    ('marketing', 'workspace.planejamento.view'),
    ('queue.manager', 'workspace.planejamento.view'),
    ('queue.manager', 'workspace.planejamento.edit'),
    ('manager', 'workspace.planejamento.view'),
    ('manager', 'workspace.planejamento.edit'),
    ('queue.director', 'workspace.planejamento.view'),
    ('queue.director', 'workspace.planejamento.edit'),
    ('queue.supervisor', 'workspace.planejamento.view'),
    ('queue.supervisor', 'workspace.planejamento.edit'),
    ('core.admin', 'workspace.planejamento.view'),
    ('core.admin', 'workspace.planejamento.edit'),
    ('director', 'workspace.planejamento.view'),
    ('director', 'workspace.planejamento.edit'),
    ('queue.owner', 'workspace.planejamento.view'),
    ('queue.owner', 'workspace.planejamento.edit'),
    ('core.owner', 'workspace.planejamento.view'),
    ('core.owner', 'workspace.planejamento.edit'),
    ('owner', 'workspace.planejamento.view'),
    ('owner', 'workspace.planejamento.edit')
)
insert into core.role_permissions (role_id, permission_key)
select role.id, role_permissions.permission_key
from core.roles role
join role_permissions on lower(role.code) = role_permissions.role_code
on conflict (role_id, permission_key) do nothing;

with template_permissions(role_template_id, permission_key) as (
  values
    ('queue.marketing', 'workspace.planejamento.view'),
    ('queue.manager', 'workspace.planejamento.view'),
    ('queue.manager', 'workspace.planejamento.edit'),
    ('queue.director', 'workspace.planejamento.view'),
    ('queue.director', 'workspace.planejamento.edit'),
    ('core.admin', 'workspace.planejamento.view'),
    ('core.admin', 'workspace.planejamento.edit'),
    ('queue.owner', 'workspace.planejamento.view'),
    ('queue.owner', 'workspace.planejamento.edit'),
    ('core.owner', 'workspace.planejamento.view'),
    ('core.owner', 'workspace.planejamento.edit')
)
insert into core.role_template_permissions (role_template_id, permission_key)
select role_template.id, template_permissions.permission_key
from core.role_templates role_template
join template_permissions on lower(role_template.id) = template_permissions.role_template_id
on conflict (role_template_id, permission_key) do nothing;
