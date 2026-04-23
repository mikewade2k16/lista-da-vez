insert into consultants (
	id,
	tenant_id,
	store_id,
	name,
	role_label,
	initials,
	color,
	monthly_goal,
	commission_rate,
	conversion_goal,
	avg_ticket_goal,
	pa_goal,
	is_active
)
values
	('dddddddd-dddd-dddd-dddd-ddddddddd001', 'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa', 'bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbb001', 'Thalles', 'Atendimento', 'TH', '#168aad', 140000, 0.0250, 0, 0, 0, true),
	('dddddddd-dddd-dddd-dddd-ddddddddd002', 'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa', 'bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbb001', 'Erik', 'Atendimento', 'ER', '#7a6ff0', 180000, 0.0300, 0, 0, 0, true),
	('dddddddd-dddd-dddd-dddd-ddddddddd003', 'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa', 'bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbb001', 'Camila', 'Atendimento', 'CA', '#d17a96', 150000, 0.0280, 0, 0, 0, true),
	('dddddddd-dddd-dddd-dddd-ddddddddd004', 'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa', 'bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbb001', 'Mariana', 'Atendimento', 'MA', '#e09f3e', 130000, 0.0240, 0, 0, 0, true),
	('dddddddd-dddd-dddd-dddd-ddddddddd005', 'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa', 'bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbb001', 'Hiro', 'Atendimento', 'HI', '#355070', 165000, 0.0290, 0, 0, 0, true),
	('dddddddd-dddd-dddd-dddd-ddddddddd006', 'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa', 'bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbb001', 'Nathalia', 'Atendimento', 'NA', '#d90429', 155000, 0.0270, 0, 0, 0, true),
	('dddddddd-dddd-dddd-dddd-ddddddddd007', 'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa', 'bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbb002', 'Aline', 'Atendimento', 'AL', '#168aad', 120000, 0.0240, 0, 0, 0, true),
	('dddddddd-dddd-dddd-dddd-ddddddddd008', 'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa', 'bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbb002', 'Joao', 'Atendimento', 'JO', '#7a6ff0', 115000, 0.0220, 0, 0, 0, true),
	('dddddddd-dddd-dddd-dddd-ddddddddd009', 'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa', 'bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbb002', 'Bruna', 'Atendimento', 'BR', '#d17a96', 135000, 0.0260, 0, 0, 0, true),
	('dddddddd-dddd-dddd-dddd-ddddddddd010', 'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa', 'bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbb002', 'Leo', 'Atendimento', 'LE', '#e09f3e', 128000, 0.0250, 0, 0, 0, true)
on conflict (id) do nothing;
