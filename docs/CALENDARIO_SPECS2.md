# Calendário — SPECS de implementação WAVE 2 (Fases 8 → 11)

> Specs operacionais para os subagentes. Fonte da visão: [CALENDARIO_PLAN.md](CALENDARIO_PLAN.md)
> (§3.9–3.12). Cada spec é ATÔMICA: começa e termina no mesmo agente, sem deixar meio-caminho.
> Progresso/erros/pendências registrados no **Progress Log** no fim deste arquivo.
> Criado 2026-07-04. Decisões do dono: config volta a ser DRAWER lateral (com abas, sem sair do
> calendário); chat de IA flutuante com voz (Whisper) e provider Gemini (free tier) para teste;
> integração calendário↔tasks; realtime com presença estilo Google Docs (que em TASKS já existe
> e serve de molde). Wave 1 (fases 3c–6) documentada em [CALENDARIO_SPECS.md](CALENDARIO_SPECS.md)
> — os contratos C1–C5 de lá CONTINUAM valendo e são referenciados aqui.

## Regras gerais (valem para TODOS os agentes)

1. **Leia antes de codar**: `.claude/skills/principios-engenharia/SKILL.md` + a(s) referência(s)
   da sua área (`references/frontend.md`, `references/backend.md`, `references/database.md`,
   `references/lint.md`). São inegociáveis.
2. **NUNCA** rode `git` (sessão multi-agente), `npm install/build/generate`, `docker` ou
   qualquer deploy. Só edite arquivos. O orquestrador roda build/lint depois.
3. Máx **450 linhas/arquivo**. Comentários em pt-BR **sem acentos** (padrão do repo). Sem emojis.
4. Multi-tenant: `account_id` SEMPRE do Principal (`accountScope(r)` no módulo calendar), nunca
   do body. Store filtra por `account_id` em todo GET/UPDATE/DELETE (defesa em profundidade).
   Recurso fora do escopo → **404** (nunca 403).
5. Go: sem pacote uuid externo (string + cast `::uuid`); scan nullable com `*string`; camadas
   `http → service → store`. Front: `<script setup lang="ts">`, sem `any`, sem `console.log`,
   classes BEM-like, tokens do design system (nunca hex hardcoded em CSS — cor escolhida pelo
   usuário é DADO e pode ir inline via style).
6. Migrations: SQL plano **idempotente**, schema-qualificado, **SEM** marcadores `-- +goose`
   (o migrator roda o arquivo INTEIRO no boot).
7. **Não remover funcionalidade existente.** Features coexistem. (Exceção DELIBERADA desta wave,
   decidida pelo dono: a página `/calendario/config` migra pro drawer e vira redirect — F6.)
8. Ao terminar, atualize o AGENT.md da SUA área (back: `back/internal/modules/calendar/AGENT.md`
   e `back/internal/modules/realtime/AGENT.md`; front: `web/app/components/AGENT.md` seção
   calendar; n8n: `automation/AGENT.md`). Docs canônicos (plano/roadmap/panorama) ficam com o
   orquestrador.
9. Config/dado faltante = aviso acionável inline (nunca default silencioso que minta).
10. **URLs de mídia/áudio no front SEMPRE via `resolveMediaUrl(apiBase)`** (`utils/media.ts`) —
    url relativa `/uploads/...` resolve contra o front :3003 no dev e quebra TUDO (falha real
    da wave 1, Progress Log de 2026-07-02).

---

## Contratos compartilhados (Go ↔ TS ↔ n8n — chaves JSON idênticas)

### C6 — CalendarConfig v3 (jsonb `calendar.config` — SEM migration nova)
Estende o C2 da wave 1 (tudo de lá é mantido):
```jsonc
{
  // ...responsibleUserIds, holidays, weekStartsOn, clientColors, typeColors, whiteLabel...
  "ai": {
    "provider": "claude",   // claude | deepseek | qwen | kimi | glm | gemini | custom  ← +gemini
    "model": "claude-sonnet-5", "baseUrl": "", "systemPrompt": "", "temperature": 0.7
  },
  "tasks": { "boardId": "", "defaultColumnId": "" }   // vazio = integracao com tasks DESLIGADA
}
```
- Base URL default do gemini (front placeholder + mapa do n8n):
  `https://generativelanguage.googleapis.com/v1beta/openai` (camada OpenAI-compatible do
  Google AI Studio; free tier — CONFERIR a URL na doc do Google ao implementar).
- Defaults preenchidos nos DOIS lados, como na wave 1: Go (`defaultConfig()` + unmarshal por
  cima) e TS (`normalizeConfig` com merge por seção). Linha antiga sem `tasks` ganha `{}`.
- Sanitização no service: `boardId`/`defaultColumnId` são UUID ou vazio (senão descarta);
  `gemini` entra no enum de provider.

### C7 — Chat do calendário (painel → Go → n8n `calendar-chat`)
```jsonc
// POST /v1/calendar/chat/ask   (RequireAuth + accountScope; espelho do /v1/omni-chat/ask)
// body:
{ "question": "", "conversationId": "", "clientId": "", "month": "YYYY-MM" }
// question obrigatoria (trim, max 4000); conversationId obrigatorio (id da conversa no front);
// clientId/month OPCIONAIS = contexto da tela (cliente filtrado + mes em foco).
// → 200 { "answer": "" } | 400 invalid_question | 502/503/504 (n8n indisponivel/erro/timeout,
//   sentinels iguais ao n8n_client.go do omni-chat) | 503 chat_not_configured (sem env).

// Payload Go → n8n (webhook calendar-chat):
{
  "question": "", "sessionKey": "<accountId>|<userId>|<conversationId>", "language": "pt-BR",
  "ai": { "provider": "", "model": "", "baseUrl": "", "systemPrompt": "", "temperature": 0.7 },
  "context": {
    "month": "YYYY-MM",
    "client": { "id": "", "name": "", "profile": { /* C3 sem clientId */ } },  // ou null
    "holidays": [{ "date": "", "name": "", "set": "" }],
    "monthNotes": "<html, pode ser vazio>",
    "events": [{ "date": "", "type": "", "title": "", "status": "", "clientId": "" }], // lean, max 100
    "plans": [{ "id": "", "month": "", "status": "", "provider": "", "model": "" }]    // lean, max 10
  }
}
// REGRA DE UNIFICACAO: context (C7) = payload do C9 SEM o campo "account" — MESMAS chaves,
// MESMOS shapes, montados pela MESMA funcao Go (BuildAIContext de B7). Nao divergir.
// Resposta do n8n: { "answer": "" }. Memoria fica no n8n (Redis Chat Memory por sessionKey);
// o Go NAO persiste mensagens (documentar no front: historico some ao recarregar — v1).
```

### C8 — Transcrição de voz (mic → Go → n8n `calendar-transcribe`)
```jsonc
// POST /v1/calendar/chat/transcribe   (RequireAuth + accountScope; multipart campo "file")
// mimes: audio/webm, audio/ogg, audio/mp4, audio/mpeg, audio/wav — max 15 MiB
// → 200 { "text": "" } | 400 invalid_media | 413 media_too_large |
//   503 transcribe_not_configured | 502/504 (n8n)
// Go → n8n: repassa o multipart (file + campo language=pt) pro webhook calendar-transcribe;
// n8n responde { "text": "" } (nó OpenAI transcribe whisper-1 — o MESMO do bot WhatsApp).
```

### C9 — Contexto compartilhado entre IAs (service-to-service)
```jsonc
// GET /v1/runtime/calendar/context?accountId=<uuid>&clientId=<uuid>&month=YYYY-MM
// Auth: Bearer AUTOMATION_RUNTIME_TOKEN (constant-time; mesmo modelo do /v1/runtime/automation/*).
// accountId OBRIGATORIO; clientId/month opcionais (sem month = mes corrente).
// → 200 {
//   "account": { "id": "" },
//   "client": { "id": "", "name": "", "profile": { /* C3 */ } },   // ou null
//   "month": "YYYY-MM", "holidays": [...], "monthNotes": "",
//   "events": [{ "date","type","title","status","clientId" }],     // lean, max 100
//   "plans": [{ "id", "month", "status", "provider", "model" }]    // lean, max 10
// }
// O bloco context do C7 = este agregado SEM "account" (mesma funcao Go monta os dois —
// mesmas chaves "plans"/"events"/etc., ver regra de unificacao no C7).
// ESCOPO: ligar os workflows EXISTENTES (WhatsApp, Omni Chat) a este endpoint fica FORA da
// wave 2 — a wave entrega a CAPACIDADE (endpoint + doc); o wiring dos outros bots e etapa
// futura do plano de automacao (registrar no doc do workflow como "proximo passo").
// Registrar FORA do gate de modulo (prefixo /v1/runtime — conferir moduleGatingRules em
// back/internal/platform/app/app.go) e SEM RequireAuth (token de servico e a autenticacao).
```

### C10 — Vínculo evento ↔ task
```jsonc
// EventView (C do modulo) ganha 2 campos:
{ /* ...campos atuais... */ "taskId": "", "version": 1 }
// taskId = task vinculada via tasks.task_relations (module='calendar', resource_type='event',
// resource_id=<eventId>); vazio = sem vinculo. version = C12.

// POST /v1/calendar/events aceita opcional:
{ /* ...EventInput atual... */ "createTask": true }
// Com createTask=true: cria a task no board/coluna da config C6 (tasks.boardId obrigatorio —
// sem config → 400 tasks_not_configured com mensagem acionavel) + AddRelation + badge.
// Task criada: title = title do evento; dueDate = date+time (RFC3339, time vazio = 09:00);
// clientAccountId = clientId do evento; responsibleUserId = responsibleId se UUID valido;
// uiMetadata.source = "calendar"; status/coluna = defaultColumnId (vazio = 1a coluna do board).
// DELETE do evento → RemoveRelation (a task NAO e arquivada). Sem sync de status na v1.
```

### C11 — Realtime do calendário (canal + presença)
```jsonc
// Topico: calendar:account:{accountId}. Rota: GET /v1/realtime/calendar?scope=account&accountId=
// (ticket efemero via POST /v1/ws/ticket, padrao unico do modulo realtime).
// Autorizacao antes do upgrade: conta ativa + membership + permissao calendar.view
// (espelho de authorizeTasksAccount trocando as permission keys; platform_admin bypass).
// Eventos publicados (INVALIDACAO, payload lean — front refaz fetch, nunca patch local):
//   calendar.event_created | calendar.event_updated | calendar.event_deleted   (resourceId=eventId, payload.date)
//   calendar.note_updated       (payload.monthKey)
//   calendar.day_media_updated  (payload.date)
//   calendar.config_updated
//   calendar.plan_updated       (resourceId=planId, payload.status)
// Presenca: topico presence:calendar:{accountId} via GET /v1/realtime/presence?scope=calendar.
// Mensagens client→server (readPresencePump ja existente): presence.heartbeat,
// presence.field_focus / field_draft / field_blur com fieldKey:
//   "notes:YYYY-MM" (notas do mes) | "event:<id>" (form de edicao de evento).
```

### C12 — Optimistic locking em eventos (migration 0188)
```jsonc
// Migration 0188_calendar_events_version.sql:
//   alter table calendar.events add column if not exists version integer not null default 1;
// PUT /v1/calendar/events/{id}:
//   - header If-Match: <version int> OPCIONAL (sem header = comportamento atual, compat).
//   - com If-Match divergente → 409 version_conflict (novo erro em writeServiceError).
//   - update com sucesso: SET version = version + 1 (WHERE id AND account_id AND version=$n
//     quando If-Match presente) e devolve o EventView novo (com version).
// Front: SEMPRE envia If-Match com a version que carregou; no 409 mostra aviso acionavel
// ("alterado por outra pessoa") + botao de recarregar o item (draft do usuario NAO e
// descartado silenciosamente).
```

Envs novos (documentar em TODOS os `.env*.example` + compose — junto com o backlog
`CALENDAR_AI_*` da wave 1 que ficou de fora): `CALENDAR_CHAT_WEBHOOK_URL`
(`http://n8n:5678/webhook/calendar-chat`), `CALENDAR_TRANSCRIBE_WEBHOOK_URL`
(`http://n8n:5678/webhook/calendar-transcribe`).

---

## LANE BACK (sequencial — um agente por spec)

### SPEC-B5 — CalendarConfig v3: seção `tasks` + provider `gemini` (pequena, sem migration)
- `back/internal/modules/calendar/model.go`: struct `TasksConfig { BoardID, DefaultColumnID string }`
  no `CalendarConfig` (json `tasks`); `defaultConfig()` preenche `TasksConfig{}`.
- `service.go`: enum de provider ganha `gemini` (hoje `claude|deepseek|qwen|kimi|glm|custom`,
  ~l.37-38); sanitização nova `sanitizeTasks` (boardId/defaultColumnId UUID ou vazio).
- Conferir que o unmarshal por cima dos defaults (`store_postgres.go` GetConfig) mantém shape
  estável para linhas antigas (maps/structs nil → zero value).
- Aceite: GET config de conta antiga devolve `"tasks":{"boardId":"","defaultColumnId":""}`;
  PUT com provider `gemini` persiste; boardId não-UUID é descartado.

### SPEC-B6 — Chat: `/chat/ask` + `/chat/transcribe` + envs (usa C7/C8)
- Arquivos novos no módulo calendar (manter <450): `chat.go` (tipos + service: monta payload C7
  chamando o agregado de contexto de B7, POST ao webhook com `http.Client` timeout 60s p/ ask
  e 120s p/ transcribe), `http_chat.go` (`RegisterChatRoutes(mux, svc, middleware)`, chamado no
  `module.go` → RegisterRoutes).
- Envs lidos no `module.go` Build via `os.Getenv` (padrão dos `CALENDAR_AI_*`):
  `CALENDAR_CHAT_WEBHOOK_URL`, `CALENDAR_TRANSCRIBE_WEBHOOK_URL`. Vazios → 503
  `chat_not_configured` / `transcribe_not_configured` (mensagem acionável citando o env).
- Erros de upstream espelham o `n8n_client.go` do omni-chat (502 indisponível / 504 timeout /
  502 resposta inválida) — NÃO copiar o arquivo, replicar só os sentinels no módulo calendar.
- `/chat/transcribe`: multipart limitado a 15 MiB (`http.MaxBytesReader`), whitelist de mime
  C8, repassa multipart ao n8n e devolve `{text}`. NADA é gravado em disco.
- `sessionKey = accountID|userID|conversationId` (espelho `omniChatSessionKey` em
  `back/internal/modules/automation/service_omnichat.go` ~l.76-83; userID do Principal).
- Adicionar TODOS os envs de calendário (`CALENDAR_AI_WEBHOOK_URL`, `CALENDAR_AI_SERVICE_TOKEN`,
  `CALENDAR_AI_CALLBACK_BASE`, `CALENDAR_CHAT_WEBHOOK_URL`, `CALENDAR_TRANSCRIBE_WEBHOOK_URL`)
  aos `.env*.example` da raiz + `back/.env.example` + serviço `api` do `docker-compose.yml`
  (com defaults comentados, valor real fica no `.env` local — NUNCA commitar segredo).
- Aceite: sem env → 503 acionável; com env fake → 502/504 coerente; question vazia → 400;
  áudio de 20MB → 413; envs presentes nos `.env*.example`.

### SPEC-B7 — Contexto compartilhado: `GET /v1/runtime/calendar/context` (usa C9)
- Arquivo novo `back/internal/modules/calendar/runtime_context.go`: service
  `BuildAIContext(ctx, accountID, clientID, month)` reusando as queries existentes de
  `store_ai_context.go` (nomes/perfis sem N+1, feriados via `HolidaysInRange` + config, nota
  do mês) + eventos lean do mês (máx 100, projeção `date,type,title,status,client_id`) +
  planos lean (máx 10). **A MESMA função alimenta o payload C7 do chat** (B6 depende daqui —
  se B6 rodar antes, deixa TODO e B7 conecta).
- `http_runtime.go`: rota `GET /v1/runtime/calendar/context` SEM RequireAuth, autenticada por
  Bearer `AUTOMATION_RUNTIME_TOKEN` em comparação constant-time (`crypto/subtle`, padrão do
  callback em `http_ai_plans.go`); env lido no Build. `accountId` obrigatório e validado UUID
  (400); token errado → 401; env ausente → 503.
- Conferir `moduleGatingRules()` em `back/internal/platform/app/app.go`: prefixo `/v1/runtime`
  precisa ficar FORA do gate de módulo (o runtime do automation já vive lá — seguir o mesmo
  registro).
- Aceite: curl com token certo devolve o agregado C9 (com e sem clientId); token errado 401;
  accountId inválido 400. O chat (B6) e o runtime devolvem o MESMO bloco de contexto.

### SPEC-B8 — Integração tasks: criar/vincular/desvincular (usa C10)
- `back/internal/modules/tasks/module.go`: accessor exportado `func (m *Module) Service()
  *Service` (devolve o service criado no Build; nil antes do Build).
- `back/internal/platform/app/app.go`: guardar a referência do módulo tasks (hoje
  `registry.MustRegister(tasks.New(...))` ~l.355 descarta) e injetar no calendar como provider
  LAZY: `calendar.New(storage, calendar.WithTasksService(func() *tasks.Service { return
  tasksModule.Service() }))` — closure resolve no primeiro uso, imune à ordem de Build.
- `back/internal/modules/calendar/task_link.go` (novo): 
  - `CreateLinkedTask(ctx, principal, accountID, event)` → `tasksSvc.ResolveAccessContext` +
    `CreateTask` (payload C10) + `AddRelation(module=calendar, resourceType=event,
    resourceId=eventID, labelCache="<date> — <title>")`. Config C6 sem boardId → erro
    `tasks_not_configured` (400 no handler).
  - `UnlinkTask(ctx, ...)` no DELETE do evento → novo `RemoveRelation` (abaixo).
  - Falha na criação da task NÃO desfaz o evento (evento salva, erro vira aviso no response:
    campo `taskWarning` no 201 — documentar no contrato do handler).
- `back/internal/modules/tasks/service_relations.go`: método exportado `RemoveRelation(ctx,
  access, taskID, module, resourceType, resourceID)` (apaga a linha + publica
  `task.relation_removed` — o front de tasks já escuta; AGENT.md do back l.120-122 pede isso).
  NÃO precisa expor rota HTTP DELETE nesta wave (uso interno) — se for trivial, registrar
  também `DELETE /v1/tasks/{taskId}/relations/{relationId}` (coexiste, não remove nada).
- RelationResolver do calendar: implementar a interface de
  `back/internal/platform/modules/relations.go` (~l.25-28) num `relations.go` do calendar
  (label = "<date> — <title>", url = `/calendario?date=<date>`) e registrar no
  `modules.NewRelationRegistry` em app.go (~l.346-350, hoje só erp/crm-erp/operations).
- `EventView.taskId`: LEFT JOIN `tasks.task_relations` (module='calendar',
  resource_type='event') no `ListEvents`/`GetEvent` do `store_postgres.go` — índice reverso
  `tasks_task_relations_module_idx` já existe; SEM N+1.
- Aceite: POST evento com `createTask:true` e config ok → task no board com
  `uiMetadata.source='calendar'` + relation + `taskId` no EventView; sem config → 400
  acionável; DELETE do evento remove a relation e NÃO arquiva a task; task de outra account
  jamais é lida/escrita (ResolveAccessContext garante).

### SPEC-B9 — Realtime: canal calendar + presença + optimistic locking (usa C11/C12)
- Migration `0188_calendar_events_version.sql` (C12; idempotente, `add column if not exists`).
- `back/internal/modules/realtime/model.go`: helper de tópico `calendarAccountTopic(accountID)`
  (`calendar:account:{id}`) + constantes dos 7 eventos C11 + (se necessário) campo `date`/
  `monthKey` no payload map (usar `Payload map[string]any` existente — NÃO inchar o struct).
- `back/internal/modules/realtime/service_calendar.go` (novo): `HandleCalendarSocket`
  reusando `serveSubscriptionSocket` (service_tasks.go ~l.702) + `authorizeCalendarAccount`
  (cópia adaptada de `authorizeTasksAccount` ~l.488 trocando as permission keys para
  `calendar.view` — mesma query de conta ativa + membership + permissão efetiva;
  platform_admin bypass) + `PublishCalendarEvent(evt Event)`.
  Presença: o ponto REAL de mudança é `resolvePresenceSubscription` (service_tasks.go ~l.408)
  — o switch de scope (~l.450) só aceita `board|task` e o parse de tópico (~l.416-434) só
  aceita prefixos `presence:board:`/`presence:task:`; adicionar case `calendar` + prefixo
  `presence:calendar:` + autorização `authorizeCalendarAccount` (fieldKeys C11).
  `HandlePresenceSocket` (~l.227) só delega, não muda.
- `back/internal/modules/realtime/http.go`: rota `GET /v1/realtime/calendar`.
- `back/internal/modules/calendar/publisher.go` (novo): interface `Publisher` (espelho de
  `tasks/publisher.go`) + no-op default; app.go injeta o realtimeService
  (`calendar.WithPublisher(realtimeService)`). Service publica nos pontos de escrita:
  create/update/delete evento, PutNotes, PutDayMedia, PutConfig, ApplyPlanResult (→
  `calendar.plan_updated` — de quebra o polling do modal de IA pode morrer depois).
- `PUT /events/{id}` com `If-Match` (C12): parse no handler, service compara e devolve
  `ErrVersionConflict` → 409 `version_conflict` no `writeServiceError`; update incrementa
  version. `EventView` ganha `version`.
- Aceite: 2 sessões (contas iguais) — criar evento numa reflete evento `calendar.event_created`
  na outra; conta DIFERENTE não recebe nada; socket sem permissão calendar.view não completa o
  handshake (fecha antes do 101, como tasks); PUT com If-Match velho → 409; sem If-Match →
  segue funcionando (compat). `golangci-lint` limpo.

## LANE FRONT (sequencial — um agente por spec)

### SPEC-F6 — Drawer de config com abas (substitui a página; usa C6)
- **PRIMEIRO: lado TS do contrato C6** (sem isso o gemini fica inselecionável e salvar
  qualquer aba APAGA a config de tasks — o PUT é full-replace e `normalizeConfig` dropa
  chaves não listadas):
  - `web/app/utils/calendar-config.ts`: `gemini` no union `CalendarAiProvider` (l.8) + nos 3
    mapas `AI_PROVIDERS` / `AI_PROVIDER_LABEL` ("Gemini (Google, free tier)") /
    `AI_PROVIDER_BASE_URL` (`https://generativelanguage.googleapis.com/v1beta/openai`).
  - Tipo `CalendarConfig` TS + `defaultCalendarConfig()`: seção
    `tasks: { boardId: '', defaultColumnId: '' }`.
  - `web/app/domain/calendar/calendar-api.ts` `normalizeConfig` (~l.52-64): merge POR SEÇÃO
    incluindo `tasks` (linha antiga sem a chave ganha o default; draft sem `tasks` NUNCA
    apaga o valor persistido).
- NOVO `web/app/components/calendar/config/CalendarConfigDrawer.vue` sobre
  **`OmniEntityDrawer`** (casca canônica — `web/app/components/ui/OmniEntityDrawer.vue`, doc
  `docs/frontend/MODAL_TEMPLATE.md`), modo `side`. Abas (nav estilo `SettingsTabs` — CSS
  próprio `calendar-config__tabs`, ou `UTabs`; escolher UM e manter):
  `responsaveis` · `feriados` · `aparencia` · `ia` · `clientes` · `integracoes` · `midia`.
- REAPROVEITAR os componentes existentes de `web/app/components/calendar/config/` dentro das
  abas (ConfigResponsibles/ConfigHolidays/ConfigAppearance/ConfigAi/ConfigClientProfiles/
  ConfigMediaLimits) — NÃO reescrever. Aba nova `integracoes` = NOVO `ConfigTasks.vue`
  (select de board + coluna; fontes: `useTasksStore` boards — carregar lazy só ao abrir a aba;
  aviso acionável quando vazio: "Nenhum board — crie um na página de Tasks").
- Modelos de salvar EXPLÍCITOS por aba: `responsaveis|feriados|aparencia|ia|integracoes`
  compartilham o draft do `CalendarConfig` + botão "Salvar configurações" no footer do drawer
  (badge de não-salvo); `clientes` e `midia` salvam com botão PRÓPRIO dentro da aba (já é
  assim — deixar o footer some/disable nessas abas pra não mentir).
- Dirty-guard ÚNICO via `ui.confirm` (padrão da casa): fechar drawer ou trocar de aba com
  draft sujo pergunta; trocar `window.confirm` do ConfigClientProfiles por `ui.confirm`.
- Gatilho: `onConfig()` em `web/app/pages/calendario/index.vue` (~l.216-219) passa a abrir o
  drawer (estado local da página) em vez de `navigateTo`. Deep-link: query `?config=<aba>`
  lida no mounted (abre o drawer na aba) e escrita ao navegar entre abas (router.replace).
- `web/app/pages/calendario/config.vue` → substituir o conteúdo por redirect
  (`navigateTo('/calendario?config=responsaveis', { replace: true })` em setup) OU deletar a
  página e criar redirect via `definePageMeta`/middleware — escolher o mais simples que
  preserve o link antigo. Mover o que a página fazia no mounted (`store.fetchConfig()` +
  `store.fetchMembers()`; o `store.init()` já roda na /calendario) para o open do drawer —
  o índice de perfis de cliente já é lazy DENTRO do ConfigClientProfiles
  (useCalendarClientProfiles), nada a mover.
- CSS: reusar `assets/styles/calendar/config.css` (adaptar seletores da página → drawer;
  remover o grid auto-fit da página; manter <450, se estourar dividir `config-drawer.css`).
- Aceite: engrenagem abre o drawer SEM sair do calendário (scroll/mês preservados); todas as
  7 abas funcionam e salvam como antes; **gemini aparece e persiste no select de provider da
  aba IA; salvar a aba Aparência NÃO apaga `tasks.boardId` salvo antes** (roundtrip GET→PUT→GET);
  `/calendario/config` redireciona; fechar com edição pendente pergunta via ui.confirm;
  eslint limpo.

### SPEC-F7 — Chat flutuante + voz (usa C7/C8; coda contra o contrato, não espera B6)
- NOVO `web/app/composables/useCalendarChat.ts` (molde `useOmniChat.ts`): estado
  `messages/draft/sending/errorMessage/conversationId`, `ask()` → POST `/v1/calendar/chat/ask`
  com `{question, conversationId, clientId: store.selectedClientId, month: mesEmFoco}`;
  `newConversation()` zera; AbortController cancela em voo; erros 503/502/504 →
  `errorMessage` acionável ("configure CALENDAR_CHAT_WEBHOOK_URL / importe o workflow").
- NOVO `web/app/components/calendar/CalendarChatPanel.vue`: painel flutuante (Teleport body,
  `position:fixed` canto inferior direito, z-index acima do drawer; `.calendar-page` tem
  overflow:hidden — precedente CalendarAiPlanModal), header (título + nova conversa + fechar),
  stream de bolhas user/assistant (padrão visual do OperationSidePanel §chat, classes próprias
  `calendar-chat__*`), "digitando…", input com auto-grow + Enter envia.
- FAB: NOVO botão flutuante na página (Teleport body, fixed bottom-right) que abre/fecha o
  painel; badge de erro quando `errorMessage`. A aba `ia` do drawer (F6) ganha um botão
  "Abrir chat" que chama o mesmo estado (provide/inject ou store — estado do chat vive no
  composable singleton via `useState`).
- VOZ: NOVO `web/app/composables/useVoiceRecorder.ts` — `MediaRecorder`
  (`audio/webm;codecs=opus`, fallback `audio/mp4`), limite 2min (para sozinho), estados
  idle/recording/transcribing, permissão negada → mensagem acionável. Botão mic no painel:
  gravar → parar → POST multipart `/v1/calendar/chat/transcribe` → texto entra no INPUT
  (usuário revisa e envia — não envia direto). Erros C8 exibidos no painel.
- CSS novo `assets/styles/calendar/chat.css` (tokens, sem hex; registrar no barrel
  `calendar.css`).
- Aceite: FAB abre o chat; pergunta → resposta renderizada (com back mockado por curl);
  cliente filtrado + mês em foco vão no body; mic grava e o texto transcrito aparece no input;
  todos os estados de erro têm mensagem acionável; sem `any`/`console.log`.

### SPEC-F8 — Tasks no front do calendário (usa C10; coda contra o contrato)
- `web/app/utils/calendar.ts`: `taskId?: string` e `version?: number` no tipo de evento;
  `createTask?: boolean` no input de criação.
- `CalendarEventForm.vue`: toggle "Criar task no board" (visível ao CRIAR evento; sugerido
  pré-ligado quando type ∈ {gravacao, reuniao, evento}); se a config `tasks.boardId` está
  vazia, o toggle vira aviso acionável com link "Configurar" → abre o drawer na aba
  `integracoes` (F6). No submit passa `createTask` ao store.
- `stores/calendar.ts` (CUIDADO: 438 linhas — extrair p/ composable se estourar):
  `createEvent` repassa `createTask`; response 201 com `taskWarning` → toast de aviso (evento
  salvou, task falhou).
- Badge/link: `DayDrawer.vue` mostra "Task vinculada" quando `taskId` (link →
  `/tasks?task=<id>` — conferir o deep-link real do board de tasks em
  `web/layers/tasks/pages/tasks.vue`; se não existir query de abertura, linkar `/tasks` e
  registrar a limitação no Progress Log). `EventChip.vue` NÃO muda (evitar poluição visual).
- Aceite: criar evento com toggle → POST com `createTask:true`; sem board configurado o
  toggle avisa e linka a aba; evento com taskId mostra o link no drawer do dia.

### SPEC-F9 — Realtime + presença no front (usa C11/C12; coda contra o contrato)
- NOVO `web/app/composables/useCalendarRealtime.ts` (~80-120 l, molde
  `web/layers/tasks/composables/useTasksRealtime.ts` sobre a base genérica
  `web/layers/tasks/composables/useRealtimeSocket.ts` — importar cross-layer, precedente já
  existe; NUNCA usar `auth.activeTenantId` como fonte única de conta — cadeia
  `resolveRealtimeAccountId` da base). Path `/v1/realtime/calendar`, scope=account.
  Aplicação por INVALIDAÇÃO com debounce 200-300ms: `calendar.event_*`/`day_media_updated` →
  refetch da janela (`store.fetchEvents` + day-media); `note_updated` → recarrega a nota do
  mês SÓ se carregada e SEM edição pendente (draft do usuário vence — princípio 1);
  `config_updated` → `store.fetchConfig`; `plan_updated` → notifica `useCalendarAiPlans`
  (encerra polling se o plano ativo ficou done/error). Montar no
  `pages/calendario/index.vue`; desligar no unmount; troca de conta reconecta (a base cuida).
- NOVO `web/app/composables/useCalendarPresence.ts` (molde
  `web/layers/tasks/composables/useTaskPresence.ts`, versão REDUZIDA): scope=calendar,
  heartbeat, participantes; `field_focus/blur` com fieldKey `notes:YYYY-MM` (editor de notas)
  e `event:<id>` (CalendarEventForm aberto). UI: avatares no `CalendarControls` (quem está no
  calendário) + badge "Fulano editando" no `MonthNotesPanel` e no `CalendarEventForm` quando
  o fieldKey correspondente está focado por OUTRO usuário. Sem lock exclusivo na v1 (só
  indicador) — documentar.
- C12 no front: guardar `version` do evento carregado; `updateEvent` envia `If-Match`; 409 →
  toast acionável "Este item foi alterado por outra pessoa" + botão recarregar (draft não é
  descartado sem aviso).
- Aceite: 2 abas na mesma conta — criar/editar evento numa atualiza a outra sem F5 (Network
  mostra WS + refetch, sem polling); editar notas numa mostra badge na outra; editar o MESMO
  evento nas duas → a segunda recebe 409 acionável; conta diferente não vê nada.

## LANE N8N/DOCS (paralela)

### SPEC-W2 — Workflow "Calendar Chat" (JSON importável) + doc
- NOVO `automation/export/workflow-calendar-chat.json` (referências de estrutura:
  `workflow-omni-chat.json` p/ AI Agent/webhook/respond — a memória de lá é
  `memoryBufferWindow`, NÃO copiar; o nó de memória Redis vem do `workflow-whatsapp.json`
  ~l.234-245 (`memoryRedisChat` + credential "Redis account"); `workflow-calendar-omni.json`
  p/ o padrão provider/baseUrl):
  1. **Webhook** POST path `calendar-chat`, responseMode=responseNode — recebe C7.
  2. **Code "Montar contexto"**: system = `ai.systemPrompt` || DEFAULT pt-BR ("assistente de
     estratégia de conteúdo da agência; use o contexto do cliente/mês; responda direto") +
     serializa `context` (perfil, feriados, eventos, notas, planos) no system; resolve
     `baseUrl` pelo mapa DEFAULT_BASE **+ `gemini:
     https://generativelanguage.googleapis.com/v1beta/openai`**.
  3. **AI Agent** (`@n8n/n8n-nodes-langchain.agent`, mesmo build do omni-chat; SEM tools —
     estão quebradas no n8n 2.23.2) + **Redis Chat Memory**
     (`memoryRedisChat`, credential "Redis account" existente, `sessionKey` do payload,
     janela ~10) + chat model: **OpenAI Chat Model com baseUrl dinâmico** apontando pro
     provider OpenAI-compatible (gemini/deepseek/etc.); p/ `claude`, ramo HTTP Request
     (`/v1/messages`, credential `calendar-ai-claude` existente) SEM memória de agente —
     registrar a limitação no doc (memória Redis só no ramo agent; claude usa fallback
     stateless na v1 OU aceitar gemini como provider default do chat em dev).
  4. **Respond to Webhook**: `{ "answer": ... }`.
- Credential nova: `calendar-ai-gemini` — o TIPO depende do nó que a usa: no ramo AI Agent
  com `lmChatOpenAi`, a credential é do tipo **openAiApi** (API key do AI Studio + baseURL
  `https://generativelanguage.googleapis.com/v1beta/openai`); Bearer/Header Auth SÓ vale em
  nó `httpRequest` (padrão do calendar-omni). Documentar troca manual: 1 credential ativa por
  vez, mesma limitação do calendar-omni.
- NOVO `docs/automation/CALENDAR_CHAT_WORKFLOW.md`: import (CLI `n8n import:workflow`,
  `MSYS_NO_PATHCONV=1` no Git Bash), credentials, envs do Omni, payload C7 de teste (curl),
  limitações (memória por sessionKey no Redis, claude sem memória na v1, tools desligadas).
- Aceite: JSON parseia, nós conectados, doc completo com curl de teste.

### SPEC-W3 — Workflow "Calendar Transcribe" (Whisper) + doc
- NOVO `automation/export/workflow-calendar-transcribe.json`:
  1. **Webhook** POST path `calendar-transcribe`, binário (multipart `file`),
     responseMode=responseNode.
  2. **OpenAI transcribe** — COPIAR o padrão comprovado do `workflow-whatsapp.json` (nó
     `Transcrever Audio` ~l.429-451: `@n8n/n8n-nodes-langchain.openAi`, resource `audio`,
     operation `transcribe`, `language: pt`, credential OpenAI existente `sCzmqFisO8bdeZ9B`).
  3. **Respond to Webhook**: `{ "text": ... }`.
- Alternativa DOCUMENTADA (não implementada): HTTP Request → Groq
  `https://api.groq.com/openai/v1/audio/transcriptions` (whisper-large-v3, free tier) — só
  trocar o nó 2 + credential Header Auth; e faster-whisper self-host (container extra,
  cuidado com memória da VPS/AC-11).
- NOVO `docs/automation/CALENDAR_TRANSCRIBE_WORKFLOW.md`: import, credential, env
  `CALENDAR_TRANSCRIBE_WEBHOOK_URL`, curl de teste com arquivo de áudio, limites (15MiB/2min).
- Aceite: JSON parseia, nós conectados, doc com curl de teste.

---

## Ordem/Dependências

- **Back B5 → B6 → B7 → B8 → B9** (sequencial, mesmo pacote Go; B6/B7 compartilham o
  agregado de contexto — quem rodar primeiro deixa a função, o outro pluga).
- **Front F6 → F7 → F8 → F9** (sequencial: F7/F8 ancoram no drawer de F6; todos tocam
  `stores/calendar.ts` — 438 linhas, EXTRAIR para composables ao crescer).
- **W2/W3 independentes** entre si e das outras lanes.
- Lanes back/front/n8n em PARALELO (áreas disjuntas). F7/F8/F9 codam contra os contratos
  C7-C12 (não esperam o back rodando).
- Depois: revisão adversarial → correções → build api (`docker compose up -d --build api`) +
  migration 0188 → lint (golangci/eslint/migrations) → validação do dono no browser → docs
  (3 docs sync: plano + AGENT.md + roadmap; panorama).

## Progress Log (preencher a cada etapa — erros, acertos, onde parou, o que falta)

| Quando | Etapa | Status | Notas |
| --- | --- | --- | --- |
| 2026-07-04 | Specs escritas (este arquivo) | ok | Decisões do dono no topo. Recon prévio: OmniEntityDrawer é a casca canônica; presença de tasks JÁ existe (molde); nó whisper comprovado no workflow WhatsApp; próxima migration livre = 0188; envs CALENDAR_AI_* ainda fora dos .env examples (B6 corrige). |
| 2026-07-04 | Revisão adversarial das specs (10 agentes: 6 recon + 4 verify) | ok | 2 CRÍTICOS corrigidos: (1) contradição C7×C9 (plansSummary vs plans) — unificado: context do chat = agregado C9 sem "account", mesma função Go; (2) lado TS do C6 estava sem spec dona (gemini inselecionável + normalizeConfig sem seção tasks apagaria boardId no full-replace) — virou o 1º item da SPEC-F6 + aceite. Menores: memória Redis copia do workflow-whatsapp (omni-chat usa buffer RAM); credential gemini tipo openAiApi p/ lmChatOpenAi (Bearer só em httpRequest); presença muda em resolvePresenceSubscription l.408 (não HandlePresenceSocket); mounted da config real é init+fetchConfig+fetchMembers (profiles index é lazy); PLAN §3.1 corrigido PATCH→PUT; §3.11 alinhado (badge só no drawer); escopo do wiring das outras IAs registrado como fora da wave. phases-part6.ts validado com o compilador TS do projeto (0 erros, 208 linhas, ids únicos). |
