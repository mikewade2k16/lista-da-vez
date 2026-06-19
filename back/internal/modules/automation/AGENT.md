# AGENT — Modulo Go `automation`

Modulo plugavel (Module Registry) do painel de automacao WhatsApp/IA dentro do Omni.
Tenant-aware (schema `automation.*`, `account_id` FK `core.accounts`). Faz o proxy
escopado da WAHA para o front, sem expor n8n/WAHA ao cliente.

> Visao de plataforma (multi-tenant, BYOK, RAG, personas): docs/automation/PLATAFORMA_AUTOMACAO.md.
> Infra dos containers (n8n/waha/redis): automation/AGENT.md (raiz). Esquema-alvo:
> docs/automation/schema_automation_sketch.sql.

## Estado: M1 + M2 + M3 + M3+ + A6 + A7 + M4 + M5 + Omni Chat (M0 + Fase 2 catalogo) — 2026-06-18

- **M1:** conectar o WhatsApp (QR via proxy WAHA), ver status e ligar/desligar o robo, pelo
  painel `/automation`. Acesso V0: **so platform_admin** (gating por modulo + bypass do admin).
- **M2:** comportamento **config-driven**. A persona (instrucoes) vive no banco
  (`automation.personas`); o n8n consome `runtime-config` a cada execucao. Liga/desliga vale.
  Persona default semeada: Tony / Crow Visuals (`//go:embed` de defaults/persona.md).
- **M3:** editor de persona (instrucoes + nome) no painel. GET/PUT `/v1/automation/persona`.
- **M3+:** knowledge por documento. `automation.knowledge_documents` (migration 0142). CRUD
  pelo painel (card Conhecimento). O `runtime-config` concatena os docs habilitados apos as
  instrucoes e antes dos guardrails (`buildSystemMessage`). RAG pgvector (P8) quando o volume
  nao couber no contexto.
- **A6 (Painel de Modelos):** catalogo global de modelos por funcao (`automation.model_catalog`,
  migration 0144, provider-agnostico OpenAI+Anthropic) + selecao por automacao+funcao
  (`automation.automation_models`). Card "Modelos" no painel aplica as regras do MODELOS.md
  sozinho (esconde temperatura em modelo de raciocinio; so modelos `vision_ok` na funcao visao).
  O `runtime-config` passa a devolver `models[]` com as flags para o n8n consumir por expression.
- **A7 (persistencia de conversa):** `automation.messages` (historico in/out) +
  `automation.lead_state` (status do funil + follow-up), migration 0145. Endpoints de runtime
  (token de servico) para o n8n gravar mensagens e ler/escrever o estado do lead.

- **M4 (trava de handover humano):** `automation.contacts.paused_until` (migration 0148).
  Endpoint runtime `POST /v1/runtime/automation/handover` pausa/retoma o bot por contato
  (atendente humano assumiu). O `GET /v1/runtime/automation/memory` passou a expor
  `paused` (bool: `paused_until > now()`) e `pausedUntil` (RFC3339 ou vazio) para o n8n
  ficar em silencio enquanto a pausa nao expira. Pausa sobrevive a writes de memoria
  (o upsert de memoria nao toca `paused_until`).
- **M5 (tool de produto + fontes):** config de fontes por automacao no `settings jsonb`
  (chave `sources`, sem migration nova) via `GET/PUT /v1/automation/sources`. Tool runtime
  `GET /v1/runtime/automation/tools/catalog` faz busca ESTREITA escopada por account em
  `site.products` (so `SELECT`; modulo site nao e tocado). Fonte plugavel via interface
  `ProductSource` (hoje `site`; ERP/catalog futuros).

- **Omni Chat (M0):** chat interno do painel de **Operacao** (`/operacao`), ligado ao n8n por
  proxy `Front -> API Go -> n8n` (o browser nunca fala com o n8n). Endpoint
  `POST /v1/omni-chat/ask` (RequireAuth, **fora** do prefixo `/v1/automation`). O Go monta o
  `systemMessage` do Tony reusando `GetOrCreateDefault` + `ensurePersona` + `ListKnowledgeDocs`
  + `buildSystemMessage` (sem sessao WAHA) e chama o webhook interno
  `POST http://n8n:5678/webhook/omni-chat`. M0 = pipeline + persona, **sem** consultar banco/CRM.
  Contrato congelado: docs/automation/OMNI_CHAT_PLAN.md.

- **Omni Chat — Fase 2 (tools de dados):** o `/ask` agora emite um **context token HMAC** opaco
  (`ctxv1`) que escopa as tools de dados; o n8n o reenvia no header `X-Omni-Context` ao chamar
  uma tool. Primeira tool: **catalogo** (`GET /v1/runtime/omni-chat/catalog`). O escopo
  (account/tenant/store/user/role) sai SO do token assinado, NUNCA do query/body do n8n. Tools de
  ranking/metas entram a seguir (reusam o mesmo token). Detalhe abaixo em "Omni Chat — Fase 2".

Proximas fases (BYOK, multi-numero, CRM, tools de ranking/metas) em PLATAFORMA_AUTOMACAO.md (P6+).

## Estrutura

```
back/internal/modules/automation/
  model.go             <- Automation, Channel, Persona, KnowledgeDoc e *View
  models.go            <- catalogo de modelos + selecao por funcao (A6) e *View
  model_conversation.go<- Message, LeadState e *View (A7)
  model_product.go     <- SourcesView, ProductHit (M5)
  waha_client.go       <- cliente HTTP interno da WAHA (status/start/stop/qr)
  store_postgres.go    <- persistencia (automations/channels/personas/knowledge_documents/contacts)
  store_models.go      <- persistencia do catalogo + selecao de modelos (A6)
  store_conversation.go<- persistencia de messages + lead_state (A7)
  store_product.go     <- settings.sources jsonb + SELECT escopado em site.products (M5)
  service.go           <- orquestra store + WAHA + n8n; buildSystemMessage (instrucoes+docs+guardrails)
  n8n_client.go        <- cliente HTTP do webhook interno do Omni Chat (POST /webhook/omni-chat) (M0)
  service_omnichat.go  <- OmniChatAsk: monta systemMessage do Tony + emite context token + chama n8n.Ask
  http_omnichat.go     <- handler POST /v1/omni-chat/ask (RequireAuth; fora do gating) (M0)
  context_token.go     <- ContextTokenManager: Issue/Parse do context token HMAC (ctxv1, Fase 2)
  http_omnichat_tools.go<- tools de dados do Omni Chat (GET /v1/runtime/omni-chat/catalog) (Fase 2)
  service_models.go    <- regras do MODELOS.md (valida catalogo, sanitiza params) (A6)
  service_conversation.go<- save de mensagem + lead-state + handover (A7/M4)
  service_product.go   <- ProductSource plugavel + Sources/SearchCatalog (M5)
  http.go              <- handlers /v1/automation* (RequireAuth; accountID do principal)
  http_models.go       <- handlers GET/PUT /v1/automation/models (A6)
  http_conversation.go <- handlers de runtime de messages + lead-state + handover (A7/M4)
  http_product.go      <- handlers GET/PUT /v1/automation/sources + tool catalog (M5)
  module.go            <- adaptador Module Registry (ID/Metadata/Permissions/Build)
  AGENT.md             <- este arquivo
```

## Tabelas

Migration `0140_automation_schema.sql`:
- `automation.automations` — o "robo" (N por account; V0 = 1 default "Tony" por account).
  `status` draft|active|paused (active = ligado). `settings jsonb` (reservado).
- `automation.channels` — conexao do canal (WAHA session = 1 numero). `provider` pluggable
  (waha|evolution|cloud_api). `session_name` unico global (V0 = "default").

Migration `0141_automation_personas.sql`:
- `automation.personas` — instrucoes de comportamento por automacao;
  `system_prompt`, `is_active` (1 ativa por automacao). Seed default = Tony/Crow.

Migration `0142_automation_knowledge_docs.sql`:
- `automation.knowledge_documents` — documentos de conhecimento (titulo + corpo + sort_order
  + enabled). Concatenados no `systemMessage` do runtime-config apos as instrucoes da persona.
  Indice por `(automation_id, sort_order)`.

Migration `0143_automation_contacts.sql`:
- `automation.contacts` — estado de conversa por chatId (seg, last_msg, last_msg_ts,
  long_memory). Substitui o `staticData.conv` do n8n. **Quando um knowledge doc e deletado
  ou editado, `long_memory` e zerada imediatamente** para todos os contatos da automacao
  (via `ClearLongMemory` no service). UNIQUE `(automation_id, chat_id)`.

Migration `0144_automation_models.sql` (A6):
- `automation.model_catalog` — catalogo GLOBAL (sem account_id) de opcoes de modelo.
  Provider-agnostico (`provider` openai|anthropic|...). PK `(provider, id, kind)`. Flags do
  MODELOS.md: `requires_responses_api`, `accepts_temperature`, `vision_ok`. Seedado na migration
  (OpenAI: gpt-4o*/gpt-4.1/gpt-5.3/gpt-5.5-pro/whisper-1; Anthropic: claude-haiku/sonnet/opus 4.x).
- `automation.automation_models` — modelo escolhido por `(automation_id, role)` (chat|vision|
  audio|classifier). `params jsonb` (temperature etc.). `account_id` FK `core.accounts`, indices
  por account_id/automation_id. `account_id` resolvido do Principal, nunca do body.

Migration `0145_automation_conversation.sql` (A7):
- `automation.messages` — historico (id, automation_id, account_id, contact_id, direction in|out,
  type text|audio|image, content, media_url, segment, created_at). Indices por account_id e por
  `(automation_id, contact_id, created_at)`. Gravada pelo runtime (n8n).
- `automation.lead_state` — estado do lead por `(automation_id, contact_id)` (status do funil,
  last_interaction, follow_up_count). Indice por account_id.

Migration `0148_automation_handover.sql` (M4, idempotente, sem goose):
- `automation.contacts.paused_until timestamptz` — janela de handover humano. Quando
  preenchido e `> now()`, o bot fica em silencio (atendente assumiu). NULL = bot ativo.

Fontes de produto (M5) — **sem migration nova:** reusa `automation.automations.settings jsonb`,
chave `sources` = `{ catalogEnabled bool, siteUrls string[] }`. A tool de catalogo so faz
`SELECT` em `site.products` (modulo site nao e tocado).

## Endpoints (`/v1/automation`, JWT + X-Account-Id)

| Verbo | Path | Acao |
|---|---|---|
| GET | `/v1/automation` | overview: estado do robo + status do WhatsApp (lido da WAHA, persistido) |
| POST | `/v1/automation/whatsapp/connect` | inicia a sessao e devolve o QR (base64); se ja WORKING, retorna conectado |
| POST | `/v1/automation/whatsapp/disconnect` | encerra a sessao |
| PUT | `/v1/automation/settings` | liga/desliga o robo (`{enabled}` -> status active/paused) |
| GET | `/v1/automation/persona` | persona ativa `{ id, name, systemPrompt }` (semeia a default se faltar) |
| PUT | `/v1/automation/persona` | edita `{ name, systemPrompt }` da persona ativa (instrucoes) |
| GET | `/v1/automation/knowledge-docs` | lista documentos de conhecimento da automacao default |
| POST | `/v1/automation/knowledge-docs` | cria documento `{ title, body }` |
| PATCH | `/v1/automation/knowledge-docs/{id}` | edita documento `{ title, body, sortOrder, enabled }` |
| DELETE | `/v1/automation/knowledge-docs/{id}` | remove documento |
| GET | `/v1/automation/models` | `{ catalog[], selection[] }` — catalogo global + escolha por funcao (A6) |
| PUT | `/v1/automation/models` | grava a escolha `{ role, provider, modelId, params }` de uma funcao (A6) |
| GET | `/v1/automation/sources` | `{ catalogEnabled: bool, siteUrls: string[] }` — fontes de produto (M5; default false/[]) |
| PUT | `/v1/automation/sources` | grava `{ catalogEnabled: bool, siteUrls: string[] }` no settings jsonb (M5) |

`account_id` vem do `Principal` (X-Account-Id), nunca do body. O proxy WAHA usa
`AUTOMATION_WAHA_INTERNAL_URL` (default `http://waha:3000`).

**Modelos (A6) — regras do MODELOS.md aplicadas no servico (defesa em profundidade):** o `PUT`
valida `(provider, modelId, role)` contra `model_catalog`; recusa modelo de visao sem `vision_ok`
(`400 invalid_model`); e **remove `temperature` de `params`** quando o modelo nao aceita
(`accepts_temperature=false`, ex.: raciocinio gpt-5*/o-series). O painel ja esconde a temperatura
nesses casos; o back garante. Cada item de `selection`/`catalog` carrega as flags
(`requiresResponsesApi`, `acceptsTemperature`, `visionOk`) para a UI e o n8n. Funcao sem escolha
explicita devolve o default (gpt-4o-mini / whisper-1).

### Runtime (consumido pelo n8n) — M2+

| Verbo | Path | Acao |
|---|---|---|
| GET | `/v1/runtime/automation/config?session=` | `{ enabled, systemMessage, persona, guardrails, docs[], models[] }` — systemMessage = montagem completa (Opcao A); persona/guardrails/docs = partes separadas para montagem dinamica (Opcao B); `models[]` = modelo por funcao com flags (A6) |
| GET | `/v1/runtime/automation/memory?session=&chatId=` | `{ seg, lastMsg, ts, longMem, paused, pausedUntil }` — memoria de conversa do contato; retorna zeros se nao existe. `paused` (bool: `paused_until > now`) e `pausedUntil` (RFC3339 ou vazio) sao o estado de handover (M4) |
| PUT | `/v1/runtime/automation/memory?session=&chatId=` | body `{ seg, lastMsg, ts, longMem }` — upsert da memoria; longMem vazio preserva o valor anterior |
| POST | `/v1/runtime/automation/messages?session=` | body `{ contactId, direction, type, content, mediaUrl, segment }` — grava 1 mensagem (A7); 201 com o registro criado |
| GET | `/v1/runtime/automation/lead-state?session=&contactId=` | `{ contactId, status, lastInteraction, followUpCount }` — estado do lead; defaults (status "new", 0) se nao existe (A7) |
| PUT | `/v1/runtime/automation/lead-state?session=&contactId=` | body `{ status, followUpCount }` — upsert do estado do lead (A7) |
| POST | `/v1/runtime/automation/handover?session=&contactId=` | body `{ pausedMinutes: 30 }` => `paused_until = now()+N min`; `{ resume: true }` (ou `pausedMinutes<=0`) => limpa. Responde a memoria atualizada `{ ..., paused, pausedUntil }` (M4) |
| GET | `/v1/runtime/automation/tools/catalog?session=&q=` | `[{ name, code, price }]` — busca estreita escopada por account em `site.products` (LIMIT 5). Lista vazia se `catalogEnabled=false` ou `q` vazio (M5) |
| GET | `/v1/runtime/omni-chat/catalog?q=` | `{ produtos: [{ name, code, price, brand, image }], total }` — tool de catalogo do **Omni Chat** (Fase 2). Escopo por **context token** no header `X-Omni-Context` (NAO usa `session`). `produtos` vazio se `q` vazio. Ignora o toggle `catalogEnabled` (uso interno). Devolve OBJETO (nao array) p/ o n8n entregar 1 item. **Base = `site.products` (lista+imagem) ENRIQUECIDA pelo ERP** (`public.erp_item_current` por sku==code): nome real, marca e preco (price_cents->reais) vem do ERP porque o `site.products` da Perola veio com nome generico e preco 0. Busca **multi-palavra** (ilike all dos tokens no nome do site + nome ERP + marca). Consumido por um FLUXO MANUAL no n8n (HTTP comum no fluxo principal) — tools nativas do AI Agent estao quebradas no build n8n 2.23.2; ver OMNI_CHAT_PLAN.md |
| GET | `/v1/automation/context-preview` | `{ personaName, instructions, knowledgeDocs[], guardrails, systemMessage }` — previa do bot para o painel (JWT normal, sem sessao) |

**Contrato `models[]` no runtime-config (A6, retrocompativel — campos antigos intactos):** cada item
e `{ role, provider, modelId, label, requiresResponsesApi, acceptsTemperature, visionOk, params }`.
Roles: `chat`, `vision`, `audio`, `classifier`. O n8n usa por expression para escolher o no/modelo
certo e os params (liga Responses API quando `requiresResponsesApi`; so envia `temperature` quando
`acceptsTemperature` e `params.temperature` existe). Funcao sem escolha vem com o default atual.

**Fora do prefixo `/v1/automation`** (de proposito: nao passa pelo `RequireModuleByPath`
nem exige X-Account-Id). Auth por **token de servico**: header `Authorization: Bearer
$AUTOMATION_RUNTIME_TOKEN` (constant-time compare; 401 se invalido, 503 se nao configurado).
A `session` resolve channel -> automacao -> persona ativa (semeia a default se faltar).

**Como o n8n consome (Opcao B — injecao dinamica):** no `Get runtime config` busca a config
com `persona`, `guardrails` e `docs[]`. O no `Montar systemMessage` (code, entre `Bot ligado?`
e o AI Agent) classifica a mensagem por keywords, seleciona os docs relevantes e monta o
`systemMessage` customizado. O AI Agent usa `$('Montar systemMessage').first().json.systemMessage`.
Fallback: se nenhum doc bate, usa todos os habilitados (equivalente a Opcao A).

**Tool de catalogo (M5) — escopo multi-tenant obrigatorio:** o `account_id` da busca em
`site.products` e resolvido pela sessao (`channel -> automacao -> account`), NUNCA do query
do n8n. Query: `... where account_id = $1::uuid and status='active' and is_active=true and
name ilike '%'||$2||'%' limit 5`. Fonte plugavel via interface `ProductSource`
(`siteProductSource` hoje; ERP/catalog futuros sem mexer no handler). O modulo site so e
lido (SELECT), nunca editado.

**Handover humano (M4):** `paused_until` sobrevive a writes de memoria (o upsert de memoria
nao toca a coluna). O n8n le `paused`/`pausedUntil` no inicio (junto da memoria) e nao
responde enquanto a pausa nao expira.

### Omni Chat (M0) — chat interno do painel de Operacao

| Verbo | Path | Acao |
|---|---|---|
| POST | `/v1/omni-chat/ask` | `{ question, topic? }` -> `{ answer, topic? }`. Chat interno ligado ao n8n. RequireAuth; accountID do principal (X-Account-Id) |

**Fora do prefixo `/v1/automation`** (de proposito): o `RequireModuleByPath` usa limite de
segmento (`pathHasSegmentPrefix`), entao `/v1/omni-chat/ask` **nao casa** a regra
`{Prefix:"/v1/automation"}` nem nenhuma outra de `moduleGatingRules()` -> nao exige o modulo
`automation` habilitado (quem usa Operacao nao precisa do painel de automacao). So `RequireAuth`.

**Fluxo (proxy `Front -> API Go -> n8n`):** o handler valida `question` (vazia -> `400
missing_question`; > 2000 chars -> `400 question_too_long`), resolve o `accountID` do principal e
monta o `ContextScope` completo (account + tenant/store/user/role do principal, **nunca** do body)
e chama `OmniChatAsk`. O service monta o `systemMessage` do Tony (persona + docs habilitados +
guardrails, via `buildSystemMessage`, **sem sessao WAHA**), **emite o context token** (Fase 2) e
chama `n8n.Ask` com `{ question, topic, systemMessage, sessionRef: "omni-chat-"+accountID,
contextToken }`. `n8n_client.go` faz `POST $AUTOMATION_N8N_INTERNAL_URL/webhook/omni-chat` com
`Authorization: Bearer $AUTOMATION_RUNTIME_TOKEN` (reusado; sem token novo no M0), `io.LimitReader`
na resposta `{ answer }`. Timeout 60s.

**Erros (contrato congelado, via `httpapi.WriteError`):** `503 omnichat_not_configured`
(`AUTOMATION_N8N_INTERNAL_URL` ou `AUTOMATION_RUNTIME_TOKEN` vazios) · `502 omnichat_error` (n8n
fora / HTTP nao-2xx / JSON invalido) · `504 omnichat_timeout` (`context.DeadlineExceeded`).
`storeId`/`accountId` **nunca** vem do body nem do n8n.

### Omni Chat — Fase 2 (tools de dados)

**Context token (`context_token.go`).** `ContextTokenManager` espelha `auth/tokens.go`: HMAC-SHA256
sobre `base64.RawURLEncoding(json(claims))`, formato `ctxv1.<payload>.<sig>`. Claims:
`{ accountId, tenantId?, storeIds?, userId, role, iat, exp }`. **TTL 300s** (cobre o salto
`/ask` -> webhook n8n -> tool; limita janela de token vazado). **Secret:**
`AUTOMATION_CONTEXT_TOKEN_SECRET` quando setado; senao **reusa `AUTOMATION_RUNTIME_TOKEN`** como
secret HMAC (MVP). `Issue(scope)` assina o escopo do principal; `Parse(token)` valida 3 partes +
prefixo + assinatura (constant-time `hmac.Equal`) + expiracao, retornando `ContextScope`. Qualquer
falha vira o erro generico `ErrInvalidContextToken` (nao vaza o motivo). Sem secret, `Issue`/`Parse`
falham e o chat segue **sem** tools (o token vai vazio; a tool recusa com 401).

**Tool de catalogo (`http_omnichat_tools.go`).** `GET /v1/runtime/omni-chat/catalog?q=`. Duas
camadas de auth: (1) **transporte** = `Authorization: Bearer $AUTOMATION_RUNTIME_TOKEN`
(constant-time via `bearerEquals`; 401 invalido / 503 nao configurado); (2) **escopo** = context
token no header `X-Omni-Context`, validado por `ctxMgr.Parse` (401 `unauthorized` se invalido/
expirado, sem vazar motivo). A busca usa `scope.AccountID` do **token**, NUNCA do query/body. `q`
vazio -> `[]`. Reusa `SearchCatalogByAccount` -> `productSource().Search` (mesmo `SELECT` escopado
por `account_id` em `site.products`, LIMIT 5, projecao `{ name, code, price }`). **Decisao
deliberada:** a tool do Omni Chat **ignora** o toggle `catalogEnabled` das sources — esse toggle
controla o que o bot publico do WhatsApp expoe a clientes externos; o chat **interno** (operadores
do painel) sempre pode consultar o catalogo da propria account.

**Ainda nao implementado (Fase 2):** ranking de vendas e metas. Reusam o mesmo context token
(`X-Omni-Context`) e o mesmo padrao de handler. Contrato: docs/automation/OMNI_CHAT_PLAN.md.

**Memoria de conversa (Postgres, M3+):** o n8n le/escreve `long_memory` via
`GET/PUT /v1/runtime/automation/memory` (mesmo token de servico). Nos workflow:
`Ler memoria` (off Webhook) -> `Ctx: ler` (le do resultado HTTP); `Salvar estado`
(parallel apos `Ctx: aplicar`) salva seg/lastMsg/ts; `Salvar memoria` (apos `Ctx: salvar
resumo`) salva o resumo gerado. Quando um doc e deletado/editado, `ClearLongMemory`
zera `long_memory` de todos os contatos da automacao — sem reimport de workflow.

## Gating / permissoes

- Registrado no Registry em `app.go` (`registry.MustRegister(automation.New())`) — seeda
  `core.modules` + permissoes `automation.view` / `automation.manage` / `automation.whatsapp.manage`.
- Rotas gateadas por `moduleGatingRules()` (`/v1/automation` -> `automation`) via
  `RequireModuleByPath` no Chain. **platform_admin tem bypass** -> admins entram; contas
  sem o modulo habilitado levam `403 module_disabled`. Front: workspace `automation` so em
  `ROLE_WORKSPACES.platform_admin` (permissions.ts), nav `beta`.

## Notas de Deploy

- Migrations `0140_automation_schema.sql` + `0141_automation_personas.sql` (idempotentes) —
  rodam no boot da api. **Rebuild obrigatorio:** `docker compose up -d --build api`.
- Migration `0142_automation_knowledge_docs.sql` (idempotente) — cria tabela + indice; roda
  no boot. **Rebuild obrigatorio:** `docker compose up -d --build api`.
- Migration `0143_automation_contacts.sql` (idempotente) — cria `automation.contacts`; roda
  no boot. **Rebuild obrigatorio:** `docker compose up -d --build api`. Apos o rebuild,
  re-importar o workflow no n8n (ultima vez — a memoria agora vive no Postgres).
- Migration `0144_automation_models.sql` (A6, idempotente) — cria `model_catalog` +
  `automation_models` e seeda o catalogo (ON CONFLICT DO NOTHING). Roda no boot.
  **Rebuild obrigatorio:** `docker compose up -d --build api`.
- Migration `0145_automation_conversation.sql` (A7, idempotente) — cria `messages` +
  `lead_state`. Roda no boot. **Rebuild obrigatorio:** `docker compose up -d --build api`.
- Migration `0148_automation_handover.sql` (M4, idempotente, sem goose) — `ALTER TABLE
  automation.contacts ADD COLUMN IF NOT EXISTS paused_until timestamptz`. Roda no boot.
  **Rebuild obrigatorio:** `docker compose up -d --build api`.
- **M5 sem migration** — config de fontes reusa `automation.automations.settings jsonb`.
- **n8n (M4/M5):** atualizar o workflow para (1) ler `paused`/`pausedUntil` da memoria e
  ficar em silencio enquanto pausado; (2) opcionalmente chamar `POST /handover` quando um
  humano assume; (3) usar a tool `GET /tools/catalog` quando `catalogEnabled`. Mesmo
  `AUTOMATION_RUNTIME_TOKEN`.
- **n8n (A6/A7):** o `runtime-config` agora devolve `models[]` (campos antigos intactos —
  retrocompativel). Para usar a selecao de modelo por expression e gravar conversa, atualizar o
  workflow para ler `models[]` e chamar `POST /messages` + `GET/PUT /lead-state` (mesmo
  `AUTOMATION_RUNTIME_TOKEN`).
- Var `AUTOMATION_WAHA_INTERNAL_URL` (default `http://waha:3000`) — proxy WAHA.
- **Omni Chat (M0):** Var **`AUTOMATION_N8N_INTERNAL_URL`** (default `http://n8n:5678`) — base do
  webhook interno `POST /webhook/omni-chat`. Set em `docker-compose.yml` + `docker-compose.prod.yml`
  (servico `api`) e em `.env.docker.example` / `.env.production.example` / `.env.staging.example`.
  Reusa `AUTOMATION_RUNTIME_TOKEN` no Bearer Go->n8n (sem token novo). Sem URL **ou** sem token ->
  `503 omnichat_not_configured`. **Rebuild obrigatorio da api** (codigo Go novo):
  `docker compose up -d --build api`. **n8n:** importar `automation/export/workflow-omni-chat.json`,
  **ativar** o workflow e restart do n8n (webhook so ouve com workflow Active; path `/webhook/`,
  nao `/webhook-test/`). **Sem migration** no M0 (so leitura do que ja existe).
- Var **`AUTOMATION_RUNTIME_TOKEN`** (M2): a **api E o n8n** usam o MESMO valor (api valida,
  n8n manda no header). Dev tem default que ja bate; prod exige set em `.env.production`
  (gere forte). Sem ele, o runtime-config responde 503.
- **Omni Chat — Fase 2 (context token):** var **opcional** `AUTOMATION_CONTEXT_TOKEN_SECRET` — secret
  HMAC do context token. **Quando ausente, reusa `AUTOMATION_RUNTIME_TOKEN`** como secret (MVP); a
  Fase 2 funciona sem setar a var nova. Em prod, recomenda-se um secret dedicado e forte. So codigo
  Go novo, **sem migration** -> **rebuild obrigatorio da api:** `docker compose up -d --build api`.
  **n8n:** atualizar o workflow do Omni Chat para (1) reenviar o `contextToken` (que ja chega no body
  do webhook) no header `X-Omni-Context` ao chamar a tool; (2) chamar
  `GET /v1/runtime/omni-chat/catalog?q=` (Bearer = `AUTOMATION_RUNTIME_TOKEN`) quando precisar do
  catalogo.
- **Ordem de ativacao do M2:** (1) set do token; (2) `up -d --build api` (sobe modulo +
  migrations); (3) restart do n8n (pega o `$env`); (4) re-importar o workflow
  (`n8n import:workflow`) que agora consome o runtime-config; (5) ativar e testar.

## Gotchas

- `liga/desliga` (M1) so persiste o `status`; o robo so passa a respeitar quando o n8n
  consumir o runtime-config (fase M2). O painel avisa isso na UI.
- WAHA Core suporta 1 sessao -> V0 opera 1 numero (`default`). Multi-numero = P11 (WAHA Plus
  ou Evolution API).
- WAHA tag e `gows-<versao>` (engine GOWS).

## Registro de falhas (debug operacional — anotar aqui pra nao repetir)

> Formato: data — sintoma — causa raiz — como diagnosticar — correcao — prevencao.

### 2026-06-18 — "ativei o n8n mas o bot nao responde no WhatsApp"

- **Sintoma:** sessao WhatsApp conectada, workflow do n8n **Active**, mas mandar mensagem
  nao gera resposta. "Em tese estava funcionando, so tirei a publicacao; religando era pra
  voltar."
- **Causa raiz:** **DOIS interruptores independentes** — religar so um nao basta:
  1. **Workflow Active/Inactive** no proprio n8n (UI do n8n). Esse o usuario religou.
  2. **Toggle liga/desliga do painel `/automation`** -> grava `automation.automations.status`
     (active/paused) -> o `runtime-config` devolve `enabled` -> o no **"Bot ligado?"** do
     workflow corta o fluxo se `enabled=false`. Esse ficou **OFF** (`status=paused`).
  Resultado: a execucao roda `Webhook -> Dados -> Dedupe -> Get runtime config` e
  **termina logo apos** (o gate "Bot ligado?" descarta os itens). Nenhuma resposta sai.
- **Como diagnosticar (passo a passo, sem precisar do n8n UI):**
  1. WAHA conectada? `GET http://localhost:3010/api/sessions` -> `status:"WORKING"`.
  2. WAHA entrega o webhook? `docker logs omni-waha-1` -> `POST ... status code: 200`
     para `http://n8n:5678/webhook/webhook`.
  3. n8n executa e onde para? `docker logs omni-n8n-1 | grep -i "node\|execution"` ->
     ver ate qual no roda. Aqui parou apos `Get runtime config`.
  4. **Confirmacao da causa:** `GET /v1/runtime/automation/config?session=default`
     (header `Authorization: Bearer $AUTOMATION_RUNTIME_TOKEN`) -> **`"enabled": false`**.
- **Correcao:** ligar o robo no painel `/automation` (toggle) OU
  `PUT /v1/automation/settings {"enabled": true}` (JWT + X-Account-Id da conta dona da
  sessao). Isso poe `status=active` -> `runtime-config.enabled=true` -> o gate libera.
- **Prevencao:** lembrar que **"Active no n8n" != "ligado no painel"**. O painel ja avisa
  na UI; o que falta e diagnostico rapido (este registro). Melhoria futura: o painel
  refletir o estado real (Active do n8n + enabled) num so indicador.

### 2026-06-18 (2) — "liguei o robo no painel mas continua sem responder; WhatsApp 'desconectado'"

- **Sintoma:** toggle "Robo ligado" ON numa conta (ex.: Crow Visuals), mas o bot nao responde
  e o painel mostra **"WhatsApp desconectado"** — mesmo com a WAHA `WORKING`.
- **Causa raiz:** **descasamento conta ↔ sessao WAHA.** A unica sessao WAHA conectada
  (`default`, WORKING, com o numero pareado) pertence a **OUTRA** automacao — uma conta
  legacy de smoke-test (`Codex QA Smoke 0606`, `status=draft`). `createChannel` hoje grava
  `session_name = UUID da automacao` (preparado p/ multi-numero), mas a **WAHA Core so roda 1
  sessao**; a automacao legacy pegou o nome `default` e ficou com o numero. Entao
  `runtime-config?session=default` resolve a automacao **legacy (draft → enabled:false)** e o
  gate corta. A automacao que voce ligou (Crow) tem canal `STOPPED` (session_name=UUID que
  nunca conectou) → painel "desconectado".
- **Diagnostico:** `select a.account_id, ac.name, a.status, c.session_name, c.status
  from automation.automations a left join automation.channels c on c.automation_id=a.id
  left join core.accounts ac on ac.id=a.account_id;` e cruzar com WAHA `GET /api/sessions`
  (qual sessao esta WORKING e a quem o `session_name` dela pertence).
- **Correcao (local):** re-bind do canal — dar o `session_name='default'` para a automacao que
  se quer testar (liberando a legacy antes, por causa do UNIQUE). Sem re-scan de QR (a sessao
  WAHA `default` segue a mesma). Alternativa: ligar a automacao **dona** da sessao conectada.
- **Prevencao / divida tecnica:** `session_name = UUID` por automacao e **incompativel com
  WAHA Core (1 sessao)** — so 1 automacao por instancia consegue conectar de fato. Falta
  regra/UI de "qual automacao possui a sessao unica" + limpar contas de teste legacy que
  seguram o `default`. Multi-numero real = P11 (WAHA Plus/Evolution).

### 2026-06-18 (3) — workflow n8n: "Bot ligado?" erra com "Node 'Get runtime config' hasn't been executed"

- **Sintoma:** com o robo ligado (`enabled:true`) o bot **ainda nao responde**; a execucao do
  n8n vai longe (passa o debounce, Ctx:*) e **falha no no "Bot ligado?"** com
  `ExpressionError: Node 'Get runtime config' hasn't been executed`.
- **Causa raiz:** **"Get runtime config" (HTTP) era um ramo PARALELO sem saida** —
  `Webhook → Get runtime config` com `outgoing = []` (beco). Os nos "Bot ligado?" e
  "Montar systemMessage" liam `$('Get runtime config')`, mas o debounce tem nos **Wait**; ao
  **retomar a execucao pos-Wait**, o n8n so da acesso via `$('node')` a nos **ancestrais do
  caminho retomado** — ramo paralelo sem ligacao pra frente nao conta -> erro.
- **Diagnostico:** `docker logs omni-n8n-1` mostrou a execucao parando no "Bot ligado?" com o
  erro; analise do JSON confirmou `Get runtime config` com `outgoing=[]` e **nao-ancestral**
  de "Bot ligado?".
- **Correcao (`automation/export/workflow-whatsapp.json`):** mover "Get runtime config" pro
  caminho principal -> `Ctx: aplicar → Get runtime config → Bot ligado?`. A URL passou a ler a
  sessao de `$('Dados').first().json.session` (ancestral on-path, sobrevive ao Wait). Como o
  HTTP node substitui o item, o "Bot ligado?" passou a repassar a msg via
  `$('Ctx: aplicar').all()`. Re-import + **restart do n8n** (re-registra o webhook ativo).
  Backup do JSON em `.bak-*`.
- **MESMO bug tambem no "Ler memoria"** (outro HTTP off-Webhook, consumido por "Ctx: ler"
  pos-Wait): movido para `Juntar → Ler memoria → Ctx: ler`; URL le `session`/`chatId` de
  `$('Dados')`; "Ctx: ler" passou a ler a msg de `$('Juntar')` (input agora e' a resposta do
  Ler memoria). Apos os 2 fixes, o fan-out do Webhook ficou so `Webhook → Dados`.
- **Como achar todos de uma vez:** varrer o JSON por `$('X')` e checar se `X` e' ancestral do
  no (script ad-hoc). Sobra so `Redis Chat Memory → $('Dados')`, que e' **OK** (sub-no do AI
  Agent; `Dados` e' ancestral do AI Agent e resolve em runtime).
- **Prevencao:** no consumido por `$('X')` **depois de um Wait/debounce PRECISA estar no
  caminho principal** (ancestral), nunca em ramo paralelo sem saida. **No DEPLOY VPS:**
  re-importar este workflow no n8n da VPS (mesmo fix) + restart do container n8n.
