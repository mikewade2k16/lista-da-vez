-- 0052_feedback_messages.sql
-- Stores the feedback conversation as lightweight threaded messages.

create table if not exists feedback_messages (
    id uuid primary key default gen_random_uuid(),
    tenant_id uuid references tenants(id),
    feedback_id uuid not null references user_feedback(id) on delete cascade,
    author_user_id uuid not null references users(id),
    author_name text not null default '',
    author_role text not null default '',
    body text not null,
    created_at timestamptz not null default now()
);

create index if not exists feedback_messages_tenant_id_idx on feedback_messages (tenant_id);
create index if not exists feedback_messages_feedback_id_created_at_idx on feedback_messages (feedback_id, created_at);

insert into feedback_messages (
    tenant_id, feedback_id, author_user_id, author_name, author_role, body, created_at
)
select
    tenant_id, id, user_id, user_name, 'user', body, created_at
from user_feedback
where not exists (
    select 1
    from feedback_messages
    where feedback_messages.feedback_id = user_feedback.id
);
