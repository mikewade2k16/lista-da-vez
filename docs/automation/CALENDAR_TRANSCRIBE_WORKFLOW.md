# Workflow "Calendar Transcribe" (n8n) — transcricao de voz (OpenAI Whisper OU Gemini)

> Workflow que transcreve o audio gravado no chat de voz do modulo Calendario (SPEC-F7)
> em texto. Fonte da visao: [../CALENDARIO_PLAN.md](../CALENDARIO_PLAN.md) (§3.10) e a spec
> SPEC-W2 em [../CALENDARIO_SPECS3.md](../CALENDARIO_SPECS3.md) (contratos PAY/SEC). Contrato C8.
> JSON importavel: [`../../automation/export/workflow-calendar-transcribe.json`](../../automation/export/workflow-calendar-transcribe.json).
>
> **Status: autorado — precisa de import + teste manual no n8n.** Nunca foi executado no n8n
> real. O JSON foi validado fora do n8n (parse, conexoes, sintaxe dos nos Code, ramos do Switch).
>
> **Mudanca da wave 3 (2026-07-04):** a transcricao virou multi-provider **sem credential do n8n**.
> O painel/back e a fonte da verdade: o Go resolve a API key no banco (`resolveAIKey`) e a manda
> **no payload** (campo `apiKey` do multipart), junto de `provider` (openai|gemini) e `model`.
> Nao ha mais credential OpenAI aqui — a versao anterior usava `sCzmqFisO8bdeZ9B`; agora nenhuma.

---

## 1. Papel no fluxo

O n8n **nunca fala com o Postgres direto**. Quem orquestra e o back Go do Calendario (SPEC-B2):

```
Painel Calendario (mic)  --grava audio (MediaRecorder, webm/opus)-->
   POST /v1/calendar/chat/transcribe  (multipart, campo "file")  -->  API Go
        API Go: kill switch (ai.enabled) + resolve provider/model/apiKey da config+secrets,
        valida mime + tamanho (max 15 MiB) e REPASSA o multipart:
        POST http://n8n:5678/webhook/calendar-transcribe
             (file + campos provider + apiKey + model)
             |
             v
        n8n "Calendar Transcribe"
          Webhook (responseNode; recebe o binario + os campos do form)
             -> Preparar (normaliza provider: openai|gemini)
             -> Switch por provider
                  openai -> Transcrever (OpenAI Whisper)  ------------\
                  gemini -> Montar base64 -> Gemini generateContent -> Extrair texto
             -> Respond to Webhook  { "text": "..." }
             |
             v
        API Go devolve { "text": "..." } ao painel; o texto entra no INPUT do chat
        (o usuario revisa e envia — a transcricao nao dispara a pergunta sozinha).
```

O Go so repassa e devolve; nada e gravado em disco (nem no back, nem no n8n).

**Kill switch / sem key (contrato PAY):** o Go NEM dispara quando `ai.enabled=false` (409
`ai_disabled`) ou quando a key resolvida e vazia (409 `ai_key_missing`). O webhook so recebe
requests com `apiKey` preenchida.

---

## 2. Nos do workflow (8 nos)

| No | Tipo | O que faz |
|---|---|---|
| **Webhook** | `n8n-nodes-base.webhook` v2.1 | `POST` path `calendar-transcribe`, `responseMode: responseNode`. Recebe o `multipart/form-data`; os campos `provider`/`apiKey`/`model` chegam em `$json.body`; o arquivo do campo `file` vira **binario** na propriedade `data0` (ver §7 — prefixo `data` + indice 0). |
| **Preparar** | `n8n-nodes-base.code` v2 | Normaliza `body.provider` para `openai` OU `gemini` (qualquer outro cai em `gemini`) e **repassa o binario** `data0` para os ramos seguintes (reatacha `item.binary`). |
| **Switch por provider** | `n8n-nodes-base.switch` v3.2 | Duas regras sobre `$json.provider`: `openai` (output 0) e `gemini` (output 1). |
| **Transcrever (OpenAI Whisper)** | `n8n-nodes-base.httpRequest` v4.2 | `POST https://api.openai.com/v1/audio/transcriptions`, body `multipart-form-data`: `file` = binario `data0` (`formBinaryData`) + `model` = `{{ body.model \|\| 'whisper-1' }}`. Header `Authorization: Bearer {{ body.apiKey }}`. OpenAI responde `{ "text": "..." }` nativo. |
| **Montar base64 (Gemini)** | `n8n-nodes-base.code` v2 | Le o binario `data0` via `getBinaryDataBuffer` -> base64; monta `contents[].parts` com `inline_data` (mime do audio + base64) + um `text` "Transcreva este audio em pt-BR, responda so a transcricao."; resolve `url = {geminiBase}/models/{model}:generateContent`, `apiKey`, `mime`. |
| **Gemini generateContent** | `n8n-nodes-base.httpRequest` v4.2 | `POST {{ url }}` (JSON), header `x-goog-api-key: {{ apiKey }}` + `content-type: application/json`, body = `reqBody`. `onError: continueRegularOutput`. |
| **Extrair texto (Gemini)** | `n8n-nodes-base.code` v2 | Extrai `candidates[0].content.parts[].text` (concatena os `parts`); erro/vazio -> `{ text: "" }`. |
| **Respond to Webhook** | `n8n-nodes-base.respondToWebhook` v1.1 | `respondWith: json`, body `={{ { "text": $json.text } }}`. Ambos os ramos convergem aqui. |

Conexoes: `Webhook -> Preparar -> Switch por provider`; ramo openai `-> Transcrever (OpenAI Whisper) -> Respond`; ramo gemini `-> Montar base64 -> Gemini generateContent -> Extrair texto -> Respond`.

**Defaults:** `provider` default `gemini`; `model` default `whisper-1` (openai) / `gemini-2.5-flash`
(gemini); `geminiBase` = `https://generativelanguage.googleapis.com/v1beta`; `mime` default
`audio/webm` (quando o binario nao trouxer `mimeType`).

**Por que a key vai por header e nao na URL do Gemini:** a spec permite `?key=...` OU header; usamos
`x-goog-api-key` para a key **nao aparecer na URL** (logs/histórico de execucao do n8n gravam a URL).

---

## 3. Credential

**Nenhuma.** A partir da wave 3 a transcricao **nao usa credential do n8n** — a API key crua vem no
payload (campo `apiKey` do multipart), resolvida server-side pelo Go a partir do banco
(`calendar.ai_secrets` da conta OU `platform_settings.calendar_ai_secrets` global, conforme
`ai.useGlobalKeys`). A key **nunca** e persistida no n8n, nem logada, nem gravada em `.env`.

> Historico: a versao anterior (wave 2) reusava a credential OpenAI `sCzmqFisO8bdeZ9B` ("OpenAI
> account"). Foi **removida** deste workflow. A credential ainda existe e e usada pelo bot WhatsApp
> (`workflow-whatsapp.json`), so nao mais aqui.

---

## 4. Env do Omni (back Go — C8 / PAY)

| Env | Para que |
|---|---|
| `CALENDAR_TRANSCRIBE_WEBHOOK_URL` | URL do webhook do n8n. Local/prod: `http://n8n:5678/webhook/calendar-transcribe`. Vazio -> `POST /v1/calendar/chat/transcribe` responde **503 `transcribe_not_configured`** (mensagem acionavel citando o env). |

As chaves de IA (Gemini/OpenAI) **nao sao mais env** deste workflow — vem no payload. As envs
`CALENDAR_GEMINI_API_KEY`/`CALENDAR_ZAI_API_KEY` (usadas so como diagnostico pelo Calendar Chat)
nao tem papel aqui.

Contrato do endpoint do back (C8 + PAY):

```jsonc
// POST /v1/calendar/chat/transcribe   (RequireAuth + accountScope; multipart campo "file")
// mimes: audio/webm, audio/ogg, audio/mp4, audio/mpeg, audio/wav — max 15 MiB
// o Go injeta provider + apiKey + model (do config/secrets) no multipart repassado ao n8n
// -> 200 { "text": "" } | 400 invalid_media | 413 media_too_large |
//    409 ai_disabled | 409 ai_key_missing |
//    503 transcribe_not_configured | 502/504 (n8n indisponivel/timeout)
```

---

## 5. Como importar

Local (profile `automation`):

```bash
# 1. copia o JSON para dentro do container
docker compose --profile automation cp automation/export/workflow-calendar-transcribe.json n8n:/tmp/wf.json
# 2. importa (Git Bash no Windows: prefixe MSYS_NO_PATHCONV=1 senao /tmp vira path Windows)
MSYS_NO_PATHCONV=1 docker compose --profile automation exec n8n n8n import:workflow --input=/tmp/wf.json --overwrite
# 3. NAO precisa de credential (a key vem no payload). Se o n8n destino tinha a versao antiga
#    com a credential OpenAI, o import --overwrite (mesmo id calendartrans001) substitui o node.
# 4. ativa e reinicia
docker compose --profile automation exec n8n n8n update:workflow --id=calendartrans001 --active=true
docker compose --profile automation restart n8n
```

Prod (VPS): mesmo procedimento com `docker-compose.prod.yml`. O webhook so ouve com o workflow
**Active** e no path `/webhook/` (nao `/webhook-test/`). Como o `versionId` mudou, **reimportar**
para pegar a versao multi-provider.

---

## 6. Como testar

### 6.1 Disparar o webhook direto com um arquivo de audio

O webhook agora exige os campos `provider` + `apiKey` (+ `model` opcional). Responde com o JSON
`{ "text": "..." }`.

```bash
# porta do host local do n8n (profile automation) = 5680 (ver automation/AGENT.md "Portas")

# --- OpenAI Whisper ---
curl -s -X POST http://localhost:5680/webhook/calendar-transcribe \
  -F 'file=@./amostra.ogg;type=audio/ogg' \
  -F 'provider=openai' \
  -F 'apiKey=sk-...' \
  -F 'model=whisper-1'
# -> {"text":"...transcricao..."}

# --- Gemini ---
curl -s -X POST http://localhost:5680/webhook/calendar-transcribe \
  -F 'file=@./amostra.ogg;type=audio/ogg' \
  -F 'provider=gemini' \
  -F 'apiKey=AIza...' \
  -F 'model=gemini-2.5-flash'
# -> {"text":"...transcricao..."}
```

Gerar uma amostra rapida (se tiver ffmpeg):

```bash
ffmpeg -f lavfi -i "sine=frequency=440:duration=3" -ac 1 amostra.ogg
```

### 6.2 Testar via back Go (fim a fim)

Depois de subir a API com `CALENDAR_TRANSCRIBE_WEBHOOK_URL` setado, configurar a IA no painel (aba
IA: provider de transcricao + key), e logar para pegar o token:

```bash
curl -s -X POST "http://localhost:9091/v1/calendar/chat/transcribe" \
  -H "Authorization: Bearer <TOKEN>" \
  -H "X-Account-Id: <ACCOUNT_ID>" \
  -F 'file=@./amostra.ogg;type=audio/ogg'
# -> {"text":"..."}
```

IA desligada -> **409 `ai_disabled`**; sem key -> **409 `ai_key_missing`**; sem o env ->
**503 `transcribe_not_configured`**; arquivo > 15 MiB -> **413 `media_too_large`**; mime fora da
whitelist -> **400 `invalid_media`**; n8n fora do ar -> **502**; timeout -> **504**.

---

## 7. Limites e gotchas

- **Nome da propriedade binaria (`data0`):** o Webhook do n8n grava arquivos de `multipart/
  form-data` como `<prefixo><indice>`, com prefixo default `data` e indice comecando em `0` — o
  primeiro (e unico) arquivo vira `data0`. Por isso o ramo openai usa `inputDataFieldName: data0`
  e o Code do Gemini le `getBinaryDataBuffer(0, 'data0')`. **Se a sua versao de n8n nomear
  diferente** (ex.: pelo nome do campo `file`), ajuste ambos — teste com o §6.1 e olhe o binario
  na execucao.
- **Gemini inline_data e o tamanho:** o `generateContent` aceita audio inline (base64) ate ~20 MB
  de request; o back ja limita a **15 MiB** (`http.MaxBytesReader`) e o front para a gravacao em
  **2 min** (SPEC-F7), entao fica dentro. Acima disso o Gemini exigiria a Files API (nao
  implementada). OpenAI Whisper: limite hard 25 MB.
- **Tamanho / duracao:** o workflow **nao** valida tamanho — a barreira e o back.
- **Base64 via helper, nao via `.data`:** o Code do Gemini usa `this.helpers.getBinaryDataBuffer`
  (funciona com binario em memoria E em filesystem, conforme o `N8N_DEFAULT_BINARY_DATA_MODE`);
  ler `item.binary.data0.data` direto so funciona no modo memoria.
- **Nao testado no n8n real.** Autorado e validado fora do n8n (parse, ramos, sintaxe dos Code).
- **Escrita de workflow via n8n-MCP e quebrada** nesta versao (`n8nio/n8n:2.23.2`, PUT publico
  estrito) — importar por CLI (`import:workflow`), nao pela API do MCP.
- **Seguranca da key:** a key crua trafega so no payload server-to-server (Go -> n8n, rede docker).
  O Gemini recebe por header `x-goog-api-key` (nao na URL); nenhum node loga o payload cru.

---

## 8. Alternativas (documentadas, NAO implementadas)

### 8.1 Whisper LOCAL (self-host, sem custo de API) — FUTURO

O enum de provider de transcricao ja preve um provider local (decisao do dono: "Whisper local fica
p/ teste futuro" — deixar preparado, nao implementar agora). Rodar um container proprio de STT
OpenAI-compativel (ex.: `fedirz/faster-whisper-server`, expondo `/v1/audio/transcriptions`) e
adicionar um 3o ramo no Switch (`local`) apontando o mesmo HTTP do ramo openai para
`http://faster-whisper:8000/v1/audio/transcriptions` (sem Bearer, ou token proprio).

- Modelos: `tiny`/`base`/`small` cabem em CPU; `medium`/`large-v3` querem GPU ou muita RAM.
- **CUIDADO com memoria da VPS (AC-11):** o stack `automation` ja limita `redis` 256m / `waha` 1g
  / `n8n` 768m. Um container faster-whisper (mesmo `small` em CPU) come **facil 1-2 GB** — **medir
  o `free -m` da VPS antes** e por `mem_limit`/`mem_reservation` no compose, senao arrisca OOM.
  Provavelmente so vale a pena com GPU dedicada.

### 8.2 Groq (Whisper large v3, free tier)

Outro provider OpenAI-compativel: `https://api.groq.com/openai/v1/audio/transcriptions`,
`model=whisper-large-v3`. Bastaria adicionar um ramo (ou reaproveitar o ramo openai trocando a
`url`) e mandar `provider=groq` + a key no payload. Free tier generoso e latencia baixa; conferir
rate limit e formatos de audio no painel do Groq.

---

## Referencia cruzada
- Visao/escopo do Calendario -> [../CALENDARIO_PLAN.md](../CALENDARIO_PLAN.md)
- Specs desta wave (CFG/SEC/PAY, SPEC-W2) -> [../CALENDARIO_SPECS3.md](../CALENDARIO_SPECS3.md)
- Workflow de chat do Calendario (SPEC-W1) -> [CALENDAR_CHAT_WORKFLOW.md](CALENDAR_CHAT_WORKFLOW.md)
- Infra containers / n8n / portas -> [../../automation/AGENT.md](../../automation/AGENT.md)
