# AGENTS — modulo `calendar`

Agenda de conteudo por cliente da agencia. Painel Omni faz o CRUD. Plano canonico:
[docs/CALENDARIO_PLAN.md](/c:/Users/Mike/Documents/Projects/fila-atendimento/docs/CALENDARIO_PLAN.md).
Front em `web/app/components/calendar/` + `web/app/stores/calendar.ts` (ver
`web/app/components/AGENT.md` secao `calendar`).

## Padrao
Segue o molde do modulo `bio`: `model.go` (tipos + views), `store_postgres.go`
(pgxpool, schema `calendar.*`), `service.go` (regras + validacao + escopo),
`http.go` (rotas `/v1/calendar/*` + handlers), `http_media.go` (anexos/upload),
`media_storage.go` (disco), `holidays.go` (datas comemorativas), `module.go` (Registry).

## Schema (`calendar`) — migrations 0181/0182/0183
- `calendar.events` (0181): id, **account_id** (dono, FK core.accounts), **client_id**
  (cliente/tenant do evento, nullable FK core.accounts), event_date, event_time,
  type, title, status, priority, responsible_id, involved_ids (jsonb), media
  (jsonb = `MediaItem[]`), description, timestamps.
- `calendar.notes` (0181): (account_id, month_key `YYYY-MM`) PK, content (HTML), updated_by, updated_at.
- `calendar.config` (0182): account_id PK + `config jsonb` (shape em `CalendarConfig`:
  `responsibleUserIds[]` + `holidays{brNational,sergipe,aracaju,luxuryIntl}`).
- `calendar.day_media` (0183): (account_id, event_date) PK + `media jsonb` (`MediaItem[]`) —
  anexos AVULSOS do dia (sem vinculo com evento).

## Anexos / midia (Fase 3)
- `MediaItem` = `{id,url,name,type("image"|"video"),contentType,sizeBytes}`; `url` sempre
  `/uploads/calendar/{accountId}/{arquivo}` (validado no service — descarta url externa).
- Storage em disco (`media_storage.go`, `DiskMediaStorage` sob `cfg.UploadsDir`/calendar/{account}/),
  injetado via `calendar.New(storage)` no `app.go` (igual `tasks`). Servido em `/uploads/...`.
- Upload stateless: `POST /v1/calendar/media` valida mime (jpg/png/webp/gif, mp4/webm/mov) +
  tamanho e devolve o `MediaItem`; o front anexa ao evento/dia e salva (full replace).
- **Limite = config GLOBAL da plataforma** em `core.platform_settings` chave `media_limits`
  (`{imageMaxBytes(10MB), videoMaxBytes(300MB)}`), lida no upload. Sem tabela nova.

Responsaveis = usuarios REAIS (`core.account_users`+`core.users`, `display_name`). Fase 2
lista membros da account; `responsibleUserIds` vazio = todos os membros. (Puxar responsaveis
de contas-cliente cross-account = fast-follow com validacao de org.)

## Escopo (multi-tenant)
- O calendario e SEMPRE escopado pela **account do contexto** (`X-Account-Id`, ou
  `TenantID` do JWT na ausencia). Nao ha visao cross-account (nem admin) — admin
  opera na account do switcher. `accountScope(r)` resolve; sem account => 403 `no_account`.
- Defesa em profundidade: o store filtra por `account_id` em todo GET/UPDATE/DELETE
  (recurso de outra account => `pgx.ErrNoRows` => 404, nunca 403).
- Gating por modulo em `app.go` (`{Prefix: "/v1/calendar", ModuleID: "calendar"}`);
  platform_admin tem bypass.

## Endpoints
- `GET /v1/calendar/events?from=&to=&clientId=` — eventos da janela (datas inclusive).
- `POST /v1/calendar/events` — cria (body = EventInput).
- `GET /v1/calendar/events/{id}` — detalhe.
- `PUT /v1/calendar/events/{id}` — substitui (full replace).
- `DELETE /v1/calendar/events/{id}`.
- `GET /v1/calendar/notes/{month}` — nota do mes (`YYYY-MM`; vazia se nao existe).
- `PUT /v1/calendar/notes/{month}` — upsert da nota (body `{content}`).
- `GET/PUT /v1/calendar/config` — config da account (responsaveis + feriados).
- `GET /v1/calendar/members` — usuarios da account (candidatos a responsavel).
- `GET /v1/calendar/responsibles` — responsaveis efetivos (subconjunto do config ou todos).
- `GET /v1/calendar/holidays?from=&to=` — feriados/datas comemorativas da janela
  (read-only, so os `set` ligados no config). Resposta `{holidays:[{date,name,set}]}`
  ordenada por date; `set` in `brNational|sergipe|aracaju|luxuryIntl`. Datas fixas
  + moveis (Pascoa/Meeus, Carnaval, Corpus Christi, Dia das Maes/Pais, Black Friday,
  Cyber Monday) calculadas em `holidays.go` — sem tabela/migration.
- `POST /v1/calendar/media` — upload multipart (campo `file`) → grava e devolve `MediaItem`.
  Corpo limitado a `videoMaxBytes`+folga; > limite => 413 `media_too_large`; tipo invalido
  => 400 `invalid_media`.
- `GET /v1/calendar/day-media?from=&to=` — anexos avulsos por dia da janela (`{days:[{date,media}]}`).
- `PUT /v1/calendar/day-media/{date}` — full replace da lista do dia (body `{media:[MediaItem]}`).
- `GET /v1/calendar/media-limits` — tetos de upload (qualquer autenticado; o front mostra/valida).
- `PUT /v1/calendar/media-limits` — altera os tetos (**so platform_admin**; body = `MediaLimits`).

As chaves JSON de `EventView` batem 1:1 com o tipo `CalendarEvent` do front
(`web/app/utils/calendar.ts`). `client_id` nao-UUID (ex.: clientes de demonstracao
do mock) e descartado no service (vira sem-cliente) para nao estourar o cast `::uuid`.

## Fases seguintes (nao implementadas)
Campos de agencia (canal, aprovacao do cliente, deadline x publicacao), aprovacao
via WhatsApp (n8n/WAHA), config/white-label, perfil do cliente + IA de sugestao de
conteudo. Ver o plano canonico. (Feriados/datas comemorativas: implementado — ver
`holidays.go` e `GET /v1/calendar/holidays`.)
