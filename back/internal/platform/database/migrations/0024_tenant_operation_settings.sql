-- Move operation settings/options/products do escopo store_id para tenant_id.
-- As tabelas store_* permanecem intactas durante a transicao para permitir backfill seguro.
-- O service passa a ler/gravar nas tabelas tenant_*; o backfill abaixo popula o estado
-- inicial em ambiente de desenvolvimento copiando da loja mais antiga de cada tenant
-- (config) e fazendo uniao deduplicada (options e products).
-- Em producao, o backfill final sera definido manualmente antes do deploy.

create table if not exists tenant_operation_settings (
	tenant_id uuid primary key references tenants(id) on delete cascade,
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
	show_customer_name_field boolean not null default true,
	show_customer_phone_field boolean not null default true,
	show_email_field boolean not null default true,
	show_profession_field boolean not null default true,
	show_notes_field boolean not null default true,
	show_product_seen_field boolean not null default true,
	show_product_seen_notes_field boolean not null default true,
	show_product_closed_field boolean not null default true,
	show_visit_reason_field boolean not null default true,
	show_customer_source_field boolean not null default true,
	show_existing_customer_field boolean not null default true,
	show_queue_jump_reason_field boolean not null default true,
	show_loss_reason_field boolean not null default true,
	allow_product_seen_none boolean not null default true,
	visit_reason_selection_mode text not null default 'multiple',
	visit_reason_detail_mode text not null default 'shared',
	loss_reason_selection_mode text not null default 'single',
	loss_reason_detail_mode text not null default 'off',
	customer_source_selection_mode text not null default 'single',
	customer_source_detail_mode text not null default 'shared',
	require_customer_name_field boolean not null default true,
	require_customer_phone_field boolean not null default true,
	require_email_field boolean not null default false,
	require_profession_field boolean not null default false,
	require_notes_field boolean not null default false,
	require_product boolean not null default true,
	require_product_seen_field boolean not null default true,
	require_product_seen_notes_field boolean not null default false,
	require_product_closed_field boolean not null default true,
	require_visit_reason boolean not null default true,
	require_customer_source boolean not null default true,
	require_customer_name_phone boolean not null default true,
	require_product_seen_notes_when_none boolean not null default true,
	product_seen_notes_min_chars integer not null default 20,
	require_queue_jump_reason_field boolean not null default true,
	require_loss_reason_field boolean not null default true,
	created_at timestamptz not null default now(),
	updated_at timestamptz not null default now()
);

create table if not exists tenant_setting_options (
	tenant_id uuid not null references tenants(id) on delete cascade,
	kind text not null,
	option_id text not null,
	label text not null,
	sort_order integer not null default 0,
	created_at timestamptz not null default now(),
	updated_at timestamptz not null default now(),
	primary key (tenant_id, kind, option_id),
	constraint tenant_setting_options_kind_check check (
		kind in (
			'visit_reason',
			'customer_source',
			'pause_reason',
			'queue_jump_reason',
			'loss_reason',
			'profession'
		)
	)
);

create index if not exists tenant_setting_options_tenant_kind_idx
	on tenant_setting_options (tenant_id, kind, sort_order);

create table if not exists tenant_catalog_products (
	tenant_id uuid not null references tenants(id) on delete cascade,
	product_id text not null,
	name text not null,
	code text not null default '',
	category text not null default 'Sem categoria',
	base_price numeric(14, 2) not null default 0,
	sort_order integer not null default 0,
	created_at timestamptz not null default now(),
	updated_at timestamptz not null default now(),
	primary key (tenant_id, product_id)
);

create index if not exists tenant_catalog_products_tenant_idx
	on tenant_catalog_products (tenant_id, sort_order);

create unique index if not exists tenant_catalog_products_tenant_code_uidx
	on tenant_catalog_products (tenant_id, upper(code))
	where trim(code) <> '';

-- Backfill local: pra cada tenant com lojas, copia a config da loja mais antiga.
-- Em producao essa logica sera revista antes do deploy.
with primary_store as (
	select distinct on (s.tenant_id)
		s.tenant_id,
		s.id as store_id
	from stores s
	join store_operation_settings sos on sos.store_id = s.id
	order by s.tenant_id, s.created_at asc, s.id asc
)
insert into tenant_operation_settings (
	tenant_id,
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
	show_customer_name_field,
	show_customer_phone_field,
	show_email_field,
	show_profession_field,
	show_notes_field,
	show_product_seen_field,
	show_product_seen_notes_field,
	show_product_closed_field,
	show_visit_reason_field,
	show_customer_source_field,
	show_existing_customer_field,
	show_queue_jump_reason_field,
	show_loss_reason_field,
	allow_product_seen_none,
	visit_reason_selection_mode,
	visit_reason_detail_mode,
	loss_reason_selection_mode,
	loss_reason_detail_mode,
	customer_source_selection_mode,
	customer_source_detail_mode,
	require_customer_name_field,
	require_customer_phone_field,
	require_email_field,
	require_profession_field,
	require_notes_field,
	require_product,
	require_product_seen_field,
	require_product_seen_notes_field,
	require_product_closed_field,
	require_visit_reason,
	require_customer_source,
	require_customer_name_phone,
	require_product_seen_notes_when_none,
	product_seen_notes_min_chars,
	require_queue_jump_reason_field,
	require_loss_reason_field,
	created_at,
	updated_at
)
select
	ps.tenant_id,
	sos.selected_operation_template_id,
	sos.max_concurrent_services,
	sos.timing_fast_close_minutes,
	sos.timing_long_service_minutes,
	sos.timing_low_sale_amount,
	sos.test_mode_enabled,
	sos.auto_fill_finish_modal,
	sos.alert_min_conversion_rate,
	sos.alert_max_queue_jump_rate,
	sos.alert_min_pa_score,
	sos.alert_min_ticket_average,
	sos.title,
	sos.product_seen_label,
	sos.product_seen_placeholder,
	sos.product_closed_label,
	sos.product_closed_placeholder,
	sos.notes_label,
	sos.notes_placeholder,
	sos.queue_jump_reason_label,
	sos.queue_jump_reason_placeholder,
	sos.loss_reason_label,
	sos.loss_reason_placeholder,
	sos.customer_section_label,
	sos.show_customer_name_field,
	sos.show_customer_phone_field,
	sos.show_email_field,
	sos.show_profession_field,
	sos.show_notes_field,
	sos.show_product_seen_field,
	sos.show_product_seen_notes_field,
	sos.show_product_closed_field,
	sos.show_visit_reason_field,
	sos.show_customer_source_field,
	sos.show_existing_customer_field,
	sos.show_queue_jump_reason_field,
	sos.show_loss_reason_field,
	sos.allow_product_seen_none,
	sos.visit_reason_selection_mode,
	sos.visit_reason_detail_mode,
	sos.loss_reason_selection_mode,
	sos.loss_reason_detail_mode,
	sos.customer_source_selection_mode,
	sos.customer_source_detail_mode,
	sos.require_customer_name_field,
	sos.require_customer_phone_field,
	sos.require_email_field,
	sos.require_profession_field,
	sos.require_notes_field,
	sos.require_product,
	sos.require_product_seen_field,
	sos.require_product_seen_notes_field,
	sos.require_product_closed_field,
	sos.require_visit_reason,
	sos.require_customer_source,
	sos.require_customer_name_phone,
	sos.require_product_seen_notes_when_none,
	sos.product_seen_notes_min_chars,
	sos.require_queue_jump_reason_field,
	sos.require_loss_reason_field,
	sos.created_at,
	sos.updated_at
from primary_store ps
join store_operation_settings sos on sos.store_id = ps.store_id
on conflict (tenant_id) do nothing;

-- Backfill de options: uniao deduplicada por (tenant_id, kind, option_id),
-- mantendo o primeiro label encontrado (loja mais antiga vence).
insert into tenant_setting_options (tenant_id, kind, option_id, label, sort_order, created_at, updated_at)
select
	tenant_id,
	kind,
	option_id,
	label,
	sort_order,
	created_at,
	updated_at
from (
	select
		s.tenant_id,
		sso.kind,
		sso.option_id,
		sso.label,
		sso.sort_order,
		sso.created_at,
		sso.updated_at,
		row_number() over (
			partition by s.tenant_id, sso.kind, sso.option_id
			order by s.created_at asc, sso.sort_order asc, sso.created_at asc
		) as rn
	from store_setting_options sso
	join stores s on s.id = sso.store_id
) ordered
where rn = 1
on conflict (tenant_id, kind, option_id) do nothing;

-- Backfill de products: mesma logica de uniao deduplicada.
insert into tenant_catalog_products (
	tenant_id,
	product_id,
	name,
	code,
	category,
	base_price,
	sort_order,
	created_at,
	updated_at
)
select
	tenant_id,
	product_id,
	name,
	code,
	category,
	base_price,
	sort_order,
	created_at,
	updated_at
from (
	select
		s.tenant_id,
		scp.product_id,
		scp.name,
		scp.code,
		scp.category,
		scp.base_price,
		scp.sort_order,
		scp.created_at,
		scp.updated_at,
		row_number() over (
			partition by s.tenant_id, scp.product_id
			order by s.created_at asc, scp.sort_order asc, scp.created_at asc
		) as rn
	from store_catalog_products scp
	join stores s on s.id = scp.store_id
) ordered
where rn = 1
on conflict (tenant_id, product_id) do nothing;
