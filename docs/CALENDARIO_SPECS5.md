# Calendário — SPECS WAVE 5 (Integração completa Calendário ↔ Tasks + IA agente)

> Specs atômicas para subagentes. Fonte da visão: decisões do dono (2026-07-05). Base já no ar:
> integração C10 **um-sentido** (evento → task, só na criação) — `task_link.go` + `relations.go`,
> fiada no `app.go`. Esta wave torna a integração **bidirecional e viva** (espelho, sync de edição
> e de status, deep-link) e dá **ação** ao chat (criar evento/task no modo propor-e-confirmar).
> Regras gerais idênticas às waves anteriores (skill principios-engenharia; NUNCA git/npm/docker nos
> agentes; máx 450 linhas/arquivo; pt-BR sem acento em comentário; multi-tenant `account_id` do
> `accountScope`, nunca do body; migrations idempotentes sem `-- +goose`; atualizar AGENT.md da área).
> **Status: IMPLEMENTADO (2026-07-05) — backend + front + n8n no ar; build/vet/golangci/eslint/
> typecheck limpos; migration 0192 aplicada. Aguardando teste do dono.**

## Decisões do dono (2026-07-05)

1. **Toggle "Criar task" LIGADO por padrão** na criação de evento (quando o board está configurado);
   o usuário ainda pode desligar. Hoje só vem sugerido pra alguns tipos.
2. **Task nasce no TOPO da coluna** (é criação nova).
3. **Sync de edição BIDIRECIONAL**: editar num lado reflete no outro (título, data/prazo, responsável,
   cliente). Hoje editar não sincroniza nada.
4. **Deep-link pro CARD específico** (evento → abre a task exata no board; não mais `/tasks` genérico).
5. **Espelho BIDIRECIONAL** (escolha do dono, não o overlay): task com prazo vira evento-espelho e
   vice-versa, com dado sincronizado nos dois lados. **Exige** guarda anti-loop/duplicação rígida.
6. **Sync de status com MAPEAMENTO COMPLETO** (escolha do dono): a aba Tasks do config mapeia cada
   status de evento ↔ coluna do board, nos dois sentidos.
7. **Chat IA cria evento/task no modo PROPOR-E-CONFIRMAR** (escolha do dono): a IA monta um cartão com
   os campos; o usuário confirma (ou ajusta) e só então cria. **A criação usa a API autenticada do
   próprio usuário** (permissão/escopo normais) — a IA/n8n NUNCA escreve com service-token.

## Fatos do recon (fundam as decisões)

- **C10 hoje** (`back/internal/modules/calendar/`): `service.go:169` `CreateEvent` chama `createLinkedTask`
  só quando `in.CreateTask` e o board está configurado (senão `ErrTasksNotConfigured` ANTES de gravar);
  `DeleteEvent:240` faz `unlinkTask`; `UpdateEvent:207` **não toca em tasks**. `task_link.go` cria a task
  (título, due date data+hora SP default 09:00, responsável, cliente, `UIMetadata.source=calendar`) e
  vincula via `tasks.AddRelation` (module=`calendar`, resourceType=`event`). `relations.go` resolve o
  reverso (label `"<date> - <title>"`, url `/calendario?date=`). Provider LAZY de tasks + resolver
  fiados no `app.go:352/386`.
- **Tasks** (`back/internal/modules/tasks/`): `CreateTaskInput.SortOrder float64` (model.go:270) → "topo"
  = menor SortOrder da coluna. `UpdateTaskInput` usa **double-pointer** (`**string`/`**time.Time`,
  model.go:278+) = present/null/absent, dá pra sincronizar campo a campo. `UpdateTask` (service.go:~420)
  detecta troca de `Status`/`ColumnID` e publica `task.status_changed`. `RemoveRelation`/`AddRelation`
  exigem `PermRelationsManage`. **Dependência é calendar → tasks** (tasks NÃO importa calendar).
- **Front tasks**: o card abre por `openTaskEditor(task)` → `TasksTaskModal.vue`; **não há deep-link por
  URL** (`?task=` inexistente). Board em `layers/tasks/components/TasksBoardView.vue`.
- **Status de evento**: `CalendarEventStatus` (`web/app/utils/calendar.ts:13`) = `planejado`, `producao`,
  … , `publicado` (STATUS_META:198). O mapeamento (E5) é sobre ESSE conjunto.
- **Chat wave 4**: `chat.go` `ChatAsk` manda pergunta+histórico+contexto ao webhook e devolve `{answer}`;
  é **Q&A puro** (não cria nada). Callback de serviço (`CALENDAR_AI_SERVICE_TOKEN`/`CALLBACK_BASE`) já
  existe das waves 3/4, mas **não vamos** usá-lo para escrever (decisão 7).
- Próxima migration livre = **0192** (0191 = chat wave 4).

---

## Contratos compartilhados

### E1 — Schema + config (migration 0192, SQL plano idempotente)
```sql
-- Marca a PROCEDENCIA do evento (anti-ping-pong do espelho E3). 'manual' = criado na tela;
-- 'task' = espelho nascido de uma task; 'ai' = criado via proposta do chat (E7).
alter table calendar.events add column if not exists source text not null default 'manual';
```
- **Sem migration no schema de tasks**: o vínculo evento↔task já vive em `tasks.task_relations`
  (module=`calendar`). O "event id" da task é lido da relation; o "task id" do evento idem (join atual).
- **Config JSON** (sem migration — `calendar.config` já é jsonb; só estende o shape validado no Go/TS):
  - `tasks.mirrorTasks: bool` — liga o espelho task→evento (**default `true`** — decisão do dono
    2026-07-05: espelho ligado por padrão; pode desligar na aba Tasks).
  - `tasks.statusColumnMap: [{ eventStatus: string, columnId: string }]` — mapa status↔coluna (E5).
  - `tasks.defaultEventType: string` — tipo do evento-espelho nascido de task (default um tipo neutro).

### E2 — Hook de sync INVERTIDO (platform/modules) — o coração da bidirecionalidade
Como tasks não importa calendar, task→evento passa por um registry de inversão (espelha o
`RelationRegistry`/`RelationResolver` já existente):
```go
// platform/modules: quem OWNa um recurso espelhável por task registra um handler. Quando uma
// task COM relation para esse module muda, o tasks chama SyncFromTask para o dono atualizar o espelho.
type TaskSyncChange struct {
    Title      *string     // nil = nao mudou
    DueDate    **time.Time // present/null/absent (espelha UpdateTaskInput)
    ColumnID   **string
    ClientID   **string
    Responsible **string
    Deleted    bool
}
type RelationSyncHandler interface {
    ModuleID() string
    SyncFromTask(ctx context.Context, accountID, resourceID string, ch TaskSyncChange) error
}
```
- **Registro**: `app.go` registra `calendar.NewTaskSyncHandler(calendarService)` num `RelationSyncRegistry`
  injetado no `tasks.NewService` (novo parâmetro opcional, nil = sync desligado — testes seguem).
- **Disparo**: `tasks.UpdateTask`/`DeleteTask`, APÓS commit, lê as relations da task; para cada relation
  cujo module tem handler, chama `SyncFromTask`. **Best-effort** (falha vira log, nunca desfaz a task).
- **Guarda anti-eco (regra dura)**: o handler do calendar atualiza o evento por um método **TERMINAL
  INTERNO** (`applyTaskSync`) que **NÃO** re-dispara o forward-sync evento→task. Simétrico no outro
  sentido: `UpdateEvent` chama `syncTaskFromEvent` (forward) por um método de tasks que **NÃO** re-chama
  o `RelationSyncHandler`. Nunca há 2º salto. (Sem flags de contexto frágeis: métodos terminais explícitos.)

### E3 — Espelho bidirecional + anti-loop (via `source` e `UIMetadata.source`)
- **Evento → task** (já existe, agora default-on): task nasce com `UIMetadata.source=calendar`. Essa task
  é source=calendar ⇒ o disparo E2 **não** cria outro evento (ela já tem o dela).
- **Task → evento** (novo, só se `config.tasks.mirrorTasks=true`): ao **criar** uma task com `dueDate`
  num board mapeado, o handler cria o evento-espelho (`source=task`, `clientId=task.clientAccountId`,
  `type=defaultEventType`, data = `dueDate` em America/Sao_Paulo) e vincula. Evento `source=task` ⇒ o
  `CreateEvent` reverso **não** cria outra task (`createTask=false` forçado). Task sem `dueDate` ⇒ sem
  espelho; remover o `dueDate` ⇒ remove o espelho (unlink + delete do evento-espelho `source=task`).
- **Delete** (sem cascata destrutiva): apagar um lado só **desvincula** — o outro permanece (hoje já é
  assim no delete do evento). Exceção: o **evento-espelho `source=task`** É apagado junto quando a task
  some (ele não tem vida própria). Nunca apagar uma task feita pelo usuário.

### E4 — Sync de EDIÇÃO (campos e direção)
Campos espelhados (last-write-wins, sem merge): **título**, **data/prazo** (`event.date`+`event.time`
SP ↔ `task.dueDate` timestamptz), **responsável** (`event.responsibleId` ↔ `task.responsibleUserId`),
**cliente** (`event.clientId` ↔ `task.clientAccountId`).
- `UpdateEvent` (forward) → `syncTaskFromEvent` (terminal em tasks).
- `UpdateTask` (via E2) → `calendar.applyTaskSync` (terminal em calendar).

### E5 — Sync de STATUS (mapeamento completo)
- Fonte: `config.tasks.statusColumnMap`. **Evento muda status** → mover a task pra `columnId` mapeada
  (no topo da coluna destino). **Task muda de coluna** → setar `event.status` pro `eventStatus` mapeado.
  Status/coluna sem mapa ⇒ no-op (não força nada). Ambos os lados passam pelos métodos terminais (E2/E4).

### E6 — Deep-link pro card (front tasks)
- `TasksBoardView.vue`: ao montar, ler `route.query.task` (e `board`); se presente, carregar e abrir
  `openTaskEditor` daquele card (fallback silencioso se não achar/sem permissão). Limpar o query após abrir.
- Link do lado calendário: `DayDrawer.vue`/badge do evento com task usa `/tasks?board=<boardId>&task=<taskId>`
  (boardId = `config.tasks.boardId`; taskId = `event.taskId`). O resolver reverso (`relations.go`, task→calendário)
  segue como está.

### E7 — Chat IA cria evento/task (propor-e-confirmar; SEM service-token escrevendo)
- **n8n** (workflow Calendar Chat): o system prompt instrui a IA — quando o usuário pedir para CRIAR
  evento/task, **não executar**; devolver, junto do texto, um bloco `proposal` JSON:
  `{"proposal":{"kind":"event"|"task","fields":{title,date?,time?,type?,status?,dueDate?,columnId?,clientId?}}}`.
  O nó "Extrair resposta" repassa `{answer, proposal?}` (proposal ausente na conversa normal).
- **Back** (`chat.go`): `ChatAskResult` ganha `Proposal *ChatProposal` (passthrough validado do webhook;
  shape fechado; clientId só aceito se ∈ escopo visível resolvido no ask — reusa `ChatAccess`).
- **Front** (`CalendarChatPanel.vue`): quando vem `proposal`, renderiza um **cartão de confirmação**
  (campos editáveis + "Criar" / "Descartar"). "Criar" chama a **API REST autenticada do próprio usuário**:
  evento → `POST /v1/calendar/events` (com `createTask` default-on, E-item-1); task → o create de tasks.
  Sucesso → confirma no chat + deep-link (E6). **Nada é criado sem o clique.** Permissão/escopo = os do
  usuário (o endpoint já valida). Sem endpoint novo de "IA escreve".

---

## Raias (subagentes) — pipeline sugerido

- **BACK-CAL** (`modules/calendar`): migration 0192 (`events.source`); `applyTaskSync` (terminal) +
  `syncTaskFromEvent` no `UpdateEvent`; criação/edição/remoção do evento-espelho `source=task`;
  aplicar `statusColumnMap` nos dois sentidos; `ChatAskResult.Proposal` passthrough validado;
  estender o shape de `CalendarTasksConfig` (mirror/statusMap/defaultEventType) + validação. AGENT.md.
- **BACK-TASKS** (`modules/tasks`): `RelationSyncRegistry` + `RelationSyncHandler` (platform/modules) +
  chamada em `UpdateTask`/`DeleteTask` (best-effort, pós-commit); `SortOrder` no topo ao criar via
  calendar; método terminal `applyCalendarSync` (sem re-disparar handler). AGENT.md.
- **WIRING** (`platform/app/app.go`): registrar `RelationSyncRegistry` no `tasks.NewService` +
  `calendar.NewTaskSyncHandler`. (1 arquivo, cuidado com ordem LAZY.)
- **N8N** (`automation/export/workflow-calendar-chat.json`): instrução de `proposal` no system prompt +
  repasse `{answer, proposal?}` no "Extrair resposta". Re-importar + reativar.
- **FRONT** (`web/app/...` + `web/layers/tasks`): toggle `createTask` default-on (CalendarEventForm);
  cartão de proposta + criar-on-confirm (CalendarChatPanel); deep-link `?task=` (TasksBoardView) +
  link no DayDrawer/badge; aba Tasks do config = toggle "espelhar tasks" + tabela status↔coluna;
  evento-espelho (`source=task`) com estilo distinto no mês.

## Notas de Deploy (ordem exata)
1. **Migration 0192** (`events.source`) — roda no migrate. Idempotente.
2. **Rebuild api** (`docker compose up -d --build api`) — muda Go (calendar+tasks+app.go).
3. **Rebuild web** (`docker compose up -d --build web`) — muda front.
4. **Re-importar o workflow n8n** Calendar Chat + reativar (proposta E7).
5. **Config por conta**: `tasks.mirrorTasks` nasce `true` (espelho ligado por padrão — decisão do dono);
   `statusColumnMap` vazio (sem sync de status até o dono mapear na aba Tasks). Sem novos envs. **Atenção
   deploy**: com espelho on por padrão, contas com board configurado passam a gerar evento-espelho de
   tasks com prazo — comportamento esperado, mas registrar no changelog do deploy.
6. AGENT.md de `calendar` e `tasks` atualizados; roadmap-data.ts (fase calendário) com os itens `cal-w5-*`.
7. **Cruzamento de mídia A (W6, 2026-07-06)**: SEM migration (usa `ui_metadata.calendarMedia`, jsonb já
   existente). Só **rebuild api** (calendar+tasks) + **rebuild web** (tasks front). Nada de env novo.
8. **Cruzamento de mídia B (W6, 2026-07-06)**: **Migration 0193** (`calendar.events.linked_media` jsonb,
   idempotente, roda no migrate do boot da api) → **rebuild api** (calendar+tasks+platform/modules) →
   **rebuild web** (calendar+tasks front). Sem env novo.
9. **Tipos/status compartilhados + collapse + fix reload (W6, 2026-07-06)**: só FRONT → **rebuild web**.
   Nada de migration/env. Board EXISTENTE não muda de colunas (só board novo nasce com os 6 status).

## Riscos / atenção
- **Loop de sync**: mitigado por métodos TERMINAIS explícitos (E2) + `source` (E3). Todo caminho de sync
  é 1 salto. Cobrir com teste: editar evento não pode gerar 2ª escrita na task e vice-versa.
- **Duplicação de espelho**: `source` + relation existente barram recriação. Criar task source=calendar
  não cria evento; criar evento source=task não cria task.
- **Segurança da IA (E7)**: a criação passa SEMPRE pela API autenticada do usuário; a proposta só sugere.
  clientId da proposta validado contra o escopo visível. Zero escrita por service-token.
- **Permissão de tasks**: sync exige o usuário ter acesso a tasks na conta (`ResolveAccessContext`);
  sem acesso, o sync é best-effort e só loga (nunca derruba o evento).

## Progress Log
- 2026-07-06 (UX + integração, 4 frentes) — **(1) Coluna de anotações minimizável:** botão nos
  controles (`i-lucide-panel-left-close`) colapsa a coluna esquerda num sidebar SLIM de 2.75rem — nome
  do mês vertical (`writing-mode: vertical-rl`, clicável para reabrir) + setas ↑↓ de mês + botão
  expandir. Estado em localStorage. **(2) Badge "evento sem task" + criar:** o DayDrawer mostra badge
  âmbar "Sem task" + botão "Criar task" quando `event.taskId` vazio; endpoint novo `POST
  /v1/calendar/events/{id}/task` (`CreateTaskForEvent` → reusa `createLinkedTask` C10, idempotente).
  **(3) Exclusão "perguntar na hora"** (decisão do dono): excluir evento → confirma → se tem task,
  2º modal "arquivar a task também?"; `DELETE .../events/{id}?archiveTask=true` arquiva a task junto
  (`archiveLinkedTask`, tolera 404 quando o archive já apagou o espelho). **(4) IA lê as anotações do
  mês:** JÁ funcionava — o back busca `GetNotes(month)` → `context.monthNotes` e o n8n injeta no system
  prompt ("Notas do mes... use como contexto"); `activeNotesMonthKey === focusMonthKey` (o chat manda o
  mesmo mês exibido no editor). Sem código novo.
- 2026-07-06 (bug de fuso no mirror) — **Task com data espelhava no DIA ANTERIOR.** Diagnóstico no
  banco: o mirror task→evento FUNCIONAVA, mas uma task com data-only (ex.: IA cria "14/07" sem hora)
  nasce `2026-07-14 00:00 UTC`; o mirror fazia `dueDate.In(saoPauloLoc)` → `13/07 21:00` → evento no
  dia 13. O dono via a task no 14 mas o evento no 13 e achava que "não criou". Fix: helper
  `mirrorEventDateTime` — meia-noite UTC = "sem hora" → usa a DATA em UTC + hora vazia (dia inteiro);
  task com hora real converte para São Paulo. Aplicado em `maybeCreateMirrorEvent` e
  `applyTaskSyncToEvent`. O sentido evento→task (`eventDueDate`) já era seguro (interpreta em SP, hora
  default 09:00). **Confirmação de comportamento**: criar TASK com data já espelha no calendário
  sozinho (mirror ligado) — não precisa a IA criar evento à parte.
- 2026-07-06 (W6 fatias 5B/6/7 + bug) — **Finalização do módulo.** (1) **Cruzamento B** (vídeo da task →
  calendário): coluna `calendar.events.linked_media` (jsonb, migration 0193) + `EventView.linkedMedia`;
  `TaskSyncSnapshot.Media` (MediaSnapshot neutro) leva os vídeos (`ui_metadata.videos`);
  `applyTaskSyncToEvent`/`maybeCreateMirrorEvent` gravam via `SetEventLinkedMedia` (terminal). O front une
  na "Mídia do post". (2) **Tipos + status COMPARTILHADOS**: `~/utils/content-taxonomy.ts` vira fonte
  única — `calendar.ts` deriva os `*_META`; o tasks usa a mesma lista (seletor de tipo lidera com os 6
  tipos + legados; colunas padrão de board novo = os 6 status). Board existente mantém colunas; o
  `statusColumnMap` faz a ponte. (3) **Collapse dos itens do dia**: DayDrawer virou accordion — cada
  evento é um header clicável cujo corpo (detalhe + edição inline + mídia) abre só no item ativo
  (`toggleItem`, um por vez). (4) **Bug reload/responsável CORRIGIDO**: `store.init()` é guardado (roda
  1×), então voltar ao /calendário não refazia o fetch e o evento parecia velho/sumido. Fix =
  `refetchWindow` ao (re)entrar na página (onMounted quando já inicializado + onActivated p/ keepalive);
  o back já sincronizava (applyTaskSyncToEvent), faltava o front refetchar.
- 2026-07-06 (W6 fatias 4b/5) — **Cruzamento de mídia calendário↔task (espelho read-only)**. Decisão
  do dono: espelho read-only (não cópia de arquivo). **Refinos (4b)**: "Mídia do post" do evento mostra a
  UNIÃO (mídia do evento + anexos do dia apontados via `MediaItem.eventId`) e cada item da lista do dia
  ganhou badge 📎 com contagem. **Cruzamento A (evento→task)**: a mídia do evento é espelhada na task
  vinculada em `ui_metadata.calendarMedia` (só exibição; a task só guarda vídeo, mas aqui imagem+vídeo
  cruzam — é display; MESMA url `/uploads/calendar/{conta}/`, sem duplicar arquivo). Coleta =
  `eventMediaForTask`; push no `syncTaskFromEvent` (update do evento) + `syncEventMediaToTask` (leve, só
  mídia) no gatilho `PutDayMedia`→`pushDayMediaToTasks` (reespelha eventos que ganharam OU perderam anexo).
  Whitelist no tasks (`normalizeCalendarMediaMetadata`, só `/uploads/calendar/`) barra body forjado. Front:
  `TaskCalendarMediaItem` + seção "Mídia do calendário" read-only no modal. SEM migration (ui_metadata jsonb).
  **Cruzamento B (task→evento) NÃO feito**: o evento não tem `ui_metadata`; exige coluna nova
  (`events.linked_media`) ou fetch no front — decisão pendente. Anti-loop mantido (tudo TERMINAL).
- 2026-07-05 (feedback do dono) — **(a) Estado "IA fora do ar" visual e distinto**: n8n marca
  `aiError:true` em toda falha do LLM (503/cota/chave/vazio) e o Respond passa adiante; o back
  propaga (`ChatAskResult.AIError`) e NAO persiste a msg de erro (memoria limpa); o front mostra um
  BADGE no cabecalho + um BLOCO vermelho no chat (nunca um balao normal) + botao Repetir, cobrindo
  tambem erro de rede/timeout. Nunca troca de IA sozinho. **(b) Realtime do sync os dois sentidos**:
  o back ja publica (`calendar.event_updated` no task->evento; `task.updated` no evento->task) — os
  dois canais refetcham quando a pagina esta aberta. Adicionado refetch-ao-voltar (visibilitychange +
  window.focus) no calendario (useCalendarLiveSync) e no board (useTasksPageContext) para cobrir o
  caso do WS nao ter entregado (pagina fora de foco): voltar mostra o estado fresco SEM recarregar.

- 2026-07-05 (fix E7) — **IA criando de verdade**. Bug raiz: o nó "Respond to Webhook" do workflow
  devolvia so `{answer}` e DESCARTAVA o `proposal` (a proposta era montada mas nunca chegava ao back).
  Corrigido: responseBody agora `{answer, proposal}`. Robustez: **modo JSON forcado**
  (`response_format: json_object`) + **schema PLANO** (`{answer, create, title, date, time, type,
  status, clientId/dueDate}`) — o LLM enche os campos de forma confiavel (objeto aninhado ele deixava
  null). Regra do envelope movida pro FIM do system (recencia). Retry 4x/2.5s. Validado por webhook com
  a chave real: criar evento OK, criar task OK (entende "sexta 11 de julho" -> dueDate), conversa normal
  proposal=null. Diagnostico feito lendo a execucao no SQLite do n8n. Ressalva viva: Gemini free 503
  intermitente; recomendar z.ai (glm) com saldo para confiabilidade.

- 2026-07-05 — SPECS WAVE 5 criadas (7 decisões do dono). Recon de tasks/calendar/front feito. Migration
  alvo 0192.
- 2026-07-05 — **IMPLEMENTADO (tudo de uma vez, espelho ON por padrão)**. Backend: migration 0192
  (`events.source`); `TasksConfig` (mirrorTasks/defaultEventType/statusColumnMap) + sanitize; hook
  invertido `RelationSyncRegistry`/`RelationSyncHandler` (platform/modules) + dispatch em
  tasks Create/Update/Move/Archive + terminal `ApplyCalendarSync`; `taskSyncHandler` no calendar
  (`task_sync.go`: mirror create/delete, applyTaskSync, forward `syncTaskFromEvent`, status-map bidi,
  `topSortOrder`); `events.source` no store; `ChatProposal` passthrough no `chat.go`; wiring `app.go`.
  n8n: instrução de `proposal` no system prompt + parse `{answer, proposal?}`. Front: toggle default-on
  (CalendarEventForm), aba Tasks (mirror + tipo espelho + mapa status↔coluna), deep-link `?task=`
  (TasksBoardView/useTasksPageContext + DayDrawer), cartão de proposta (useCalendarChat +
  CalendarChatPanel), evento-espelho distinto (EventChip). build/vet/golangci/eslint/typecheck limpos
  (só warnings pré-existentes + `max-lines` do CalendarChatPanel=548 → faxina). api/web/n8n rebuildados,
  migration aplicada. **Aguardando teste do dono.** Ressalva: a proposta da IA depende do LLM emitir o
  JSON — degrada pra texto normal se não emitir; e a criação SEMPRE passa pela API autenticada do usuário.
