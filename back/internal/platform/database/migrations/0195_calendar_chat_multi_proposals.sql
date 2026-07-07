-- Chat do calendario: MULTIPLAS propostas por mensagem (multi-tarefa pelo chat).
-- `proposals` e um array [{id,kind,fields,status}]; cada proposta tem status proprio
-- (pending|accepted|rejected) e um id estavel (indice dentro da mensagem). O
-- proposal/proposal_status SINGULAR fica so para retrocompat de leitura; o backfill
-- abaixo migra as mensagens antigas para a lista de 1.

alter table calendar.chat_messages
  add column if not exists proposals jsonb not null default '[]'::jsonb;

-- Backfill idempotente: mensagem antiga com proposal unico vira lista de 1 (id '0',
-- status = o proposal_status atual). So toca em quem tem proposal e ainda esta com
-- proposals vazio, entao rodar de novo e no-op.
update calendar.chat_messages
set proposals = jsonb_build_array(jsonb_build_object(
  'id', '0',
  'kind', proposal->>'kind',
  'fields', coalesce(proposal->'fields', '{}'::jsonb),
  'status', proposal_status
))
where proposal is not null
  and (proposals is null or proposals = '[]'::jsonb);
