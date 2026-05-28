alter table user_feedback
    add column if not exists user_last_read_at timestamptz;

update user_feedback
set user_last_read_at = created_at
where user_last_read_at is null;

alter table user_feedback
    alter column user_last_read_at set default now();

alter table user_feedback
    alter column user_last_read_at set not null;

create index if not exists user_feedback_user_last_read_at_idx on user_feedback (user_last_read_at desc);