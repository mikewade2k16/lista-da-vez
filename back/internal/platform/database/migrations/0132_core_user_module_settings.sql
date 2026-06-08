-- 0132 — core.user_module_settings: config/opcoes especificas por modulo por usuario.
-- Plano: docs/USER_MODEL_UNIFICATION_PLAN.md (estagio U1).
--
-- Um usuario = uma linha em core.users (fonte da verdade). Config que e' especifica
-- de UM modulo (ex.: opcoes da Fila por usuario: employee_code, store, link de
-- consultor) vive aqui, NAO em tabela legada nem espalhada em core.users.
-- Cada modulo le/escreve so a sua linha (user_id, module_id).

create table if not exists core.user_module_settings (
    user_id    uuid  not null references core.users(id) on delete cascade,
    module_id  text  not null,
    config     jsonb not null default '{}'::jsonb,
    created_at timestamptz not null default now(),
    updated_at timestamptz not null default now(),
    primary key (user_id, module_id)
);

create index if not exists core_user_module_settings_module_idx
    on core.user_module_settings (module_id);
