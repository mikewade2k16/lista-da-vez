-- Impede que a exclusao de uma credencial global deixe configuracoes do
-- Assistente 360 ou da analise de atendimentos apontando para um segredo
-- inexistente. NOT VALID preserva instalacoes com legado inconsistente sem
-- abrir mao da verificacao para novas escritas e deletes do registro pai.

do $$
begin
    if not exists (
        select 1
        from pg_constraint
        where conname = 'automation_omni_chat_configs_credential_fk'
          and conrelid = 'automation.omni_chat_configs'::regclass
    ) then
        alter table automation.omni_chat_configs
            add constraint automation_omni_chat_configs_credential_fk
            foreign key (credential_id)
            references messaging.ai_credentials(id)
            on delete restrict
            not valid;
    end if;
end
$$;

do $$
begin
    if not exists (
        select 1
        from pg_constraint
        where conname = 'queue_attendance_analysis_configs_credential_fk'
          and conrelid = 'queue.attendance_analysis_configs'::regclass
    ) then
        alter table queue.attendance_analysis_configs
            add constraint queue_attendance_analysis_configs_credential_fk
            foreign key (account_id, credential_id)
            references messaging.ai_credentials(account_id, id)
            on delete restrict
            not valid;
    end if;
end
$$;
