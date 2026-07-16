# Calendário — SPECS 8 (WAVE 13: toda mídia pertence a um ITEM; "Anexos do dia" eliminado)

Continuação de `CALENDARIO_SPECS7.md`. Plano canônico: `CALENDARIO_PLAN.md`. Regra dos 3
docs: este doc + `back/internal/modules/calendar/AGENT.md` + roadmap `cal-w13-midia-por-item`.

## Decisão do dono (2026-07-13)

"Não vai ter anexos do dia — os anexos vão estar SEMPRE linkados a um item, e não mais ao
dia. E no item ter o drag-and-drop para mover os itens de lugar, e também para arrastarmos
itens do PC para ele; clicando também faz o upload, e remove."

Ou seja: o conceito de **"Anexos do dia"** (`calendar.day_media`, uma lista de mídia solta
por `(conta, dia)`) é **eliminado**. Toda mídia pertence a um **evento/item** (já existia
`calendar.events.media`). A seção "Mídia do post" do item, que era read-only, vira
**editável**: upload por clique, arrastar arquivo do PC, remover, e reordenar (drag-and-drop).

## O que foi feito

### Migração de dados (migration 0199, `0199_calendar_drop_day_media.sql`)

DO-block idempotente (só roda se `calendar.day_media` ainda existe):
1. **Anexos vinculados** (MediaItem com `eventId` apontando para um evento real da mesma
   conta) → concatenados em `events.media` do evento, sem duplicar (dedup por id do item;
   remove as chaves internas `eventId`/`clientId`).
2. **Anexos órfãos** (sem `eventId`, ou `eventId` inválido) → viram um item especial
   `source='media'` no próprio dia (título = nome do arquivo, `media=[o item]`) — paridade
   com o fluxo que o DayDrawer já fazia na WAVE 11. **Nada se perde.**
3. `drop table calendar.day_media`.

Validado com **dry-run** (transação `rollback`) nos dados reais antes de aplicar: 3 anexos
vinculados entraram em 2 eventos (ex.: "Gravação Pérolas" 0→2 mídias), 4 órfãos viraram
itens. Zero perda, zero duplicação.

### Backend (Go)

- `service.go`: removidos `ListDayMedia`, `PutDayMedia`, `pushDayMediaToTasks` (+ da
  interface do store).
- `store_postgres.go`: removidos `Store.ListDayMedia`/`PutDayMedia`.
- `http_media.go`: removidas as rotas `GET /day-media` e `PUT /day-media/{date}` e seus
  handlers. `POST /media` (upload) e `media-limits` continuam.
- `model.go`: removidos `DayMediaView`/`DayMediaInput`.
- `task_sync.go`: `eventMediaForTask` passa a usar só `ev.Media` (não lê mais day_media);
  `syncEventMediaToTask` (morto) removido.
- `runtime_context.go`: `enrichContextEvents` → `setContextClientNames` (a mídia da IA já
  vem da query de eventos via `scanAIContextEvent`, que une `events.media` + `linked_media`).
- `publisher.go` / `realtime/model.go`: removida a constante `day_media_updated`.
- `go build`/`vet`/`test` verdes.

### Frontend

- **`CalendarMediaUploader.vue`**: ganha **drop de arquivos externos** (arrastar do PC). A
  raiz `.calendar-media` capta `@dragover`/`@dragleave`/`@drop`, distinguindo o drop de
  arquivo (`dataTransfer.types` inclui `Files`) do drag de reordenação interno
  (`text/plain`). Overlay "Solte para enviar" (`.calendar-media__dropzone`); só ativo quando
  editável (`!readonly`). Upload/remover/reorder já existiam. CSS em `assets/styles/calendar/media.css`.
- **`DayDrawer.vue`**: seção "Anexos do dia" **removida** (+ `onDayMedia`, `dayItemOptions`).
  "Mídia do post" agora **editável** (`@update:model-value="onEventMedia"` → grava `ev.media`
  via `updateField`, mesmo caminho dos campos inline; optimistic locking C12). Os vídeos
  espelhados da task (`linkedMedia`, read-only) aparecem numa seção separada "Mídia da task".
- **Fundo do dia**: `dayBackgroundUrls(events)` (sem o parâmetro `dayMedia`) — 100% dos
  eventos. `MonthGrid.vue`/`WeekView.vue` perdem a prop `dayMediaByDate`; `index.vue` para de
  passá-la.
- **Removidos** (órfãos): `useCalendarDayMedia.ts` (composable deletado),
  `fetchDayMediaInRange`/`putDayMedia`/`CalendarDayMedia` (calendar-api), a fiação
  `dayMediaByDate`/`selectedDayMedia`/`saveDayMedia`/`fetchDayMedia` (store), e o handler
  `day_media_updated` do realtime.
- eslint 0 erros (warnings de max-lines pré-existentes); vue-tsc sem erro NOVO (os erros
  restantes são pré-existentes do projeto: `~/types/*` runtime + `unknown`).

## Notas de Deploy

- **Migration 0199** (roda no rebuild da api / `migrate`) — consolida e dropa `day_media`.
  IRREVERSÍVEL (a tabela é dropada); os dados foram preservados nos eventos. SEM env nova.
- Rebuild api (`docker compose up -d --build api`) + web (`up -d --build web`).
- n8n: reimportar `calendar-chat` (o prompt não mudou nesta wave, mas o import é idempotente).
