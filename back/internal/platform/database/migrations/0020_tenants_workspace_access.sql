insert into access_permissions (key, scope, description)
values
	('workspace.clientes.view', 'tenant', 'Visualizar a workspace Clientes.'),
	('workspace.clientes.edit', 'tenant', 'Editar clientes e grupos acessiveis.')
on conflict (key) do update
set
	scope = excluded.scope,
	description = excluded.description;

insert into access_role_permissions (role, permission_key)
values
	('owner', 'workspace.clientes.view'),
	('owner', 'workspace.clientes.edit'),
	('platform_admin', 'workspace.clientes.view'),
	('platform_admin', 'workspace.clientes.edit')
on conflict (role, permission_key) do nothing;