-- Ativa o Omni Chat de contas-cliente com uma credencial global da propria
-- conta ou da conta-agencia da mesma organizacao. O runtime ja valida esse
-- compartilhamento pela fachada ResolveRuntimeCredential.

update automation.omni_chat_configs config
set
    credential_id = candidate.id,
    provider = candidate.provider,
    model = case
        when candidate.provider = 'gemini' then 'gemini-2.5-flash'
        when candidate.provider = 'glm' then 'glm-4.6'
        else 'gpt-4.1-mini'
    end,
    updated_at = now()
from core.accounts consumer
join lateral (
    select credential.id, credential.provider
    from core.accounts owner
    join messaging.ai_credentials credential
      on credential.account_id = owner.id
    where owner.organization_id = consumer.organization_id
      and (owner.id = consumer.id or owner.is_agency = true)
      and credential.provider in ('openai', 'gemini', 'glm')
    order by
        case when owner.id = consumer.id then 0 else 1 end,
        case credential.provider when 'openai' then 0 when 'gemini' then 1 else 2 end,
        credential.created_at
    limit 1
) candidate on true
where config.account_id = consumer.id
  and config.credential_id is null;
