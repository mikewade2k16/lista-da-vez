# Plano — Módulo Cardápio Online (`cardapio`)

> Doc canônico do módulo `cardapio`. Espelhado em `web/app/components/roadmap/roadmap-data.ts` (fase `cardapio-online`).
> Contrato de saída: "Contrato da API pública" do repo do front cardápio (Nuxt estático). **Fonte da verdade do shape é `apps/web/app/types.ts` daquele repo** — em caso de dúvida, o type vence.
> Criado em 2026-06-12. Status: C1+C2 ENTREGUES e wiring central APLICADO (2026-06-12, subagentes Opus C/D + integração). Falta C3: aplicar migration 0153 + rebuild api + habilitar módulo na account Crow + e2e/preflight CORS (passos do usuário) e tirar o `hidden`. Nome "cardapio-online" é provisório (renomear depois é só label).

---

## 1. Objetivo

Gerir no painel Omni os **cardápios online** (restaurantes) servidos por um front Nuxt **estático** que roda no host do cliente (outro site/servidor). O front não tem backend: tudo vem da nossa API pública (`/v1/public/*`, chamada **do browser** do visitante). O painel é o CRUD de restaurante, categorias, produtos (variações/adicionais), avaliações, domínios e pedidos recebidos.

Requisitos do produto:

- **Multitenant igual à bio**: restaurante sempre pertence a uma account. Por enquanto todos ficam na **account da agência Crow**; quando um cliente assumir, é só criar/trocar a account (mesmo modelo "cliente de bio").
- Cliente vê e edita só os restaurantes da própria account; `platform_admin` gerencia todos com filtro por cliente.
- Menu: **página própria** top-level "Cardápio Online" (`/cardapio`), não dentro de Site.
- Multitenancy público resolvida **por host**: um bundle só servido em vários domínios; o front pergunta `GET /v1/public/resolve?host=` e recebe o slug.

## 2. Diferenças críticas vs módulo bio

| Aspecto | Bio | Cardápio |
|---|---|---|
| Quem chama a API pública | Servidor Nuxt (SSR, server-to-server) | **Browser do visitante** (site estático) |
| CORS | Irrelevante | **Obrigatório**: `Access-Control-Allow-Origin: *` em `/v1/public/*`, preflight `OPTIONS` → 204, sem credenciais/cookie |
| Dados | jsonb (contrato profundo, leitura) | **Normalizado** (recálculo de preço por id, ordenação, disponibilidade, pedidos) |
| Escrita pública | Nenhuma | `POST orders` + `POST events` → **rate limit por IP** e validação forte |
| Dinheiro | — | Sempre inteiro em **centavos** (`...Cents`) |

## 3. Banco — migration `0153_cardapio_schema.sql`

Idempotente, schema qualificado, sem `-- +goose Down`. Schema `cardapio`. **Toda tabela tem `account_id uuid not null references core.accounts(id)`** (defesa em profundidade), além do FK de agregação.

| Tabela | Campos principais |
|---|---|
| `cardapio.restaurants` | id, account_id, slug (único global `lower(slug)`), name, tagline, description, logo_url, banner_url, whatsapp, phone, email, instagram, `address jsonb`, `hours jsonb`, `settings jsonb` (deliveryFeeCents, deliveryEnabled, pickupEnabled, dineInEnabled, minOrderCents, freeDeliveryAboveCents), `theme jsonb` (tokens livres), is_active, `last_order_number int default 0`, created_at, updated_at |
| `cardapio.restaurant_domains` | `host text primary key` (normalizado: lowercase, sem porta, sem `www.`), restaurant_id FK, account_id, is_primary, created_at |
| `cardapio.categories` | id, account_id, restaurant_id FK, slug, name, description, sort_order, is_active, created_at. Unique `(restaurant_id, lower(slug))` |
| `cardapio.products` | id, account_id, restaurant_id, category_id FK nullable, slug, name, short_desc, description, body, price_cents, image_url, `gallery jsonb`, weight, cook_time, `diet jsonb`, `allergens jsonb`, `pairing jsonb` nullable, `tags jsonb`, is_available, is_featured, sort_order, rating numeric nullable, review_count, sold_count, created_at, updated_at. Unique `(restaurant_id, lower(slug))` |
| `cardapio.product_variations` | id, account_id, product_id FK, name, price_delta_cents, sort_order |
| `cardapio.product_addons` | id, account_id, product_id FK, name, price_cents, sort_order |
| `cardapio.reviews` | id, account_id, restaurant_id, product_id FK, author_name, author_level, rating (1-5), body, is_highlight, date_label, sort_order, created_at |
| `cardapio.orders` | id, account_id, restaurant_id, customer_id nullable, order_number, status, type, session_id, customer_name, customer_phone, `delivery_address jsonb`, notes, subtotal_cents, delivery_fee_cents, discount_cents, total_cents, created_at, updated_at |
| `cardapio.order_items` | id, account_id, order_id FK, product_id nullable (snapshot sobrevive a delete), product_name, variation_name, `addons jsonb` (`[{name, priceCents}]`), quantity, unit_price_cents, total_cents, notes |
| `cardapio.events` | id, account_id, restaurant_id, name, session_id, `context jsonb`, created_at. Index `(restaurant_id, created_at)` |

Índices: `account_id` em todas; `restaurant_id` nas filhas; `(restaurant_id, status, created_at)` em orders.

`status ∈ {recebido, em_preparo, pronto, saiu_entrega, entregue, cancelado}` · `type ∈ {retirada, entrega, local}` — validados no service (CHECK opcional).

## 4. Backend — `back/internal/modules/cardapio/`

Módulo próprio no Module Registry (padrão `automation`/`meta_ads`/`bio`): ID `cardapio`, schema `cardapio`. Arquivos divididos por agregado (máx 450 linhas/arquivo):

| Arquivo | Responsabilidade |
|---|---|
| `module.go` | Registry: metadata, permissões, role templates, Build/handle |
| `model.go` / `model_order.go` | DTOs em camelCase EXATO do contrato (Restaurant, Category, Product+Variation+Addon, Review, Order+OrderItem, requests) |
| `store_restaurants.go` | CRUD restaurants + domains (host normalizado) |
| `store_catalog.go` | CRUD categories/products/variations/addons (variations/addons: replace-all transacional no PATCH do produto); reviews |
| `store_orders.go` | Insert transacional de pedido (order_number via `UPDATE restaurants SET last_order_number = last_order_number + 1 ... RETURNING`), listagem paginada, update de status |
| `store_events.go` | Insert de evento + listagem paginada |
| `service.go` | Escopo multitenant (accountId filtro dentro do permitido; fora → 404), CRUD do painel |
| `service_public.go` | `Resolve(host)` (normaliza → subdomínio `CARDAPIO_BASE_DOMAIN` → tabela domains → opcional `CARDAPIO_DEV_DEFAULT_SLUG` p/ localhost), cardápio completo (só ativos/disponíveis), prato + reviews, eventos (allowlist) |
| `service_orders.go` | **Recalcula tudo do banco**: preço produto + delta variação + adicionais; valida type, 1-50 itens, quantity 1-50, nome, telefone se entrega, variação/adicional pertencem ao produto, produto disponível; delivery fee de `settings` (zera acima de `freeDeliveryAboveCents`); monta snapshot dos itens |
| `media_storage.go` | Upload local `uploads/cardapio/{accountId}/...` (`0o750`/`0o600`, allowlist mime, imagem 5MB) |
| `http.go` / `http_catalog.go` / `http_orders.go` | Rotas do painel (JWT + X-Account-Id, gating) |
| `http_public.go` | Rotas públicas + **formato de erro do contrato** `{"error":{"code","message"}}` (mensagens pt-BR) + rate limit por IP em memória (orders 10/min, events 60/min) + `Cache-Control: public, max-age=60` nos GETs |
| `service_test.go` / `service_orders_test.go` | Resolve por host, escopo 404, allowlist de eventos, **recálculo de pedido** (casos: variação, adicionais, frete grátis, item indisponível, quantidade inválida) |
| `AGENT.md` | Doc do módulo |

### Endpoints públicos (sem JWT, sem gating, CORS `*`)

Exatos do contrato (prefixo `/v1/public`):

| Verbo | Path | Resposta |
|---|---|---|
| GET | `/v1/public/resolve?host=` | `200 {slug}` / `404` |
| GET | `/v1/public/restaurants/{slug}` | `200 {restaurant, categories[], products[]}` (products já com variations/addons embutidos; só ativos/disponíveis) / `404` |
| GET | `/v1/public/restaurants/{slug}/products/{productSlug}` | `200 {restaurant, product, reviews[]}` / `404` |
| POST | `/v1/public/restaurants/{slug}/orders` | `201 {order}` (preços recalculados no servidor) |
| POST | `/v1/public/restaurants/{slug}/events` | `202 {status:"ok"}`; nome fora da allowlist → `400`; `context` ≤ 8KB |

Allowlist de eventos: `page_view, restaurant_viewed, menu_viewed, category_viewed, product_viewed, product_clicked, add_to_cart, remove_from_cart, cart_opened, checkout_started, whatsapp_order_clicked, reservation_started, reservation_sent, coupon_viewed, coupon_used`.

Público só serve restaurante `is_active` + account ativa + módulo `cardapio` habilitado na account (1 join) — senão `404` uniforme. Imagens absolutizadas via `PUBLIC_API_BASE_URL` (mesma env do módulo bio).

### CORS wildcard em `/v1/public/*` (mudança no platform)

`httpapi.CORS` hoje só seta `Allow-Origin` para a allowlist e intercepta `OPTIONS` globalmente — preflight do site estático falharia. Mudança em `middleware.go` (única alteração fora do módulo, **só o subagente C toca**): se `r.URL.Path` começa com `/v1/public/`, setar `Access-Control-Allow-Origin: *`, `Access-Control-Allow-Methods: GET, POST, OPTIONS`, `Access-Control-Allow-Headers: Content-Type` e responder `OPTIONS` 204 — **sem** `Allow-Credentials` (cookie-less). Rotas não-públicas seguem allowlist intocada. Cobrir com teste em `middleware_test` (origem qualquer em `/v1/public/...` vs rota normal).

### Endpoints do painel (gating `/v1/cardapio` → módulo `cardapio`)

| Verbo | Path |
|---|---|
| GET | `/v1/cardapio/restaurants?accountId=&q=` (lean: id, accountId, accountName, slug, name, isActive, domínio primário, updatedAt) |
| POST | `/v1/cardapio/restaurants` (`{accountId?, slug, name}`; não-admin usa account do contexto) |
| GET/PATCH/DELETE | `/v1/cardapio/restaurants/{id}` (PATCH cobre dados, address/hours/settings/theme, isActive) |
| GET/POST | `/v1/cardapio/restaurants/{id}/categories` · PATCH/DELETE `/v1/cardapio/categories/{id}` |
| GET/POST | `/v1/cardapio/restaurants/{id}/products` (lista lean) · GET/PATCH/DELETE `/v1/cardapio/products/{id}` (full; PATCH aceita `variations[]`/`addons[]` replace-all) |
| GET/POST | `/v1/cardapio/products/{id}/reviews` · PATCH/DELETE `/v1/cardapio/reviews/{id}` |
| GET | `/v1/cardapio/restaurants/{id}/orders?status=&page=&perPage=` · PATCH `/v1/cardapio/orders/{id}` (`{status}`) |
| GET/POST | `/v1/cardapio/restaurants/{id}/domains` · DELETE `/v1/cardapio/domains?host=` |
| GET | `/v1/cardapio/restaurants/{id}/events?page=` (lista crua; dashboard de analytics é fase futura) |
| POST | `/v1/cardapio/restaurants/{id}/media` (multipart → `{url}`) |

Regras multitenant idênticas à bio: `accountId` de query é filtro validado contra o Principal; fora do escopo → `404`; repo sempre filtra `account_id`.

### Permissões e role templates

- `cardapio.view` / `cardapio.manage` / `cardapio.orders.manage` (scope `account`).
- Templates: `cardapio.manager` (as 3), `cardapio.viewer` (view).

### Registro central (feito na INTEGRAÇÃO, não pelo subagente)

- `app.go`: `registry.MustRegister(cardapio.New())` + gating `{Prefix: "/v1/cardapio", ModuleID: "cardapio"}`.

## 5. Frontend — página `/cardapio`

### Wiring (os 4 lugares — feito na INTEGRAÇÃO, não pelo subagente)

1. `workspaces.ts`: `{ id: 'cardapio_web', label: 'Cardápio Online', icon: 'dashboard_customize', path: '/cardapio' }`.
2. `permissions.ts`: `WORKSPACE_ACCESS_DEFINITIONS` + `cardapio_web` nas `ROLE_WORKSPACES` (espelhar roles que têm `site_tracking_web`).
3. `nav.config.ts`: item top-level `{ id: 'cardapio', label: 'Cardápio Online', icon: 'boxes', path: '/cardapio', workspaceId: 'cardapio_web', moduleId: 'cardapio', hidden: true }` (hidden até validar).
4. `module-enabled.global.ts`: `{ prefix: '/cardapio', moduleId: 'cardapio' }`.

### Páginas e componentes (subagente D cria; tudo novo, zero arquivo compartilhado)

| Arquivo | Conteúdo |
|---|---|
| `pages/cardapio/index.vue` | Orquestra `CardapioListWorkspace` |
| `pages/cardapio/[id].vue` | Orquestra `CardapioEditorWorkspace` |
| `components/cardapio/CardapioListWorkspace.vue` | Tabela de restaurantes (nome, slug, cliente, domínio, ativo, atualizado) + busca + filtro por cliente (só admin, via `useTenantsStore`) + criar |
| `components/cardapio/CardapioCreateModal.vue` | Nome + slug (+ select de cliente, só admin) |
| `components/cardapio/CardapioEditorWorkspace.vue` | Shell: sidebar de seções + painel ativo + barra de status (ativo/inativo, link público se domínio configurado) |
| `components/cardapio/sections/CardapioSectionDados.vue` | Dados do restaurante: identidade, contato, endereço, horários, settings de entrega (centavos com máscara R$), tema |
| `components/cardapio/sections/CardapioSectionCategorias.vue` | CRUD categorias + reordenar (subir/descer) |
| `components/cardapio/sections/CardapioSectionProdutos.vue` | Lista de produtos por categoria + modal/painel de produto (campos do contrato + variações + adicionais + galeria com upload) |
| `components/cardapio/sections/CardapioSectionAvaliacoes.vue` | CRUD reviews por produto |
| `components/cardapio/sections/CardapioSectionPedidos.vue` | Lista de pedidos (filtro por status, paginação) + mudar status (select imediato + toast) |
| `components/cardapio/sections/CardapioSectionDominios.vue` | CRUD domínios custom + explicação do subdomínio por convenção |
| `components/cardapio/AGENT.md` | Doc da área |
| `stores/cardapio.ts` | Pinia: lista, restaurante ativo, catálogo, pedidos |
| `composables/useCardapioEditor.ts` | Estado/salvamento por seção, dirty-check, upload |
| `domain/cardapio/types.ts` | Port EXATO do contrato (Restaurant, Category, Product, Variation, Addon, Review, Order, OrderItem — camelCase, centavos) |

Sub-componentes de produto (variações/adicionais/galeria) podem virar arquivos próprios para respeitar 450 linhas.

### Regras de UI

Mesmas da bio: raiz com scroll (`flex:1; min-height:0; overflow-y:auto`), tokens do design system, BEM, sem emoji, feedback imediato (spinner/toast), estados vazios orientativos, máscara de moeda exibindo R$ mas persistindo centavos inteiros.

## 6. Notas de Deploy

1. Migration `0153_cardapio_schema.sql` (local: conferir `:5433`).
2. **Rebuild api**: `docker compose up -d --build api`.
3. Envs novas em `.env.production` **E** `docker-compose.prod.yml`:
   - `PUBLIC_API_BASE_URL` (compartilhada com bio — imagens absolutas).
   - `CARDAPIO_BASE_DOMAIN` (ex.: `tavola.app`) — resolve por subdomínio; vazio desliga a convenção.
   - `CARDAPIO_DEV_DEFAULT_SLUG` (opcional, só dev/local).
4. Front cardápio (repo separado, host do cliente): `NUXT_PUBLIC_API_BASE=https://<api-do-painel>` (ele chama `/v1/public/*`).
5. Validar preflight real: `curl -X OPTIONS -H "Origin: https://qualquer.com" https://<api>/v1/public/resolve` → 204 com `Access-Control-Allow-Origin: *`.

## 7. Fases (espelhadas no roadmap-data.ts, grupo `cardapio`)

- **C1 — Banco + módulo Go `cardapio`** (subagente C): migration 0153 + módulo completo + CORS público no middleware + testes + AGENT.md. **Sem `app.go`.**
- **C2 — Painel `/cardapio`** (subagente D, em paralelo): types + store + composable + páginas + componentes + AGENT.md. **Sem wiring compartilhado.**
- **C3 — Integração e validação** (agente principal + usuário): wiring central (junto com o da bio), migration, rebuild, habilitar módulo na account Crow, seed do primeiro restaurante pelo painel, e2e com o front cardápio (resolve → cardápio → prato → pedido → evento), preflight CORS, tirar `hidden`, sync 3 docs + panorama.

## 8. Specs dos subagentes Opus

Regras comuns (idênticas às do plano bio §8): ler `AGENT_RULES.md` + `docs/ENGINEERING_PRINCIPLES.md` + este plano; **nenhum comando git**; não aplicar migration/rebuild/portas; máx 450 linhas/arquivo; sem emoji; lint zero; atualizar AGENT.md.

**Regra de paralelismo (4 agentes simultâneos):** PROIBIDO tocar em `app.go`, `nav.config.ts`, `workspaces.ts`, `permissions.ts`, `module-enabled.global.ts` — o wiring central é da integração. Exceção única: subagente C pode alterar `httpapi/middleware.go` (CORS público) porque nenhum outro agente toca nesse arquivo.

**Subagente C — back+banco (C1).** Entregáveis: §3 (migration 0153), §4 (módulo completo + CORS no middleware + testes de recálculo/resolve/allowlist). Validação: `go build ./... && go vet ./...` + `golangci-lint run` = 0 issues. Padrões: IDs `string`, scan nullable `*string`, SQL schema-qualificado e parametrizado, 404 uniforme, centavos `int64`, camelCase exato do contrato no JSON.

**Subagente D — front painel (C2).** Entregáveis: §5 (sem os 4 arquivos de wiring). Contrato da API do painel = §4. Validação: `eslint` 0 erros + `vue-tsc --noEmit` limpo (exceto ruído repo-wide pré-existente). NÃO rodar `npm install` no host.

Independência: C (back/) × D (web/) × A (bio back/) × B (bio web/) — nenhum arquivo em comum entre os 4.

## 9. Fora do escopo do MVP (fases futuras)

- Dashboard de analytics dos eventos (hoje: lista crua paginada).
- Notificação realtime de pedido novo no painel (WebSocket) — hoje lista com refresh.
- Cupons (`coupon_viewed/coupon_used` já entram na allowlist de eventos; modelagem de cupom fica para depois).
- Reservas (eventos `reservation_*` aceitos; fluxo de reserva no painel depois).
- Clientes (`customer_id` reservado em orders; cadastro de cliente final depois).
- Renomear o módulo (label "Cardápio Online" é provisório; id `cardapio` interno se mantém).
