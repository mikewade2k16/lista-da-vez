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

## Estado do multi-tenant — atualizado 2026-06-04 (C20 — guard ATIVO)

### O que está ativo agora

- **`AccountModulesGuard` ATIVO** — instanciado uma vez como `modulesGuard`, passado via `Dependencies.ModulesGuard` (parou de ser descartado com `_ =`) E aplicado no `Chain` via `modulesGuard.RequireModuleByPath(moduleGatingRules())`. O gating é centralizado por prefixo de path (gate-list), não por wrapper em cada handler — ver `moduleGatingRules()` neste pacote e [httpapi/AGENT.md](../httpapi/AGENT.md).
- **Invalidação reativa** — `app.go` assina `account.modules.changed` → `modulesGuard.Invalidate(accountID)` (403 sem esperar o TTL de 60s).
- **`SetAccountChecker` chamado** — `authMiddleware.SetAccountChecker(auth.NewPostgresAccountMemberChecker(pool))` habilita `RequireAuthWithAccount` (membership em `core.account_users`).
- **`queue.New()` e `crm.New()` registrados no Registry** — sem isso `core.modules` não teria as entradas `queue`/`crm`, a seed 0124 viraria no-op e o guard fail-close quebraria queue/crm. Rotas continuam no wiring legado (`Build` retorna handle sem rotas).
- **`core.account_modules` seedado** — migrations 0124/0125 populam `queue`, `tasks`, `crm` para accounts ativas.

### Gating por path (espelha o front)

Gateados: `queue` (`/v1/operations,/v1/alerts,/v1/reports,/v1/analytics,/v1/feedback,/v1/consultants,/v1/settings,/v1/stores`), `crm` (`/v1/erp,/v1/catalog`), `tasks` (`/v1/tasks,/v1/task-boards`). Rotas não listadas (auth, me, **admin**, users, notifications, access, tenants, webhooks, realtime, bi, roadmap) passam direto. O front injeta `X-Account-Id = auth.activeTenantId` em todo request via `createApiRequest` (plugin `account-id-bridge.client.ts`).

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

## Referências

- [docs/ROADMAP.md → "Estado real em 2026-05-28"](../../../../docs/ROADMAP.md)
- [docs/MULTITENANT_COMPLETION_PLAN.md](../../../../docs/MULTITENANT_COMPLETION_PLAN.md)
- [docs/CONTRACT_FREEZE.md](../../../../docs/CONTRACT_FREEZE.md)
- [platform/httpapi/AGENT.md](../httpapi/AGENT.md)
- [platform/modules/AGENT.md](../modules/AGENT.md)
- [modules/core/AGENT.md](../../modules/core/AGENT.md)
