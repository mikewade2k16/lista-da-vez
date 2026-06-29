# AGENT — Modulo Go `cardapio`

Modulo plugavel (Module Registry) dos cardapios online (restaurantes) do painel Omni.
Tenant-aware (schema `cardapio.*`, `account_id` FK `core.accounts` em TODA tabela). O
painel e o CRUD multitenant; a API publica (`/v1/public/*`) e consumida pelo **browser
do visitante** de um front Nuxt **estatico** hospedado no site do cliente.

> Plano canonico: `docs/cardapio/PLANO_MODULO_CARDAPIO.md` (fase C1). Espelhado em
> `web/app/components/roadmap/roadmap-data.ts` (fase `cardapio-online`).
> Contrato de saida (camelCase, centavos): types do front cardapio (Nuxt estatico).

## Estado: C1 — back + banco (2026-06-12) · Fase 2 back (2026-06-19) · Fase 10/F1 ingestao de telemetria (2026-06-25) · Fase 10/F2 API de analytics (2026-06-25) · slugify canonico (2026-06-29)

Entregue slugify canonico (2026-06-29): `normalizeSlug` em `service.go` passou a
delegar para `stringsx.Slugify` (NFD + sem acentos). Mudanca deliberada: slugs NOVOS
de restaurante/categoria/produto passam a ter acentos normalizados (ex.: "Acai" em vez
de "acai" de "Açaí"). Slugs ja gravados no banco NAO sao re-gerados.

Entregue C1: migration `0153_cardapio_schema.sql`, modulo Go completo, CORS publico no
middleware da plataforma, testes (recalculo de pedido, resolve por host, allowlist de
eventos, escopo multitenant 404, CORS publico). **Sem `app.go`** (registro central e da
integracao C3). Front `/cardapio` (C2) e wiring (C3) sao outras frentes.

Entregue Fase 2 (back): **WS-A** zonas de entrega (tabela `delivery_zones`, CRUD do painel,
frete por zona no pedido, `deliveryZones` no menu publico); **WS-B** `settings.payment`
informativo (jsonb, sem migration/rota); **WS-C** colunas extras do restaurante
(segment/facebook/youtube/GA/Pixel/HTML) + endereco no `address` jsonb. Front (painel +
TAVOLA) e seed do Mostarda (WS-E) sao outras frentes. Plano: `docs/cardapio/PLANO_CARDAPIO_FASE2.md`.

Entregue Fase 2 (**WS-F**, 2026-06-20): campos opcionais de catalogo do contrato TAVOLA —
`Category.imageUrl` (coluna + painel) e `Product.compareAtPriceCents` (coluna + painel),
migration `0168`; `Category.productCount` derivado no `service_public` (conta produtos
disponiveis por categoria, **sem coluna**; DTO `omitempty`). Auditoria do gap (TAVOLA x Omni):
o layout de secoes do site (sections-catalog / GET-PUT layout / editor) segue **ausente** —
e a Fase 3 (`docs/cardapio/PLANO_CARDAPIO_SITE_BUILDER.md`, decisao A/B em aberto).

Entregue Fase 3 (**back / Opcao B**, 2026-06-21): **site builder** — tabela `site_layouts`
(migration 0170), DTOs do layout (`model_layout.go`), `GET /v1/public/.../layout` (publicado,
ETag, **`Cache-Control: no-cache`** p/ publicar refletir num F5, 404->fallback) +
`PUT /v1/cardapio/.../layout` (rascunho, If-Match -> 412) + `POST .../layout/publish`
(mesmo auth do painel), validacao estrutural + version. `SiteLayout` (`model_layout.go`)
carrega `pages` + `theme` + **`announcement`** (`{enabled,text,link,linkLabel}` — faixa de
aviso do site, controlada pelo Studio): `validateSiteLayout` valida so `pages`/blocos, mas faz
`json.Unmarshal`->`json.Marshal`, entao campo de site NAO declarado no struct seria **DROPADO**
no save do painel — por isso `theme` e `announcement` precisam estar no struct. Front (Studio embed + aba no painel) =
Fases 2-3; **migracao layout-driven do site TAVOLA (home/cardapio/prato render-from-layout
com fallback curado) CONCLUIDA** no repo TAVOLA (sem deploy do back). Gating de
plano/sanitizacao/sections-catalog = Fase 4. Plano: `docs/cardapio/PLANO_CARDAPIO_SITE_BUILDER.md`.

Entregue Fase 2 (**WS-G**, 2026-06-20): **codigo do pedido** voltado ao cliente —
coluna `orders.code` (migration `0169`), gerada no `CreateOrder` (base32 Crockford,
6 chars, unica por restaurante via `uniqueOrderCode` + unique index parcial). Sai no
DTO `Order.code` (publico e painel); o `order_number` sequencial continua para uso interno.

Correcao (mover de conta atomico, 2026-06-22): o PATCH com `accountId` (mover de conta)
deixou de atualizar so `restaurants.account_id` (que deixava a subarvore orfã e o site
publico em `404`). Agora move a subarvore inteira numa transacao via
`MoveRestaurantToAccount` e auto-habilita o modulo `cardapio` em `core.account_modules` no
destino. Sem migration (so codigo Go); exige rebuild da api.

Fase 9 (gestao/UX do painel — back F1+F2 ENTREGUE, 2026-06-22 —
`docs/cardapio/PLANO_CARDAPIO_GESTAO_UX.md`):
- **F1 — duplicar restaurante** `POST /v1/cardapio/restaurants/{id}/duplicate`
  (so `platform_admin`; nao-admin => **403** forbidden, que aqui e correto pois e
  negacao de papel numa acao admin-only, nao leak de escopo). Body `{name, slug}`
  obrigatorios; slug livre globalmente (senao `ErrSlugConflict`/409). Source escopado
  por `scopedAccountID` (404 fora de escopo). Copia TRANSACIONAL na MESMA account do
  source (`DuplicateRestaurant` em `store_restaurants.go`, espelha
  `MoveRestaurantToAccount`): restaurante (novo id/slug/name, **`is_active=false`**,
  `last_order_number=0`, demais campos copiados), categorias, produtos (remapeando
  `category_id` por slug), `product_variations`/`product_addons` (ligados ao produto
  novo por slug), `delivery_zones`, `site_layouts` (draft+published+version). **NAO
  copia** `restaurant_domains`, `reviews`, `orders`/`order_items`, `events`. Resposta
  `201 {restaurant}` (full, novo id).
- **F2 — avaliacoes de estabelecimento** (migration `0171`): `reviews.product_id`
  agora nullable (NULL = review do **estabelecimento**) + coluna
  `show_on_establishment boolean default false` (marca review de produto para a
  vitrine do estabelecimento). Rotas novas: `GET /v1/cardapio/restaurants/{id}/reviews`
  (reviews do estabelecimento: `product_id IS NULL OR show_on_establishment = true`,
  order by sort_order) e `POST /v1/cardapio/restaurants/{id}/reviews` (cria com
  `product_id = NULL`; valida rating 1-5). `PATCH/DELETE /v1/cardapio/reviews/{id}`
  (existentes) gravam/leem `show_on_establishment`. DTO: `Review.productId` agora
  `omitempty` (scan nullable via `*string`) + `Review.showOnEstablishment`;
  `ReviewInput.showOnEstablishment`. Publico (`/v1/public/*`) NAO mexido — exposicao
  no TAVOLA e follow-up.
- Acesso do painel reusa `cardapio.manage`/`cardapio.orders.manage` (sem RBAC novo).
  Exige migration `0171` + rebuild api. Front (P1..P8) = outras frentes.

Fase 10 / F1 (ingestao de telemetria — ENTREGUE, 2026-06-25 —
`docs/cardapio/PLANO_CARDAPIO_TRACKING_ANALYTICS.md`, secoes 4/5/6/8.1/8.2): endpoint
publico de lote `POST /v1/public/restaurants/{slug}/events/batch` (best-effort,
`202 {accepted,rejected}`, 1..50 eventos, corpo <= 256KB, dedupe por `eventId`),
allowlist expandida para **36** (`model_order.go`), enriquecimento server-side
(User-Agent -> `device_type`/`browser`/`os`; `referrer_host`; `ip_hash` via
`CARDAPIO_TELEMETRY_SALT`; `occurred_at` clampado; `created_at` do servidor),
anti-PII (deny-list de chaves no context + `menu_search` so `length`/`hasResults`),
rate limit por IP no bucket `events` compartilhado (`allowN`, 600/min — singular legado
migrou pro mesmo orcamento), upsert da sessao agregada (`cardapio.sessions`), migration
`0174`. Front TAVOLA (F3) + dashboard (F4) = outras frentes. **Exige
migration `0174` + rebuild api.**

Fase 10 / F2 (API de analytics — ENTREGUE, 2026-06-25 —
`docs/cardapio/PLANO_CARDAPIO_TRACKING_ANALYTICS.md`, secoes 4/6/8.3): 9 GETs de
leitura/agregacao sob `/v1/cardapio/restaurants/{id}/analytics/*` (`overview`,
`timeseries`, `funnel`, `top-products`, `sources`, `devices`, `pages`, `dwell`,
`clicks`). Le `cardapio.sessions` (overview/sources/devices/timeseries) e cai em
`cardapio.events`/`orders`/`order_items` no detalhe (funil/top-produtos/dwell/cliques).
Escopo via `scopedAccountID` + validacao de pertencimento (`GetRestaurant`) => **404
uniforme** fora de escopo; permissao reusa `cardapio.view` (sem RBAC novo). Range
half-open `[from, to)` por `created_at`, span max 90 dias, bots excluidos, `Cache-Control:
private, max-age=60`. Calculos derivados (conversao sem div/0, taxas do funil monotonico,
serie densa, conversao visto->comprado) no service; SQL parametrizado/schema-qualificado e
sem regra de negocio no store. So leitura — nao mexe no schema. Detalhes na secao "Endpoints
de analytics do painel". Dashboard do painel (F4) = outra frente. **Exige rebuild api**
(mudanca Go, sem migration nova).

## Banco (`cardapio.*`, migration 0153)

| Tabela | Resumo |
|---|---|
| `restaurants` | entidade central; `slug` unico global `lower(slug)`; `address/hours/settings/theme jsonb`; `last_order_number int`; `is_active` |
| `restaurant_domains` | `host text PK` (normalizado por `normalizeHost`: lower, **sem esquema `http(s)://`**, sem path, sem porta, sem `www.` — aceita URL colada inteira sem virar lixo tipo `https`) -> restaurante |
| `categories` | unique `(restaurant_id, lower(slug))` |
| `products` | `price_cents bigint`; `gallery/diet/allergens/tags jsonb`; `pairing jsonb` nullable; unique `(restaurant_id, lower(slug))` |
| `product_variations` | `price_delta_cents` (soma) |
| `product_addons` | `price_cents` (cumulativo) |
| `reviews` | avaliacoes curadas (rating 1-5); `product_id` **nullable** (F2, migration 0171: NULL = review do estabelecimento) + `show_on_establishment boolean default false` (marca review de produto para a vitrine do estabelecimento) |
| `orders` | pedidos; `order_number` sequencial por restaurante; `code` (WS-G, migration 0169) curto p/ o cliente, unique parcial `(restaurant_id, code) where code<>''`; valores em centavos |
| `order_items` | snapshot (`product_id` nullable; `addons jsonb [{name,priceCents}]`) |
| `events` | telemetria do front publico; index `(restaurant_id, created_at)`. Fase 10/F1 (migration 0174) desnormalizou colunas server-side: `occurred_at`, `event_id`, `device_id`, `page_path`, `product_slug`, `device_type`, `browser`, `os`, `referrer_host`, `utm_source/medium/campaign`, `ip_hash`, `dwell_ms` (todas com default => backfill implicito). Indices: `(restaurant_id, name, created_at)`, `(restaurant_id, session_id)`, `(restaurant_id, product_slug, created_at) where product_slug<>''` e UNIQUE parcial `(restaurant_id, event_id) where event_id<>''` (dedupe via ON CONFLICT DO NOTHING) |
| `sessions` | (Fase 10/F1, migration 0174) sessao agregada na ingestao (upsert por `(restaurant_id, session_id)`): `device_id`, `first_seen_at`/`last_seen_at`, `duration_ms`, `pageviews`, `events`, `utm_*`, `referrer_host`, `device_type`, `landing_path`, `had_order` (sempre `false` no F1; ligado ao pedido em outra frente). Index `account_id` e `(restaurant_id, last_seen_at)`. Tambem: index parcial `orders (restaurant_id, session_id) where session_id<>''` (casa evento<->pedido) |
| `delivery_zones` | (Fase 2 / WS-A, migration 0166) bairro + `fee_cents`; unique `(restaurant_id, lower(name))`; index `account_id` e `(restaurant_id, sort_order)` |
| `site_layouts` | (Fase 3 / Opcao B, migration 0170) layout de secoes do site por restaurante: `draft`/`published` jsonb + `version`; unique `(restaurant_id)` |

**Colunas extras do restaurante (Fase 2 / WS-C, migration 0167):** `segment`,
`facebook`, `youtube`, `google_analytics_id`, `facebook_pixel_id`,
`custom_head_html` (todas `text not null default ''`). No DTO: `segment`,
`facebook`, `youtube`, `googleAnalyticsId`, `facebookPixelId`, `customHeadHtml`.
Numero/complemento/ponto de referencia do endereco entram no `address` jsonb
(`number`, `complement`, `reference` — opcionais, omitempty), sem coluna.

**Campos opcionais de catalogo (Fase 2 / WS-F, migration 0168):** `categories.image_url`
(`text not null default ''`) e `products.compare_at_price_cents` (`bigint not null default 0`).
No DTO: `Category.imageUrl`, `Product.compareAtPriceCents` (omitempty). `Category.productCount`
NAO tem coluna — e derivado em `service_public.PublicMenu` (conta produtos disponiveis por
categoria) e a foto da categoria e absolutizada como a do produto.

**Banner por categoria (migration 0173):** `categories.banner_url` (`text not null default ''`).
No DTO: `Category.bannerUrl` / `CategoryInput.bannerUrl`. Os tres campos de midia/texto da
categoria tem papeis distintos no site publico (TAVOLA): `description` = **SUBTITULO** (texto
curto sob o nome), `image_url`/`imageUrl` = **CAPA** (foto representativa, WS-F),
`banner_url`/`bannerUrl` = **BANNER** (imagem larga de topo da secao). Os tres saem no menu
publico via `ListCategories` (reusado por `service_public.PublicMenu`) e entram pelo
PATCH/POST full-replace de categoria (`CategoryInput`); o clone de restaurante
(`copyCategories`) preserva os tres. Sem mudanca nas rotas.
⚠️ `custom_head_html` = HTML livre injetado no site publico = risco de XSS; gate
de edicao (so platform_admin) e renderizacao controlada sao do front (nao do back).

Status do pedido: `recebido, em_preparo, pronto, saiu_entrega, entregue, cancelado`.
Tipo: `retirada, entrega, local` — validados no service contra as `settings`.

## Endpoints publicos (sem JWT, sem gating, CORS `*`, `Cache-Control: public, max-age=60` nos GETs)

| Verbo | Path | Resposta |
|---|---|---|
| GET | `/v1/public/resolve?host=` | `200 {slug}` / `404`. localhost+`CARDAPIO_DEV_DEFAULT_SLUG`; subdominio de `CARDAPIO_BASE_DOMAIN`; senao `restaurant_domains` |
| GET | `/v1/public/restaurants/{slug}` | `{restaurant, categories[], products[], deliveryZones[]}` (so ativos/disponiveis; `deliveryZones` so ativas, order by sort_order; variations/addons embutidos, sem N+1) |
| GET | `/v1/public/restaurants/{slug}/layout` | `SiteLayout` publicado (ETag + **`Cache-Control: no-cache`**, NAO `max-age=60` como os demais GETs publicos — assim publicar reflete num F5 do site; o ETag continua evitando payload repetido); `404` se sem publicado => o site cai no `defaultHomeLayout` |
| GET | `/v1/public/restaurants/{slug}/products/{productSlug}` | `{restaurant, product, reviews[]}` / `404` |
| POST | `/v1/public/restaurants/{slug}/orders` | `201 {order}` — **preco recalculado do banco** |
| POST | `/v1/public/restaurants/{slug}/events` | `202 {status:"ok"}`; nome fora da allowlist => `400`; `context` <= 8KB (singular legado, mantido p/ compat) |
| POST | `/v1/public/restaurants/{slug}/events/batch` | (Fase 10/F1) `202 {accepted,rejected}`. Body `{sessionId, deviceId, events:[{eventId,name,sessionId,occurredAt,context}]}`; 1..50 eventos, corpo <= 256KB (decoder dedicado `readBatchJSON` + `DisallowUnknownFields`), `context` <= 8KB/evento. Best-effort: nome fora da allowlist ou context grande => `rejected++`, NAO derruba o lote. `eventId` dedupe (ON CONFLICT DO NOTHING). `account_id` resolvido 1x pelo slug. Enriquece server-side: `device_type`/`browser`/`os` (User-Agent), `referrer_host` (Referer), `ip_hash` (sha256(ip+`CARDAPIO_TELEMETRY_SALT`); vazio sem salt), `occurred_at` (cliente, clamp [now-24h, now+5min]); `created_at` = SEMPRE relogio do servidor. Promove `productSlug` (so se pertencer ao restaurante)/`pagePath`/`dwellMs`/`deviceId`/`utm*` do context para coluna. Faz um `UpsertSession` agregado por lote. Anti-PII: deny-list de chaves no context (`name/phone/email/cpf/telefone/endereco/address`); `menu_search` grava so `{length,hasResults}`, descarta o termo cru |

- So serve restaurante `is_active` + account ativa + modulo `cardapio` habilitado
  (`core.account_modules`) — 1 join; senao `404` uniforme.
- Imagens absolutizadas via `PUBLIC_API_BASE_URL` (strings `/uploads/...` -> base+path).
- Erro no formato do contrato `{"error":{"code","message"}}` (pt-BR) via `httpapi.WriteError`.
  `writePublicError` mapeia erros ESPECIFICOS do checkout (type_unavailable, name_required,
  phone_required, empty_cart, below_min_order, item_unavailable, option_invalid) — nada de
  "Pedido invalido" generico para causas conhecidas.
- **`customer.address` no corpo do pedido**: `httpapi.ReadJSON` usa `DisallowUnknownFields`, e o
  front sempre envia `customer.address`; por isso `PublicCustomerInput` TEM o campo `Address`
  (jsonb livre). `PlaceOrder` usa `customer.address` como `deliveryAddress` (fallback p/ o campo
  top-level `deliveryAddress`). Remover esse campo volta a derrubar TODO pedido com `400`.
- Rate limit por (tenant, IP) em memoria (`rate_limit.go`): orders 10/min, events 600/min => `429`.
  A chave do bucket inclui o **slug** (`orders|<slug>`, `events|<slug>`), isolando o orcamento entre
  restaurantes (um tenant ruidoso nao consome a cota dos vizinhos). Independente do `RateLimit`
  global por user. **`clientIP` usa o ULTIMO hop de `X-Forwarded-For`** (o IP que o proxy confiavel
  — Caddy em prod — anexou), nao o primeiro: o primeiro elemento e 100% controlado pelo cliente e
  seria trivial de forjar para escapar do limite. **Premissa:** EXATAMENTE UM proxy confiavel na
  frente (Caddy); com mais de um proxy o ultimo hop deixa de ser o cliente e o calculo precisaria do
  numero de proxies. Em dev (Docker) todos os requests chegam com o IP do gateway, entao o limite vira
  efetivamente por-slug-global — `docker compose restart api` zera o bucket.

### Recalculo de pedido (`service_orders.go`)
`unitPrice = product.price_cents + variation.price_delta_cents + Σ addons.price_cents`;
total do item = unit × quantity; subtotal = Σ itens; deliveryFee so para `entrega`
(zera se subtotal >= `freeDeliveryAboveCents > 0`). **Frete por zona (WS-A):** o corpo
do pedido aceita `deliveryZoneId`; em entrega, se preenchido, a zona TEM que existir,
estar ativa e pertencer ao restaurante (`zoneForOrder`) — caso contrario `ErrOptionInvalid`
(`option_invalid`, 400). Com zona valida o frete base = `zone.fee_cents`; sem zona escolhida
cai no fallback `settings.deliveryFeeCents`. O frete gratis acima do limiar zera mesmo com
zona. O nome do bairro (zona) e gravado em `delivery_address.neighborhood` (merge no jsonb).
Valida tambem: tipo habilitado, 1-50 itens, quantity 1-50, nome obrigatorio, telefone se
entrega, produto existe/disponivel/do restaurante, variationId/addonIds pertencem ao produto,
subtotal >= `minOrderCents`. **O total enviado pelo cliente e ignorado.**

### Forma de pagamento no pedido (migration 0172)
O corpo do pedido aceita `paymentMethod` (token `pix`/`cash`/`debit`/`credit`/`ticket`/`other`)
e `changeForCents` (troco para). `resolvePayment` valida: vazio e aceito (cliente legado);
quando informado, TEM que ser uma forma que o restaurante aceita em `settings.payment`
(`paymentAccepted`) — senao `ErrPaymentInvalid` (`payment_invalid`, 400). `changeForCents` so
persiste em **entrega + dinheiro** (e > 0); nos demais casos zera. Persistido em
`cardapio.orders.payment_method` / `change_for_cents` (migration `0172_cardapio_order_payment.sql`,
colunas com default) e devolvido no DTO `Order`. **Rebuild da api obrigatorio** (mudanca Go):
`docker compose up -d --build api`.

### `settings.payment` (WS-B, informativo no menu publico — jsonb sem migration)
`settings` tem o sub-objeto `payment`: `{cash bool, debit {accepted bool, brands []string},
credit {accepted bool, brands []string}, pix bool, ticket bool, other string}` (camelCase).
Sai no menu publico (settings ja e serializado) e entra pelo PATCH do restaurante
(`UpdateRestaurantInput.Settings` e pointer). Define quais formas o checkout pode oferecer e
contra quais `resolvePayment` valida o `paymentMethod` do pedido (ver acima).

Allowlist de eventos (exata, 36 — Fase 10/F1, secao 5 de
`docs/cardapio/PLANO_CARDAPIO_TRACKING_ANALYTICS.md`): `page_view, session_start, session_end,
restaurant_viewed, menu_viewed, product_impression, product_clicked, product_viewed,
product_option_changed, add_to_cart, cart_qty_changed, remove_from_cart, cart_opened, cart_cleared,
checkout_started, checkout_type_changed, checkout_payment_selected, checkout_submitted,
checkout_failed, order_created, whatsapp_order_clicked, category_viewed, category_tab_clicked,
menu_search, menu_filter_changed, menu_sort_changed, cta_clicked, outbound_click, scroll_depth,
page_dwell, product_dwell, section_dwell, coupon_viewed, coupon_used, reservation_started,
reservation_sent`. Definida em `model_order.go` (`allowedEvents`); sincronizar com o teste de
allowlist (`service_test.go`) e o front TAVOLA. Rate limit do bucket `events` (singular + batch)
compartilha o mesmo orcamento (`allowN`, 600/min/IP); o batch debita `len(events)` de uma vez.

## Endpoints do painel (JWT + gating `/v1/cardapio` -> modulo `cardapio`)

`accountId` (query/header `X-Account-Id`) e filtro validado contra o Principal
(`scopedAccountID` em `http.go`): `platform_admin` ve qualquer account (ou todas na
listagem); demais papeis ficam fixos na propria account; accountId divergente => `404`
uniforme. Repo SEMPRE filtra `account_id` (defesa em profundidade).

**Permissao fina aplicada (gate por handler — `requireCardapioPerm` em `http.go`,
implementacao em `rbac.go`).** Alem do `RequireAuth` + gating de modulo no Chain, CADA
handler do painel agora checa a permissao no banco via `core.RBACService.HasAccountPermission`
(mesma usada por ~20 modulos). Regra:
- **GET** (listar/obter restaurante, dominios, catalogo, reviews, layout, zonas, pedidos,
  eventos e os 9 GETs de analytics) => `cardapio.view`.
- **Mutacao de catalogo/restaurante/layout/zona/dominio/media** (POST/PATCH/DELETE de
  restaurante, categoria, produto, review, dominio, zona, layout PUT/publish, upload de
  media) => `cardapio.manage`.
- **Mutacao de pedido** (`PATCH /v1/cardapio/orders/{id}`, troca de status) =>
  `cardapio.orders.manage`.

`platform_admin` (via `Principal.Role`) e `agency_owner` (da org dona da account, resolvido
no banco em `cardapioGate.isAgencyOwner`, espelhando `core.CanAccessAccountRoles`) entram em
**curto-circuito** (permitem sem checar a permissao fina). Falha de permissao => `ErrForbidden`,
que `writeServiceError` traduz para **404 uniforme** (nao vaza existencia/escopo). As rotas
PUBLICAS (`http_public.go`) NAO passam pelo gate. O gate e injetado no `Build` do modulo
(`module.go`: `core.NewRBACService(core.NewPostgresRBACRepository(deps.Pool))` -> `WithGate`),
sem tocar `app.go`. Nos testes (`newServiceWithStore`) o gate fica nil e e **fail-closed** (so
`platform_admin` passa) — os testes exercitam o service direto, nao os handlers HTTP.

`handleDuplicateRestaurant` mantem a negacao explicita por papel (so `platform_admin` => `403`
para nao-admin): e acao admin-only, negacao de papel, nao leak de escopo (ver Seguranca).

A LISTAGEM (`GET /v1/cardapio/restaurants`) usa `listScopeAccountID`: para `platform_admin`
o filtro vem SO do query `accountId` (vazio = todas as accounts, igual a bio) — o header
`X-Account-Id` serve so ao gating de modulo, nao restringe o que o admin enxerga. Ja as
rotas by-id usam `scopedAccountID` (query `accountId` tem precedencia sobre o header), entao
o painel passa `?accountId=` ao abrir restaurante de outra account (front: `?account=` na rota).

- Restaurants: `GET/POST /v1/cardapio/restaurants` (lista lean com `accountName` + dominio
  primario), `GET/PATCH/DELETE /v1/cardapio/restaurants/{id}`,
  `POST /v1/cardapio/restaurants/{id}/duplicate` (F1; **so `platform_admin` => 403 para
  nao-admin**; body `{name, slug}`; copia transacional do catalogo/zonas/layout sob novo
  id/slug, `is_active=false`, sem dominios/reviews/pedidos/eventos; `201 {restaurant}`).
  O PATCH aceita
  `accountId` (mover de conta — coluna **Cliente** da lista, edicao inline): so
  `platform_admin` move; o handler ZERA `in.AccountID` para nao-admin. No service
  (`resolveMoveAccount`, espelha bio): vazio/conta atual => nao move; conta destino
  inexistente => `404` (via `AccountExists`, antes do move).
  - **Mover de conta = move da SUBARVORE INTEIRA + auto-habilita o modulo (atomico).**
    Quando ha move, o service NAO usa o `UpdateRestaurant` generico: chama
    `MoveRestaurantToAccount` (`store_restaurants.go`), que numa UNICA transacao
    troca o `account_id` da raiz **e de todas as filhas** (`restaurant_domains`,
    `categories`, `products`, `product_variations`, `product_addons`, `reviews`,
    `delivery_zones`, `orders`, `order_items`, `events`, `site_layouts`) para a conta
    destino, e faz upsert em `core.account_modules (account_id, module_id='cardapio',
    enabled=true)` (PK `(account_id, module_id)`). **Auto-habilita o modulo no destino**
    (decisao de negocio). Sem isso o cardapio ficava orfao (filhas com `account_id`
    antigo) e o site publico caia (`404`, pois o publico exige `account_modules`
    habilitado na conta nova). A raiz e escopada pela conta ATUAL no WHERE
    (`account_id = $current`): 0 linhas => `404` (fora de escopo, sem vazar
    existencia). Retorna o restaurante ja sob a conta NOVA. As filhas sao escopadas
    pelo `restaurant_id` (variacoes/adicionais via subquery no produto; itens via
    subquery no pedido).
  - **PATCH com move + outros campos juntos:** o move tem precedencia e os demais
    campos do MESMO corpo sao ignorados (o painel dispara o move como edicao isolada
    da coluna Cliente, sem outros campos). O caminho SEM move segue no
    `UpdateRestaurant` generico (campos do restaurante, COALESCE; nunca toca
    `account_id`).
- Domains: `GET/POST /v1/cardapio/restaurants/{id}/domains`, `DELETE /v1/cardapio/domains?host=`.
  A coluna **Dominio** da lista edita o primario inline (front): host vazio = NO-OP (deletar
  e so na aba Dominios); preenchido = DELETE do primario antigo (se houver) + POST do novo
  como primario. Sem rota nova — reusa os endpoints de dominio existentes.
- Delivery zones (WS-A): `GET/POST /v1/cardapio/restaurants/{id}/delivery-zones`,
  `PATCH/DELETE /v1/cardapio/delivery-zones/{id}`. PATCH e parcial (pointer-based,
  `UpdateDeliveryZoneInput`) — toggle de `isActive` nao precisa do body inteiro.
- Site layout (Fase 3 / Opcao B): `GET/PUT /v1/cardapio/restaurants/{id}/layout` (rascunho; PUT com `If-Match` = version, 412 se conflito), `POST /v1/cardapio/restaurants/{id}/layout/publish` (promove rascunho->publicado). Mesmo auth/escopo dos demais. Validacao ESTRUTURAL (pages/blocks/id unico/type nao-vazio); gating de plano + sanitizacao pesada de props/theme + sections-catalog = Fase 4.
- Categories: `GET/POST /v1/cardapio/restaurants/{id}/categories`, `PATCH/DELETE /v1/cardapio/categories/{id}`.
- Products: `GET/POST /v1/cardapio/restaurants/{id}/products` (lean), `GET/PATCH/DELETE
  /v1/cardapio/products/{id}` (full; PATCH faz **replace-all transacional** de `variations[]`/`addons[]`).
- Reviews: `GET/POST /v1/cardapio/products/{id}/reviews` (por produto),
  `GET/POST /v1/cardapio/restaurants/{id}/reviews` (F2; do estabelecimento:
  `product_id IS NULL OR show_on_establishment = true`; o POST cria com `product_id NULL`),
  `PATCH/DELETE /v1/cardapio/reviews/{id}` (servem produto e estabelecimento; gravam
  `showOnEstablishment`).
- Orders: `GET /v1/cardapio/restaurants/{id}/orders?status=&page=&perPage=`, `PATCH /v1/cardapio/orders/{id}` (`{status}`).
- Events: `GET /v1/cardapio/restaurants/{id}/events?page=` (lista crua paginada).
- Media: `POST /v1/cardapio/restaurants/{id}/media` (multipart `file` -> `{url}`).

## Endpoints de analytics do painel (Fase 10 / F2)

Base `/v1/cardapio/restaurants/{id}/analytics/*` — **9 GETs** de leitura/agregacao
(`http_analytics.go`, `RegisterAnalyticsRoutes`). **Escopo:** `scopedAccountID(r,false)`
(query `accountId` tem precedencia sobre o header `X-Account-Id`, como as rotas by-id);
o service valida o PERTENCIMENTO do restaurante via `GetRestaurant(accountID,id)` ANTES de
qualquer agregacao — 0 linhas/fora de escopo => **404 uniforme** (nunca vazio silencioso,
nunca 403). **Permissao:** reusa `cardapio.view` (mesmo mecanismo dos demais GETs: `RequireAuth`
+ gating de modulo no Chain; sem RBAC novo). TODA query do store filtra `restaurant_id = $ AND
account_id = $` (defesa em profundidade), exclui bots por padrao (`device_type <> 'bot'`) e usa
**`created_at` do servidor** nas janelas/horarios. Resposta com `Cache-Control: private,
max-age=60`.

**Params comuns:** `from`/`to` (YYYY-MM-DD), `tz` (IANA, default `America/Sao_Paulo`; invalido
cai no default), `accountId` (validado), `limit` (default 20, max 100 onde aplica). Range default
= ultimos 30 dias; `from <= to`; `to` clampa em hoje; span maximo **90 dias** (senao
`ErrValidation`/400). O range vira half-open `[fromTs, toTsExclusive)` em UTC (fromTs = from 00:00
no tz; toTsExclusive = (to+1) 00:00 no tz), casando o indice `(restaurant_id, created_at)`.
Metricas/dimensoes/granularidades vem de allowlists fechadas (`model_analytics.go`); valor fora =>
`ErrValidation`/400.

| Endpoint | Conteudo / fonte |
|---|---|
| `overview` | KPIs: visitantes unicos (sessao e device), sessoes, pageviews, eventos, pedidos, conversao (orders/sessoes), ticket medio (sum total_cents/orders), duracao media, novos vs recorrentes, abandono de sacola (add_to_cart sem pedido / com add_to_cart). Le `sessions` + `orders` + `events`; taxas/medias derivadas no service (sem div/0) |
| `timeseries?granularity=day\|hour_of_day\|weekday_hour` | serie por dia (densa, 0 nos dias sem dado), distribuicao por hora-do-dia (0..23, todas) e heatmap dia-da-semana x hora (`at time zone $tz`). Pontos com visits/sessions/pageviews/orders |
| `funnel` | `restaurant_viewed -> menu_viewed -> product_viewed -> add_to_cart -> checkout_started -> pedido` por SESSAO (eventos via session distinct; pedido via `orders.session_id`); monotonicidade (min corrente) + `rateFromStart`/`rateFromPrev` no service; `session_id=''` excluido |
| `top-products?metric=viewed\|clicked\|add_to_cart\|orders&limit=` | por `product_slug`: contagem do evento + pedidos via `order_items` (join `orders`, status<>cancelado) + conversao visto->comprado; nome resolvido pelo catalogo (cai no slug); ranking + limit |
| `sources?dimension=utm_source\|utm_medium\|utm_campaign\|referrer` | sessoes e pedidos por origem (le `sessions`, utm/referrer ja agregados); value vazio => `(direto)` |
| `devices` | breakdown `device_type`/`browser`/`os` (le `sessions`; fonte canonica = coluna server-side do UA) |
| `pages?limit=` | paginas mais vistas (`page_view`, group by `page_path`) + dwell medio (segundos) |
| `dwell?dimension=page\|product\|section&limit=` | tempo medio (`avg dwell_ms/1000`) por `page_path`/`product_slug`/`sectionId` (do context); so amostras com `dwell_ms > 0` e descartando heartbeats parciais (`context->>'final' = 'false'`) |
| `clicks` | cliques agregados: `cta_clicked`/`outbound_click`/`whatsapp_order_clicked`/`coupon_used`/`reservation_sent` por label/kind (`ctaLabel`/`ctaKind`/`kind` do context) — "quais botoes" |

Front (painel dashboard, aba Relatorios) = F4 (outra frente). DTOs camelCase, centavos `int64`,
duracoes em segundos. **Exige rebuild api** (mudanca Go). Arquivos: `model_analytics.go`,
`service_analytics.go`, `store_analytics.go` + `store_analytics_detail.go`, `http_analytics.go`,
`service_analytics_test.go`.

## Permissoes e templates

- `cardapio.view`, `cardapio.manage`, `cardapio.orders.manage` (scope `account`).
- Templates: `cardapio.manager` (as 3), `cardapio.viewer` (view).
- **Aplicadas por handler** (ver "Endpoints do painel"): GET => `view`; mutacao de
  catalogo/restaurante/layout/zona => `manage`; mutacao de pedido => `orders.manage`.
  `platform_admin`/`agency_owner` em curto-circuito; falha => 404 uniforme. Antes desta
  fase os handlers so exigiam `RequireAuth` + gating de modulo (sem permissao fina).

## Media (`media_storage.go`)

Disco local `uploads/cardapio/{accountId}/...`, `0o750` dir / `0o600` arquivo. Aceita
**imagem e video** (mesmo endpoint/fluxo):
- Imagem: allowlist `jpeg/png/webp/gif`, sniff (`http.DetectContentType`) + fallback do
  header. Teto **5MB**.
- Video (fundo de hero/CTA/banner): allowlist `video/mp4` (.mp4), `video/webm` (.webm),
  `video/quicktime` (.mov). Sniff de video e fraco (mp4 cai em octet-stream), entao confia
  no `Content-Type` declarado **somente** se estiver na allowlist E a extensao do arquivo
  casar com o mime (nunca aceita mime arbitrario). Teto **60MB**.

O teto fino e escolhido por tipo dentro de `Save`; o handler le ate 60MB e o multipart
(`maxMediaMultipartBytes`) cobre o maior tipo. O **poster** (1o frame) do video e gerado
no cliente (Studio/TAVOLA) e subido como imagem normal — o back NAO gera poster nem usa
ffmpeg. A config de video (qual bloco usa, loop/mute etc.) vive no layout jsonb
(`block.props`), nao no schema. Caminho gravado relativo (`/uploads/cardapio/...`);
absolutizado no publico via `PUBLIC_API_BASE_URL`.

## Arquivos

`module.go` (Registry; monta o gate de permissao via `core.RBACService`) · `rbac.go`
(`cardapioGate`/`requireCardapioPerm` helpers + chaves `permView`/`permManage`/`permOrdersManage`)
· `model.go`/`model_order.go` (DTOs) · `store.go` (interface `dataStore`)
· `store_restaurants.go`/`store_catalog.go`/`store_orders.go`/`store_events.go`/`store_sessions.go`/`store_public.go`/`store_zones.go`
· `service.go`/`service_public.go`/`service_orders.go`/`service_analytics.go` (F2) · `telemetry_enrich.go`
(parse UA / ip_hash / referrer / sanitize anti-PII da ingestao) · `model_analytics.go` (DTOs + range +
allowlists do analytics) · `store_analytics.go`/`store_analytics_detail.go` (queries de leitura/agregacao)
· `media_storage.go` · `rate_limit.go` (`allow`/`allowN`)
· `http.go`/`http_catalog.go`/`http_orders.go`/`http_public.go`/`http_zones.go`/`http_analytics.go` (F2) · `errors.go` ·
`service_test.go`/`service_orders_test.go`/`service_gestao_test.go` (F1+F2)/`service_analytics_test.go` (Fase 10/F2)/`store_fake_test.go`.

## Variaveis de ambiente

- `PUBLIC_API_BASE_URL` — absolutiza `/uploads/*` no publico (compartilhada com bio).
- `CARDAPIO_BASE_DOMAIN` — resolve por subdominio (ex.: `tavola.app`); vazio desliga a convencao.
- `CARDAPIO_DEV_DEFAULT_SLUG` — opcional, so dev/local (host `localhost`).
- `UPLOADS_DIR` — raiz dos uploads (default `uploads`).
- `CARDAPIO_TELEMETRY_SALT` (Fase 10/F1) — salt do `ip_hash` da ingestao de telemetria
  (`sha256(ip + salt)`). **Vazia => `ip_hash` fica vazio (sem IP cru gravado) e o boot loga um
  WARN (`deps.Logger`); NAO derruba o modulo nem o boot.** Isto e **fail-closed quanto a PII**: na
  duvida nao grava o IP. Definir o salt em producao e a recomendacao (LGPD), mas o modulo NAO
  impoe isso no boot (decisao deliberada: o enforcement de boot seria cross-cutting; ver `module.go`).
- `CARDAPIO_TELEMETRY_RETENTION_DAYS` (Fase 10/F5) — opcional, default **90**. Janela da poda
  diaria de telemetria (`events` por `created_at` + `sessions` por `last_seen_at`); `<=0`
  desliga. `startRetentionLoop` (`telemetry_retention.go`) roda a 1a poda ~5min apos o boot,
  depois a cada 24h, e para no `handle.Close` (canal `stopPrune`). `PruneTelemetry`
  (`store_sessions.go`) faz DELETE parametrizado por `make_interval`. **Observabilidade F5:**
  nomes de evento fora da allowlist no batch sao logados (`slog.Warn`) para detectar drift de
  deploy fora de ordem, sem depender do front (fire-and-forget).

## Notas de Deploy

1. Migrations `0153_cardapio_schema.sql`, `0166_cardapio_delivery_zones.sql` (WS-A),
   `0167_cardapio_restaurant_extra.sql` (WS-C), `0168_cardapio_catalog_optional_fields.sql`
   (WS-F: `image_url` em categories, `compare_at_price_cents` em products),
   `0169_cardapio_order_code.sql` (WS-G: `code` em orders),
   `0170_cardapio_site_layouts.sql` (Fase 3 / Opcao B: `site_layouts`),
   `0171_cardapio_establishment_reviews.sql` (F2: `reviews.product_id` nullable +
   `show_on_establishment`), `0172_cardapio_order_payment.sql` (forma de pagamento +
   troco no pedido), `0173_cardapio_category_banner.sql` (`banner_url` em categories),
   `0174_cardapio_events_volume.sql` (Fase 10/F1: colunas desnormalizadas em `events`,
   tabela `sessions`, indices de analytics + UNIQUE parcial de dedupe por `event_id` +
   index parcial `orders (restaurant_id, session_id)`; idempotente, sem goose Down)
   (local: conferir porta do Postgres).
   Obs.: os numeros 0154/0155 citados no plano ja estavam ocupados (site_product_*);
   as migrations da Fase 2 foram para 0166/0167 (proximos livres).
2. **Rebuild api** (mudou Go): `docker compose up -d --build api`. F1 (duplicar, so codigo
   Go) + F2 (reviews de estabelecimento, migration `0171`) exigem rebuild da api. A
   correcao do move atomico (2026-06-22) e so codigo Go, mas tambem exige rebuild. A
   Fase 10/F1 (ingestao de telemetria) mudou Go + migration `0174` — exige rebuild. O gate de
   permissao fina do painel (esta fase) e so codigo Go (sem migration) — exige rebuild da api.
3. Envs novas em `.env.production` E `docker-compose.prod.yml`: `CARDAPIO_BASE_DOMAIN`,
   `CARDAPIO_DEV_DEFAULT_SLUG` (opcional), `CARDAPIO_TELEMETRY_SALT` (Fase 10/F1, recomendada
   em producao p/ o `ip_hash` da telemetria; vazia loga WARN no boot e nao grava IP — fail-closed
   quanto a PII, NAO derruba o boot),
   `CARDAPIO_TELEMETRY_RETENTION_DAYS` (Fase 10/F5, opcional, default 90; `<=0` desliga a poda).
   `PUBLIC_API_BASE_URL` ja existe (bio).
4. Registro central (integracao C3, NAO neste modulo):
   `registry.MustRegister(cardapio.New())` + gating `{Prefix: "/v1/cardapio", ModuleID: "cardapio"}`.
5. Validar preflight: `curl -X OPTIONS -H "Origin: https://qualquer.com" <api>/v1/public/resolve`
   => `204` com `Access-Control-Allow-Origin: *`.

## Seguranca

- **Permissao fina por handler do painel** (`requireCardapioPerm` -> `cardapioGate.Authorize`,
  `rbac.go`): GET => `cardapio.view`; mutacao de catalogo/restaurante/layout/zona => `cardapio.manage`;
  mutacao de pedido => `cardapio.orders.manage`. Resolve no banco (`HasAccountPermission`:
  role_permissions + overrides allow/deny). `platform_admin`/`agency_owner` em curto-circuito.
  Falha => `ErrForbidden` => **404 uniforme** (nao 403; nao vaza existencia). Rotas publicas NAO
  gateadas. Gate **fail-closed** sem RBAC injetado (so platform_admin passa).
- `account_id` nunca vem do body cru; o accountId de query/header e filtro validado contra o
  Principal. Fora do escopo => `404` (nunca 403; nao vaza existencia).
- Mover de conta (`MoveRestaurantToAccount`) so para `platform_admin` (handler zera
  `in.AccountID` p/ nao-admin); a raiz e escopada pela conta ATUAL (0 linhas => `404`); todas
  as filhas trocam de `account_id` na MESMA transacao (sem orfaos cross-tenant).
- Duplicar (F1) so para `platform_admin`: o handler nega nao-admin com **403** (negacao de
  papel numa acao admin-only; nao e leak de escopo). O source e escopado pela account
  (`DuplicateRestaurant` faz o insert-from-select com `where account_id = $`: 0 linhas =>
  `404`). A copia mantem o `account_id` do source (nunca cruza tenant).
- SQL 100% parametrizado. Pedido publico recalcula tudo do banco (cliente nao define preco).
- CORS publico cookie-less: `Allow-Origin: *`, **nunca** `Allow-Credentials`.
