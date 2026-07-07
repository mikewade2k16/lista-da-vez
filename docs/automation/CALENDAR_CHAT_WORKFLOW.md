# Workflow "Calendar Chat" (n8n) — chat de IA do Calendario

> Workflow que responde o **chat de IA flutuante** do modulo Calendario. Fonte da visao:
> [../CALENDARIO_PLAN.md](../CALENDARIO_PLAN.md) (§3.9) e a spec SPEC-W2 em
> [../CALENDARIO_SPECS2.md](../CALENDARIO_SPECS2.md). Contrato C7 (payload) + C6 (config `ai`).
> JSON importavel: [`../../automation/export/workflow-calendar-chat.json`](../../automation/export/workflow-calendar-chat.json).
>
> **Status: autorado — precisa de import + criacao das credentials + teste manual no n8n.**
> Nunca foi executado no n8n real. O JSON foi validado fora do n8n (parse, conexoes, ids unicos,
> sintaxe dos nos Code).

---

## 1. Papel no fluxo

O n8n **nunca fala com o Postgres direto** (mesmo principio dos outros workflows do Omni). Quem
orquestra e o back Go do Calendario (SPEC-B6). O back monta o payload C7 (com o bloco `context`
identico ao agregado C9, montado pela MESMA funcao `BuildAIContext`) e faz um POST **sincrono** ao
webhook, esperando `{ "answer": "" }` de volta:

```
Painel Calendario (chat flutuante)
   --POST /v1/calendar/chat/ask-->  API Go
       API Go monta o payload C7 (question + sessionKey + ai + context) e faz
       POST http://n8n:5678/webhook/calendar-chat  (espera a resposta na mesma request)
        |
        v
   n8n "Calendar Chat"
     Webhook (responseNode) -> Montar contexto -> Switch provider
        claude        -> HTTP Claude (POST {baseUrl}/v1/messages) -> Extrair resposta Claude
        demais (gemini/deepseek/...) -> AI Agent (OpenAI Chat Model + Redis Chat Memory)
     -> Respond to Webhook { "answer": ... }
        |
        v
   API Go devolve { "answer": "" } pro painel.
```

A memoria da conversa fica **no Redis do n8n** (por `sessionKey`), no ramo do AI Agent. O back Go
**nao persiste** as mensagens — o historico some ao recarregar o painel (limitacao v1, documentada
no front).

---

## 2. Nos do workflow (9 nos)

| No | Tipo | O que faz |
|---|---|---|
| **Webhook** | `n8n-nodes-base.webhook` v2.1 | POST path `calendar-chat`, `responseMode=responseNode` (responde pelo no Respond). Recebe o payload C7 no `body`. |
| **Montar contexto** | `code` v2 | Le o `body`; monta `system` (o `ai.systemPrompt` da config OU o DEFAULT pt-BR "assistente de estrategia de conteudo" + o `context` serializado: perfil do cliente, feriados, eventos, notas, planos). Resolve `provider`/`model`/`baseUrl`/`temperature` (mapa `DEFAULT_BASE` com **gemini** OpenAI-compatible incluido; clamp de temperature 0..1). Repassa `question` e `sessionKey`. |
| **Switch provider** | `switch` v3.4 | Saida 0 = `claude` (ramo HTTP stateless); fallback (saida 1) = demais providers OpenAI-compat (gemini/deepseek/qwen/kimi/glm/custom) -> ramo do AI Agent com memoria. |
| **Claude** | `httpRequest` v4.2 | `POST {baseUrl}/v1/messages`, headers `x-api-key` (via credential `calendar-ai-claude`) + `anthropic-version: 2023-06-01`. Body Anthropic (`system` + `messages` com a `question`). `onError: continueRegularOutput`. **Sem memoria de conversa** (ver §7). |
| **Extrair resposta Claude** | `code` v2 | Normaliza a resposta Anthropic (`content[0].text`) para `{ output }`; item de erro do HTTP node vira mensagem acionavel. |
| **AI Agent** | `@n8n/n8n-nodes-langchain.agent` v3.1 | `promptType=define`, `text = question`, `systemMessage = system`. **Sem tools** (estao quebradas no n8n 2.23.2). Saida em `$json.output`. |
| **OpenAI Chat Model** | `@n8n/n8n-nodes-langchain.lmChatOpenAi` v1.3 | Chat model do agente com **baseURL dinamico** (`options.baseURL = {{ ...baseUrl }}`) apontando pro provider OpenAI-compativel ativo. Credential `calendar-ai-gemini` (tipo `openAiApi`). |
| **Redis Chat Memory** | `@n8n/n8n-nodes-langchain.memoryRedisChat` v1.6 | Memoria da conversa por `sessionKey` do payload (janela 10, TTL 3600s). Credential `Redis account` (a mesma do workflow WhatsApp). |
| **Respond to Webhook** | `n8n-nodes-base.respondToWebhook` v1.1 | Responde `{{ { "answer": $json.output } }}`. Recebe `output` do AI Agent OU do Extrair resposta Claude. |

Os dois ramos (claude vs agent) convergem no mesmo **Respond to Webhook**: ambos deixam a resposta
em `$json.output`.

**Por que o `sessionKey` do payload:** o back monta `sessionKey = <accountId>|<userId>|<conversationId>`
(C7). Isso isola a memoria Redis por conta + usuario + conversa. Trocar de conversa no painel
(novo `conversationId`) = nova memoria.

---

## 3. Credentials a criar no n8n

| Credential (nome/id) | Tipo | Usada por | Conteudo |
|---|---|---|---|
| `calendar-ai-gemini` | **openAiApi** | no `OpenAI Chat Model` (lmChatOpenAi) | API key do **Google AI Studio** + Base URL `https://generativelanguage.googleapis.com/v1beta/openai` |
| `calendar-ai-claude` | **Header Auth** (`httpHeaderAuth`) | no `Claude` (httpRequest) | Header `x-api-key` = `<ANTHROPIC_API_KEY>` (ja existe do workflow Calendar Omni) |
| `Redis account` | **Redis** | no `Redis Chat Memory` | host/port/senha do Redis do stack automation (ja existe do workflow WhatsApp, id `pkxksfWdwYDbv6B3`) |

Notas importantes:
- **O TIPO da credential do provider OpenAI-compat depende do no.** No ramo do AI Agent o chat model
  e o `lmChatOpenAi`, que so aceita credential **`openAiApi`** (API key + baseURL). Bearer / Header
  Auth **so** vale em no `httpRequest` (o padrao do Calendar Omni). Por isso o gemini aqui e uma
  credential `openAiApi`, nao Header Auth.
- **Um provider OpenAI-compat ativo por vez.** A credential `calendar-ai-gemini` guarda **uma** API
  key. Para usar deepseek/qwen/kimi/glm no lugar do gemini, troque a API key **e** a Base URL dessa
  credential (o `baseURL` dinamico do no ainda vem do payload/config, mas a chave e uma so). Mesma
  limitacao do Calendar Omni. Se precisar de mais de um provider em paralelo, duplique o no chat
  model + credential e ramifique o Switch.
- **Chaves de API ficam SO nas credentials do n8n** — nunca no `calendar.config` jsonb, nunca no
  front, nunca no payload C7. Regra dura do C6/C2.
- `N8N_ENCRYPTION_KEY` fixo: nao mudar depois de salvar credenciais (senao quebram).
- Provider default do chat em dev = **gemini** (free tier do AI Studio). Se preferir claude, ele
  responde stateless (ver §7).

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
    "events": [{ "date": "", "type": "", "title": "", "status": "", "clientId": "" }], // lean, max 100
    "plans": [{ "id": "", "month": "", "status": "", "provider": "", "model": "" }]    // lean, max 10
  }
}
```

Base URLs default por provider (o `Montar contexto` usa quando `ai.baseUrl` vem vazio — espelha
C2/C6): `claude https://api.anthropic.com`,
**`gemini https://generativelanguage.googleapis.com/v1beta/openai`**,
`deepseek https://api.deepseek.com`, `qwen https://dashscope.aliyuncs.com/compatible-mode/v1`,
`kimi https://api.moonshot.cn/v1`, `glm https://open.bigmodel.cn/api/paas/v4`.

> A URL OpenAI-compatible do Google AI Studio pode mudar — conferir na doc do Google ao configurar
> a credential `calendar-ai-gemini`. O `lmChatOpenAi` chama `{baseURL}/chat/completions`.

---

## 5. Como importar

Local (profile `automation`):

```bash
# 1. copia o JSON para dentro do container
docker compose --profile automation cp automation/export/workflow-calendar-chat.json n8n:/tmp/wf-chat.json
# 2. importa (Git Bash no Windows: prefixe MSYS_NO_PATHCONV=1 senao /tmp vira path Windows)
MSYS_NO_PATHCONV=1 docker compose --profile automation exec n8n n8n import:workflow --input=/tmp/wf-chat.json --overwrite
# 3. cria/garante as credentials calendar-ai-gemini (openAiApi), calendar-ai-claude (Header Auth) e
#    Redis account (Redis) na UI do n8n
# 4. ativa e reinicia
docker compose --profile automation exec n8n n8n update:workflow --id=calendarchat0001 --active=true
docker compose --profile automation restart n8n
```

Prod (VPS): mesmo procedimento com `docker-compose.prod.yml` (`import:workflow` ->
`update:workflow --id=calendarchat0001 --active=true` -> `restart n8n`). O webhook so ouve com o
workflow **Active** e no path `/webhook/` (nao `/webhook-test/`).

---

## 6. Como testar

### 6.1 Disparar o webhook direto (payload C7 de exemplo)

O webhook e sincrono: o `curl` recebe `{ "answer": "" }` na propria resposta. Use o provider
`gemini` (free tier) para testar sem gastar credito.

```bash
curl -s -X POST http://localhost:5680/webhook/calendar-chat \
  -H 'content-type: application/json' \
  -d '{
    "question": "Sugira 3 ideias de post para o Dia dos Pais para esse cliente.",
    "sessionKey": "acc-1|user-1|conv-1",
    "language": "pt-BR",
    "ai": { "provider": "gemini", "model": "gemini-2.0-flash", "baseUrl": "", "systemPrompt": "", "temperature": 0.7 },
    "context": {
      "month": "2026-08",
      "client": { "id": "c1", "name": "Loja Exemplo", "profile": { "segment": "moda", "brandVoice": "jovem e direto" } },
      "holidays": [{ "date": "2026-08-11", "name": "Dia dos Pais", "set": "brNational" }],
      "monthNotes": "<p>foco em Dia dos Pais</p>",
      "events": [{ "date": "2026-08-05", "type": "gravacao", "title": "Gravacao de reels", "status": "scheduled", "clientId": "c1" }],
      "plans": [{ "id": "p1", "month": "2026-08", "status": "done", "provider": "gemini", "model": "gemini-2.0-flash" }]
    }
  }'
```

Resposta esperada (sincrona):

```jsonc
{ "answer": "1) ... 2) ... 3) ..." }
```

Testar a **memoria por sessionKey**: mande uma 2a request com o **mesmo** `sessionKey` (ex.:
"e para o cliente 2?") — o agente deve lembrar do contexto anterior. Mudar o `conversationId` (parte
final do `sessionKey`) zera a memoria.

### 6.2 Testar o ramo claude (stateless)

Troque `"provider": "claude"` (e `"model": "claude-sonnet-5"`, ou deixe o default). A request vai
pelo no HTTP `Claude` (credential `calendar-ai-claude`), **sem** passar pela memoria Redis — cada
pergunta e independente (ver §7).

### 6.3 Pelo back Go

Quando o back (SPEC-B6) estiver rodando: `POST /v1/calendar/chat/ask` com `{ question,
conversationId, clientId?, month? }` (RequireAuth + accountScope). O back monta o `sessionKey` e o
`context` (via `BuildAIContext`) e repassa ao webhook. Sem `CALENDAR_CHAT_WEBHOOK_URL` -> 503
`chat_not_configured`; n8n fora do ar -> 502/504.

---

## 7. Limitacoes / dividas

- **Nao testado no n8n real.** Autorado e validado fora do n8n (parse, conexoes, ids unicos, sintaxe
  dos nos Code). Precisa de import + credentials + teste manual antes de confiar em prod.
- **Memoria so no ramo do AI Agent (providers OpenAI-compat).** A memoria de conversa e o
  `Redis Chat Memory` por `sessionKey`, ligado **so** ao AI Agent. O ramo **claude usa HTTP Request
  direto (`/v1/messages`) e e stateless na v1** — cada pergunta e independente, sem historico. Para
  ter memoria com claude seria preciso um chat model langchain do Anthropic com o mesmo no de
  memoria (fora do escopo desta wave). **Recomendacao dev: usar `gemini` como provider default do
  chat** (free tier + memoria). Registrado tambem no C7 (o Go nao persiste mensagens).
- **Tools desligadas.** O AI Agent roda **sem tools** — elas estao quebradas no n8n 2.23.2 (mesma
  limitacao do Omni Chat). O assistente responde so com o `context` serializado no system prompt;
  nao consulta o banco/API em runtime. Ligar os workflows a `GET /v1/runtime/calendar/context`
  (C9) para busca dinamica e **proximo passo** do plano de automacao (fora da wave 2).
- **Um provider OpenAI-compat por vez** (credential unica `calendar-ai-gemini`). Multi provider em
  paralelo = duplicar o chat model + credential + ramo no Switch (§3).
- **URL OpenAI-compat do gemini pode mudar** — conferir na doc do Google AI Studio. O `lmChatOpenAi`
  chama `{baseURL}/chat/completions`; a base default e `.../v1beta/openai`.
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
