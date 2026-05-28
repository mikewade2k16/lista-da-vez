# AGENT — platform/httpapi

## Escopo

Pacote `back/internal/platform/httpapi/`. Middlewares HTTP compartilhados
entre todos os módulos: chain, CORS, RequestID, RateLimit, Logging,
`AccountModulesGuard` (multi-tenant), helpers de erro padronizados.

## Peças

- `chain.go` — `Chain(handler, middlewares...)` para empilhar middlewares.
- `cors.go` — `CORS(allowedOrigins)`.
- `request_id.go` — `RequestID` injeta X-Request-Id e logging structure key.
- `rate_limit.go` — `RateLimit(opts)`: token bucket por identidade (preferida
  `Principal.UserID`, fallback IP). Resposta 429 com `Retry-After`. Coberto
  por testes em `rate_limit_test.go`.
- `account_guard.go` — `AccountModulesGuard`: middleware multi-tenant que
  lê `core.account_modules` e bloqueia rotas de módulos desabilitados.
- `error.go` / `response.go` — `WriteError`, `WriteJSON` padronizados.

## `AccountModulesGuard` — estado real em 2026-05-28

> **O guard está codificado mas DESCARTADO em runtime.**

[../app/app.go:313](../app/app.go#L313) faz:

```go
_ = httpapi.NewAccountModulesGuard(pool)
```

Nenhuma chamada a `guard.RequireModule(...)` em qualquer rota do binário.
Consequência: a checagem multi-tenant existe só no papel — habilitar ou
desabilitar módulo no banco não tem efeito até essa pendência fechar na
[multitenant-completion](../../../../docs/MULTITENANT_COMPLETION_PLAN.md).

### Contrato esperado quando ele for ativado

```go
guard := httpapi.NewAccountModulesGuard(pool)

mux.Handle("GET /v1/finance/invoices", guard.RequireModule("finance")(handler))
mux.Handle("GET /v1/omni/conversations", guard.RequireModule("omni")(handler))
// ... etc para cada módulo satélite
```

Regras inegociáveis (já implementadas em `account_guard.go`):

1. Lê `X-Account-Id` do header. Ausente em rota gateada → `400 missing_account_id`.
2. Consulta `core.account_modules WHERE account_id = $1 AND enabled = true`.
3. Cache em memória (60s TTL) com `Invalidate(accountID)` e `InvalidateAll()`.
4. Quando o módulo está habilitado, prossegue. Quando não, retorna
   `403 module_disabled`.
5. Se `core` ou schema novo ainda não existir, interpreta como "nenhum módulo
   habilitado" (fail-closed, não fail-open).

### Quando o guard fica realmente ligado

Plano em [docs/MULTITENANT_COMPLETION_PLAN.md → C2 (mt-c2-guard-wire)](../../../../docs/MULTITENANT_COMPLETION_PLAN.md).
Pré-requisitos: migration `0124_core_account_modules_seed.sql` rodar para
popular pelo menos `queue` e `crm` para Pérola/Duby. Sem seed, ativar o
guard quebra o produto.

## Invalidação reativa do cache

Quando `account.modules.changed` é publicado no event bus (após
`PUT /v1/admin/accounts/:id/modules`), o handler do evento **deve** chamar
`guard.Invalidate(accountID)` para descartar o cache daquele account. Sem
isso, a UI mostra o módulo habilitado mas o backend continua bloqueando até
60s.

Esse wiring de evento → invalidate ainda não existe. Entra como parte da
C2/C3 da multitenant-completion.

## Padrões de erro

- `WriteError(w, r, statusCode, code, message)`:
  - `code` é uma chave estável (`module_disabled`, `missing_account_id`,
    `invalid_account_id`, `internal_error`, etc.).
  - `message` é texto humano em pt-BR.
  - Cross-account (recurso de outra account) → **404** (nunca 403). Reservar
    403 para "perm faltando na mesma account".

## Rate limit

Padrão atual: 60 req/min por identidade. Configurável via env. Aplicado
**antes** do logging para que 429 também apareça nos logs.

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
