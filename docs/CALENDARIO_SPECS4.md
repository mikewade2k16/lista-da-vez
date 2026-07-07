# Calendário — SPECS WAVE 4 (Chat com memória, persistência e escopo de clientes)

> Specs atômicas para subagentes. Fonte da visão: decisões do dono (2026-07-04). Base já no ar:
> chat do calendário (Crow Assistente) com voz (Whisper self-hosted + ditado ao vivo), provider
> IA por conta/cliente (waves 3/3.1). Este doc adiciona PERSISTÊNCIA + MEMÓRIA + CONTEXTO DE
> CLIENTES + SELECT DE ESCOPO com controle de acesso. Regras gerais idênticas às waves anteriores
> (skill principios-engenharia; NUNCA git/npm/docker nos agentes; máx 450 linhas/arquivo; pt-BR
> sem acentos em comentário; multi-tenant account_id do accountScope; migrations idempotentes sem
> `-- +goose`; atualizar AGENT.md da área). **Status: SPECS — aguardando OK do dono para implementar.**

## Decisões do dono

1. **Conversas HÍBRIDAS**: cada usuário-cliente vê só as SUAS conversas; a AGÊNCIA (a organização)
   vê TODAS. Deriva do modelo existente: `isAgency` (platform_admin OU agency_owner na org da conta)
   vê tudo; usuário-cliente (só membership em `core.account_users`) vê só `created_by = ele`.
2. **Escopo do chat AGORA**: "Todos os clientes" + "cliente específico". "Organização" fica p/ depois
   (quando houver multi-org; hoje 1 agência = 1 org, então "Todos" ≈ a org dela).
3. **Controle de acesso do SELECT (o ponto crítico)**: NÃO inventar conceito novo. A lista de clientes
   já vem permission-scoped do back (`/v1/tenants` → `tenants/scope_queries.go`). Regra: usuário que
   enxerga **>1 cliente** (ou é agency/admin) → mostra o select; usuário com **1 cliente** (cliente-side)
   → **esconde o select** e a IA entra travada nesse cliente.
4. **Memória**: a IA lê as ÚLTIMAS N mensagens da conversa (suas e dela) — persistidas no banco.

## Fatos do recon (fundam as decisões)

- Persistência de chat do calendário HOJE = zero (nem tabela, nem memória no n8n; o workflow é
  stateless; `conversationId`/`sessionKey` viajam mas são ignorados). Padrão a copiar:
  `tasks.task_comments` (`0108_tasks_schema_foundation.sql:197` + `repository_postgres_collab.go`:
  account-scoped, `order by created_at`, soft-delete).
- `BuildAIContext` (`runtime_context.go:74`) já monta contexto de DADOS de **1 cliente** (perfil C3 +
  feriados + eventos lean + notas + planos), isolado por `account_id`. Falta o modo "todos".
- Visibilidade org-aware canônica (fonte da verdade): `core/store_postgres.go:56` +
  `auth/account_checker.go:23` (platform_admin / agency_owner via `core.organization_users` /
  `core.account_users`). `core.accounts.is_agency` (0158) marca a conta-agência.
- Front: `store.clients` (`stores/calendar.ts:88`) = `tenantsStore.tenants` (já permission-scoped);
  `accountStore.activeAccountId` é a conta ativa (X-Account-Id). Próxima migration livre: **0191**.

---

## Contratos compartilhados

### D1 — Schema (migration 0191)
```sql
create schema if not exists calendar;
create table if not exists calendar.chat_conversations (
  id                 uuid primary key default gen_random_uuid(),
  account_id         uuid not null references core.accounts(id) on delete cascade,
  created_by_user_id uuid not null references core.users(id),
  title              text not null default '',
  scope_mode         text not null default 'client',   -- 'client' | 'all'
  scope_client_id    uuid,                              -- preenchido quando scope_mode='client'
  created_at         timestamptz not null default now(),
  updated_at         timestamptz not null default now(),
  deleted_at         timestamptz
);
create index if not exists calendar_chat_conv_idx
  on calendar.chat_conversations (account_id, created_by_user_id, updated_at desc)
  where deleted_at is null;

create table if not exists calendar.chat_messages (
  id              uuid primary key default gen_random_uuid(),
  conversation_id uuid not null references calendar.chat_conversations(id) on delete cascade,
  account_id      uuid not null references core.accounts(id) on delete cascade,
  role            text not null,   -- 'user' | 'assistant'
  content         text not null,
  created_at      timestamptz not null default now()
);
create index if not exists calendar_chat_msg_idx
  on calendar.chat_messages (conversation_id, created_at);
```

### D2 — Acesso do chat (resolvido server-side, nunca do body)
`resolveChatAccess(ctx, principal, accountID) -> ChatAccess{ IsAgency bool, VisibleClientIDs []string }`:
- `IsAgency` = `principal.is_platform_admin` OU `agency_owner` em `core.organization_users` da org da
  conta ativa (query espelhando `account_checker`/`core/store_postgres`).
- `VisibleClientIDs` = clientes que o usuário PODE ver (reusar `tenants/scope_queries.go`
  `buildListAccessibleQuery` OU a mesma lógica; **resolvido no back**, não confia na lista do front).
- **Regra do select**: `canSelectScope = IsAgency || len(VisibleClientIDs) > 1`. Se `false` e há 1
  cliente → `lockedClientId = VisibleClientIDs[0]`.
- **Validação de escopo no ask**: `scopeClientId` DEVE estar em `VisibleClientIDs` (senão 404
  `invalid_client`); `scopeMode='all'` só é aceito se `canSelectScope` (senão força `client` no
  `lockedClientId`). Multi-tenant: nunca dá contexto de cliente que o usuário não pode ver.

### D3 — Conversa e mensagens (API do painel)
```jsonc
// GET /v1/calendar/chat/conversations  (RequireAuthWithAccount)
//   Agency -> TODAS da conta; cliente-side -> so as created_by = ele. Lean:
{ "conversations": [{ "id","title","scopeMode","scopeClientId","createdByUserId","createdByName","updatedAt" }] }

// GET /v1/calendar/chat/conversations/{id}  -> conversa + mensagens (order by created_at)
{ "id","title","scopeMode","scopeClientId", "messages": [{ "id","role","content","createdAt" }] }
//   Acesso: dono OU IsAgency; fora disso 404.

// POST /v1/calendar/chat/conversations  body { scopeMode, scopeClientId?, title? } -> cria (201 {id})
// DELETE /v1/calendar/chat/conversations/{id}  -> soft-delete (deleted_at); dono ou IsAgency.

// GET /v1/calendar/chat/scope  -> alimenta o SELECT do front (acesso resolvido no back)
{ "canSelect": bool, "lockedClientId": "uuid|''", "clients": [{ "id","name" }] }
```

### D4 — Ask com memória + escopo (EXTENDE /v1/calendar/chat/ask)
```jsonc
// body: { "question", "conversationId", "scopeMode": "client|all", "scopeClientId": "uuid|''", "month" }
// Fluxo no ChatAsk (back):
//  1. resolveChatAccess -> valida/normaliza escopo (D2).
//  2. Garante a conversa (cria se conversationId novo; grava scope_mode/scope_client_id;
//     valida ownership/acesso se ja existe).
//  3. Grava a mensagem do usuario (role=user).
//  4. Carrega as ultimas N mensagens (N=12) da conversa -> historico.
//  5. Contexto: scope 'client' -> BuildAIContext(clientId) (perfil completo C3); scope 'all' ->
//     BuildAIContextAll (resumo lean de cada cliente visivel + eventos/notas agregados, com teto).
//  6. Payload n8n: { question, sessionKey, language, ai{...+apiKey}, context, history:[{role,content}] }.
//  7. Grava a resposta (role=assistant); titula a conversa pela 1a pergunta (se vazio); updated_at.
// -> 200 { "answer", "conversationId", "title" }
```
`BuildAIContextAll(account, visibleClientIDs)`: para cada cliente (teto 30) um item lean
`{ id, name, segment, brandVoice(resumo) }` do perfil; + eventos lean do mês (todos os clientes, teto
100) + nota do mês. Token-bounded (truncar). Reusa as queries de `store_ai_context.go` (sem N+1).

### D5 — Workflow n8n (history na conversa)
`workflow-calendar-chat.json` — nó "Montar contexto": a array de `messages` do LLM passa a ser
`[{role:'system', content: system+context}, ...body.history, {role:'user', content: body.question}]`
(hoje é só system+user). O Go manda `history` no payload. Sem memória no n8n (a fonte é o banco).

---

## LANE BACK (sequencial)

### SPEC-B10 — Migration 0191 + store de conversas/mensagens + acesso
- Migration `0191_calendar_chat_conversations.sql` (D1).
- `chat_store.go` (novo): CRUD de conversas (create/get/list/soft-delete, account-scoped + regra
  agency/own) + mensagens (append/listLastN). Espelhar `repository_postgres_collab.go`.
- `chat_access.go` (novo): `resolveChatAccess` (D2) — IsAgency (query org-aware) + VisibleClientIDs
  (reusar/espelhar `tenants` scope). Sem confiar no body.
- Aceite: agency lista todas; cliente-side lista só as suas; conversa de outra conta => 404;
  scopeClientId fora do visível => 404.

### SPEC-B11 — Ask com memória + escopo + BuildAIContextAll + endpoints
- `chat.go` ChatAsk (D4): persiste user msg, carrega history (N=12), valida escopo, monta contexto
  (client OU all), manda `history` no payload, persiste a resposta, titula/atualiza a conversa.
- `runtime_context.go`: `BuildAIContextAll` (D4) — agregado lean multi-cliente (teto 30) + eventos/nota.
- `http_chat.go`: rotas D3 (conversations list/get/create/delete + /chat/scope), todas
  `RequireAuthWithAccount`. `/ask` estende o body.
- `chat.go` chatWebhookPayload: campo `History []ChatMessage` (role/content).
- Aceite: 2 perguntas na mesma conversa -> a 2a resposta considera a 1a (memória); escopo 'all' só
  p/ agency; recarregar a página e reabrir a conversa mostra o histórico do banco.

## LANE N8N

### SPEC-W5 — Chat workflow com history
- `workflow-calendar-chat.json` "Montar contexto" (D5): monta `messages` = system+context, depois
  `...body.history`, depois a pergunta. Validar JSON. Doc + AGENT.md.

## LANE FRONT (sequencial)

### SPEC-F10 — Persistência + lista de conversas + memória
- `domain/calendar/calendar-api.ts`: fetch/list/get/create/delete conversations + fetch scope (D3).
- `useCalendarChat.ts`: ao abrir o chat, busca a lista + o escopo; `ask()` manda scopeMode/scopeClientId
  e conversationId; a resposta atualiza conversationId/title; `openConversation(id)` carrega mensagens
  do banco; `newConversation()` cria nova. Estado de "carregando conversa".
- `CalendarChatPanel.vue`: header ganha um botão/menu "Conversas" (lista as conversas persistidas —
  agency vê todas com o autor; cliente só as suas) + "Nova conversa".
- Aceite: conversa persiste após reload; reabrir mostra o histórico; nova conversa começa limpa.

### SPEC-F11 — Select de escopo (Todos / cliente) com controle de acesso
- `CalendarChatPanel.vue`: um SELECT no header (ou perto do input) alimentado por `GET /chat/scope`:
  `canSelect=false` => NÃO renderiza o select (usuário-cliente), a IA usa `lockedClientId`;
  `canSelect=true` => opções "Todos os clientes" + cada cliente visível. A escolha vai no `ask()`
  (scopeMode/scopeClientId) e fica salva na conversa.
- Aceite: usuário-cliente (1 cliente) não vê o select e a IA responde com os dados do cliente dele;
  agency vê o select, troca entre Todos/cliente, e o contexto muda de acordo.

---

## Ordem/Dependências
- BACK B10 -> B11 (mesmo pacote). N8N W5 (independe, casa com B11). FRONT F10 -> F11 (F11 usa o
  /chat/scope de B11; F10/F11 codam contra os contratos). Lanes em paralelo.
- Depois: build api + migration 0191, reimportar o workflow, revisão adversarial (isolamento
  multi-tenant do acesso às conversas + do contexto multi-cliente), testar, docs.
- Segurança: acesso às conversas e ao contexto de clientes SEMPRE resolvido server-side pela
  permissão (nunca do body); conversa/cliente fora do visível => 404.

## Progress Log
| Quando | Etapa | Status | Notas |
| --- | --- | --- | --- |
| 2026-07-04 | Specs escritas (recon + decisões do dono) | ok | Conversas híbridas (cliente vê as suas, agência vê todas); escopo Todos+cliente (org depois); acesso derivado do modelo existente. Aguardando OK p/ implementar. |
| 2026-07-05 | Implementação (5 agentes: B10-B11, W5, F10-F11) | ok | Migration 0191 (chat_conversations+chat_messages) + chat_store/chat_access (org-aware, VisibleClientIDs reusa tenants.Service.ListAccessible via WithClientScope) + ChatAsk com memória (history N=12) + BuildAIContextAll + endpoints D3 (conversations/scope) sob RequireAuthWithAccount + workflow com history + front (persistência, lista de conversas, select de escopo). golangci 0 issues; memória validada via webhook (a IA lembrou da msg anterior). |
| 2026-07-05 | Revisão adversarial (4 revisores) + correções | ok | 2 CRÍTICOS: (1) modo 'all' listava eventos de TODOS os clientes da conta → novo `ListEventsLeanForClients` filtra por `client_id = ANY(visíveis) OR NULL`; (2) GetChatConversation/resolveChatTarget não revalidavam o escopo salvo contra o acesso ATUAL → novo `canAccessSavedScope` nega 404 quem perdeu acesso ao cliente (fecha o GET E o history replay). 2 MENORES: planos (metadata da conta) só p/ agência no 'client'; history replay coberto pelo fix #2. build/vet/golangci limpos; api rebuildada. |
| 2026-07-04 | SPEC-B10 (migration 0191 + chat_store + chat_access) | ok | Fundação: tabelas calendar.chat_*, CRUD account-scoped + IsAgencyOfAccount + resolveChatAccess/validateScope/authorizeConversation. |
| 2026-07-04 | SPEC-B11 (ask com memória + escopo + BuildAIContextAll + endpoints D3) | ok | ChatAsk reescrito (persiste conversa + history N=12, escopo normalizado server-side, IA checada antes de materializar); `BuildAIContextAll` (agregado lean multi-cliente, teto 30/100 + trunc); `chat_conversations.go` (novo) = rotas D3 (list/get/create/delete/scope); payload ganhou `history[{role,content}]` + `context any`; `writeChatError` mapeia `ErrInvalidClient`=>404. build/vet/golangci limpos. **Falta**: rebuild da api (migration 0191 no startup, back/ mudou) + LANE N8N W5 (history no workflow) + LANE FRONT F10/F11. |
