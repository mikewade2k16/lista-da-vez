-- Fase 4 da refatoracao de settings: converte as chaves do config jsonb de snake_case
-- para camelCase, alinhando com o formato gerado por json.Marshal do Go sobre ModalConfig.
-- Tambem adiciona a chave finishFlowMode ao jsonb (mesmo valor da coluna tipada).
--
-- A migration 0042 populou o config com chaves snake_case (ex: product_seen_label).
-- O codigo Go usa json.Marshal/json.Unmarshal com os json tags do struct (camelCase).
-- Este update converte as linhas existentes de uma vez, tornando o jsonb legivel pelo Go.
--
-- Idempotente: o predicado WHERE verifica se a chave 'productSeenLabel' (camelCase)
-- ainda nao existe. Reexecutar apos a conversao nao altera nada.
--
-- Rollback:
--   Nao ha rollback simples (forward-only). Para desfazer: restaurar backup pre-migracao
--   ou reexecutar o backfill da migration 0042 com DROP/CREATE da tabela.

update tenant_finish_modal_settings
set config = jsonb_build_object(
    'title',                        config->>'title',
    'finishFlowMode',               finish_flow_mode,
    'productSeenLabel',             config->>'product_seen_label',
    'productSeenPlaceholder',       config->>'product_seen_placeholder',
    'productClosedLabel',           config->>'product_closed_label',
    'productClosedPlaceholder',     config->>'product_closed_placeholder',
    'purchaseCodeLabel',            config->>'purchase_code_label',
    'purchaseCodePlaceholder',      config->>'purchase_code_placeholder',
    'notesLabel',                   config->>'notes_label',
    'notesPlaceholder',             config->>'notes_placeholder',
    'queueJumpReasonLabel',         config->>'queue_jump_reason_label',
    'queueJumpReasonPlaceholder',   config->>'queue_jump_reason_placeholder',
    'lossReasonLabel',              config->>'loss_reason_label',
    'lossReasonPlaceholder',        config->>'loss_reason_placeholder',
    'customerSectionLabel',         config->>'customer_section_label',
    'customerNameLabel',            config->>'customer_name_label',
    'customerPhoneLabel',           config->>'customer_phone_label',
    'customerEmailLabel',           config->>'customer_email_label',
    'customerProfessionLabel',      config->>'customer_profession_label',
    'existingCustomerLabel',        config->>'existing_customer_label',
    'productSeenNotesLabel',        config->>'product_seen_notes_label',
    'productSeenNotesPlaceholder',  config->>'product_seen_notes_placeholder',
    'visitReasonLabel',             config->>'visit_reason_label',
    'customerSourceLabel',          config->>'customer_source_label',
    'cancelReasonLabel',            config->>'cancel_reason_label',
    'cancelReasonPlaceholder',      config->>'cancel_reason_placeholder',
    'cancelReasonOtherLabel',       config->>'cancel_reason_other_label',
    'cancelReasonOtherPlaceholder', config->>'cancel_reason_other_placeholder',
    'stopReasonLabel',              config->>'stop_reason_label',
    'stopReasonPlaceholder',        config->>'stop_reason_placeholder',
    'stopReasonOtherLabel',         config->>'stop_reason_other_label',
    'stopReasonOtherPlaceholder',   config->>'stop_reason_other_placeholder'
)
|| jsonb_build_object(
    'showCustomerNameField',        (config->>'show_customer_name_field')::boolean,
    'showCustomerPhoneField',       (config->>'show_customer_phone_field')::boolean,
    'showEmailField',               (config->>'show_email_field')::boolean,
    'showProfessionField',          (config->>'show_profession_field')::boolean,
    'showNotesField',               (config->>'show_notes_field')::boolean,
    'showProductSeenField',         (config->>'show_product_seen_field')::boolean,
    'showProductSeenNotesField',    (config->>'show_product_seen_notes_field')::boolean,
    'showProductClosedField',       (config->>'show_product_closed_field')::boolean,
    'showPurchaseCodeField',        (config->>'show_purchase_code_field')::boolean,
    'showVisitReasonField',         (config->>'show_visit_reason_field')::boolean,
    'showCustomerSourceField',      (config->>'show_customer_source_field')::boolean,
    'showExistingCustomerField',    (config->>'show_existing_customer_field')::boolean,
    'showQueueJumpReasonField',     (config->>'show_queue_jump_reason_field')::boolean,
    'showLossReasonField',          (config->>'show_loss_reason_field')::boolean,
    'showCancelReasonField',        (config->>'show_cancel_reason_field')::boolean,
    'showStopReasonField',          (config->>'show_stop_reason_field')::boolean,
    'allowProductSeenNone',         (config->>'allow_product_seen_none')::boolean,
    'visitReasonSelectionMode',     config->>'visit_reason_selection_mode',
    'visitReasonDetailMode',        config->>'visit_reason_detail_mode',
    'lossReasonSelectionMode',      config->>'loss_reason_selection_mode',
    'lossReasonDetailMode',         config->>'loss_reason_detail_mode',
    'customerSourceSelectionMode',  config->>'customer_source_selection_mode',
    'customerSourceDetailMode',     config->>'customer_source_detail_mode',
    'cancelReasonInputMode',        config->>'cancel_reason_input_mode',
    'stopReasonInputMode',          config->>'stop_reason_input_mode'
)
|| jsonb_build_object(
    'requireCustomerNameField',         (config->>'require_customer_name_field')::boolean,
    'requireCustomerPhoneField',        (config->>'require_customer_phone_field')::boolean,
    'requireEmailField',                (config->>'require_email_field')::boolean,
    'requireProfessionField',           (config->>'require_profession_field')::boolean,
    'requireNotesField',                (config->>'require_notes_field')::boolean,
    'requireProduct',                   (config->>'require_product')::boolean,
    'requireProductSeenField',          (config->>'require_product_seen_field')::boolean,
    'requireProductSeenNotesField',     (config->>'require_product_seen_notes_field')::boolean,
    'requireProductClosedField',        (config->>'require_product_closed_field')::boolean,
    'requirePurchaseCodeField',         (config->>'require_purchase_code_field')::boolean,
    'requireVisitReason',               (config->>'require_visit_reason')::boolean,
    'requireCustomerSource',            (config->>'require_customer_source')::boolean,
    'requireCustomerNamePhone',         (config->>'require_customer_name_phone')::boolean,
    'requireProductSeenNotesWhenNone',  (config->>'require_product_seen_notes_when_none')::boolean,
    'productSeenNotesMinChars',         (config->>'product_seen_notes_min_chars')::integer,
    'requireQueueJumpReasonField',      (config->>'require_queue_jump_reason_field')::boolean,
    'requireLossReasonField',           (config->>'require_loss_reason_field')::boolean,
    'requireCancelReasonField',         (config->>'require_cancel_reason_field')::boolean,
    'requireStopReasonField',           (config->>'require_stop_reason_field')::boolean
)
where schema_version = 1
    and not (config ? 'productSeenLabel');
