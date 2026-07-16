-- WAVE 13: TODA midia do calendario pertence a um ITEM (evento). O conceito de "anexo
-- do dia" (calendar.day_media) e ELIMINADO: a midia passa a viver so em calendar.events.media.
-- Decisao do dono (2026-07-13): anexos ja vinculados a um evento migram para a midia dele;
-- anexos ORFAOS (sem eventId) viram um item especial source='media' (titulo = nome do arquivo,
-- paridade com o fluxo do drawer). SQL plano e idempotente (o migrator roda o arquivo INTEIRO;
-- sem `-- +goose Down`, que se auto-destruiria). Se rodar 2x: a tabela ja nao existe (guarda
-- to_regclass), entao os DO-blocks viram no-op.

do $$
begin
  -- So faz a consolidacao se a tabela ainda existir (idempotente: 2a execucao pula tudo).
  if to_regclass('calendar.day_media') is null then
    return;
  end if;

  -- (1) ANEXOS VINCULADOS: cada MediaItem com eventId nao-vazio e apontando para um evento
  -- REAL da MESMA conta e concatenado em events.media, SEM duplicar (dedup por id do item).
  -- jsonb '-' remove chaves internas (eventId/clientId nao pertencem a media do evento).
  update calendar.events e
  set media = e.media || sub.novos,
      updated_at = now()
  from (
    select ev.id as event_id,
           jsonb_agg((m - 'eventId' - 'clientId')) as novos
    from calendar.day_media dm
    cross join lateral jsonb_array_elements(dm.media) as m
    join calendar.events ev
      on ev.id = nullif(m->>'eventId','')::uuid
     and ev.account_id = dm.account_id
    where coalesce(m->>'eventId','') <> ''
      -- ainda nao esta na media do evento (dedup por id do MediaItem)
      and not exists (
        select 1 from jsonb_array_elements(ev.media) as em
        where coalesce(em->>'id','') = coalesce(m->>'id','')
          and coalesce(m->>'id','') <> ''
      )
    group by ev.id
  ) as sub
  where e.id = sub.event_id;

  -- (2) ANEXOS ORFAOS (sem eventId, ou eventId apontando p/ evento inexistente): viram um
  -- item especial source='media' no proprio dia (titulo = nome do arquivo; midia = [o item]).
  -- Espelha o fluxo do DayDrawer (WAVE 11) que ja transformava anexo avulso em evento.
  insert into calendar.events
    (account_id, client_id, event_date, event_time, type, title, status, priority,
     responsible_id, involved_ids, media, description, source)
  select dm.account_id,
         nullif(m->>'clientId','')::uuid,
         dm.event_date,
         '',
         'post',
         coalesce(nullif(trim(m->>'name'),''), 'Midia'),
         'planejado',
         'media',
         '',
         '[]'::jsonb,
         jsonb_build_array(m - 'eventId'),
         '',
         'media'
  from calendar.day_media dm
  cross join lateral jsonb_array_elements(dm.media) as m
  where coalesce(m->>'url','') <> ''
    and (
      coalesce(m->>'eventId','') = ''
      or not exists (
        select 1 from calendar.events ev
        where ev.id = nullif(m->>'eventId','')::uuid
          and ev.account_id = dm.account_id
      )
    );

  -- (3) A tabela cumpriu seu papel: some (a migration 0183 deixa de ter uso).
  drop table calendar.day_media;
end $$;
