-- Configuracao autoritativa dos ciclos de feedback por tenant.

create table if not exists queue.performance_feedback_settings (
    tenant_id uuid primary key,
    cadence text not null default 'monthly'
        check (cadence in ('monthly', 'weekly')),
    default_sections jsonb not null default '[]'::jsonb
        check (jsonb_typeof(default_sections) = 'array'),
    updated_by_user_id uuid null,
    created_at timestamptz not null default now(),
    updated_at timestamptz not null default now(),
    version integer not null default 1 check (version > 0)
);

create index if not exists performance_feedback_settings_updated_idx
    on queue.performance_feedback_settings (tenant_id, updated_at desc);
