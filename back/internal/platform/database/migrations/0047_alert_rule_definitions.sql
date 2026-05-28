-- 0046_alert_rule_definitions.sql
-- Creates alert_rule_definitions table for dynamic alert rule management
-- Backfills long_open_service rule for each existing tenant

create table if not exists alert_rule_definitions (
    id uuid primary key default gen_random_uuid(),
    tenant_id uuid not null references tenants(id) on delete cascade,
    name text not null,
    description text not null default '',
    is_active boolean not null default true,

    -- Trigger configuration
    trigger_type varchar(40) not null check (trigger_type in (
        'long_open_service',
        'long_queue_wait',
        'long_pause',
        'idle_store',
        'outside_business_hours'
    )),
    threshold_minutes integer not null check (threshold_minutes > 0),
    severity varchar(20) not null default 'attention' check (severity in ('info', 'attention', 'critical')),

    -- Display configuration
    display_kind varchar(30) not null default 'banner' check (display_kind in (
        'card_badge', 'banner', 'toast', 'corner_popup', 'center_modal', 'fullscreen'
    )),
    color_theme varchar(20) not null default 'amber' check (color_theme in (
        'amber', 'red', 'blue', 'green', 'purple', 'slate'
    )),
    title_template text not null,
    body_template text not null default '',

    -- Interaction configuration
    interaction_kind varchar(30) not null default 'none' check (interaction_kind in (
        'none', 'dismiss', 'confirm_choice', 'select_option'
    )),
    response_options jsonb not null default '[]'::jsonb,
    is_mandatory boolean not null default false,

    -- Notification channels
    notify_dashboard boolean not null default true,
    notify_operation_context boolean not null default true,
    notify_external boolean not null default false,
    external_channel varchar(20) not null default 'none' check (external_channel in (
        'none', 'whatsapp', 'email'
    )),

    -- Audit
    created_by uuid references users(id) on delete set null,
    updated_by uuid references users(id) on delete set null,
    created_at timestamptz not null default now(),
    updated_at timestamptz not null default now()
);

-- Index for common query patterns
create index if not exists alert_rule_definitions_tenant_active_idx
    on alert_rule_definitions (tenant_id, is_active, trigger_type);

with ranked as (
    select
        id,
        row_number() over (
            partition by tenant_id, trigger_type, name
            order by is_active desc, updated_at desc, created_at desc, id
        ) as row_rank
    from alert_rule_definitions
)
delete from alert_rule_definitions rule_definitions
using ranked
where rule_definitions.id = ranked.id
  and ranked.row_rank > 1;

create unique index if not exists alert_rule_definitions_tenant_trigger_name_uidx
    on alert_rule_definitions (tenant_id, trigger_type, name);

create index if not exists alert_rule_definitions_trigger_type_idx
    on alert_rule_definitions (trigger_type);

-- Backfill: create long_open_service rule for each existing tenant
-- Source: tenant_operational_alert_rules
insert into alert_rule_definitions (
    tenant_id,
    name,
    description,
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
    tenant_id,
    'Atendimento longo (padrão)',
    'Alerta padrão para atendimentos que excedem o tempo configurado',
    'long_open_service',
    long_open_service_minutes,
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
    notify_dashboard,
    notify_operation_context,
    notify_external,
    'none'
from tenant_operational_alert_rules
on conflict (tenant_id, trigger_type, name) do nothing;

-- Rollback:
-- drop index if exists alert_rule_definitions_tenant_trigger_name_uidx;
-- drop index if exists alert_rule_definitions_trigger_type_idx;
-- drop index if exists alert_rule_definitions_tenant_active_idx;
-- drop table if exists alert_rule_definitions;
