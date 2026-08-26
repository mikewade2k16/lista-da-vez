-- Escopo de clientes por board de Tasks.
-- O filtro e configuracao do board; tasks fora do recorte permanecem persistidas
-- e reaparecem se o escopo for ampliado.

alter table tasks.boards
    add column if not exists client_scope_mode text not null default 'active',
    add column if not exists client_scope_ids uuid[] not null default '{}'::uuid[];

do $$
begin
    if not exists (
        select 1
        from pg_constraint
        where conname = 'tasks_boards_client_scope_mode_check'
          and conrelid = 'tasks.boards'::regclass
    ) then
        alter table tasks.boards
            add constraint tasks_boards_client_scope_mode_check
            check (client_scope_mode in ('all', 'active', 'selected'));
    end if;
end
$$;

create index if not exists tasks_boards_client_scope_ids_gin_idx
    on tasks.boards using gin (client_scope_ids);
