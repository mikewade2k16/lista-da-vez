-- Fase 3 da refatoracao de settings: tabela de configuracao do modal de encerramento.
-- finish_flow_mode fica como coluna tipada (discriminador de modo).
-- schema_version permite evoluir o formato do config jsonb sem quebrar leituras antigas.
-- config jsonb armazena todos os campos do modal exceto finish_flow_mode.
-- tenant_operation_settings continua existindo e nao e alterada por esta migration.
--
-- Schema v1 do config jsonb: labels, placeholders, show/hide, required,
-- modos de selecao e modos de detalhe de todos os campos do modal.
--
-- Rollback:
--   drop table if exists tenant_finish_modal_settings;

create table if not exists tenant_finish_modal_settings (
    tenant_id        uuid primary key references tenants(id) on delete cascade,
    finish_flow_mode text not null default 'legacy',
    schema_version   integer not null default 1,
    config           jsonb not null default '{}'::jsonb,
    updated_by       uuid null,
    updated_at       timestamptz not null default now()
);

-- Backfill a partir de tenant_operation_settings.
-- Usa coalesce nas colunas adicionadas em migrations posteriores que podem ser nulas
-- em ambientes antigos (purchase_code_*, cancel_reason_*, stop_reason_*,
-- show_purchase_code_field, show_cancel_reason_field, show_stop_reason_field,
-- cancel_reason_input_mode, stop_reason_input_mode, require_purchase_code_field,
-- require_cancel_reason_field, require_stop_reason_field).
-- on conflict do nothing: seguro para reexecutar sem risco de sobrescrita.
insert into tenant_finish_modal_settings (
    tenant_id,
    finish_flow_mode,
    schema_version,
    config,
    updated_at
)
select
    tenant_id,
    coalesce(finish_flow_mode, 'legacy'),
    1,
    jsonb_build_object(
        'title',                           title,
        'product_seen_label',              product_seen_label,
        'product_seen_placeholder',        product_seen_placeholder,
        'product_closed_label',            product_closed_label,
        'product_closed_placeholder',      product_closed_placeholder,
        'purchase_code_label',             coalesce(purchase_code_label, ''),
        'purchase_code_placeholder',       coalesce(purchase_code_placeholder, ''),
        'notes_label',                     notes_label,
        'notes_placeholder',               notes_placeholder,
        'queue_jump_reason_label',         queue_jump_reason_label,
        'queue_jump_reason_placeholder',   queue_jump_reason_placeholder,
        'loss_reason_label',               loss_reason_label,
        'loss_reason_placeholder',         loss_reason_placeholder,
        'customer_section_label',          customer_section_label,
        'customer_name_label',             customer_name_label,
        'customer_phone_label',            customer_phone_label,
        'customer_email_label',            customer_email_label,
        'customer_profession_label',       customer_profession_label,
        'existing_customer_label',         existing_customer_label,
        'product_seen_notes_label',        product_seen_notes_label,
        'product_seen_notes_placeholder',  product_seen_notes_placeholder,
        'visit_reason_label',              visit_reason_label,
        'customer_source_label',           customer_source_label,
        'cancel_reason_label',             coalesce(cancel_reason_label, ''),
        'cancel_reason_placeholder',       coalesce(cancel_reason_placeholder, ''),
        'cancel_reason_other_label',       coalesce(cancel_reason_other_label, ''),
        'cancel_reason_other_placeholder', coalesce(cancel_reason_other_placeholder, ''),
        'stop_reason_label',               coalesce(stop_reason_label, ''),
        'stop_reason_placeholder',         coalesce(stop_reason_placeholder, ''),
        'stop_reason_other_label',         coalesce(stop_reason_other_label, ''),
        'stop_reason_other_placeholder',   coalesce(stop_reason_other_placeholder, '')
    )
    || jsonb_build_object(
        'show_customer_name_field',        show_customer_name_field,
        'show_customer_phone_field',       show_customer_phone_field,
        'show_email_field',                show_email_field,
        'show_profession_field',           show_profession_field,
        'show_notes_field',                show_notes_field,
        'show_product_seen_field',         show_product_seen_field,
        'show_product_seen_notes_field',   show_product_seen_notes_field,
        'show_product_closed_field',       show_product_closed_field,
        'show_purchase_code_field',        coalesce(show_purchase_code_field, true),
        'show_visit_reason_field',         show_visit_reason_field,
        'show_customer_source_field',      show_customer_source_field,
        'show_existing_customer_field',    show_existing_customer_field,
        'show_queue_jump_reason_field',    show_queue_jump_reason_field,
        'show_loss_reason_field',          show_loss_reason_field,
        'show_cancel_reason_field',        coalesce(show_cancel_reason_field, false),
        'show_stop_reason_field',          coalesce(show_stop_reason_field, false),
        'allow_product_seen_none',         allow_product_seen_none,
        'visit_reason_selection_mode',     visit_reason_selection_mode,
        'visit_reason_detail_mode',        visit_reason_detail_mode,
        'loss_reason_selection_mode',      loss_reason_selection_mode,
        'loss_reason_detail_mode',         loss_reason_detail_mode,
        'customer_source_selection_mode',  customer_source_selection_mode,
        'customer_source_detail_mode',     customer_source_detail_mode,
        'cancel_reason_input_mode',        coalesce(cancel_reason_input_mode, 'text'),
        'stop_reason_input_mode',          coalesce(stop_reason_input_mode, 'text')
    )
    || jsonb_build_object(
        'require_customer_name_field',         require_customer_name_field,
        'require_customer_phone_field',        require_customer_phone_field,
        'require_email_field',                 require_email_field,
        'require_profession_field',            require_profession_field,
        'require_notes_field',                 require_notes_field,
        'require_product',                     require_product,
        'require_product_seen_field',          require_product_seen_field,
        'require_product_seen_notes_field',    require_product_seen_notes_field,
        'require_product_closed_field',        require_product_closed_field,
        'require_purchase_code_field',         coalesce(require_purchase_code_field, true),
        'require_visit_reason',                require_visit_reason,
        'require_customer_source',             require_customer_source,
        'require_customer_name_phone',         require_customer_name_phone,
        'require_product_seen_notes_when_none', require_product_seen_notes_when_none,
        'product_seen_notes_min_chars',        product_seen_notes_min_chars,
        'require_queue_jump_reason_field',     require_queue_jump_reason_field,
        'require_loss_reason_field',           require_loss_reason_field,
        'require_cancel_reason_field',         coalesce(require_cancel_reason_field, false),
        'require_stop_reason_field',           coalesce(require_stop_reason_field, false)
    ),
    updated_at
from tenant_operation_settings
on conflict (tenant_id) do nothing;
