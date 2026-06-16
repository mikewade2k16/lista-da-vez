# AGENT — Modulo Go `automation`

Modulo plugavel (Module Registry) do painel de automacao WhatsApp/IA dentro do Omni.
Tenant-aware (schema `automation.*`, `account_id` FK `core.accounts`). Faz o proxy
escopado da WAHA para o front, sem expor n8n/WAHA ao cliente.

> Visao de plataforma (multi-tenant, BYOK, RAG, personas): docs/automation/PLATAFORMA_AUTOMACAO.md.
> Infra dos containers (n8n/waha/redis): automation/AGENT.md (raiz). Esquema-alvo:
> docs/automation/schema_automation_sketch.sql.

## Estado: M1 + M2 + M3 + M3+ + A6 + A7 + M4 + M5 — 2026-06-11

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

Proximas fases (BYOK, multi-numero, CRM, tools) em PLATAFORMA_AUTOMACAO.md (P6+).

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
  service.go           <- orquestra store + WAHA; buildSystemMessage (instrucoes+docs+guardrails)
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
- Var **`AUTOMATION_RUNTIME_TOKEN`** (M2): a **api E o n8n** usam o MESMO valor (api valida,
  n8n manda no header). Dev tem default que ja bate; prod exige set em `.env.production`
  (gere forte). Sem ele, o runtime-config responde 503.
- **Ordem de ativacao do M2:** (1) set do token; (2) `up -d --build api` (sobe modulo +
  migrations); (3) restart do n8n (pega o `$env`); (4) re-importar o workflow
  (`n8n import:workflow`) que agora consome o runtime-config; (5) ativar e testar.

## Gotchas

- `liga/desliga` (M1) so persiste o `status`; o robo so passa a respeitar quando o n8n
  consumir o runtime-config (fase M2). O painel avisa isso na UI.
- WAHA Core suporta 1 sessao -> V0 opera 1 numero (`default`). Multi-numero = P11 (WAHA Plus
  ou Evolution API).
- WAHA tag e `gows-<versao>` (engine GOWS).
