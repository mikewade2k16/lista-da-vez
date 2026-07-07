-- WAVE 5 (E1): procedencia do evento do calendario, para a guarda anti-ping-pong do
-- espelho bidirecional calendario<->tasks. 'manual' = criado na tela; 'task' = evento-
-- espelho nascido de uma task; 'ai' = criado via proposta confirmada do chat (E7). O
-- espelho task->evento so cria/apaga eventos com source='task' (nunca toca nos manuais).
-- SQL plano e idempotente (migrator roda o arquivo inteiro; sem marcadores goose).

alter table calendar.events
  add column if not exists source text not null default 'manual';

-- Indice parcial: o handler de sync busca o evento-espelho de uma task por source='task'.
-- Mantem a busca barata sem pesar a tabela inteira.
create index if not exists calendar_events_source_task_idx
  on calendar.events (account_id)
  where source = 'task';
