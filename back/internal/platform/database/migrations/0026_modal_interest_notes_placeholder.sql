alter table tenant_operation_settings
	add column if not exists product_seen_notes_placeholder text not null default '';
