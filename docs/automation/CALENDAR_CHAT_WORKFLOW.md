# Workflow "Calendar Chat" (n8n) — chat de IA do Calendario

> Workflow que responde o **chat de IA flutuante** do modulo Calendario. Fonte da visao:
> [../CALENDARIO_PLAN.md](../CALENDARIO_PLAN.md) (§3.9) e a spec SPEC-W2 em
> [../CALENDARIO_SPECS2.md](../CALENDARIO_SPECS2.md). Contrato C7 (payload) + C6 (config `ai`).
> JSON importavel: [`../../automation/export/workflow-calendar-chat.json`](../../automation/export/workflow-calendar-chat.json).
>
> **Status:** a versão anterior foi importada em produção em 2026-07-07. O JSON local agora está na
> revisao `calendarcht07` (campos completos de proposta + `context.tasks` do board configurado) e precisa ser reimportado apos o deploy desta mudanca.

---

## 1. Papel no fluxo

O n8n **nunca fala com o Postgres direto** (mesmo principio dos outros workflows do Omni). Quem
orquestra e o back Go do Calendario (SPEC-B6). O back monta o payload C7 (com o bloco `context`
identico ao agregado C9, montado pela MESMA funcao `BuildAIContext`) e faz um POST **sincrono** ao
webhook, esperando `{ "answer": "", "eventIds": [], "proposals": [] }` de volta:

```
Painel Calendario (chat flutuante)
   --POST /v1/calendar/chat/ask-->  API Go
       API Go monta o payload C7 (question + sessionKey + ai + context) e faz
       POST http://n8n:5678/webhook/calendar-chat  (espera a resposta na mesma request)
        |
        v
   n8n "Calendar Chat"
     Webhook -> Montar contexto -> Chamar LLM (OpenAI-compatible)
       -> Extrair resposta -> Respond to Webhook
        |
        v
   API Go valida eventIds, persiste a mensagem rica e devolve o snapshot pro painel.
```

A memória da conversa fica em **`calendar.chat_messages` no Postgres**. O n8n é stateless e recebe
somente as últimas mensagens no campo `history`. Propostas e cards continuam no histórico após reload.

---

## 2. Nós do workflow (5 nós)

| No | Tipo | O que faz |
|---|---|---|
| **Webhook** | `n8n-nodes-base.webhook` v2.1 | POST path `calendar-chat`, `responseMode=responseNode` (responde pelo no Respond). Recebe o payload C7 no `body`. |
| **Montar contexto** | `code` v2 | Monta `system + context + history`; serializa eventos reais com ID, `taskId`, horario, cliente, status, prioridade, descricao e midia, e tambem `context.tasks` do board configurado; exige JSON com `answer/proposals/eventIds`. Quando `eventIds` tiver itens, o `answer` deve ser so uma sintese curta; a lista completa aparece nos cards do Go. |
| **Chamar LLM** | `httpRequest` v4.2 | Chama o endpoint OpenAI-compatible configurado usando a key enviada server-to-server pelo Go. |
| **Extrair resposta** | `code` v2 | Normaliza `answer`, `proposals[]` e `eventIds`; preserva campos de create/update/delete (`priority`, `description`, `responsibleId`, `involvedIds`, datas de task etc.); erros do provedor viram mensagem acionável com `aiError=true`. Erros 502/504 antes desse nó indicam n8n/webhook indisponível ou timeout, não uma resposta ruim do modelo. |
| **Respond to Webhook** | `respondToWebhook` v1.1 | Responde `{answer, proposals, eventIds, aiError}`. |

O `sessionKey` continua identificando a conversa no payload, mas a memória autoritativa é o
`history` carregado do Postgres pelo Go; o workflow não mantém estado próprio.

---

## 3. Credenciais

O workflow não usa credentials do n8n. A API resolve a chave efetiva da conta e a envia apenas na
requisição server-to-server em `body.ai.apiKey`; ela não é devolvida ao navegador nem persistida no n8n.

---

## 4. Envs do Omni (back Go — C7)

O back precisa deste env para disparar o webhook:

| Env | Para que |
|---|---|
| `CALENDAR_CHAT_WEBHOOK_URL` | URL do webhook do n8n. Local/prod: `http://n8n:5678/webhook/calendar-chat`. Vazio -> `POST /v1/calendar/chat/ask` responde **503 `chat_not_configured`** (mensagem acionavel citando o env). |

O payload C7 que o back manda ao webhook:

```jsonc
{
  "question": "",
  "sessionKey": "<accountId>|<userId>|<conversationId>",
  "language": "pt-BR",
  "ai": { "provider": "", "model": "", "baseUrl": "", "systemPrompt": "", "temperature": 0.7 },
  "context": {
    "month": "YYYY-MM",
    "client": { "id": "", "name": "", "profile": { /* C3 sem clientId */ } },  // ou null
    "holidays": [{ "date": "", "name": "", "set": "" }],
    "monthNotes": "<html, pode ser vazio>",
    "events": [{ "id": "", "taskId": "", "date": "", "time": "", "type": "", "title": "", "status": "", "priority": "", "responsibleId": "", "involvedIds": [], "clientId": "", "clientName": "", "description": "", "media": [] }], // max 100
    "tasks": [{ "id": "", "boardId": "", "columnId": "", "title": "", "status": "", "priority": "", "dueDate": "", "dueEndDate": "", "responsibleId": "", "involvedIds": [], "clientId": "", "clientName": "", "type": "", "description": "" }], // max 100, board configurado
    "plans": [{ "id": "", "month": "", "status": "", "provider": "", "model": "" }]    // lean, max 10
  }
}
```

Base URLs default por provider (o `Montar contexto` usa quando `ai.baseUrl` vem vazio — espelha
C2/C6): **`gemini https://generativelanguage.googleapis.com/v1beta/openai`**,
`deepseek https://api.deepseek.com`, `qwen https://dashscope.aliyuncs.com/compatible-mode/v1`,
`kimi https://api.moonshot.cn/v1`, `glm https://api.z.ai/api/paas/v4`, `openai https://api.openai.com/v1`.

> O nó HTTP chama `{baseURL}/chat/completions`; a key vem em `body.ai.apiKey` somente no tráfego interno.

---

## 5. Como importar

Local (profile `automation`):

```bash
# 1. copia o JSON para dentro do container
docker compose --profile automation cp automation/export/workflow-calendar-chat.json n8n:/tmp/wf-chat.json
# 2. importa (Git Bash no Windows: prefixe MSYS_NO_PATHCONV=1 senao /tmp vira path Windows)
MSYS_NO_PATHCONV=1 docker compose --profile automation exec n8n n8n import:workflow --input=/tmp/wf-chat.json
# 3. ativa e reinicia (não há credentials do n8n neste workflow)
docker compose --profile automation exec n8n n8n update:workflow --id=calendarchat0001 --active=true
docker compose --profile automation restart n8n
```

Prod (VPS): mesmo procedimento com `docker-compose.prod.yml` (`import:workflow` ->
`update:workflow --id=calendarchat0001 --active=true` -> `restart n8n`). O webhook so ouve com o
workflow **Active** e no path `/webhook/` (nao `/webhook-test/`).

---

## 6. Como testar

### 6.1 Disparar o webhook direto (payload C7 de exemplo)

O webhook é síncrono. Este teste chama o provedor e consome a cota correspondente.

```bash
curl -s -X POST http://localhost:5680/webhook/calendar-chat \
  -H 'content-type: application/json' \
  -d '{
    "question": "Sugira 3 ideias de post para o Dia dos Pais para esse cliente.",
    "sessionKey": "acc-1|user-1|conv-1",
    "language": "pt-BR",
    "ai": { "provider": "gemini", "model": "gemini-2.5-flash", "baseUrl": "", "systemPrompt": "", "temperature": 0.7, "apiKey": "<CHAVE_DE_TESTE>" },
    "context": {
      "month": "2026-08",
      "client": { "id": "c1", "name": "Loja Exemplo", "profile": { "segment": "moda", "brandVoice": "jovem e direto" } },
      "holidays": [{ "date": "2026-08-11", "name": "Dia dos Pais", "set": "brNational" }],
      "monthNotes": "<p>foco em Dia dos Pais</p>",
      "events": [{ "id": "e1", "date": "2026-08-05", "time": "09:00", "type": "gravacao", "title": "Gravacao de reels", "status": "planejado", "clientId": "c1", "clientName": "Loja Exemplo", "media": [] }],
      "plans": [{ "id": "p1", "month": "2026-08", "status": "done", "provider": "gemini", "model": "gemini-2.0-flash" }]
    }
  }'
```

Resposta esperada (sincrona):

```jsonc
{ "answer": "1) ... 2) ... 3) ...", "eventIds": [], "proposals": [], "aiError": false }
```

Para testar memória, use o endpoint do **back Go**, pois é ele que persiste e envia o `history`.

### 6.2 Pelo back Go

Quando o back (SPEC-B6) estiver rodando: `POST /v1/calendar/chat/ask` com `{ question,
conversationId, clientId?, month? }` (RequireAuth + accountScope). O back monta o `sessionKey` e o
`context` (via `BuildAIContext`) e repassa ao webhook. Sem `CALENDAR_CHAT_WEBHOOK_URL` -> 503
`chat_not_configured`; n8n fora do ar -> 502/504.

---

## 7. Limitações / dívidas

- **Sem teste de prompt real no deploy.** Importado/publicado no n8n de prod e comparado com o
  JSON local; a chamada ao provider ficou manual para nao consumir tokens involuntariamente.
- **Janela mensal:** o contexto é limitado a 100 eventos do mês. O Go reconhece mês citado em
  `YYYY-MM`, `DD/MM[/AAAA]` e nomes em português; sem mês explícito usa o foco da tela.
- **Teste real de prompt:** continua manual para não consumir tokens involuntariamente no deploy.
- **Escrita de workflow via n8n-MCP e quebrada** nesta versao do n8n (PUT publico estrito) —
  importar por CLI (`import:workflow`), nao pela API do MCP.
- **Expressoes do n8n nao suportam optional chaining (`?.`)** — os nos Code usam `(obj || {})` e
  checagens explicitas.

---

## Referencia cruzada
- Visao/escopo do Calendario -> [../CALENDARIO_PLAN.md](../CALENDARIO_PLAN.md)
- Specs da wave 2 (C6–C12, SPEC-W2/W3) -> [../CALENDARIO_SPECS2.md](../CALENDARIO_SPECS2.md)
- Workflow irmao (transcricao de voz) -> [CALENDAR_TRANSCRIBE_WORKFLOW.md](CALENDAR_TRANSCRIBE_WORKFLOW.md)
- Workflow do plano estrategico -> [CALENDAR_OMNI_WORKFLOW.md](CALENDAR_OMNI_WORKFLOW.md)
- Infra containers / n8n -> [../../automation/AGENT.md](../../automation/AGENT.md)
- Padrao dos workflows -> [OMNI_CHAT_PLAN.md](OMNI_CHAT_PLAN.md), [WORKFLOW.md](WORKFLOW.md)
