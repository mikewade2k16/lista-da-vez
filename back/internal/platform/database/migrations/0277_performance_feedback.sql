-- Feedbacks de desempenho de consultores. Dominio separado do canal de Suporte
-- legado (`queue.user_feedback`). O snapshot garante comparacao historica mesmo
-- quando metas e indicadores autoritativos mudarem depois da reuniao.

create table if not exists queue.performance_feedback_reviews (
    id uuid primary key default gen_random_uuid(),
    tenant_id uuid not null,
    store_id uuid not null references queue.stores(id) on delete cascade,
    consultant_id uuid not null references queue.consultants(id) on delete restrict,
    consultant_user_id uuid null,
    period_month date not null,
    week smallint not null default 0 check (week between 0 and 4),
    status text not null default 'draft'
        check (status in ('draft', 'shared', 'acknowledged')),
    manager_notes_html text not null default '',
    action_plan_html text not null default '',
    consultant_notes_html text not null default '',
    metrics_snapshot jsonb not null default '{}'::jsonb
        check (jsonb_typeof(metrics_snapshot) = 'object'),
    created_by_user_id uuid null,
    updated_by_user_id uuid null,
    shared_at timestamptz null,
    acknowledged_at timestamptz null,
    created_at timestamptz not null default now(),
    updated_at timestamptz not null default now(),
    version integer not null default 1 check (version > 0),
    unique (tenant_id, store_id, consultant_id, period_month, week)
);

create index if not exists performance_feedback_store_period_idx
    on queue.performance_feedback_reviews (tenant_id, store_id, period_month desc, week);

create index if not exists performance_feedback_consultant_period_idx
    on queue.performance_feedback_reviews
        (tenant_id, consultant_id, period_month desc, week desc, updated_at desc);

insert into access_permissions (key, scope, description)
values
    ('workspace.performance_feedback.view', 'tenant', 'Visualizar feedbacks de desempenho.'),
    ('workspace.performance_feedback.edit', 'tenant', 'Conduzir feedbacks de desempenho.')
on conflict (key) do update
set scope = excluded.scope,
    description = excluded.description;

insert into access_role_permissions (role, permission_key)
values
    ('consultant', 'workspace.performance_feedback.view'),
    ('manager', 'workspace.performance_feedback.view'),
    ('manager', 'workspace.performance_feedback.edit'),
    ('owner', 'workspace.performance_feedback.view'),
    ('owner', 'workspace.performance_feedback.edit'),
    ('platform_admin', 'workspace.performance_feedback.view'),
    ('platform_admin', 'workspace.performance_feedback.edit')
on conflict (role, permission_key) do nothing;

insert into core.modules (id, schema_name, label, is_core)
values ('core', 'core', 'Plataforma core', true)
on conflict (id) do nothing;

insert into core.permissions (key, module_id, label, description, scope)
values
    ('workspace.performance_feedback.view', 'core', 'Ver pagina Feedback', 'Visualizar feedbacks de desempenho dos consultores.', 'account'),
    ('workspace.performance_feedback.edit', 'core', 'Conduzir feedback', 'Registrar feedback e plano de acao dos consultores.', 'account')
on conflict (key) do update
set label = excluded.label,
    description = excluded.description,
    scope = excluded.scope,
    deprecated_at = null;

with role_permissions(role_code, permission_key) as (
    values
        ('queue.consultant', 'workspace.performance_feedback.view'),
        ('consultant', 'workspace.performance_feedback.view'),
        ('queue.manager', 'workspace.performance_feedback.view'),
        ('queue.manager', 'workspace.performance_feedback.edit'),
        ('manager', 'workspace.performance_feedback.view'),
        ('manager', 'workspace.performance_feedback.edit'),
        ('queue.owner', 'workspace.performance_feedback.view'),
        ('queue.owner', 'workspace.performance_feedback.edit'),
        ('owner', 'workspace.performance_feedback.view'),
        ('owner', 'workspace.performance_feedback.edit')
)
insert into core.role_permissions (role_id, permission_key)
select role.id, role_permissions.permission_key
from core.roles role
join role_permissions on lower(role.code) = role_permissions.role_code
on conflict (role_id, permission_key) do nothing;
