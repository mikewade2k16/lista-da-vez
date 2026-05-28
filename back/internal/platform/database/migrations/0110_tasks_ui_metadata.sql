-- Persistencia server-side dos campos visuais do Tasks MVP.
-- Evita divergencia entre navegadores enquanto a fase completa de field_values
-- configuraveis nao substitui a ponte front-first.

alter table tasks.tasks
    add column if not exists ui_metadata jsonb not null default '{}'::jsonb;
