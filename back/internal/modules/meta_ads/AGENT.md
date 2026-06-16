# AGENT — Modulo Go `meta_ads`

Modulo plugavel (Module Registry) de integracao com **Meta/Facebook Ads** dentro do Omni.
Tenant-aware (schema `meta_ads.*`, `account_id` FK `core.accounts`). Puxa dados da
Marketing API para o nosso cache (fonte de verdade dos relatorios) e, em fase seguinte,
cria/edita campanhas. Espelha o modulo `automation` (modulo Go + cliente HTTP externo).

> Regras herdadas (obrigatorias): @AGENT_RULES.md + @docs/ENGINEERING_PRINCIPLES.md.
> Plano canonico do desenho: docs/meta-ads/PLANO_INTEGRACAO_META_ADS.md.
> Modelo agencia->cliente (reservado p/ P5): docs/AGENCY_TENANT_ARCHITECTURE.md.

## Estado: MVP (A1 fundacao + A2 cliente/sync + A3 HTTP) + Assistente MCP (MA2) — 2026-06-11

- **MVP:** conectar (System User token cifrado), listar contas de anuncio, sincronizar
  campanhas + insights diarios (Graph -> cache) e ler KPIs/series/campanhas pelo painel
  `/meta-ads`. Acesso V0: **so platform_admin** (gating por modulo + bypass do admin).
- **"Conectou, puxou, apareceu"** = fim do MVP. Write ops (criar/editar/pausar campanha),
  agregacoes ricas, sync em background, IA e OAuth Facebook Login vem na fase Plataforma
  (P1-P10, em docs/meta-ads/PLANO_INTEGRACAO_META_ADS.md).
- **MA2 (assistente MCP):** chat persistido por account (`meta_ads.assistant_messages`) +
  proxy para o **agent-runner** (sidecar Node, fase MA1) que roda o Claude headless com o
  MCP oficial da Meta. Pos-acao dispara sync best-effort para o cache refletir na hora.
  Plano canonico: PLANO_INTEGRACAO_META_ADS.md secao 12 (fases MA1-MA4).

## Estrutura

```
back/internal/modules/meta_ads/
  model.go            <- Connection, AdAccount, Campaign, InsightDaily + *View + mappers (FROZEN)
  model_assistant.go  <- AssistantMessage + AssistantAction/MessageView/SendResult/HealthView (FROZEN)
  meta_client.go      <- cliente HTTP da Graph/Marketing API (GetAdAccounts/ListCampaigns/GetInsights)
  meta_client_instagram.go <- cliente Graph p/ Instagram Business (ListPagesWithInstagram/ListInstagramMedia)
  runner_client.go    <- cliente HTTP do agent-runner (POST /run, GET /healthz, Bearer interno)
  store_postgres.go   <- persistencia de connections (cifra/decifra token via pgcrypto) + scan helpers
  store_cache.go      <- persistencia de ad_accounts/campaigns/insights_daily (upserts ON CONFLICT)
  store_assistant.go  <- persistencia de assistant_messages (List ultimas N reordenadas asc + Insert)
  service.go          <- conexao + overview + read paths (ad-accounts/campaigns/insights) + KPIs + SetBridgeToken
  service_sync.go     <- orquestracao do sync Graph -> cache (campanhas + insights nos 2 niveis)
  service_assistant.go<- historico + send (persiste, roda runner, sync pos-acao) + health
  service_instagram.go<- bridge do runner: InstagramAccounts/InstagramMedia (token decifrado -> Graph)
  http.go             <- RegisterRoutes + handlers (overview/connection/ad-accounts) + erro + accountIDFromContext
  http_reports.go     <- handlers de sync/campaigns/insights
  http_assistant.go   <- handlers do assistente (messages GET/POST + health) + writeAssistantError
  http_instagram.go   <- BRIDGE INTERNO do runner (/internal/meta-ads/*, bearer de servico, SEM JWT)
  module.go           <- adaptador Module Registry (ID/Metadata/Permissions/Build) (FROZEN)
  AGENT.md            <- este arquivo
```

`model.go`, `model_assistant.go` e `module.go` sao o **contrato congelado** (assinaturas de
construtor e shapes de View — o front MA3 codifica contra eles). O resto e implementacao
contra esse contrato. `NewService(store, client, runner)` recebe o RunnerClient desde MA2.

## Tabelas (migrations `0149_meta_ads_schema.sql` + `0150_meta_ads_assistant.sql`, idempotentes)

Convencao multitenant: toda tabela tem `account_id NOT NULL` FK `core.accounts`.
`organization_id`/`client_account_id` entram **nullable e reservados** (backfill em P5 quando
o modelo agencia->cliente subir); o MVP **nao** depende deles.

- `meta_ads.connections` — 1 conexao Meta por account. `encrypted_token bytea` cifrado via
  `pgp_sym_encrypt(token, $key)`; `status` active|revoked. UNIQUE `(account_id)`. O token **nunca**
  vive na struct `Connection` nem e devolvido ao front — e decifrado sob demanda
  (`GetDecryptedToken`) so para chamar a Graph.
- `meta_ads.ad_accounts` — contas de anuncio (`act_...`) descobertas na conexao. UNIQUE
  `(account_id, meta_ad_account_id)`.
- `meta_ads.campaigns` — cache de campanhas (sync da Marketing API). `daily_budget`/`lifetime_budget`
  em `numeric(15,2)` (unidade da moeda, **nao** centavos). UNIQUE `(ad_account_id, meta_campaign_id)`.
- `meta_ads.insights_daily` — cache de metricas diarias (alimenta graficos). `meta_campaign_id = ''`
  (string vazia, **nao** NULL) e a linha **agregada da conta** no dia; `<> ''` e por campanha.
  UNIQUE `(ad_account_id, meta_campaign_id, date)`. Indices `(account_id, date desc)` e
  `(ad_account_id, date desc)`.
- `meta_ads.assistant_messages` (0150) — historico do chat do assistente, por account.
  `role` check `user|assistant`; `actions jsonb` = `[]AssistantAction` (`{tool, summary,
  status}`) executadas pelo runner, NULL quando a resposta nao executou acao. Indice
  `(account_id, created_at desc)`. E tambem a **auditoria** do que a IA fez.

## Endpoints (`/v1/meta-ads`, JWT + X-Account-Id)

`accountID` vem do `Principal` (X-Account-Id ou `principal.TenantID`), **nunca** do body.

| Verbo | Path | Permissao | Acao |
|---|---|---|---|
| GET | `/v1/meta-ads/overview?adAccountId=` | `meta_ads.view` | status da conexao + KPIs (do cache). Sem conexao: 200 com `connection.connected=false` (NAO e erro) |
| POST | `/v1/meta-ads/connection` | `meta_ads.connect` | body `{ token }`: valida na Graph, cifra e persiste; descobre/cacheia ad accounts. 201 `ConnectionView` |
| DELETE | `/v1/meta-ads/connection` | `meta_ads.connect` | remove a conexao (cascade no cache). 204 |
| GET | `/v1/meta-ads/ad-accounts` | `meta_ads.view` | `[]AdAccountView` do cache (busca ao vivo + popula se vazio) |
| POST | `/v1/meta-ads/sync` | `meta_ads.view` | body `{ adAccountId }` (ou `?adAccountId=`): sync Graph->cache de campanhas + insights (30d). `SyncResult` |
| GET | `/v1/meta-ads/campaigns?adAccountId=` | `meta_ads.view` | `[]CampaignView` do cache |
| GET | `/v1/meta-ads/insights?adAccountId=&range=&level=` | `meta_ads.view` | `[]InsightPoint` do cache. `range` = last_7d/14d/30d/90d (default 30d); `level` = account (default)/campaign |
| GET | `/v1/meta-ads/assistant/messages?limit=` | `meta_ads.view` | historico do chat: array puro de `AssistantMessageView` em ordem cronologica (ultimas N; default/teto 50) |
| POST | `/v1/meta-ads/assistant/messages` | `meta_ads.manage` (gating V0: platform_admin) | body `{ message, adAccountId }` (max 64KB; mensagem max 4000 chars): persiste a msg do usuario, roda o runner, persiste a resposta+acoes. 200 `AssistantSendResult` `{ messages: [userView, assistantView], syncTriggered }` |
| GET | `/v1/meta-ads/assistant/health` | `meta_ads.view` | status do runner: `AssistantHealthView` `{ ok, claudeAuth, detail }`. **200 sempre** (down = `ok:false` + detail `runner_not_configured`/`runner_unreachable`) |

**Mapeamento de erro (`writeServiceError`):**
- nao conectado (sync/campaigns/insights) -> `404 not_connected` "Conecte uma conta Meta primeiro." (`ErrNotConnected`)
- recurso de outra account / inexistente (`pgx.ErrNoRows`) -> `404 not_found`
- chave de cifra ausente no connect -> `503 crypto_not_configured` (`ErrCryptoKeyMissing`)
- falha da Graph (prefixo `meta graph:`) -> `502 meta_error`
- body invalido / faltando token ou adAccountId -> `400`
- resto -> `500 internal_error`

**Mapeamento de erro do assistente (`writeAssistantError`, http_assistant.go):**
- mensagem vazia -> `400 missing_message`; acima de 4000 chars -> `400 message_too_long`
- runner nao configurado (env vazia) -> `503 assistant_not_configured` (`ErrRunnerNotConfigured`)
- runner falhou (rede/HTTP nao-2xx/JSON invalido) -> `502 assistant_error`
  "O assistente nao conseguiu responder. Verifique o runner." (`errRunnerFailed`)
- resto (store) -> `writeServiceError`

**Shapes do assistente (contrato CONGELADO com o front MA3):**
- `AssistantMessageView` = `{ id, role: "user"|"assistant", content, actions: AssistantAction[], createdAt }` —
  `actions` NUNCA e null (sem acoes = `[]`); `createdAt` RFC3339 UTC.
- `AssistantAction` = `{ tool, summary, status: "ok"|"error" }`.
- `AssistantSendResult` = `{ messages: AssistantMessageView[2], syncTriggered: bool }`.
- `AssistantHealthView` = `{ ok, claudeAuth, detail }`.

## Bridge interno do runner (`/internal/meta-ads/runner/*`, SEM JWT)

O assistente (runner Node em `meta-ads-assistant/`, processo no **HOST**) precisa das
postagens recentes do Instagram Business para criar campanhas a partir delas. O MCP
oficial da Meta **nao tem** ferramenta de feed; o Go ja tem o System User token, entao
expoe um **bridge interno** que o runner consome.

> **Gotcha de seguranca:** as rotas `/internal/*` **NAO passam pelo middleware JWT** nem
> pelo gating de modulo (ficam fora do prefixo `/v1/meta-ads`). A seguranca e: **bearer de
> servico** (`META_ADS_RUNNER_BRIDGE_TOKEN`, comparado em tempo constante via
> `bridgeBearerEquals`) **+ rede interna**. O `accountId` vem da query e e confiavel APENAS
> por estar atras do bearer; ainda assim validamos formato (nao-vazio) e a existencia da
> conexao Meta (404). Shape de erro **FLAT** `{ "error", "message" }` — distinto do envelope
> `{ error: { code, message } }` do painel (`httpapi.WriteError`); por isso os handlers do
> bridge montam o JSON direto com `httpapi.WriteJSON`.

| Verbo | Path | Resposta 200 |
|---|---|---|
| GET | `/internal/meta-ads/runner/instagram/accounts?accountId=<uuid>` | `{ "accounts": [{ igUserId, username, pageId, pageName }] }` |
| GET | `/internal/meta-ads/runner/instagram/media?accountId=<uuid>&igUserId=<opcional>&limit=<1..20, default 5>` | `{ "media": [{ id, caption, mediaType: IMAGE\|VIDEO\|CAROUSEL_ALBUM, mediaUrl, thumbnailUrl, permalink, timestamp }] }` |

- `igUserId` vazio = usa a **primeira** conta IG disponivel da conexao. Campos ausentes na
  Graph viram string vazia (ex.: `thumbnailUrl` so existe para VIDEO). `limit` fora de
  1..20 -> clamp (`clampMediaLimit`).
- **Erros do bridge** (`writeBridgeError`/`writeBridgeErrorCode`, shape FLAT):
  - env `META_ADS_RUNNER_BRIDGE_TOKEN` vazia -> `503 { "error": "bridge_not_configured" }`
  - token errado / sem Bearer -> `401 { "error": "unauthorized" }`
  - `accountId` ausente -> `400 { "error": "missing_account_id" }`
  - account sem conexao Meta ativa -> `404 { "error": "not_connected" }` (`ErrNotConnected`)
  - falha da Graph -> `502 { "error": "graph_error", "message": "<mensagem da Graph SEM token>" }`
  - resto -> `500 { "error": "internal_error" }`
- **Token nunca vaza:** `service_instagram.go` decifra o token sob demanda (`connectionToken`
  -> `GetConnection` + `GetDecryptedToken`, mesmo caminho do sync) e o passa ao `MetaClient`.
  A `message` do `graph_error` vem de `graphError` (meta_client.go), que so ecoa a mensagem +
  status da Graph — a URL com `access_token` **nunca** entra na string de erro.
- **Cliente Graph (`meta_client_instagram.go`):** `ListPagesWithInstagram` ->
  `/me/accounts?fields=id,name,instagram_business_account{id,username}&limit=50` (so paginas
  COM IG Business entram; pagina pelo cursor `paging.cursors.after`, **max 3 paginas**).
  `ListInstagramMedia` -> `/{ig-user-id}/media?fields=id,caption,media_type,media_url,
  thumbnail_url,permalink,timestamp&limit={limit}`. Reusa `getJSON` (host de config, timeout
  15s, contexto, token via query oculto do erro).
- **Payload do runner (`POST /run`):** ganhou o campo `accountId` (`runner_client.go`
  `Run(ctx, prompt, history, adAccountID, accountID, opts)`). O runner devolve esse
  `accountId` ao bridge para buscar o Instagram da conta certa. `AssistantSend` ja tinha o
  `accountID` em maos. Nenhum outro campo do shape mudou.

## Cliente Meta (`meta_client.go`)

- Base = `META_ADS_GRAPH_BASE` (default `https://graph.facebook.com/v21.0`). `http.Client` 15s,
  contexto em toda chamada (`http.NewRequestWithContext`). Auth por `access_token` em query param.
- `GetAdAccounts` -> `/me/adaccounts?fields=account_id,name,currency,account_status` (tambem **valida**
  o token no connect).
- `ListCampaigns` -> `/act_{id}/campaigns?fields=id,name,objective,status,daily_budget,lifetime_budget`
  (prefixo `act_` garantido por `actPrefixed`).
- `GetInsights` -> `/act_{id}/insights?level=&date_preset=&time_increment=1&fields=...,actions`.
- **Paginacao:** MVP le **uma pagina** (campos `data`/`paging.next` mapeados; seguir `next` fica
  para P3). Limites altos (200 campanhas / 500 insights) cobrem o MVP.

## Cliente do agent-runner (`runner_client.go`) — contrato CONGELADO com MA1

- Sidecar Node **interno** (rede do compose, profile `meta-ads-assistant`) que roda o Claude
  headless (assinatura, sem credito de API) com o MCP oficial da Meta. **Nunca exposto ao
  cliente** — o painel so fala com o Go, que persiste o historico e faz o proxy por account.
- Config: `META_ADS_ASSISTANT_RUNNER_URL` + `META_ADS_ASSISTANT_TOKEN` (lidas no `Build` do
  module.go, `strings.TrimSpace`, **sem default no Go** — o compose fornece). Qualquer uma
  vazia => `ErrRunnerNotConfigured` (503) sem tentar rede.
- `http.Client` com timeout **150s** (runs sao lentos: Claude + tools MCP), contexto em toda
  chamada (`http.NewRequestWithContext`), corpo de resposta limitado a 4MB.
- `Run(ctx, prompt, history, adAccountID, accountID)` -> `POST {base}/run` com `Authorization:
  Bearer {token}`, body `{"prompt", "history":[{"role","content"}...], "adAccountId",
  "accountId"}`; resposta `{"reply", "actions":[{"tool","summary","status"}]}`. O `accountId`
  (nosso ID interno) volta ao **bridge interno** do Go para o runner buscar o feed do Instagram.
- `Health(ctx)` -> `GET {base}/healthz` -> `{ok, claudeAuth, detail?}`. Resposta fora do
  contrato vira `ok:false` (sem erro).

## Fluxo do AssistantSend (service_assistant.go)

1. Trim + valida a mensagem (vazia -> 400; > 4000 chars -> 400).
2. **Persiste a msg do usuario primeiro** — se o runner falhar, o que foi digitado nao se perde.
3. Carrega as ultimas **12** mensagens como historico (excluindo a recem-inserida — ela vai
   como `prompt`, nao como turno repetido) e chama `runner.Run`.
4. Persiste a resposta (`reply` + `actions` serializadas em jsonb; sem acoes = NULL).
5. Se houve acoes **e** veio `adAccountId`: `s.Sync(ctx, accountID, adAccountID)` best-effort
   (`syncTriggered:true`; falha do sync = so `slog.Warn`, **nao** falha a requisicao).
6. Devolve as duas Views novas (`messages[0]`=user, `messages[1]`=assistant).

## Decisoes de mapeamento (conferir na integracao)

- **Orcamento (budget):** a Graph devolve `daily_budget`/`lifetime_budget` como **string em centavos**
  da moeda da conta. `budgetCentsToUnits` divide por 100 -> unidade da moeda (reais/dolares), `*float64`
  (string vazia -> `nil` = sem aquele tipo de orcamento). A coluna e `numeric(15,2)`.
- **Conversoes:** derivadas do array `actions` do insight (`conversionsFromActions`), somando os
  `action_type` de compra/lead/registro (purchase / *_purchase / lead / complete_registration). Sem
  acao reconhecida -> 0. MVP simples; afinar a lista por objetivo na fase de relatorios (P3).
- **KPIs do overview:** somatorio dos insights **agregados da conta** (`meta_campaign_id = ''`) na
  janela de 30d; `CTR = clicks/impressions*100` e `CPC = spend/clicks` recalculados do total (nao media
  de medias). Zerados (sem erro) quando nao ha `adAccountId` ou nao ha insights.
- **Sync:** puxa `last_30d` com `time_increment=1` nos **dois niveis** (campanha + agregado da conta) e
  campanhas; faz upsert (ON CONFLICT) — re-sync e idempotente.

## Gating / permissoes

- Registrado no Registry em `app.go` (`registry.MustRegister(metaads.New())`) — seeda `core.modules`
  + permissoes `meta_ads.view` / `meta_ads.manage` / `meta_ads.connect` e o RoleTemplate
  `meta_ads.manager`. (Registro feito pela fundacao A1.)
- Rotas gateadas por path (`/v1/meta-ads` -> `meta_ads`) via `RequireModuleByPath` no Chain;
  o handler so exige `RequireAuth` (sem checagem de permissao por handler, espelhando o automation).
  **platform_admin tem bypass** -> admins entram; contas sem o modulo habilitado levam
  `403 module_disabled`. Front: workspace `meta_ads` so em `ROLE_WORKSPACES.platform_admin`
  (permissions.ts), nav `hidden:true` ate validar o MVP.

## Seguranca do token (obrigatorio)

- O **System User token** da longa duracao tem acesso total a conta de anuncios -> tratado como
  segredo. Cifrado at-rest via pgcrypto (`pgp_sym_encrypt`/`pgp_sym_decrypt`, chave
  `META_ADS_CRYPTO_KEY`). **Nunca** logado (slog so com campos explicitos; nunca o valor do token),
  **nunca** devolvido ao front (sem campo de token em nenhuma View), so existe em claro dentro do
  processo Go no instante da chamada a Graph.
- Sem `META_ADS_CRYPTO_KEY` o connect **falha rapido** com `ErrCryptoKeyMissing` ->
  `503 crypto_not_configured` (nao grava token em claro).

## Notas de Deploy

- Migrations `0149_meta_ads_schema.sql` (extensao `pgcrypto` + schema `meta_ads`) e
  `0150_meta_ads_assistant.sql` (`meta_ads.assistant_messages`) — idempotentes, rodam no boot
  da api. **Rebuild obrigatorio:** `docker compose up -d --build api`.
- Vars (em `.env.docker.example`, `.env.production.example` **e** `docker-compose.prod.yml`
  na secao `environment`):
  - `META_ADS_GRAPH_BASE` (default `https://graph.facebook.com/v21.0`).
  - `META_ADS_CRYPTO_KEY` (chave simetrica pgcrypto p/ cifrar o token — **obrigatoria p/ conectar**).
  - `META_ADS_ASSISTANT_RUNNER_URL` (MA2; ex. `http://meta-ads-assistant:8787` — rede interna do
    compose, **sem default no Go**; vazia = assistente desligado com 503).
  - `META_ADS_ASSISTANT_TOKEN` (MA2; Bearer interno Go -> runner; vazia = assistente desligado).
  - `META_ADS_RUNNER_BRIDGE_TOKEN` (bridge `/internal/meta-ads/*`; Bearer de servico runner ->
    Go; vazia = bridge desligado, `503 bridge_not_configured`). Adicionada em
    `.env.docker.example` e na secao `environment` do servico `api` em `docker-compose.yml`.
- O **agent-runner** (container do profile `meta-ads-assistant`, fase MA1) e dependencia do
  assistente, nao da api: sem ele a api sobe normal e os endpoints `/assistant/*` respondem
  503/`ok:false`. Relatorios continuam sem container novo (Go -> Graph direto).
- Ativar o modulo para a conta da agencia: inserir `meta_ads` em `core.account_modules` da conta
  Crow (ou testar via platform_admin, isento do gating).

## Gotchas

- `meta_campaign_id = ''` (string vazia, nao NULL) e o agregado da conta no `insights_daily` — o
  UNIQUE deduplica (NULLs seriam distintos). `ListInsights` filtra por `level` usando isso.
- O cache e a fonte do dashboard: `campaigns`/`insights`/`overview` **leem do cache**, nunca batem na
  Graph. So `POST /sync` (e o `connect`/ad-accounts-vazio) vao a Graph (principio de performance).
- `accountStatusLabel` traduz o `account_status` numerico da Graph (1=active, 2=disabled, ...) em
  rotulo curto guardado em `ad_accounts.status`.
- 1 conexao por account no MVP (UNIQUE em `account_id`); reconectar com novo token faz upsert
  (sobrescreve o token cifrado, reativa).
- `ListAssistantMessages` aplica o limit nas **mais recentes** (subselect `desc limit N`) e
  reordena `asc` — o front recebe cronologico sem perder as ultimas mensagens.
- `actions` no JSON do front **nunca** e null: jsonb NULL/invalido vira `[]` no mapper
  (`toAssistantMessageView`). O `[]AssistantAction` so e gravado quando `len(actions) > 0`.

## Guardrails do assistente (MA2/MA4)

- **Toda acao de escrita exige confirmacao explicita no chat** antes de executar — politica do
  prompt do runner (MA1/MA4); o Go persiste o dialogo inteiro como trilha de auditoria.
- **Campanha criada por IA nasce PAUSADA** (regra nativa do MCP oficial da Meta) — ativacao e
  manual, por humano. E guardrail, nao bug; o painel lembra isso ao usuario.
- O runner consome a **franquia da assinatura Claude** (mesma cota do Claude Code) — uso interno
  da agencia; se escalar para clientes, troca-se o driver do runner por API sem mudar este modulo.
- O token do runner e infra interna (compose) — nunca devolvido ao front nem logado.
