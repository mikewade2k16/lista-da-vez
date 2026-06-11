# AGENT — Modulo Go `automation`

Modulo plugavel (Module Registry) do painel de automacao WhatsApp/IA dentro do Omni.
Tenant-aware (schema `automation.*`, `account_id` FK `core.accounts`). Faz o proxy
escopado da WAHA para o front, sem expor n8n/WAHA ao cliente.

> Visao de plataforma (multi-tenant, BYOK, RAG, personas): docs/automation/PLATAFORMA_AUTOMACAO.md.
> Infra dos containers (n8n/waha/redis): automation/AGENT.md (raiz). Esquema-alvo:
> docs/automation/schema_automation_sketch.sql.

## Estado: M1 + M2 + M3 + M3+ — 2026-06-10

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

Proximas fases (BYOK, multi-numero, CRM, tools) em PLATAFORMA_AUTOMACAO.md (P6+).

## Estrutura

```
back/internal/modules/automation/
  model.go         <- Automation, Channel, Persona, KnowledgeDoc e *View
  waha_client.go   <- cliente HTTP interno da WAHA (status/start/stop/qr)
  store_postgres.go<- persistencia (automations/channels/personas/knowledge_documents)
  service.go       <- orquestra store + WAHA; buildSystemMessage (instrucoes+docs+guardrails)
  http.go          <- handlers /v1/automation* (RequireAuth; accountID do principal)
  module.go        <- adaptador Module Registry (ID/Metadata/Permissions/Build)
  AGENT.md         <- este arquivo
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

`account_id` vem do `Principal` (X-Account-Id), nunca do body. O proxy WAHA usa
`AUTOMATION_WAHA_INTERNAL_URL` (default `http://waha:3000`).

### Runtime (consumido pelo n8n) — M2+

| Verbo | Path | Acao |
|---|---|---|
| GET | `/v1/runtime/automation/config?session=` | `{ enabled, systemMessage, persona, guardrails, docs[] }` — systemMessage = montagem completa (Opcao A); persona/guardrails/docs = partes separadas para montagem dinamica (Opcao B) |
| GET | `/v1/runtime/automation/memory?session=&chatId=` | `{ seg, lastMsg, ts, longMem }` — memoria de conversa do contato; retorna zeros se nao existe |
| PUT | `/v1/runtime/automation/memory?session=&chatId=` | body `{ seg, lastMsg, ts, longMem }` — upsert da memoria; longMem vazio preserva o valor anterior |
| GET | `/v1/automation/context-preview` | `{ personaName, instructions, knowledgeDocs[], guardrails, systemMessage }` — previa do bot para o painel (JWT normal, sem sessao) |

**Fora do prefixo `/v1/automation`** (de proposito: nao passa pelo `RequireModuleByPath`
nem exige X-Account-Id). Auth por **token de servico**: header `Authorization: Bearer
$AUTOMATION_RUNTIME_TOKEN` (constant-time compare; 401 se invalido, 503 se nao configurado).
A `session` resolve channel -> automacao -> persona ativa (semeia a default se faltar).

**Como o n8n consome (Opcao B — injecao dinamica):** no `Get runtime config` busca a config
com `persona`, `guardrails` e `docs[]`. O no `Montar systemMessage` (code, entre `Bot ligado?`
e o AI Agent) classifica a mensagem por keywords, seleciona os docs relevantes e monta o
`systemMessage` customizado. O AI Agent usa `$('Montar systemMessage').first().json.systemMessage`.
Fallback: se nenhum doc bate, usa todos os habilitados (equivalente a Opcao A).

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
