-- WAVE 6 (cruzamento B): midia ESPELHADA da task vinculada no evento (read-only). O evento nao
-- tem ui_metadata (so a task tem), entao a midia emprestada da task (os videos de tasks.tasks
-- ui_metadata.videos) vive numa coluna propria. jsonb = MediaItem[] (mesmo shape de events.media),
-- servido apenas para exibicao no calendario; o sync task->evento a mantem. NUNCA editada pela UI
-- do calendario (nao passa por validateEvent/EventInput). Espelha o cruzamento A (calendarMedia na
-- task) no sentido inverso. SQL plano e idempotente (migrator roda o arquivo inteiro; sem goose).

alter table calendar.events
  add column if not exists linked_media jsonb not null default '[]'::jsonb;
