# AGENT — module `core`

## Escopo

Modulo `back/internal/modules/core/`. Plataforma multi-tenant nova:
identidade global, accounts (substitui `tenants`), organizations (agencia),
membership (`account_users`), modules habilitados por account, RBAC dinamico
(roles + permissions clonadas de templates).

Branch alvo: `refactor/multi-tenant-core`. Documento mestre:
`~/.claude/plans/preciso-que-analise-nosso-ancient-orbit.md`.

## Estado por fase

### Fase 1 — leituras basicas (concluida)

- Tabelas core.* criadas via migration `0100_core_schema.sql`.
- Seed inicial de `tenants → accounts` e `users → users` em `0101_core_seed_from_legacy.sql`.
- Endpoints `/v2/me/accounts` (lean) e `/v2/me/context?accountId=...` (full)
  expostos APENAS quando `CORE_V2_ENABLED=true`.
- `roles` e `permissions` ainda retornam `[]` (Fase 3).

### Fase 2 (atual) — Module Registry

- `module.go` adapta o core para a interface `modules.Module`. Agora o core
  passa pelo Registry no boot (em vez de wiring direto via `core.RegisterRoutes`).
- 8 permissoes declaradas (`core.account.view/manage`, `core.users.view/manage`,
  `core.roles.view/manage`, `core.modules.manage`, `core.organization.consolidated_read`).
- 3 role templates: `core.owner` (acesso total, locked nas accounts), `core.admin`
  (gerencia usuarios e cargos), `core.member` (membership basica).
- `SyncCatalog` no boot popula `core.modules`, `core.permissions`,
  `core.role_templates` e `core.role_template_permissions` declarativamente.
- Endpoints `/v2/me/accounts` e `/v2/me/context` continuam servidos pelo handle
  retornado de `Module.Build()` — mesmas rotas, mesmo shape.

### Fase 3 — RBAC dinamico (em andamento)

#### Item 1 — CloneTemplateToAccount (concluido)

- Migration `0102_rbac_locked_templates.sql` adiciona `is_locked` em `core.role_templates`.
- `RoleTemplateDef.IsLocked` propagado de `platform/modules/module.go` ate `catalog_postgres.go`.
- Template `core.owner` declarado com `IsLocked: true` — roles clonados dele nao podem ser deletados.
- `rbac_model.go` — structs `RoleTemplate` e `Role` com `ToSummary()`.
- `rbac_repository.go` — `RBACRepository` + `PostgresRBACRepository`:
  `ListTemplatesForModules`, `ListTemplatePermissionKeys`, `CloneTemplate`, `SetRolePermissions`.
- `rbac_service.go` — `RBACService.InitAccountRoles(ctx, accountID, moduleIDs)`: seed idempotente
  de roles para a account. Chamado ao criar account ou habilitar modulo novo.
- `module.go` — `Build()` cria `PostgresRBACRepository` e `RBACService`; exposto em `handle.rbacService`.

#### Items pendentes

- **Item 2**: `EnsureModuleRoles` — ao habilitar modulo em account existente, seed sem resetar customizacoes.
- **Item 3**: Endpoints `POST /v1/accounts`, `GET/POST/PATCH/DELETE /v1/accounts/:id/roles`, `AssignRoleToUser`.
- **Item 4**: Migration de dados — `user_tenant_roles` + `user_store_roles` → `core.user_role_assignments`.
- **Item 5**: `MeContext` popula `Roles[]` e `Permissions[]` reais a partir de `core.role_permissions`.

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
- `errors.go` — erros padronizados: identidade, account, RBAC.
- `store_postgres.go` — `PostgresRepository` implementando `Repository`.
- `service.go` — orquestra leituras, valida membership.
- `http.go` — handlers, registro de rotas.
- `rbac_model.go` — structs `RoleTemplate` e `Role`.
- `rbac_repository.go` — `RBACRepository` + `PostgresRBACRepository`.
- `rbac_service.go` — `RBACService` (seed de roles, futuramente CRUD e resolucao).

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
