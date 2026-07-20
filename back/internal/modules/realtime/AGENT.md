# AGENT

## Escopo

Estas instrucoes valem para `back/internal/modules/realtime`.

## Responsabilidade do modulo

O modulo `realtime` cuida do transporte em tempo real da plataforma.

Hoje ele deve responder por:

- conexoes WebSocket autenticadas
- assinatura por loja
- assinatura administrativa por tenant
- entrega de eventos leves para a UI revalidar estado
- isolamento do transporte realtime em relacao aos modulos de negocio

Ele nao deve cuidar de:

- auth como fonte de verdade
- regra da fila
- montagem de snapshot operacional
- persistencia do estado

## Estado atual (Fase T2 concluída em 2026-05-14)

- Canais legados mantidos: `/v1/realtime/operations` e `/v1/realtime/context`.
- Canais Tasks adicionados: `/v1/realtime/tasks`, `/v1/realtime/presence` e `/v1/realtime/notifications`.
- `Service` implementa `tasks.Publisher`; o app injeta `realtimeService` em `tasks.New(realtimeService)`.
- `PresenceStore` em memória entrega snapshot, joined, left, field_locked e field_unlocked com TTL 30s.
- Novos canais têm rate limit de entrada de 30 mensagens/s por conexão e buffer 16.
- Validação executada: `go test ./...` em `back/`.

## Contrato atual

- `POST /v1/ws/ticket` autenticado por `Authorization: Bearer ...`
- `GET /v1/realtime/operations?storeId=...&ticket=...`
- `GET /v1/realtime/context?tenantId=...&ticket=...`
- `GET /v1/realtime/tasks?scope=...&accountId=...&ticket=...`
- `GET /v1/realtime/presence?scope=...&accountId=...&ticket=...` (scope in `board|task|calendar`)
- `GET /v1/realtime/notifications?userId=...&accountId=...&ticket=...`
- `GET /v1/realtime/calendar?scope=account&accountId=...&ticket=...` (Wave 2, C11)
- `GET /v1/realtime/omnichannel?scope=account&accountId=...&ticket=...` (F5)

O ticket WS e efemero, fica apenas em memoria do processo, expira em 30s e e consumido com `LoadAndDelete`
antes do upgrade WebSocket. Ele e single-use: reconexao precisa pedir um ticket novo.

Fallback temporario por 1 release: `access_token`, `token` e header `Authorization: Bearer ...` ainda sao
aceitos nos handlers WS, mas uso de token em query string emite log de deprecacao. O frontend oficial nao deve
usar fallback; se `POST /v1/ws/ticket` falhar, deve logar o erro e nao abrir WebSocket.

Eventos atuais:

- `realtime.connected`
- `operation.updated`
- `context.updated`

Shape atual do evento:

- `type`
- `tenantId`, quando for evento de contexto
- `storeId`, quando for evento operacional
- `action`
- `resource`
- `resourceId`
- `personId`
- `savedAt`

## Regras de arquitetura

- o payload do evento deve ser leve e orientado a invalidacao, nao um snapshot inteiro
- a leitura autoritativa continua em `GET /v1/operations/snapshot`
- `context.updated` serve para invalidacao leve de:
  - lojas acessiveis
  - usuarios e acessos
  - settings operacionais (agora tenant-wide; vale para todas as lojas do tenant)
  - header/contexto autenticado
- para settings, que agora e tenant-wide, o contrato publicado e apenas:
  - `context.updated` com `resource = settings`, `action = updated` e `resourceId = {tenantId}`
- o frontend pode revalidar snapshot apos receber um evento
- o frontend pode revalidar `GET /v1/me/context` e leituras administrativas apos `context.updated`
- o frontend atual revalida `GET /v1/settings?storeId=...` tanto no canal de contexto quanto no canal operacional quando a loja afetada coincide com a loja ativa
- a implementacao atual usa hub em memoria por processo para manter a base simples
- cada assinatura usa buffer pequeno e descarta evento antigo quando o consumidor fica para tras; realtime e invalidacao, nao fila duravel
- quando houver escala horizontal, este modulo deve trocar o hub local por broker externo sem quebrar o contrato WebSocket
- middlewares HTTP que embrulham `http.ResponseWriter` precisam preservar `http.Hijacker` e `http.Flusher`, senao o upgrade do websocket quebra
- conexoes enviam `ping` periodico e esperam `pong`; mensagens recebidas do cliente sao lidas apenas para manter a conexao viva

## Regras de seguranca

- toda conexao precisa resolver um principal autenticado antes do upgrade
- conexoes WebSocket oficiais usam `ticket` efemero em vez de JWT longo na query string
- tickets WebSocket nao usam Redis, persistencia nem multiplos usos; a implementacao atual e mapa em memoria por processo
- toda conexao precisa validar acesso do usuario a `store_id`
- a conexao operacional exige permissao efetiva `workspace.operacao.view` quando o principal ja vem com matriz resolvida; sem matriz resolvida, cai no fallback por papel operacional
- a conexao de contexto resolve o tenant pelo principal; `platform_admin` pode informar `tenantId`, e usuarios tenant-scoped nao podem assinar outro tenant
- o modulo deve respeitar a mesma politica de `Origin` configurada para o HTTP

## Publicadores atuais

- `operations` publica `operation.updated` para comandos da fila/atendimento
- `settings` publica somente `context.updated` quando a configuracao do tenant muda (modal, operation settings, produtos e catalogos ordenaveis); como settings agora e tenant-wide, o canal de contexto ja entrega a invalidacao a todos os atendentes do tenant e o evento `operation.updated` por loja deixou de ser usado por settings
- `stores`, `users` e `auth` publicam `context.updated` para reidratar contexto administrativo e autenticado

## Canais Tasks / Presence / Notifications (Fase T2)

Novos canais a adicionar em `service_tasks.go` sem quebrar os canais existentes:

```
tasks:account:{accountId}        boards do account — task.created/updated/deleted
tasks:board:{boardId}            mudancas dentro do board — task.moved, column.*
tasks:task:{taskId}              mudancas finas — comments, tracking, relations, shares
presence:board:{boardId}         avatares no board (snapshot, joined, left)
presence:task:{taskId}           avatares + field locks no detalhe
notifications:user:{userId}      canal pessoal — notification.created, notification.read
```

### Autorização dos novos canais

Antes do upgrade WS:
1. `AuthenticateToken` (ja existente)
2. Resolver `accountId` ativo do principal
3. Para `tasks:board:{boardId}` → confirmar `account_id` bate OU existe share ativa
4. Verificar perm `tasks.tasks.view` ou `tasks.client_view`

### Eventos de tasks

```
task.created, task.updated, task.moved, task.deleted, task.assigned
task.comment_added, task.relation_added, task.relation_removed
task.share_added, task.share_revoked
task.time_started, task.time_paused, task.time_resumed, task.time_stopped
board.column_added, board.column_updated, board.column_deleted
```

### Eventos de presence

```
presence.snapshot       lista completa ao entrar no canal
presence.user_joined    { userId, displayName, avatarPath }
presence.user_left      { userId }
presence.field_locked   { userId, fieldKey, lockId }
presence.field_draft    { userId, fieldKey, draftValue }
presence.field_unlocked { userId, fieldKey }
```

Presence usa `PresenceStore` em memoria com TTL 30s. Heartbeat do cliente a cada 15s.
Ticker server-side varre entries expiradas e publica `presence.user_left`.

`presence.field_draft` so e aceito para o user que ja detem o lock daquele `fieldKey`; o store guarda um unico `draftValue` efemero por participante/canal para popular snapshots e previews ao vivo sem virar fonte de verdade de persistencia.

**DisplayName no payload de presence:** usa `principal.Nick` (coluna `nick` em `core.users`, opcional)
quando preenchido; cai para `principal.DisplayName` e por ultimo para `principal.Email`. Front exibe
o que vier no payload — sem regra de fallback adicional no client. Nick e' a identidade curta
preferida em mascaras de presence/selects (T7.1).

**Lock exclusivo por fieldKey (T7.2):** `LockField` valida se outro usuario ja esta no mesmo
`fieldKey` dentro do TTL. Quando ocupado, vira no-op (nao publica `field_locked`, nao altera
estado). Front-end ja desabilita o input via `:disabled` quando `isPresenceFieldLocked`; o guard
server-side e' defesa em camada para evitar dois clientes verem o outro editando o mesmo campo
simultaneamente (problema mascarado quando display_names sao iguais).

### Eventos de notifications

```
notification.created    payload completo (economiza round-trip REST)
notification.read       { notificationId }
```

### Rate limit dos novos canais

- 30 events/seg por conexao (entrada do cliente; presence heartbeat conta)
- Buffer de subscription = 16 (presence e mais barulhento)
- Drop oldest quando cheio (comportamento atual do Hub)
- Close code 1008 quando rate-limit excedido

### Interface Publisher (injetada em tasks/module.go)

```go
// back/internal/modules/tasks/publisher.go
type Publisher interface {
    PublishTaskEvent(ctx context.Context, evt TaskEvent)
    PublishBoardEvent(ctx context.Context, evt BoardEvent)
    PublishPresenceEvent(ctx context.Context, evt PresenceEvent)
}
```

`NoopPublisher` retorna nil em tudo (usado em testes de service).

## Canal Calendar (Wave 2, contrato C11)

`service_calendar.go` adiciona o canal de eventos do calendario sem mexer nos canais existentes:

```
calendar:account:{accountId}     eventos de invalidacao do calendario da conta
presence:calendar:{accountId}    avatares/presenca no calendario (fieldKeys de C11)
```

- `GET /v1/realtime/calendar?scope=account&accountId=...` → `HandleCalendarSocket` reusa
  `serveSubscriptionSocket`. Presenca em `GET /v1/realtime/presence?scope=calendar` — o ponto de
  extensao e `resolvePresenceSubscription` (novo case `calendar` + prefixo `presence:calendar:`);
  `HandlePresenceSocket` so delega. FieldKeys: `notes:YYYY-MM` (notas do mes) e `event:<id>`
  (form de edicao de evento) — o `readPresencePump` repassa o fieldKey livre.
- **Autorizacao** (`authorizeCalendarAccount`, copia adaptada de `authorizeTasksAccount`): conta
  ativa + membership + permissao efetiva `calendar.view`; `platform_admin` bypass. Socket sem
  `calendar.view` fecha antes do handshake (como em tasks). Conta diferente nunca recebe eventos.
- **Eventos publicados** (INVALIDACAO, payload lean — o front refaz fetch, nunca patch local):

```
calendar.event_created | calendar.event_updated | calendar.event_deleted   (resourceId=eventId, payload.date; version no updated)
calendar.note_updated       (payload.monthKey)
calendar.day_media_updated  (payload.date)
calendar.config_updated
calendar.plan_updated       (resourceId=planId, payload.status)
```

- **Publisher (direcao realtime -> calendar)**: o modulo `calendar` define a interface
  `calendar.Publisher` + o tipo `calendar.RealtimeEvent` (`calendar/publisher.go`); o `realtime`
  a implementa em `PublishCalendarEvent`, mapeando `ResourceID`/`Version` para o `Event` e
  jogando `date`/`monthKey`/`status` no `Payload map[string]any` (sem inchar o struct). O app
  injeta `calendar.WithPublisher(realtimeService)`. Constantes espelhadas em `model.go`
  (`EventTypeCalendar*`) e em `calendar/publisher.go` (privadas) — os dois lados concordam.

## Canal Omnichannel (F5)

`service_omnichannel.go` adiciona o canal do atendimento WhatsApp sem mexer nos canais existentes:

```
omnichannel:account:{accountId}   eventos do inbox da conta (message.*/conversation.*)
```

- `GET /v1/realtime/omnichannel?scope=account&accountId=...` → `HandleOmnichannelSocket` reusa
  `serveSubscriptionSocket` (buffer 16, rate limit 30 msg/s, close 1008). Ticket efemero como
  todo canal. Rota FORA do `moduleGatingRules()` (app.go) — `/v1/realtime/*` nunca e gateado por
  modulo; o sinal de `module_disabled` vem da camada REST, nao do WS.
- **Autorizacao** (`authorizeOmnichannelAccount`): conta ativa + membership + permissao efetiva
  `omnichannel.conversations.view`; `platform_admin` bypass apos a conta existir.
  **DIVERGENCIA DELIBERADA do calendar** (canonico §10, enumeration): NAO membro → **404**
  (`errRealtimeNotFound`), NUNCA 403. `authorizeCalendarAccount` devolve 403 para nao-membro;
  copiar cego reintroduz o vazamento. Permissao FALTANDO (membro sem a key) → **403**
  (`errRealtimeForbidden`) — permissao gateia feature. Conta diferente nunca recebe eventos.
- **Eventos publicados — exatamente 3, com o payload COMPLETO por CALL-SITE (nao unificar):**

```
message.created         resourceId=messageId  (webhook: subconjunto do Message; envio HTTP: Message completo — F6)
message.updated         resourceId=messageId  (3 shapes: worker minimo / webhook subconjunto / rehidratacao completa — F6)
conversation.updated    resourceId=conversationId  (webhook: sem instanceName; status/contacts: com — F6/F7)
```

  Na **F5 o unico produtor e o webhook inbound**, que publica **`message.created`** (id interno +
  conversationId + direction/messageType/content/status/createdAt + mediaUrl sanitizada). O front
  `message.created` nao ramifica por shape: com conversationId ele faz patch local na thread aberta
  e, para conversa nova nao-cacheada, dispara refresh REST do sidebar. `message.updated` /
  `conversation.updated` do webhook ficam para a F6 (onde a view completa da conversa ja e lida) —
  evita empurrar um card parcial (regressao do `upsertConversation`, que e merge raso).

- **DIVERGENCIA CONSCIENTE do padrao da casa (principio 4):** os demais canais mandam payload
  **lean de invalidacao** (o front refaz fetch). O omnichannel carrega o **payload completo** em
  `Event.Payload` porque o front e **verbatim (D-B)** e faz **patch local** (`mergeMessages`,
  `upsertConversation`). Excecao deliberada, alvo de reavaliacao na F14.

- **Sanitizacao de midia (obrigatoria):** `mediaUrl` que comeca com `data:` vira `null` no WS —
  NUNCA base64 no socket. Feito no **call-site** (`omnichannel/realtime.go`,
  `sanitizeMediaURLForRealtime`) e **repetido** no publisher (`PublishOmnichannelEvent` zera
  `payload["mediaUrl"]` data:) como cinto e suspensorio. O front busca a midia por
  `GET /v1/omnichannel/conversations/{cid}/messages/{mid}/media` (F6).

- **Publisher (direcao realtime → omnichannel):** o modulo `omnichannel` define a interface
  `omnichannel.Publisher` + o tipo `omnichannel.RealtimeEvent` (`omnichannel/publisher.go`); o
  `realtime` a implementa em `PublishOmnichannelEvent`. O app injeta
  `omnichannel.WithPublisher(realtimeService)`. Constantes espelhadas em `model.go`
  (`EventTypeOmnichannel*`) e em `omnichannel/publisher.go` (`RealtimeEvent*`) — os dois lados
  concordam. `noopPublisher` = default (canal desligado; testes de service dispensam o realtime).
  **Nao publicar dentro da transacao** do webhook: persiste → commita → publica.

## Evolucao esperada

1. modulo notifications persistente usando `notifications:user:{userId}` (Fase T3)
2. broker externo para multiplas replicas
3. resume/replay idempotente
4. observabilidade e metricas de conexao
