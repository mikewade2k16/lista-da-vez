# AGENT - `back/internal/modules/site`

## Escopo

Modulo de leads, produtos e tracking do site publico do tenant.

Decisao C17 (2026-05-29): leads/produtos deixaram o BFF mock e viraram dados reais via API Go + PostgreSQL.
Decisao C18 (2026-06-01): tracking do painel Perola entra como webhook assinado e persiste eventos brutos para dashboards.

Estado real (P0.5, 2026-06-07): as rotas foram REGISTRADAS no boot — `site.RegisterAdminRoutes` (`/v1/admin/leads|products|tracking-events|tracking-analytics|webhook-sources`) + `site.RegisterIngestRoutes` (`/v1/webhooks/leads|products/{slug}`) no `app.go`. Antes existiam no codigo mas NAO eram montadas, e o front recebia 404. As rotas `/v1/admin/*` nao passam pelo gating de modulo (gestao platform_admin).

## Banco

Schema proprio: `site`.

Migrations:

- `0128_site_schema.sql`: `site.webhook_sources`, `site.leads`, `site.products`.
- `0129_site_tracking_events.sql`: amplia `webhook_sources.entity_type` para `tracking` e cria `site.tracking_events`.

Tabelas:

| Tabela | Conteudo |
|---|---|
| `site.webhook_sources` | fontes externas com `slug`, `entity_type` (`leads`, `products`, `tracking`) e `secret` HMAC em claro |
| `site.leads` | leads captados manualmente ou por webhook |
| `site.products` | catalogo do site, separado do raw ERP |
| `site.tracking_events` | eventos brutos de analytics: batch, visitor/session, pagina, elemento, device, UTM, payload raw |

O `secret` fica em claro porque HMAC usa o secret como chave. Nunca logar, nunca retornar em listagem; revelar apenas na criacao/rotate.

## Endpoints Admin

Todos exigem JWT + account context.

| Verbo | Path |
|---|---|
| GET/POST | `/v1/admin/leads` |
| GET/PATCH/DELETE | `/v1/admin/leads/{id}` |
| GET/POST | `/v1/admin/products` |
| GET/PATCH/DELETE | `/v1/admin/products/{id}` |
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

## Permissoes

- `site.leads.view`
- `site.leads.manage`
- `site.products.view`
- `site.products.manage`
- `site.tracking.view`
- `site.webhooks.manage`

## Arquivos

- `model.go`: DTOs de leads/produtos/sources e interfaces base.
- `tracking_model.go`: tipos e interface do tracking.
- `leads_repository.go`: CRUD `site.leads`.
- `products_repository.go`: CRUD `site.products`.
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
