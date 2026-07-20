# Plano de Integracao — Automacao WhatsApp/IA no Omni

> **Visao maior:** este doc e o detalhe da PRIMEIRA automacao (`atendimento`). A visao de
> plataforma multi-tenant (cada cliente cria N automacoes, BYOK, RAG, super-robo) esta em
> [PLATAFORMA_AUTOMACAO.md](PLATAFORMA_AUTOMACAO.md), que **generaliza** o modelo de dados
> abaixo para ser **automation-centric** (`automation_id` central, N por account). Onde este
> doc assume "1 sessao default -> 1 account", a plataforma usa "sessao -> automacao -> account".

> Status: **design** (pending). Fonte de verdade da integracao do bot com o Omni.
> Espelhado em [roadmap-data.ts](../../web/app/components/roadmap/roadmap-data.ts) (fase
> `automation-whatsapp`) e [automation/AGENT.md](../../automation/AGENT.md).
> As fases que viram modulo Go / banco / front (A1 em diante) **aguardam o fechamento da
> branch `refactor/multi-tenant-complete`** (regra do MULTITENANT_COMPLETION_PLAN: nenhuma
> fase nova de modulo satelite avanca antes). Este doc e o desenho para quando liberar.
> Criado 2026-06-04.

> **Nota (2026-07-16).** Existe um plano para um modulo **`omnichannel`** separado (inbox humano
> + setores/filas + triagem IA): [../omnichannel/PLANO_ATENDIMENTO.md](../omnichannel/PLANO_ATENDIMENTO.md).
> Ele e **independente** e **nao toca em nada deste doc** — nem schema `automation.*`, nem rota,
> nem workflow n8n. O Tony segue exatamente como esta descrito aqui. Se um dia for preciso
> integrar os dois, isso vira um plano proprio.
> - **Aviso de operacao — um numero = um cerebro:** nao aponte o mesmo numero de WhatsApp para o
>   Tony e para outro sistema de atendimento ao mesmo tempo; dois robos respondendo o mesmo
>   cliente e incidente visivel para o cliente final. **Nenhum dos dois sistemas valida isso
>   automaticamente** — a responsabilidade e de quem opera.

---

## 1. Objetivo

Trazer o bot de WhatsApp (n8n + WAHA, persona Tony) de "configurado na mao dentro do n8n"
para **config-driven pelo painel do Omni**:

- O **painel** (front) configura tudo: WhatsApp conectado (QR/status), modelos por funcao,
  personas/prompts (criar/editar Tony e outros, escolher o ativo), liga/desliga e contexto
  temporario (ex.: "em gravacao ate 16h").
- A config vive no **Postgres do Omni** (schema `automation.*`), por account (tenant-aware).
- O **n8n consome** a config pela **API Go** a cada execucao (systemMessage, modelos,
  contexto, enabled), em vez de ter persona/modelo cravados nos nos.
- O bot acessa **produto/estoque/preco e CRM** via **tools do agente batendo na API Go**
  (catalogo/ERP), nao SQL cru nas tabelas.
- Roda **igual local e na VPS**: o comportamento vem do banco, cada ambiente le da sua API.

### Hoje (ponto de partida)
- Persona + guardrails + modelos estao cravados no `systemMessage` e nos nos do workflow.
- Memoria longa e uma versao "lite" no `staticData` do n8n. Sem CRM persistente.
- Sem painel: trocar persona/modelo exige editar o workflow.

---

## 2. Principios (herdados de ENGINEERING_PRINCIPLES + AGENT_RULES)

- **Tenant-aware**: toda tabela `automation.*` tem `account_id` NOT NULL com FK para
  `core.accounts`. `account_id` nunca vem do body — vem do `Principal.AccountID` (painel) ou
  do **token de servico** que o n8n usa (runtime).
- Modulo Go no padrao do repo: `model.go`, `store_postgres.go`, `service.go`, `http.go`,
  `module.go` (Module Registry quando `CORE_V2_ENABLED`), `AGENT.md`.
- IDs `string` (sem uuid externo), scan nullable com `*string`, migrations idempotentes
  (`IF NOT EXISTS`), permissoes no banco (`core.permissions`), sem SQL concatenado.
- Front: `<script setup lang="ts">`, composables `use*`, classes BEM-like, sem emojis,
  esconder pagina nao-pronta com `hidden`/`beta`, modal e board card espelhados.

---

## 3. Arquitetura alvo

```
                          PAINEL OMNI (Nuxt)
                          /automation (workspace)
                                │  JWT + X-Account-Id (+ permissao automation.config.manage)
                                ▼
   ┌─────────────────────────  API Go (modulo automation)  ─────────────────────────┐
   │  Admin:   settings · personas · modelos · status WhatsApp · CRM                  │
   │  Runtime: runtime-config · messages · memory · tools (catalog/erp/leads/orders)  │
   │           autenticado por TOKEN DE SERVICO por account (resolve account_id)       │
   └───────────────┬───────────────────────────────────────┬─────────────────────────┘
                   │ Postgres (schema automation.*)         │ proxy
                   ▼                                         ▼
        contatos/mensagens/lead/                     WAHA (sessao por account)
        follow-ups/compras/memoria                          ▲
        settings/personas/modelos                           │ webhook (rede interna)
                   ▲                                         │
                   │ HTTP (le config, grava msg, tools)      │
                   └──────────────  n8n (1 workflow parametrizado)  ──────────────────┘
```

Pontos-chave:
- **n8n nao fala com o Postgres direto.** Tudo passa pela API Go (auth, escopo por account,
  contrato estavel) — coerente com a remocao de fontes paralelas do `MULTITENANT_COMPLETION_PLAN`.
- A **sessao WAHA identifica a account**: o webhook traz `session`; o backend mapeia
  `session -> account` (tabela `automation.waha_sessions`) e resolve o resto.

---

## 4. Modelo de dados (schema `automation.*`) — esboco

> DDL final vira migration `####_automation_schema.sql` na implementacao (idempotente).
> Esboco para revisao (NAO e migration, nao aplicar): [schema_automation_sketch.sql](schema_automation_sketch.sql).

| Tabela | Campos principais | Nota |
|---|---|---|
| `automation_settings` | `account_id` (unico), `enabled bool`, `active_persona_id`, `models jsonb` (`{chat,vision,audio,classifier}`), `temp_context text`, `temp_context_expires_at timestamptz`, `debounce_seconds int default 7` | 1 linha por account. Config global do bot. |
| `personas` | `id`, `account_id`, `name`, `slug`, `system_prompt text`, `is_active bool`, `created_at`, `updated_at` | Tony, Perola, etc. So 1 ativa por account. |
| `guardrails` | `account_id`, `body text` | Regras anexadas a QUALQUER persona (PT-BR, texto puro, baloes). Default global seedado. |
| `model_catalog` | `id` (ex.: `gpt-4o`), `kind` (`chat`/`vision`/`audio`/`classifier`), `label`, `requires_responses_api bool`, `accepts_temperature bool`, `vision_ok bool` | Seed com as regras do MODELOS.md. O painel aplica as regras sozinho. |
| `waha_sessions` | `account_id`, `session_name` (ex.: `default`), `status`, `connected_phone`, `updated_at` | Mapa sessao WAHA -> account. |
| `service_tokens` | `id`, `account_id`, `token_hash`, `name`, `last_used_at`, `revoked_at` | Token que o n8n usa no runtime. Resolve a account (account_id nunca do body). Rotacionavel. |
| `contacts` | `id`, `account_id`, `channel`, `remote_id` (chatId), `push_name`, `phone`, `first_seen`, `last_seen` | Mini-CRM. |
| `messages` | `id`, `account_id`, `contact_id`, `direction` (`in`/`out`), `type` (`text`/`audio`/`image`), `content text`, `media_url`, `segment`, `created_at` | Historico. |
| `lead_state` | `contact_id`, `account_id`, `status`, `last_interaction`, `follow_up_count` | Estado do lead (motor proativo). |
| `long_memory` | `contact_id`, `account_id`, `summary text`, `updated_at` | Substitui a memoria "lite" do staticData. |
| `follow_ups` | `id`, `account_id`, `contact_id`, `due_at`, `kind`, `status`, `attempts` | Etapa 3 (proativo). |
| `purchases` | `id`, `account_id`, `contact_id`, ... | Pos-venda. |

Indices por `account_id` (+ `remote_id`, `due_at`, `created_at` nos hot paths).

---

## 5. Endpoints (API Go, modulo `automation`)

### Runtime (consumido pelo n8n; auth por token de servico -> resolve account)
| Verbo | Path | Retorno / acao |
|---|---|---|
| GET | `/v1/automation/runtime-config?session=default` | `{ enabled, systemMessage (persona+guardrails+tempContext), models{chat,vision,audio,classifier} com flags responsesApi/temperature, debounceSeconds }` |
| POST | `/v1/automation/messages` | grava cada msg (in/out) no CRM |
| GET/PUT | `/v1/automation/contacts/{remoteId}/memory` | le/atualiza o resumo de memoria longa |
| GET | `/v1/automation/tools/catalog?q=` | tool: busca produto (proxy `crm/catalog`) |
| GET | `/v1/automation/tools/stock?code=` | tool: estoque/preco (proxy `erp`) |
| POST | `/v1/automation/tools/leads` | tool: registra lead |
| POST | `/v1/automation/tools/orders` | tool: registra pedido/interesse |

### Admin (painel; JWT + `X-Account-Id` + permissao `automation.config.manage`)
| Verbo | Path | Acao |
|---|---|---|
| GET/PUT | `/v1/automation/settings` | le/edita enabled, modelos, contexto temporario, debounce |
| GET/POST | `/v1/automation/personas` | lista/cria persona |
| GET/PATCH/DELETE | `/v1/automation/personas/{id}` | edita/remove |
| POST | `/v1/automation/personas/{id}/activate` | define a persona ativa |
| GET | `/v1/automation/models` | catalogo de modelos + regras (MODELOS.md) |
| GET | `/v1/automation/whatsapp/status` | status da sessao WAHA |
| POST | `/v1/automation/whatsapp/session/start` | inicia sessao / retorna QR |
| POST | `/v1/automation/whatsapp/session/stop` | encerra sessao |
| GET | `/v1/automation/contacts` | lista CRM (paginada) |
| POST | `/v1/automation/service-tokens` / `.../{id}/rotate` | gera/rotaciona token do n8n |

Permissoes (em `core.permissions`, seedadas pelo Module Registry):
`automation.config.manage`, `automation.config.view`, `automation.crm.view`,
`automation.whatsapp.manage`.

---

## 6. Como o n8n consome (config-driven)

- **Auth**: credencial n8n "Header Auth" com `Authorization: Bearer <service_token da account>`.
- **Primeiro no util** apos normalizar: HTTP Request `Get runtime config`
  (`/v1/automation/runtime-config?session={{session}}`). Se `enabled=false`, encerra o fluxo.
- **AI Agent**: `systemMessage` vira expression = `runtime-config.systemMessage` (persona +
  guardrails + contexto temporario ja montados pelo backend). Para de ser cravado no no.
- **Modelos**: ids vem da config por expression. O backend ja entrega as flags
  (`responsesApiEnabled`, se aceita `temperature`) por modelo, aplicando o MODELOS.md.
  Ver **Decisao em aberto #4** (trocar modelo por expression no no OpenAI tem limitacao).
- **Tools do agente**: nos "HTTP Request Tool" apontando para `/v1/automation/tools/*`.
- **Persistencia**: nos HTTP para `POST /messages` e `PUT /memory` (substitui o staticData).
- A **sessao WAHA** no webhook resolve a account no backend (via `waha_sessions`).

### A6/A7 — ligar modelos + persistencia no n8n (passo a passo, 2026-06-11)

> Back pronto (A6 modelos, A7 messages/lead_state). Falta ligar no workflow + testar 1 msg real.
> Pre: `docker compose up -d --build api`; `AUTOMATION_RUNTIME_TOKEN` igual na api e na credencial
> Header Auth do n8n; escolher os modelos no painel `/automation` (card Modelos). Base interna do
> n8n -> api: `http://api:8080`. Para curl do host: `http://localhost:9091`.

**A6 (modelos config-driven).** O `runtime-config` agora traz `models[]`
(`{role,provider,modelId,label,requiresResponsesApi,acceptsTemperature,visionOk,params}`; roles
`chat|vision|audio|classifier`). No no **OpenAI Chat Model**, campo *Model* por expression (n8n nao
suporta `?.`, use `(obj||{})`):
`{{ ((($('Get runtime config').first().json.models||[]).find(m=>m.role==='chat'))||{}).modelId || 'gpt-4o-mini' }}`
Visao = role `'vision'` (fallback `gpt-4o`); classificador = role `'classifier'`. **Cuidado:** so
modelos de chat normal trocam por expression direto. Raciocinio (`gpt-5*`/o-series:
`requiresResponsesApi=true`, sem temperature, nao roda no no de imagem) exige modo Responses +
**Switch** pela flag. V1 = so chat.

**A7 (persistir).** Reusar a credencial Header Auth e o mesmo `session`/`chatId` dos nos de memoria.
- Entrada: HTTP `POST http://api:8080/v1/runtime/automation/messages?session=default`, body
  `{ "contactId": "<chatId>", "direction": "in", "type": "text", "content": "...", "mediaUrl": "", "segment": "" }` apos o classificador.
- Saida: idem com `"direction": "out"`, `content` = resposta, apos o AI Agent.
- lead-state (opcional, base do A9): `GET/PUT /v1/runtime/automation/lead-state?session=&contactId=`,
  PUT body `{ "status": "engaged", "followUpCount": 0 }`.

**Testar:** `curl -H "Authorization: Bearer $TOKEN" ".../config?session=default" | jq .models`; POST de
uma mensagem; conferir `select * from automation.messages order by created_at desc limit 5;`. Depois
**exportar** o workflow: `n8n export:workflow --id=<id> --output=automation/export/workflow-whatsapp.json`.

---

## 7. Painel (front Omni)

- Rota `/automation`, `workspaceId: automation_config`, `moduleId: 'automation'` (gating no
  nav + middleware `module-enabled`). Esconder com `hidden`/`beta` ate ficar pronta.
- Abas/cards do workspace:
  1. **Status**: WhatsApp conectado (status + botao conectar -> QR), liga/desliga do bot,
     contexto temporario (texto + expiracao).
  2. **Modelos**: escolher chat/visao/audio/classificador a partir do `model_catalog`
     (regras do MODELOS.md aplicadas sozinhas).
  3. **Personas/Prompts**: listar, criar e editar personas; escolher a ATIVA. Guardrails
     anexados automaticamente (nao editaveis junto da persona, ou editaveis numa secao a
     parte). **Modal e board card espelhados.**
  4. **CRM** (fase posterior): contatos/leads/mensagens.
  5. **Execucoes/Logs** (fase posterior): espelho leve das execucoes do n8n.
- Composables: `useAutomationSettings`, `usePersonasManager`, `useAutomationModels`,
  `useWahaSession`. Wire do `workspaceId` em **3 lugares** (`workspaces.ts`, `permissions.ts`,
  `nav.config.ts`) — licao do ENGINEERING_PRINCIPLES.

---

## 8. Local + VPS (rodar automatico)

A config no banco garante mesmo comportamento nos dois ambientes; cada n8n le da API do
seu ambiente. Nao se edita o n8n na mao.

- **Local**: profile `automation` no `docker-compose.yml` (ja feito). API em `http://api:8080`.
- **VPS** — infra **preparada em 2026-06-08** (compose.prod + env + runbook prontos; deploy
  em si e do Mike). Decisao de exposicao: **Caddy + basic auth** nos subdominios `n8n.` e
  `waha.`; Redis **so disponivel** na rede `app` (sem mexer na API Go ainda).
  - `redis`/`waha`/`n8n` ja adicionados ao `docker-compose.prod.yml` (profile `automation`,
    `restart: unless-stopped`, volumes nomeados). **Mesmos nomes de servico do dev**
    (`redis`/`waha`/`n8n`) para o workflow e as credenciais rodarem igual local e prod.
  - **Sem portas publicas**: `n8n`/`waha` entram na rede `proxy` (alias `automation-n8n` /
    `automation-waha`) e o Caddy roteia por subdominio com basic auth. Binds `127.0.0.1`
    so para tunel SSH. Webhook WAHA->n8n fica interno (`http://n8n:5678`).
  - `redis` so na rede `app` (nunca publico), pronto para a API Go do Omni consumir depois
    (`redis:6379`) — quando a multitenant-completion liberar A1+.
  - Vars novas no `.env.production.example` (bloco `AUTOMATION_*`): host/webhook do n8n,
    `N8N_ENCRYPTION_KEY`, hosts/aliases, dashboard da WAHA, `AUTOMATION_REDIS_PASSWORD`.
  - Runbook de deploy: [SETUP.md](SETUP.md) secao 8 (Caddy, env, subir, QR, backups).
  - **Pendente (do Mike)**: snippet do Caddy no projeto do proxy, DNS dos subdominios,
    subir na VPS, escanear QR, ativar. Backups dos volumes (`automation_n8n_data`,
    `automation_waha_sessions`). n8n aponta para a API interna (`http://api:8080`) +
    service token quando A2/A3 existirem; hoje a persona/modelo ainda vivem no workflow.

---

## 9. Fases (espelham roadmap-data: fase `automation-whatsapp`)

| Cod | Entrega | Depende |
|---|---|---|
| A0 | Infra: containers no profile `automation`, pastas, docs (FEITO 2026-06-04) | — |
| A1 | Migration `automation.*` (schema + seeds: guardrails default, model_catalog) | multitenant-completion |
| A2 | Modulo Go `automation`: settings, personas, model_catalog, **runtime-config** + service_tokens | A1 |
| A3 | n8n consome runtime-config (systemMessage/modelos/contexto/enabled dinamicos) | A2 |
| A4 | Painel: Status (WhatsApp connect/QR) + liga/desliga + contexto temporario | A2 |
| A5 | Painel: Personas/Prompts CRUD + ativa (guardrails auto-anexados) | A2 |
| A6 | Painel: Modelos (catalogo + regras MODELOS.md) — **FEITO 2026-06-11** (0144) | A2 |
| A7 | CRM persistente (contacts/messages/lead_state/long_memory) — **back FEITO 2026-06-11** (0145 + endpoints runtime); n8n gravar = passo de workflow | A2 |
| A8 | Tools do agente (catalog/stock/price, leads/orders) via API | A7 |
| A9 | Motor proativo (follow-ups, pos-venda, nurture) — Etapa 3 | A7 |
| A10 | Deploy VPS (compose.prod + Caddy + auth + backups) — **infra preparada 2026-06-08** (bot standalone, sem integracao Go ainda); deploy/QR/ativacao pendentes do Mike | — (infra independe de A1+) |

---

## 10. Notas de Deploy

- **Infra prod (A10) — feito 2026-06-08, deploy pendente do Mike.** Ordem na VPS:
  1. Preencher o bloco `AUTOMATION_*` no `.env.production` (gerar `N8N_ENCRYPTION_KEY` com
     `openssl rand -hex 24` e **nao** muda-lo depois; senhas fortes em Redis/WAHA dashboard).
  2. Adicionar as rotas Caddy (`n8n.` e `waha.`) com basic auth (`caddy hash-password`) no
     projeto do proxy e apontar os DNS; recarregar o Caddy.
  3. `docker compose -f docker-compose.prod.yml --profile automation up -d`.
  4. Primeiro acesso: criar conta dona do n8n, instalar community node `n8n-nodes-waha`,
     importar credenciais+workflow (CLI), escanear QR na WAHA, ativar (confirma com o Mike —
     responde no WhatsApp real). Detalhe: SETUP.md secao 8.
  5. Backup dos volumes `automation_n8n_data` e `automation_waha_sessions`.
  - Idempotencia: `up -d` e seguro repetir; o estado vive nos volumes. Trocar
    `N8N_ENCRYPTION_KEY` depois de salvar credenciais **quebra** a decriptacao (nao mudar).
- **Migrations M-series (2026-06-09/10, idempotentes) — feito localmente, deploy pendente:**
  - `0140_automation_schema.sql` — tabelas `automation.automations` + `automation.channels`.
  - `0141_automation_personas.sql` — tabela `automation.personas`; seed Tony/Crow Visuals.
  - `0142_automation_knowledge_docs.sql` — tabela `automation.knowledge_documents`; indice por `(automation_id, sort_order)`.
  - `0143_automation_contacts.sql` — tabela `automation.contacts`; memoria de conversa por chatId no Postgres (substitui staticData do n8n). UNIQUE `(automation_id, chat_id)`.
  - **Rebuild obrigatorio apos cada deploy:** `docker compose up -d --build api`.
  - **Reimport do workflow (uma unica vez apos 0143):** `n8n import:workflow --input=automation/export/workflow-whatsapp.json --overwrite`. A memoria agora vive no Postgres; o workflow le/escreve via `GET/PUT /v1/runtime/automation/memory`. Nao e necessario reimportar novamente quando docs forem editados/deletados.
- **A6/A7 (2026-06-11, idempotentes) — feito localmente, deploy pendente:**
  - `0144_automation_models.sql` — `automation.model_catalog` (global, provider-agnostico OpenAI+Anthropic, flags requires_responses_api/accepts_temperature/vision_ok) + `automation.automation_models` (modelo por automacao+funcao). Seed `ON CONFLICT DO NOTHING`.
  - `0145_automation_conversation.sql` — `automation.messages` + `automation.lead_state`.
  - `0147_automation_contacts_fix.sql` — **recria `automation.contacts`**. A 0143 tem bug: usa marcadores `-- +goose Up/Down`, mas o migrator (`migrator.go`) roda o arquivo INTEIRO como script, entao o `DROP TABLE` do bloco Down dropava a tabela no mesmo boot. Aplicar antes de usar memoria/CRM. (A 0135 foi auditada: OK — DROP intencional de tabelas legadas, sem goose.)
  - **Contrato runtime-config mudou:** `GET /v1/runtime/automation/config` agora devolve `models[]` (`{role,provider,modelId,label,requiresResponsesApi,acceptsTemperature,visionOk,params}`) alem de enabled/systemMessage/persona/guardrails/docs[] (intactos). Novos endpoints runtime: `POST /v1/runtime/automation/messages`, `GET/PUT /v1/runtime/automation/lead-state` (mesmo `AUTOMATION_RUNTIME_TOKEN`). Painel: `GET/PUT /v1/automation/models`.
  - **Rebuild obrigatorio:** `docker compose up -d --build api`. Reimport do workflow n8n so quando os nos passarem a consumir `models[]` / gravar messages.
- **Migration nova `automation.*`** (A1, idempotente). Permissoes `automation.*` seedadas
  pelo Module Registry no boot (`CORE_V2_ENABLED`). Rebuild da API obrigatorio (codigo Go
  novo): `docker compose up -d --build api`. **Bloqueado pela multitenant-completion.**
- Token de servico do n8n por account (A2+): gerar no painel, colar na credencial Header
  Auth do n8n. Rotacionavel. Nunca em log.
- Vars: bloco `AUTOMATION_*` local em `.env.docker.example` e prod em
  `.env.production.example`. Quando A2/A3 existirem, o n8n consome a API interna
  (`http://api:8080`) + service token (na credencial, nao em env).

---

## 11. Decisoes fechadas (2026-06-04)

1. **Tenancy**: schema **account-scoped desde ja**; a V1 opera **1 sessao `default` mapeada a
   1 account** (ex.: Perola). Multi-sessao/multi-account fica para sub-fase posterior.
2. **Topologia n8n**: **1 workflow parametrizado por account** (a sessao WAHA resolve a account).
3. **n8n na VPS**: **headless** — workflow importado no deploy; edicao so local. Sem expor o
   editor online por enquanto.
4. **Permissao**: gestao no painel exige `automation.config.manage`. Na V1 quem opera e o
   `platform_admin`; a permissao pode ser atribuida a um cargo do account depois (RBAC).
5. **Rota da pagina**: `/automation` (renomeavel sem custo enquanto nao houver link externo).

### A validar tecnicamente (nao e decisao de produto)
- **Troca de modelo por expression** (A3): o no OpenAI Chat Model aceita o `id` por expression,
  mas `responsesApiEnabled` por expression precisa de validacao — pode exigir 2 nos (um Responses,
  um Chat) + Switch pela flag do `model_catalog`. Validar antes de prometer "qualquer modelo".

---

## Referencia cruzada

- Modulo e infra -> [../../automation/AGENT.md](../../automation/AGENT.md)
- Runbook de subida -> [SETUP.md](SETUP.md)
- Arquitetura/roadmap do bot (n8n) -> [WORKFLOW.md](WORKFLOW.md) · [ROADMAP.md](ROADMAP.md)
- Regras de modelos -> [MODELOS.md](MODELOS.md)
- Persona ativa / guardrails -> [gpt-tony.md](gpt-tony.md) · [guardrails-resposta.md](guardrails-resposta.md)
- Modulo `omnichannel` (inbox/filas/triagem, vizinho deste bot) -> [../omnichannel/PLANO_ATENDIMENTO.md](../omnichannel/PLANO_ATENDIMENTO.md)
- Plano da branch atual (bloqueia A1+) -> [../MULTITENANT_COMPLETION_PLAN.md](../MULTITENANT_COMPLETION_PLAN.md)
- Principios/regras -> [../ENGINEERING_PRINCIPLES.md](../ENGINEERING_PRINCIPLES.md) · [../../AGENT_RULES.md](../../AGENT_RULES.md)
