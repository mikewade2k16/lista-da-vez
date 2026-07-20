# Specs — Port do módulo Omnichannel

> ## LEIA ANTES — numeração antiga, canônico é outro (2026-07-16)
>
> - **Canônico do módulo:** [`PLANO_ATENDIMENTO.md`](PLANO_ATENDIMENTO.md). O
>   [`PLANO_PORT_OMNICHANNEL.md`](PLANO_PORT_OMNICHANNEL.md) — antes citado aqui como plano
>   canônico — foi rebaixado a **anexo técnico do front**.
> - **Estas specs continuam válidas** como o **contrato verbatim do FRONT**: são a razão de
>   este arquivo existir e **não foram reescritas**. As specs por fase da fusão
>   (`docs/omnichannel/specs/OMNI-F*.md`) **referenciam** este arquivo em vez de copiá-lo —
>   duplicar contrato é criar duas verdades (princípio 1).
> - **A numeração F0..F9 daqui é a ANTIGA (do port).** A fusão usa **F0..F14**. O **mapa de
>   renumeração** está em **[`PLANO_ATENDIMENTO.md`](PLANO_ATENDIMENTO.md) §9.1**. Resumo dos
>   desvios que mais confundem:
>
> | Aqui (port) | Fusão | Atenção |
> |---|---|---|
> | F1 Front verbatim | **F1** | Inalterada — vale inteira |
> | F2 Schema + leitura | **F2** | Colunas de estado/fila/provider **nascem na migration** (canônico §7.2). Migrations a partir de **0200** |
> | F3 Evolution + inbound | **F4** | Vira `ChannelProvider` + adapters (`mock` + `evolution`). As **proteções do webhook** aqui valem; o provider deixa de ser cravado |
> | F4 Realtime | **F5** | Inalterada — os 3 eventos e os shapes por call-site valem |
> | F5 Envio + mídia | **F6** | O outbox/worker sai daqui e vira **`platform/jobs`** (canônico §8, **F3 nova**). Retry classificado + monitor com filtro de conta seguem valendo |
> | F6 Ações | **F7** | Passa pela **máquina de estados** (F8 nova) — não escrever `status` na mão |
> | F7 Stickers/GIF/avatar | **F12** | Inalterada |
> | **F8 IA no n8n** | **F9** | **SUPERADA** — a IA vai para o **Go** (canônico §2, **D-C**). n8n sai do caminho crítico |
> | F9 Refactor | **F14** | Renumerada |
>
> - **F0 daqui está superada:** D1 foi decidida como **multi-provider** (canônico §2, **D-A**);
>   D2 mantida; **D3 saiu de escopo** (o módulo é independente e não toca `automation.*`);
>   **D4 DECIDIDA: fora** — decisão do dono (**D-F**, 2026-07-17): `OmnichannelAuditModule.vue`
>   + `useOmnichannelAudit.ts` **não são portados**. Nenhuma segue aberta.
>
> > **LIBERADO PARA IMPLEMENTAÇÃO — 2026-07-17 (decisão do dono).** A branch
> > `refactor/multi-tenant-complete` fechou e o dono liberou a implementação (**D-D**). O aviso
> > de congelamento que constava aqui **não vale mais**. Estas specs seguem sendo *anexo* — a
> > autorização e a ordem de execução vêm do canônico e das specs `specs/OMNI-F*.md`.

Specs executáveis por fase — **contrato verbatim do front**.

Cada spec é autocontida: dá para entregar a um agente sem contexto prévio. Ler
`principios-engenharia` antes de qualquer uma.

**Convenções que valem para todas:**
- Fonte do front: `web-reference/app/**/omnichannel/**` (idêntico a `whats-test/apps/painel-web`).
- Fonte do contrato: `whats-test/apps/atendimento-online-api` (Fastify + Prisma + Evolution).
- `account_id` **sempre** do Principal, nunca do body. Repositório filtra por conta também.
- Fora de escopo → **404**, nunca 403.
- Mudou `back/` → `docker compose up -d --build api`. Migration nova → `build --no-cache api`.
- Não rodar git. Não commitar. Devolver comandos ao usuário.

---

## F0 — Decisões + fundação

**Objetivo:** destravar o resto. Nenhuma linha de código de produto.

**Entregas:**
1. Registrar as decisões D1..D4 do plano (§6) com a escolha do usuário e a data.
2. `docs/LEGADO.md`: adicionar os 4 itens do §11 do plano.
3. Roadmap: `phases-part7.ts` (grupo `omnichannel-port`, fases F1..F9 `pending`) +
   `groups.ts` + `modules.ts` (omnichannel `pending` → `in_progress`, descrição/escopo reais).
4. Se D1 = Evolution: spec de infra do container (compose, profile, volumes, Caddy).

**Verificável:** roadmap mostra as fases; `LEGADO.md` lista os adaptadores.

---

## F1 — Front verbatim + costura

**Objetivo:** `/omnichannel` abre e é visualmente o inbox do legado. Nenhum dado carrega.

### F1.1 — Copiar os 71 verbatim (73 − 2 pelo **D-F**)

Copiar **byte a byte**, sem reformatar, sem rodar Prettier/ESLint --fix:

| De (`web-reference/app/`) | Para (`web/app/`) |
|---|---|
| `composables/omnichannel/*` (50) | `composables/omnichannel/*` |
| `components/omnichannel/**` (23) | `components/omnichannel/**` |
| `pages/admin/omnichannel/inbox.vue` | `pages/omnichannel/index.vue` |

- **Não** copiar os 4 redirects (`index/operacao/auditoria/docs.vue`) — apontam para
  rotas do legado que não existem aqui.
- **Não** copiar `OmnichannelAuditModule.vue` + `useOmnichannelAudit.ts` — **D4 DECIDIDA: fora**
  (**D-F**, 2026-07-17, decisão do dono): código inalcançável, nunca renderiza nem no legado.
- Copiar em `web/app/`, **não** em layer: dentro de layer `~` resolve para `web/app` e
  os imports quebram (`web/layers/finance/AGENT.md:48-53`).
- `definePageMeta` da página: `{ layout: 'dashboard', workspaceId: 'omnichannel' }`
  (o legado usa `layout: 'admin'` — não existe aqui).

**Armadilhas conhecidas:**
- `OmnichannelInboxLoading.vue` usa `USkeleton` **sem bloco `<script>`** — auto-import puro.
- `useOmnichannelInboxRealtime.ts:42` usa `ref()` **sem importar de `vue`** — auto-import Nuxt.
- Nuxt UI aqui é v4.7 (legado: v4.4). `UDashboardGroup/Sidebar/Panel` existem (Pro unificado no v4).

### F1.2 — Os 6 arquivos de costura

Criar em `web/app/`, conforme §5.2 do plano:
`composables/useApi.ts`, `composables/useAdminSession.ts`,
`composables/usePageBootstrapLoading.ts`, `stores/session-simulation.ts`,
`stores/ui.ts`, `types/index.ts`.

`useApi().apiFetch(path, opts)` deve:
- prefixar `/v1/omnichannel` (o legado prefixava `/api/bff`);
- delegar para `createApiRequest(runtimeConfig, () => auth.accessToken)` de `~/utils/api-client`;
- **não** setar `X-Account-Id` (o provider global injeta — `plugins/account-id-bridge.client.ts`);
- exportar `ApiClientError` com a mesma superfície (`useOmnichannelInboxHistory.ts:3` importa).

`useAdminSession()` devolve `{ user, coreUser, token, coreToken, legacyRole, tenantSlug, logout, syncSessionFromToken }`.
`tenantSlug` = slug da conta ativa. `legacyRole` = mapear papel do Omni para
`ADMIN|SUPERVISOR|AGENT|VIEWER` (o front gateia por ele).

### F1.3 — Os 5 repontados

Conforme §5.3 do plano. Em F1 só repontar a URL (o Go ainda não existe):
`useOmnichannelInboxRealtime.ts` (F4 reescreve de fato), `useInboxChatGifAssets.ts`,
`useAvatarProxy.ts`, `useInboxChatMediaActions.ts`, `useOmnichannelInboxOutboundPipeline.ts`
(remover o bypass direto do `:252`, deixar só o `apiFetch`).

### F1.4 — Registro no painel (6 pontos — todos obrigatórios)

Sincronizar, senão dá drift (menu esconde mas rota abre):
1. `web/nuxt.config.ts` — `/omnichannel` já tem `ssr: false`; **adicionar `/omnichannel/**`**.
2. `web/app/utils/workspaces.ts` — `{ id: 'omnichannel', label: 'Omnichannel', icon: 'messages', path: '/omnichannel' }`.
   `icon` é **chave do `NAV_ICON_MAP`** (`useDashboardNav.ts:44-79`), não nome livre — `messages` existe.
3. `web/app/domain/utils/permissions.ts` — `WORKSPACE_ACCESS_DEFINITIONS` +
   `ROLE_WORKSPACES.platform_admin`/`.owner` + `MODULE_WORKSPACE_PERMISSION_PREFIXES.omnichannel = 'omnichannel.'`.
4. `web/layers/queue/nav.config.ts:8-14` — tirar `hidden: true`, adicionar
   `workspaceId: 'omnichannel'` + `moduleId: 'omnichannel'` + `beta: true`.
5. `web/app/middleware/module-enabled.global.ts` — `{ prefix: '/omnichannel', moduleId: 'omnichannel' }`.
6. `back/internal/platform/app/app.go` — `registry.MustRegister` do módulo `omnichannel`
   + `moduleGatingRules` `{Prefix: "/v1/omnichannel", ModuleID: "omnichannel"}`.
   `SyncCatalog` no boot registra permissões e auto-habilita nas contas `is_agency`.

### F1.5 — Limpar o demo

Remover `web/app/pages/omnichannel.vue` (placeholder) e a chave `omnichannel` de
`web/app/utils/demo-pages.ts:22-56`. Precedente: `web/layers/finance/AGENT.md:28-29`.

### F1.6 — Badge de "sem backend" (princípio 4)

Banner visível **só para admin** na página: "SEM BACKEND (F1) — a tela é real, os dados
não. Ver docs/omnichannel/PLANO_PORT_OMNICHANNEL.md". Remover ao fechar F3.

**Verificável:** `/omnichannel` abre logado, layout idêntico ao legado, badge visível,
requests 404 no console. ESLint acusa `max-lines` (esperado). `npm run dev` sem erro de
resolução de import.

---

## F2 — Go: schema + leitura

**Objetivo:** o inbox lista dados reais (vazios, mas do banco).

### F2.1 — Migrations `messaging.*`

8 tabelas conforme §7 do plano. Regras: SQL plano **idempotente** (`IF NOT EXISTS`),
**sem `-- +goose Down`** (o migrator roda o arquivo inteiro e o Down se auto-destrói),
schema-qualificado, `account_id uuid NOT NULL REFERENCES core.accounts(id)`.

Índices que **têm** que existir (o legado depende deles):
- `conversations`: `UNIQUE(account_id, external_id, channel, instance_scope_key)`,
  `(account_id, last_message_at DESC)`, `(account_id, instance_scope_key, last_message_at DESC)`
- `messages`: `(account_id, created_at)`, `(conversation_id, created_at)`
- `contacts`: `UNIQUE(account_id, phone)`
- `whatsapp_instances`: `UNIQUE(account_id, instance_name)`

### F2.2 — Módulo Go

`back/internal/modules/omnichannel/` no padrão da casa: `module.go`, `http*.go`,
`service*.go`, `store_postgres*.go`. Camadas estritas (handler → service → repository).
Teto ~450 linhas/arquivo — **aqui o limite vale** (é código novo).

### F2.3 — Rotas de leitura

`GET /v1/omnichannel/conversations` (query `instanceId?`, ordena `last_message_at DESC`, **sem paginação**),
`GET .../conversations/{id}/messages`, `GET .../conversations/{cid}/messages/{mid}`,
`GET .../contacts`, `POST .../contacts`, `PATCH .../contacts/{id}`,
`GET|PATCH .../account`, `GET .../whatsapp/instances`, `GET .../whatsapp/instances/access`.

**Paginação de mensagens (replicar exato):** `limit` 1..200 default 100 + `beforeId`.
Resolve `beforeId → created_at`, filtra `created_at <`, ordena `DESC`, `take limit`,
**inverte** (devolve ASC). `hasMore` = existe mais antiga que a primeira. Resposta:
`{ conversationId, messages[], hasMore }`.

**Shapes:** copiar campo a campo de §3 do relatório de contrato
(`Message`, `Conversation` com `lastMessage` aninhado, `Contact`). JSON em camelCase.
Divergir um campo = quebrar o front.

**Verificável:** logado, `/omnichannel` lista do banco. `X-Account-Id` de outra conta
→ 404. Inserir uma conversa na mão no banco → aparece na tela.

---

## F3 — Go: Evolution + webhook inbound

**Bloqueada por D1.**

**Objetivo:** conectar um número e ver mensagem recebida chegar no banco.

1. **Client Evolution** (`evolution_client.go`): header `apikey`, timeout 30s.
   `createInstance`, `connect` (`GET /instance/connect/{i}`), `fetchInstances`,
   `logout` (`DELETE /instance/logout/{i}`), `setWebhook`, `getBase64FromMediaMessage`.
2. **Sessão:** `bootstrap` (valida limite de canais → 409; cria/renomeia instância;
   promove default; re-escopa conversas `default`), `connect`, `logout`,
   `status` (com cache + dedupe de in-flight; `includeWebhook` compara e **auto-repara**),
   `qrcode` (QR normalizado para data URL; cache **Redis TTL 120s**,
   chave `wa:qrcode:{accountId}:{instanceName}`).
3. **Webhook** `POST /v1/webhooks/evolution/{accountSlug}` — **rota pública, sem JWT**.
   Proteções na ordem: rate-limit 600/min por `slug:ip` (block 5 min → 429);
   `x-webhook-token` com comparação **constant-time** → 401; allowlist de content-type → 415;
   `content-length` > limite → 413; conta inexistente → 404; idempotência em Redis
   (`done` → 202 duplicate; `processing` → 202 already_processing; erro → release).
   Eventos: `MESSAGES_UPSERT` (create), `MESSAGES_UPDATE` (deleção), `*QRCODE*` → cache.
   Resto → 202 `{status:"ignored", event}`.
   Payload é **dinâmico** — parsear defensivamente. `event` de
   `payload.event ?? payload.type ?? data.event ?? {eventName}`, normalizado
   (`[^a-zA-Z0-9]+ → _`, uppercase). Instância desconhecida é **auto-criada** no legado.
4. Inbound grava `status: SENT` e `created_at` = `messageTimestamp` do provider.
   Dedupe de `external_message_id` é **aplicativo** (não há UNIQUE).

**Verificável:** ler QR no painel, conectar o número, mandar mensagem do celular e ela
existir em `messaging.messages` com a conversa certa. Webhook sem token → 401.

---

## F4 — Realtime

**Objetivo:** mensagem aparece ao vivo.

1. **Go:** `GET /v1/realtime/omnichannel` no padrão ticket (`POST /v1/ws/ticket` →
   `?scope=account&accountId=&ticket=`). Canal `omnichannel:account:{id}`.
   Injetar `realtimeService` como `Publisher` (igual `calendar.WithPublisher`).
2. **Eventos — exatamente 3:** `message.created`, `message.updated`, `conversation.updated`.
   **Replicar os shapes por call-site**, sem unificar:
   - `message.created`: envio HTTP → Message completo + `correlationId`;
     webhook → subconjunto `{id, content, messageType, mediaUrl, direction, status, createdAt, correlationId}`.
   - `message.updated`: worker → **mínimo** `{id, status, externalMessageId, updatedAt, correlationId}`;
     webhook → subconjunto; rehidratação de mídia → Message completo **sem** `correlationId`.
   - `conversation.updated`: webhook → **sem** `instanceName`/`instanceDisplayName`;
     status/contacts → **com**.
3. **Sanitizar mídia:** data URL → `null` no realtime. Nunca base64 no WS.
4. **Front:** reescrever `useOmnichannelInboxRealtime.ts` sobre `useRealtimeSocket`
   (`web/layers/tasks/composables/useRealtimeSocket.ts`), preservando handlers, nomes de
   evento e os fallbacks de polling (status 45s, stale 20s, heartbeat 5min, cooldown 5s).
   Erros de auth eram por string (`ModuleAccessDenied`, `Unauthorized`) — mapear.
   **accountId pela MESMA fonte do REST** (`accountStore.activeAccountId || ...`),
   senão o `platform_admin` cai no seed e o handshake nunca vira 101
   (`web/layers/tasks/AGENT.md:315-331`).

**Verificável:** duas abas em contas diferentes; mensagem do celular aparece ao vivo só
na conta certa. Derrubar o WS → polling assume → volta ao reconectar.

---

## F5 — Envio + mídia

1. **`POST /v1/omnichannel/conversations/{id}/messages`** — body do legado
   (`type` TEXT|IMAGE|AUDIO|VIDEO|DOCUMENT, `content` max 4000, `mediaUrl` 1..60MB,
   `mediaMimeType`, `mediaFileName`, `mediaFileSizeBytes`, `mediaCaption`,
   `mediaDurationSeconds`, `metadataJson`). TEXT exige `content`; resto exige `mediaUrl`.
   Fluxo: valida escopo → valida upload (413/415 com `{message, code, details}`) →
   cria message `PENDING`/`OUTBOUND` → atualiza `last_message_at` + `status=OPEN` →
   enfileira → publica `message.created` → audita → **200**.
   Falha ao enfileirar → marca `FAILED`, publica, **202**.
   VIEWER → 403 (é permissão, não escopo).
2. **Outbox + worker** (goroutine, sem BullMQ). Retry: transitório → 5;
   401/403/404/405 e 400/422 conhecidos → **1 (unrecoverable)**; 429 → 5; 5xx → 4;
   sem status → 4; outros → 3. Esgotou → `FAILED` + audit.
   Monitor de `PENDING` presas >10 min re-enfileira até 20/ciclo a cada 5 min —
   **com filtro de conta** (o legado varre a tabela toda).
   Reply/quote vai em `metadataJson`; sticker = `type: IMAGE` + `metadataJson.media.sendAsSticker`.
3. **`GET .../messages/{mid}/media`** — query `disposition` (`inline`|`attachment`),
   `download`. Exclui `hidden_messages` do usuário → 404. Se `mediaUrl` vazio ou
   `metadata.requiresMediaDecrypt` ou `mediaSourceKind === "url_encrypted"` →
   **rehidratar** via `POST /chat/getBase64FromMediaMessage/{i}`, persistir e emitir
   `message.updated` (one-shot por request). Anti-SSRF: bloquear hosts internos → 403;
   protocolo ≠ http/https → 422. `Cache-Control: private, max-age=60`.
   **Divergência (D2):** stream do disco com `Range`, sem carregar em memória
   (o legado faz `Buffer.from(await res.arrayBuffer())` inteiro).

**Verificável:** responder do painel e chegar no celular; status vira SENT; foto/áudio
sobem e reproduzem; derrubar o Evolution → FAILED após os retries.

---

## F6 — Ações

`POST .../messages/{mid}/reaction`, `POST .../messages/forward`
(`{messageIds[1..100], targetConversationId}`), `POST .../messages/delete-for-me`
(grava em `hidden_messages`), `POST .../messages/delete-for-all`,
`PATCH .../conversations/{id}/status` (emite `conversation.updated` **com**
`instanceName`), `PATCH .../conversations/{id}/assign`,
`GET .../conversations/{id}/group-participants`, `POST .../conversations/sync-open`,
`POST .../messages/sync-history`, `POST .../contacts/import-whatsapp`,
`POST .../contacts/{id}/open-conversation`.

Auditar em `messaging.audit_events`: `MESSAGE_OUTBOUND_QUEUED|SENT|FAILED`,
`CONVERSATION_STATUS_CHANGED`, `CONVERSATION_ASSIGNED`.

**Verificável:** cada botão da UI funciona ponta a ponta.

---

## F7 — Stickers / GIF / avatar

1. `GET|POST|DELETE /v1/omnichannel/stickers` — `limit` 1..200 default **36**; array direto;
   allowlist `image/webp|png|jpeg|jpg|gif` → 415; limite `min(conta.max_upload_mb, 20MB)` → 413;
   POST → **201**; DELETE → **204**; poda FIFO acima de **200/conta**.
2. `GET /v1/omnichannel/gif/search` + `/gif/media` — substituem o Nitro.
   **Chave do Tenor vem do painel/banco**, não de env.
3. `GET /v1/omnichannel/avatar?url=` — proxy com anti-SSRF (mesma allowlist do `/media`).

**Verificável:** salvar/usar/apagar sticker; buscar GIF; avatar carrega sem CORS.

---

## F8 — IA no n8n (o encanamento)

**Objetivo:** bot respondendo, **com toda a config no painel** e o n8n sem lógica.

1. **Config no painel** (banco): provider, modelo, prompt/persona, robô on/off por conta,
   handover (pausar bot por contato), janela de debounce.
2. **Rotas runtime** (auth por token de serviço, fora do gating):
   - `GET /v1/runtime/omnichannel/config?conversationId=` → tudo que o n8n precisa
     (provider, modelo, prompt montado, histórico, estado do handover).
   - `POST /v1/runtime/omnichannel/reply` → o Go persiste e envia pelo Evolution.
3. **Dispatch:** o **Go** decide (pela config) se chama o n8n, depois de já ter
   persistido e emitido o realtime. Handover pausado → não dispara.
4. **Workflow n8n** (~4 nós, zero lógica):
   `Webhook → HTTP GET config → AI Agent (modelo/prompt por expression) → HTTP POST reply`.
   Nada de prompt, credencial de modelo ou regra dentro do nó.
   Depois: `npm run n8n:import` (o n8n roda do banco, não do arquivo).

**Verificável:** mandar mensagem → bot responde. **Trocar o prompt no painel muda a
resposta sem tocar no n8n.** Exportar e reimportar o workflow não perde nada. Pausar o
contato → bot cala.

---

## F9 — Refactor (o "nosso novo modo")

Só depois de F1..F8 verdes:
1. Split dos arquivos >450 linhas (`useOmnichannelInbox.ts` 1.467, `InboxConversationsSidebar.vue`
   1.128, `InboxChatPanel.vue` 1.110, `useOmnichannelInboxHistory.ts` 774, `useOmnichannelAdmin.ts` 764).
   **Incremental, com smoke no browser a cada split** (lição do `useTasksPageContext` 3.063: adiado por ser arriscado de uma vez).
2. Remover os 6 adaptadores; o módulo passa a usar `createApiRequest`/auth do Omni direto.
3. Mover `web/app/**/omnichannel/**` → `web/layers/omnichannel/` (aí sim, com imports relativos).
4. Trocar componentes do legado pelos do design system (`OmniEntityDrawer` etc.).
5. Tirar os itens do `docs/LEGADO.md`.
6. Avaliar `DELIVERED`/`READ` (feature nova).
7. `web/app/components/omnichannel/AGENT.md` + `back/internal/modules/omnichannel/AGENT.md`.

**Verificável:** ESLint sem `max-lines`; sem adaptadores; `LEGADO.md` limpo; tudo que
funcionava em F8 continua funcionando.
