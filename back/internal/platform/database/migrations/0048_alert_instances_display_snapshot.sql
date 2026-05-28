-- 0047_alert_instances_display_snapshot.sql
-- Adds snapshot fields from rule definition to alert_instances
-- Ensures that editing a rule doesn't affect existing alerts

alter table alert_instances
    add column if not exists rule_definition_id uuid references alert_rule_definitions(id) on delete set null,
    add column if not exists display_kind varchar(30) not null default 'banner' check (display_kind in (
        'card_badge', 'banner', 'toast', 'corner_popup', 'center_modal', 'fullscreen'
    )),
    add column if not exists color_theme varchar(20) not null default 'amber' check (color_theme in (
        'amber', 'red', 'blue', 'green', 'purple', 'slate'
    )),
    add column if not exists response_options jsonb not null default '[]'::jsonb,
    add column if not exists is_mandatory boolean not null default false;

-- Expand interaction_kind CHECK constraint to include new values
alter table alert_instances
    drop constraint if exists alert_instances_interaction_kind_check;

alter table alert_instances
    add constraint alert_instances_interaction_kind_check check (
        interaction_kind in ('none', 'reminder', 'response_required', 'dismiss', 'confirm_choice', 'select_option')
    );

-- Backfill: existing long_open_service alerts get the classic banner display config
update alert_instances
set
    display_kind = 'banner',
    color_theme = 'amber',
    response_options = '[
        {
            "value": "still_happening",
            "label": "Ainda está acontecendo"
        },
        {
            "value": "forgotten",
            "label": "Esqueci de fechar"
        }
    ]'::jsonb,
    interaction_kind = 'confirm_choice'
where type = 'long_open_service'
    and interaction_kind != 'confirm_choice';

-- Create index for display kind queries
create index if not exists alert_instances_display_kind_idx
    on alert_instances (display_kind, store_id);

-- Rollback:
-- drop index if exists alert_instances_display_kind_idx;
-- alter table alert_instances
--     drop constraint if exists alert_instances_interaction_kind_check;
-- alter table alert_instances
--     add constraint alert_instances_interaction_kind_check check (
--         interaction_kind in ('none', 'reminder', 'response_required')
--     );
-- alter table alert_instances
--     drop column if exists rule_definition_id,
--     drop column if exists display_kind,
--     drop column if exists color_theme,
--     drop column if exists response_options,
--     drop column if exists is_mandatory;
