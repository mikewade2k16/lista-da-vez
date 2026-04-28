-- Dev-only seed: add the ERP MVP store 184 and a store-scoped validation user.
-- This is skipped in production by migrator.go.

insert into stores (
	id,
	tenant_id,
	code,
	name,
	city,
	is_active
)
values (
	'bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbb0184',
	'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa',
	'184',
	'Loja 184',
	'',
	true
)
on conflict (tenant_id, code) do update
set
	name = excluded.name,
	city = excluded.city,
	is_active = excluded.is_active,
	updated_at = now();

insert into users (
	id,
	email,
	display_name,
	password_hash,
	must_change_password,
	is_active,
	employee_code,
	job_title
)
values (
	'cccccccc-cccc-cccc-cccc-cccccccc0184',
	'erp.184@demo.local',
	'ERP Loja 184',
	crypt('desde1967', gen_salt('bf', 10)),
	false,
	true,
	'184',
	'Gerente ERP Loja 184'
)
on conflict (id) do update
set
	email = excluded.email,
	display_name = excluded.display_name,
	password_hash = excluded.password_hash,
	must_change_password = excluded.must_change_password,
	is_active = excluded.is_active,
	employee_code = excluded.employee_code,
	job_title = excluded.job_title,
	updated_at = now();

delete from user_platform_roles
where user_id = 'cccccccc-cccc-cccc-cccc-cccccccc0184'::uuid;

delete from user_tenant_roles
where user_id = 'cccccccc-cccc-cccc-cccc-cccccccc0184'::uuid;

delete from user_store_roles
where user_id = 'cccccccc-cccc-cccc-cccc-cccccccc0184'::uuid;

insert into user_store_roles (
	user_id,
	store_id,
	role
)
values (
	'cccccccc-cccc-cccc-cccc-cccccccc0184',
	'bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbb0184',
	'manager'
)
on conflict (user_id, store_id, role) do nothing;

delete from user_access_overrides
where user_id = 'cccccccc-cccc-cccc-cccc-cccccccc0184'::uuid
	and permission_key = 'workspace.erp.edit';

insert into user_access_overrides (
	user_id,
	permission_key,
	effect,
	tenant_id,
	store_id,
	created_by_user_id,
	note,
	is_active
)
values (
	'cccccccc-cccc-cccc-cccc-cccccccc0184',
	'workspace.erp.edit',
	'allow',
	'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa',
	'bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbb0184',
	null,
	'Dev-only ERP MVP validation user for store 184.',
	true
);