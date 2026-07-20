# OMNI-F5 — Realtime

**Prioridade:** P0
**Plano canônico:** `docs/omnichannel/PLANO_ATENDIMENTO.md` (§9 F5, §9.1, §10, §11)

> ## LIBERADO PARA IMPLEMENTAÇÃO (2026-07-17, decisão do dono)
>
> A branch `refactor/multi-tenant-complete` fechou e o dono **liberou a implementação em
> 2026-07-17** (decisão **D-D**, canônico §2). O aviso de congelamento que constava aqui não vale mais.

---

## Objetivo

Mensagem recebida no webhook aparece **ao vivo** no inbox, sem refresh e **só na conta certa**.
Nasce o canal `omnichannel:account:{id}` sobre o transporte WS da casa (ticket + hub em memória),
com os 3 eventos do legado replicados **por call-site**. O `useOmnichannelInboxRealtime.ts` deixa
de falar socket.io e passa a falar o WS nativo do Omni.

## Depende de / Bloqueia

| | Fases |
|---|---|
| **Depende de** | **F4** (webhook inbound — sem inbound não há o que publicar); F2 (schema + leitura) |
| **Bloqueia** | **F6** (envio via outbox publica `message.created`/`message.updated`); F7 (ações publicam `conversation.updated`) |
| **Não depende** | F3, F8, F9 — o canal não sabe de outbox, fila nem IA |

---

## Entregas

### Go — transporte (`back/internal/modules/realtime/`)

| # | Item | Arquivo | Molde confirmado no disco |
|---|---|---|---|
| 1 | `omnichannelAccountTopic(id) = "omnichannel:account:" + id` | `model.go` | `calendarAccountTopic` (`model.go:110-113`) |
| 2 | Constantes `EventTypeOmnichannel*` (os 3 literais) | `model.go` | `EventTypeCalendar*` (`model.go:38-45`) |
| 3 | `HandleOmnichannelSocket` + `resolveOmnichannelAccount` + `authorizeOmnichannelAccount` | **`service_omnichannel.go` (novo)** | `service_calendar.go:60/82/101` — copiar a estrutura |
| 4 | `PublishOmnichannelEvent` (implementa `omnichannel.Publisher`) | `service_omnichannel.go` | `PublishCalendarEvent` (`service_calendar.go:21`) |
| 5 | `mux.HandleFunc("GET /v1/realtime/omnichannel", service.HandleOmnichannelSocket)` | `http.go` | linha `:16` (calendar); `RegisterRoutes` em `:9` |
| 6 | Atualizar `back/internal/modules/realtime/AGENT.md` | — | **existe e é obrigatório** (contrato + seção do canal) |

### Go — módulo (`back/internal/modules/omnichannel/`)

| # | Item | Molde |
|---|---|---|
| 7 | `publisher.go`: `Publisher` interface + `RealtimeEvent` + `noopPublisher` + `WithPublisher(p) Option` | `calendar/publisher.go` (`:42`, `:53`) — direção **realtime → omnichannel**, sem ciclo |
| 8 | `module.go`: `WithPublisher(m.publisher)` no `Build` | `calendar/module.go:114` |
| 9 | `app.go`: `omnichannel.New(..., omnichannel.WithPublisher(realtimeService))` | `app.go:398` (calendar); `realtime.RegisterRoutes` já em `app.go:305` |
| 10 | Sanitizar mídia **no call-site**, antes de publicar (ver Contratos) | — |

### Front (`web/app/composables/omnichannel/useOmnichannelInboxRealtime.ts`)

11. **Reescrever** sobre `useRealtimeSocket` (`web/layers/tasks/composables/useRealtimeSocket.ts`),
    trocando `socket.io-client` por WS nativo + ticket (`buildRealtimeSocketURL`,
    `web/app/composables/useRealtimeConnection.ts:38`).
12. **Preservar intactos** (fonte: `web-reference/app/composables/omnichannel/useOmnichannelInboxRealtime.ts`):
    os 3 handlers, os nomes de evento, a superfície de retorno (`:545-551`) e os fallbacks de
    polling (`:55-60` — status 45s, stale 20s/delay 4s, heartbeat 5min, cooldown 5s, visibility 5min).
13. **Adapter de envelope** + **mapa de estado de conexão** (itens NOVOS — ver Contratos).

---

## Contratos

### Rota

```
GET /v1/realtime/omnichannel?scope=account&accountId={uuid}&ticket={t}
```

Padrão ticket **inalterado**: `POST /v1/ws/ticket` (`http.go:10`, `RequireAuth`) → ticket efêmero,
30s, single-use (`LoadAndDelete`). Reconexão pede ticket novo. Canal: `omnichannel:account:{id}`.
Buffer `tasksSubscriptionBuffer` + `readPumpWithRateLimit` (30 msg/s por conexão, close **1008**),
via `serveSubscriptionSocket` — igual ao calendar (`service_calendar.go:72-76`).
Primeiro evento: `realtime.connected` (`EventTypeConnected`, `model.go:6`).

### Os 3 eventos — shapes **por call-site**, sem unificar

`message.created` · `message.updated` · `conversation.updated`.

**Os shapes campo a campo estão em `SPECS_PORT_OMNICHANNEL.md` F4 item 2** (3 de `message.updated`,
2 de `conversation.updated`, subconjunto do webhook) e em `PLANO_PORT_OMNICHANNEL.md` §8 item 3.
**Não duplicados aqui** — lá é a fonte.

> **Por que unificar quebra o front (evidência, não opinião):** o handler de `message.updated`
> **discrimina pela forma do payload** — `isFullMessagePayload` exige `conversationId` não-vazio
> **+** `direction` string **+** `createdAt` string
> (`web-reference/app/composables/omnichannel/useOmnichannelInboxRealtime.ts:448-453`) e ramifica.
> Mandar sempre o Message completo faz o ramo "mínimo" — o que atualiza
> `conversations[].lastMessage.status` (`:496-514`) — **nunca rodar**. O shape É o contrato.

### Envelope — a diferença que o port não cobre

O legado (socket.io) emitia o objeto **cru**: `socket.on("message.created", (payload: Message))`.
O WS da casa manda o struct `Event` (`model.go:48-70`) serializado:

```jsonc
{ "type": "message.created", "accountId": "…", "resourceId": "<messageId>",
  "payload": { /* o shape do call-site, camelCase */ }, "savedAt": "…" }
```

Logo: `useRealtimeSocket` chama `onMessage(envelope)`; o composable **despacha por
`envelope.type`** e entrega **`envelope.payload`** ao handler preservado. Envelope sem `type`
conhecido ou sem `payload` → ignorar (não derrubar a tela).

> **Divergência consciente do canal calendar (registrar, princípio 4):** o `realtime/AGENT.md`
> manda payload **lean de invalidação** (o front refaz fetch, nunca patch local). O omnichannel
> carrega o **payload completo** em `Event.Payload` porque o front é **verbatim (D-B)** e faz
> patch local (`mergeMessages`, `upsertConversation`). É exceção deliberada à regra da casa,
> **documentar na seção nova do `realtime/AGENT.md`** — alvo de reavaliação na F14.

### Publisher (Go)

```go
// back/internal/modules/omnichannel/publisher.go — molde: calendar/publisher.go
type RealtimeEvent struct {
    Type       string         // message.created | message.updated | conversation.updated
    AccountID  string         // SEMPRE do Principal — nunca do body
    ResourceID string         // messageId | conversationId
    Payload    map[string]any // o shape do call-site, ja sanitizado
}

type Publisher interface {
    PublishOmnichannelEvent(ctx context.Context, evt RealtimeEvent)
}
```

`noopPublisher` = default (canal desligado; testes de service não precisam do realtime).
`PublishOmnichannelEvent` no realtime: `hub == nil` ou `Type`/`AccountID` vazios → **no-op**
(espelho de `service_calendar.go:21-30`).

### Sanitização de mídia — obrigatória

`mediaUrl` que comece com `data:` → **`null`** no payload do realtime. **Nunca base64 no WS.**
O front busca em `GET .../messages/{mid}/media` (F6).

- **Onde:** no **call-site do módulo `omnichannel`**, antes de montar o `RealtimeEvent` — o
  `realtime` é transporte genérico e não conhece `mediaUrl`.
- **Cinto e suspensório:** o publisher também zera `payload["mediaUrl"]` quando `data:` — um
  call-site novo que esqueça não vaza megabytes no socket.

### Autorização (antes do upgrade)

| Situação | Resultado |
|---|---|
| Sem ticket/principal | 401 (`authenticateRealtimeRequest`) |
| `accountId` vazio | 400 `validation_error` |
| Usuário comum pedindo **outra** conta | `resolveRealtimeAccountID` (`service_tasks.go:482-500`) resolve a conta do Principal |
| Conta inexistente/inativa | **404** |
| **Não é membro da conta** | **404** — escopo (**não** 403) |
| Membro **sem** `omnichannel.conversations.view` | **403** — permissão gateia feature |
| `platform_admin` | bypass após a conta existir |

Permissão: reusar `hasAnyCoreTaskPermission` (query genérica por chave) com
`omnichannel.conversations.view`, + o fallback `principal.PermissionsResolved` — igual
`authorizeCalendarAccount` (`service_calendar.go:140-152`).

> **Atenção — divergência real com o precedente:** `authorizeCalendarAccount` devolve
> **403** (`errRealtimeForbidden`) para não-membro. O canônico (§10) manda **404 para fora de
> escopo, nunca 403** (enumeration). Esta spec segue o **canônico**: no-membro → `errRealtimeNotFound`.
> Copiar o calendar cegamente reintroduz o 403. `writeRealtimeAccessError`
> (`service_tasks.go:882-895`) já traduz `errRealtimeNotFound` → 404.

### Estados de conexão — mapa obrigatório

O legado lia o motivo do erro pela **string** do `connect_error` do socket.io:
`"ModuleAccessDenied"` (`:320`) e `"Unauthorized"` (`:328`). **Isso não existe no WS nativo:**
`writeRealtimeAccessError` responde o HTTP **antes** do upgrade, então o browser só vê
**close 1006, sem status legível**. Mapa a implementar:

| Sinal observável | `RealtimeConnectionState` (`:9`) | Ação |
|---|---|---|
| `POST /v1/ws/ticket` falha (401/403) → hook `logTicketError` (`useRealtimeSocket.ts:91`) | `auth_error` | inicia stale polling |
| Gate de módulo: REST `/v1/omnichannel/*` → 403 `module_disabled` | `module_denied` | fecha socket + stale polling |
| `status` = `connected` | `connected` | para stale polling, inicia heartbeat, sync forçado |
| `status` = `connecting` | `connecting` | — |
| `status` = `idle`/`reconnecting`/`error` | `disconnected` | inicia stale polling |

- `useRealtimeSocket` expõe `'idle' \| 'connecting' \| 'connected' \| 'reconnecting' \| 'error'`
  (`:17`) — **não** tem `module_denied`/`auth_error`. Manter o union do legado (o restante do
  front lê esse estado) e derivar por este mapa.
- **`module_denied` não vem do WS.** `/v1/realtime/*` **não é gateado por módulo** — confirmado:
  `app.go:516` lista `realtime` entre as rotas fora do `moduleGatingRules()`. O sinal vem da
  camada REST. Não inventar um gate no handler para "sinalizar" o front.
- Todos os ramos degradados **iniciam o stale polling** — é o que preserva o inbox vivo hoje.

---

## Armadilhas / o que NÃO fazer

1. **A armadilha cara — `accountId` pela MESMA fonte do REST.** Nunca `auth.activeTenantId` direto:
   para `platform_admin` ele cai no seed `aaaaaaaa-…`, **diverge da conta do switcher**, o
   handshake **nunca vira 101** e o socket entra em **close 1006 em loop**. Caso real, com
   autópsia: `web/layers/tasks/AGENT.md:315-331`. Fonte correta:
   `accountStore.activeAccountId || auth.activeTenantId || auth.tenantContext?.[0]?.id`.
   `useRealtimeSocket` **já protege** — `resolveRealtimeAccountId` (`:35-51`) inclui
   `accountStore.activeAccountId` no fallback. **Não anular essa defesa** passando um
   `accountId` explícito resolvido da fonte errada: o prop explícito **vence**.
   Ver também `project_account_source_divergence`.
2. **Não unificar os shapes.** Ver a evidência acima. "Simplificar" aqui é regressão.
3. **Não trafegar base64 no WS.** Data URL → `null`, sempre.
4. **Não mexer nos canais existentes.** `service_omnichannel.go` é arquivo **novo** — o calendar
   nasceu assim, sem tocar em tasks/presence/operations.
5. **Não trocar o polling por "o WS resolve".** Os fallbacks (45s/20s/5min/5s) seguram o inbox
   quando o socket cai — princípio 3: features coexistem, o WS **soma** ao polling.
6. **Não voltar `token`/`access_token` na query string.** Ticket, sempre (`realtime/AGENT.md:48-50`).
7. **Não filtrar `instanceId` no servidor.** O recorte por instância é do front
   (`conversationMatchesSelectedInstance`, `:62-72`); o canal é **por conta**.
8. **Não publicar dentro da transação** do webhook/outbox: persiste → commita → publica. Realtime
   é entrega, não fila durável (evento perdido ≠ dado perdido; o polling cobre).
9. **`correlationId` com prefixo `sync-history:`** é backfill e o front **ignora de propósito**
   (`:371`, `:432`). Manter o prefixo ao publicar backfill — senão o histórico "chove" na tela.
10. **Teto de 450 linhas vale** — é código novo (Go e o composable reescrito).

## Segurança

- **`account_id` SEMPRE do Principal**, nunca do body/query confiado: `resolveRealtimeAccountID`
  (`service_tasks.go:482-500`) — `platform_admin` pode informar `accountId`; usuário comum só
  assina a própria conta.
- **Defesa em profundidade:** o `authorize*` valida conta ativa **+** membership **+** permissão
  efetiva **contra o banco** (`core.accounts` / `core.account_users`), não contra o JWT.
- **Fora de escopo → 404, nunca 403** (enumeration). Permissão faltando → 403. Ver a tabela e a
  nota de divergência acima.
- **Conta diferente nunca recebe evento:** o tópico é `omnichannel:account:{id}`; o publish usa
  o `AccountID` do evento — nunca o `accountId` da query de quem conectou.
- **Payload bruto nunca em log** (canônico §10) — nem no log de close/erro do socket.
- Origin: o WS respeita a mesma política de `Origin` do HTTP (`cfg.CORSAllowedOrigins`).

## Verificável

Um humano prova no browser/banco, sem ler código:

1. **Ao vivo:** aba logada em `/omnichannel`; mandar mensagem do celular para o número conectado
   (F4); a mensagem **aparece sozinha**, sem refresh. Rede → WS → `/v1/realtime/omnichannel`
   em **101 Switching Protocols**, com frames chegando.
2. **Isolamento (o teste que importa):** duas abas, **contas diferentes** (switcher). A mensagem
   aparece **só na aba da conta certa**. Na outra, **nada**.
3. **`platform_admin` (a regressão cara):** logar como `platform_admin`, trocar a conta no
   switcher, abrir `/omnichannel` → handshake **101** e eventos da conta **selecionada**.
   Se aparecer close **1006 em loop**, o `accountId` voltou para a fonte errada.
4. **Fallback:** derrubar o WS (DevTools offline / `docker compose restart api`) → estado vai a
   `disconnected`, **o polling assume** (conversas atualizam, ~20s) e ao voltar reconecta + sync.
5. **Escopo:** `wscat`/DevTools em `/v1/realtime/omnichannel?scope=account&accountId=<conta alheia>`
   com ticket de usuário comum → **404** (não 403, não 101).
6. **Sem base64:** enviar imagem; inspecionar os frames do WS → `mediaUrl` **`null`** ou URL, nunca
   `data:image/...`. Frame na casa dos bytes, não dos megabytes.
7. **Shapes por call-site:** com o inbox aberto, mudar o status de uma mensagem → o
   `message.updated` mínimo chega e a prévia na sidebar atualiza (prova que o ramo mínimo roda).

## Notas de Deploy

| # | Item | Detalhe |
|---|---|---|
| 1 | **Migration** | **Nenhuma.** O hub é em memória; F5 não cria tabela |
| 2 | **Env var** | **Nenhuma nova** |
| 3 | **Build da API** | Mudou `back/` → `docker compose up -d --build api`. **`--no-cache` não é necessário** (sem `.sql` novo para o `embed.FS` re-embutir) |
| 4 | **Build do web** | Composable reescrito → rebuild do web (dev: `npm run dev` = `up --build --watch`) |
| 5 | **Caddy / proxy** | Rota já coberta: `/v1/realtime/*` existe (tasks/calendar). Sem mudança |

**Ordem:** build api → build web → smoke dos 7 itens do Verificável.

> Middleware HTTP novo no caminho do WS precisa preservar `http.Hijacker` e `http.Flusher`,
> senão o upgrade quebra (`realtime/AGENT.md:86`). F5 não adiciona middleware — se alguém
> adicionar, é aqui que quebra.
