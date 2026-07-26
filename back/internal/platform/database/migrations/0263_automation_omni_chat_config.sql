-- Configuracao autoritativa do Omni Chat por conta.
-- Uma unica linha vale para todas as lojas e todos os usuarios da account.
-- A chave permanece no cofre global messaging.ai_credentials; aqui fica apenas
-- a referencia opaca selecionada pelo administrador.

create table if not exists automation.omni_chat_configs (
    account_id uuid primary key references core.accounts(id) on delete cascade,
    enabled boolean not null default true,
    system_prompt text not null default '',
    credential_id uuid,
    provider text not null default 'openai'
        check (provider in ('openai', 'gemini', 'glm')),
    model text not null default 'gpt-4.1-mini',
    temperature numeric(3,2) not null default 0.20
        check (temperature between 0 and 1),
    history_window integer not null default 5
        check (history_window between 1 and 20),
    updated_by uuid,
    created_at timestamptz not null default now(),
    updated_at timestamptz not null default now()
);

create index if not exists automation_omni_chat_configs_credential_idx
    on automation.omni_chat_configs (account_id, credential_id)
    where credential_id is not null;

comment on table automation.omni_chat_configs is
    'Configuracao unica do Omni Chat por conta, compartilhada por todas as lojas e usuarios.';
comment on column automation.omni_chat_configs.credential_id is
    'Referencia opaca ao cofre global messaging.ai_credentials; nenhum segredo e persistido aqui.';

-- Preserva prompt e memoria da implementacao anterior e, quando existe, ativa
-- automaticamente a primeira chave OpenAI global da mesma conta.
insert into automation.omni_chat_configs (
    account_id,
    enabled,
    system_prompt,
    credential_id,
    provider,
    model,
    history_window
)
select
    automation_row.account_id,
    true,
    coalesce(automation_row.settings ->> 'omniChatPersona', ''),
    credential.id,
    coalesce(credential.provider, 'openai'),
    case
        when credential.provider = 'gemini' then 'gemini-2.5-flash'
        when credential.provider = 'glm' then 'glm-4.6'
        else 'gpt-4.1-mini'
    end,
    greatest(
        1,
        least(
            20,
            case
                when coalesce(automation_row.settings ->> 'omniChatHistoryWindow', '') ~ '^[0-9]+$'
                    then (automation_row.settings ->> 'omniChatHistoryWindow')::integer
                else 5
            end
        )
    )
from automation.automations automation_row
left join lateral (
    select id, provider
    from messaging.ai_credentials
    where account_id = automation_row.account_id
      and provider in ('openai', 'gemini', 'glm')
    order by
        case provider when 'openai' then 0 when 'gemini' then 1 else 2 end,
        created_at
    limit 1
) credential on true
where automation_row.slug = 'tony'
on conflict (account_id) do nothing;
