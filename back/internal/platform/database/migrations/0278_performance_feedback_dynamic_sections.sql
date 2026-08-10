-- Move o feedback de desempenho para blocos livres com titulo, preservando os
-- dois campos fixos da 0277 quando eles ja possuem conteudo.

alter table queue.performance_feedback_reviews
    add column if not exists feedback_sections jsonb not null default '[]'::jsonb;

update queue.performance_feedback_reviews
set feedback_sections = jsonb_build_array(
    jsonb_build_object(
        'id', 'strengths-and-opportunities',
        'title', 'Pontos fortes e oportunidades',
        'contentHtml', manager_notes_html
    ),
    jsonb_build_object(
        'id', 'action-plan',
        'title', 'Plano de acao e combinados',
        'contentHtml', action_plan_html
    )
)
where feedback_sections = '[]'::jsonb
  and (btrim(manager_notes_html) <> '' or btrim(action_plan_html) <> '');

alter table queue.performance_feedback_reviews
    drop column if exists manager_notes_html,
    drop column if exists action_plan_html;

do $$
begin
    if not exists (
        select 1
        from pg_constraint
        where conrelid = 'queue.performance_feedback_reviews'::regclass
          and conname = 'performance_feedback_sections_array_check'
    ) then
        alter table queue.performance_feedback_reviews
            add constraint performance_feedback_sections_array_check
            check (jsonb_typeof(feedback_sections) = 'array');
    end if;
end
$$;

insert into access_role_permissions (role, permission_key)
values
    ('consultant', 'workspace.consultor.view'),
    ('manager', 'workspace.consultor.view')
on conflict (role, permission_key) do nothing;

insert into core.permissions (key, module_id, label, description, scope)
values
    ('workspace.performance_feedback.view', 'core', 'Ver feedback no Consultor', 'Visualizar feedbacks de desempenho dentro da pagina Consultor.', 'account')
on conflict (key) do update
set label = excluded.label,
    description = excluded.description,
    scope = excluded.scope,
    deprecated_at = null;

with role_permissions(role_code, permission_key) as (
    values
        ('queue.consultant', 'workspace.consultor.view'),
        ('consultant', 'workspace.consultor.view'),
        ('queue.manager', 'workspace.consultor.view'),
        ('manager', 'workspace.consultor.view')
)
insert into core.role_permissions (role_id, permission_key)
select role.id, role_permissions.permission_key
from core.roles role
join role_permissions on lower(role.code) = role_permissions.role_code
on conflict (role_id, permission_key) do nothing;
