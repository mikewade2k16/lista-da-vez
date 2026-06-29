# AGENT — platform/httpapi

## Escopo

Pacote `back/internal/platform/httpapi/`. Middlewares HTTP compartilhados
entre todos os módulos: chain, SecurityHeaders, CORS, RequestID, RateLimit,
Logging, Gzip, `AccountModulesGuard` (multi-tenant), helpers de erro padronizados.

## Peças

- `chain.go` — `Chain(handler, middlewares...)` para empilhar middlewares (índice 0 = mais externo).
- `security_headers.go` — `SecurityHeaders(enableHSTS)` (P1.10): X-Content-Type-Options, X-Frame-Options, Referrer-Policy, COOP e CSP (`default-src 'none'; frame-ancestors 'none'`) em toda resposta; HSTS só em produção. Mais externo no Chain.
- `compress.go` — `Gzip()` (P1.16): comprime respostas quando o cliente aceita gzip; pula WebSocket (Hijack), uploads/binários (Content-Type não-comprimível) e respostas sem corpo (204/304/1xx). Preserva Flush/Hijack. Mais interno (após Logging, para o status logado ser o real).
- `cors.go` — `CORS(allowedOrigins)`.
- `request_id.go` — `RequestID` injeta X-Request-Id e logging structure key.
- `rate_limit.go` — `RateLimit(opts)`: token bucket por identidade (preferida
  `Principal.UserID`, fallback IP). Resposta 429 com `Retry-After`. Coberto
  por testes em `rate_limit_test.go`.
- `account_guard.go` — `AccountModulesGuard`: middleware multi-tenant que
  lê `core.account_modules` e bloqueia rotas de módulos desabilitados.
- `rls.go` — `RLSConnGuard` (RLS fase 1, SEC-1, `docs/RLS_PLAN.md`): injeta uma
  conexão por request com o GUC de tenant setado para o Row-Level Security do
  Postgres. `Wrap(handler)` faz `pool.Acquire`, `set_config('app.account_id', ...)`
  (e `app.bypass_rls=on` p/ platform_admin), põe a conn no context
  (`database.WithConn`) e no `defer` faz `reset all` + `Release` SEMPRE (senão
  vaza conexão do pool). O escopo (`RLSScope`) é extraído do `Principal` por um
  resolver definido no `app.go` (httpapi não importa `auth` — ciclo). Aplicado
  DENTRO do `RequireAuth` e SO no grupo de rotas que migrou (fase 1: `/v1/feedback`),
  nunca global. Resolver nil / escopo vazio sem bypass => segue no pool (fail-safe).
- `error.go` / `response.go` — `WriteError`, `WriteJSON` padronizados.

## `AccountModulesGuard` — ATIVO desde 2026-06-04 (C20)

> O guard estava codificado mas **descartado** (`_ = httpapi.NewAccountModulesGuard(pool)`)
> até 2026-06-04. A multitenant-completion C20 ativou de verdade.

[../app/app.go](../app/app.go) agora:

1. Instancia **uma** `modulesGuard := httpapi.NewAccountModulesGuard(pool)` e a
   passa via `Dependencies.ModulesGuard` (parar de descartar).
2. Aplica `modulesGuard.RequireModuleByPath(moduleGatingRules())` como middleware
   no `Chain` (último/mais interno, para o Logging capturar o 403).
3. Assina `account.modules.changed` no bus → `modulesGuard.Invalidate(accountID)`.
4. Chama `authMiddleware.SetAccountChecker(...)` (habilita `RequireAuthWithAccount`).

### `RequireModuleByPath` (gate-list por prefixo)

Em vez de embrulhar cada handler dos ~9 módulos legados, um único middleware
mapeia prefixo de path → módulo (espelha o `MODULE_PATH_GUARDS` do front em
`web/app/middleware/module-enabled.global.ts`):

```go
queue:  /v1/operations, /v1/alerts, /v1/reports, /v1/analytics,
        /v1/feedback, /v1/consultants, /v1/settings, /v1/stores
crm:    /v1/erp, /v1/catalog
tasks:  /v1/tasks, /v1/task-boards
```

Rotas não listadas (auth, me, **admin**, users, notifications, access, tenants,
webhooks, realtime, bi, roadmap, uploads, healthz) **passam direto** (fail-open
p/ não listadas; fail-closed só nas gateadas). As rotas `/v1/admin/*` (gestão
platform_admin) **não** são gateadas — inclui os endpoints admin do módulo
`site`, que não está na seed 0124.

Regras (em `account_guard.go`):

1. Lê `X-Account-Id` do header. Ausente em rota gateada → `400 missing_account_id`.
2. Consulta `core.account_modules WHERE account_id = $1::uuid AND enabled = true`.
3. Cache em memória (60s TTL) com `Invalidate(accountID)` e `InvalidateAll()`.
4. Módulo habilitado → prossegue; senão → `403 module_disabled`.
5. Schema `core` inexistente → "nenhum módulo habilitado" (fail-closed).

**Bypass platform_admin (`SetBypass`):** o guard roda no Chain ANTES do `RequireAuth`
por rota (principal ainda não está no contexto), então o app.go injeta um predicado
via `modulesGuard.SetBypass(fn)` que autentica o token e libera quando o papel é
`platform_admin`. Motivo: o admin gerencia TODAS as accounts e não está preso aos
módulos de uma account ativa — sem isto, se a account ativa dele estiver sem o
módulo, todas as rotas de uso dão 403 e o front entra em loop de redirect (painel
em branco). Espelha a isenção equivalente no `module-enabled.global.ts` do front.

**Pré-requisito:** seed `0124_core_account_modules_seed.sql` aplicada (queue/tasks/crm
para todas as accounts ativas) — e `queue.New()`/`crm.New()` registrados no Registry
para que `core.modules` contenha as entradas. Sem isso, o gating fail-close quebra
o produto.

## Invalidação reativa do cache

`PUT /v1/admin/accounts/:id/modules` → `AdminService` publica `account.modules.changed`
→ o handler em `app.go` chama `modulesGuard.Invalidate(accountID)`, descartando o
cache na hora (403 sem esperar o TTL de 60s). O `AdminService` também recebe o
mesmo guard via `Dependencies.ModulesGuard` (Invalidate redundante, defensivo).

## Padrões de erro

- `WriteError(w, r, statusCode, code, message)`:
  - `code` é uma chave estável (`module_disabled`, `missing_account_id`,
    `invalid_account_id`, `internal_error`, etc.).
  - `message` é texto humano em pt-BR.
  - Cross-account (recurso de outra account) → **404** (nunca 403). Reservar
    403 para "perm faltando na mesma account".

## Rate limit

Padrão atual: 60 req/min por identidade + **300 req/min por tenant** (P1·11).
Configurável via env (`HTTPRateLimitRequests`/`HTTPRateLimitWindow`). Aplicado
**antes** do logging para que 429 também apareça nos logs.

`RateLimitOptions.AccountResolver` extrai `X-Account-Id` do header e aplica uma
segunda cota de `AccountLimit` (padrão `Limit×5`) por account. Impede
noisy-neighbor: um tenant com muitos usuários não degrada vizinhos. As duas cotas
(user + account) compartilham a mesma `Window`; qualquer estouro retorna 429.
Testes: `rate_limit_test.go` — `TestRateLimit_AccountQuota*`.

## Quando atualizar este AGENT.md

- Adicionar/remover middleware.
- Mudar regras do `AccountModulesGuard` (incluindo: quando ele finalmente for
  ativado, atualizar a seção "estado real").
- Mudar política de erro padrão (4xx vs 5xx).

## Referências

- [platform/app/AGENT.md](../app/AGENT.md)
- [modules/core/AGENT.md](../../modules/core/AGENT.md)
- [docs/ROADMAP.md → "Estado real em 2026-05-28"](../../../../docs/ROADMAP.md)
- [docs/MULTITENANT_COMPLETION_PLAN.md](../../../../docs/MULTITENANT_COMPLETION_PLAN.md)
- [docs/CONTRACT_FREEZE.md](../../../../docs/CONTRACT_FREEZE.md)
