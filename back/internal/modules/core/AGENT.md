# AGENT — module `core`

## Escopo

Modulo `back/internal/modules/core/`. Plataforma multi-tenant nova:
identidade global, accounts (substitui `tenants`), organizations (agencia),
membership (`account_users`), modules habilitados por account, RBAC dinamico
(roles + permissions clonadas de templates).

Branch alvo: `refactor/multi-tenant-core`. Documento mestre:
`~/.claude/plans/preciso-que-analise-nosso-ancient-orbit.md`.

## Estado por fase

### Fase 1 (atual) — leituras basicas

- Tabelas core.* criadas via migration `0100_core_schema.sql`.
- Seed inicial de `tenants → accounts` e `users → users` em `0101_core_seed_from_legacy.sql`.
- Endpoints `/v2/me/accounts` (lean) e `/v2/me/context?accountId=...` (full)
  expostos APENAS quando `CORE_V2_ENABLED=true`.
- `roles` e `permissions` ainda retornam `[]` (Fase 3).

### Fase 2 (proxima) — Module Registry

- `core.modules`, `core.permissions`, `core.role_templates` populados pelo
  `SyncCatalog` no boot a partir do Module Registry.
- `core.account_modules` recebe registro para todos os accounts existentes
  (todos os modulos atuais habilitados — nao quebra nada).

### Fase 3 — RBAC dinamico

- `core.roles` por account: clones de `core.role_templates`.
- `core.role_permissions` populadas (validadas contra catalogo).
- `core.user_role_assignments` migrada a partir de `public.user_*_roles`.
- Service `MeContext` passa a popular `Roles[]` e `Permissions[]` reais.

## Endpoints expostos (gated por `CORE_V2_ENABLED`)

| Verbo | Path | Resposta | Status |
|---|---|---|---|
| GET | `/v2/me/accounts` | `MeAccountsResponse` (accounts lean) | implementado |
| GET | `/v2/me/context?accountId=<id>` | `MeContextResponse` (full) | implementado, roles/permissions vazios ate Fase 3 |
| POST | `/v2/me/active-account` | — | nao implementado (frontend gerencia cookie ate Fase 5) |

`/v2/me/context` valida que o user e membership ativo da account (defesa
em profundidade contra spoofing de `accountId` na query). Resposta
`account_not_found` cobre tanto "nao existe" quanto "nao e membership"
para nao vazar existencia.

## Regras inegociaveis (vide `docs/CONTRACT_FREEZE.md`)

- `account_id` SO vem do middleware (a partir do Principal) para handlers
  legados v1. Em endpoints v2 expostos aqui, `accountId` chega na query
  porque a especificacao ainda nao implementou o middleware `X-Account-Id`
  (chega na Fase 2/3). Validacao de membership e feita no service.
- Repository nunca recebe `account_id` direto vindo do request body — sempre
  passa pelo service que valida membership primeiro.
- Nao introduzir FK de `core.*` para schemas satelites (`queue.*`, `finance.*`).
  Se precisar de dado, abstrair via interface in-process.

## Arquivos

- `model.go` — structs (Account, Organization, User), DTOs (Summary, Context),
  interface `Repository`.
- `errors.go` — erros padronizados (`ErrUserNotFound`, `ErrAccountNotMember`, ...).
- `store_postgres.go` — `PostgresRepository` implementando `Repository`.
- `service.go` — orquestra leituras, valida membership.
- `http.go` — handlers, registro de rotas. Chamado por `app.go` apenas se
  `cfg.CoreV2Enabled` e true.

## Como testar manualmente

```bash
# Subir backend com flag ligada:
CORE_V2_ENABLED=true go run ./cmd/api

# Aplicar migrations (incluindo 0100/0101):
go run ./cmd/migrate up

# Healthz mostra a flag:
curl http://localhost:8080/healthz

# Fazer login (JWT v1 atual):
curl -X POST http://localhost:8080/v1/auth/login -d '{"email":"...","password":"..."}'

# Listar accounts do user logado:
curl http://localhost:8080/v2/me/accounts -H "Cookie: AUTH_TOKEN=..."

# Contexto de uma account especifica:
curl "http://localhost:8080/v2/me/context?accountId=<uuid>" -H "Cookie: AUTH_TOKEN=..."
```

## Quando atualizar este AGENT.md

- Sempre que adicionar/remover endpoint v2.
- Quando uma das fases do roadmap mudar de status (atualizar tabela "Estado por fase").
- Quando o contrato de algum DTO publico mudar.
