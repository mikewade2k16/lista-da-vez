insert into access_permissions (key, scope, description)
values
	('workspace.alertas.view', 'tenant', 'Visualizar a workspace Alertas.'),
	('workspace.alertas.edit', 'tenant', 'Gerenciar a workspace Alertas.'),
	('alerts.rules.manage', 'tenant', 'Editar regras tenant-wide do modulo de alertas.'),
	('alerts.actions.manage', 'tenant', 'Executar acknowledge e resolucao de alertas operacionais.')
on conflict (key) do update
set
	scope = excluded.scope,
	description = excluded.description;

insert into access_role_permissions (role, permission_key)
values
	('store_terminal', 'workspace.alertas.view'),
	('store_terminal', 'alerts.actions.manage'),
	('manager', 'workspace.alertas.view'),
	('manager', 'alerts.actions.manage'),
	('owner', 'workspace.alertas.view'),
	('owner', 'workspace.alertas.edit'),
	('owner', 'alerts.rules.manage'),
	('owner', 'alerts.actions.manage'),
	('platform_admin', 'workspace.alertas.view'),
	('platform_admin', 'workspace.alertas.edit'),
	('platform_admin', 'alerts.rules.manage'),
	('platform_admin', 'alerts.actions.manage')
on conflict (role, permission_key) do nothing;