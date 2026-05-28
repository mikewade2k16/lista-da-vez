create table if not exists queue.tenant_ui_theme_settings (
	tenant_id         uuid primary key references tenants(id) on delete cascade,
	active_theme      text not null default 'light',
	custom_theme_name text not null default 'Custom',
	overrides         jsonb not null default '{}'::jsonb,
	updated_by        uuid null,
	updated_at        timestamptz not null default now()
);

create or replace view public.tenant_ui_theme_settings as
	select * from queue.tenant_ui_theme_settings;