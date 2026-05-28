insert into access_permissions (key, scope, description)
values
	('workspace.manage.view', 'tenant', 'Visualizar as rotas agrupadas em Manage.'),
	('workspace.themes.view', 'tenant', 'Visualizar a workspace Temas.'),
	('workspace.tools.view', 'tenant', 'Visualizar a workspace Tools.')
on conflict (key) do update
set
	scope = excluded.scope,
	description = excluded.description;

insert into access_role_permissions (role, permission_key)
values
	('owner', 'workspace.manage.view'),
	('owner', 'workspace.themes.view'),
	('owner', 'workspace.tools.view'),
	('platform_admin', 'workspace.manage.view'),
	('platform_admin', 'workspace.themes.view'),
	('platform_admin', 'workspace.tools.view')
on conflict (role, permission_key) do nothing;