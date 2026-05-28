create table if not exists feedback_read_states (
    feedback_id uuid not null references user_feedback(id) on delete cascade,
    user_id uuid not null references users(id) on delete cascade,
    last_read_at timestamptz not null default now(),
    created_at timestamptz not null default now(),
    updated_at timestamptz not null default now(),
    primary key (feedback_id, user_id)
);

create index if not exists feedback_read_states_user_id_idx
    on feedback_read_states (user_id, last_read_at desc);

insert into feedback_read_states (
    feedback_id, user_id, last_read_at
)
select
    id,
    user_id,
    coalesce(user_last_read_at, created_at)
from user_feedback
on conflict (feedback_id, user_id) do nothing;