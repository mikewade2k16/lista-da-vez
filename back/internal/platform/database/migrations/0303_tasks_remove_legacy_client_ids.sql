-- Tasks que ja tinham client_account_id real ainda podiam carregar uma copia
-- numerica antiga em ui_metadata. Remove essa duplicidade e mantem o nome vindo
-- da account autoritativa, sem alterar o escopo/cliente associado.

update tasks.tasks task
   set ui_metadata = jsonb_set(
           coalesce(task.ui_metadata, '{}'::jsonb) - 'clientId',
           '{clientName}',
           to_jsonb(client.name),
           true
       ),
       updated_at = now()
  from core.accounts client
 where client.id = task.client_account_id
   and coalesce(task.ui_metadata->>'clientId', '') ~ '^[0-9]+$';
