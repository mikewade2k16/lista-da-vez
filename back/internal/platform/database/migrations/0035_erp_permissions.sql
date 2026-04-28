insert into access_permissions (key, scope, description)
values
	('workspace.erp.view', 'tenant', 'Visualizar a workspace ERP.'),
	('workspace.erp.edit', 'tenant', 'Executar sync manual e administrar a workspace ERP.')
on conflict (key) do update
set
	scope = excluded.scope,
	description = excluded.description;

insert into access_role_permissions (role, permission_key)
values
	('manager', 'workspace.erp.view'),
	('marketing', 'workspace.erp.view'),
	('director', 'workspace.erp.view'),
	('owner', 'workspace.erp.view'),
	('owner', 'workspace.erp.edit'),
	('platform_admin', 'workspace.erp.view'),
	('platform_admin', 'workspace.erp.edit')
on conflict (role, permission_key) do nothing;