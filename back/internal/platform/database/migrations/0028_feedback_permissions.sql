insert into access_permissions (key, scope, description)
values
	('workspace.feedback.view', 'tenant', 'Visualizar a workspace Feedback.'),
	('workspace.feedback.edit', 'tenant', 'Editar feedback e notas administrativas.')
on conflict (key) do update
set
	scope = excluded.scope,
	description = excluded.description;

insert into access_role_permissions (role, permission_key)
values
	('manager', 'workspace.feedback.view'),
	('manager', 'workspace.feedback.edit'),
	('owner', 'workspace.feedback.view'),
	('owner', 'workspace.feedback.edit'),
	('platform_admin', 'workspace.feedback.view'),
	('platform_admin', 'workspace.feedback.edit')
on conflict (role, permission_key) do nothing;
