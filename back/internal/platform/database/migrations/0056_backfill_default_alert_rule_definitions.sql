-- 0056_backfill_default_alert_rule_definitions.sql
-- Ensures every tenant has the default long_open_service rule definition.

insert into alert_rule_definitions (
    tenant_id,
    name,
    description,
    is_active,
    trigger_type,
    threshold_minutes,
    severity,
    display_kind,
    color_theme,
    title_template,
    body_template,
    interaction_kind,
    response_options,
    is_mandatory,
    notify_dashboard,
    notify_operation_context,
    notify_external,
    external_channel
)
select
    tenants.id,
    'Atendimento longo (padrão)',
    'Alerta padrão para atendimentos que excedem o tempo configurado',
    true,
    'long_open_service',
    greatest(coalesce(rules.long_open_service_minutes, 25), 1),
    'critical',
    'banner',
    'amber',
    'Atendimento em aberto há {elapsed}',
    'O atendimento de {consultant} segue aberto acima do tempo configurado.',
    'confirm_choice',
    '[
        {
            "value": "still_happening",
            "label": "Ainda está acontecendo"
        },
        {
            "value": "forgotten",
            "label": "Esqueci de fechar"
        }
    ]'::jsonb,
    false,
    coalesce(rules.notify_dashboard, true),
    coalesce(rules.notify_operation_context, true),
    coalesce(rules.notify_external, false),
    'none'
from tenants
left join tenant_operational_alert_rules as rules on rules.tenant_id = tenants.id
on conflict (tenant_id, trigger_type, name) do nothing;