-- A analise de transcricoes passa a referenciar o cofre global de credenciais
-- da conta. Queue guarda somente o identificador; o segredo e resolvido pela
-- fachada server-side do Omnichannel no composition root.

alter table queue.attendance_analysis_configs
    add column if not exists credential_id uuid;

-- Transicao idempotente para instalacoes que ja salvaram a chave dedicada da
-- primeira versao do MVP. A tabela legada e mantida apenas para rollback.
insert into messaging.ai_credentials (
    account_id,
    name,
    provider,
    secret_ciphertext,
    secret_last4,
    created_by
)
select
    secret.account_id,
    'transcricoes_' || coalesce(nullif(secret.api_key_last4, ''), 'migrada')
        || '_' || substr(md5(secret.api_key_ciphertext), 1, 8),
    config.provider,
    secret.api_key_ciphertext,
    secret.api_key_last4,
    secret.updated_by
from queue.attendance_analysis_secrets secret
join queue.attendance_analysis_configs config
  on config.account_id = secret.account_id
where config.credential_id is null
  and config.provider in ('gemini', 'openai')
on conflict do nothing;

update queue.attendance_analysis_configs config
set
    credential_id = credential.id,
    updated_at = now()
from queue.attendance_analysis_secrets secret,
     messaging.ai_credentials credential
where secret.account_id = config.account_id
  and credential.account_id = config.account_id
  and credential.provider = config.provider
  and credential.name = (
      'transcricoes_' || coalesce(nullif(secret.api_key_last4, ''), 'migrada')
      || '_' || substr(md5(secret.api_key_ciphertext), 1, 8)
  )
  and config.credential_id is null;

create index if not exists queue_attendance_analysis_configs_credential_idx
    on queue.attendance_analysis_configs (account_id, credential_id)
    where credential_id is not null;

comment on column queue.attendance_analysis_configs.credential_id is
    'Referencia opaca ao cofre global account-scoped messaging.ai_credentials; sem segredo no schema Queue.';
