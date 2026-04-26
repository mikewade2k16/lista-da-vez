alter table store_operation_settings
	add column if not exists show_product_seen_notes_field boolean not null default true,
	add column if not exists show_existing_customer_field boolean not null default true,
	add column if not exists allow_product_seen_none boolean not null default true,
	add column if not exists require_product_seen_notes_field boolean not null default false,
	add column if not exists require_product_seen_notes_when_none boolean not null default true,
	add column if not exists product_seen_notes_min_chars integer not null default 20;

update store_operation_settings
set
	show_product_seen_notes_field = true,
	show_existing_customer_field = true,
	allow_product_seen_none = true,
	require_product_seen_notes_field = false,
	require_product_seen_notes_when_none = true,
	product_seen_notes_min_chars = 20
where true;

update store_operation_settings
set product_seen_label = 'Interesses do cliente'
where trim(coalesce(product_seen_label, '')) = ''
	or trim(coalesce(product_seen_label, '')) = 'Produto visto pelo cliente';

update store_operation_settings
set product_seen_placeholder = 'Busque e selecione interesses'
where trim(coalesce(product_seen_placeholder, '')) = ''
	or trim(coalesce(product_seen_placeholder, '')) = 'Busque e selecione um produto';

update store_operation_settings
set product_closed_label = ''
where trim(coalesce(product_closed_label, '')) = 'Produto reservado/comprado';
