# Workflow "Calendar Omni" (n8n) — plano estrategico do mes por IA

> Workflow que gera o **plano estrategico de conteudo do mes** do modulo Calendario.
> Fonte da visao: [../CALENDARIO_PLAN.md](../CALENDARIO_PLAN.md) (§3.5–3.8) e a spec
> SPEC-W1 em [../CALENDARIO_SPECS.md](../CALENDARIO_SPECS.md). Contratos C2/C4/C5.
> JSON importavel: [`../../automation/export/workflow-calendar-omni.json`](../../automation/export/workflow-calendar-omni.json).
>
> **Status: importado e ativo no n8n de producao em 2026-07-07.** Estrutura exportada da VPS
> confere com o JSON local. O teste com chamada ao modelo continua manual para evitar consumo
> involuntario de tokens durante o deploy.

---

## 1. Papel no fluxo

O n8n **nunca fala com o Postgres direto** (mesmo principio da automacao de WhatsApp). Quem
orquestra e o back Go do Calendario (SPEC-B4):

```
Painel Calendario  --POST /v1/calendar/ai/plan-->  API Go
   API Go cria a row `pending`, responde 201, e dispara em goroutine:
   POST http://n8n:5678/webhook/calendar-omni   (payload C5)
        |
        v
   n8n "Calendar Omni"
     Webhook (responde na hora) -> Montar prompt -> Switch provider
        claude       -> HTTP Claude       (POST {baseUrl}/v1/messages)
        deepseek/... -> HTTP OpenAI compat (POST {baseUrl}/chat/completions)
     -> Extrair JSON -> Callback (POST {callbackUrl} com X-Service-Token)
        |
        v
   API Go callback POST /v1/public/calendar-ai/plans/{id}/result
     valida X-Service-Token (constant-time), transiciona `pending -> done|error`,
     persiste `content` (shape C4). Front faz polling e mostra o plano.
```

O plano (`content`) e salvo pelo back; o front pode aplica-lo como **notas do mes** e/ou
**eventos** nos dias.

---

## 2. Nos do workflow (7 nos)

| No | Tipo | O que faz |
|---|---|---|
| **Webhook** | `n8n-nodes-base.webhook` v2.1 | POST path `calendar-omni`, **responde imediatamente** (respond on received). Recebe o payload C5 no `body`. |
| **Montar prompt** | `code` v2 | Le o `body`; monta `system` (o `ai.systemPrompt` da config OU o DEFAULT pt-BR que instrui saida **JSON estrito** no shape C4.content, sem markdown) e `user` (mes + feriados + clientes/perfis + notas). Resolve `baseUrl` (o da config OU o default por provider do mapa C2), faz clamp de `temperature` (0..1) e repassa `callbackUrl`/`serviceToken`. |
| **Switch provider** | `switch` v3.4 | Saida 0 = `claude`; fallback (saida 1) = demais providers (deepseek/qwen/kimi/glm/custom). |
| **Claude** | `httpRequest` v4.2 | `POST {baseUrl}/v1/messages`, headers `x-api-key` (via credential `calendar-ai-claude`) + `anthropic-version: 2023-06-01`. Body Anthropic (`system` + `messages`). `onError: continueRegularOutput`. |
| **OpenAI compat** | `httpRequest` v4.2 | `POST {baseUrl}/chat/completions`, `Authorization: Bearer` (via credential `calendar-ai-openai-compat`). Body OpenAI (`messages` com role system+user). `onError: continueRegularOutput`. |
| **Extrair JSON** | `code` v2 | Normaliza a resposta (Anthropic `content[0].text` vs OpenAI `choices[0].message.content`), remove cercas de codigo (```), recorta do 1o `{` ao ultimo `}`, `JSON.parse` com try/catch. Item de erro do HTTP node -> `status:'error'`. Devolve `{planId, status, content, error, callbackUrl, serviceToken}`. |
| **Callback** | `httpRequest` v4.2 | `POST {{callbackUrl}}` com header `X-Service-Token: {{serviceToken}}` e body `{status, content, error}`. |

**Por que `x-api-key` por credential e nao inline:** chave de API **nunca** viaja no payload
nem fica no JSON do workflow. Ela mora nas **credentials do n8n** (ver §3). O `baseUrl`,
`model`, `systemPrompt` e `temperature` vem do payload (config do painel).

---

## 3. Credentials a criar no n8n (uma por provider)

Cada credential e do tipo **Header Auth** (`httpHeaderAuth`). Crie na UI do n8n
(Credentials -> New -> Header Auth) ou via `n8n import:credentials`. O JSON referencia estas
duas por id/nome — mantenha os **nomes** abaixo:

| Credential (nome/id) | Provider(s) | Header Name | Header Value |
|---|---|---|---|
| `calendar-ai-claude` | claude (Anthropic) | `x-api-key` | `<sua ANTHROPIC_API_KEY>` |
| `calendar-ai-openai-compat` | deepseek / qwen / kimi / glm / custom | `Authorization` | `Bearer <chave do provider ativo>` |

Notas:
- O no **OpenAI compat** e generico: ele atende **um** provider OpenAI-compativel por vez (o
  que estiver na credential `calendar-ai-openai-compat`). Trocar de provider chines = trocar o
  `Bearer` dessa credential (e o `baseUrl`/`model` na config do painel, §3.8 do plano). Se
  precisar de mais de um provider chines ativo em paralelo, duplique o no + credential
  (`calendar-ai-deepseek`, `calendar-ai-qwen`, ...) e ramifique o Switch por provider.
- **Chaves de API ficam SO aqui (credentials do n8n)** — nunca no `calendar.config` jsonb, nunca
  no front, nunca no payload C5. E a regra dura do C2.
- `N8N_ENCRYPTION_KEY` fixo: nao mudar depois de salvar credenciais (senao quebram).

---

## 4. Envs do Omni (back Go — C5)

O back precisa destes envs para disparar o n8n e receber o callback:

| Env | Para que |
|---|---|
| `CALENDAR_AI_WEBHOOK_URL` | URL do webhook do n8n. Local/prod: `http://n8n:5678/webhook/calendar-omni`. Vazio -> `POST /ai/plan` responde **503 `ai_not_configured`**. |
| `CALENDAR_AI_SERVICE_TOKEN` | Token de servico comparado (constant-time) no callback. **Mesmo valor** no back e no que o back envia no payload (`serviceToken`). |
| `CALENDAR_AI_CALLBACK_BASE` | Base publica da API para o n8n chamar de volta. O back monta `callbackUrl = {base}/v1/public/calendar-ai/plans/{id}/result`. Local: `http://api:8080`. |

O payload C5 que o back manda ao webhook:

```jsonc
{
  "planId": "", "month": "YYYY-MM", "language": "pt-BR",
  "ai": { "provider": "", "model": "", "baseUrl": "", "systemPrompt": "", "temperature": 0.7 },
  "clients": [{ "id": "", "name": "", "profile": { /* C3 sem clientId */ } }],
  "holidays": [{ "date": "", "name": "", "set": "" }],
  "monthNotes": "<html da nota do mes, pode ser vazio>",
  "callbackUrl": "<CALENDAR_AI_CALLBACK_BASE>/v1/public/calendar-ai/plans/{id}/result",
  "serviceToken": "<CALENDAR_AI_SERVICE_TOKEN>"
}
```

Base URLs default por provider (o `Montar prompt` usa quando `ai.baseUrl` vem vazio — espelha C2):
`claude https://api.anthropic.com`, `deepseek https://api.deepseek.com`,
`qwen https://dashscope.aliyuncs.com/compatible-mode/v1`, `kimi https://api.moonshot.cn/v1`,
`glm https://open.bigmodel.cn/api/paas/v4`.

---

## 5. Como importar

Local (profile `automation`):

```bash
# 1. copia o JSON para dentro do container
docker compose --profile automation cp automation/export/workflow-calendar-omni.json n8n:/tmp/wf.json
# 2. importa (Git Bash no Windows: prefixe MSYS_NO_PATHCONV=1 senao /tmp vira path Windows)
docker compose --profile automation exec n8n n8n import:workflow --input=/tmp/wf.json
# 3. cria as credentials calendar-ai-claude e calendar-ai-openai-compat (UI ou import:credentials)
# 4. ativa e reinicia
docker compose --profile automation exec n8n n8n update:workflow --id=calendaromni0001 --active=true
docker compose --profile automation restart n8n
```

Prod (VPS): mesmo procedimento com `docker-compose.prod.yml` (`import:workflow` ->
`update:workflow --id=calendaromni0001 --active=true` -> `restart n8n`). O webhook so ouve com o
workflow **Active** e no path `/webhook/` (nao `/webhook-test/`).

---

## 6. Como testar

### 6.1 Disparar o webhook direto (payload C5 de exemplo)

Use um `callbackUrl` que voce controla (ex.: um `webhook.site`) para ver o resultado, ja que o
callback real exige o back Go rodando.

```bash
curl -s -X POST http://localhost:5680/webhook/calendar-omni \
  -H 'content-type: application/json' \
  -d '{
    "planId": "test-1",
    "month": "2026-08",
    "language": "pt-BR",
    "ai": { "provider": "deepseek", "model": "deepseek-chat", "baseUrl": "", "systemPrompt": "", "temperature": 0.7 },
    "clients": [{ "id": "c1", "name": "Loja Exemplo", "profile": { "segment": "moda", "brandVoice": "jovem e direto" } }],
    "holidays": [{ "date": "2026-08-11", "name": "Dia dos Pais", "set": "brNational" }],
    "monthNotes": "<p>foco em Dia dos Pais</p>",
    "callbackUrl": "https://webhook.site/<seu-id>",
    "serviceToken": "token-de-teste"
  }'
```

O webhook responde na hora (corpo vazio/echo). O plano chega no `callbackUrl` alguns segundos
depois, no shape do callback C4:

```jsonc
{ "status": "done", "content": { "summary": "...", "pillars": [...], "clients": [...] }, "error": "" }
```

### 6.2 Simular o callback no back (curl)

Para validar o endpoint publico do back sem depender do LLM:

```bash
curl -s -X POST "http://localhost:9091/v1/public/calendar-ai/plans/<planId>/result" \
  -H "X-Service-Token: <CALENDAR_AI_SERVICE_TOKEN>" \
  -H 'content-type: application/json' \
  -d '{ "status": "done", "content": { "summary": "ok", "pillars": [], "clients": [] }, "error": "" }'
```

Token errado -> **403**; plano ja `done`/`applied` -> **409**; env ausente -> **503**.

---

## 7. Limitacoes / dividas

- **Sem teste de prompt real no deploy.** Importado/publicado no n8n de prod e comparado com o
  JSON local; a chamada ao provider ficou manual para nao consumir tokens involuntariamente.
- **Um provider OpenAI-compat por vez** (credential unica `calendar-ai-openai-compat`). Multi
  provider chines em paralelo = duplicar no + credential + ramo no Switch (§3).
- **Escrita de workflow via n8n-MCP e quebrada** nesta versao do n8n (PUT publico estrito) —
  importar por CLI (`import:workflow`), nao pela API do MCP. Ver `AGENTS.md` e o
  `back/internal/modules/automation/AGENT.md`.
- **Expressoes do n8n nao suportam optional chaining (`?.`)** — os nos Code usam `(obj || {})`
  e checagens explicitas.
- O `Extrair JSON` depende do LLM devolver JSON valido. O DEFAULT system prompt pede JSON
  estrito, mas se o provider devolver markdown/texto extra, o no tenta recortar do 1o `{` ao
  ultimo `}`; se ainda assim falhar, devolve `status:'error'` com `invalid_json_from_llm` (o
  back marca a row como `error`, o front mostra a falha).
- Os nos de LLM tem `onError: continueRegularOutput` — falha de rede/5xx do provider vira item
  de erro, que o `Extrair JSON` converte em `status:'error'` + `error`, e o callback ainda
  dispara (o back nao fica em `pending` para sempre).

---

## Referencia cruzada
- Visao/escopo do Calendario -> [../CALENDARIO_PLAN.md](../CALENDARIO_PLAN.md)
- Specs dos subagentes (C1–C5, SPEC-W1) -> [../CALENDARIO_SPECS.md](../CALENDARIO_SPECS.md)
- Infra containers / n8n -> [../../automation/AGENT.md](../../automation/AGENT.md)
- Padrao do workflow WhatsApp/Omni Chat -> [OMNI_CHAT_PLAN.md](OMNI_CHAT_PLAN.md), [WORKFLOW.md](WORKFLOW.md)
