-- A tabela vive em queue.* desde 0105_queue_operations.sql; public.* e uma view de compat.
alter table queue.tenant_operation_core_settings
	add column if not exists crm_list_usage_tiers jsonb not null default '[
		{"id":"pessimo","label":"Pessimo","minRate":0},
		{"id":"ruim","label":"Ruim","minRate":10},
		{"id":"normal","label":"Normal","minRate":50},
		{"id":"bom","label":"Bom","minRate":65},
		{"id":"otimo","label":"Otimo","minRate":80},
		{"id":"perfeito","label":"Perfeito","minRate":100}
	]'::jsonb,
	add column if not exists crm_list_usage_min_orders_for_highlight integer not null default 5,
	add column if not exists crm_goal_payout_policy jsonb not null default '{
		"consultant":[
			{"threshold":80,"value":1,"mode":"percent"},
			{"threshold":90,"value":2,"mode":"percent"},
			{"threshold":100,"value":3,"mode":"percent"},
			{"threshold":120,"value":3.2,"mode":"percent"}
		],
		"manager":[
			{"threshold":80,"value":0.8,"mode":"percent"},
			{"threshold":90,"value":0.9,"mode":"percent"},
			{"threshold":100,"value":1,"mode":"percent"},
			{"threshold":120,"value":1.2,"mode":"percent"}
		],
		"support":[
			{"threshold":80,"value":80,"mode":"amount"},
			{"threshold":90,"value":90,"mode":"amount"},
			{"threshold":100,"value":100,"mode":"amount"},
			{"threshold":120,"value":120,"mode":"amount"}
		]
	}'::jsonb;

-- Views com SELECT * congelam as colunas no CREATE VIEW.
-- Recria a view para expor as colunas novas a consumidores legados em public.*.
create or replace view public.tenant_operation_core_settings as
	select * from queue.tenant_operation_core_settings;
