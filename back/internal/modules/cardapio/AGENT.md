# AGENT — Modulo Go `cardapio`

Modulo plugavel (Module Registry) dos cardapios online (restaurantes) do painel Omni.
Tenant-aware (schema `cardapio.*`, `account_id` FK `core.accounts` em TODA tabela). O
painel e o CRUD multitenant; a API publica (`/v1/public/*`) e consumida pelo **browser
do visitante** de um front Nuxt **estatico** hospedado no site do cliente.

> Plano canonico: `docs/cardapio/PLANO_MODULO_CARDAPIO.md` (fase C1). Espelhado em
> `web/app/components/roadmap/roadmap-data.ts` (fase `cardapio-online`).
> Contrato de saida (camelCase, centavos): types do front cardapio (Nuxt estatico).

## Estado: C1 — back + banco (2026-06-12)

Entregue: migration `0153_cardapio_schema.sql`, modulo Go completo, CORS publico no
middleware da plataforma, testes (recalculo de pedido, resolve por host, allowlist de
eventos, escopo multitenant 404, CORS publico). **Sem `app.go`** (registro central e da
integracao C3). Front `/cardapio` (C2) e wiring (C3) sao outras frentes.

## Banco (`cardapio.*`, migration 0153)

| Tabela | Resumo |
|---|---|
| `restaurants` | entidade central; `slug` unico global `lower(slug)`; `address/hours/settings/theme jsonb`; `last_order_number int`; `is_active` |
| `restaurant_domains` | `host text PK` (normalizado: lower, sem porta, sem `www.`) -> restaurante |
| `categories` | unique `(restaurant_id, lower(slug))` |
| `products` | `price_cents bigint`; `gallery/diet/allergens/tags jsonb`; `pairing jsonb` nullable; unique `(restaurant_id, lower(slug))` |
| `product_variations` | `price_delta_cents` (soma) |
| `product_addons` | `price_cents` (cumulativo) |
| `reviews` | avaliacoes curadas por produto (rating 1-5) |
| `orders` | pedidos; `order_number` sequencial por restaurante; valores em centavos |
| `order_items` | snapshot (`product_id` nullable; `addons jsonb [{name,priceCents}]`) |
| `events` | telemetria do front publico; index `(restaurant_id, created_at)` |

Status do pedido: `recebido, em_preparo, pronto, saiu_entrega, entregue, cancelado`.
Tipo: `retirada, entrega, local` — validados no service contra as `settings`.

## Endpoints publicos (sem JWT, sem gating, CORS `*`, `Cache-Control: public, max-age=60` nos GETs)

| Verbo | Path | Resposta |
|---|---|---|
| GET | `/v1/public/resolve?host=` | `200 {slug}` / `404`. localhost+`CARDAPIO_DEV_DEFAULT_SLUG`; subdominio de `CARDAPIO_BASE_DOMAIN`; senao `restaurant_domains` |
| GET | `/v1/public/restaurants/{slug}` | `{restaurant, categories[], products[]}` (so ativos/disponiveis; variations/addons embutidos, sem N+1) |
| GET | `/v1/public/restaurants/{slug}/products/{productSlug}` | `{restaurant, product, reviews[]}` / `404` |
| POST | `/v1/public/restaurants/{slug}/orders` | `201 {order}` — **preco recalculado do banco** |
| POST | `/v1/public/restaurants/{slug}/events` | `202 {status:"ok"}`; nome fora da allowlist => `400`; `context` <= 8KB |

- So serve restaurante `is_active` + account ativa + modulo `cardapio` habilitado
  (`core.account_modules`) — 1 join; senao `404` uniforme.
- Imagens absolutizadas via `PUBLIC_API_BASE_URL` (strings `/uploads/...` -> base+path).
- Erro no formato do contrato `{"error":{"code","message"}}` (pt-BR) via `httpapi.WriteError`.
- Rate limit por IP em memoria (`rate_limit.go`): orders 10/min, events 60/min => `429`.
  Independente do `RateLimit` global por user.

### Recalculo de pedido (`service_orders.go`)
`unitPrice = product.price_cents + variation.price_delta_cents + Σ addons.price_cents`;
total do item = unit × quantity; subtotal = Σ itens; deliveryFee = `settings.deliveryFeeCents`
so para `entrega` (zera se subtotal >= `freeDeliveryAboveCents > 0`). Valida: tipo habilitado,
1-50 itens, quantity 1-50, nome obrigatorio, telefone se entrega, produto existe/disponivel/do
restaurante, variationId/addonIds pertencem ao produto, subtotal >= `minOrderCents`. **O total
enviado pelo cliente e ignorado.**

Allowlist de eventos (exata): `page_view, restaurant_viewed, menu_viewed, category_viewed,
product_viewed, product_clicked, add_to_cart, remove_from_cart, cart_opened, checkout_started,
whatsapp_order_clicked, reservation_started, reservation_sent, coupon_viewed, coupon_used`.

## Endpoints do painel (JWT + gating `/v1/cardapio` -> modulo `cardapio`)

`accountId` (query/header `X-Account-Id`) e filtro validado contra o Principal
(`scopedAccountID` em `http.go`): `platform_admin` ve qualquer account (ou todas na
listagem); demais papeis ficam fixos na propria account; accountId divergente => `404`
uniforme. Repo SEMPRE filtra `account_id` (defesa em profundidade).

- Restaurants: `GET/POST /v1/cardapio/restaurants` (lista lean com `accountName` + dominio
  primario), `GET/PATCH/DELETE /v1/cardapio/restaurants/{id}`.
- Domains: `GET/POST /v1/cardapio/restaurants/{id}/domains`, `DELETE /v1/cardapio/domains?host=`.
- Categories: `GET/POST /v1/cardapio/restaurants/{id}/categories`, `PATCH/DELETE /v1/cardapio/categories/{id}`.
- Products: `GET/POST /v1/cardapio/restaurants/{id}/products` (lean), `GET/PATCH/DELETE
  /v1/cardapio/products/{id}` (full; PATCH faz **replace-all transacional** de `variations[]`/`addons[]`).
- Reviews: `GET/POST /v1/cardapio/products/{id}/reviews`, `PATCH/DELETE /v1/cardapio/reviews/{id}`.
- Orders: `GET /v1/cardapio/restaurants/{id}/orders?status=&page=&perPage=`, `PATCH /v1/cardapio/orders/{id}` (`{status}`).
- Events: `GET /v1/cardapio/restaurants/{id}/events?page=` (lista crua paginada).
- Media: `POST /v1/cardapio/restaurants/{id}/media` (multipart `file` -> `{url}`).

## Permissoes e templates

- `cardapio.view`, `cardapio.manage`, `cardapio.orders.manage` (scope `account`).
- Templates: `cardapio.manager` (as 3), `cardapio.viewer` (view).

## Media (`media_storage.go`)

Disco local `uploads/cardapio/{accountId}/...`, `0o750` dir / `0o600` arquivo, imagem ate
5MB, mime allowlist (jpeg/png/webp/gif) com sniff + fallback do header. Caminho gravado
relativo (`/uploads/cardapio/...`); absolutizado no publico via `PUBLIC_API_BASE_URL`.

## Arquivos

`module.go` (Registry) · `model.go`/`model_order.go` (DTOs) · `store.go` (interface `dataStore`)
· `store_restaurants.go`/`store_catalog.go`/`store_orders.go`/`store_events.go`/`store_public.go`
· `service.go`/`service_public.go`/`service_orders.go` · `media_storage.go` · `rate_limit.go`
· `http.go`/`http_catalog.go`/`http_orders.go`/`http_public.go` · `errors.go` ·
`service_test.go`/`service_orders_test.go`/`store_fake_test.go`.

## Variaveis de ambiente

- `PUBLIC_API_BASE_URL` — absolutiza `/uploads/*` no publico (compartilhada com bio).
- `CARDAPIO_BASE_DOMAIN` — resolve por subdominio (ex.: `tavola.app`); vazio desliga a convencao.
- `CARDAPIO_DEV_DEFAULT_SLUG` — opcional, so dev/local (host `localhost`).
- `UPLOADS_DIR` — raiz dos uploads (default `uploads`).

## Notas de Deploy

1. Migration `0153_cardapio_schema.sql` (local: conferir `:5433`).
2. **Rebuild api** (mudou Go): `docker compose up -d --build api`.
3. Envs novas em `.env.production` E `docker-compose.prod.yml`: `CARDAPIO_BASE_DOMAIN`,
   `CARDAPIO_DEV_DEFAULT_SLUG` (opcional). `PUBLIC_API_BASE_URL` ja existe (bio).
4. Registro central (integracao C3, NAO neste modulo):
   `registry.MustRegister(cardapio.New())` + gating `{Prefix: "/v1/cardapio", ModuleID: "cardapio"}`.
5. Validar preflight: `curl -X OPTIONS -H "Origin: https://qualquer.com" <api>/v1/public/resolve`
   => `204` com `Access-Control-Allow-Origin: *`.

## Seguranca

- `account_id` nunca vem do body cru; o accountId de query/header e filtro validado contra o
  Principal. Fora do escopo => `404` (nunca 403; nao vaza existencia).
- SQL 100% parametrizado. Pedido publico recalcula tudo do banco (cliente nao define preco).
- CORS publico cookie-less: `Allow-Origin: *`, **nunca** `Allow-Credentials`.
