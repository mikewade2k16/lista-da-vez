-- Paginas de Tasks podem funcionar como visualizacoes salvas sobre outras paginas.
-- O padrao continua isolado: somente tasks pertencentes ao proprio board.

alter table tasks.boards
    add column if not exists task_source_mode text not null default 'own',
    add column if not exists task_source_board_ids uuid[] not null default '{}'::uuid[];

do $$
begin
    if not exists (
        select 1
        from pg_constraint
        where conname = 'tasks_boards_task_source_mode_check'
          and conrelid = 'tasks.boards'::regclass
    ) then
        alter table tasks.boards
            add constraint tasks_boards_task_source_mode_check
            check (task_source_mode in ('own', 'all', 'selected'));
    end if;
end
$$;

create index if not exists tasks_boards_task_source_ids_gin_idx
    on tasks.boards using gin (task_source_board_ids);

create table if not exists tasks.user_preferences (
    account_id uuid not null references core.accounts(id) on delete cascade,
    user_id uuid not null references core.users(id) on delete cascade,
    last_board_id uuid not null,
    updated_at timestamptz not null default now(),
    primary key (account_id, user_id),
    constraint tasks_user_preferences_last_board_fk
        foreign key (last_board_id, account_id)
        references tasks.boards(id, account_id)
        on delete cascade
);

create index if not exists tasks_user_preferences_last_board_idx
    on tasks.user_preferences (last_board_id, account_id);
