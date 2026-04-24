create table if not exists store_operation_settings (
	store_id uuid primary key references stores(id) on delete cascade,
	selected_operation_template_id text not null default 'joalheria-padrao',
	max_concurrent_services integer not null default 10,
	timing_fast_close_minutes integer not null default 5,
	timing_long_service_minutes integer not null default 25,
	timing_low_sale_amount numeric(14, 2) not null default 1200,
	test_mode_enabled boolean not null default false,
	auto_fill_finish_modal boolean not null default false,
	alert_min_conversion_rate numeric(8, 2) not null default 0,
	alert_max_queue_jump_rate numeric(8, 2) not null default 0,
	alert_min_pa_score numeric(8, 2) not null default 0,
	alert_min_ticket_average numeric(14, 2) not null default 0,
	title text not null default '',
	product_seen_label text not null default '',
	product_seen_placeholder text not null default '',
	product_closed_label text not null default '',
	product_closed_placeholder text not null default '',
	notes_label text not null default '',
	notes_placeholder text not null default '',
	queue_jump_reason_label text not null default '',
	queue_jump_reason_placeholder text not null default '',
	loss_reason_label text not null default '',
	loss_reason_placeholder text not null default '',
	customer_section_label text not null default '',
	show_email_field boolean not null default true,
	show_profession_field boolean not null default true,
	show_notes_field boolean not null default true,
	visit_reason_selection_mode text not null default 'multiple',
	visit_reason_detail_mode text not null default 'shared',
	loss_reason_selection_mode text not null default 'single',
	loss_reason_detail_mode text not null default 'off',
	customer_source_selection_mode text not null default 'single',
	customer_source_detail_mode text not null default 'shared',
	require_product boolean not null default true,
	require_visit_reason boolean not null default true,
	require_customer_source boolean not null default true,
	require_customer_name_phone boolean not null default true,
	created_at timestamptz not null default now(),
	updated_at timestamptz not null default now()
);

create table if not exists store_setting_options (
	store_id uuid not null references stores(id) on delete cascade,
	kind text not null,
	option_id text not null,
	label text not null,
	sort_order integer not null default 0,
	created_at timestamptz not null default now(),
	updated_at timestamptz not null default now(),
	primary key (store_id, kind, option_id),
	constraint store_setting_options_kind_check check (
		kind in ('visit_reason', 'customer_source', 'queue_jump_reason', 'loss_reason', 'profession')
	)
);

create index if not exists store_setting_options_store_kind_idx
	on store_setting_options (store_id, kind, sort_order);

create table if not exists store_catalog_products (
	store_id uuid not null references stores(id) on delete cascade,
	product_id text not null,
	name text not null,
	code text not null default '',
	category text not null default 'Sem categoria',
	base_price numeric(14, 2) not null default 0,
	sort_order integer not null default 0,
	created_at timestamptz not null default now(),
	updated_at timestamptz not null default now(),
	primary key (store_id, product_id)
);

create index if not exists store_catalog_products_store_idx
	on store_catalog_products (store_id, sort_order);

create unique index if not exists store_catalog_products_store_code_uidx
	on store_catalog_products (store_id, upper(code))
	where trim(code) <> '';

insert into store_operation_settings (
	store_id,
	selected_operation_template_id,
	max_concurrent_services,
	timing_fast_close_minutes,
	timing_long_service_minutes,
	timing_low_sale_amount,
	test_mode_enabled,
	auto_fill_finish_modal,
	alert_min_conversion_rate,
	alert_max_queue_jump_rate,
	alert_min_pa_score,
	alert_min_ticket_average,
	title,
	product_seen_label,
	product_seen_placeholder,
	product_closed_label,
	product_closed_placeholder,
	notes_label,
	notes_placeholder,
	queue_jump_reason_label,
	queue_jump_reason_placeholder,
	loss_reason_label,
	loss_reason_placeholder,
	customer_section_label,
	show_email_field,
	show_profession_field,
	show_notes_field,
	visit_reason_selection_mode,
	visit_reason_detail_mode,
	loss_reason_selection_mode,
	loss_reason_detail_mode,
	customer_source_selection_mode,
	customer_source_detail_mode,
	require_product,
	require_visit_reason,
	require_customer_source,
	require_customer_name_phone,
	created_at,
	updated_at
)
select
	store_id,
	selected_operation_template_id,
	coalesce((settings_json->>'maxConcurrentServices')::integer, 10),
	coalesce((settings_json->>'timingFastCloseMinutes')::integer, 5),
	coalesce((settings_json->>'timingLongServiceMinutes')::integer, 25),
	coalesce((settings_json->>'timingLowSaleAmount')::numeric(14, 2), 1200),
	coalesce((settings_json->>'testModeEnabled')::boolean, false),
	coalesce((settings_json->>'autoFillFinishModal')::boolean, false),
	coalesce((settings_json->>'alertMinConversionRate')::numeric(8, 2), 0),
	coalesce((settings_json->>'alertMaxQueueJumpRate')::numeric(8, 2), 0),
	coalesce((settings_json->>'alertMinPaScore')::numeric(8, 2), 0),
	coalesce((settings_json->>'alertMinTicketAverage')::numeric(14, 2), 0),
	coalesce(modal_config_json->>'title', ''),
	coalesce(modal_config_json->>'productSeenLabel', ''),
	coalesce(modal_config_json->>'productSeenPlaceholder', ''),
	coalesce(modal_config_json->>'productClosedLabel', ''),
	coalesce(modal_config_json->>'productClosedPlaceholder', ''),
	coalesce(modal_config_json->>'notesLabel', ''),
	coalesce(modal_config_json->>'notesPlaceholder', ''),
	coalesce(modal_config_json->>'queueJumpReasonLabel', ''),
	coalesce(modal_config_json->>'queueJumpReasonPlaceholder', ''),
	coalesce(modal_config_json->>'lossReasonLabel', ''),
	coalesce(modal_config_json->>'lossReasonPlaceholder', ''),
	coalesce(modal_config_json->>'customerSectionLabel', ''),
	coalesce((modal_config_json->>'showEmailField')::boolean, true),
	coalesce((modal_config_json->>'showProfessionField')::boolean, true),
	coalesce((modal_config_json->>'showNotesField')::boolean, true),
	coalesce(nullif(modal_config_json->>'visitReasonSelectionMode', ''), 'multiple'),
	coalesce(nullif(modal_config_json->>'visitReasonDetailMode', ''), 'shared'),
	coalesce(nullif(modal_config_json->>'lossReasonSelectionMode', ''), 'single'),
	coalesce(nullif(modal_config_json->>'lossReasonDetailMode', ''), 'off'),
	coalesce(nullif(modal_config_json->>'customerSourceSelectionMode', ''), 'single'),
	coalesce(nullif(modal_config_json->>'customerSourceDetailMode', ''), 'shared'),
	coalesce((modal_config_json->>'requireProduct')::boolean, true),
	coalesce((modal_config_json->>'requireVisitReason')::boolean, true),
	coalesce((modal_config_json->>'requireCustomerSource')::boolean, true),
	coalesce((modal_config_json->>'requireCustomerNamePhone')::boolean, true),
	created_at,
	updated_at
from store_settings
on conflict (store_id) do update
set
	selected_operation_template_id = excluded.selected_operation_template_id,
	max_concurrent_services = excluded.max_concurrent_services,
	timing_fast_close_minutes = excluded.timing_fast_close_minutes,
	timing_long_service_minutes = excluded.timing_long_service_minutes,
	timing_low_sale_amount = excluded.timing_low_sale_amount,
	test_mode_enabled = excluded.test_mode_enabled,
	auto_fill_finish_modal = excluded.auto_fill_finish_modal,
	alert_min_conversion_rate = excluded.alert_min_conversion_rate,
	alert_max_queue_jump_rate = excluded.alert_max_queue_jump_rate,
	alert_min_pa_score = excluded.alert_min_pa_score,
	alert_min_ticket_average = excluded.alert_min_ticket_average,
	title = excluded.title,
	product_seen_label = excluded.product_seen_label,
	product_seen_placeholder = excluded.product_seen_placeholder,
	product_closed_label = excluded.product_closed_label,
	product_closed_placeholder = excluded.product_closed_placeholder,
	notes_label = excluded.notes_label,
	notes_placeholder = excluded.notes_placeholder,
	queue_jump_reason_label = excluded.queue_jump_reason_label,
	queue_jump_reason_placeholder = excluded.queue_jump_reason_placeholder,
	loss_reason_label = excluded.loss_reason_label,
	loss_reason_placeholder = excluded.loss_reason_placeholder,
	customer_section_label = excluded.customer_section_label,
	show_email_field = excluded.show_email_field,
	show_profession_field = excluded.show_profession_field,
	show_notes_field = excluded.show_notes_field,
	visit_reason_selection_mode = excluded.visit_reason_selection_mode,
	visit_reason_detail_mode = excluded.visit_reason_detail_mode,
	loss_reason_selection_mode = excluded.loss_reason_selection_mode,
	loss_reason_detail_mode = excluded.loss_reason_detail_mode,
	customer_source_selection_mode = excluded.customer_source_selection_mode,
	customer_source_detail_mode = excluded.customer_source_detail_mode,
	require_product = excluded.require_product,
	require_visit_reason = excluded.require_visit_reason,
	require_customer_source = excluded.require_customer_source,
	require_customer_name_phone = excluded.require_customer_name_phone,
	updated_at = now();

insert into store_setting_options (store_id, kind, option_id, label, sort_order, created_at, updated_at)
select
	settings.store_id,
	source.kind,
	source.item->>'id',
	source.item->>'label',
	(source.ordinality - 1)::integer,
	settings.created_at,
	settings.updated_at
from store_settings settings
cross join lateral (
	select 'visit_reason'::text as kind, item, ordinality
	from jsonb_array_elements(coalesce(settings.visit_reason_options_json, '[]'::jsonb)) with ordinality as visit(item, ordinality)
	union all
	select 'customer_source'::text as kind, item, ordinality
	from jsonb_array_elements(coalesce(settings.customer_source_options_json, '[]'::jsonb)) with ordinality as customer(item, ordinality)
	union all
	select 'queue_jump_reason'::text as kind, item, ordinality
	from jsonb_array_elements(coalesce(settings.queue_jump_reason_options_json, '[]'::jsonb)) with ordinality as queue_jump(item, ordinality)
	union all
	select 'loss_reason'::text as kind, item, ordinality
	from jsonb_array_elements(coalesce(settings.loss_reason_options_json, '[]'::jsonb)) with ordinality as loss(item, ordinality)
	union all
	select 'profession'::text as kind, item, ordinality
	from jsonb_array_elements(coalesce(settings.profession_options_json, '[]'::jsonb)) with ordinality as profession(item, ordinality)
) as source
where coalesce(source.item->>'id', '') <> ''
  and coalesce(source.item->>'label', '') <> ''
on conflict (store_id, kind, option_id) do update
set
	label = excluded.label,
	sort_order = excluded.sort_order,
	updated_at = now();

insert into store_catalog_products (store_id, product_id, name, code, category, base_price, sort_order, created_at, updated_at)
select
	settings.store_id,
	product.item->>'id',
	product.item->>'name',
	upper(coalesce(product.item->>'code', '')),
	coalesce(nullif(product.item->>'category', ''), 'Sem categoria'),
	coalesce((product.item->>'basePrice')::numeric(14, 2), 0),
	(product.ordinality - 1)::integer,
	settings.created_at,
	settings.updated_at
from store_settings settings
cross join lateral jsonb_array_elements(coalesce(settings.product_catalog_json, '[]'::jsonb)) with ordinality as product(item, ordinality)
where coalesce(product.item->>'id', '') <> ''
  and coalesce(product.item->>'name', '') <> ''
on conflict (store_id, product_id) do update
set
	name = excluded.name,
	code = excluded.code,
	category = excluded.category,
	base_price = excluded.base_price,
	sort_order = excluded.sort_order,
	updated_at = now();

drop table if exists store_settings;
