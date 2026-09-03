-- Converte os clientId numericos gravados pelo prototipo antigo de Tasks para
-- client_account_id (UUID de core.accounts). O mapa e o contrato historico do
-- importador legado; a conta alvo ainda precisa existir na MESMA organizacao.
-- Assim a migration nunca inventa cliente nem cruza tenants.

with legacy_map(legacy_id, account_slug) as (
    values
        ('1',   'perola'),
        ('2',   'dr-antonio-tavares'),
        ('4',   'crow'),
        ('5',   'cleo-moraes'),
        ('6',   'am-malls'),
        ('7',   'uno'),
        ('8',   'juliana-oliveira'),
        ('10',  'mostarda'),
        ('11',  'duby'),
        ('101', 'perola'),
        ('104', 'dr-antonio-tavares'),
        ('105', 'uno'),
        ('106', 'crow')
), resolved as (
    select task.id as task_id, target.id as client_account_id, target.name as client_name
      from tasks.tasks task
      join core.accounts owner on owner.id = task.account_id
      join legacy_map mapping on mapping.legacy_id = btrim(task.ui_metadata->>'clientId')
      join core.accounts target
        on target.organization_id = owner.organization_id
       and lower(target.slug) = mapping.account_slug
     where task.client_account_id is null
)
update tasks.tasks task
   set client_account_id = resolved.client_account_id,
       ui_metadata = jsonb_set(
           coalesce(task.ui_metadata, '{}'::jsonb) - 'clientId',
           '{clientName}',
           to_jsonb(resolved.client_name),
           true
       ),
       updated_at = now()
  from resolved
 where task.id = resolved.task_id;

-- Zero era o valor do antigo "sem cliente", nao um cliente real.
update tasks.tasks
   set ui_metadata = coalesce(ui_metadata, '{}'::jsonb) - 'clientId' - 'clientName',
       updated_at = now()
 where client_account_id is null
   and btrim(ui_metadata->>'clientId') = '0';
