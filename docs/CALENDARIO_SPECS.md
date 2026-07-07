# Calendário — SPECS de implementação (Fases 3c → 6)

> Specs operacionais para os subagentes. Fonte da visão: [CALENDARIO_PLAN.md](CALENDARIO_PLAN.md)
> (§3.5–3.8). Cada spec é ATÔMICA: começa e termina no mesmo agente, sem deixar meio-caminho.
> Progresso/erros/pendências registrados no **Progress Log** no fim deste arquivo.
> Criado 2026-07-02 (decisões do dono: poster no front via canvas; perfil em tabela dedicada;
> seletor de provider de IA desde a v1 — Claude + IAs chinesas OpenAI-compatible).

## Regras gerais (valem para TODOS os agentes)

1. **Leia antes de codar**: `.claude/skills/principios-engenharia/SKILL.md` + a(s) referência(s)
   da sua área (`references/frontend.md`, `references/backend.md`, `references/database.md`,
   `references/lint.md`). São inegociáveis.
2. **NUNCA** rode `git` (sessão multi-agente), `npm install/build/generate`, `docker` ou
   qualquer deploy. Só edite arquivos. O orquestrador roda build/lint depois.
3. Máx **450 linhas/arquivo**. Comentários em pt-BR **sem acentos** (padrão do repo). Sem emojis.
4. Multi-tenant: `account_id` SEMPRE do Principal (`accountScope(r)` no módulo calendar), nunca
   do body. Store filtra por `account_id` em todo GET/UPDATE/DELETE (defesa em profundidade).
   Recurso fora do escopo → **404** (nunca 403).
5. Go: sem pacote uuid externo (string + cast `::uuid`); scan nullable com `*string`; camadas
   `http → service → store`. Front: `<script setup lang="ts">`, sem `any`, sem `console.log`,
   classes BEM-like, tokens do design system (nunca hex hardcoded em CSS — cor escolhida pelo
   usuário é DADO e pode ir inline via style).
6. Migrations: SQL plano **idempotente**, schema-qualificado, **SEM** marcadores `-- +goose`
   (o migrator roda o arquivo INTEIRO no boot).
7. **Não remover funcionalidade existente.** Features coexistem.
8. Ao terminar, atualize o AGENT.md da SUA área (back: `back/internal/modules/calendar/AGENT.md`;
   front: `web/app/components/AGENT.md` seção calendar) com endpoints/arquivos novos. Docs
   canônicos (plano/roadmap/panorama) ficam com o orquestrador.
9. Config/dado faltante = aviso acionável inline (nunca default silencioso que minta).

---

## Contratos compartilhados (Go ↔ TS ↔ n8n — chaves JSON idênticas)

### C1 — MediaItem v2 (`posterUrl`)
```jsonc
{ "id": "", "url": "/uploads/calendar/{accountId}/x.mp4", "name": "", "type": "image|video",
  "contentType": "", "sizeBytes": 0,
  "posterUrl": "/uploads/calendar/{accountId}/poster-x.jpg" } // OPCIONAL; so p/ video
```
`posterUrl` passa pela MESMA validação de prefixo `/uploads/calendar/{accountId}/` do `url`
(descartar/limpar se externo). Go: `PosterURL string \`json:"posterUrl,omitempty"\``.

### C2 — CalendarConfig v2 (jsonb `calendar.config`, migration 0182 — sem migration nova)
```jsonc
{
  "responsibleUserIds": [],
  "holidays": { "brNational": true, "sergipe": true, "aracaju": true, "luxuryIntl": true },
  "weekStartsOn": "sunday",              // "sunday" | "monday" (default sunday = atual)
  "clientColors": {},                    // { [clientId]: "#rrggbb" | "none" } — vazio = paleta
  "typeColors": {},                      // { [CalendarEventType]: "#rrggbb" } — vazio = cor do cliente
  "whiteLabel": { "logoUrl": "", "title": "", "primaryColor": "" },
  "ai": {
    "provider": "claude",                // claude | deepseek | qwen | kimi | glm | custom
    "model": "claude-sonnet-5",
    "baseUrl": "",                       // vazio = default do provider (mapa no n8n e no front)
    "systemPrompt": "",                  // vazio = prompt default do workflow
    "temperature": 0.7
  }
}
```
Defaults preenchidos nos DOIS lados: Go (`defaultConfig()` + unmarshal por cima dos defaults —
linhas antigas sem os campos novos ganham default) e TS (`normalizeConfig` vira merge **por
seção**, não spread raso). **Chaves de API NUNCA aqui** — vivem nas credentials do n8n.
Base URLs default por provider (front placeholder + n8n): deepseek `https://api.deepseek.com`,
qwen `https://dashscope.aliyuncs.com/compatible-mode/v1`, kimi `https://api.moonshot.cn/v1`,
glm `https://open.bigmodel.cn/api/paas/v4`, claude `https://api.anthropic.com`.

### C3 — Perfil estratégico do cliente (migration 0185 `calendar.client_profiles`)
PK `(account_id, client_id)` — account = dona do calendário (Principal), client = conta-cliente.
```jsonc
// GET/PUT /v1/calendar/client-profile?clientId=<uuid>   (PUT = upsert full-replace)
{
  "clientId": "uuid",
  "segment": "", "positioning": "", "description": "", "history": "",
  "siteUrl": "", "instagram": "", "address": "", "objectives": "", "brandVoice": "",
  "extra": { "audience": "", "offer": "", "pillars": "", "cadence": "",
             "restrictions": "", "performance": "", "assets": "" },
  "updatedAt": "ISO"
}
// GET /v1/calendar/client-profiles → { "profiles": [{ "clientId", "filled": bool, "updatedAt" }] }
```
`filled` = algum campo estável não-vazio. GET de perfil inexistente → objeto vazio com defaults
(200, não 404 — perfil é opcional por design).

### C4 — Plano de IA (migration 0186 `calendar.ai_plans`)
Status: `pending → done | error`; `done → applied`. Content (contrato com o n8n e o front):
```jsonc
{
  "summary": "",
  "pillars": [{ "name": "", "proportion": "", "rationale": "" }],
  "clients": [{
    "clientId": "", "clientName": "", "strategy": "",
    "days": [{ "date": "YYYY-MM-DD", "type": "post|story|reels", "idea": "", "copy": "" }]
  }]
}
```
Endpoints (todos RequireAuth + accountScope, exceto o callback):
- `POST /v1/calendar/ai/plan` body `{ "month": "YYYY-MM", "clientIds": ["uuid"] }` →
  cria row `pending`, responde **201 `{id, status}`**, e dispara o n8n em goroutine (payload C5).
  Sem `CALENDAR_AI_WEBHOOK_URL` no env → **503 `ai_not_configured`** com mensagem acionável.
  Falha no dispatch → goroutine marca `status=error` + `error`.
- `GET /v1/calendar/ai/plans?month=` → lista LEAN `{plans:[{id,month,clientIds,status,provider,model,createdAt}]}`.
- `GET /v1/calendar/ai/plans/{id}` → completo (com `content`).
- `POST /v1/calendar/ai/plans/{id}/applied` → marca `applied` (só se `done`).
- `DELETE /v1/calendar/ai/plans/{id}`.
- **Callback (n8n → api, SEM JWT)**: `POST /v1/public/calendar-ai/plans/{id}/result`, header
  `X-Service-Token` comparado (constant-time, `crypto/subtle`) com env `CALENDAR_AI_SERVICE_TOKEN`;
  body `{ "status": "done|error", "content": {...}, "error": "" }` (máx 2 MiB). Só transiciona a
  partir de `pending` (plano já `done`/`applied` → 409). Env ausente → 503. Prefixo `/v1/public`
  fica FORA do gate de módulo (conferir `moduleGatingRules()` em `back/internal/platform/app/app.go`).

### C5 — Payload Omni → n8n (webhook "Calendar Omni")
```jsonc
{
  "planId": "", "month": "YYYY-MM", "language": "pt-BR",
  "ai": { "provider": "", "model": "", "baseUrl": "", "systemPrompt": "", "temperature": 0.7 },
  "clients": [{ "id": "", "name": "", "profile": { /* C3 sem clientId */ } }],
  "holidays": [{ "date": "", "name": "", "set": "" }],   // feriados DO mês (config da conta)
  "monthNotes": "<html da nota do mês, pode ser vazio>",
  "callbackUrl": "<CALENDAR_AI_CALLBACK_BASE>/v1/public/calendar-ai/plans/{id}/result",
  "serviceToken": "<CALENDAR_AI_SERVICE_TOKEN>"
}
```
Envs novos (documentar): `CALENDAR_AI_WEBHOOK_URL`, `CALENDAR_AI_SERVICE_TOKEN`,
`CALENDAR_AI_CALLBACK_BASE` (base pública da api p/ o n8n chamar de volta).

---

## LANE BACK (sequencial — um agente por spec)

### SPEC-B1 — `posterUrl` no MediaItem (pequena)
- `back/internal/modules/calendar/model.go`: campo `PosterURL` no `MediaItem` (C1).
- `service.go` (ou onde a mídia de evento/dia é sanitizada — procurar a validação de `url`
  com prefixo `/uploads/calendar/`): aplicar a MESMA regra ao `posterUrl`; se inválido/externo,
  zerar o campo (não rejeitar o item inteiro).
- Upload (`http_media.go`) NÃO muda: o poster é upload normal de imagem.
- Aceite: `PUT /events/{id}` e `PUT /day-media/{date}` com `posterUrl` válido persistem e
  devolvem o campo; `posterUrl` externo volta vazio.

### SPEC-B2 — Perfil estratégico do cliente (migration 0185 + endpoints)
- Migration `0185_calendar_client_profiles.sql` (colunas: segment, positioning, description,
  history, site_url, instagram, address, objectives, brand_voice, extra jsonb default '{}',
  updated_by, created_at, updated_at; PK (account_id, client_id); FKs core.accounts on delete
  cascade; idempotente, `create schema if not exists calendar;` no topo).
- Arquivos novos no módulo (manter <450): `profile.go` (tipos + service), `store_profile.go`,
  `http_profile.go` (`RegisterProfileRoutes`), chamado no `module.go` → `RegisterRoutes`.
- Contrato C3. `clientId` validado como UUID (senão 400 `invalid_client`). Escopo: tudo por
  `accountScope(r)`; perfil de outra account nunca aparece (PK composta + WHERE account_id).
- `PUT` = upsert full-replace (`on conflict ... do update`), `updated_by` = `principalLabel(r)`.
- Aceite: GET vazio devolve defaults; PUT→GET roundtrip; conta B não lê perfil da conta A.

### SPEC-B3 — CalendarConfig v2 (sem migration)
- `model.go`: estender `CalendarConfig` com C2 (`WeekStartsOn`, `ClientColors map[string]string`,
  `TypeColors map[string]string`, `WhiteLabel struct`, `AI struct`). `defaultConfig()` preenche
  todos os defaults. Conferir no `store_postgres.go` que o unmarshal do jsonb é feito POR CIMA
  dos defaults (linhas antigas ganham os campos novos); PutConfig continua full-replace.
- Sanitização no service: `weekStartsOn` ∈ {sunday,monday} (senão default); `ai.provider` ∈
  {claude,deepseek,qwen,kimi,glm,custom}; `temperature` clamp 0..1; cores validadas
  (`#rrggbb` ou `none`, senão descartada); strings trim.
- Aceite: GET de conta antiga (config só com responsáveis/feriados) devolve o shape completo C2.

### SPEC-B4 — IA: migration 0186 + endpoints + dispatch n8n + callback
- Migration `0186_calendar_ai_plans.sql` (colunas conforme C4 + index (account_id, month_key)).
- Arquivos novos: `ai_plans.go` (tipos + service + dispatch), `store_ai_plans.go`,
  `http_ai_plans.go` (rotas autenticadas + callback público). Envs lidos no padrão do app
  (ver como `cfg`/env chega ao módulo em `module.go`/`app.go` — seguir o padrão existente,
  ex.: `UploadsDir` → `MediaStorage`; injete via `calendar.New(...)`).
- `POST /ai/plan`: valida month `YYYY-MM` e clientIds UUID; monta payload C5 (perfil de cada
  cliente via store de B2; nome via core.accounts; feriados do mês via `HolidaysInRange` +
  config; nota do mês via store) e dispara em goroutine (`http.Client` timeout 15s).
- Callback público C4 (constant-time compare; transição só de `pending`; log em erro).
- Rota pública: registrar no mux com prefixo `/v1/public/calendar-ai/` (fora do gate — conferir
  `moduleGatingRules()`); NÃO usar RequireAuth nela.
- Aceite: sem env → 503 acionável; com env fake → row `pending` vira `error` (dispatch falha);
  callback com token errado → 403; com token certo → `done` + content persistido.

## LANE FRONT (sequencial — um agente por spec)

### SPEC-F1 — Viewer (clicar abre) + poster no upload
- `web/app/utils/calendar.ts`: `posterUrl?: string` no `CalendarMediaItem` (C1).
- NOVO `web/app/components/calendar/CalendarMediaViewer.vue`: overlay (Teleport body) com
  imagem em tela cheia OU `<video controls autoplay :poster>`; navegação ‹ › entre os itens
  recebidos (`items`, `startIndex`); fechar por X, Esc E clique no backdrop (coexistem);
  nome + tamanho no rodapé. Estilos em `assets/styles/calendar/media.css` (tokens, sem hex).
- `CalendarMediaUploader.vue`: clicar num item abre o viewer (botão remover continua
  funcionando via `@click.stop`); thumb de vídeo usa `posterUrl` como `<img>` quando existir
  (senão mantém `<video preload="metadata">` atual — não remover).
- `useCalendarMedia.ts`: helper `capturePosterFromVideo(file: File): Promise<File | null>` —
  objectURL + `<video muted playsinline>`, seek `min(0.5s, 10% da duração)`, canvas máx 640px
  de largura, `toBlob('image/jpeg', 0.82)`, nome `poster-<base>.jpg`, timeout 8s → null,
  SEMPRE `revokeObjectURL`. No fluxo de upload de vídeo: sobe o vídeo (progresso), depois
  captura+sobe o poster silenciosamente e seta `posterUrl` no item; falha do poster NÃO
  falha o upload (fallback = sem poster). Cuidar do limite de 450 linhas (extrair p/ util se precisar).
- Aceite: subir vídeo gera 2 arquivos (vídeo + poster) e o item tem `posterUrl`; clicar em
  imagem/vídeo abre o viewer e reproduz; Esc/clique-fora fecham.

### SPEC-F2 — Fundo do dia com a(s) postagem(ns) em grade
- `domain/calendar/calendar-api.ts`: `fetchDayMediaInRange(api, from, to)` →
  `GET /v1/calendar/day-media?from=&to=` → `Array<{date, media}>`.
- `stores/calendar.ts`: estado `dayMediaByDate: Map<string, CalendarMediaItem[]>`, buscado na
  MESMA janela debounced dos eventos (`scheduleWindowFetch`), zerado na troca de conta;
  método `saveDayMedia(date, media)` (PUT + atualiza o Map local) — `DayDrawer` passa a usar o
  store (mantendo `useCalendarMedia` só p/ upload/limits). Cuidar <450 (se estourar, extrair
  bloco p/ `composables/useCalendarDayMedia.ts` integrado ao store).
- `utils/calendar.ts`: helper puro `dayBackgroundUrls(events, dayMedia): string[]` — regra:
  mídias dos EVENTOS do dia primeiro (ordem por horário; imagem → `url`, vídeo → `posterUrl`,
  vídeo sem poster é pulado); se nenhum evento tem mídia, usa os anexos avulsos do dia;
  máximo 4 urls.
- `DayCell.vue`: prop `bgUrls?: string[]`; camada `calendar-cell__bg` (absolute, inset 0,
  atrás do conteúdo) com tiles `background-image` em grade por `data-count`
  (1 = inteiro; 2 = 2 colunas; 3 = 1 alto à esquerda + 2 à direita; 4 = 2×2) + overlay
  gradiente com token de superfície (legibilidade do número/feriado/chips nos DOIS temas).
  Conteúdo da célula ganha z-index acima. `MonthGrid.vue`/`WeekView.vue` calculam e passam
  `bgUrls` por dia (usando `eventsByDate` — já respeita o filtro de cliente — + `dayMediaByDate`).
- CSS em `assets/styles/calendar/media.css` (ou `grid.css` se for o lugar natural), só tokens.
- Aceite: dia com 1 foto = fundo inteiro; 2 mídias = dividido; vídeo entra pelo poster;
  número do dia/chips continuam legíveis no claro e escuro; filtro de cliente reflete no fundo.

### SPEC-F3 — Página /calendario/config (migra o modal) + aplicar config
- Mover `pages/calendario.vue` → `pages/calendario/index.vue` (rota `/calendario` inalterada;
  conferir o checklist de página nova em `references/frontend.md` + `auth.global.ts`).
  NOVO `pages/calendario/config.vue` (mesmo `definePageMeta` preview `workspaceId: ''`),
  header com voltar p/ `/calendario`.
- Engrenagem (`CalendarControls` → evento `config`): em vez de abrir modal, `navigateTo`
  `/calendario/config`. **Deletar** `CalendarConfigModal.vue` (sem código morto) migrando o
  conteúdo p/ seções da página, em `components/calendar/config/`:
  `ConfigResponsibles.vue`, `ConfigHolidays.vue` (migrados do modal),
  `ConfigAppearance.vue` (weekStartsOn + clientColors com opção "sem cor" + typeColors +
  whiteLabel logo/título/cor), `ConfigAi.vue` (provider select, model, baseUrl com placeholder
  do default por provider, systemPrompt textarea, temperature; AVISO fixo: "as chaves de API
  ficam no n8n (credentials), nunca aqui"), `ConfigMediaLimits.vue` (GET sempre; edição só
  `isPlatformAdmin` — regra `isPlatformAdmin || has(...)`).
- Botão único "Salvar configurações" (store.saveConfig) + feedback ui.success/error; campos
  obrigatórios/faltando sinalizados (nunca botão morto sem aviso).
- Tipos TS C2 em `utils/calendar.ts` + `normalizeConfig` (calendar-api.ts) com merge por seção.
- APLICAR a config no calendário: `weekStartsOn` da config → viewport (store observa config);
  `clients` computed usa `clientColors` (hex→triplet via novo helper `hexToTriplet`; `none` →
  cinza neutro [148,163,184]); `typeColors` desce como prop `typeColors` por
  MonthGrid/WeekView/DayCell → EventChip (override da cor do cliente quando setado).
- Aceite: /calendario/config abre, salva e o calendário reflete (semana começa segunda, cor
  custom do cliente, cor por tipo); /calendario continua funcionando igual; modal deletado.

### SPEC-F4 — Form do perfil estratégico (usa B2 — codar contra o contrato C3)
- NOVO `components/calendar/config/ConfigClientProfiles.vue` na página de config: select de
  cliente (store.clients) + form com os campos estáveis (segmento, posicionamento, descrição,
  história, site, instagram, endereço, objetivos, tom de voz) + textareas do `extra`
  (público-alvo, oferta, pilares, cadência, restrições, performance, assets).
- `domain/calendar/calendar-api.ts`: `fetchClientProfile`, `putClientProfile`,
  `fetchClientProfilesIndex` (C3). Salvar por cliente (botão próprio + feedback), com dirty
  guard (trocar de cliente com edição pendente avisa). Badge "preenchido/vazio" na lista
  (via `filled` do index).
- Aceite: preencher → salvar → trocar de cliente e voltar → dados persistem (vêm do back).

### SPEC-F5 — Botão de IA + modal do plano (usa B4 — codar contra C4/C5)
- `CalendarControls.vue`: botão "IA do mês" (ícone sparkles) → abre NOVO
  `components/calendar/CalendarAiPlanModal.vue`.
- Modal: seleção de 1+ clientes (multi), mês (default = mês em foco), resumo do provider/modelo
  configurado com link "configurar" → /calendario/config; "Gerar plano" → `POST /ai/plan` →
  polling `GET /ai/plans/{id}` a cada 3s (máx 5min, para ao fechar o modal); estados
  pending/erro com mensagem acionável (503 `ai_not_configured` → explica envs/n8n e linka config).
- Resultado (`CalendarAiPlanResult.vue` p/ manter <450): summary + pilares + por cliente os
  dias (data, tipo, ideia, copy). Ações: **"Aplicar nas notas"** (anexa HTML formatado à nota
  do mês alvo — se for o mês ativo usa `store.setNotesForActiveMonth`, senão GET+append+PUT via
  calendar-api) e **"Criar eventos"** (loop `store.createEvent`: title=idea, description=copy,
  type mapeado (fora do enum → post), status planejado, priority media, clientId do bloco,
  date do item) — depois `POST /plans/{id}/applied`. Lista dos planos anteriores do mês
  (GET lean) com abrir/excluir.
- I/O novo em `composables/useCalendarAiPlans.ts` (polling + apply) + funções em calendar-api.ts.
- Aceite: fluxo completo com back mockado por curl (o orquestrador valida end-to-end depois);
  polling PARA ao fechar; dupla aplicação não duplica silenciosamente (confirmar antes de
  reaplicar um plano `applied`).

## LANE N8N/DOCS (paralela)

### SPEC-W1 — Workflow "Calendar Omni" (JSON importável) + doc
- NOVO `automation/export/workflow-calendar-omni.json`, espelhando o formato dos exports
  existentes (`workflow-omni-chat.json` como referência de estrutura/typeVersion):
  1. **Webhook** POST path `calendar-omni` (respond immediately) — recebe C5.
  2. **Code "Montar prompt"**: system = `ai.systemPrompt` || DEFAULT (estratégia de conteúdo
     pt-BR; instrui saída JSON ESTRITO no shape C4.content, sem markdown); user = mês +
     feriados + clientes/perfis + notas; monta também o mapa de baseUrl default por provider (C2).
  3. **Switch provider**: ramo `claude` → HTTP Request `POST {baseUrl}/v1/messages`
     (headers `x-api-key` via credential `calendar-ai-claude`, `anthropic-version`); demais →
     HTTP Request `POST {baseUrl}/chat/completions` (Bearer via credential
     `calendar-ai-<provider>`). `continueOnFail` ligado nos nós de LLM.
  4. **Code "Extrair JSON"**: normaliza resposta (anthropic `content[0].text` vs openai
     `choices[0].message.content`), remove cercas de código, `JSON.parse` com try/catch →
     `{status:'done', content}` ou `{status:'error', error}`.
  5. **HTTP Request "Callback"**: POST `{{callbackUrl}}` header `X-Service-Token: {{serviceToken}}`.
- NOVO `docs/automation/CALENDAR_OMNI_WORKFLOW.md`: como importar, credentials a criar
  (nomes acima, uma por provider), envs do Omni (C5), como testar (payload de exemplo +
  curl do callback), limitações (workflow autorado, precisa de import + teste manual no n8n).
- Aceite: JSON válido (parse), nós conectados, doc completo.

---

## Ordem/Dependências
- Back B1→B2→B3→B4 (sequencial, mesmo pacote Go). Front F1→F2→F3→F4→F5 (sequencial, arquivos
  compartilhados). W1 independente. Lanes back/front/n8n em PARALELO (áreas disjuntas).
- F4 e F5 codam contra os contratos C3/C4 (não esperam o back estar rodando).
- Depois: revisão adversarial → correções → build api + migrations → lint → docs.

## Progress Log (preencher a cada etapa — erros, acertos, onde parou, o que falta)

| Quando | Etapa | Status | Notas |
| --- | --- | --- | --- |
| 2026-07-02 | Specs escritas (este arquivo) | ok | Decisões do dono registradas no topo. |
| 2026-07-02 | SPEC-B1 (posterUrl no MediaItem) | ok | Campo + validação de prefixo interno no service (normalizeMedia). |
| 2026-07-02 | SPEC-B2 (perfil estratégico do cliente) | ok | Migration 0185 + profile.go/store_profile.go/http_profile.go; 3 endpoints C3; build+lint+vet limpos. |
| 2026-07-02 | SPEC-B3 (CalendarConfig v2) | ok | Sem migration: CalendarConfig estendido com C2 (weekStartsOn/clientColors/typeColors/whiteLabel/ai) + defaults completos; unmarshal por cima dos defaults no GetConfig (guarda maps/enums nil); sanitizacao no PutConfig (enum/cores/temperature/trim). build+vet+golangci limpos. |
| 2026-07-02 | SPEC-B4 (IA: migration 0186 + endpoints + dispatch n8n + callback) | ok | Migration 0186 `calendar.ai_plans` (id PK, account_id FK, month_key, client_ids/content jsonb, status pending->done|error->applied, provider/model snapshot, indices por account+mes/created). Novos arquivos: ai_plans.go (tipos C4 + service CRUD/transicoes + WithAI), ai_dispatch.go (payload C5 + POST goroutine http.Client 15s + log), store_ai_plans.go, store_ai_context.go (nomes/perfis/feriados/nota sem N+1 via ANY), http_ai_plans.go (RegisterAIPlanRoutes: painel RequireAuth + callback publico). Callback fora do gate (/v1/public), autenticado por X-Service-Token constant-time (crypto/subtle); so transiciona de pending (GET previo separa 404 de 409). account_id sempre do accountScope (callback resolve pelo id). Envs lidos no Build via os.Getenv (padrao cardapio): CALENDAR_AI_WEBHOOK_URL / _SERVICE_TOKEN / _CALLBACK_BASE. build+vet limpos, golangci 0 issues, gofmt limpo nos arquivos novos (store_postgres.go tem diff pre-existente de smart-quote fora do escopo), migrations-lint EXIT=0. Falta FRONT (F5) + workflow n8n (W1). |
| 2026-07-02 | Revisao adversarial back (3 achados) | ok | (1) ai_dispatch.go dispatchPlan: marcacao de `error` agora usa ctx NOVO e curto (markErrorTimeout=5s, context.Background) em vez do ctx do POST — no timeout do webhook o ctx do POST ja estava DeadlineExceeded e o UPDATE de error nunca rodava (plano preso em pending para sempre); C4 cumprido. (2) normalizeMedia (extraido p/ media_normalize.go p/ ficar <450): valida `url` E `posterUrl` contra o prefixo COM accountId (`/uploads/calendar/{accountId}/`), nao mais o generico — fecha vazamento cross-tenant de midia pelo file server publico (contrato C1). accountID desce do accountScope via validateEvent/PutDayMedia, nunca do body. (3) loadAccountNames (store_ai_context.go): resolve nome SO de contas ja referenciadas por evento/perfil DESTA account (EXISTS em calendar.events/client_profiles) — barra enumeracao cross-account de nomes por UUID arbitrario. build+vet+gofmt limpos; service.go 419 linhas. |
| 2026-07-02 | Lane FRONT (F1-F5) | ok | F1: CalendarMediaViewer (lightbox/player, Esc+clique-fora+setas) + clicar-abre no uploader + poster via canvas (capturePosterFromVideo, timeout 8s, fallback sem poster). F2: dayMediaByDate na janela do store + dayBackgroundUrls + fundo do dia em grade (1/2/3/4 tiles + overlay) em DayCell/MonthGrid/WeekView. F3: pages/calendario/{index,config}.vue (modal deletado, engrenagem navega) + secoes ConfigResponsibles/Holidays/Appearance/Ai/MediaLimits + config v2 aplicada (weekStartsOn no viewport, clientColors/typeColors nos chips). F4: ConfigClientProfiles + fetch/put/index no calendar-api (contrato C3). F5: botao IA do mes + CalendarAiPlanModal/Result + useCalendarAiPlans (polling 3s max 5min, para ao fechar; aplicar em notas/eventos + marcar applied). |
| 2026-07-02 | SPEC-W1 (workflow n8n Calendar Omni) | ok | automation/export/workflow-calendar-omni.json (webhook -> montar prompt -> switch claude/openai-compatible -> extrair JSON -> callback X-Service-Token) + docs/automation/CALENDAR_OMNI_WORKFLOW.md (import, credentials calendar-ai-*, envs, teste). PENDENTE: importar no n8n + criar credentials + testar ao vivo. |
| 2026-07-02 | Revisao adversarial front (1 critical) | ok | calendar-api.ts putClientProfile nao mandava ?clientId= na URL (PUT sempre 400) — corrigido. 2 achados refutados pelos ceticos (falso positivo de perf no WeekView + fragilidade hipotetica). |
| 2026-07-02 | Orquestrador: build + migrations + lints | ok | docker compose up -d --build api EXIT 0; migration_up_ok; 6 tabelas no schema calendar (client_profiles + ai_plans criadas). golangci-lint 0 issues; eslint EXIT 0; nenhum arquivo do calendario > 450 linhas. Workflow: 21 agentes, 10/10 specs done, 6 achados -> 4 confirmados -> 4 corrigidos. |
| 2026-07-02 | O QUE FALTA (retomada) | pendente | (1) Validar no dev/browser: viewer, poster, fundo do dia, /calendario/config, perfil, modal IA (regra: implementar e parar p/ revisao). (2) n8n: importar workflow-calendar-omni.json, criar credentials calendar-ai-{claude,deepseek,qwen,kimi,glm}, setar CALENDAR_AI_WEBHOOK_URL/_SERVICE_TOKEN/_CALLBACK_BASE no compose e testar end-to-end. (3) Fase 7 (WhatsApp + visao compartilhavel) nao iniciada. |
| 2026-07-02 | VALIDACAO DO DONO (browser) — 4 erros reais | corrigido | (1) BUG PRINCIPAL: uploader/viewer/fundo do dia usavam url RELATIVA (/uploads/...) que no dev resolve contra o front :3003 => 404 => nenhuma imagem em lugar nenhum. O helper resolveMediaUrl(apiBase) JA EXISTIA (utils/media.ts, padrao do bio) e o calendario nao usava — falha da spec (nao citou) E da revisao (nao conhecia o padrao). Corrigido em DayCell/CalendarMediaUploader/CalendarMediaViewer. (2) AVIF rejeitado pelo back (whitelist sem image/avif) enquanto o front aceita image/* => "Falha ao enviar mostarda.avif"; adicionado avif na whitelist + extensao (media_storage.go) + api rebuildada. (3) Erro de upload era seco ("Falha ao enviar X") — agora mostra o motivo do back (invalid_media/media_too_large/network/HTTP nnn) via onError no uploadMedia. (4) Video antigo (subido antes do poster) nao tem posterUrl e e PULADO no fundo do dia — comportamento da spec; re-subir o video gera poster. |
| 2026-07-02 | Workflow Calendar Omni IMPORTADO no n8n local | ok | docker cp + n8n import:workflow no container omni-n8n-1 => "Successfully imported 1 workflow" (id calendaromni0001, inativo). FALTA: criar credentials calendar-ai-* + setar envs CALENDAR_AI_* na api + ativar e testar. |
| 2026-07-02 | Upload de video travado em 0% (validacao do dono, rodada 2) | corrigido/mitigado | Fundo da IMAGEM funcionou (fix da url confirmado). Video nao vira fundo porque o video existente e ANTERIOR ao poster (sem posterUrl => pulado por spec); re-upload geraria o poster, MAS o envio travou em 0%: diagnostico = api saudavel (preflight 204, PUT/GET ok) e NENHUM POST /media chegou no log => corpo grande morrendo na ponte de portas do Docker Desktop (mesmo fantasma do login, memoria do projeto). Fix: docker compose up -d --force-recreate api + blindagem no front (xhr.timeout 15min + ontimeout/onabort com mensagem acionavel — nunca mais pendura em 0% sem aviso). |
