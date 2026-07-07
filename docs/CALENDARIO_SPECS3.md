# Calendário — SPECS WAVE 3 (IA 100% pelo painel + janela de chat customizável)

> Fonte da visão: decisões do dono (2026-07-04). Cada spec é ATÔMICA. Progresso no Progress Log.
> Regras gerais: idênticas ao [CALENDARIO_SPECS2.md](CALENDARIO_SPECS2.md) (skill principios-engenharia +
> references da área; NUNCA git/npm/docker; máx 450 linhas/arquivo; pt-BR sem acentos em comentário;
> multi-tenant account_id do accountScope; migrations idempotentes sem `-- +goose`; não remover
> funcionalidade; atualizar o AGENT.md da área; aviso acionável em vez de default silencioso).

## Decisões do dono (a lei desta onda)

1. **O PAINEL é a fonte da verdade da IA do calendário.** O n8n vira EXECUTOR BURRO: recebe
   provider, modelo, baseUrl, prompt, temperatura E **a própria API key** no payload que o Go lê
   do banco. Nada de credential/$env no n8n para as keys de IA. (O token de serviço do runtime
   segue como está.)
2. **API keys MASCARADAS (write-only).** O navegador NUNCA recebe a chave crua de volta — só o
   status `{set, last4}`. Para trocar, o usuário digita uma nova; vazio = manter; ação "limpar".
3. **Escopo com toggle no painel:** `useGlobalKeys` — `true` usa as chaves GLOBAIS da plataforma
   (um conjunto pra todas as contas, editável só por platform_admin), `false` usa as chaves DESTA
   conta. O painel mostra o toggle.
4. **Botão liga/desliga a IA** (`ai.enabled`): kill switch. Desligado → chat/plano/transcrição
   respondem "IA desligada" (acionável), sem chamar provider.
5. **Transcrição selecionável no painel:** OpenAI Whisper OU Gemini (por agora). Whisper local
   fica pra teste futuro (deixar o enum/preparado, não implementar o local agora).
6. **Prompt de comportamento editável no painel** (`ai.systemPrompt`), salvo no banco = lei da IA.
7. **Providers de chat/plano:** Gemini, GLM (z.ai), OpenAI (trocáveis) + os já existentes.
8. **Janela de chat:** tirar o FAB do canto → janela CENTRALIZADA ocupando a largura interna do
   calendário, com MINIMIZAR e FECHAR; posição e tamanho configuráveis no painel como o modal
   (esquerda = largura do painel esquerdo, direita = largura do modal direito, centro = largura
   do calendário), salvo no banco.

---

## Contratos compartilhados

### CFG — CalendarConfig v4 (jsonb `calendar.config`, sem migration)
Estende o C6 (tudo mantido). Bloco `ai` ganha campos + bloco `chat` novo:
```jsonc
"ai": {
  "enabled": true,                 // NOVO: kill switch da IA do calendario
  "useGlobalKeys": true,           // NOVO: true = chaves da plataforma; false = chaves desta conta
  "provider": "gemini",            // gemini | glm | openai | claude | deepseek | qwen | kimi | custom
  "model": "gemini-2.5-flash",
  "baseUrl": "",
  "systemPrompt": "",              // a LEI da IA (comportamento) — editavel no painel
  "temperature": 0.7,
  "transcribeProvider": "gemini",  // NOVO: openai | gemini  (local fica p/ depois)
  "transcribeModel": ""            // NOVO: whisper-1 (openai) | gemini-2.5-flash (gemini); vazio = default
},
"chat": {                          // NOVO: layout da janela de chat (por conta)
  "position": "center",            // center | left | right
  "width": 0,                      // px; 0 = default da posicao
  "height": 0                      // px; 0 = default da posicao
}
```
Defaults nos DOIS lados (Go defaultConfig + TS defaultCalendarConfig/normalizeConfig por seção).
`ai.provider`/`transcribeProvider` sanitizados por enum; `position` ∈ {center,left,right};
width/height clamp (0..2000). **As keys NUNCA moram na config** (vão nos secrets abaixo).

### SEC — Secrets de IA (migration 0189 + platform_settings)
- **Por conta**: tabela `calendar.ai_secrets (account_id uuid, provider text, api_key text,
  updated_by text, updated_at timestamptz, PK(account_id, provider))`, FK account_id→core.accounts
  on delete cascade. Migration **0189** (idempotente, `create schema if not exists calendar;`).
- **Global**: `core.platform_settings` chave `calendar_ai_secrets` = `{ "gemini":"", "glm":"",
  "openai":"" }` (raw). Mesmo padrão do `media_limits`. Só platform_admin escreve.
- **Mask** helper (Go): `mask(key) -> {"set": key!="", "last4": ultimos 4}`. NUNCA devolver raw.
- **Resolver interno** `resolveAIKey(ctx, account, provider) string`: lê a config; se
  `ai.useGlobalKeys` → chave global; senão → `calendar.ai_secrets` da conta. Vazio = "".
- **Endpoints** (todos accountScope, exceto o global que exige platform_admin):
  - `GET /v1/calendar/ai-keys` → `{ "scope": "global|account", "keys": { "gemini": {set,last4},
    "glm": {set,last4}, "openai": {set,last4} } }` (status da FONTE ATIVA, mascarado).
  - `PUT /v1/calendar/ai-keys` body `{ "provider": "gemini|glm|openai", "apiKey": "..." }` —
    grava na conta; `apiKey` vazio = **limpar**. (Só quando `useGlobalKeys=false`.)
  - `GET /v1/calendar/ai-keys/global` (platform_admin) → status mascarado das globais.
  - `PUT /v1/calendar/ai-keys/global` (platform_admin) body `{provider, apiKey}`.
- **Isolamento**: conta A nunca lê/escreve secret de conta B (PK composta + WHERE account_id).
  Front recebe SÓ status; raw só existe server-side no resolver/dispatch.

### PAY — Payloads Go→n8n passam a carregar a KEY (server-to-server, rede docker)
- **Chat** (webhook `calendar-chat`): `ai` ganha `apiKey` (raw, do resolver). O nó "Montar
  contexto" lê `body.ai.apiKey` e o HTTP usa `Authorization: Bearer {{ apiKey }}` — **remover o
  `$env`** introduzido na wave anterior.
- **Transcrição** (webhook `calendar-transcribe`): multipart `file` + campos `provider`
  (openai|gemini) + `apiKey` + `model`. Roteia: openai → `POST {openaiBase}/audio/transcriptions`
  (whisper); gemini → Gemini `generateContent` com áudio inline (base64) → texto. Devolve `{text}`.
- **Plano** (webhook `calendar-omni`): `ai.apiKey` no payload; HTTP usa a key do payload.
- **Kill switch**: `ai.enabled=false` → o Go NEM dispara: chat/transcribe/plan devolvem
  `ai_disabled` (409) com mensagem "IA do calendário desligada — ligue na aba IA".
- **Sem key**: resolver devolve "" → Go devolve `ai_key_missing` (409) acionável ("configure a
  chave do provider X na aba IA" / "peça ao admin as chaves globais").

### CHATUI — Janela de chat (aplicação no front)
`config.chat.{position,width,height}` dirige a janela: `center` = largura da área interna do
calendário; `left` = largura do painel esquerdo; `right` = largura do modal direito. Redimensionável
(arrasto, como OmniEntityDrawer), persistindo em `config.chat` (debounced PUT). Minimizar =
colapsa numa barra/pill re-expansível; Fechar = some (reabre por um gatilho nos controles).

---

## LANE BACK (sequencial)

### SPEC-B1 — Secrets (0189 + global) + config v4 + resolver + endpoints
- Migration `0189_calendar_ai_secrets.sql` (SEC).
- `model.go`: config v4 (CFG) — `Enabled`, `UseGlobalKeys`, `TranscribeProvider`,
  `TranscribeModel` no `AIConfig`; novo `ChatConfig{Position,Width,Height}` em `CalendarConfig`.
  defaultConfig preenche (enabled=true, useGlobalKeys=true, transcribeProvider="gemini",
  chat.position="center"). Enum de provider ganha `openai`.
- `secrets.go` (novo): tipos KeyStatus + service (GetAccountKeyStatus, PutAccountKey,
  GetGlobalKeyStatus, PutGlobalKey via platform_settings, resolveAIKey, mask). `store_secrets.go`
  (novo): CRUD `calendar.ai_secrets` + leitura/escrita da chave global no platform_settings.
- `http_secrets.go` (novo): `RegisterSecretRoutes` (GET/PUT /ai-keys, GET/PUT /ai-keys/global;
  global exige `principal.Role == platform_admin` senão 403). Registrar no module.go.
- `service.go` PutConfig: sanitizar os campos novos (enums, position, clamps).
- Aceite: PUT key da conta → GET devolve `{set:true,last4}` (nunca raw); global só platform_admin;
  conta B não vê secret de A; config v4 volta shape completo p/ conta antiga.

### SPEC-B2 — Dispatch com key no payload + kill switch + transcribe multi-provider
- `chat.go`/`ai_dispatch.go`: antes de disparar, checar `ai.enabled` (senão `ErrAIDisabled` →
  409 `ai_disabled`); resolver a key (`resolveAIKey`); vazia → `ErrAIKeyMissing` → 409
  `ai_key_missing`; incluir `apiKey` no payload C7/C5 (campo `ai.apiKey`).
- `chat.go` transcribe: ler `transcribeProvider`+`transcribeModel` da config, resolver a key
  (provider = openai usa a key `openai`; gemini usa a key `gemini`), montar multipart p/ o n8n
  com `provider`+`apiKey`+`model`+`file`. Kill switch idem.
- `service.go`: novos sentinels `ErrAIDisabled`/`ErrAIKeyMissing` no writeServiceError/writeChatError.
- Aceite: enabled=false → 409 acionável; sem key → 409 acionável; com key → dispara e responde;
  a key NUNCA aparece em log (não logar o payload cru).

## LANE N8N (sequencial)

### SPEC-W1 — Chat: key do payload (remove $env)
- `workflow-calendar-chat.json`: "Montar contexto" passa a expor `apiKey = body.ai.apiKey`; o nó
  "Chamar LLM" usa `Authorization: Bearer {{ $json.apiKey }}` (remover a leitura de `$env`).
  Mantém o guard de modelo×provider. Reimportável.

### SPEC-W2 — Transcrição OpenAI Whisper + Gemini (key do payload)
- `workflow-calendar-transcribe.json` (reescrever): Webhook (multipart) → Switch por `provider`:
  - `openai` → HTTP `POST https://api.openai.com/v1/audio/transcriptions` (multipart file +
    model=whisper-1, `Authorization: Bearer {{ apiKey }}`).
  - `gemini` → Code monta base64 do áudio → HTTP `POST {geminiBase}/models/{model}:generateContent`
    (inline_data audio/…, prompt "transcreva") com `key`/Bearer → Code extrai o texto.
  - → Respond `{text}`. `apiKey`/`model`/`provider` vêm do payload (form fields).
- Doc `CALENDAR_TRANSCRIBE_WORKFLOW.md` atualizado (sem OpenAI credential; key no payload;
  alternativa local documentada, não implementada).

### SPEC-W3 — Plano: key do payload (calendar-omni)
- `workflow-calendar-omni.json`: usar `ai.apiKey` do payload no(s) nó(s) HTTP (remover dependência
  das credentials calendar-ai-*). Manter o switch de provider (OpenAI-compat + claude opcional).

## LANE FRONT (sequencial)

### SPEC-F1 — Aba IA completa (painel = fonte da verdade)
- `ConfigAi.vue`: **remover** o aviso "As chaves de API ficam no n8n". Adicionar:
  - **Toggle liga/desliga a IA** (`ai.enabled`).
  - **Toggle de escopo** (`ai.useGlobalKeys`): "chaves globais da plataforma" × "chaves desta
    conta". Global editável só por `isPlatformAdmin` (senão status read-only).
  - **Campos de API key** por provider (gemini, glm, openai) — MASCARADOS: mostra
    `configurada ••••1234` (status via `GET /ai-keys`), input "trocar/definir" (write-only,
    `PUT /ai-keys` ou `/ai-keys/global` conforme escopo), botão "limpar".
  - **Provider + modelo** (adicionar `openai` no enum/mapas TS: base `https://api.openai.com/v1`,
    label "OpenAI").
  - **Transcrição**: select provider (OpenAI Whisper | Gemini) + modelo.
  - **Prompt do sistema** (textarea proeminente) = a lei da IA.
- I/O novo em calendar-api.ts (fetch/put ai-keys + global). Config v4 nos tipos TS.
- Aceite: definir key → aba mostra `••••1234` (nunca a key crua); desligar IA reflete; trocar
  escopo troca a fonte; openai aparece no select.

### SPEC-F2 — Janela de chat customizável (tira FAB do canto)
- `CalendarChatPanel.vue`: **remover o FAB do canto**; a janela abre CENTRALIZADA sobre a área do
  calendário, largura = área interna (`center`), com **Minimizar** e **Fechar** no header.
  Minimizado = pill re-expansível; Fechar = some (gatilho de reabrir: botão nos `CalendarControls`
  e o "Abrir chat" da aba IA já existente).
- **Posição/tamanho configuráveis** (`config.chat`): seletor de posição (esquerda=largura do painel
  esquerdo, direita=largura do modal direito, centro=largura do calendário) + redimensionar por
  arrasto (molde do OmniEntityDrawer), persistindo em `config.chat` (store.saveConfig debounced).
- Aplicar posição/tamanho via style calculado; Teleport body mantido (overflow do calendar).
- Aceite: chat abre centralizado ocupando a largura interna; minimiza/fecha; muda pra esquerda/
  direita/centro e persiste após reload; redimensiona e persiste.

---

## WAVE 3.1 — Escopo da IA por cliente (ADENDO 2026-07-04, implementar APÓS a 3.0 cair)

Decisão do dono: a config de IA pode valer para TODOS os clientes (geral) OU ser INDIVIDUAL por
cliente; e mesmo no modo geral, dá pra **desativar a IA para 1 ou vários clientes específicos**.
Camada EM CIMA da 3.0 (config por conta = o default/geral); não substitui nada.

### CFG+ (adição ao `ai` da config, sem migration)
```jsonc
"ai": {
  // ...campos da 3.0...
  "scopeMode": "general",        // "general" (uma config p/ todos) | "perClient" (config por cliente)
  "disabledClientIds": []        // no modo general: clientes com a IA DESLIGADA (exceções)
}
```
### SEC+ — override por cliente (migration 0190)
- `alter table calendar.client_profiles add column if not exists ai_config jsonb not null default '{}'`
  (a home natural do por-cliente já existe; PK account_id+client_id). Guarda o override:
  `{ enabled, provider, model, baseUrl, systemPrompt, temperature }` (sem keys — as chaves seguem
  no nível conta/global da 3.0; cada cliente só muda COMPORTAMENTO, não a credencial).
- Resolver `EffectiveAIConfig(account, clientId)`:
  1. base = config `ai` da conta (geral).
  2. `enabled` efetivo = `ai.enabled` E (clientId ∉ `disabledClientIds`) E (se perClient e há
     override, o `enabled` do override).
  3. se `scopeMode=="perClient"` e o cliente tem override → merge por campo (override vence).
  4. provider/model/prompt/temperature/baseUrl efetivos saem desse merge. A KEY resolve pelo
     provider EFETIVO (resolveAIKey da 3.0).
- Endpoints: `GET/PUT /v1/calendar/ai-config/client?clientId=` (override por cliente, accountScope).

### SPEC-B3 (back) — scopeMode/disabledClientIds na config + ai_config por cliente + resolver
- model.go: `ScopeMode string`, `DisabledClientIDs []string` no AIConfig; sanitizar (enum
  general|perClient; UUIDs). Migration 0190 + store do override em client_profiles.
- chat/transcribe/plan dispatch: trocar o uso direto de `ai` pelo `EffectiveAIConfig(account,
  clientId)` (o clientId já chega no contexto do chat/plano). Kill switch honra o override.
- Aceite: geral com cliente X em disabledClientIds → chat p/ X responde ai_disabled; perClient
  com override de X (prompt próprio) → o system do X vem do override.

### SPEC-F3 (front) — seletor de escopo por cliente na aba IA
- ConfigAi: seletor **Geral × Individual** (`ai.scopeMode`). No modo Geral: multi-select de
  clientes para **desativar** (`ai.disabledClientIds`). No modo Individual: seletor de cliente +
  form de override (enabled/provider/model/prompt/temperature) por cliente, salvo via
  `/ai-config/client`. Badge "usa config geral" quando sem override.
- Aceite: dá pra ligar geral e excluir clientes; dá pra dar prompt próprio a um cliente e ver o
  chat daquele cliente responder com o comportamento dele.

---

## Ordem/Dependências
- BACK B1→B2 (mesmo pacote). N8N W1→W2→W3. FRONT F1→F2. Lanes em paralelo (áreas disjuntas;
  F codifica contra os contratos). Depois: rebuild api + migration 0189, importar/ativar os 3
  workflows, testar (chat com key da conta; transcrição gemini; kill switch; escopo global).
- Segurança: a key crua só existe server-side (resolver/dispatch); front só status mascarado;
  `.env`/log nunca recebem a key do usuário.

## Progress Log
| Quando | Etapa | Status | Notas |
| --- | --- | --- | --- |
| 2026-07-04 | Specs escritas | ok | Decisões: masked write-only; toggle escopo global/conta; kill switch; n8n executor; janela custom. |
| 2026-07-04 | SPEC-W2 (transcribe) | autorado | workflow-calendar-transcribe.json reescrito: Webhook->Preparar->Switch(openai/gemini)->Respond{text}; key do payload, sem credential n8n; Gemini via inline_data base64 (getBinaryDataBuffer); Whisper local documentado como futuro. Validado fora do n8n (parse+ramos+jsCode). Import/teste manual pendente. |
| 2026-07-04 | SPEC-W3 (calendar-omni) | autorado | workflow-calendar-omni.json: "Montar prompt" expoe apiKey=body.ai.apiKey; nos Claude (x-api-key) e OpenAI-compat (Authorization Bearer) usam a key do payload; removidas as credentials calendar-ai-claude/openai-compat (sem $env/credential). DEFAULT_BASE ganhou openai/gemini e glm alinhado a z.ai (igual Calendar Chat). versionId ->calendaromni03 (reimportar). JSON valida. Import/teste manual pendente. |
| 2026-07-04 | SPEC-B3 (escopo por cliente, back) | autorado | Migration 0190 (client_profiles.ai_config jsonb, add column if not exists). model.go: AIConfig +scopeMode/+disabledClientIds; novo tipo ClientAIOverride (ponteiros enabled/temperature). service.go sanitizeAI: scopeMode enum + disabledClientIds dedup; store GetConfig completa shape v4.1. Novos client_ai.go (EffectiveAIConfig + merge + service GetClientAIOverride/PutClientAIOverride), store_client_ai.go (CRUD do override), http_client_ai.go (GET/PUT /v1/calendar/ai-config/client, RequireAuthWithAccount) registrado no module.go. Dispatch: chat usa EffectiveAIConfig(account,clientId); plano filtra disabledClientIds (filterDisabledClients) mantendo o ai config GERAL; transcribe usa config geral. resolveDispatchKey passou a receber enabled bool. `go build`+`go vet` limpos. Teste manual (rebuild api + migration) pendente. Override por-cliente no PLANO fica p/ depois. |
