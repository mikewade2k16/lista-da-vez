insert into tenants (id, slug, name, is_active)
values ('aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa', 'tenant-demo', 'Tenant Demo', true)
on conflict (id) do nothing;

insert into stores (id, tenant_id, code, name, city, is_active)
values
	('bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbb001', 'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa', 'PJ-RIO', 'Perola Riomar', 'Aracaju', true),
	('bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbb002', 'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa', 'PJ-JAR', 'Perola Jardins', 'Aracaju', true)
on conflict (id) do nothing;

insert into users (id, email, display_name, password_hash, is_active)
values
	('cccccccc-cccc-cccc-cccc-ccccccccc001', 'consultor@demo.local', 'Consultor Demo', '$2a$10$k8vaTZguiU5hwQnaWbr/oOwFZmLCFXRiX/ddAYCFBM6c7x9neiW1u', true),
	('cccccccc-cccc-cccc-cccc-ccccccccc002', 'gerente@demo.local', 'Gerente Demo', '$2a$10$k8vaTZguiU5hwQnaWbr/oOwFZmLCFXRiX/ddAYCFBM6c7x9neiW1u', true),
	('cccccccc-cccc-cccc-cccc-ccccccccc003', 'marketing@demo.local', 'Marketing Demo', '$2a$10$k8vaTZguiU5hwQnaWbr/oOwFZmLCFXRiX/ddAYCFBM6c7x9neiW1u', true),
	('cccccccc-cccc-cccc-cccc-ccccccccc004', 'proprietario@demo.local', 'Proprietario Demo', '$2a$10$k8vaTZguiU5hwQnaWbr/oOwFZmLCFXRiX/ddAYCFBM6c7x9neiW1u', true),
	('cccccccc-cccc-cccc-cccc-ccccccccc005', 'plataforma@demo.local', 'Platform Admin Demo', '$2a$10$k8vaTZguiU5hwQnaWbr/oOwFZmLCFXRiX/ddAYCFBM6c7x9neiW1u', true)
on conflict (id) do nothing;

insert into user_store_roles (user_id, store_id, role)
values
	('cccccccc-cccc-cccc-cccc-ccccccccc001', 'bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbb001', 'consultant'),
	('cccccccc-cccc-cccc-cccc-ccccccccc002', 'bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbb001', 'manager')
on conflict (user_id, store_id, role) do nothing;

insert into user_tenant_roles (user_id, tenant_id, role)
values
	('cccccccc-cccc-cccc-cccc-ccccccccc003', 'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa', 'marketing'),
	('cccccccc-cccc-cccc-cccc-ccccccccc004', 'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa', 'owner')
on conflict (user_id, tenant_id, role) do nothing;

insert into user_platform_roles (user_id, role)
values
	('cccccccc-cccc-cccc-cccc-ccccccccc005', 'platform_admin')
on conflict (user_id) do nothing;
