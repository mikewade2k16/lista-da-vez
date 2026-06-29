# PLANO — Tracking completo do TAVOLA + Analytics no painel (Cardápio, Fase 10)

> Doc canônico desta frente. Fonte de verdade das decisões reconciliadas; espelhado em
> `web/app/components/roadmap/roadmap-data.ts` (fase `cardapio-online`, tasks `card-f10-*`)
> e no AGENT.md do módulo (`back/internal/modules/cardapio/AGENT.md` +
> `web/app/components/cardapio/AGENT.md`). Sincronizar os três ao concluir cada WS.
>
> Status: **PLANO/SPECS** (2026-06-25); **F4 (painel dashboard) EM ANDAMENTO** (2026-06-25, código
> local). Escopo aprovado pelo dono: **completo reconciliado**; LGPD por **aviso + legítimo interesse**
> (sem banner bloqueante); execução **paralela multi-agente** após a Fase 1, com "ok" explícito antes
> de disparar.

---

## 1. Objetivo

Tracking de verdade do site público (TAVOLA): **acessos, páginas, produtos vistos, botões
clicados, quantidades, horários e tempo em cada lugar (dwell)**, trazendo tudo agregado para a
API do Omni, onde o dono vê e analisa num **dashboard por estabelecimento**.

A fundação já existe (tabela `cardapio.events`, endpoint público de eventos com allowlist,
`orders.session_id`). Esta frente **estende**, não refaz: coleta rasa → coleta completa, e
listagem crua → leitura analítica.

## 2. Estado atual (diagnóstico)

- **Back:** `cardapio.events (id, account_id, restaurant_id, name, session_id, context jsonb,
  created_at)`; `POST /v1/public/restaurants/{slug}/events` (allowlist de 15, rate limit
  60/min/IP, `context ≤ 8KB`, `account_id` resolvido pelo slug); `GET /v1/cardapio/
  restaurants/{id}/events` (lista crua paginada, **sem agregação**). `orders.session_id` existe.
- **Front TAVOLA** (`Projects/TAVOLA/apps/web/app`): `composables/useTracking.ts` fire-and-forget;
  `sessionId` persistente em `localStorage('tavola-sid')`; **só 8 dos 15** eventos disparam.
  Sem dwell, scroll, page_view por rota SPA, nem flush no unload.
- **Painel** (`web/`): **não há dashboard**. A aba "Estatísticas" é só config de GA/Pixel ID.

## 3. Arquitetura (4 camadas)

```
TAVOLA (front)                 API Go (cardapio)                       Painel (web)
plugin telemetry.client.ts ->  POST .../events/batch (ingestão)
useTracking (dwell/scroll/     cardapio.events (+ colunas desnorm.)    aba "Relatórios"
  clicks/sessões)              cardapio.sessions (agregada na ingestão) GET .../analytics/* ->
sessionId casado c/ orders     enriquecimento UA/referrer/ip_hash       KPIs, funil, horários,
                               GET .../analytics/* (agregação)          top-produtos, dwell, cliques
```

---

## 4. DECISÕES RECONCILIADAS (fonte única — resolvem os conflitos entre specs)

Os specs de design divergiam em nomes de evento, modelo de dwell e contrato de endpoints.
Estas decisões são **canônicas** e prevalecem sobre qualquer spec por camada:

| Tema | Decisão |
|---|---|
| **Allowlist** | Uma só (36 nomes, §5). Definida em `model_order.go` + AGENT.md + front, nome-a-nome. Nome fora da lista = descartado no back (`rejected++`), nunca derruba o lote. |
| **Dwell** | Eventos `page_dwell`/`product_dwell`/`section_dwell` com `dwellMs`+`wallMs`+`maxScrollPct` no `context`. Heartbeat = `page_dwell` parcial `final:false`. **Agregação usa só `final:true`** (parciais não somam → sem double-count). |
| **Chave de produto** | Desnormalizar **`product_slug` (text)** em `events` (casa com `/prato/[slug]` e com o analytics). Validado contra os produtos do restaurante no ingest; não pertence → grava NULL. `product_id uuid` opcional/secundário. |
| **"Horários"/picos** | Sempre por **`created_at` (servidor)** — imune a clock-skew/forja. `occurred_at` (cliente) só ordena eventos **dentro de um batch** (reconstrução de dwell), nunca alimenta histograma de hora. |
| **Endpoints** | Base única `/v1/cardapio/restaurants/{id}/analytics/*` (`overview, timeseries, funnel, top-products, sources, devices, pages, dwell, clicks`). DTOs idênticos nos dois lados (§8). |
| **Batch** | **Obrigatório**: `POST .../events/batch` (array, sendBeacon no unload). Singular legado mantido só p/ compat, com o mesmo cap de 8KB. |
| **Tabela `sessions`** | `cardapio.sessions` agregada na ingestão (upsert). Analytics **lê dela** para overview/sources/device; cai em `events` só para funil/dwell/top-produtos/cliques. |
| **Dedupe** | Cada evento carrega `eventId` (uuid do cliente). Ingest faz dedupe (skip em conflito) → beacon/visibilitychange duplicado não conta em dobro. |
| **Identidade** | `deviceId` persistente (`tavola:did`) + `sessionId` que reseta após 30min de inatividade (`tavola:sid`). O `sessionId` atual é enviado no `POST /orders` (casa com `orders.session_id`). |

---

## 5. Allowlist canônica (36 eventos)

`*` = novo (21 a adicionar ao map `allowedEvents` em `model_order.go`). Os demais (15) já existem;
parte deles ainda não disparava no front.

| Grupo | Eventos |
|---|---|
| Navegação/sessão | `page_view`, `session_start`*, `session_end`* |
| Acesso | `restaurant_viewed`, `menu_viewed` |
| Produto | `product_impression`*, `product_clicked`, `product_viewed`, `product_option_changed`* |
| Carrinho/qtd | `add_to_cart`, `cart_qty_changed`*, `remove_from_cart`, `cart_opened`, `cart_cleared`* |
| Checkout | `checkout_started`, `checkout_type_changed`*, `checkout_payment_selected`*, `checkout_submitted`*, `checkout_failed`*, `order_created`*, `whatsapp_order_clicked` |
| Catálogo/busca | `category_viewed`, `category_tab_clicked`*, `menu_search`*, `menu_filter_changed`*, `menu_sort_changed`* |
| CTA/saída | `cta_clicked`* (banner = `ctaKind:'banner'`), `outbound_click`* |
| Engajamento | `scroll_depth`*, `page_dwell`*, `product_dwell`*, `section_dwell`* |
| Cupom/reserva | `coupon_viewed`, `coupon_used`, `reservation_started`, `reservation_sent` |

Sincronizar em 3 lugares: `model_order.go` (`allowedEvents`), `AGENT.md` (linha da allowlist),
teste `service_test.go` (allowlist). **Nenhum context carrega PII** (nome/telefone/endereço/CPF);
`menu_search` envia só `length` + `hasResults`, nunca o termo cru (defesa também no back — §6.B).

---

## 6. Riscos travados (da crítica de segurança/privacidade/escala)

- **A1 — 404 vs vazio (isolamento):** analytics resolve o pertencimento do restaurante 1x
  (mesma validação do painel); fora do escopo → **404 uniforme**, nunca "vazio silencioso". Repo
  filtra `account_id` + `restaurant_id` em toda query (defesa em profundidade).
- **A2 — envenenamento de funil (endpoint público sem auth):** conversão **ancora em `orders`
  real** (fonte de verdade); eventos são sinais best-effort. `device_type='bot'` classificado no
  ingest e **excluído por padrão** em TODA query. Doc de produto: métricas de ingestão são
  best-effort, não auditáveis.
- **A3/A4 — LGPD/PII:** IP nunca cru → `ip_hash = sha256(ip + CARDAPIO_TELEMETRY_SALT)`; salt
  **obrigatório no boot** (sem fail-open silencioso). `context` é jsonb livre → o back aplica
  **deny-list de chaves** (`name/phone/email/cpf/telefone/endereco/...`) e, para `menu_search`,
  descarta o termo (grava só `length`/`hasResults`). Não confiar na promessa do front.
- **A5 — escala:** `product_impression` = **1 evento agregado por viewport** (lista de ids+posições
  vistos), não 1 por card. Heartbeat 30s, pausado em aba de fundo. `cardapio.sessions` agregada é
  obrigatória (overview/sources lêem dela). Partição mensal de `events` planejada. `eventId` p/ dedupe.
- **A6 — confiabilidade do horário:** `created_at` servidor (acima).
- **LGPD (decisão do dono): aviso + legítimo interesse.** `deviceId`/`sessionId` tratados como
  **pseudônimos** (dado pessoal), não "anônimos". Aviso de privacidade na página do TAVOLA +
  base legal "legítimo interesse" (analytics próprio agregado). Kill-switch técnico
  `NUXT_PUBLIC_TELEMETRY_DISABLED`. Retenção curta (90d) para linhas com `device_id`/`session_id`;
  rollup de longo prazo só **anônimo** (sem id). Eventos/sessões crus no painel só `platform_admin`
  (com `LegacyMarker`). Banner de consentimento fica como débito documentado, não nesta fase.

---

## 7. Camada FRONT (TAVOLA) — `Projects/TAVOLA/apps/web/app`

**Decisão estrutural:** um `plugin telemetry.client.ts` (singleton) faz autocaptura
(`page_view` no `router.afterEach`, dwell, scroll, cliques de CTA via delegação `data-tel`,
`IntersectionObserver` único); `useTracking(slug)` vira wrapper fino sobre o singleton e mantém a
assinatura atual (não quebra call sites). Fila + batch (flush a cada 20 eventos / 5s / no
`pagehide` via `sendBeacon`; buffer offline em `sessionStorage`; backoff em 429).

**Identidade:** `tavola:did` (deviceId, persistente) + `tavola:sid` (`{id,startedAt,lastSeenAt}`,
reseta 30min). Migrar `tavola-sid` → `tavola:did`. `CartDrawer` passa a enviar o `sessionId` atual
no `POST /orders` e no `POST /events` (casa com `orders.session_id`) — **no mesmo release** (evita
janela em que o funil quebra).

**Context base (todo evento):** `eventId`, `deviceId`, `sessionId`, `sessionAgeMs`, `isReturning`,
`occurredAt`+`tzOffsetMin`+`localHour`, `route`+`pageName`, UTMs+`gclid`/`fbclid`, `referrerHost`
(1ª carga), `deviceType`/`viewport`/`screen`/`dpr`/`language`/`connection`. Resumir arrays (ids+count),
nunca objeto inteiro; teto 8KB.

**Arquivos:**

| Arquivo | Ação |
|---|---|
| `app/plugins/telemetry.client.ts` | **NOVO** — singleton (identidade, fila/batch, context base, `router.afterEach`, listeners visibility/scroll/pagehide/online, IntersectionObserver, autocaptura CTA/outbound, sendBeacon). |
| `app/composables/useTracking.ts` | **REESCREVER** — wrapper fino sobre `$telemetry`; expõe `track/trackPageView/startDwell/stopDwell/flush/sessionId/deviceId`; migra `tavola-sid`→`tavola:did`. |
| `app/components/ui/TDishCard.vue` | `product_impression` (viewport, agregado) + `product_clicked` (`listName`/`position` via props). |
| `app/pages/index.vue` / `cardapio.vue` | passar `listName`/`position`; `category_viewed` no watch de `activeCategory`. |
| `app/pages/prato/[productSlug].vue` | enriquecer `product_viewed`/`add_to_cart`; `startDwell({productId})` → `product_dwell`. |
| `app/components/sections/ProductBuyPanel.vue` | `product_option_changed` (variação/adicional). |
| `app/components/sections/MenuSidebar.vue` | `category_viewed` com `source`. |
| `app/components/public/CartDrawer.vue` | handlers nomeados novos: `onQtyChange`→`cart_qty_changed`, `onClearCart`→`cart_cleared`, `onTypeChange`→`checkout_type_changed`, `selectPayment`→`checkout_payment_selected`, `submit`→`checkout_submitted`/`checkout_failed`/`order_created`; **trocar `sessionId` enviado ao `/orders`**. |
| `app/components/public/PubHeader.vue` | `data-tel`/`outbound_click` no WhatsApp; `cart_opened` já existe. |
| `app/components/SectionRenderer.vue` | **adicionar `:data-block-id` + `data-tel-section`** no wrapper `.sr-block` (sem isso `section_dwell` e `cta_clicked.sectionId` ficam sem alvo). |
| `library/**` (CTAs/cupom/reserva) | `data-tel="cta"`/`data-tel-id`/`data-tel-label`; reserva/cupom manuais. |

---

## 8. Camada BACK — ingestão + schema + analytics (`back/internal/modules/cardapio/`)

### 8.1 Ingestão (Fase 1)
- **Endpoint batch** `POST /v1/public/restaurants/{slug}/events/batch`: body
  `{ sessionId, deviceId, events:[{eventId,name,occurredAt,context}] }`; até **50 eventos**,
  body **≤ 256KB** (decoder dedicado — `httpapi.ReadJSON` é 1MB fixo + `DisallowUnknownFields`),
  `context ≤ 8KB`/evento. Resposta `202 {accepted,rejected}` (best-effort; nome fora da allowlist
  ou context grande → `rejected++`, não derruba o lote). `account_id` resolvido 1x pelo slug.
- **Rate limit:** `allowN(scope, ip, n, ...)` debita N slots; chave **IP + slug**; orçamento
  ~600 eventos/min. `allow` atual vira `allowN(...,1,...)`. Singular legado migra p/ o mesmo bucket.
- **Enriquecimento server-side:** parse de User-Agent → `device_type`/`browser`/`os` (sem dep
  pesada; classifica `bot`); `referrer_host`; `ip_hash`; `created_at` (servidor) + `occurred_at`
  (cliente, clampado). Promove `product_slug`/`page_path`/`dwell_ms` do context p/ colunas.
- **Anti-PII:** deny-list de chaves no context; `menu_search` grava só `length`/`hasResults`.

### 8.2 Schema — migration `0174_cardapio_events_volume.sql` (idempotente, sem goose Down)
- `alter table cardapio.events add column if not exists`: `occurred_at timestamptz`, `event_id text`,
  `device_id text`, `page_path text`, `product_slug text`, `device_type text`, `browser text`,
  `os text`, `referrer_host text`, `utm_source/medium/campaign text`, `ip_hash text`,
  `dwell_ms integer` (todas com default → backfill implícito).
- `create table if not exists cardapio.sessions` (account_id, restaurant_id, session_id,
  device_id, first_seen/last_seen, duration_ms, pageviews, events, utm_*, referrer_host,
  device_type, landing_path, had_order bool; unique `(restaurant_id, session_id)`).
- Índices: `(restaurant_id, name, created_at)`, `(restaurant_id, session_id)`,
  `(restaurant_id, product_slug, created_at) where product_slug<>''`, dedupe parcial em `event_id`,
  e `cardapio.orders (restaurant_id, session_id) where session_id<>''`.
- Partição mensal de `events`: migration pronta, ativar quando o volume provar.

### 8.3 Analytics API (Fase 2) — base `/v1/cardapio/restaurants/{id}/analytics/*`
JWT + gating módulo + `cardapio.view`. `scopedAccountID` + 404 de pertencimento. Params comuns:
`from`/`to` (YYYY-MM-DD), `tz` (default America/Sao_Paulo). `Cache-Control: private, max-age=60`.
Span máx 90 dias. Bots excluídos por padrão.

| Endpoint | Conteúdo |
|---|---|
| `overview` | visitantes (sessão/device), sessões, pageviews, eventos, pedidos, conversão (orders/sessões), ticket médio, duração média, **novos vs recorrentes**, **abandono de sacola**. Lê `sessions`+`orders`. |
| `timeseries?granularity=day\|hour_of_day\|weekday_hour` | série por dia + distribuição por hora-do-dia + heatmap dia×hora ("horários"). |
| `funnel` | `restaurant_viewed→menu_viewed→product_viewed→add_to_cart→checkout_started→pedido` (join `events.session_id = orders.session_id`); por sessão; inclui taxa de abandono. |
| `top-products?metric=` | vistos/clicados/add/pedidos + **conversão visto→comprado** por `product_slug`; pedidos via `order_items`. |
| `sources?dimension=` | utm/referrer + sessões e pedidos por origem (lê `sessions`). |
| `devices` | breakdown device/browser/os (fonte canônica = coluna server-side do UA). |
| `pages?limit=` | páginas mais vistas + dwell médio. |
| `dwell?dimension=page\|product\|section` | tempo médio por página/produto/seção (`dwell_ms`, só `final:true`). |
| `clicks` | cliques agregados: `cta_clicked`/`outbound_click`/`whatsapp_order_clicked`/`coupon_used`/`reservation_sent` por label/kind ("quais botões"). |

Arquivos: `model_analytics.go`, `service_analytics.go`, `store_analytics.go`, `http_analytics.go`,
`service_analytics_test.go` + `RegisterAnalyticsRoutes`. Interface `dataStore` ganha as assinaturas
(+ stubs em `store_fake_test.go`). DTOs camelCase, centavos `int64`, durações em segundos/ms.

---

## 9. Camada PAINEL (dashboard) — `web/`

> **F4 EM ANDAMENTO (2026-06-25, código local).** Entregue: tipos `domain/cardapio/analytics.ts`
> (9 DTOs + `AnalyticsRange` + helpers de formato), composable `composables/useCardapioAnalytics.ts`
> (única camada de fetch; `store.analyticsRequest` herda `withScope`+`X-Account-Id`; 1 bloco/endpoint
> com data/pending/error; debounce ~250ms; `Promise.all` sem derrubar bloco; refresh por bloco),
> diretório `components/cardapio/analytics/` (infra `CardapioAnalyticsCard`/`Chart`/`Toolbar`/
> `DonutList`; blocos `Kpis`/`Trend`/`Hours`/`Funnel`/`TopProducts`/`Traffic`/`Devices`/`Dwell`/
> `Pages`/`Clicks`), orquestrador `sections/CardapioSectionRelatorios.vue`, e a aba **Relatórios**
> ligada em `CardapioEditorWorkspace.vue` (`gate:'all'`, após Pedidos). Reuso fiel: padrão do
> `MetaAdsReportChart` (Apex via `defineLazyComponent`+`ClientOnly`, tokens lidos em runtime),
> `MetaAdsOverviewCard` (KPIs+skeleton), `AppDatePicker` (range), `CoreErrorState` (erro com @retry
> real, sem reload). Vazio honesto "Sem dados neste período." em todos os blocos; zero mock.
> **DESVIO:** o erro de bloco usa `CoreErrorState` (que CoreAsyncError reusa) e não `CoreAsyncError`,
> pois este recarrega a página inteira no retry (perderia o escopo do editor) — `CoreErrorState`
> emite `@retry` real que re-chama só o `refreshX` do bloco. **Falta (passo do dono):** validação
> visual no dev contra os endpoints reais da F2.

Nova aba **"Relatórios"** dentro de `CardapioEditorWorkspace.vue` (`gate:'all'`) — herda
`restaurantId` + `scopeAccountId` do editor (`?accountId=` via `withScope`), sem re-resolver escopo.
Sub-gate `platform_admin` (com `LegacyMarker`) só para sessões/eventos crus (LGPD).

Diretório novo `web/app/components/cardapio/analytics/`. Orquestrador fino
`sections/CardapioSectionRelatorios.vue` + composable `useCardapioAnalytics.ts` (única camada de
fetch, herda `withScope`/`X-Account-Id` do `useCardapioStore`) + tipos `domain/cardapio/analytics.ts`.
Reuso: `MetaAdsReportChart`/`MetaAdsOverviewCard` (chart/KPI), `AppDatePicker` (range, pt-BR),
`CoreAsyncError` (erro), `OmniDataTable` (top-produtos). Estados loading/vazio honestos — **sem mock**.

Blocos (cada um consome 1 função do composable, sem fetch próprio): `Toolbar` (date range +
presets), `Kpis`, `Trend` (visitas×pedidos), `Hours` (barras hora + heatmap), `Funnel`,
`TopProducts` (+ conversão), `Traffic`, `Devices`, `Dwell`, `Clicks`. Infra reutilizável:
`AnalyticsCard` (shell loading/erro/vazio) + `AnalyticsChart` (wrapper ApexCharts tema-aware).
Cada arquivo ≤ 450 linhas, BEM `cardapio-analytics`, tokens de cor em runtime (zero hex).

---

## 10. Fases e paralelização

| Fase | Entrega | Depende de | Repo |
|---|---|---|---|
| **F0 — Contrato** | Este doc (allowlist/schema/endpoints/LGPD congelados) + roadmap. Confirmar workspace `site_tracking_web`. | — | — |
| **F1 — Back ingestão** | migration 0174, endpoint batch, allowlist+21, enriquecimento, anti-PII, dedupe, rate limit IP+slug, testes, AGENT.md. **Rebuild api.** | F0 | fila-atendimento |
| **F2 — Back analytics** | `*_analytics.go` (9 endpoints), 404 de pertencimento, lê `sessions`+`events`, testes. **Rebuild api.** | F1 (schema) | fila-atendimento |
| **F3 — Front TAVOLA** | plugin + useTracking + identidade + dwell/scroll/sendBeacon + batch + data-block-id + handlers CartDrawer + 36 eventos. | F1 (allowlist+batch) | TAVOLA |
| **F4 — Painel dashboard** | aba Relatórios + 14 componentes + composable + tipos. **EM ANDAMENTO** (código local 2026-06-25; falta validação visual no dev). | F2 (endpoints) | fila-atendimento |
| **F5 — LGPD/hardening** | aviso de privacidade/opt-out no TAVOLA, job de retenção/pruning, métrica de `rejected`, partição se volume. | F1–F4 | ambos |

**Paralelização (decisão do dono):** após a F1, **F2 (back) e F3 (front) rodam em paralelo** com
agentes em worktree (repositórios distintos → zero conflito de arquivo). F4 entra após a F2.
Disparar agentes **só com "ok" explícito** do dono.

## 11. Notas de Deploy (ordem exata)

1. Migration **`0174_cardapio_events_volume.sql`** (idempotente, schema-qualificada, sem goose Down;
   local: conferir porta do Postgres). Índices/colunas/tabela `sessions`.
2. **Rebuild api** (mudou Go, F1 e F2): `docker compose up -d --build api`. Restart não basta.
3. Envs novas: **`CARDAPIO_TELEMETRY_SALT`** (obrigatória em prod p/ `ip_hash`; em dev loga WARN no
   boot, sem derrubar) e **`CARDAPIO_TELEMETRY_RETENTION_DAYS`** (opcional, default 90; `<=0`
   desliga a poda diária) em `.env.production` E `docker-compose.prod.yml`. No TAVOLA:
   `NUXT_PUBLIC_TELEMETRY_DISABLED` (kill-switch global) + opt-out do visitante via
   `PubPrivacyNotice` (flag `tavola:telemetry-optout`, lida pelo plugin).
4. Front TAVOLA: build de prod via `nuxt generate` com `.env` de PROD (apiBase do Omni, devSlug vazio).
5. Validar preflight do batch: `curl -X OPTIONS .../v1/public/restaurants/{slug}/events/batch`.
6. Sincronizar doc canônico (este) + AGENT.md (cardapio back e web) + `roadmap-data.ts` + panorama HTML.

## 12. Itens a confirmar / débitos

- Confirmar existência do workspace `site_tracking_web` citado num spec (eventos crus) — este
  dashboard é por-estabelecimento e complementar, não o substitui.
- Banner de consentimento LGPD: débito documentado (decisão = aviso + legítimo interesse agora).
- Rollup pré-agregado (`events_daily`) e particionamento: só quando o volume provar (F5+).

---

## 13. Estado da implementação (HANDOFF — 2026-06-25)

> Snapshot para retomar sem perder contexto. Tudo é **código local, não commitado, sem deploy**.
> A api local foi rebuildada (migration 0174 aplicada no Postgres de dev). Branch:
> `refactor/multitenant-complete`.

### Status por fase
| Fase | Status | Validação feita | Pendência |
|---|---|---|---|
| F0 — Contrato/doc + roadmap | **CONCLUÍDA** | doc canônico + `roadmap-data.ts` (`card-f13-*`) | — |
| F1 — Back ingestão | **CONCLUÍDA + VALIDADA** | `go build/vet/test` OK; migration 0174 aplicada (`migration_up_ok`); smoke do `/events/batch` ao vivo: `{accepted:3,rejected:1}`, PII removida, `menu_search` sem termo cru, `device_type`/`referrer_host` parseados, `product_slug` inexistente descartado, sessão agregada gravada (dados de smoke já apagados) | — |
| F2 — Back analytics API | **CONCLUÍDA** (código) | `go build/vet/test` OK (12 testes); `golangci-lint` limpo; api no ar, 9 rotas registradas | Validação **autenticada** end-to-end (precisa token de sessão do painel → fazer no browser) |
| F3 — Front TAVOLA | **CÓDIGO COMPLETO** | — (não rodei `npm`/`generate` por regra) | **Validação visual no dev** (rodar TAVOLA dev e conferir eventos no banco) |
| F4 — Painel dashboard | **EM ANDAMENTO** (agente background) | — | Aguardando o agente terminar; depois revisar diff + validação visual no dev |
| F5 — LGPD/hardening | **CÓDIGO COMPLETO** (back validado) | rejected metric (`slog.Warn` de nomes fora da allowlist) + retenção/pruning diário (`PruneTelemetry`, default 90d, env `CARDAPIO_TELEMETRY_RETENTION_DAYS`, goroutine parada no `Close`): go build/vet/test OK, api rebuildada (poda = no-op em dev); aviso de privacidade + opt-out no TAVOLA (`PubPrivacyNotice` + flag lida pelo plugin) | partição mensal de `events` = débito (volume-gated); validação visual do aviso no dev |

### Onde está o trabalho (arquivos)
- **F1 (back `cardapio`):** migration `0174_cardapio_events_volume.sql`; novos `telemetry_enrich.go`, `store_sessions.go`; alterados `model_order.go` (allowlist 36 + structs batch), `rate_limit.go` (`allowN`), `store_events.go` (`InsertEventsBatch`), `service_public.go` (`RecordEventBatch`), `http_public.go` (`/events/batch`), `store.go`+`store_fake_test.go`, `service.go`+`module.go` (`TelemetrySalt`), `service_test.go`, `AGENT.md`. Detalhe na `back/internal/modules/cardapio/AGENT.md`.
- **F2 (back `cardapio`):** novos `model_analytics.go`, `service_analytics.go`, `store_analytics.go`, `store_analytics_detail.go`, `http_analytics.go`, `service_analytics_test.go`; alterados `store.go`, `store_fake_test.go`, `module.go` (`RegisterAnalyticsRoutes`), `AGENT.md`. Contrato dos 9 endpoints/DTOs: §8.3 + `model_analytics.go`.
- **F3 (TAVOLA `apps/web/app`):** novos `utils/telemetry.ts`, `plugins/telemetry.client.ts`; reescrito `composables/useTracking.ts`; editados `nuxt.config.ts` (kill-switch), `components/public/CartDrawer.vue` (sessionId→orders + 7 eventos), `pages/prato/[productSlug].vue`, `components/ui/TDishCard.vue`, `pages/cardapio.vue`, `components/sections/ProductBuyPanel.vue`, `components/SectionRenderer.vue` (`data-block-id`), `AGENT.md` (§7). Resumo na `Projects/TAVOLA/AGENT.md` §7.
- **F4 (painel `web/`):** sendo escrito por agente em background. **Output/transcript:** `…/57d8cd1f-…/tasks/afb10a719679c11c7.output`. Pelo que já registrou no §9 deste doc, criou `domain/cardapio/analytics.ts`, `composables/useCardapioAnalytics.ts`, `components/cardapio/analytics/*` (Card/Chart/Toolbar/DonutList + Kpis/Trend/Hours/Funnel/TopProducts/Traffic/Devices/Dwell/Pages/Clicks), `sections/CardapioSectionRelatorios.vue`, e ligou a aba em `CardapioEditorWorkspace.vue`. **Ao retomar:** se o agente concluiu, revisar o diff de `web/` e o report; se não, conferir o output e completar os arquivos faltantes (especialmente a edição de `CardapioEditorWorkspace.vue` e do store `cardapio.ts`).

### Como retomar (passos do dono / próxima sessão)
1. **F2 — confirmar rotas no ar (já feito):** api rebuildada; `GET /v1/cardapio/restaurants/{id}/analytics/overview` sem header dá `400 missing_account_id` (gating do prefixo). Validação real = abrir o painel logado.
2. **F3 — validar no dev:** no repo TAVOLA, `npm --prefix apps/web run dev` (`.env` com `apiBase=http://localhost:9091`, `devSlug=mk`). Navegar home→cardápio→prato→sacola→checkout e conferir:
   `docker compose exec postgres psql -U omni -d omni -c "select name,count(*) from cardapio.events group by 1 order by 2 desc;"`
   Esperado: `page_view`, `product_impression`, `product_dwell`, `scroll_depth`, `add_to_cart`, etc., e linha em `cardapio.sessions`. Conferir que o pedido criado tem o MESMO `session_id` dos eventos (funil).
3. **F4 — validar no dev:** abrir o painel `/cardapio/{id}` → aba **Relatórios**, conferir os blocos contra dados reais (precisa de tráfego do passo 2 para não vir vazio).
4. **F5 — implementar** (não iniciada).

### Regras de deploy a lembrar (quando for subir)
- Mudou Go (F1+F2) → **rebuild da imagem api** (`docker compose up -d --build api`); restart não basta. Migration 0174 roda no boot.
- Env nova **`CARDAPIO_TELEMETRY_SALT`** em `.env.production` E `docker-compose.prod.yml` (sem ela, `ip_hash` fica vazio — hoje em dev loga WARN no boot).
- TAVOLA: build prod via `nuxt generate` com `.env` de PROD (apiBase do Omni, devSlug vazio).
- Sincronizar ao finalizar: este doc + `back/.../cardapio/AGENT.md` + `web/.../cardapio/AGENT.md` + `TAVOLA/AGENT.md` + `roadmap-data.ts` (+ panorama HTML, hoje desatualizado).

### Follow-ups conhecidos (documentados, não bloqueiam o core)
- **F3:** marcar `data-tel="cta"`/`data-tel-id`/`data-tel-label` nos CTAs da `library/` (heroes/banners) para `cta_clicked`; disparar `coupon_*`/`reservation_*`/`menu_search`/`menu_filter_changed`/`menu_sort_changed`/`session_end` quando as UIs existirem (estão na allowlist mas ainda não disparam). `product_impression` hoje é 1 evento por card (otimização futura = agregar por viewport — crítica A5).
- **F2:** `dwell final:true` é inferido por `context->>'final' = 'false'` excluído (sem coluna dedicada); revisar se valer coluna.
- **Geral:** confirmar workspace `site_tracking_web`; consentimento LGPD (débito); rollup/partição quando o volume provar.
