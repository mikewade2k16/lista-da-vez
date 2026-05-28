alter table if exists store_setting_options
	drop constraint if exists store_setting_options_kind_check;

alter table if exists store_setting_options
	add constraint store_setting_options_kind_check
	check (
		kind in (
			'visit_reason',
			'customer_source',
			'pause_reason',
			'queue_jump_reason',
			'loss_reason',
			'profession'
		)
	);
