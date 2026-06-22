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

- **Omni Chat — persona dedicada + catalogo DESCONECTADO (2026-06-19, decisao do usuario):** o Omni Chat
  NAO usa mais o Tony nem consulta o catalogo. Usa uma **persona propria** (`defaults/omni_chat_persona.md`,
  embed `omniChatPersona`): copiloto de vendas/conhecimento da **Perola Joias** (joias, relogios Seiko/
  Bulova/Victorinox, concorrentes, persuasao). Workflow n8n simplificado p/ `Webhook -> AI Agent -> Respond`.
  O catalogo (codigo Go + nos do n8n) fica intacto, so desconectado — ver registro de falhas "2026-06-19 (3)".
  A descricao de Fase 2 abaixo (context token + tool de catalogo) segue valida como infra, mas o workflow
  atual NAO a chama.

- **Omni Chat — Fase 2 (tools de dados) [infra, hoje nao usada pelo workflow]:** o `/ask` ainda emite um **context token HMAC** opaco
  (`ctxv1`) que escopa as tools de dados; o n8n o reenvia no header `X-Omni-Context` ao chamar
  uma tool. Primeira tool: **catalogo** (`GET /v1/runtime/omni-chat/catalog`). O escopo
  (account/tenant/store/user/role) sai SO do token assinado, NUNCA do query/body do n8n. Tools de
  ranking/metas entram a seguir (reusam o mesmo token). Detalhe abaixo em "Omni Chat — Fase 2".
  Resposta do `/ask`: `{ answer, topic?, products? }` — o n8n inclui no Respond o resultado da tool
  de catalogo (`products[]`) e o Go faz pass-through (`OmniChatResultView.Products`) p/ o front
  renderizar **cards com imagem** (a imagem e' o path `/uploads/...` servido pela api). O n8n consome
  a tool por um FLUXO MANUAL (HTTP comum no fluxo principal: extrai termo -> busca -> compoe), porque
  as tools nativas do AI Agent estao quebradas no build n8n 2.23.2 (Tools Agent V3). Ver OMNI_CHAT_PLAN.md.

Proximas fases (BYOK, multi-numero, CRM, tools de ranking/metas) em PLATAFORMA_AUTOMACAO.md (P6+).

## Estrutura

```
back/internal/modules/automation/
  model.go             <- Automation, Channel, Persona, KnowledgeDoc e *View
  models.go            <- catalogo de modelos + selecao por funcao (A6) e *View
  model_conversation.go<- Message, LeadState e *View (A7)
  model_product.go     <- SourcesView, ProductHit (M5)
  waha_client.go       <- cliente HTTP interno da WAHA (status/start/logout/qr); start usa
                          POST /api/sessions/{name}/start (compativel com a engine gows), nao o
                          atalho POST /api/sessions {start:true}; disconnect usa /logout (libera o
                          numero pareado p/ trocar de numero), nao /stop
  store_postgres.go    <- persistencia (automations/channels/personas/knowledge_documents/contacts)
  store_models.go      <- persistencia do catalogo + selecao de modelos (A6)
  store_conversation.go<- persistencia de messages + lead_state (A7)
  store_product.go     <- settings.sources jsonb + SELECT escopado em site.products (M5)
  store_omnichat.go    <- settings.omniChatPersona jsonb (Get/Set da persona editavel do Omni Chat)
  service.go           <- orquestra store + WAHA + n8n; buildSystemMessage (instrucoes+docs+guardrails)
  n8n_client.go        <- cliente HTTP do webhook interno do Omni Chat (POST /webhook/omni-chat) (M0)
  service_omnichat.go  <- OmniChatAsk (systemMessage = persona efetiva do banco) + Get/SetOmniChatPersona + emite context token + chama n8n.Ask
  http_omnichat.go     <- handlers POST /v1/omni-chat/ask + GET/PUT /v1/omni-chat/persona (RequireAuth; fora do gating)
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
| POST | `/v1/automation/whatsapp/disconnect` | **logout** da sessao (libera o numero pareado p/ trocar de numero) |
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

**Sessao fisica unica (WAHA Core):** a WAHA Core so aceita a sessao **`default`** (1 numero por
instancia) — qualquer outro nome devolve **422** ("WAHA Core support only 'default' session ...
get WAHA PLUS"). Por isso todas as chamadas WAHA (Status/Start/Logout/QR) usam a sessao de
`AUTOMATION_WAHA_SESSION` (default `default`), e nao o `session_name` por-conta do canal. Ou seja:
**1 numero de WhatsApp compartilhado** entre as contas. Trocar de numero = `disconnect` (logout) +
`connect` (novo QR). Multi-numero real (1 sessao por conta) exige **WAHA Plus**; nesse caso setar
`AUTOMATION_WAHA_SESSION=@channel` faz o service voltar a usar o `session_name` do canal.

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
| GET | `/v1/runtime/omni-chat/catalog?q=` | `{ produtos: [{ name, code, price, brand, image }], total, mode }` — tool de catalogo do **Omni Chat** (Fase 2). Escopo por **context token** no header `X-Omni-Context` (NAO usa `session`). Ignora o toggle `catalogEnabled` (uso interno). **3 intencoes (`OmniChatCatalog`):** `q` vazio (NONE) -> `mode=empty`, `produtos=[]`; **`q="LISTAR"`** (sentinel do extrator p/ pedido generico/sugestao) -> **amostra real** do catalogo (`SampleSiteProducts`), `mode=sample`; `q=<termo>` -> busca especifica, e se **0 resultados** cai numa amostra como sugestao, `mode=suggestion` (bot nunca trava nem fica "burro"). `mode=match` quando a busca acha. Devolve OBJETO (nao array) p/ o n8n entregar 1 item. **Base = `site.products` (lista+imagem) ENRIQUECIDA pelo ERP** (`queue.erp_item_current` por sku==`split_part(code,'_',1)`, code multi-parte; cobre ~511/773): nome real, marca e preco (price_cents->reais) vem do ERP porque o `site.products` da Perola veio com nome generico e preco 0. Indice `(tenant_id,sku)` (migration **0165**) deixa o enrich rapido (~60ms vs ~8s). Marca puramente numerica (cod. de loja) escondida; produtos duplicados deduplicados por nome. Busca **multi-palavra** (ilike all dos tokens no nome do site + nome ERP + marca). Consumido por um FLUXO MANUAL no n8n (HTTP comum no fluxo principal) — tools nativas do AI Agent estao quebradas no build n8n 2.23.2; ver OMNI_CHAT_PLAN.md |
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
| GET | `/v1/omni-chat/persona` | `{ systemPrompt, isDefault }` — persona EFETIVA da account: custom salvo no banco, ou o embed `omniChatPersona` como default (`isDefault=true`). RequireAuth; accountID do principal |
| PUT | `/v1/omni-chat/persona` | body `{ systemPrompt }` -> `{ systemPrompt, isDefault:false }`. Salva o custom (passa a valer). `400 empty_prompt` (vazio apos trim) · `400 prompt_too_long` (> 20000 chars). RequireAuth; accountID do principal |

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

**Persona do Omni Chat editavel (banco; embed = default) — sem migration.** A persona deixou de ser
fixa no embed: agora vive no `automation.automations.settings jsonb`, chave **`omniChatPersona`**
(valor string), na automacao default da account (`GetOrCreateDefault`) — **mesmo padrao das sources
(M5)**, sem migration nova. O embed `omniChatPersona` (`defaults/omni_chat_persona.md`) vira o
**DEFAULT/fallback**: vale enquanto a conta nunca salvou um custom.
- `GET /v1/omni-chat/persona` -> `{ systemPrompt, isDefault }`: `systemPrompt` = prompt EFETIVO da
  account (custom salvo, senao o embed); `isDefault=true` quando ainda e o embed, `false` quando ja
  ha custom salvo.
- `PUT /v1/omni-chat/persona` body `{ systemPrompt }` -> `{ systemPrompt, isDefault:false }`: salva o
  custom (trim) e passa a valer. `400 empty_prompt` (vazio apos trim) · `400 prompt_too_long`
  (> 20000 chars). `MaxBytesReader` 64KB.
- **`OmniChatAsk` usa o prompt EFETIVO** (`OmniChatPersona(scope.AccountID)`) como `systemMessage`,
  nao mais o embed verbatim. Store: `GetOmniChatPersonaSetting`/`SetOmniChatPersonaSetting`
  (store_omnichat.go) espelham `GetSources`/`SetSources`. Ambas as rotas sao `RequireAuth`, **fora**
  do prefixo `/v1/automation` (sem gating de modulo, como `/v1/omni-chat/ask`); `accountId` vem do
  principal (X-Account-Id), nunca do body.

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
- Var **`AUTOMATION_WAHA_SESSION`** (default `default`) — sessao fisica usada nas chamadas WAHA
  (Status/Start/Logout/QR). A WAHA Core so aceita `default`; `@channel` volta ao modo 1-sessao-por-conta
  (so com WAHA Plus). Set nos 2 composes (servico `api`) + `.env.docker.example` /
  `.env.production.example`. So muda comportamento; **rebuild da api** ao alterar o codigo Go.
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
- WAHA Core suporta **so a sessao `default`** (1 numero por instancia); qualquer outro nome ->
  **422**. Por isso o service usa `AUTOMATION_WAHA_SESSION` (default `default`) em TODAS as chamadas
  WAHA, nao o `session_name` por-conta. Multi-numero = P11 (WAHA Plus, `AUTOMATION_WAHA_SESSION=@channel`).
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

### 2026-06-19 — painel "Conectar" do WhatsApp da 502 (local e VPS)

- **Sintoma:** clicar **Conectar** no painel `/automation` retorna 502 ("Nao foi possivel falar
  com o WhatsApp (WAHA)"). Reproduzido em dev e na VPS.
- **Causa raiz:** o `WAHAClient.Start` usava o atalho **`POST /api/sessions {name, start:true}`**
  (criar+iniciar num passo). A engine **gows** (`devlikeapro/waha:gows-2026.5.1`) **recusa** esse
  atalho quando a sessao **ja existe** (estado STOPPED apos um deploy/restart), devolvendo erro que
  o Go propagava como 502. Curl confirmou: `POST /api/sessions {start:true}` numa sessao existente
  falha; `POST /api/sessions/{name}/start` numa sessao existente = **201**.
- **Diagnostico:** `curl` direto na WAHA interna comparando os dois caminhos; o `/{name}/start`
  responde 201, o atalho falha.
- **Correcao (`waha_client.go`):** `Start` agora tenta **`POST /api/sessions/{name}/start`**; se a
  sessao nao existe (**404**), cria com `POST /api/sessions {name}` e re-inicia. Idempotente: 4xx em
  sessao ja rodando e' tolerado, so 5xx vira erro. Rebuild da api (`docker compose up -d --build api`)
  local + deploy `prod:deploy:vps:inc -Services api` na VPS.
- **Prevencao:** preferir o endpoint **especifico por acao** da WAHA (`/{name}/start`, `/{name}/stop`)
  ao atalho combinado; o comportamento do atalho varia entre engines/versoes da WAHA.
- **Causa raiz 2 (a que pegava a Perola — mais profunda):** o fix do start NAO resolveu sozinho. O
  canal de cada conta guarda `session_name = UUID da automacao`, mas a **WAHA Core so aceita a sessao
  `default`**. Toda chamada para a sessao da Perola (`ce2b406f-...`) devolvia **422** ("WAHA Core
  support only 'default' session ... get WAHA PLUS"). O fluxo `Connect`: `Status(UUID)` 422 -> erro ->
  `liveStatus` cai no status persistido (STOPPED) -> guard "ja WORKING" nao dispara -> `Start(UUID)` 422
  (tolerado, <500) -> **`QR(UUID)` 422 -> erro -> 502**. So a conta com `session_name='default'` (Crow)
  conectava; Perola/Codex (UUID) batiam no 422.
- **Diagnostico:** `GET /api/sessions?all=true` na WAHA mostrou **so a sessao `default`** (WORKING,
  554284138129); `POST /api/sessions/{UUID}/start` -> 422 com a mensagem do WAHA Core. `select` em
  `automation.channels` confirmou os `session_name` UUID por conta.
- **Correcao (sessao fisica unica):** `Service` passou a usar **`AUTOMATION_WAHA_SESSION`** (default
  `default`) em TODAS as chamadas WAHA (helper `sessionName(ch)`), em vez do `session_name` por-conta.
  Assim toda conta opera a sessao unica `default` (1 numero compartilhado — limite do WAHA Core).
  `@channel` reativa o modo por-conta (so com WAHA Plus). Alem disso, **`disconnect` agora faz `logout`**
  (nao `stop`): libera o numero pareado, permitindo "desconectar e conectar outro numero" (escanear
  novo QR). O handler tambem passou a **logar o erro real** (`slog.Error`) — antes o 502 era cego.
- **Prevencao:** com WAHA Core, nunca usar nome de sessao != `default`; o painel opera 1 numero so.
  Multi-numero por conta = WAHA Plus (P11) + `AUTOMATION_WAHA_SESSION=@channel`.

### 2026-06-19 — QR aparece no painel mas o scan falha ("nao foi possivel conectar o dispositivo")

- **Sintoma:** apos os fixes acima o QR **aparecia** no painel (dev e prod), mas ao escanear a
  WhatsApp dava *"nao foi possivel conectar o dispositivo"*. Na prod aparecia so depois do self-heal.
- **Causa raiz:** a **WAHA rotaciona o QR a cada ~20s** (gera um QR novo; o anterior expira). O
  front (`web/.../useAutomation.ts`) buscava o QR **uma unica vez** no clique do Conectar e exibia
  essa imagem **estatica**; o `startPolling` so checava status, **nao re-buscava o QR**. Passados ~20s
  o QR da tela ficava vencido — o usuario escaneava um QR velho que a WAHA ja descartou -> pareamento
  falha. Logs da WAHA: `Failed QR item event {Event:"timeout"}` + `websocket ... EOF` a cada ~20s, com
  QR novo sendo impresso no intervalo, sem nenhum `pair-success`.
- **Diagnostico:** `docker logs ...waha... | grep -i "qr\|pair\|fail"` mostrou a cadencia de ~20s de
  QRs novos e o timeout sem pareamento. Relogio da VPS estava OK (NTP), descartando skew de tempo.
- **Correcao (`web/app/composables/useAutomation.ts`, SO front):** `startPolling` passou a **re-buscar
  o QR a cada poll (3s)** via `refreshQr()` (reusa o `POST /connect`, que ainda recupera se a sessao
  cair p/ FAILED), atualizando `qr` **so quando muda** (sem flicker). Assim a tela sempre mostra o QR
  corrente da rotacao. Detecta WORKING e para o polling.
- **Prevencao:** QR de pareamento e' efemero — nunca exibir como imagem estatica; re-buscar mais rapido
  que a rotacao (~20s). Se o scan falhar mesmo com QR fresco, suspeitar de bloqueio do IP da VPS pela
  WhatsApp (datacenter) — ai a saida e' proxy residencial ou outro numero.

### 2026-06-19 — Omni Chat: "Buscar catalogo" da "Authorization failed" e TRAVA todas as respostas

- **Sintoma:** no n8n, o no **"Buscar catalogo"** falha com *"Authorization failed - please check your
  credentials"* (GET `http://api:8080/v1/runtime/omni-chat/catalog`). Pior: quando ele falha, **a
  execucao inteira para** — "AI Agent" (compositor) e "Respond to Webhook" nunca rodam, o webhook nao
  responde e o `/v1/omni-chat/ask` do Go estoura timeout -> o chat **congela** (nenhuma resposta).
  Quando o catalogo voltava VAZIO, o bot so dizia "nao encontrei" (burro).
- **Causa raiz:** o node manda `X-Omni-Context: {{ $('Webhook').first().json.body.contextToken }}`. O
  context token tem **TTL 300s** e so e emitido pelo `/ask` do Go. Em teste "Debug in editor" (sem Go) ele
  vem **vazio**; num replay de execucao antiga vem **expirado** -> `ctxMgr.Parse` rejeita -> tool **401**
  -> o HTTP node erra e, com `onError` no default (`stopWorkflow`), **mata a execucao**. (Mesmo B.O. #4 do
  OMNI_CHAT_PLAN.md, mas o impacto real e o congelamento, nao so o 401.)
- **Diagnostico:** bateria `c:\tmp\test_omnichat_resilience.py` (POST direto no webhook). Token vazio/
  invalido -> resposta **vazia** (JSONDecodeError) = freeze comprovado; catalogo vazio (conta zero) -> 200
  com "nao encontrei".
- **Correcao (`automation/export/workflow-omni-chat.json`, SO n8n — sem mexer no Go):**
  1. **"Buscar catalogo" com `"onError": "continueRegularOutput"`** — qualquer falha do catalogo (401
     token vazio/expirado, timeout, 5xx, rede) NAO derruba mais o fluxo: segue pro compositor com
     `produtos` vazio e o webhook **sempre** responde.
  2. **Prompt do compositor "AI Agent" reescrito** — leitura defensiva `JSON.stringify($json.produtos || [])`
     (item de erro vira `[]`) e instrucao: **nunca** travar/responder vazio; se nao achar o item, dizer em 1
     frase que nao achou e **sugerir alternativas relacionadas (categorias/marcas/parecidos) + 1 pergunta
     pra refinar**, em vez de so "nao encontrei".
- **Verificado (e2e local):** token vazio/invalido (catalogo 401) -> responde com sugestao (Casio/Orient/
  Citizen) **sem travar**; catalogo vazio -> sugere alternativa; pergunta normal -> normal; Perola -> 5
  produtos reais (sem regressao).
- **Prevencao:** todo HTTP node de tool no fluxo principal do Omni Chat deve ter `onError:
  continueRegularOutput` (a tool e best-effort; a resposta do chat nao pode depender dela). O compositor
  sempre recebe `produtos` defensivo. **Sem migration, sem rebuild da api** (so o workflow do n8n). **No
  DEPLOY VPS:** re-importar `workflow-omni-chat.json` no n8n da VPS + `update:workflow --active=true` +
  restart do container n8n (mesmo fix). Import via Git Bash no Windows exige **`MSYS_NO_PATHCONV=1`** senao
  o `--input=/tmp/...` vira path Windows (`C:/Users/.../Temp/...`) e da ENOENT silencioso.

### 2026-06-19 (2) — Omni Chat: catalogo volta VAZIO para pedido generico/sugestao (bot "burro")

- **Sintoma (no navegador, conta Perola):** "Lista produtos do catalogo", "me sugere uma joia",
  "produtos para presente" -> **0 produtos**, bot diz "nao encontrei". Ate "relogio seiko" parecia
  vazio. O usuario: "ainda nao consegue acessar o catalogo".
- **Causa raiz (reconstruida das execucoes do n8n — SQLite `database.sqlite`, formato *flatted*):**
  o problema NAO era auth (catalogo respondia **200**) nem a conta (token carregava a Perola
  `aaaaaaaa-...` correta). Era o **extrator + a busca literal**:
  1. O **extrator** classificava "Lista produtos do catalogo" como **NONE** (so reconhecia produto
     ESPECIFICO) -> `q` vazio -> 0.
  2. Para pedidos genericos ele gerava termos que **nao casam com nome nenhum** ("presente",
     "perfume") -> a busca e `ilike` LITERAL multi-palavra no `nome_site + nome_ERP + marca`, e em
     `site.products` da Perola o nome e' o CODIGO (`368145_...`) — o nome real vem do ERP, e nenhum
     produto se chama "joia"/"produto"/"presente". Joalheria: tudo e' joia, mas nada se *chama* joia.
  - (O "relogio seiko" vazio que o usuario viu caiu na conta **Crow Visuals** `80caf5d5` — 0 produtos;
    so a Perola tem os 773. Platform view escopa na agencia Crow -> tambem 0.)
- **Diagnostico:** reconstruir a execucao do n8n (copiar `database.sqlite` + `-wal`, parser flatted em
  Python) revelou `question -> Extrair termo.output -> Buscar catalogo.produtos` reais. Ex.: "Lista
  produtos do catalogo" -> output **NONE** -> 0; "produtos para presente" -> "presente" -> 0; "me lista
  2 relogios seiko" -> "relogio seiko" -> **5** (busca especifica SEMPRE funcionou).
- **Correcao (Go + n8n):**
  1. **Go `OmniChatCatalog` (service_product.go) com 3 intencoes + fallback:** `q` vazio (NONE) ->
     `mode=empty`; **`q="LISTAR"`** (sentinel) -> **amostra real** do catalogo (`SampleSiteProducts`
     em store_product.go: mesma enrich/dedup, so itens com `price>0` + imagem, ordem **aleatoria
     (`random()`)** p/ VARIAR a cada chamada — senao o bot repetia sempre os mesmos 5) `mode=sample`;
     `q=<termo>` -> busca, e se **0 resultados** cai numa
     **amostra como sugestao** `mode=suggestion` (bot sugere "outra coisa que tenha a ver" com produtos
     REAIS, sem inventar). A tool passou a devolver `{ produtos, total, mode }`.
  2. **n8n extrator 3-way:** NONE (nao-produto) / **LISTAR** (ver/listar/sugerir sem produto concreto:
     "lista produtos", "me sugere uma joia", "produtos para presente") / termo especifico.
  3. **n8n compositor usa `mode`:** match/sample -> "Veja algumas opcoes do catalogo:"; suggestion ->
     "Nao achei isso exato, mas separei estas opcoes:"; vazio -> sugere/pergunta; NONE -> responde normal.
- **Verificado (e2e local, Perola):** "Lista produtos"/"me sugere uma joia"/"presente" -> **5 produtos**;
  "relogio seiko" (com e sem acento) -> match 5; "tem rolex?" -> "Nao achei Rolex, mas separei estas
  opcoes de joias" + 5 cards; "bom dia" -> normal, 0 produtos. (`c:\tmp\test_catalog_modes.py` direto +
  `c:\tmp\test_omnichat_browse.py` e2e.)
- **Prevencao / nota de produto:** os 773 produtos estao SO na **Perola**; **Crow Visuals e platform
  view tem 0** -> precisa Perola ativa pra ver catalogo. Acento ja funciona (nome do site tem a forma
  acentuada). **Rebuild da api obrigatorio** (Go novo) + **reimport do workflow** no n8n. **Sem migration.**

### 2026-06-19 (3) — Omni Chat: persona DEDICADA (Perola Joias) + catalogo DESCONECTADO (decisao do usuario)

- **Decisao do usuario:** desconectar o catalogo "por enquanto" e parar de usar o **Tony** no Omni Chat.
  O chat passa a ser **so GPT + o conhecimento do prompt** (sem consultar produtos). Nova persona:
  um **copiloto de vendas/conhecimento da Perola Joias** (joalheria; ouro 18k/prata, relogios Seiko/
  Bulova/Victorinox; concorrentes Vivara/Lea Pain; mercado de luxo; tecnicas de venda/persuasao). Fala
  com a EQUIPE (interno), nao com o cliente final.
- **O que mudou:**
  1. **Persona dedicada** em `defaults/omni_chat_persona.md`, embed `omniChatPersona` (defaults.go).
     Independente do Tony (`defaultPersona`, ainda usado pelo WhatsApp). Sem guardrails de WhatsApp.
  2. **`OmniChatAsk` (service_omnichat.go)** usa `omniChatPersona` verbatim como systemMessage — NAO
     consulta mais banco/persona (dropou GetOrCreateDefault/ensurePersona/ListKnowledgeDocs/
     buildSystemMessage). Mantem o `contextToken` (inofensivo) p/ reconectar tools depois sem retrabalho.
  3. **Workflow n8n simplificado:** `Webhook -> AI Agent -> Respond` (removidos "Extrair termo",
     "Buscar catalogo", "OpenAI Model (extrator)"). Respond devolve `{ answer, topic }` (sem products).
- **Catalogo nao foi removido — so desconectado.** O codigo Go (`OmniChatCatalog`, `SampleSiteProducts`,
  `GET /v1/runtime/omni-chat/catalog`) fica intacto; reconectar = voltar os nos no workflow + ajustar a
  persona. O front (`useOmniChat`) ja trata products ausente (renderiza sem cards).
- **Limitacao honesta:** o AI Agent deste build (n8n 2.23.2) **nao navega na web ao vivo** (tools nativas
  quebradas). "Pesquisar na net" = conhecimento do modelo + o que esta no prompt. A persona instrui a
  **nao inventar** preco/estoque/datas/fatos atuais de concorrente (dizer "confirmar na fonte").
- **Verificado (e2e, `c:\tmp\test_omnichat_persona.py`):** Seiko Prospex x Presage; vender Bulova mais
  caro; diferenciar da Vivara; presente de formatura; 4 Cs do diamante; saudacao — todas com resposta
  util de vendas/conhecimento, **sem citar Tony e sem products**.
- **Deploy:** **rebuild da api** (Go novo: embed da persona) + **reimport do workflow** no n8n. **Sem
  migration.** (Persona editavel resolvida em 2026-06-19 (4): vive no `settings jsonb`, embed = default.)

### 2026-06-19 (4) — Omni Chat: persona EDITAVEL no banco (embed = default/fallback)

- **Mudanca:** a persona do Omni Chat deixou de ser fixa no embed. Agora e guardada no banco
  (`automation.automations.settings jsonb`, chave `omniChatPersona`, na automacao default da account
  via `GetOrCreateDefault`) e **editavel** por `GET/PUT /v1/omni-chat/persona`. O embed
  `omniChatPersona` (`defaults/omni_chat_persona.md`) vira o **DEFAULT/fallback** — vale ate a conta
  salvar um custom.
- **Padrao reusado:** identico ao Sources (M5) — `GetOmniChatPersonaSetting` (`settings ->> $2`) e
  `SetOmniChatPersonaSetting` (`jsonb_set` + `to_jsonb($prompt::text)` + `updated_at=now()`) em
  store_omnichat.go; service `OmniChatPersona`/`SetOmniChatPersona`. `OmniChatAsk` passou a montar o
  `systemMessage` com o prompt EFETIVO (`OmniChatPersona`), nao mais o embed verbatim.
- **Contrato (congelado, front depende):** `GET` -> `{ systemPrompt, isDefault }`; `PUT` `{ systemPrompt }`
  -> `{ systemPrompt, isDefault:false }`; `400 empty_prompt` (vazio apos trim) · `400 prompt_too_long`
  (> 20000 chars). RequireAuth, fora do prefixo `/v1/automation` (sem gating), accountId do principal.
- **Deploy:** **rebuild da api** (Go novo). **Sem migration.** (Nao toca o workflow do n8n.)

### 2026-06-19 (5) — Omni Chat: memoria de conversa (node de memoria no n8n) + janela configuravel

- **Mudanca:** o chat passou a SEGUIR a conversa (ultimas N interacoes, INCLUSIVE as respostas da propria
  IA). **De-risk:** node nativo de memoria FUNCIONA neste build (2.23.2) — as TOOLS quebram (exigem
  supplyData+execute), mas memoria usa so `supplyData` (igual o chat model que funciona). Provado em teste
  de 3 turnos: a IA lembrou o nome do usuario E um numero que ela mesma gerou.
- **n8n (workflow-omni-chat.json):** node **Window Buffer Memory** (`@n8n/n8n-nodes-langchain.memoryBufferWindow`,
  typeVersion 1.3) ligado ao AI Agent por `ai_memory`. `sessionKey={{ $('Webhook').first().json.body.sessionKey }}`,
  `contextWindowLength={{ Number($('Webhook').first().json.body.historyWindow) || 5 }}`.
- **Go:** `OmniChatAsk(ctx, scope, question, topic, conversationID)` agora manda `sessionKey` + `historyWindow`
  ao n8n (`OmniChatRunRequest`). **sessionKey escopado** = `accountID|userID|conversationId` (`omniChatSessionKey`),
  isola memoria entre operadores (nunca por accountID puro). Janela no `settings` jsonb (chave
  `omniChatHistoryWindow`, default 5, clamp 1..20; `Get/SetOmniChatHistoryWindow` em store_omnichat.go).
  **`OmniChatPersona`/`SetOmniChatPersona` foram substituidos por `OmniChatConfig`/`SetOmniChatConfig`**
  (leem/gravam persona + janela numa so resolucao de automacao default).
- **Contrato (aditivo, retrocompativel):** `GET/PUT /v1/omni-chat/persona` agora tambem carrega
  `historyWindow`; `POST /v1/omni-chat/ask` aceita `conversationId` (escopa a memoria; nunca confiado p/
  escopo de tenant — so compoe o sessionKey).
- **Verificado (local):** memoria + janela dinamica via webhook (`c:\tmp\test_memory.py`); go build/vet;
  SQL da janela (rollback); eslint. UI final = navegador.
- **Deploy:** **rebuild da api** + **reimport do workflow** (node de memoria novo) + restart n8n. **Sem migration.**

### 2026-06-19 (6) — VPS: bot do WhatsApp nao responde — 4 problemas EMPILHADOS (node + 2 credenciais de DEV + workflow antigo)

- **Sintoma:** na VPS, WhatsApp conectado (WAHA WORKING) e robo ligado no painel, mas mandar
  mensagem **nao gera resposta** nenhuma. Cada camada que eu corrigia revelava a proxima.
- **Causa raiz (4 problemas em serie):**
  1. **Node community `n8n-nodes-waha` ausente** no volume `~/.n8n/nodes` (so registro fantasma na
     tabela `installed_packages` do SQLite, **sem arquivos** em `node_modules`; a UI mostrava
     "instalado" porque le do banco, nao do disco). Boot do n8n: `Unrecognized node type:
     n8n-nodes-waha.WAHA` -> workflow "Whatsapp" nunca ativava (so o banco marcava active) ->
     webhook nao registrava -> WAHA tomava **404** em todos os POSTs.
  2. **Credencial Redis** (`Redis account`, id `pkxksfWdwYDbv6B3`) com config de **DEV**:
     `host.docker.internal:6380` + senha curta. ioredis fica em retry infinito de auth ->
     execucoes travavam em `running` (`waitTill` NULL) no 1o no Redis ("Fila push"), **sem erro
     no log**. Sintoma confirmavel: `redis-cli DBSIZE` = 0 apesar de N mensagens entregues.
  3. **Workflow era a versao ANTIGA** (sem o fix de 2026-06-18(3)): "Ler memoria"/"Get runtime
     config" pendurados como becos do Webhook -> erro `Node 'Ler memoria' hasn't been executed`
     no no "Ctx: ler" ao retomar pos-Wait.
  4. **Credencial WAHA** (`WAHA account`, id `OeshCwyq7C4bSPAo`) com config de **DEV**:
     `http://host.docker.internal:3010` -> "Send a text message"/"Send Seen" (envio da resposta)
     falharia. (OpenAI `sCzmqFisO8bdeZ9B` estava OK: key `sk-proj` 164 chars.)
- **Como diagnosticar (so SQLite + curl, sem a UI):** o n8n usa **SQLite** `~/.n8n/database.sqlite`
  (NAO tem better-sqlite3; ler/escrever com `node --experimental-sqlite` via `node:sqlite`).
  `execution_entity` (status/`waitTill`) + `execution_data` (formato **flatted** — `require` o
  pacote `flatted` do n8n p/ ler `resultData.runData`/`error`) mostram em que no a execucao parou.
  Credenciais: descriptografar/encriptar com `CipherAes256CBC` de
  `n8n-core/dist/encryption/aes-256-cbc.js` (OpenSSL `Salted__`; so precisa data+key; key de
  `N8N_ENCRYPTION_KEY` ou `~/.n8n/config`). Webhook registrado? `curl localhost:15680/webhook/webhook`
  -> `"not registered for GET"` = POST ativo; `"...is not registered"` seco = workflow inativo.
- **Correcao (tudo no container `listaatendimento-n8n-1`):** (1) `npm install n8n-nodes-waha` em
  `~/.n8n/nodes`; (2/4) UPDATE das credenciais no SQLite p/ `redis:6379` + senha
  `AUTOMATION_REDIS_PASSWORD` e `http://waha:3000` (reencriptar com o MESMO Cipher); (3)
  `n8n import:workflow --input=workflow-whatsapp.json` (o id do workflow `lzhb5JjN5kdcVuRR` e os
  IDs de credencial batem -> o import so troca nodes/conexoes/params, **NAO** sobrescreve
  credenciais). **Cada troca de credencial/import exige `docker restart listaatendimento-n8n-1`**
  (o webhook so re-registra com o workflow Active no boot).
- **Prevencao:** ao subir o n8n da automacao num ambiente novo, **as credenciais importadas vem com
  host de DEV** (`host.docker.internal`) — reapontar Redis (`redis:6379`) e WAHA (`http://waha:3000`)
  e **reimportar o `workflow-whatsapp.json` atual** (o que ja estava no n8n pode ser uma versao
  velha). Instalar o community node `n8n-nodes-waha` no volume; idealmente setar
  `N8N_REINSTALL_MISSING_PACKAGES=true` no compose (auto-cura do node em recreate do container).
  Paridade conferida pos-fix: **42 nodes, 0 diff, conexoes/settings identicos** VPS x local (so os
  VALORES das credenciais diferem, de proposito).

### 2026-06-20 — painel: editar o prompt "nao muda nada no n8n" (era pausa + guardrails da api antiga)

- **Sintoma:** usuario edita o prompt de comportamento em `/automation`, salva, mas "nao muda nada
  dentro do n8n". (VPS, conta **Crow Visuals**, sessao `default`.)
- **NAO era bug de salvamento:** `GET /v1/runtime/automation/config?session=default` ja retornava
  `persona` = o texto do painel (provado em execucoes reais via SQLite `execution_data` flatted). O
  caminho painel → DB → runtime-config **sempre funcionou**.
- **Causa 1 (o "nao muda"):** o **robo estava pausado** (de proposito). O gate `Bot ligado?` corta o
  fluxo (passa 0 itens) quando `cfg.enabled=false` → o **AI Agent nunca roda** → nenhuma execucao
  chega a usar o prompt. Alem disso, o no `AI Agent` usa o systemMessage por **expressao**
  (`{{ $('Montar systemMessage').first().json.systemMessage }}`), entao abrir o no nunca mostra o
  texto literal — e pull em runtime, comportamento correto.
- **Causa 2 (faria "responder errado" ao ligar):** a **api da VPS e antiga** e ainda anexa os
  `guardrails` (2047 chars) por cima da persona; o bloco comeca com "Responda SEMPRE em PT-BR, mesmo
  que a persona esteja em ingles" → **sobrescrevia** o teste "responde em ingles". A decisao "prompt e
  a lei: sem guardrails" (service.go `Guardrails:""`) estava no working tree, **nao-commitada/nao-deployada**.
- **Correcao (sem redeploy da api; escolha do usuario "prompt = lei"):** removido o append dos
  guardrails do no **`Montar systemMessage`** — no n8n da VPS (patch no SQLite: regex
  `sm\+='[^']*'\+guardrails;` + drop da `const guardrails`) **e no
  `automation/export/workflow-whatsapp.json`** do repo (pra re-import/deploy futuro nao reverter o fix).
  Restart do n8n; robo seguiu **pausado** (nao foi ligado). Provado: `systemMessage === cfg.persona`
  para qualquer mensagem; cadeia `Bot ligado? → Montar systemMessage → AI Agent` intacta. Backup do
  jsCode original em `/tmp/montar_jscode_backup.txt` no container n8n.
- **Prevencao:** "Active no n8n" e "ligado no painel" sao interruptores distintos — com pausa, **nada**
  chega ao AI Agent. O fix duravel de verdade e **redeploy da api** (passa a mandar `guardrails=""`
  nativo, hoje pre-Fase-2 na VPS); ate la, o no `Montar systemMessage` (VPS+repo) ja nao anexa
  guardrails, entao o **prompt do painel e a unica regra** — idioma/formato/baloes precisam estar
  DENTRO do proprio prompt.
