-- Corrige registros criados antes de show_cancel_reason_field e show_stop_reason_field
-- existirem, que ficaram com default false da migration 0037.
-- Todos os templates padrao definem esses campos como true, entao o comportamento
-- correto para qualquer configuracao que nao foi explicitamente ajustada e exibir o campo.
update tenant_operation_settings
set
    show_cancel_reason_field = true,
    show_stop_reason_field   = true
where
    show_cancel_reason_field = false
    and show_stop_reason_field = false;
