# AGENT - `back/internal/modules/site`

## Escopo

Modulo de leads, produtos e tracking do site publico do tenant.

Decisao C17 (2026-05-29): leads/produtos deixaram o BFF mock e viraram dados reais via API Go + PostgreSQL.
Decisao C18 (2026-06-01): tracking do painel Perola entra como webhook assinado e persiste eventos brutos para dashboards.
Decisao B8 (2026-06-13): produtos podem ser SINCRONIZADOS (pull) da API publica do site do cliente (primeiro cliente = Perola: `https://perolajoias.com/api/products/`). Cada account tem zero/mais fontes em `site.product_sources`; o upsert grava em `site.products` por `(account_id, source, external_id)`. Arquitetura plugavel (GET publico, sem credencial).

> CAVEAT PRODUTOS (2026-06-14): o back de produtos (editor, cache de imagens,
> cruzamento ERP) esta validado POR API, mas a UI de edicao/sincronizacao em
> `/site/produtos` esta **PENDENTE DE REVISAO/TESTE NO NAVEGADOR** — o usuario
> reportou que o sync pela tela NAO funcionou. Nao marcar a tela de produtos como
> pronta antes de revisar/testar no browser.

Estado real (P0.5, 2026-06-07): as rotas foram REGISTRADAS no boot — `site.RegisterAdminRoutes` (`/v1/admin/leads|products|tracking-events|tracking-analytics|webhook-sources`) + `site.RegisterIngestRoutes` (`/v1/webhooks/leads|products/{slug}`) no `app.go`. Antes existiam no codigo mas NAO eram montadas, e o front recebia 404. As rotas `/v1/admin/*` nao passam pelo gating de modulo (gestao platform_admin).

## Banco

Schema proprio: `site`.

Migrations:

- `0128_site_schema.sql`: `site.webhook_sources`, `site.leads`, `site.products`.
- `0129_site_tracking_events.sql`: amplia `webhook_sources.entity_type` para `tracking` e cria `site.tracking_events`.
- `0154_site_product_sources.sql` (B8): cria `site.product_sources`; adiciona `site.products.external_id` e `site.products.source` (default `''`); cria o indice unico parcial `(account_id, source, external_id)` (`where source <> '' and external_id <> ''`) usado pelo upsert do sync; registra a fonte da Perola via SELECT em `core.accounts` (idempotente, `on conflict do nothing`) somente se a account `perola` existir.

Tabelas:

| Tabela | Conteudo |
|---|---|
| `site.webhook_sources` | fontes externas com `slug`, `entity_type` (`leads`, `products`, `tracking`) e `secret` HMAC em claro |
| `site.leads` | leads captados manualmente ou por webhook |
| `site.products` | catalogo do site, separado do raw ERP. `external_id`/`source` marcam a origem do sync |
| `site.product_sources` | config de fonte externa de produtos por account (`type`, `base_url`, `enabled`) para o pull/sync |
| `site.tracking_events` | eventos brutos de analytics: batch, visitor/session, pagina, elemento, device, UTM, payload raw |

O `secret` fica em claro porque HMAC usa o secret como chave. Nunca logar, nunca retornar em listagem; revelar apenas na criacao/rotate.

## Endpoints Admin

Todos exigem JWT + account context.

| Verbo | Path |
|---|---|
| GET/POST | `/v1/admin/leads` |
| GET/PATCH/DELETE | `/v1/admin/leads/{id}` |
| GET/POST | `/v1/admin/products` | GET: `?q&status&category&campaign&page&perPage` -> `{products,total,page,perPage}`. Ordem `created_at desc, id desc` (mais novos primeiro). PATCH aceita arrays `categories`/`campaigns`, `status`, `stock`, etc. |
| POST | `/v1/admin/products/sync` |
| GET/PATCH/DELETE | `/v1/admin/products/{id}` |
| POST | `/v1/admin/products/{id}/image` | multipart `file` -> salva em `/uploads/site/products/{account}/up-{rand}.{ext}` (valida por MAGIC BYTES, nao confia no content-type do cliente) e seta `products.image`. Devolve o `ProductView`. Cap 15 MB (`MaxBytesReader`). Reusa `ImageCache.SaveUpload`/`isImageBytes` do `image_cache.go` |
| POST | `/v1/admin/products/erp-match` | **(ERP)** recalcula os links produto<->ERP da account -> `{matched, products}` |
| GET | `/v1/admin/products/erp-unmatched` | **(ERP)** `?page&perPage&q` -> `{items:[{sku,name,description}], total, page, perPage}`: itens do `erp_item_current` (tenant=account) sem produto no site |
| POST | `/v1/admin/products/from-erp` | **(ERP)** body `{sku}` -> cria `site.products` (name/description do ERP, code=sku, source='erp') + linka; 201 `ProductView`. Sku inexistente -> 404 |
| GET/PATCH | `/v1/admin/products/source` | **(toggle fonte)** GET -> `{mode:'local'|'online'|'custom', baseUrl}` (derivado do `base_url` da fonte `external_api` da account); PATCH `{mode:'local'|'online'}` seta o `base_url` p/ a URL conhecida (local XAMPP `host.docker.internal/painel-perola` / online `perolajoias.com`). So muda a fonte do PROXIMO sync; a bio reflete apos re-sincronizar. Sem fonte na account -> 404 |
| GET | `/v1/admin/tracking-events` |
| GET | `/v1/admin/tracking-analytics` |
| GET/POST | `/v1/admin/webhook-sources` |
| POST | `/v1/admin/webhook-sources/{id}/rotate` |
| DELETE | `/v1/admin/webhook-sources/{id}` |

## Endpoints Ingest

Publicos, sem JWT, autenticados por HMAC do `secret` da source.

| Verbo | Path | Contrato |
|---|---|---|
| POST | `/v1/webhooks/leads/{sourceSlug}` | `X-Signature: sha256=<hex(HMAC_SHA256(rawBody, secret))>` |
| POST | `/v1/webhooks/products/{sourceSlug}` | `X-Signature: sha256=<hex(HMAC_SHA256(rawBody, secret))>` |
| POST | `/v1/webhooks/tracking/{sourceSlug}` | `X-Omni-Timestamp` + `X-Omni-Signature: sha256=<hex(HMAC_SHA256(timestamp + "." + rawBody, secret))>` |

Tracking tambem exige `X-Omni-Source`, payload `event_key = "site_tracking"` e `events[]`. O timestamp tem janela de 5 minutos. O insert e idempotente por `source_id + source_event_id`.

Resposta de tracking:

```json
{
  "ok": true,
  "batchId": "...",
  "received": 25,
  "inserted": 25,
  "skipped": 0
}
```

## Sync de produtos (B8)

`POST /v1/admin/products/sync?accountId=` (admin) puxa os produtos das fontes
externas habilitadas da account e faz UPSERT em `site.products`. Resposta:

```json
{ "inserted": 12, "updated": 3, "skipped": 1 }
```

- O `accountId` da query e validado contra o principal: `platform_admin` pode
  sincronizar qualquer account; demais papeis so a propria (account fora do
  escopo retorna `404 not_found`, nao 403). Sem `accountId`, usa o contexto.
- Cada produto e mapeado e gravado por `(account_id, source='perola', external_id=id_origem)`:
  insere novos, atualiza mudados, marca inativo (`is_active=false`, `status='inactive'`)
  os que vierem com `deleted_at`. Upsert em lote (sem N+1) via `unnest` + `ON CONFLICT`.
- Mapeamento da `image` (`perolaImageURL`): http(s) passa direto; com `/` so
  prefixa o host; SO o nome do arquivo (caso da Perola, ex.: `368252.avif`) ->
  monta `https://perolajoias.com/assets/images/products/{segmento}/{arquivo}`,
  onde `{segmento}` = 1a campanha (ou 1a categoria como fallback; sem ambas, sem
  segmento). Heuristica fragil (campanha-vs-categoria e ambiguo) — o ideal e a
  API da Perola devolver a URL pronta; pendencia anotada em `painel-perola/AGENT.md`.
- **Candidatos de imagem (`perolaImageCandidates`)**: o `Image` acima e so o 1o
  palpite. `mapPerolaProduct` tambem preenche `ImageCandidates` — lista ORDENADA
  de URLs que o cache tenta no sync (a 1a que responder 200 vence). Espelha o
  `buildImageFileVariants`/`slug_us` do painel-perola (crow-notion, que funciona):
  segmentos = cada campanha, cada categoria e por fim **`default`** (cobre o caso
  sem campanha/categoria — o bug antigo montava `/products/0278091.webp` sem pasta
  e dava 404); arquivos = nome original + `_sm.avif`, `.avif`, `_sm.<ext>`,
  `.jpg`/`.JPG`. Para http/path resolvido, candidata unica. **Por que no servidor
  e nao no front**: o crow-notion faz isso com `onerror` no browser; aqui isso
  reintroduziria o hotlink que a Perola bloqueia (ver Cache abaixo).
  WAF da Perola bloqueia o UA padrao do Go (406) -> `fetchPage` manda
  `User-Agent: OmniSync/1.0`. `status` `active/desactive` -> `active/inactive`;
  `categories`/`campaigns`
  chegam como TEXTO JSON-array e sao parseados; `price` numerico (numeric 14,2 em
  reais, nao centavos — o schema de `site.products` usa `price`, nao `priceCents`).
- Sem fonte habilitada -> `404 no_product_source`. Service sem `WithProductSync`
  -> `503 product_sync_unavailable`.
- **Cache de imagens (`image_cache.go`)**: no sync, ANTES do upsert, as imagens
  externas sao baixadas UMA vez (UA `OmniSync`, concorrencia 5, teto 150s) e
  gravadas em `UPLOADS_DIR/site/products/{account}/{sha1(url)}.{ext}`; o
  `item.Image` passa a apontar pro path local `/uploads/site/products/...`. Cada
  item tenta sua lista `ImageCandidates` (via `candidateURLs`) em ordem e fica com
  a 1a que baixar 200. Falha de UMA candidata (404/timeout) -> tenta a proxima.
  Falha de TODAS: se todas eram do host da Perola OU de `host.docker.internal`
  (`allPerolaHost` — o browser nao alcanca nenhum), **zera** `item.Image` (o front
  mostra "sem img" — evita martelar uma URL que da `ERR_CONNECTION_TIMED_OUT`); de
  outras origens, MANTEM a externa como fallback. Nunca quebra o sync. Idempotente
  (arquivo ja existe -> nao rebaixa). **Valida que o corpo e imagem de verdade**
  (`looksLikeImage`: Content-Type image/* OU magic bytes png/jpeg/gif/webp/avif) —
  sem isso, um 200 com HTML de erro (ex.: `.htaccess` da Perola cai no index.php)
  seria cacheado como "imagem" (bug real: 418 arquivos de 563 bytes com "Fatal
  error" PHP). Fonte da URL: `ImageCandidates`/`Image` do item; com a API local
  mandando `image_path` (caminho real), o `mapPerolaProductAt` usa esse caminho
  direto (candidata unica certeira) em vez da heuristica de pasta — 797/825 locais.
  `ProductSyncResult` ganhou `imagesCached`. **Motivo**: hotlink direto da origem martelava o CDN/Cloudflare
  do cliente (a Perola bloqueou nosso IP por excesso de requests do browser) e
  dependia da origem estar no ar a cada view. Com o cache, bio e admin servem do
  nosso `/uploads` (rapido, sem hotlink). O publico da bio absolutiza `/uploads/`
  via `PUBLIC_API_BASE_URL`; o admin de produtos absolutiza no front (apiBase).
  Wiring em `app.go`: `.WithImageCache(site.NewImageCache(cfg.UploadsDir))`.
- **Listagem do admin carrega tudo**: `products_repository.List` cap de `perPage`
  subiu p/ 5000; o front (`useProductsManager`) pede `perPage=5000` p/ carregar o
  catalogo inteiro (filtros/dropdowns de categoria-campanha sao client-side e
  precisam ver todos os produtos — antes so via os da pagina 1).
- **Cruzamento com o ERP (`product_erp_repository.go`, migration `0155`)**: tabela
  `site.product_erp_links (account_id, product_id, erp_sku, erp_name, erp_description,
  matched_at; unique(product_id, erp_sku))` guarda o RESULTADO do cruzamento. O
  `MatchERP(accountID)` faz `unnest(string_to_array(p.code,'_'))` (codigo multi
  separado por `_`) e junta com `erp_item_current e on e.sku=seg and e.tenant_id =
  p.account_id::uuid` (o tenant do ERP == o account_id do site), `on conflict do
  update` + delete de orfaos. O `List`/`Find` fazem LEFT JOIN LATERAL e expoem
  `erpSynced`/`erpName`/`erpDescription` no `ProductView` (info ADICIONAL do ERP,
  nao sobrescreve o nome/descricao proprios). `erp-unmatched` = itens do ERP sem
  produto no site; `from-erp` cria o produto a partir do item ERP (e ai some do
  unmatched). Front: aba "Produtos do ERP (fora do site)" + botao "Cruzar com ERP"
  + tag "ERP" na linha + "Puxar pro site".
- **Editor de produtos (/site/produtos, front)**: a pagina e de EDICAO, nao so
  visualizacao. Reusa `OmniDataTable` (layer tasks) com colunas editaveis: imagem
  (upload via `POST .../{id}/image`), `status` como switch "Visivel no site",
  switch "Tem estoque" (mapeado p/ `stock` 0/1), `categories`/`campaigns` como
  multiselect creatable (opcoes dos facets `/v1/bio/sources/site_products/facets`),
  preco/fator/tipo/estoque. Dois modos: "Carregar tudo" (perPage 5000, filtro
  client-side) e "Paginado" (`page`/`perPage`, filtros server-side `q`/`status`/
  `category`/`campaign`). Componentes: `SiteProductsAdminWorkspace.vue` +
  `useProductsManager.ts` + `useSiteProductColumns.ts` + `SiteProductsTableFooter.vue`
  + `SiteProductInfoCard.vue`. **Filtro `campaign`** adicionado no `List`
  (`campaigns @> [valor]`). `ImageCache.SaveUpload` (em `image_cache.go`) grava o
  upload manual validando por magic bytes (`isImageBytes`).
- **Toggle fonte LOCAL <-> ONLINE** (quando o site online da Perola cai): o host
  das imagens segue o `base_url` da fonte via `perolaSiteRoot` (strip do `/api/...`).
  - Online: `base_url = https://perolajoias.com/api/products/` -> imagens em
    `https://perolajoias.com/assets/...`.
  - Local (XAMPP em `C:\xampp\htdocs\painel-perola`): `base_url =
    http://host.docker.internal/painel-perola/api/products/` -> imagens em
    `http://host.docker.internal/painel-perola/assets/...`. O container alcanca o
    host por `host.docker.internal`. O `fetchPage` forca `Host: localhost` quando
    o host e `host.docker.internal` porque o `index.php` da Perola so usa o banco
    OFFLINE (`conect-offline.php`, db `perola_on`) quando `HTTP_HOST` contem
    "localhost". As imagens sao estaticas (Apache serve em qualquer vhost).
  - Trocar = `UPDATE site.product_sources SET base_url='...' WHERE id=...`. O cache
    zera a imagem (front mostra "sem img") quando TODAS as candidatas sao do host
    da Perola OU de `host.docker.internal` (o browser nao alcanca nenhum) —
    `allPerolaHost`.

## Permissoes

- `site.leads.view`
- `site.leads.manage`
- `site.products.view`
- `site.products.manage`
- `site.tracking.view`
- `site.webhooks.manage`

## Arquivos

- `model.go`: DTOs de leads/produtos/sources e interfaces base (inclui `ProductSource`, `ProductUpsertItem` — com `ImageCandidates` —, `ProductSyncResult`, `ProductSourceRepository`).
- `tracking_model.go`: tipos e interface do tracking.
- `leads_repository.go`: CRUD `site.leads`.
- `products_repository.go`: CRUD `site.products`.
- `product_sources_repository.go`: `site.product_sources` (`ListByAccount`) + UPSERT em lote em `site.products` (`unnest` + `ON CONFLICT (account_id, source, external_id)`; `RETURNING (xmax=0)` distingue insert de update).
- `perola_client.go`: cliente HTTP da fonte externa. `FetchAll` pagina `GET {base_url}?page=&limit=100` seguindo `meta.has_more` (teto de seguranca), parseia o envelope `{data, meta}` + os JSON-arrays texto, e mapeia para `ProductUpsertItem` (`http.NewRequestWithContext`, timeout 30s). `perolaImageCandidates`/`perolaSegments`/`perolaFileVariants`/`perolaSlug` montam a lista de URLs de imagem (segmento `default` + variantes `_sm.avif`/.avif/.jpg) que o cache tenta no sync.
- `perola_client_test.go`: parse do fixture Perola, mapeamento, `perolaImageURL` (path/http/so-nome com segmento campanha/categoria), `perolaImageCandidates` (default + variantes + slug), tolerancia de JSON-array, e sync idempotente (2x nao duplica) via fakes.
- `image_cache.go` / `image_cache_test.go`: cache de imagens do sync (tenta `ImageCandidates` em ordem, download 1x -> `/uploads/site/products/...`; falha total no host Perola zera a imagem, outras origens mantem externa; idempotente). Testes via `httptest` (download+reescrita, candidatos com 404->200, `allPerolaHost`, 404 mantem externa, sem rootDir = no-op, extensao).
- `webhooks_repository.go`: CRUD `site.webhook_sources` e lookup por slug.
- `tracking_repository.go`: listagem admin + insert de lote em `site.tracking_events` com `ON CONFLICT DO NOTHING`.
- `tracking_analytics_model.go`: DTOs do dashboard (`TrackingAnalyticsView`, totais, conversoes, etc.) e `Analytics` na interface `TrackingRepository`.
- `tracking_analytics_repository.go`: agregacoes para o dashboard (totais, dispositivos, eventos por tipo, conversoes, acessos/dia via `generate_series`, top referrers, ultimas visitas). Tudo escopado por `account_id` + janela `days`, usando os indices existentes.
- `tracking_analytics_service.go`: `GetTrackingAnalytics` aplica rotulos amigaveis a eventos conhecidos (whatsapp/maps_click/cookie_accept) com fallback humanize + percentual sobre visitantes.
- `service.go`: service principal e sources.
- `tracking_service.go`: normalizacao do payload flat/nested do tracker.
- `http_admin.go`: rotas admin.
- `http_ingest.go`: rotas publicas de ingest.
- `http_tracking.go`: HMAC timestamped do tracking.
- `module.go`: registro via Module Registry.

## Drift Cross-Camada

- Leads/produtos usam `X-Signature = HMAC_SHA256(rawBody, secret)`.
- Tracking Perola usa `X-Omni-Signature = HMAC_SHA256(timestamp + "." + rawBody, secret)`.
- Se alterar rotas ou assinatura, atualizar tambem `painel-perola/docs/site-tracking-webhook.md` e `WebhookSourcesDrawer.vue`.

## Validacao Local

1. Subir `postgres + api`.
2. Em dev, a migration `0130_seed_dev_site_tracking_source.sql` cria a source
  `tracking` com slug `perola-site` no account demo usando o secret fixo
  `dev-perola-site-tracking-secret`.
3. Se o ambiente estiver sem seeds dev, criar manualmente a source `tracking`
  com slug `perola-site` e usar o mesmo secret acima.
4. O painel admin agora le os eventos por `GET /v1/admin/tracking-events` e a
  rota canonica do frontend fica em `/site/tracking`. A tela tem duas abas:
  **Resumo** (dashboard de `GET /v1/admin/tracking-analytics?days=N`, agregando
  totais, conversoes dinamicas, dispositivos, eventos por tipo, acessos/dia,
  top referrers e ultimas visitas) e **Eventos** (a grade crua original).
  Conversoes sao genericas: qualquer `event_name` de interacao vira card, com
  rotulo amigavel para os conhecidos (whatsapp/maps_click/cookie_accept).
5. Configurar o painel Perola:
   ```php
   'site_tracking' => [
     'endpoint' => 'http://localhost:8883/v1/webhooks/tracking/perola-site',
    'secret' => 'dev-perola-site-tracking-secret',
     'source' => 'perola_site',
     'auth_type' => 'hmac_sha256',
   ],
   ```
6. POST em `http://localhost/painel-perola/back/site-track.php`.
7. Esperado: Perola retorna `webhook_configured=true` e `webhook_queued=false`; Omni insere em `site.tracking_events`.
