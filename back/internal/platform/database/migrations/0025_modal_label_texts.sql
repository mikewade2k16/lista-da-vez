alter table tenant_operation_settings
	add column if not exists customer_name_label text not null default '',
	add column if not exists customer_phone_label text not null default '',
	add column if not exists customer_email_label text not null default '',
	add column if not exists customer_profession_label text not null default '',
	add column if not exists existing_customer_label text not null default '',
	add column if not exists product_seen_notes_label text not null default '',
	add column if not exists visit_reason_label text not null default '',
	add column if not exists customer_source_label text not null default '';
