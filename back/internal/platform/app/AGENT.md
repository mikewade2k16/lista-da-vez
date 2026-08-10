# AGENT — platform/app

## Escopo

Pacote `back/internal/platform/app/`. Bootstrap do binário: cria pool, monta
config, instancia módulos (legados + Module Registry quando
`CORE_V2_ENABLED=true`), pluga middlewares globais (CORS, RequestID, RateLimit,
Logging) e retorna o handler HTTP final.

## Peças

- `app.go` — `BuildHandler(...)`: ponto de entrada. Cria stores (`tenants`,
  `users`, `stores`, `consultants`, `settings`, `auth`, etc.), services, HTTP
  handlers; quando `CORE_V2_ENABLED`, instancia `Registry` e roda
  `SyncCatalog` + `Build`.
- `context_http.go` — `/v1/me/context` legado.
- `*_adapter.go` — adapters finos entre módulos legados (ex:
  `operations_store_scope_adapter.go`).
- `GET /healthz` (em `app.go`) — desde AC-16 faz `pool.Ping` (timeout 2s) e devolve
  `200 + db:"ok"` ou `503 + db:"unreachable"` (erro só no `slog.Warn healthz_db_ping_failed`,
  nunca no corpo — endpoint público). Consumidores do 200: healthcheck do compose, smoke do
  deploy, UptimeRobot e `scripts/monitoring/check-vps.sh`.

## Estado do multi-tenant — atualizado 2026-06-04 (C20 — guard ATIVO)

### O que está ativo agora

- **`AccountModulesGuard` ATIVO** — instanciado uma vez como `modulesGuard`, passado via `Dependencies.ModulesGuard` (parou de ser descartado com `_ =`) E aplicado no `Chain` via `modulesGuard.RequireModuleByPath(moduleGatingRules())`. O gating é centralizado por prefixo de path (gate-list), não por wrapper em cada handler — ver `moduleGatingRules()` neste pacote e [httpapi/AGENT.md](../httpapi/AGENT.md).
- **Invalidação reativa** — `app.go` assina `account.modules.changed` → `modulesGuard.Invalidate(accountID)` (403 sem esperar o TTL de 60s).
- **`SetAccountChecker` chamado** — `authMiddleware.SetAccountChecker(auth.NewPostgresAccountMemberChecker(pool))` habilita `RequireAuthWithAccount` (membership em `core.account_users`).
- **`queue.New()` e `crm.New()` registrados no Registry** — sem isso `core.modules` não teria as entradas `queue`/`crm`, a seed 0124 viraria no-op e o guard fail-close quebraria queue/crm. Rotas continuam no wiring legado (`Build` retorna handle sem rotas).
- **`core.account_modules` seedado** — migrations 0124/0125 populam `queue`, `tasks`, `crm` para accounts ativas.

### Gating por path (espelha o front)

O prefixo `/v1/performance-feedback` tambem pertence ao modulo `queue` e segue o
mesmo gate por account das rotas de analytics e operacao.

Gateados: `queue` (`/v1/operations,/v1/alerts,/v1/reports,/v1/analytics,/v1/feedback,/v1/consultants,/v1/settings,/v1/stores`), `crm` (`/v1/erp,/v1/catalog`), `tasks` (`/v1/tasks,/v1/task-boards`) e os módulos satélites declarados em `moduleGatingRules()`, incluindo `social_publishing` (`/v1/social-publishing`). Rotas não listadas (auth, me, **admin**, users, notifications, access, tenants, webhooks, realtime, bi, roadmap) passam direto. O front injeta `X-Account-Id = auth.activeTenantId` em todo request via `createApiRequest` (plugin `account-id-bridge.client.ts`).

### O que ainda está pendente

- `CORE_V2_ENABLED=false`: Registry não roda; `modulesGuard` fica nil; o gating é pulado (modo legado sem multi-tenant) e endpoints `/v2/me/*` não existem.
- `RequireModule("site")` nas rotas admin do site segue deferido (site não está na seed 0124; rotas `/v1/admin/*` não são gateadas).

## Regras ao mexer aqui

- Não introduzir feature flag nova sem registrar em [config/AGENT.md](../config/AGENT.md).
- Rota nova de módulo que deve ser gateada por contratação: adicionar o prefixo em `moduleGatingRules()` (mapa prefixo → moduleId). O front precisa enviar `X-Account-Id` (já automático via `createApiRequest`).
- Rotas de gestão platform_admin (`/v1/admin/*`) NÃO entram no gate-list — são gestão, não uso por tenant.
- Não importar nada de `web/server/` no Go (BFF Nitro marcado para remoção em C7).

## Comportamento sem feature-flag

`CORE_V2_ENABLED=false` (default): Registry e guard não existem. Apenas wiring legado.

`CORE_V2_ENABLED=true`: Registry roda, SyncCatalog popula catálogos, guard ativo, `RequireAuthWithAccount` disponível.

## PrincipalCache wiring (AC-01)

- `principal_cache_wiring.go` — `wirePrincipalCache(cfg, logger, authService, accessService, usersService)` cria o `httpapi.PrincipalCache[auth.Principal]` quando `AUTH_PRINCIPAL_CACHE_TTL > 0` (default `30s`; `0s` retorna `nil` e loga `principal_cache_disabled`). Liga os setters de `auth`/`access`/`users` e dispara a goroutine de manutencao (`Cleanup()` a cada 60s + log `principal_cache_stats` a cada 5 min quando houve trafego).
- `app.go` (2 pontos): chama `wirePrincipalCache` logo apos criar `usersService`; injeta o cache retornado em `modules.Dependencies{PrincipalCache: principalCache}` (nil-safe — `core` faz nil-check no Build). Invalidacao e direta/sincrona, NAO via bus. Fora do bloco `CoreV2Enabled` o cache ja funciona para auth/access/users; os setters do `core` so ligam quando o Registry roda.

## Referências

- [docs/ROADMAP.md → "Estado real em 2026-05-28"](../../../../docs/ROADMAP.md)
- [docs/MULTITENANT_COMPLETION_PLAN.md](../../../../docs/MULTITENANT_COMPLETION_PLAN.md)
- [docs/CONTRACT_FREEZE.md](../../../../docs/CONTRACT_FREEZE.md)
- [platform/httpapi/AGENT.md](../httpapi/AGENT.md)
- [platform/modules/AGENT.md](../modules/AGENT.md)
- [modules/core/AGENT.md](../../modules/core/AGENT.md)

## Wiring das fontes de Customer Intelligence (2026-07-23)

- `app.go` registra os adapters owner-owned de Calendar, ERP, Site e BI no
  `customerintelligence.Service`.
- ERP e Site passam primeiro por Customer Data e so consultam o owner para
  `source_link` exato. Calendar produz Business Context sem escopo pessoal. BI
  faz apenas health/validacao local e nao dispara chamada externa.
- Os adapters vivem neste pacote para preservar a direcao de dependencias:
  modulos nao importam Customer Intelligence nem um ao outro.
- Erro permanente de contrato/configuracao e classificado por reason code e nao
  entra em retry; indisponibilidade de qualquer fonte permanece isolada do
  runtime de chat.
