alter table tenant_operation_settings
	add column if not exists service_cancel_window_seconds integer not null default 30,
	add column if not exists cancel_reason_label text not null default '',
	add column if not exists cancel_reason_placeholder text not null default '',
	add column if not exists cancel_reason_other_label text not null default '',
	add column if not exists cancel_reason_other_placeholder text not null default '',
	add column if not exists stop_reason_label text not null default '',
	add column if not exists stop_reason_placeholder text not null default '',
	add column if not exists stop_reason_other_label text not null default '',
	add column if not exists stop_reason_other_placeholder text not null default '',
	add column if not exists show_cancel_reason_field boolean not null default false,
	add column if not exists show_stop_reason_field boolean not null default false,
	add column if not exists cancel_reason_input_mode text not null default 'text',
	add column if not exists stop_reason_input_mode text not null default 'text',
	add column if not exists require_cancel_reason_field boolean not null default false,
	add column if not exists require_stop_reason_field boolean not null default false;

alter table tenant_setting_options
	drop constraint if exists tenant_setting_options_kind_check;

alter table tenant_setting_options
	add constraint tenant_setting_options_kind_check check (
		kind in (
			'visit_reason',
			'customer_source',
			'pause_reason',
			'cancel_reason',
			'stop_reason',
			'queue_jump_reason',
			'loss_reason',
			'profession'
		)
	);
