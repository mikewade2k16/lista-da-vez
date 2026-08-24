-- Permite credenciais Anthropic somente no cofre nomeado e no Assistente 360.
-- O keyring legado dos agentes Omnichannel permanece inalterado.

do $$
declare
    constraint_name text;
begin
    for constraint_name in
        select conname
        from pg_constraint
        where conrelid = 'messaging.ai_credentials'::regclass
          and contype = 'c'
          and pg_get_constraintdef(oid) ilike '%provider%'
    loop
        execute format('alter table messaging.ai_credentials drop constraint %I', constraint_name);
    end loop;
end
$$;

alter table messaging.ai_credentials
    add constraint messaging_ai_credentials_provider_check
    check (provider in ('openai', 'anthropic', 'gemini', 'glm'));

do $$
declare
    constraint_name text;
begin
    for constraint_name in
        select conname
        from pg_constraint
        where conrelid = 'automation.omni_chat_configs'::regclass
          and contype = 'c'
          and pg_get_constraintdef(oid) ilike '%provider%'
    loop
        execute format('alter table automation.omni_chat_configs drop constraint %I', constraint_name);
    end loop;
end
$$;

alter table automation.omni_chat_configs
    add constraint automation_omni_chat_configs_provider_check
    check (provider in ('openai', 'anthropic', 'gemini', 'glm'));

comment on column messaging.ai_credentials.provider is
    'Provedor da credencial nomeada: openai, anthropic, gemini ou glm.';
