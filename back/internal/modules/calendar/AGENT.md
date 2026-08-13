# AGENTS — modulo `calendar`

Agenda de conteudo por cliente da agencia. Painel Omni faz o CRUD. Plano canonico:
[docs/CALENDARIO_PLAN.md](/c:/Users/Mike/Documents/Projects/fila-atendimento/docs/CALENDARIO_PLAN.md).
Front em `web/app/components/calendar/` + `web/app/stores/calendar.ts` (ver
`web/app/components/AGENT.md` secao `calendar`).

## Padrao
Segue o molde do modulo `bio`: `model.go` (tipos + views), `store_postgres.go`
(pgxpool, schema `calendar.*`), `service.go` (regras + validacao + escopo),
`http.go` (rotas `/v1/calendar/*` + handlers), `http_media.go` (anexos/upload),
`media_storage.go` (disco), `media_normalize.go` (sanitizacao/escopo de `MediaItem` por conta),
`holidays.go` (datas comemorativas), `module.go` (Registry).
Perfil estrategico do cliente (Fase 4) em `profile.go` (tipos + service), `store_profile.go`
(persistencia) e `http_profile.go` (`RegisterProfileRoutes`, chamado no `module.go`).
Planos de IA (Fase 6) em `ai_plans.go` (tipos + service + `WithAI`), `ai_dispatch.go`
(payload C5 + POST ao n8n em goroutine + log), `store_ai_plans.go` (CRUD + transicoes de
status), `store_ai_context.go` (insumos do payload: nomes/perfis/feriados/nota sem N+1; `loadAccountNames`
resolve nome SO de contas ja referenciadas por evento/perfil DESTA account — barra enumeracao
cross-account de nomes por UUID arbitrario; tambem `ListEventsLean` = projecao lean de eventos
para o contexto compartilhado das IAs) e
`http_ai_plans.go` (`RegisterAIPlanRoutes`: painel autenticado + callback publico).
Contexto compartilhado das IAs (C9, WAVE 2) em `runtime_context.go` (`BuildAIContext` = agregado
account/client/month/holidays/monthNotes/events/plans, montado reusando `planContext` +
`ListEventsLean` + `ListAIPlans`; a MESMA funcao alimenta o bloco `context` do chat C7, apenas
sem o campo `account`) e `http_runtime.go` (`RegisterRuntimeRoutes`: `GET /v1/runtime/calendar/context`,
sem JWT, autenticado por `AUTOMATION_RUNTIME_TOKEN` em comparacao constant-time).
Integracao calendario<->tasks (C10, WAVE 2) em `task_link.go` (`WithTasksService` = Option do
`New`; `WithTasks` encadeia o provider LAZY no Build; `createLinkedTask`/`unlinkTask`/`eventDueDate`)
e `relations.go` (`NewRelationResolver` = interface `platformmodules.RelationResolver` do modulo,
registrada no `NewRelationRegistry` do `app.go`; label "<date> - <title>", url `/calendario?date=`).
O provider do modulo tasks entra via `calendar.New(storage, calendar.WithTasksService(func()
*tasks.Service { return tasksModule.Service() }))` no `app.go` — closure LAZY, imune a ordem de
Build. `createTask` NUNCA desfaz o evento: pre-condicao (sem `tasks.boardId` => 400
`tasks_not_configured`, antes de gravar) e best-effort depois (falha vira `taskWarning` no 201).
Integracao BIDIRECIONAL (WAVE 5) em `task_sync.go`: `taskSyncHandler` implementa
`platformmodules.RelationSyncHandler` (registrado LAZY no `RelationSyncRegistry` do `app.go`) — o
tasks avisa o calendar a cada mudanca de task e o `handleTaskSync` mantem o evento-espelho
(`maybeCreateMirrorEvent` com `events.source='task'`, `applyTaskSyncToEvent`, `deleteMirrorEvent`).
Sentido evento->task no `UpdateEvent` via `syncTaskFromEvent` -> `tasks.ApplyCalendarSync` (TERMINAL,
nao re-dispara). Guarda anti-loop: metodos terminais + `events.source` (0192) + `ui_metadata.source`.
DESCRICAO (WAVE 6.1): `event.Description` (texto simples) espelha no CORPO da task (`ContentHTML`, o editor
rico) via `descToHTML` (escapa + `<br>` + `<p>`). No `createLinkedTask` sempre; no `syncTaskFromEvent` SO
quando a descricao MUDOU (`UpdateEvent` compara old/new e passa `syncContent`) — assim editar outro campo do
evento NAO sobrescreve o texto rico que o usuario escreveu direto no editor da task. Sentido unico
(evento->task); o task->event preserva a descricao propria do evento (`eventToInput`).
Sync de status (E5) bidi pelo mapa `config.tasks.statusColumnMap` (`statusForColumn`/`columnForStatus`).
Cruzamento de MIDIA A (WAVE 6, read-only): a midia do evento (`ev.Media`) e espelhada na task vinculada
em `ui_metadata.calendarMedia` (so exibicao — a task so guarda video, aqui imagem+video cruzam; MESMA url
`/uploads/calendar/{conta}/`, sem duplicar arquivo). Coletada por `eventMediaForTask`; empurrada no
`syncTaskFromEvent` (update do evento). **WAVE 13: "anexos do dia" (`calendar.day_media`) ELIMINADO** —
toda midia pertence a um ITEM (evento), em `calendar.events.media`. Upload/reorder/remove sao por evento
(`PUT /events/{id}` full-replace de `media`); nao ha mais `ListDayMedia`/`PutDayMedia`/`pushDayMediaToTasks`/
`syncEventMediaToTask`/rotas `day-media`/realtime `day_media_updated`. Migration 0199 consolidou: anexos
vinculados foram para `events.media` do evento, orfaos viraram itens `source='media'`, e `drop table
calendar.day_media`. Cruzamento de MIDIA B (WAVE 6, o outro sentido): os
VIDEOS da task (`ui_metadata.videos`) sao espelhados no evento em `calendar.events.linked_media` (jsonb,
migration 0193). O `TaskSyncSnapshot.Media` (`MediaSnapshot` neutro) carrega os videos; `applyTaskSyncToEvent`
e `maybeCreateMirrorEvent` gravam via `store.SetEventLinkedMedia` (NAO passa por normalizeMedia — coluna
de exibicao propria, url `/uploads/tasks/`). `EventView.linkedMedia` -> o front une na "Midia do post".
Ambos TERMINAIS (ApplyCalendarSync / store direto; sem loop). TIPO/STATUS/PRIORIDADE do calendario sao
os MESMOS do tasks: fonte unica no front `web/app/utils/content-taxonomy.ts` (o `calendar.ts` deriva os
`*_META`; o tasks usa a mesma lista). O sync task->evento ainda valida o tipo (`eventTypeSet`) para tasks
antigas com tipo legado fora da taxonomia.
FUSO do mirror (WAVE 6): `mirrorEventDateTime` deriva a (data,hora) do evento a partir do dueDate da
task — task data-only nasce meia-noite UTC; converter cru para SP rolava para o DIA ANTERIOR, entao
meia-noite UTC = "sem hora" usa a DATA em UTC (dia inteiro) e hora real converte para SP.
"Evento sem task" (WAVE 6): `POST /v1/calendar/events/{id}/task` (`CreateTaskForEvent`) cria+vincula a
task de um evento sem task (reusa `createLinkedTask` C10, idempotente) — o botao do badge no DayDrawer.
`createLinkedTask` cria a task COMPLETA (2026-07-11, paridade com `syncTaskFromEvent`): titulo, corpo
(descToHTML), prazo, cliente, responsavel (id + `ui_metadata.responsible` com o NOME), `Priority`,
`ui_metadata.type` e `ui_metadata.calendarMedia` (midia do evento, read-only via `eventMediaForTask`).
ITEM ESPECIAL DE MIDIA (WAVE 11): `EventInput.IsMediaItem` (json `mediaItem`, client-controlled seguro)
=> o `CreateEvent` seta `Source='media'` server-side (o body continua SEM poder mandar 'task'). Upload
avulso do dia no front cria um evento assim (titulo = nome do arquivo, media=[item], createTask) — o
calendario esconde o titulo do chip (`EventChip` source='media'); a task nasce com o nome do arquivo.
EXCLUSAO (WAVE 6, "perguntar na hora"): `DELETE /v1/calendar/events/{id}?archiveTask=true` arquiva a
task vinculada junto (`archiveLinkedTask`); sem o param so remove a relation (task fica). Tolera 404
quando o archive ja apagou o evento-espelho (source='task') pelo sync.
Espelho task->evento ligado por padrao (`config.tasks.mirrorTasks=true`). Card nasce no topo
(`topSortOrder`, sort_order asc). IA propoe criar (E7 + **multi-tarefa WAVE 5.1**): o webhook devolve
`proposals[]` (lista); `chat.go` sanitiza cada uma (`sanitizeProposalList`/`sanitizeProposal`, teto
`maxChatProposals=31`), gera id por indice + status `pending` (`storedProposalsFrom`) e persiste em
`chat_messages.proposals`. A criacao real e do FRONT pela API autenticada do usuario (sem service-token
escrevendo), aprovando em LOTE. Status por proposta muda em `PATCH /v1/calendar/chat/conversations/{id}/
messages/{messageId}/proposals/{proposalId}/status` (`SetProposalStatus` idempotente: so item ainda
`pending`, via `jsonb_agg`+`jsonb_set` preservando a ordem).
CRUD de ANOTACAO e PERFIL do cliente pelo chat (WAVE 7): os `kind` de proposta ganham `note` e
`clientProfile` alem de event|task. `chat_proposals_crud.go` (novo) traz `ChatProposalNote`
(`{month,content,mode(append|replace)}`) e `ChatProposalProfile` (9 estaveis + `extra` +
`clearFields[]`/`clearAll`), os sub-objetos `ChatProposalFields.Note`/`.Profile`, e os sanitizers por
kind que `sanitizeProposal` (chat.go) DESPACHA (`canonicalProposalKind`/`sanitizeNoteProposal`/
`sanitizeProfileProposal`; note: create/update exige `content`, delete limpa; clientProfile: exige >=1
campo, delete exige clearAll|clearFields, `clientId` opcional). O cliente-alvo do perfil reusa
`fields.clientId` (a IA resolve por NOME do contexto ou o dono escolhe no cartao). A EXECUCAO e do FRONT
(`web/app/utils/calendar-chat-crud.ts`) pela API do usuario: anotacao via `PUT /notes/{month}` (ACRESCENTA
por padrao; mes ativo usa `store.setNotesForActiveMonth`, senao GET+aplica+PUT); perfil via
GET->merge->`PUT /client-profile` (full-replace preservando os campos nao tocados; clearAll usa o perfil
default). Insistencia: `runtime_context.go` `missingProfileFields` expoe os campos vazios — `ProfileMissing`
no `AIContextClientLean` (escopo all) + o no "Montar contexto" calcula de `ctx.client.profile` (escopo
client) — e o prompt manda a IA avisar o que falta e insistir com moderacao. Sem migration/env nova; so
re-importar o workflow `calendar-chat` (nos "Montar contexto" + "Extrair resposta" atualizados). Doc
[docs/CALENDARIO_SPECS6.md](/c:/Users/Mike/Documents/Projects/fila-atendimento/docs/CALENDARIO_SPECS6.md).
Secrets de IA (WAVE 3, SEC) em `secrets.go` (tipos `KeyStatus{set,last4}`/`KeyStatusView{scope,keys}`/
`GlobalSecrets` + service: `GetAccountKeyStatus`/`PutAccountKey`/`GetGlobalKeyStatus`/`PutGlobalKey`/
`resolveAIKey`/`mask`), `store_secrets.go` (interface `secretStore` + CRUD de `calendar.ai_secrets` por
conta e da chave GLOBAL em `core.platform_settings` key `calendar_ai_secrets`, mesmo padrao do
`media_limits`) e `http_secrets.go` (`RegisterSecretRoutes`, chamado no `module.go`). A API key CRUA
SO existe server-side (resolver/dispatch); o front recebe SO o status MASCARADO `{set,last4}` — NUNCA
a key crua. Escopo por PK composta `(account_id, provider)`: conta A nunca le/escreve o secret de B.
Listagem de modelos por provedor (Opcao C do painel) em `ai_models.go` (`ListAIModels` + mapa
`providerDefaultBaseURL` espelho do front + `filterChatModels`/`isChatModel` + `ErrModelsUnavailable`)
e `http_ai_models.go` (`RegisterAIModelsRoutes`, chamado no `module.go`). O campo Modelo do painel deixa
de ser texto livre: o back resolve a chave server-side (`resolveAIKey`) e faz `GET {baseURL}/models` do
provedor (endpoint CANONICO do mapa — a Base URL vinda do cliente NAO e usada aqui, para nao abrir SSRF;
a Base URL customizada segue valendo no dispatch/n8n). NAO passa pelo n8n e NAO aplica o kill switch
(e config, nao dispatch). A key so vai no header Bearer, nunca em log.
Chat de IA + voz (C7/C8, WAVE 2) em `chat.go` (tipos + service + `WithChat`: monta o payload C7
reusando `BuildAIContext` — bloco `context` = agregado C9 SEM `account`, via `chatContextFrom`, sem
remontar — e faz proxy fino ao n8n com `http.Client` sem Timeout global e deadline por `context`
(ask 60s / transcribe 120s); `sessionKey = accountId|userId|conversationId`, espelho de
`omniChatSessionKey`) e `http_chat.go` (`RegisterChatRoutes`: preflight sem tokens
`GET /v1/calendar/chat/status` + `POST /v1/calendar/chat/ask` +
`/chat/transcribe` + as rotas D3 de conversas/scope, RequireAuthWithAccount; `writeChatError` mapeia
os sentinels de upstream + `ErrInvalidClient` => 404 no chat). **WAVE 4 (D4)**: o Go passou a PERSISTIR
a conversa e a memoria (banco, ver "Chat com memoria" abaixo); o `sessionKey` agora usa o id da conversa
PERSISTIDA e o payload leva `history` (ultimas N do banco) — a memoria e o banco, nao mais so o Redis do n8n.
Dispatch com key + kill switch (WAVE 3, SPEC-B2, contrato PAY): o gate comum `resolveDispatchKey`
(em `ai_dispatch.go`) aplica o kill switch (`ai.enabled=false` => `ErrAIDisabled` => 409 `ai_disabled`)
e resolve a KEY CRUA do provider via `resolveAIKey` (`""` => `ErrAIKeyMissing` => 409 `ai_key_missing`),
SINCRONO, ANTES de disparar/criar a linha. A key crua entra no payload em `ai.apiKey` (chat: `chatPayloadAI`
= AIConfig + apiKey; plano: `aiPayloadAI.apiKey`, C5) e no multipart de transcribe (campos
`provider`+`apiKey`+`model`+`file`; provider `openai` usa a key `openai`, `gemini` usa `gemini`). A key
so existe server-to-server: NUNCA e logada (o payload cru nao vai para log) nem devolvida ao front. Os
mesmos sentinels sao mapeados em `writeChatError` E `writeServiceError` (helpers `writeAIDisabled`/`writeAIKeyMissing`).
Escopo da IA por cliente (WAVE 3.1, SPEC-B3, contratos CFG+/SEC+) em `client_ai.go` (constantes
`scopeModeGeneral`/`scopeModePerClient`; `EffectiveAIConfig(account,clientID)` = resolver da config
EFETIVA por cliente; service `GetClientAIOverride`/`PutClientAIOverride`; `mergeOverride`/`sanitizeOverride`/
`overrideHasValue`/`containsID`; view `ClientAIOverrideView{clientId,hasOverride,override}`), `store_client_ai.go`
(interface `clientAIStore` + CRUD do override em `calendar.client_profiles.ai_config`) e `http_client_ai.go`
(`RegisterClientAIRoutes`, chamado no `module.go`). A config `ai` da conta e o default GERAL; o modo
(`ai.scopeMode` = general|perClient) e as excecoes (`ai.disabledClientIds`) vivem no `config` jsonb (CFG+,
sem migration). O override de COMPORTAMENTO por cliente (`{enabled,provider,model,baseUrl,systemPrompt,
temperature}`, ponteiros p/ distinguir "nao setado" de zero) mora em `client_profiles.ai_config` (migration
0190) — a KEY NUNCA vive aqui (segue no nivel conta/global da 3.0; a key resolve pelo provider EFETIVO).
`EffectiveAIConfig`: `enabled` efetivo = `ai.enabled` E cliente fora de `disabledClientIds` E (perClient com
override => o `enabled` do override); em perClient com override, merge por campo (override nao-vazio vence).
Dispatch: o CHAT usa `EffectiveAIConfig(account, req.clientId)` (o clientId chega no request); o PLANO honra
`disabledClientIds` (`filterDisabledClients` pula clientes desativados na montagem do payload) mas mantem o ai
config GERAL da conta (override por-cliente no plano fica p/ depois); a TRANSCRICAO usa o config geral (sem
cliente no contexto do audio). O gate `resolveDispatchKey(ctx, account, enabled, provider)` passou a receber
`enabled bool` + `provider` (efetivo no chat, geral no plano/transcricao). Endpoints `GET/PUT /v1/calendar/
ai-config/client?clientId=` sob `RequireAuthWithAccount` (account-scoped e sensivel, como os secrets/chat da
3.0); `clientId` UUID obrigatorio (senao 400 `invalid_client`). Isolamento: conta A nunca le/escreve o override
de B (PK composta `(account_id, client_id)` + WHERE account_id no store).
Realtime + optimistic locking (C11/C12, WAVE 2) em `publisher.go` (interface `Publisher` +
`RealtimeEvent` + `noopPublisher` default + `WithPublisher` = Option do `New`; o modulo `realtime`
implementa a interface, direcao realtime->calendar, sem ciclo). O `Service` publica eventos LEAN
de invalidacao (`s.publishCalendar`) nos pontos de escrita: create/update/delete evento, `PutNotes`,
`PutDayMedia`, `PutConfig`, `ApplyPlanResult` e **`PutClientProfile` (WAVE 10, `calendar.client_profile_updated`,
resourceId=clientId — a aba Clientes refaz o fetch sem reload)**. O front so recebe a dica e refaz o fetch (nunca
patch local). Injecao no `app.go`: `calendar.New(storage, ..., calendar.WithPublisher(realtimeService))`.
Chat com memoria + escopo de clientes (WAVE 4, D1/D2, SPEC-B10) em `chat_store.go` (tipos
`ChatConversation`/`ChatMessage`/`ChatConversationInput` + interface `chatConversationStore` embutida em
`calendarStore`; CRUD account-scoped de conversas/mensagens em `calendar.chat_conversations`/
`calendar.chat_messages`, soft-delete + `order by created_at`, espelho do `tasks.repository_postgres_collab`;
`ListConversations` = agencia ve TODAS da conta com o nome do autor, cliente-side so as `created_by=ele`;
`AppendMessage` grava SO em conversa VIVA da MESMA conta via insert-select-where (senao `ErrNotFound`);
`ListLastMessages(n)` = ultimas N em ordem cronologica p/ a memoria do LLM; `IsAgencyOfAccount` resolve a
visibilidade org-aware NO BANCO, espelho de `auth/account_checker` + `core/store_postgres`) e `chat_access.go`
(`resolveChatAccess(principal,account) -> ChatAccess{IsAgency,VisibleClientIDs}`; `IsAgency` = platform_admin
OU agency_owner na org da conta; `VisibleClientIDs` REUSA a lista permission-scoped de `/v1/tenants` via
`clientScopeLister` = `tenants.Service.ListAccessible`, injetada por `WithClientScope` no Build — sem duplicar
a query de escopo; `canSelectScope = IsAgency || len(visibleClients)>1`, `lockedClientID` trava o cliente-side;
`validateScope` normaliza (`scopeMode` `client`|`all` + `scopeClientId`) SEM confiar no body — cliente fora do
visivel => `ErrInvalidClient`, `all` so p/ quem tem select; `authorizeConversation` = dono OU IsAgency, senao
`ErrNotFound`/404). Acesso resolvido 100% server-side; conversa/cliente fora do visivel NUNCA vaza (404).
**SPEC-B11 (D3/D4/D5)** consome a fundacao B10: `chat.go` `ChatAsk` reescrito (assinatura `(ctx, account,
principal, req)`) persiste a conversa COM memoria e escopo — (1) `resolveChatAccess`; (2) `resolveChatTarget`
normaliza o escopo (existente valida dono-ou-agencia + REVALIDA o escopo salvo contra o acesso atual; nova ainda
NAO materializa); (3) checa IA EFETIVA + KEY CRUA ANTES de materializar/gravar (sem conversa orfa se a IA esta
off); (4) cria a conversa nova so entao; (5) `history` = ultimas N (`chatHistoryLimit=40`, WAVE 17: le quase a
conversa inteira "tipo WhatsApp"; o teto e a limpeza p/ nao estourar tokens) JA existentes carregadas ANTES de
gravar a pergunta (a pergunta vai no campo `question`, nao no `history`, p/ nao duplicar quando o n8n concatena
system+history+question, D5); `toHistory` reanexa ao content do ASSISTENTE um resumo compacto dos cards que ele
propos (`summarizeStoredProposals`/`describeStoredProposal`, WAVE 17: acao/tipo/titulo/data/cliente/status,
bounded `maxHistoryCardsPerMsg=20`) — as `Proposals` ficavam salvas mas nao viajavam no history, e a IA nao
lembrava o que mandou nos cards; (6) contexto `client` => `BuildAIContext`, `all` => `BuildAIContextAll`;
(7) grava a resposta (role=assistant), titula pela 1a pergunta (`deriveChatTitle`, `TouchConversation` bump de
`updated_at` — `AppendMessage` NAO move) e responde `{answer,conversationId,title}`. `chatWebhookPayload` ganhou
`History []chatHistoryMessage{role,content}` e `Context any` (client => `calendarChatContext`, all =>
`AIContextAll`). `runtime_context.go` `BuildAIContextAll(account,visibleClientIDs,month)` = agregado LEAN
multi-cliente (contrato D4): resumo `{id,name,segment,brandVoice(trunc 280)}` de cada cliente (teto
`maxContextClients=30`, reusa `planContext` sem N+1) + feriados/nota + eventos lean do mes de TODOS os clientes
(teto 100); helpers `capClientIDs`/`truncateRunes`. WAVE 17: `buildChatContext` nomeia todo cliente VISIVEL do
contexto (`fillLeanClientNames` no all; `block.Client.Name` no client) a partir de `access.VisibleClients`
(id+nome do select de escopo) — `loadAccountNames` so nomeia cliente com evento/perfil, entao cliente visivel SEM
evento/perfil viajava sem nome e a IA nao o citava ("faltou Duby/Bari"); nao afrouxa a trava (mesmos nomes do
select). `chat_conversations.go` (novo) = camada service das rotas D3
(`ListChatConversations`/`GetChatConversation`/`CreateChatConversation`/`DeleteChatConversation`/`ChatScope`) +
views (`ChatConversationSummary`/`ChatConversationDetail`/`ChatMessageView`/`ChatScopeView`/`ChatScopeClient`) +
helpers do ask (`resolveChatTarget`/`buildChatContext`/`deriveChatTitle`/`ptrToStr`). `chat_access.go` ganhou
`resolveChatContext` (acesso + clientes NOMEADOS numa UNICA ida ao tenants scope) e `visibleClients` (id+name);
`ChatAccess.VisibleClients` (id+nome) + `clientNameByID()` alimentam o preenchimento de nome do contexto (WAVE 17).

## Schema (`calendar`) — migrations 0181/0182/0183/0185/0186/0188/0189/0190/0191
- `calendar.events` (0181; **+`version` na 0188**): id, **account_id** (dono, FK core.accounts),
  **client_id** (cliente/tenant do evento, nullable FK core.accounts), event_date, event_time,
  type, title, status, priority, responsible_id, involved_ids (jsonb), media
  (jsonb = `MediaItem[]`), description, timestamps, **`version` (integer not null default 1)**.
  `version` e o contador de optimistic locking (C12): toda escrita bem-sucedida faz
  `version = version + 1`; o PUT pode enviar `If-Match: <version>` e divergencia => 409.
- `calendar.notes` (0181): (account_id, month_key `YYYY-MM`) PK, content (HTML), updated_by, updated_at.
- `calendar.config` (0182): account_id PK + `config jsonb` (shape em `CalendarConfig`).
  **C2 (SPEC-B3, sem migration nova)**: `responsibleUserIds[]`,
  `holidays{brNational,sergipe,aracaju,luxuryIntl}`, `weekStartsOn` (sunday|monday,
  default sunday), `clientColors{[clientId]:"#rrggbb"|"none"}`, `typeColors{[tipo]:"#rrggbb"}`,
  `whiteLabel{logoUrl,title,primaryColor}`, `ai{provider(claude|deepseek|qwen|kimi|glm|gemini|custom),
  model,baseUrl,systemPrompt,temperature}`.
  **C6 (SPEC-B5, sem migration nova)**: provider ganha `gemini` (camada OpenAI-compatible do
  Google AI Studio, free tier) + secao `tasks{boardId,defaultColumnId}` (integracao
  calendario<->tasks; ambos UUID-ou-vazio, vazio = integracao DESLIGADA). `defaultConfig()`
  preenche TODOS os campos (incl. `Tasks:{}`) e o `GetConfig` desserializa POR CIMA dos defaults
  (linha antiga so com responsaveis/feriados, ou sem a secao `tasks`, ganha o shape completo —
  struct de valor com chave ausente/null vira no-op, entao `tasks` fica `{boardId:"",defaultColumnId:""}`).
  **CFG v4 (WAVE 3, sem migration nova)**: `ai` ganha `enabled` (kill switch, default true),
  `useGlobalKeys` (true = chaves GLOBAIS da plataforma, false = chaves DESTA conta; default true),
  `provider` ganha `openai`, `transcribeProvider(openai|gemini, default gemini)` + `transcribeModel`;
  nova secao `chat{position(center|left|right, default center),width,height(px, clamp 0..2000)}`
  (layout da janela de chat). `PutConfig` sanitiza (enums + clamps via `sanitizeAI`/`sanitizeChat`);
  `GetConfig` completa o shape v4 para conta antiga (transcribeProvider->gemini, chat.position->center).
  CHAVES de API NUNCA vivem no `config` jsonb — moram nos secrets (`calendar.ai_secrets` / global no
  `platform_settings`), resolvidas SO server-side (ver "Secrets de IA").
  **CFG+ (WAVE 3.1, sem migration nova)**: `ai` ganha `scopeMode(general|perClient, default general)` +
  `disabledClientIds[]` (clientes com a IA desligada, excecoes; valem nos DOIS modos). `sanitizeAI`
  normaliza (scopeMode case-insensitive canonico; disabledClientIds = UUIDs validos dedup via
  `normalizeClientIDs`); `GetConfig` completa o shape v4.1 para conta antiga (scopeMode->general,
  disabledClientIds->[]). O override de COMPORTAMENTO por cliente NAO vive no `config` — mora em
  `client_profiles.ai_config` (ver abaixo); a config aqui so guarda o modo + a lista de excecoes.
- `calendar.day_media` (0183): (account_id, event_date) PK + `media jsonb` (`MediaItem[]`) —
  anexos AVULSOS do dia (sem vinculo com evento).
- `calendar.client_profiles` (0185; **+`ai_config` na 0190**): (**account_id**, **client_id**) PK, ambos
  FK core.accounts on delete cascade. Perfil estrategico 1:1 por cliente (segment, positioning, description,
  history, site_url, instagram, address, objectives, brand_voice, `extra jsonb`, updated_by,
  timestamps). Insumo da IA (Fase 6). Perfil e OPCIONAL por design. Contrato C3.
  **`ai_config jsonb not null default '{}'` (0190, WAVE 3.1 SEC+)**: override de COMPORTAMENTO da IA por
  cliente (`{enabled,provider,model,baseUrl,systemPrompt,temperature}`, sem keys — so muda comportamento,
  nunca a credencial). `PutClientAIOverride` faz upsert: cria a linha so com account/client/ai_config se o
  perfil nao existe, ou no conflito atualiza SO o `ai_config` (preserva o perfil estrategico). So o modo
  `perClient` consulta o override (ver `EffectiveAIConfig`). Contrato SEC+.
- `calendar.ai_plans` (0186): `id` PK (uuid), **account_id** (dona do calendario, FK core.accounts
  on delete cascade), `month_key` (`YYYY-MM`), `client_ids jsonb` (uuids escolhidos), `status`
  (pending -> done|error -> applied), `provider`/`model` (snapshot da config no disparo),
  `content jsonb` (shape C4.content, preenchido no callback), `error`, `created_by`, timestamps.
  Indices (account_id, month_key) e (account_id, created_at desc). Contrato C4.
- `calendar.ai_secrets` (0189, WAVE 3 SEC): (**account_id** FK core.accounts on delete cascade,
  **provider** text) PK composta + `api_key` (raw, server-side apenas), `updated_by` (text),
  `updated_at`. Guarda a API key CRUA da IA por conta. A key SO sai no resolver/dispatch; o front
  recebe apenas o status MASCARADO `{set,last4}`. A chave GLOBAL da plataforma vive fora desta tabela,
  em `core.platform_settings` key `calendar_ai_secrets` (`{gemini,glm,openai}`, so platform_admin escreve).
- `calendar.chat_conversations` (0191, WAVE 4 D1): `id` PK (uuid), **account_id** (FK core.accounts on
  delete cascade), **created_by_user_id** (FK core.users on delete cascade, o dono da conversa), `title`,
  `scope_mode` (`client`|`all`, default client), `scope_client_id` (uuid nullable, preenchido no modo client),
  timestamps + `deleted_at` (soft-delete). Indice parcial `(account_id, created_by_user_id, updated_at desc)
  where deleted_at is null`. Conversas HIBRIDAS: cliente-side ve so as suas, agencia ve todas da conta.
- `calendar.chat_messages` (0191 + 0194 + **0195**): `id` PK (uuid), **conversation_id** (FK
  calendar.chat_conversations on delete cascade), **account_id** (FK core.accounts on delete cascade, defesa
  em profundidade), `role` (`user`|`assistant`), `content`, `proposal jsonb`, `proposal_status`
  (`none`|`pending`|`accepted`|`rejected`), **`proposals jsonb not null default '[]'` (0195, multi-tarefa)**,
  `calendar_items jsonb` e `created_at`. Indice `(conversation_id, created_at)`.
  **MULTI-TAREFA (WAVE 5.1)**: uma mensagem pode trazer VARIAS propostas de criacao — `proposals` e um array
  `[{id,action,kind,fields,status}]`, cada uma com **status proprio** (`pending|accepted|rejected`) e id
  estavel (indice na mensagem). `action` (`create`|`update`|`delete`) + `fields.targetId` dirigem o **CRUD pelo chat** (create/update/delete
  de EVENTOS e TASKS do board configurado via `context.tasks` ou `taskId` vinculado). `fields` cobre `title/date/time/type/status/priority`,
  `responsibleId/involvedIds`, `description/contentHtml`, `dueDate/dueEndDate`, `clientId/clientName`,
  `columnId`, `archived`, `targetId`. `sanitizeProposal` valida por acao: update/delete exigem `targetId`;
  delete dispensa titulo; create exige titulo; update aceita edicao parcial (ex.: so prioridade/descricao).
  O front (`applyProposal`) executa evento via `store.updateEvent`/`deleteEvent`; task via store de tasks
  quando `targetId` for id real de `context.tasks` ou evento com `taskId` vinculado. `buildChatContext` acrescenta
  `tasks` com a projecao lean do board configurado (`maxContextTasks=100`) usando permissao real do usuario.
  O `proposal`/`proposal_status` singular vira retrocompat: o backfill da 0195 migra mensagem antiga para a
  lista de 1 (id '0') e o scan tem a mesma rede de seguranca. `calendar_items` guarda o snapshot de eventos
  reais cujos IDs vieram do contexto e foram revalidados pelo Go. Sem soft-delete (some junto com a conversa
  via cascade). Fonte da memória do LLM.
  **GUARDA DE ALVO POR DIA/CLIENTE (WAVE 14, `chat_target_guard.go`)**: o modelo erra DEMAIS escolhendo o
  alvo — chega a dizer "nao ha evento no dia 13" tendo um — entao o BACK RESOLVE o alvo, determinista, em
  vez de so validar a escolha da IA. Roda no `ChatAsk` ANTES de `resolveProposalTargets` (pode reescrever o
  `targetId`): `extractTargetCriteria` le a pergunta (dia ancorado no mes em foco via `chatDayNumberRe`/
  `chatNumericDateRe`; cliente por nome contra `contextClients`) e `guardProposalTargets` decide por
  update/delete com **prioridade-calendario** (`calendarMatches` = eventos do calendario que batem dia+cliente):
  **1 match** => REESCREVE `targetId` p/ esse evento (mantem os campos que a IA queria mudar) e devolve
  `resolvedTitle` -> o answer ganha "Vou alterar: X. Confirme no cartao."; **varios** => BARRA + lista SO os
  eventos do calendario p/ o dono escolher; **0 no calendario mas ha task** => BARRA + avisa "so existe em
  Tasks: X, quer alterar la?". Assim, com 1 evento no dia, o alvo esta SEMPRE certo, independe do modelo. O
  prompt do workflow reforca (regra PRIORIDADE DO CALENDARIO). Testes em `chat_target_guard_test.go`. Sem migration/env.
  **BUSCA AMPLA POR TITULO (WAVE 14, `appendWideTitleMatches` em chat_targets.go)**: o contexto e
  montado do MES EM FOCO da tela — se a tela esta em outro mes/ano, o item citado nem chega ao modelo
  ("nao encontrei", caso real: tela em 2025, Postagem Bari em 07/2026). Roda no `ChatAsk` apos o
  buildChatContext: titulo citado na pergunta que NAO esta nos eventos do mes => busca em ±24 meses
  (`wideSearchWindow`; mesmas queries scoped `ListEventsLean`/`ForClients`, limit 1000) e ANEXA os
  matches (max 8, `mergeCalendarItems`) ao contexto ANTES do LLM. Destaque/listas mostram o ANO quando
  o item e de outro ano que o foco (`titleWithDay`/`dedupCandidates` com `crit.focusYear`).
  **RESOLUCAO DE PESSOAS (WAVE 14, mesmo arquivo)**: `resolvePeopleInProposals` fecha o mesmo tipo de
  falha no `responsibleId`/`involvedIds` — o modelo manda lixo ("iasmin-id"), o NOME cru, ou ESQUECE o
  campo. Roda antes da guarda de alvo: valor que ja e id conhecido de `contextPeople` fica; NOME conhecido
  vira o id; valor lixo com a PERGUNTA citando 1 pessoa conhecida (e "responsavel") usa essa. Determinista,
  independe do modelo. O card mostra o NOME (o front resolve pelo id real).
  **INTELIGENCIA + REDES DE SEGURANCA (WAVE 15, docs/CALENDARIO_SPECS10.md)**: a inteligencia principal
  e o MODELO (gpt-4o) + systemPrompt do PAINEL + regras de dominio no workflow (typo/fonetica/consultoria/
  "o cartao e a confirmacao"/"update minimo"); o back so garante que escorregoes nao quebrem: (1) fuzzy de
  titulo como fallback do `titleMatchedEvents` (`fuzzyTitleMatch`, Levenshtein normalizada, janela de
  tokens k±1, limiar 0.25 + margem 0.10 — 1 candidato inequivoco ou nada); (2) `snapProposalTypes` — type
  fora de `eventTypeSet` vira o mais proximo com <=2 edicoes, senao e limpo (fonetico tipo "rios" e
  trabalho do modelo, nao do snap); (3) `resolveClientsInProposals` — clientId lixo resolve por
  clientName/cliente-citado ou e limpo, id valido preenche o label; (4) update/delete SEM targetId PASSA
  na sanitizacao (`sanitizeContentProposal`) para a guarda resolver pelo titulo citado; o que TERMINAR sem
  alvo cai em `dropTargetlessEditable` (pos-guarda) com aviso deterministico — antes a proposta morria
  antes da guarda e o answer mentia "preparei a proposta" sem card. (5) `ChatAskRequest.ViaVoice` viaja
  ate o webhook (`body.viaVoice`) e liga o aviso de "transcricao de audio, erros foneticos provaveis" no
  prompt. Config 100% do painel (`body.ai`); n8n so liga os nos.
  ATENCAO — validacao mora numa camada SO: o no "Extrair resposta" do n8n tinha a MESMA validacao
  (update/delete sem targetId => descarta; hasEditable ignorando clientId/clientName) e continuou matando
  as propostas DEPOIS que o back foi relaxado ("troca o cliente pra Bari" sumia no workflow). Ao mexer em
  validacao de proposta, conferir as 3 camadas: extrator do workflow, sanitize do back, apply do front.
  No front, task espelhada (targetId = id do EVENTO, reescrito pela guarda) roteia para o caminho de TASK
  (patch parcial) — pelo caminho de evento o full-replace vazava defaults (priority 'media') pra task via
  mirror e o delete apagava so o espelho.
  **LER PERFIL DO CLIENTE CITADO NO ESCOPO 'ALL' (WAVE 16, `appendNamedClientProfile` em chat_targets.go)**:
  no escopo 'all' os clientes vao ENXUTOS (`AIContextClientLean`: nome/segmento/tom + `profileMissing`) para
  nao estourar o contexto com muitos clientes — entao "traz os dados do cliente X" fazia a IA dizer "nao
  temos" (o dado EXISTE no banco, so nao viajava). O helper roda no `ChatAsk` apos `appendWideTitleMatches`:
  se a pergunta nomeia UM cliente visivel (`singleNamedClient`, inequivoco), busca o perfil COMPLETO dele
  (`GetClientProfile`, scoped por account) e o anexa como `AIContextAll.Client` (`*planClient`, json:"client")
  — o workflow ja renderiza "Cliente em foco" com todos os campos (MESMO bloco do escopo 'client'), SEM tocar
  no workflow. No escopo 'client' o perfil completo ja viaja: no-op. `foldChatLabel` ficou tolerante a
  pontuacao (";", "?", ":" viram espaco; "/" e ":" so sobrevivem ENTRE DIGITOS p/ data/hora "15/07"/"14:30")
  — senao "da Perola:" nao casava "Perola". `maxChatQuestion` subiu 4000->12000 (briefing falado longo).

  **RESOLUCAO POR NOME + ALVO NO CARD (WAVE 12, `chat_targets.go`)**: fecha o vai-e-volta de "preciso do ID".
  (1) `People []Member` entra no contexto dos DOIS escopos (`calendarChatContext.People` e `AIContextAll.People`,
  populados por `chatPeopleContext` = `ListResponsibles`, mesma fonte do GET /responsibles) — a IA resolve
  "responsavel vai ser a Iasmin" por NOME e devolve o id em `responsibleId`/`involvedIds`; o prompt do workflow
  proibe exigir ID do usuario (nome fora da lista viaja como NOME e o front `resolveResponsibleId` resolve).
  (2) `resolveProposalTargets` roda no `ChatAsk` sobre as proposals sanitizadas: `targetId` que veio como
  TITULO (a IA escorrega) e reescrito para o id REAL cruzando com o contexto autoritativo (match unico por
  titulo, sem acento, entre `context.events`+`context.tasks`); e cada alvo de update/delete vira um SNAPSHOT
  anexado aos `calendar_items` da mensagem (task pura vira `AIContextEvent` sintetizado via
  `aiContextEventFromTask`, `TaskID=ID`, data/hora pela MESMA heuristica de fuso do mirror
  `mirrorEventDateTime`/`taskDueDateParts`) — o card do front SEMPRE mostra o titulo e o "antes" do item que
  sera alterado, mesmo task sem evento (o front ja resolvia titulo/antes pelos `calendarItems` e ja esconde
  esses snapshots da secao "Calendario" pelo filtro de targetId). Testes em `chat_targets_test.go`.

## Anexos / midia (Fase 3)
- `MediaItem` = `{id,url,name,type("image"|"video"),contentType,sizeBytes,posterUrl?}`; `url` e
  `posterUrl` sempre `/uploads/calendar/{accountId}/{arquivo}`. A sanitizacao (`normalizeMedia`
  em `media_normalize.go`) valida AMBOS contra o prefixo COM o accountId do Principal
  (`/uploads/calendar/{accountId}/`, nunca o generico) — `url` fora do prefixo descarta o item,
  `posterUrl` fora apenas zera o campo. Isolamento multi-tenant (contrato C1): o file server em
  `/uploads/` e publico e sem escopo de conta, entao essa amarra e a UNICA barreira contra
  referenciar arquivo de outra conta no jsonb. accountID vem do `accountScope(r)`, nunca do body.
  `posterUrl` e a
  capa do video (opcional, so p/ `type "video"`), capturada no FRONT via canvas e subida como
  imagem normal via `POST /v1/calendar/media` (upload nao muda). Contrato C1 (SPEC-B1).
- Novos uploads usam o modulo central `storage`/Cloudflare R2 por adapter fino no `app.go`.

- O viewer de midia e modal, compartilhado com Tasks, navega lateralmente e sempre usa `contain`
  com aspect-ratio original. Miniatura de video tambem usa `contain`; nenhum quadro pode ser cortado.
- Depois que a API recebe integralmente o arquivo, o worker Go do storage entrega e retoma no R2
  pelo staging duravel; o usuario nunca reenviara o mesmo arquivo para concluir essa entrega.
  As URLs continuam `/uploads/calendar/{accountId}/{objectId}/{arquivo}` e sao lidas pela API com
  suporte a Range; o file server em disco permanece para arquivos legados. O original nao e
  convertido nem alterado; poster e um objeto derivado separado.
- Upload stateless: `POST /v1/calendar/media` valida mime (jpg/png/webp/gif/avif, mp4/webm/mov) +
  tamanho e devolve o `MediaItem`; o front anexa ao evento/dia e salva (full replace).
  A rota limpa, via `http.ResponseController`, os deadlines globais de leitura (15s) e escrita
  (30s), inclusive através dos wrappers Logging/Gzip; `ReadHeaderTimeout` e `MaxBytesReader`
  continuam ativos. Erros de transporte são distintos: 408 `upload_timeout`, 413
  `media_too_large`, 400 `invalid_media` e 503 `upload_unavailable` se o writer não aceitar
  controle de deadline.
  OBS: avif nao e sniffado pelo http.DetectContentType — passa pelo fallback do contentType
  declarado (normalizeImageMime). No FRONT, `/uploads/*` e SEMPRE absolutizado com
  `resolveMediaUrl(url, apiBase)` (utils/media.ts) — em dev web (:3003) e api (:9091) sao
  origens diferentes e a url relativa quebra thumb/viewer/fundo do dia (bug real corrigido
  2026-07-02 na validacao do dono).
- **Limite = config GLOBAL da plataforma** em `core.platform_settings` chave `media_limits`
  (`{imageMaxBytes(10MB), videoMaxBytes(300MB)}`), lida no upload. Sem tabela nova.

Responsaveis = usuarios REAIS (`core.account_users`+`core.users`, `display_name`). Fase 2
lista membros da account; `responsibleUserIds` vazio = todos os membros. (Puxar responsaveis
de contas-cliente cross-account = fast-follow com validacao de org.)

## Escopo (multi-tenant)
- A account ATIVA continua vindo do Principal validado por `RequireAuthWithAccount`.
  `GET /v1/calendar/scope` resolve o recorte autoritativo: conta-agencia usa a propria
  agenda e pode selecionar os clientes ativos da mesma organization; conta-cliente usa
  a agenda da conta-agencia ativa mais antiga da mesma organization e fica travada em
  `client_id = account ativa`. `StorageAccountID` e interno e nunca sai no JSON.
- O CRUD de eventos repete as duas dimensoes: `account_id = agenda resolvida` e, para
  cliente, `client_id = account ativa`. Filtro/body forjado e sobrescrito; detalhe,
  update, delete e criacao de task devolvem 404 para evento de outro cliente.
- Defesa em profundidade: o store filtra por `account_id` em todo GET/UPDATE/DELETE
  (recurso de outra account => `pgx.ErrNoRows` => 404, nunca 403).
- Rotas principais usam membership validada e as permissoes ACCOUNT-SCOPED
  `calendar.view`/`calendar.manage`; `RequireAuthWithAccount` hidrata a RBAC custom efetiva
  antes de `requireCalendarPermission`, incluindo overrides allow/deny. Nao voltar a ler apenas
  a matriz coarse global: em 2026-08-12 isso barrava com 403 a agency_member Crow com papel
  custom `editor`, mesmo com `calendar.view/manage` ativos na conta. `owner`/`platform_admin`
  preservam o bypass existente.
- Gating por modulo em `app.go` (`{Prefix: "/v1/calendar", ModuleID: "calendar"}`);
  platform_admin tem bypass.

## Endpoints
- `GET /v1/calendar/scope` — `{canSelect,lockedClientId,clients[]}`. Cliente recebe
  somente a propria account e `canSelect=false`; a conta-agencia recebe os clientes
  ativos da mesma organization. A conta de armazenamento nao e exposta.
- `GET /v1/calendar/events?from=&to=&clientId=` — eventos da janela (datas inclusive).
- `POST /v1/calendar/events` — cria (body = EventInput). **C10 (WAVE 2)**: aceita `createTask:true`
  para criar+vincular uma task no board da config C6. Sem `tasks.boardId` => **400 `tasks_not_configured`**
  (nao cria evento orfao). Com config ok: task no board (`uiMetadata.source='calendar'`, dueDate =
  date+time RFC3339 [time vazio = 09:00, UTC], clientAccountId = clientId do evento, responsibleUserId
  = responsibleId se UUID, coluna = `defaultColumnId` ou 1a do board) + relation (module=`calendar`,
  resourceType=`event`, resourceId=eventId) + `taskId` no 201. Falha na task DEPOIS do evento salvo =>
  evento permanece + `taskWarning` no 201 (best-effort; nunca desfaz o evento).
- `GET /v1/calendar/events/{id}` — detalhe (inclui `taskId` via LEFT JOIN de relations, C10; e
  `version`, C12).
- `PUT /v1/calendar/events/{id}` — substitui (full replace) e incrementa `version` (C12).
  Header `If-Match: <version>` **OPCIONAL** (sem header = comportamento legado, compat): com header,
  `version` divergente => **409 `version_conflict`**; `If-Match` nao-numerico => 400 `invalid_if_match`.
  A resposta traz `version` (novo) mas ainda NAO traz `taskId` (so `GET /events`, `GET /events/{id}`
  e o 201 do POST); o front preserva/refaz fetch. Escrita bem-sucedida publica `calendar.event_updated`.
- `DELETE /v1/calendar/events/{id}` — se ha task vinculada, remove a relation ANTES de apagar
  (a task NAO e arquivada; best-effort). C10. Publica `calendar.event_deleted`.
- `GET /v1/calendar/notes/{month}` — nota do mes (`YYYY-MM`; vazia se nao existe).
- `PUT /v1/calendar/notes/{month}` — upsert da nota (body `{content}`).
- `GET/PUT /v1/calendar/config` — config da account (shape completo C2/C6: responsaveis,
  feriados, weekStartsOn, clientColors, typeColors, whiteLabel, ai, tasks, **shortcuts**). PUT
  sanitiza no service (weekStartsOn no enum, cores `#rrggbb`/`none` validadas, provider no enum
  — inclui `gemini`, temperature clamp 0..1, `tasks.boardId`/`defaultColumnId` UUID-ou-vazio via
  `sanitizeTasks`, strings trim, **shortcuts via `sanitizeShortcuts`: acoes whitelist de
  `shortcutDefaults()`, tecla 1-char a-z/0-9 ou especial enter/escape/space/arrow*, vazio =
  desligado, invalido = default — WAVE 11**) e continua full-replace; GET devolve o shape
  completo mesmo para conta antiga (`tasks:{boardId:"",defaultColumnId:""}`).
- `GET /v1/calendar/members` — usuarios da account (candidatos a responsavel).
- `GET /v1/calendar/responsibles` — responsaveis efetivos (subconjunto do config ou todos).
- `GET /v1/calendar/holidays?from=&to=` — feriados/datas comemorativas da janela
  (read-only, so os `set` ligados no config). Resposta `{holidays:[{date,name,set}]}`
  ordenada por date; `set` in `brNational|sergipe|aracaju|luxuryIntl`. Datas fixas
  + moveis (Pascoa/Meeus, Carnaval, Corpus Christi, Dia das Maes/Pais, Black Friday,
  Cyber Monday) calculadas em `holidays.go` — sem tabela/migration.
- `POST /v1/calendar/media` — upload multipart (campo `file`) → grava e devolve `MediaItem`.
  Corpo limitado a `videoMaxBytes`+folga; > limite => 413 `media_too_large`; tipo invalido
  => 400 `invalid_media`; conexão interrompida/timeout de leitura => 408 `upload_timeout`.
- `GET /v1/calendar/day-media?from=&to=` — anexos avulsos por dia da janela (`{days:[{date,media}]}`).
- `PUT /v1/calendar/day-media/{date}` — full replace da lista do dia (body `{media:[MediaItem]}`).
- `GET /v1/calendar/media-limits` — tetos de upload (qualquer autenticado; o front mostra/valida).
- `PUT /v1/calendar/media-limits` — altera os tetos (**so platform_admin**; body = `MediaLimits`).
- `GET /v1/calendar/ai-keys` — status MASCARADO da FONTE ATIVA da conta (WAVE 3 SEC). Resposta
  `{scope:"global|account", keys:{gemini:{set,last4}, glm:{...}, openai:{...}}}`. `scope` depende de
  `ai.useGlobalKeys` da config. NUNCA devolve a key crua.
- `PUT /v1/calendar/ai-keys` — grava/limpa a key DESTA conta (body `{provider(gemini|glm|openai),
  apiKey}`; `apiKey` vazio = **limpar**; provider fora do enum => 400 `invalid_provider`). Devolve o
  status mascarado atualizado. So faz sentido com `useGlobalKeys=false` (o front so mostra em escopo conta).
- `GET /v1/calendar/ai-keys/global` — status mascarado das chaves GLOBAIS (**so platform_admin**, senao 403).
- `PUT /v1/calendar/ai-keys/global` — grava/limpa a key GLOBAL (**so platform_admin**; body `{provider,apiKey}`;
  vazio = limpar). Preserva as demais keys do conjunto. Devolve o status mascarado.
- `GET /v1/calendar/ai/models?provider=<gemini|glm|openai>` — **listagem de modelos do provedor (Opcao C)**.
  `RequireAuthWithAccount` (key-adjacent: resolve a chave da conta server-side, tao sensivel quanto os
  secrets). Faz `GET {baseURL}/models` no endpoint CANONICO do provedor (mapa `providerDefaultBaseURL`; a
  Base URL do cliente NAO entra — evita SSRF), filtra os modelos de CHAT e devolve `{models:[...]}`. NAO
  aplica o kill switch (`ai.enabled`) — listar e config. Erros: 400 `invalid_provider` (fora do enum),
  **409 `ai_key_missing`** (provider sem chave gravada), **502 `models_unavailable`** (provedor falhou /
  chave invalida / endpoint sem `/models`). Alimenta o SELECT nao-editavel do painel (fim do texto livre
  em Modelo). A key so vai no header Bearer server-to-server, nunca logada nem devolvida.
- `GET /v1/calendar/client-profile?clientId=<uuid>` — perfil estrategico do cliente. Perfil
  inexistente => objeto vazio com defaults (**200, nao 404** — perfil e opcional). `clientId`
  nao-UUID => 400 `invalid_client`.
- `PUT /v1/calendar/client-profile?clientId=<uuid>` — upsert full-replace do perfil (body =
  `ProfileInput`, contrato C3 sem `clientId`/`updatedAt`); `updated_by` = rotulo do Principal.
- `GET /v1/calendar/client-profiles` — indice lean `{profiles:[{clientId,filled,updatedAt}]}`;
  `filled` = algum campo estavel (fora de `extra`) nao-vazio. Ordenado por `updatedAt desc`.
- `POST /v1/calendar/ai/plan` — cria plano `pending` e dispara o n8n em goroutine (payload C5,
  com a KEY CRUA em `ai.apiKey`). Body `{month:"YYYY-MM", clientIds:["uuid"]}`. Resposta **201
  `{id,status}`**. `month` malformado => 400 `invalid_date`; nenhum `clientId` UUID valido => 400
  `invalid_client`; sem `CALENDAR_AI_WEBHOOK_URL` no env => **503 `ai_not_configured`** (nao cria
  linha). **WAVE 3 (SPEC-B2)**: `ai.enabled=false` => **409 `ai_disabled`** e provider sem key gravada
  => **409 `ai_key_missing`**, ambos SINCRONO ANTES de criar a linha (nao cria plano orfao preso em
  pending). Falha no dispatch => a goroutine marca `status=error` via a mesma transicao do callback.
- `GET /v1/calendar/ai/plans?month=` — lista LEAN `{plans:[{id,month,clientIds,status,provider,model,createdAt}]}` (sem `content`), mais recentes primeiro.
- `GET /v1/calendar/ai/plans/{id}` — plano completo (com `content`), no escopo da account (404 fora).
- `POST /v1/calendar/ai/plans/{id}/applied` — marca `applied` (**so se `done`**; outro estado =>
  409 `plan_conflict`; fora do escopo => 404).
- `DELETE /v1/calendar/ai/plans/{id}` — remove no escopo da account (404 fora).
- `POST /v1/public/calendar-ai/plans/{id}/result` — **callback do n8n, SEM JWT** (fora do gate
  de modulo — o prefixo `/v1/public` nao esta em `moduleGatingRules`). Autentica por header
  `X-Service-Token` (comparacao constant-time com `CALENDAR_AI_SERVICE_TOKEN`). Env ausente =>
  503 `ai_not_configured`; token errado => 403 `invalid_token`; body `{status:"done|error",
  content:{...},error}` (max 2 MiB, `status` fora de done|error => 400 `invalid_status`). So
  transiciona a partir de `pending` (plano ja `done`/`applied` => 409 `plan_conflict`; inexistente => 404).
- `GET /v1/runtime/calendar/context?accountId=<uuid>&clientId=<uuid>&month=YYYY-MM` — **contexto
  compartilhado das IAs (C9, WAVE 2), SEM JWT** (chamada service-to-service; fora do gate — o
  prefixo `/v1/runtime` nao esta em `moduleGatingRules`, junto com o runtime do automation).
  Autentica por `Authorization: Bearer <AUTOMATION_RUNTIME_TOKEN>` (constant-time, `crypto/subtle`).
  `accountId` OBRIGATORIO e validado UUID; `clientId`/`month` opcionais (`month` vazio = mes
  corrente). Resposta 200 `{account{id}, client|null, month, holidays[], monthNotes,
  events[{date,type,title,status,clientId}] (lean, max 100), plans[{id,month,status,provider,model}]
  (lean, max 10)}`. Env ausente => **503 `runtime_not_configured`**; Bearer errado/ausente => **401
  `unauthorized`**; `accountId` ausente/nao-UUID => **400 `invalid_account`**. Isolamento: o
  `accountId` vem do query (sem X-Account-Id), mas TODAS as queries do `BuildAIContext` filtram
  por `account_id` e o nome/perfil de `clientId` forjado de outra conta volta vazio (mesma amarra
  de `loadAccountNames`/`loadProfiles`). O bloco `context` do chat (C7) e este agregado SEM
  `account`, montado pela MESMA `BuildAIContext`.
- `POST /v1/calendar/chat/ask` — **chat de IA com memoria + escopo (C7/D4, WAVE 4)**.
  RequireAuthWithAccount. Body `{question, conversationId?, scopeMode?('client'|'all'), scopeClientId?,
  clientId?(legado, fallback de scopeClientId), month?}` (`question` obrigatoria, trim, max 4000 chars).
  Persiste na conversa (cria se `conversationId` vazio), grava a pergunta, carrega as ultimas N=12 como
  `history`, monta o payload (`ai` = config EFETIVA + KEY CRUA em `ai.apiKey`; `context` = `BuildAIContext`
  no `client` OU `BuildAIContextAll` no `all`, SEM `account`; `history:[{role,content}]`) e faz proxy ao
  `calendar-chat`; grava a resposta e titula a conversa. Resposta 200 `{answer, conversationId, title}`.
  Quando o webhook devolve `eventIds`, o Go cruza os IDs com o contexto autoritativo e persiste
  `calendarItems[]` para os cards; se o `answer` vier com lista textual repetida, compacta para uma
  sintese curta + "lista nos cards" antes de gravar.
  Escopo SEMPRE normalizado server-side (`validateScope`): cliente-side (1 cliente) trava no seu cliente;
  `scopeClientId` fora do visivel => **404 `not_found`** (nao vaza QUAIS clientes existem); `all` so p/ quem
  tem select (agency/multi-cliente). Erros: 400 `invalid_question`; 400 `invalid_date` (`month` malformado);
  404 (`conversationId`/cliente fora do visivel); **409 `ai_disabled`**/`ai_key_missing` (WAVE 3, checados
  ANTES de materializar a conversa — sem conversa orfa); 503 `chat_not_configured`; 502 `upstream_error`;
  504 `upstream_timeout` (>60s). `sessionKey = accountId|userId|conversationId` (id da conversa PERSISTIDA);
  a memoria e o BANCO (`calendar.chat_messages`), o n8n so recebe o `history`.
- `GET /v1/calendar/chat/status?scopeMode=&scopeClientId=` — **preflight sem tokens** usado ao abrir
  o chat e antes de cada envio. RequireAuthWithAccount; valida webhook configurado, escopo efetivo,
  kill switch e chave do provider, depois consulta apenas o `/healthz` da mesma instancia n8n (janela
  curta de 3s, com 1 retry de 250ms para falha transiente). Nao cria conversa/mensagem e nao chama
  modelo. Resposta 200 `{available:true}`; indisponibilidade
  reutiliza os erros do chat (`ai_disabled`, `ai_key_missing`, `chat_not_configured`, upstream/timeout).
- `GET /v1/calendar/chat/conversations` — **lista de conversas (D3)**. RequireAuthWithAccount. Agency ve
  TODAS da conta (com `createdByName` via join); cliente-side so as `created_by = ele`. Resposta lean
  `{conversations:[{id,title,scopeMode,scopeClientId,createdByUserId,createdByName,updatedAt}]}` (updated_at desc).
- `GET /v1/calendar/chat/conversations/{id}` — **conversa + mensagens (D3)**. Dono OU agency (senao 404).
  Resposta `{id,title,scopeMode,scopeClientId,messages:[{id,role,content,createdAt}]}` (ordem cronologica).
- `POST /v1/calendar/chat/conversations` — **cria conversa vazia (D3)**. Body `{scopeMode?,scopeClientId?,title?}`
  (escopo normalizado server-side; cliente fora do visivel => 404). Resposta **201** com o resumo da conversa.
- `DELETE /v1/calendar/chat/conversations/{id}` — **soft-delete (D3)**. Dono OU agency (senao 404). 204.
- `GET /v1/calendar/chat/scope` — **alimenta o SELECT de escopo (D3)**. Resposta `{canSelect, lockedClientId,
  clients:[{id,name}]}` — `canSelect = IsAgency || >1 cliente visivel`; `lockedClientId` = unico cliente do
  cliente-side (select escondido). Clientes REUSAM a lista permission-scoped de `/v1/tenants` (server-side).
- `POST /v1/calendar/chat/transcribe` — **transcricao de voz (C8, WAVE 2)**. RequireAuth +
  accountScope. Multipart campo `file`, max **15 MiB** (`MaxBytesReader` + `ParseMultipartForm`
  com `maxMemory` = teto; NADA gravado em disco). Whitelist de mime: `audio/webm|ogg|mp4|mpeg|wav`
  (parametros do mime ignorados, ex.: `audio/webm;codecs=opus`). **WAVE 3 (SPEC-B2)**: repassa multipart
  (file + campos `provider`+`apiKey`+`model`+`language=pt`) ao webhook `calendar-transcribe`; `provider`/
  `model` vem da config v4 (`transcribeProvider`/`transcribeModel`) e a KEY CRUA do resolver (`openai`
  usa a key `openai`, `gemini` usa `gemini`). Devolve 200 `{text}`. Erros: 400 `invalid_media` (mime
  fora da whitelist / arquivo ausente / multipart malformado); 413 `media_too_large` (>15 MiB); **409
  `ai_disabled`** (kill switch) / **409 `ai_key_missing`** (provider de transcricao sem key); 503
  `transcribe_not_configured` (sem `CALENDAR_TRANSCRIBE_WEBHOOK_URL`); 502/504 (upstream).

### Realtime + optimistic locking (WAVE 2, C11/C12)
- Canal do transporte no modulo `realtime`: `GET /v1/realtime/calendar?scope=account&accountId=`
  (topico `calendar:account:{id}`) + presenca `GET /v1/realtime/presence?scope=calendar`
  (topico `presence:calendar:{id}`, fieldKeys `notes:YYYY-MM` e `event:<id>`). Autorizacao =
  conta ativa + membership + permissao efetiva `calendar.view` (platform_admin bypass) — ver
  `back/internal/modules/realtime/AGENT.md`.
- O modulo `calendar` so PUBLICA (via `Publisher`, `publisher.go`): eventos LEAN de invalidacao
  (o front refaz fetch, nunca patch local): `calendar.event_created|updated|deleted`
  (resourceId=eventId, payload.date; +version no updated), `calendar.note_updated`
  (payload.monthKey), `calendar.day_media_updated` (payload.date), `calendar.config_updated`,
  `calendar.plan_updated` (resourceId=planId, payload.status; publicado no `ApplyPlanResult`) e
  `calendar.client_profile_updated` (WAVE 10, resourceId=clientId; publicado no `PutClientProfile`).
- Optimistic locking (C12): `EventView` ganha `version`; o PUT compara `If-Match` no service
  (guard `and version = $n` no UPDATE; ausencia de linha desambigua 404 x 409 via GET escopado).

### Envs da IA (Fase 6)
Lidos no `Build` do modulo (mesmo padrao do `cardapio`, via `os.Getenv`; injetados no service via
`WithAI`). As CHAVES de API dos provedores NAO ficam aqui nem em `.env`/log — a partir da WAVE 3
(SPEC-B2) o Go resolve a key CRUA dos secrets (`resolveAIKey`) e a injeta no payload (`ai.apiKey`),
tornando o n8n um executor burro (sem credential/$env de IA).
- `CALENDAR_AI_WEBHOOK_URL` — webhook "Calendar Omni" do n8n. Vazio => `POST /ai/plan` responde 503.
- `CALENDAR_AI_SERVICE_TOKEN` — token do callback (header `X-Service-Token`). Vazio => callback 503.
- `CALENDAR_AI_CALLBACK_BASE` — base publica da api que o n8n usa para chamar de volta
  (`{base}/v1/public/calendar-ai/plans/{id}/result`). Vazio => caminho relativo no payload.
- `AUTOMATION_RUNTIME_TOKEN` (WAVE 2, C9) — REUSADO do runtime do automation. Autentica
  `GET /v1/runtime/calendar/context` (Bearer, constant-time). Vazio => a rota responde 503
  `runtime_not_configured` (nao aceita chamada anonima). Lido no `Build` e guardado no `handle`.
- `CALENDAR_CHAT_WEBHOOK_URL` (WAVE 2, C7) — webhook `calendar-chat` do n8n. Vazio => `POST
  /v1/calendar/chat/ask` responde 503 `chat_not_configured`. Injetado no service via `WithChat`.
- `CALENDAR_TRANSCRIBE_WEBHOOK_URL` (WAVE 2, C8) — webhook `calendar-transcribe` do n8n (Whisper).
  Vazio => `POST /v1/calendar/chat/transcribe` responde 503 `transcribe_not_configured`.

As chaves JSON de `EventView` batem 1:1 com o tipo `CalendarEvent` do front
(`web/app/utils/calendar.ts`). `client_id` nao-UUID (ex.: clientes de demonstracao
do mock) e descartado no service (vira sem-cliente) para nao estourar o cast `::uuid`.

## Deploy na VPS — WAVE 3 (IA pelo painel) — CHECKLIST OBRIGATORIO

Ao subir a WAVE 3 pra producao, NAO basta o deploy da imagem. Fazer, em ordem:
1. **Migrations**: garantir que rodam **0188** (`calendar.events.version`), **0189**
   (`calendar.ai_secrets`) e **0190** (`calendar.client_profiles.ai_config`, WAVE 3.1 — `add column
   if not exists`, sem backfill). O migrate roda no boot da api; conferir `migration_up_ok` no log.
2. **Envs no `docker-compose.prod.yml` (servico `api`)** — HOJE SO EXISTEM NO DEV, faltam no prod:
   `CALENDAR_AI_WEBHOOK_URL`, `CALENDAR_AI_SERVICE_TOKEN`, `CALENDAR_AI_CALLBACK_BASE`,
   `CALENDAR_CHAT_WEBHOOK_URL`, `CALENDAR_TRANSCRIBE_WEBHOOK_URL` (apontando pro n8n de prod) +
   valores no `.env.production`. Sem eles => chat/plano/transcricao respondem 503.
3. **Chaves de API dos provedores (Gemini/GLM/OpenAI)**: NAO vao em env/git. Configurar PELO PAINEL
   (aba IA) OU semear as **globais** em `core.platform_settings` chave `calendar_ai_secrets`
   (`{gemini,glm,openai}`) — mesmo shape do dev. A partir da WAVE 3 o n8n NAO usa mais credential/$env
   de IA: o Go injeta a key no payload (`resolveAIKey`).
4. **n8n de prod**: importar e ATIVAR os 3 workflows (`workflow-calendar-chat.json`,
   `workflow-calendar-transcribe.json`, `workflow-calendar-omni.json`) — todos passam a ler
   `body.ai.apiKey`. Reiniciar o n8n apos ativar (o webhook so registra depois do restart).
5. **Proxy**: `POST /chat/transcribe` sobe audio (ate 15 MiB) — o proxy da frente precisa aceitar o
   body. WebSocket `/v1/realtime/calendar` precisa de upgrade liberado (igual aos canais existentes).
6. **Seguranca**: rotas `/ai-keys`, `/ai-config/client` (WAVE 3.1), `/chat/*` e `/ai/*` usam
   `RequireAuthWithAccount` (membership). O restante do modulo (events/notes/config) ainda usa
   `RequireAuth`+header (item pre-existente de plataforma a fechar depois).

## Fases seguintes (nao implementadas) — ver plano canonico §3.6-3.8
- **Midia rica (3c)**: `MediaItem.posterUrl` no back = IMPLEMENTADO (SPEC-B1: struct + validacao
  de prefixo no service, sem ffmpeg). Falta o FRONT: abrir visualizador ao clicar e usar a midia
  da postagem como fundo do dia em grade (SPEC-F1/F2).
- **Perfil estrategico do cliente (Fase 4)**: IMPLEMENTADO (SPEC-B2) — tabela DEDICADA
  `calendar.client_profiles` 1:1 por (account, cliente) na migration 0185 (NAO em core.accounts);
  `GET/PUT /v1/calendar/client-profile?clientId=` + `GET /v1/calendar/client-profiles` (indice).
  Insumo da IA. Falta o FRONT (SPEC-F4).
- **Config v2 (Fase 5, back)**: IMPLEMENTADO (SPEC-B3) — `CalendarConfig` estendido com o
  contrato C2 (weekStartsOn, clientColors, typeColors, whiteLabel, ai) sem migration nova
  (mesma coluna `config jsonb` da 0182); defaults completos + sanitizacao no `PutConfig`.
  Falta o FRONT (SPEC-F3): pagina `/calendario/config` que aplica cores/semana/white-label
  e edita a config da IA. Chaves de API ficam server-side (credentials do n8n).
- **IA de calendario + n8n "Calendar Omni" (Fase 6, back)**: IMPLEMENTADO (SPEC-B4) — migration
  0186 `calendar.ai_plans` + `POST /v1/calendar/ai/plan` (clientIds[]+month) cria linha `pending`
  e dispara o n8n em goroutine (payload C5: perfis/nomes/feriados/nota do mes; `http.Client`
  timeout 15s; falha marca `error`). Callback publico `POST /v1/public/calendar-ai/plans/{id}/result`
  autenticado por `X-Service-Token` (constant-time, `crypto/subtle`); so transiciona de `pending`.
  Lista lean + get completo + `applied` + delete. Provider plugavel (snapshot da config no disparo);
  n8n nunca fala com o Postgres direto (so via callback). Envs em "Envs da IA (Fase 6)". Falta o
  FRONT (SPEC-F5) e o workflow n8n (SPEC-W1).
- **Aprovacao via WhatsApp (Fase 7)** + visao compartilhavel read-only pro cliente.
(Feriados/datas comemorativas: implementado — ver `holidays.go` e `GET /v1/calendar/holidays`.)

## Fachada para Customer Intelligence (2026-07-23)

- `customer_intelligence_context.go` e a fachada publica, somente leitura, do
  owner Calendar para a composition root.
- A leitura exige `account_id + client_account_id` exatos e consulta apenas
  `calendar.client_profiles`; o consumidor nao acessa tabelas de Calendar.
- O retorno e **Business Context** do cliente contratante, nunca fato de uma
  pessoa. O DTO e a observacao resultante nao possuem `subject_id` nem
  `relationship_id`.
- As secoes sao fechadas em `strategy|presence|voice|brief`, com limite por
  campo e limite total de bytes. Nao adicionar secao por prompt ou configuracao
  sem primeiro alterar e testar este contrato owner-owned.
- A fonte `calendar.client_profile` e somente `on_demand`. Ausencia de perfil
  retorna conjunto vazio; falha desta fonte nao pode interromper o chat.
