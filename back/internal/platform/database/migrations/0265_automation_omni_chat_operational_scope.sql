-- Correcao unica do legado: o editor antigo gravava na account ativa da agencia,
-- embora o Omni Chat estivesse sendo usado nas lojas de uma conta-cliente.
-- Para contas que realmente possuem lojas da fila, herda a configuracao mais
-- recente da account-agencia da mesma organizacao. Depois desta migration o
-- frontend grava diretamente na account operacional, portanto nao ha sync
-- continuo nem duas fontes de verdade.

update automation.omni_chat_configs client_config
set
    enabled = agency_config.enabled,
    system_prompt = agency_config.system_prompt,
    credential_id = agency_config.credential_id,
    provider = agency_config.provider,
    model = agency_config.model,
    temperature = agency_config.temperature,
    history_window = agency_config.history_window,
    updated_by = agency_config.updated_by,
    updated_at = now()
from core.accounts client_account
join lateral (
    select config.*, legacy_automation.updated_at as legacy_updated_at
    from core.accounts agency_account
    join automation.omni_chat_configs config
      on config.account_id = agency_account.id
    join automation.automations legacy_automation
      on legacy_automation.account_id = agency_account.id
     and legacy_automation.slug = 'tony'
    where agency_account.organization_id = client_account.organization_id
      and agency_account.is_agency = true
      and config.system_prompt <> ''
    order by legacy_automation.updated_at desc
    limit 1
) agency_config on true
where client_config.account_id = client_account.id
  and client_account.is_agency = false
  and exists (
      select 1
      from queue.stores store
      where store.tenant_id = client_account.id
  )
  and agency_config.legacy_updated_at >= coalesce((
      select client_automation.updated_at
      from automation.automations client_automation
      where client_automation.account_id = client_account.id
        and client_automation.slug = 'tony'
      limit 1
  ), '-infinity'::timestamptz);
