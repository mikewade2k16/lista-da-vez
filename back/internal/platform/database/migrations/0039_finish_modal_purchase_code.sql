alter table tenant_operation_settings
	add column if not exists finish_flow_mode text not null default 'legacy',
	add column if not exists purchase_code_label text not null default '',
	add column if not exists purchase_code_placeholder text not null default '',
	add column if not exists show_purchase_code_field boolean not null default true,
	add column if not exists require_purchase_code_field boolean not null default true;

alter table tenant_operation_settings
	drop constraint if exists tenant_operation_settings_finish_flow_mode_check;

alter table tenant_operation_settings
	add constraint tenant_operation_settings_finish_flow_mode_check
	check (finish_flow_mode in ('legacy', 'erp-reconciliation'));

update tenant_operation_settings
set
	finish_flow_mode = coalesce(nullif(trim(finish_flow_mode), ''), 'legacy'),
	purchase_code_label = coalesce(nullif(trim(purchase_code_label), ''), 'Codigo da compra'),
	purchase_code_placeholder = coalesce(
		nullif(trim(purchase_code_placeholder), ''),
		'Informe o codigo da compra para conciliacao posterior'
	)
where true;

alter table operation_service_history
	add column if not exists purchase_code text not null default '';
