-- 0051_allow_hex_alert_colors.sql
-- Allows custom hexadecimal colors while preserving legacy named alert themes.

alter table if exists alert_rule_definitions
    drop constraint if exists alert_rule_definitions_color_theme_check;

alter table if exists alert_rule_definitions
    add constraint alert_rule_definitions_color_theme_check
    check (
        color_theme in ('amber', 'red', 'blue', 'green', 'purple', 'slate')
        or color_theme ~ '^#[0-9A-Fa-f]{6}$'
    );

alter table if exists alert_instances
    drop constraint if exists alert_instances_color_theme_check;

alter table if exists alert_instances
    add constraint alert_instances_color_theme_check
    check (
        color_theme in ('amber', 'red', 'blue', 'green', 'purple', 'slate')
        or color_theme ~ '^#[0-9A-Fa-f]{6}$'
    );
