# Plano — Omni Chat interno (painel de Operação ligado ao n8n)

> Status: **design + M0 em implementação** (2026-06-18). Fonte de verdade do canal de chat
> interno do painel. Espelhado em [roadmap-data.ts](../../web/app/components/roadmap/roadmap-data.ts)
> (fase `automation-whatsapp`, tarefas `oc-*`) e em
> [back/internal/modules/automation/AGENT.md](../../back/internal/modules/automation/AGENT.md).
>
> Reaproveita a infra do bot de WhatsApp (módulo `automation` Go M1–M5, n8n, persona Tony,
> token `AUTOMATION_RUNTIME_TOKEN`), mas é um **canal novo e independente** do WhatsApp/WAHA.

---

## 1. Objetivo

Transformar o bloco "Omni Chat" do painel de **Operação** (`/operacao`, 3ª coluna,
`web/app/components/operation/OperationSidePanel.vue`) — hoje só stub visual com input
desabilitado e badge "Prévia" — num assistente real ligado ao **n8n**.

Diferente do WhatsApp: não é canal externo, **tem usuário logado** (conta/loja conhecidas pelo
JWT) e deve, na evolução, responder sobre **produtos, vendas/ranking e metas** consultando o
CRM/ERP via API Go.

**MVP (M0):** provar o fluxo ponta-a-ponta com a persona Tony, **sem** consultar banco ainda.
**Fase 2:** ferramentas de dados na ordem Produtos/Catálogo → Vendas & Ranking → Metas.

### Decisões travadas (2026-06-18)
1. **Topologia:** `Front → API Go → n8n` (proxy). O navegador **nunca** fala com o n8n.
2. **Persona (M0):** reaproveitar o **Tony** (dívida: é customer-facing; persona interna depois).
3. **M0:** pipeline + persona, sem dados.
4. **1ª leva de tools:** Produtos → Vendas/Ranking → Metas (Consultores na leva 2).

---

## 2. Arquitetura

```
Browser (painel Operação — JWT + X-Account-Id injetado pelo api-client.ts)
   │  POST /v1/omni-chat/ask   { question, topic }
   ▼
API Go  (módulo automation)
   │  1. RequireAuth → principal { accountId, storeIds, userId, role }
   │  2. resolve systemMessage do Tony p/ a account
   │       (REUSO GetOrCreateDefault + ensurePersona + buildSystemMessage do service.go)
   │  3. (Fase 2) assina contextToken HMAC curto { accountId, storeId, userId, role, exp }
   │  POST http://n8n:5678/webhook/omni-chat   (Authorization: Bearer AUTOMATION_RUNTIME_TOKEN)
   │       { question, topic, systemMessage, sessionRef, contextToken }
   ▼
n8n  (automation/export/workflow-omni-chat.json — enxuto)
   Webhook (Header Auth) → AI Agent (OpenAI, systemMessage pronto) → Respond to Webhook { answer }
   │  (Fase 2) AI Agent tools → HTTP → /v1/runtime/automation/tools/* (ecoa contextToken)
   ▲ { answer }
   └────────► Go devolve { answer, topic } ──────► Browser renderiza no chat
```

### Princípio multi-tenant (inegociável)
`storeId`/`accountId` **nunca** vêm do client nem do n8n. A única fonte é o `principal`
autenticado. Na Fase 2 o escopo viaja num **token HMAC opaco** que o n8n só ecoa (não consegue
forjar, não tem o segredo). Endpoint de tool faz `WHERE account_id = $1`. Recurso fora de escopo
= **404** (nunca 403, que vaza existência).

---

## 3. Contrato congelado (sincroniza as 3 trilhas)

As trilhas BACK / N8N / FRONT podem rodar em paralelo porque aderem a este contrato. Mudou o
contrato → atualizar este doc + as 3 trilhas juntas.

### 3.1 HTTP painel (FRONT ↔ BACK)
- **Rota:** `POST /v1/omni-chat/ask` — auth `RequireAuth` (fora do prefixo `/v1/automation` para
  não exigir o módulo `automation` de quem usa Operação; a trilha BACK confirma que o path não
  cai em `RequireModuleByPath`).
- **Request:** `{ "question": string, "topic"?: string }`
- **Response 200:** `{ "answer": string, "topic"?: string }`
- **Erros** (via `httpapi.WriteError`):
  - `400 missing_question` — pergunta vazia.
  - `400 question_too_long` — > 2000 chars.
  - `503 omnichat_not_configured` — `AUTOMATION_N8N_INTERNAL_URL` vazio.
  - `502 omnichat_error` — n8n fora do ar / HTTP não-2xx / JSON fora do contrato.
  - `504 omnichat_timeout` — LLM/n8n estourou o deadline.
- `accountId` vem de `accountIDFromContext` (header `X-Account-Id`, auto-injetado pelo
  `web/app/utils/api-client.ts`); `storeId`/`userId`/`role` vêm do `principal`. Nunca do body.

### 3.2 Webhook n8n (BACK ↔ N8N)
- **Endpoint:** `POST http://n8n:5678/webhook/omni-chat` (interno; nunca exposto ao browser).
- **Auth:** o Go envia `Authorization: Bearer <AUTOMATION_RUNTIME_TOKEN>`. **No MVP local o nó
  Webhook está SEM autenticação** (o webhook é interno à rede docker, nunca exposto) para subir
  rápido sem configurar credencial na UI. Reativar a checagem = adicionar uma credencial
  "Header Auth" (Name `Authorization`, Value `Bearer <token>`) no nó Webhook; o Go já manda o
  header, então é só ligar. *(Marcado como simplificação de MVP — ver Riscos.)*
- **Request body:** `{ question, topic, systemMessage, sessionRef, contextToken }`
  - `systemMessage` vem **pronto do Go** no M0 (persona Tony montada). `contextToken` é vazio no M0.
- **Response:** `{ "answer": string, "topic"?: string }` — `answer` vem de `$json.output` do AI
  Agent; `topic` é lido do nó Webhook via `$('Webhook').first().json.body.topic` (no ponto do
  Respond, `$json` já é a saída do agente e não carrega `body`).

### 3.3 contextToken (Fase 2)
`base64url(payload) + "." + hex(HMAC-SHA256(secret, base64url(payload)))`, payload =
`{ accountId, storeId, userId, role, exp }` com `exp = now + 120s`. Segredo:
`AUTOMATION_CONTEXT_TOKEN_SECRET` (ou reusar `AUTOMATION_RUNTIME_TOKEN` no início). Precedente de
HMAC: `back/internal/modules/auth/tokens.go`.

---

## 4. M0 — implementação por trilha

### Trilha BACK — módulo `automation` Go
Arquivos novos (padrão do módulo, < 450 linhas cada):
- `n8n_client.go` — cliente HTTP do webhook interno. **Molde de
  `back/internal/modules/meta_ads/runner_client.go`**: struct `{baseURL, token, http}`, timeout
  ~60s, `io.LimitReader`, sentinel errors (`ErrN8NNotConfigured` → 503, `errN8NFailed` → 502),
  `do(ctx, method, path, body)` com `Authorization: Bearer`.
- `service_omnichat.go` — `OmniChatAsk(ctx, accountID, question, topic) (OmniChatResultView, error)`:
  `GetOrCreateDefault` → `ensurePersona` → `ListKnowledgeDocs` → `buildSystemMessage` (REUSO) →
  `n8n.Ask`.
- `http_omnichat.go` — `handleOmniChatAsk(svc)`: `http.MaxBytesReader`, valida `question`,
  `accountIDFromContext`, mapeia erros (sentinels + `context.DeadlineExceeded` → 504).
Editar:
- `http.go` — registrar `POST /v1/omni-chat/ask` com `RequireAuth`.
- `module.go` — ler `AUTOMATION_N8N_INTERNAL_URL` (default `http://n8n:5678`) e construir
  `NewN8NClient(url, runtimeToken)` no `Build`/`handle` (espelha o wire do `runtimeToken`).
- `AGENT.md` — seção do Omni Chat (endpoint, env, contrato, Notas de Deploy).

### Trilha N8N — `automation/export/workflow-omni-chat.json` (novo, enxuto)
- **Manter** do `workflow-whatsapp.json`: AI Agent (`@n8n/n8n-nodes-langchain.agent`) + sub-nó
  OpenAI Chat Model (credencial `sCzmqFisO8bdeZ9B`).
- **Remover** todo o pré/pós WhatsApp (WAHA webhook/send, Dados, Switch, Tipo, Dedupe, mídia,
  Fila Redis/debounce, Ctx classificar, Resumir, split de balões, Loop/Wait/Digitando/Send Seen,
  Get runtime config, Montar systemMessage).
- **Adicionar:** `Webhook` (POST path `omni-chat`, Respond = "Using Respond to Webhook node",
  **sem auth no MVP — interno**) → `AI Agent` (System Message = `{{ $json.body.systemMessage }}`,
  User = `{{ $json.body.question }}`, **sem memória no M0**) → `Respond to Webhook`
  (`{ answer: {{ $json.output }}, topic: {{ $('Webhook').first().json.body.topic }} }`).
- Resultado: 3 nós principais + 1 sub-nó. Importar + **ativar** + restart do n8n.
- **Estado local (2026-06-18):** `automation/export/workflow-omni-chat.json` (id `omnichatmvp00001`)
  importado, ativo e **verificado** no n8n local — `/webhook/omni-chat` devolveu `{answer, topic}`
  reais via OpenAI. Credencial OpenAI reusada do workflow do WhatsApp.

### Trilha FRONT — `web/app/components/operation/OperationSidePanel.vue` (já é TS)
- Novo `web/app/composables/useOmniChat.ts` (TS, molde de `useAutomation.ts`):
  `createApiRequest(runtimeConfig, () => auth.accessToken)`, `sendQuestion(question, topic)` →
  `POST /v1/omni-chat/ask`; estados `messages`/`draft`/`activeTopic`/`sending`/`errorMessage`;
  `AbortController`; tipo `ChatMessage`; sem `any`, sem `console.log`. **Não** envia
  storeId/accountId (vão automáticos no header).
- Componente: habilitar input (`v-model`, `@keydown.enter`) e botão; chips clicáveis setam tópico
  ativo; `v-for` de bolhas user/assistant (classes BEM `.operation-side__chat-msg--user|--assistant`);
  bolha "digitando" em `sending`; faixa de erro; scroll automático ao fim. Manter badge "Prévia".

---

## 5. Fase 2 — ferramentas de dados (após M0)

Ordem: **Produtos/Catálogo → Vendas & Ranking → Metas**. Pré-requisito comum: o **contextToken** (§3.3).

- **Endpoints runtime espelho** (o n8n não pode chamar as rotas de painel, que exigem JWT de usuário):
  - `GET /v1/runtime/automation/tools/catalog` (já existe — adaptar p/ aceitar contextToken): `site.products`.
  - `GET /v1/runtime/automation/tools/ranking` → espelha `/v1/analytics/ranking` escopado.
  - `GET /v1/runtime/automation/tools/goals` → espelha `/v1/operations/goals` escopado.
  - Cada um valida `Bearer` (transporte) + `contextToken` (HMAC+exp), usa `{accountId, storeId}` do
    token como autoritativo. Fora de escopo = 404.
- **n8n:** 1 "HTTP Request Tool" por domínio plugado no AI Agent, ecoando o contextToken no header
  `X-Omni-Context`. Levas incrementais e paralelas entre si.

---

## 6. Refinamentos (dívida pós-MVP)
- Persona interna dedicada ("Omni Operador") separada do Tony customer-facing.
- Streaming (SSE) para efeito de digitação e tolerância a LLM lento.
- Memória de conversa por **`userId`** (nunca `accountId` — evita vazamento entre operadores).
- Histórico persistente reusando `POST /v1/runtime/automation/messages`.
- Consultores/Equipe como 2ª leva de ferramentas.

---

## 7. Notas de Deploy
- **Env nova (1):** `AUTOMATION_N8N_INTERNAL_URL` (default `http://n8n:5678`) no serviço `api` de
  `docker-compose.yml` e `docker-compose.prod.yml` + `.env.docker.example` e `.env.production.example`
  (seção "Modulo automation"). Fase 2: opcional `AUTOMATION_CONTEXT_TOKEN_SECRET`.
- Reusa `AUTOMATION_RUNTIME_TOKEN` para o auth Go→n8n (sem token novo no M0).
- **Rebuild obrigatório da api** (código Go novo): `docker compose up -d --build api`.
- **n8n:** importar `workflow-omni-chat.json`, **ativar** o workflow e restart do n8n (webhook só
  ouve com workflow Active; usar path `/webhook/omni-chat`, não `/webhook-test/`).
- **Migration:** `0165_erp_item_current_tenant_sku_idx.sql` — índice `(tenant_id, sku)` em
  `queue.erp_item_current` (a PK é `(tenant_id, store_id, sku)`; sem store_id o enrich do catálogo
  varria ~360k itens/lookup → ~8s; com o índice → ~60ms). Idempotente. (Antes do enrich era sem migration.)

### Deploy na VPS (prod) — runbook do Omni Chat
Prod **já preparado** no `docker-compose.prod.yml`: n8n `2.23.2` (igual ao dev → o fluxo manual roda
idêntico), `api` e `n8n` na rede `app` (n8n alcança `http://api:8080`), envs
`AUTOMATION_N8N_INTERNAL_URL` (default `http://n8n:5678`) e `AUTOMATION_RUNTIME_TOKEN` no compose. api/web
são imagens **GHCR** (CI builda, VPS faz pull). **1 migration** (`0165`, índice no ERP — roda no
`migrate up` do deploy da api; cria índice em tabela grande, breve lock de escrita). Ordem (comandos do Mike — [[feedback_local_only]]):
1. **Código → imagens:** commit + push da branch; o CI builda `omni-api`/`omni-web` (tag = SHA). Pipeline: `docs/DEPLOY_VPS.md`.
2. **`.env.production` (VPS):** `AUTOMATION_RUNTIME_TOKEN=<forte>` (MESMO valor na api e no n8n). Demais `AUTOMATION_*`/`AUTH_GATEWAY_COOKIE_DOMAIN` conforme `SSO_GATEWAY_PLAN.md` §6.
3. **VPS — api/web:** `docker compose -f docker-compose.prod.yml pull api web && up -d api web` (api tem código Go novo; web tem os cards).
4. **VPS — n8n:** subir o profile se preciso: `... --profile automation up -d`. Importar o workflow + credencial OpenAI:
   - SCP `automation/export/workflow-omni-chat.json` (versionado) + `automation/export/credentials.decrypted.json` (gitignored) p/ a VPS.
   - `docker compose cp workflow-omni-chat.json n8n:/tmp/` → `exec n8n n8n import:workflow --input=/tmp/...` → `n8n update:workflow --id=omnichatmvp00001 --active=true` → `restart n8n`.
   - Credencial OpenAI (workflow referencia id `sCzmqFisO8bdeZ9B`): `n8n import:credentials --input=/tmp/credentials.decrypted.json` (ou criar na UI). **N8N_ENCRYPTION_KEY** fixo (não mudar depois de salvar credenciais).
5. **n8n online:** seguir `SSO_GATEWAY_PLAN.md` §9 — `n8n.crowvisuals.com.br` com **login próprio do n8n** (sem gate Omni); **CRIAR o owner do n8n ANTES de expor** (land-grab); bloco Caddy `reverse_proxy`; DNS já feito.
6. **Dados/imagens:** p/ os cards mostrarem foto/preço, a VPS precisa de `site.products` + `erp_item_current` da Pérola e as imagens no volume `api_uploads`. Se a prod já é a base real, ok; senão rodar o sync (`POST /v1/admin/products/sync?accountId=<perola>`).
- **Pendência atual:** este código está na branch `refactor/multitenant-complete` (não commitado). O deploy carrega o que for buildado a partir dela.

---

## 8. Riscos
1. **Timeout síncrono (LLM > 30s):** client 60s + context 55s → `504`; AbortController no front;
   conferir que nenhum proxy (Caddy) corta antes. Streaming fica como refinamento.
2. **Workflow inativo:** webhook só ouve Active; apontar para `/webhook/` (prod).
3. **Persona Tony é customer-facing:** respostas internas soam de venda. Dívida (§6).
4. **Gate de módulo:** o prefixo `/v1/automation` é gateado por módulo; por isso a rota fica em
   `/v1/omni-chat/ask` (sem gate de módulo). Confirmar na trilha BACK.
5. **Memória (Fase 2):** sessionKey por `userId`, nunca `accountId`.
6. **Webhook sem auth no MVP (dívida):** o nó Webhook subiu sem Header Auth para acelerar; mitigado
   por ser interno (rede docker, nunca via Caddy). O Go já manda o Bearer — reativar a checagem
   antes de qualquer exposição pública.

---

## Estado atual e ponto de retomada (2026-06-18, PAUSADO)

### Funcionando e verificado
- **M0 (pipeline + persona Tony)** — ponta-a-ponta no n8n local. `POST /webhook/omni-chat` →
  `{answer, topic}` real via OpenAI. Workflow `omnichatmvp00001` importado + ativo. Webhook **sem
  auth** (interno). `responseBody` corrigido (`$('Webhook').first().json.body.topic`).
- **Go `/v1/omni-chat/ask`** — registrado (401 sem auth), api **rebuildada** com o código da Fase 2.
- **Fase 2 fundação (context token)** — `context_token.go` (ctxv1, HMAC, TTL 300s, secret =
  `AUTOMATION_RUNTIME_TOKEN` no dev). Endpoint **`GET /v1/runtime/omni-chat/catalog`** verificado por
  curl com token mintado: token válido (conta zero) → `200 []`; context token inválido → `401`;
  transport token errado → `401`. O `contextToken` já viaja no body do webhook (`OmniChatAsk` emite).

### Catálogo (Produtos) — FUNCIONANDO via FLUXO MANUAL (2026-06-18)
As **tools nativas do AI Agent não funcionam** neste build (`n8nio/n8n:2.23.2`, monorepo): o Tools
Agent V3 coleta a tool exigindo `supplyData` mas a executa via `runNode` exigindo `execute()` —
nenhum nó tem os dois (testado: `toolHttpRequest`, `httpRequest`, header estático, agent 2.2, Responses
API off — todos falham). **NÃO usar tool node nativo neste build.**

Decisão (usuário): **fluxo manual**, só com nós comprovados (AI Agent SEM tools + HTTP Request comum
no fluxo principal). Workflow `omnichatmvp00001` (linear):
```
Webhook
  → "Extrair termo" (AI Agent, sem tools): devolve o termo do produto OU "NONE"
  → "Buscar catalogo" (httpRequest GET http://api:8080/v1/runtime/omni-chat/catalog?q=<termo>,
       headers Authorization: Bearer $env.AUTOMATION_RUNTIME_TOKEN + X-Omni-Context =
       {{ $('Webhook').first().json.body.contextToken }})
  → "AI Agent" (compõe com {{ JSON.stringify($json.produtos) }} + systemMessage do Tony)
  → Respond to Webhook { answer, topic }
```
Dois nós OpenAI (extrator + compositor), `responsesApiEnabled=false`. **Verificado e2e:** pergunta de
produto → `GET /v1/runtime/omni-chat/catalog 200` → resposta; pergunta normal → NONE → responde normal;
sem erro no n8n. O endpoint Go passou a devolver **objeto** `{produtos:[...],total}` (não array) p/ o
n8n entregar 1 item.

**Dados enriquecidos pelo ERP (2026-06-18):** a query (`store_product.go: SearchSiteProducts`) usa
`site.products` (lista + imagem) + LEFT JOIN LATERAL `queue.erp_item_current` (por
`sku == split_part(code,'_',1)` — o code do site é multi-parte; cobre ~511/773) p/ trazer **nome real,
marca e preço** — porque o `site.products` da Pérola veio com nome genérico e **preço R$ 0**. Índice
`(tenant_id, sku)` (migration 0165) deixa rápido (~60ms vs ~8s sem). Marca puramente numérica (cód. de
loja) é escondida; produtos duplicados (variantes de código) são deduplicados por nome. Resposta:
`{ name, code, price, brand, image }`. Busca **multi-palavra**
(`ilike all` dos tokens no nome do site + nome ERP + marca); o extrator devolve termos no singular sem
preposição. Provado com Pérola: "me lista 2 relogios seiko" → 2 SEIKO completos (R$ 5.840/5.300, marca,
link de imagem). "anel ouro" vazio é dado (143 "anel", só 1 com "ouro" no texto).

**Imagem no chat (cards) — FEITO:** o `/ask` agora devolve `products[]` estruturado (`{name, code,
price, brand, image}`) — o n8n inclui no Respond o resultado da tool, o Go faz pass-through, e o front
(`OperationSidePanel.vue` + `useOmniChat.ts`) renderiza **cards com a foto** (prefixa `apiBase` no path
`/uploads/...`; a api serve `/uploads/*`). O compositor dá resposta curta de 1 linha (os detalhes/foto
vêm nos cards, sem repetir no texto). Verificado: "me lista 2 relogios seiko" → texto curto + 5 cards
com imagem/preço/marca. (Preço = ERP maior>0 entre lojas — store-scoping fino fica como refinamento.)

**Falta:** teste no NAVEGADOR (trocar a conta ativa p/ **Pérola** — os produtos estão só nela). Padrão
pronto para estender a Ranking/Metas (ver abaixo).

### Depois do catálogo (resto da Fase 2)
Ranking e Metas: criar `GET /v1/runtime/omni-chat/ranking` e `.../goals` reusando os services
existentes — reconstruindo um `auth.Principal` a partir do context token (claims já carregam
accountId, tenantId, storeIds, userId, role) e chamando `analyticsService.Ranking(...)` /
`goalsService.List(...)`. Decisão pendente de escopo: loja ativa (front manda storeId, validado) vs
"todas as lojas" acessíveis. Endpoints/shapes mapeados em `back/internal/modules/queue/analytics`,
`queue/reports`, `operationgoals`.

## Referência cruzada
- Plano can. da automação WhatsApp → [PLANO_INTEGRACAO_OMNI.md](PLANO_INTEGRACAO_OMNI.md)
- Gate SSO n8n/WAHA → [SSO_GATEWAY_PLAN.md](SSO_GATEWAY_PLAN.md)
- Módulo Go → [../../back/internal/modules/automation/AGENT.md](../../back/internal/modules/automation/AGENT.md)
- Infra containers → [../../automation/AGENT.md](../../automation/AGENT.md)
- Princípios → [../ENGINEERING_PRINCIPLES.md](../ENGINEERING_PRINCIPLES.md)
