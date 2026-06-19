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

### Fase 3 — RBAC dinamico (codigo entregue, runtime parcial)

> **Aviso 2026-05-28**: a auditoria descobriu que apesar de tudo abaixo estar
> implementado em codigo, `core.user_role_assignments` no banco local estava com
> 0 linhas — o seed efetivo nao rodou (a 0103 rodou antes do primeiro boot
> bem-sucedido do SyncCatalog, com catalogo vazio -> CROSS JOIN vazio -> no-op).
> Sem isso, `Principal.Permissions` em runtime cai no fallback legado
> (`access.Service.ResolveUserPermissions`).
>
> **Correcao (multitenant-completion C1, migration 0125)**: re-executa o backfill
> agora que o catalogo esta vivo. Diferenca vs 0103: clona templates de TODOS
> os modulos (nao so 'core'), entao cada account ja sai com a matriz completa
> de roles disponivel para o painel C10. Idempotente — re-executar e seguro.

- Migration `0102_rbac_locked_templates.sql` adiciona `is_locked` em `core.role_templates`.
- `RoleTemplateDef.IsLocked` propagado de `platform/modules/module.go` ate `catalog_postgres.go`.
- Template `core.owner` declarado com `IsLocked: true` — roles clonados dele nao podem ser deletados.
- `rbac_model.go` — structs `RoleTemplate` e `Role` com `ToSummary()`, erros RBAC em `errors.go`.
- `rbac_repository.go` — `RBACRepository` + `PostgresRBACRepository`:
  todos os metodos de seed, CRUD, atribuicao, resolucao de contexto e validacao de permissoes.
- `rbac_service.go` — `RBACService`: `InitAccountRoles`, `ListRoles`, `GetRole`, `CreateRole`,
  `UpdateRolePermissions`, `DeleteRole`, `AssignRoleToUser`, `RemoveRoleFromUser`,
  `ResolveUserContext` (retorna roles e permissions reais via UNION/EXCEPT de overrides).
- `rbac_http.go` — 7 endpoints RBAC registrados via `RegisterRBACRoutes`.
- Migration `0103_rbac_seed_roles_and_assignments.sql` — seed de roles para accounts ativas
  e migracao de `user_tenant_roles`+`user_store_roles` → `core.user_role_assignments`.
- `service.go` — `MeContext` popula `Roles[]` e `Permissions[]` reais quando `rbacService != nil`.
- `module.go` — `Build()` cria `PostgresRBACRepository`, `RBACService`, wira os dois services.

## Endpoints expostos (gated por `CORE_V2_ENABLED`)

### /me — contexto do usuario autenticado

| Verbo | Path | Resposta | Notas |
|---|---|---|---|
| GET | `/v2/me/accounts` | `MeAccountsResponse` (lean) | lista accounts do usuario. Cada `AccountSummary` expoe `isAgency` (bool) e `organizationName` (string, via `left join core.organizations`) alem dos campos base — consumidos pelo AccountSwitcher para (a) gatear o menu Manage admin-global so na conta-agencia e (b) agrupar clientes por organizacao. |
| GET | `/v2/me/context?accountId=<id>` | `MeContextResponse` (full) | contexto completo do account |

> `/v1/me/context` continua servido pelo legado em `platform/app/context_http.go` com shape `{user, principal, context: {tenants, stores}}` esperado pelo frontend. Quando o frontend migrar para o shape v2 (com `account`/`roles`/`permissions`), o alias v1 pode voltar aqui. Os aliases v1 do core foram removidos em 2026-05-29 (post-C9) por causarem conflito de rota + mismatch de shape.

### /v1/admin/accounts — CRUD admin de accounts (C3, todos exigem platform_admin)

| Verbo | Path | Resposta | Notas |
|---|---|---|---|
| GET | `/v1/admin/accounts` | `AdminListAccountsResponse` | filtros: q, status, organizationId, page, perPage. Resposta inclui userCount/userNicks/projectCount/projectSegments/modules/stores agregados (C9, 2026-05-29). **EXCLUI contas `is_agency=true`** (a conta-agência não é cliente — Trilho 2, migration 0158). `count(*)` e dados usam o mesmo `where a.is_agency = false`. |
| POST | `/v1/admin/accounts` | `AccountAdminView` | cria account + clona roles + membership do adminEmail + **seed dos modulos default (queue/tasks/crm, igual 0124)** — sem isto a conta nasce com `account_modules` vazio e o guard barra todas as rotas |
| GET | `/v1/admin/accounts/{id}` | `AccountAdminView` | detalhe completo com billing/contact + agregados. **NÃO** aplica o filtro `is_agency` — a conta-agência continua acessível no detalhe (só some da lista). |
| PATCH | `/v1/admin/accounts/{id}` | `AccountAdminView` | patch semantico (campos nil ignorados). Aceita `active` (toggle status) desde C9 |
| DELETE | `/v1/admin/accounts/{id}` | 204 | soft delete (is_active=false) |
| GET | `/v1/admin/accounts/{id}/modules` | `AdminModulesResponse` | lista todos os modulos com enabled/disabled |
| PUT | `/v1/admin/accounts/{id}/modules` | `AdminModulesResponse` | habilita/desabilita; invalida cache do guard; publica account.modules.changed |
| GET | `/v1/admin/accounts/{id}/stores` | `AdminStoresResponse` | lojas da account com billing_amount por loja |
| PUT | `/v1/admin/accounts/{id}/stores` | `AdminStoresResponse` | atualiza billing_amount por loja (modo per_store) |
| POST | `/v1/admin/accounts/{id}/webhook/rotate` | `AdminWebhookRotateResponse` | gera novo webhook_key (64 hex chars) |

### /v1/admin/users — CRUD admin de users (C14, todos exigem platform_admin)

| Verbo | Path | Resposta | Notas |
|---|---|---|---|
| GET | `/v1/admin/users` | `AdminUserListResponse` | filtros: q (email/nome/nick), status (active/inactive), platformAdmin (true/false), page, perPage. Resposta inclui `accountCount` e `accountNames` (nomes das accounts, não slugs — renomeado em 2026-05-29) via `core.account_users` JOIN `core.accounts`. **Paginacao server-side** (default perPage=20, cap 100); o front (`/manage/users`) consome UMA pagina por vez com os filtros aplicados no backend — nao baixa mais todas as paginas para filtrar no cliente (Track D perf, 2026-06-15). Param opcional `includeAccounts` (default `true`): `includeAccounts=false` devolve a **projecao lean** (sem o LATERAL join de contas, `accountCount=0`/`accountNames=""`) para callers que so precisam acima-da-dobra — o contrato default permanece inalterado. |
| POST | `/v1/admin/users` | `AdminUserView` | cria user. Senha opcional — se vazia, `must_change_password=true` (precisa convite). `accountId`+`role` matricula no cliente; `organizationId`+`orgRole` (`agency_owner`/`agency_member`, default member) matricula na agencia **e** na conta-agencia (is_agency) com papel `owner`/`director` — necessario para o user de agencia LOGAR (sem papel resolvido o login retorna 403 `user_no_role`). Enroll reaproveitado por `enrollUserInAccount`. |
| GET | `/v1/admin/users/{id}` | `AdminUserView` | detalhe completo com agregados. |
| PATCH | `/v1/admin/users/{id}` | `AdminUserView` | patch semantico. Safeguard: nao permite rebaixar/desativar ultimo platform_admin ativo (`ErrLastPlatformAdmin`, HTTP 409). Aceita `password` (define/reseta senha): **ausente ou vazio = NAO toca no hash** (regra "nunca sobrescrever senha sem acao explicita"); senha nao-vazia (min 8) e hasheada pelo service e zera `must_change_password`. Senha nunca e logada. |
| DELETE | `/v1/admin/users/{id}` | 204 | soft delete (`is_active=false`). Mesmo safeguard do PATCH. |
| GET | `/v1/admin/users/{id}/memberships` | `AdminMembershipsResponse` | lista contas em que o user e membro: `accountId`, `slug`, `name`, `isActive`, `joinedAt`, **`role`** (papel coarse na conta: owner/director/marketing/...) e **`isAgency`** (conta-agencia vs cliente). |
| PATCH | `/v1/admin/users/{id}/memberships/{accountId}` | `AdminMembershipsResponse` | troca o nivel/papel do user naquela conta (RBAC Fase 1). Aceita `role` em {owner, director, marketing} (tenant-scoped, sem loja); invalido -> 400 `invalid_role`; nao-membro -> 404. Faz replace dos `user_role_assignments` via `SetUserAccountRole`. |

### /v1/admin/organizations — CRUD admin de organizations (C15, todos exigem platform_admin)

| Verbo | Path | Resposta | Notas |
|---|---|---|---|
| GET | `/v1/admin/organizations` | `AdminOrganizationListResponse` | filtros: q (nome/slug), status, page, perPage. Resposta inclui `accountCount` + `accountNames` (nomes das accounts, não slugs — renomeado em 2026-05-29) via `core.accounts WHERE organization_id`. |
| POST | `/v1/admin/organizations` | `OrganizationAdminView` | cria org. Slug ≥2 chars, lowercase forçado. |
| GET | `/v1/admin/organizations/{id}` | `OrganizationAdminView` | detalhe com agregados. |
| PATCH | `/v1/admin/organizations/{id}` | `OrganizationAdminView` | patch semantico (`slug`, `name`, `isActive` opcionais). |
| DELETE | `/v1/admin/organizations/{id}` | 204 | soft delete (`is_active=false`). Accounts vinculadas continuam funcionando — só perdem o agrupamento. |

`PATCH /v1/admin/accounts/{id}` agora aceita `organizationId` no body: string vazia (`""`) → desvincula (NULL); UUID válido → vincula. C15.

### /v1/platform/menu-layout — config GLOBAL do menu (platform-level)

Config de **NÍVEL PLATAFORMA** (NÃO per-account, NÃO per-user): organiza o menu
(quais itens vão no header vs sidebar). Persistida em `core.platform_settings`
sob a chave singleton `menu_layout`. GET é para **todos** os usuários autenticados;
PATCH exige **platform_admin**.

| Verbo | Path | Auth | Resposta |
|---|---|---|---|
| GET | `/v1/platform/menu-layout` | `RequireAuth` (todos) | `MenuLayoutResponse` |
| PATCH | `/v1/platform/menu-layout` | `RequireAuth` + `requirePlatformAdmin` | `MenuLayoutResponse` |

- Body do PATCH: `{ "layout": <Layout> }`.
- Resposta (GET e PATCH): `{ "layout": <Layout>, "updatedAt": <RFC3339|null>, "updatedBy": <userId|null> }`.
- `<Layout>` = `{ "version": int, "sections": [ {"id": string, "order": int} ], "items": { "<navItemId>": {"placement": string, "order": int} } }`.
- `placement` ∈ `{header, sidebar, both, hidden}`. Placement inválido → **400** `validation_error` (validado no service).
- Quando a linha `menu_layout` ainda não existe, o GET devolve o default vazio
  `{ "layout": {"version":1,"sections":[],"items":{}}, "updatedAt": null, "updatedBy": null }` (sem erro).

#### Tabela `core.platform_settings` (migration 0160)

Key-value **singleton por chave** de configuração global da plataforma
(`key text primary key`, `config jsonb`, `updated_at timestamptz`, `updated_by uuid
references core.users(id)` nullable). **Exceção consciente** à regra "toda tabela
tem account_id": é deliberadamente platform-global (config única para a plataforma
inteira, igual ao catálogo de módulos). A primeira chave é `menu_layout`.

### Conta-agência: `is_agency` (Trilho 2, migration 0158)

`core.accounts.is_agency` (boolean not null default false) marca a conta-WORKSPACE
da agência ("Crow Visuals", slug 'crow') — dona do board geral de Tasks, com TODOS
os módulos habilitados. Ela **NÃO é cliente**, então `ListAccounts` (camada admin)
filtra `where a.is_agency = false` (no `count(*)` e na query de dados). O campo é
exposto como `isAgency` em `AccountAdminView` e é scaneado junto com as demais colunas
de `core.accounts` nos 3 SELECTs que usam `scanAdminAccount` (List/Find/Update returning).

> **Importante**: o FILTRO `is_agency = false` (esconder a conta-agência da lista de
> clientes) é EXCLUSIVO da camada admin (`admin_repository.go`). `store_postgres.go`
> (`ListAccountsForUser`/`scanAccount` do switcher) **NÃO** filtra `is_agency` — o admin
> precisa enxergar a conta-agência no switcher como conta-casa. Não propagar este filtro
> para a visibilidade org-aware.
>
> **Switcher consome `isAgency`/`organizationName` (2026-06-15)**: as queries
> `listAccountsForUserQuery` e `findAccountIfAccessibleQuery` agora SELECIONAM
> `a.is_agency` e `coalesce(o.name, '')` (via `left join core.organizations o on o.id =
> a.organization_id`, 1:1, não multiplica linhas), e `scanAccount` os mapeia para
> `Account.IsAgency` / `Account.OrganizationName`. `Account.Summary()` os propaga para
> `AccountSummary.IsAgency` (json `isAgency`) e `AccountSummary.OrganizationName`
> (json `organizationName`), expostos em `GET /v2/me/accounts`. Isto é SELECT para
> exibição, distinto do FILTRO admin acima — a visibilidade org-aware permanece inalterada.

`/v2/me/context` valida que o user e membership ativo da account (defesa em profundidade
contra spoofing de `accountId`). Resposta `account_not_found` cobre tanto "nao existe" quanto
"nao e membership" para nao vazar existencia.

### Visibilidade org-aware de accounts (Trilho B — Etapa 3, AGENCY_TENANT_ARCHITECTURE)

`ListAccountsForUser` e `FindAccountIfMember` (`store_postgres.go`) decidem o escopo
de accounts 100% no banco, via a clausula `accountVisibilityWhere` (parametrizada por
`$1` = userID). Uma account ativa e visivel quando QUALQUER um dos tres caminhos vale:

1. **platform_admin** — `core.users.is_platform_admin = true` (user ativo) → ve TODAS
   as accounts ativas da plataforma.
2. **agency_owner** — existe linha em `core.organization_users` com
   `org_role = 'agency_owner'` cujo `organization_id = a.organization_id` → o dono da
   agencia ve TODAS as accounts da SUA organization.
3. **membership** — existe membership ativa em `core.account_users`
   (`is_active = true`) → comportamento legado, inalterado, para os demais users.

Os tres ramos sao `exists(...)` unidos por `OR` num unico predicado (sem JOIN que
multiplique linhas), entao cada account aparece no maximo uma vez — `DISTINCT`
desnecessario. `FindAccountIfMember` aplica a MESMA regra filtrando por `$2` (accountID);
se nada bate, `pgx.ErrNoRows` → `ErrAccountNotMember` (nao distingue "nao existe" de
"nao acessivel", para nao vazar existencia). A traducao de erro fica isolada em
`accountFromAccessibleRow` (testavel sem Postgres).

Defesa em profundidade: o client nunca decide escopo — a regra inteira vive no SQL.
Teste de contrato + traducao de erro em `store_postgres_test.go`.

> **N+1 AMPLIADO (atencao supervisor)**: `MeAccounts` chama `ListEnabledModuleIDs` por
> account num loop (N+1 ja conhecido — ENGINEERING_PRINCIPLES §10.3). Com platform_admin
> agora enxergando TODAS as accounts ativas, esse loop passa a rodar para a base inteira
> a cada `GET /v2/me/accounts` desse perfil. Nao foi corrigido aqui (fora do escopo).
> Candidato a agregacao batch: `WHERE account_id = ANY($1)` carregando os modulos de
> todas as accounts numa unica query (espelhar `loadModulesByAccount` de
> `admin_repository_aggregates.go`).

## Regras inegociaveis (vide `docs/CONTRACT_FREEZE.md`)

- `account_id` SO vem do middleware (a partir do Principal) para handlers
  legados v1. Em endpoints v2 expostos aqui, `accountId` chega na query
  porque a especificacao ainda nao implementou o middleware `X-Account-Id`
  (chega na multitenant-completion C2). Validacao de membership e feita no service.
- Repository nunca recebe `account_id` direto vindo do request body — sempre
  passa pelo service que valida membership primeiro.
- Nao introduzir FK de `core.*` para schemas satelites (`queue.*`, `finance.*`).
  Se precisar de dado, abstrair via interface in-process.
- `core.account_modules` e a fonte de verdade para "esse cliente contratou o
  modulo X?". Seed inicial via migration 0124 (multitenant-completion C1):
  popula `queue`, `tasks` e `crm` como habilitados para toda account ativa.
  Quando o `AccountModulesGuard` for ativado (multitenant-completion C2),
  nenhuma rota de modulo satelite pode ser acessada sem entry correspondente
  com `enabled=true`.

## Arquivos

- `model.go` — structs (Account, Organization, User), DTOs (Summary, Context), interface `Repository`.
- `errors.go` — erros padronizados: identidade, account, RBAC, admin.
- `store_postgres.go` — `PostgresRepository` implementando `Repository`. `ListAccountsForUser`/`FindAccountIfMember` usam a regra org-aware (`accountVisibilityWhere`): platform_admin vê todas, agency_owner vê as da org, demais via membership. As duas queries também selecionam `a.is_agency` + `coalesce(o.name,'')` (via `left join core.organizations`) e `scanAccount` os mapeia para `Account.IsAgency`/`Account.OrganizationName` (switcher). Ver "Visibilidade org-aware de accounts" acima.
- `service.go` — orquestra leituras de /me, valida acessibilidade (org-aware).
- `http.go` — handlers /v2/me/accounts e /v2/me/context. Aliases v1 removidos pós-C9 (conflito de rota com legacy + shape diferente).
- `rbac_model.go` — structs `RoleTemplate` e `Role`, `RoleSummary`.
- `rbac_repository.go` — `RBACRepository` + `PostgresRBACRepository`.
- `rbac_service.go` — `RBACService` completo.
- `rbac_http.go` — 7 endpoints RBAC (list/create/get/patch/delete roles + assign/remove).
- `admin_model.go` — DTOs admin (AccountAdminView com agregados, filtros, modules, stores, webhook) + interface `AdminRepository`.
- `admin_repository.go` — `PostgresAdminRepository`: CRUD de accounts (List/Find/Create/Update/SoftDelete). Update aceita `Active *bool`. List e Find chamam `enrichAccounts` para popular agregados. `ListAccounts` exclui contas `is_agency=true` (filtro base `a.is_agency = false`); `FindAdminAccount` não filtra. Os 3 SELECTs selecionam/scaneiam `is_agency` → `AccountAdminView.IsAgency` (json `isAgency`).
- `admin_repository_aggregates.go` — loaders batch (`loadUserAggregates`, `loadProjectAggregates`, `loadModulesByAccount`, `loadStoresByAccount`) + `enrichAccounts` que mescla tudo. Chamados por List e Find. Evita N+1 — uma query por agregado independente do número de accounts.
- `admin_repository_secondary.go` — métodos secundários: modules (`GetAccountModules`, `SetAccountModuleEnabled`), stores (`GetAccountStores`, `SetStoreBillingAmount`), webhook (`RotateWebhookKey`).
- `admin_users_model.go` — DTOs admin de users (`AdminUserView`, `AdminUserListFilter`, `AdminCreateUserInput`, `AdminUpdateUserInput`, `AccountMembershipView`) + interface `AdminUserRepository`.
- `admin_users_repository.go` — `PostgresAdminUserRepository`: List com `accountCount`/`accountNames` agregados via LATERAL join SO quando `filter.IncludeAccounts` (default); com `includeAccounts=false` o join e omitido (projecao lean). Find; Create (com hash de senha); Update; SoftDelete; GetMemberships; CountActivePlatformAdmins (safeguard). A paginacao (page/perPage, cap 100) e aplicada na query — a tela `/manage/users` busca por pagina, nao tudo de uma vez.
- `admin_users_service.go` — `AdminUserService`: valida unicidade de email (via constraint), hash de senha (`auth.BcryptHasher`) e safeguard do ultimo platform_admin antes de update/delete.
- `admin_users_http.go` — `RegisterAdminUsersRoutes`: 6 endpoints `/v1/admin/users*` (exigem platform_admin).
- `nick.go` — `BuildNickname(displayName, maxLength)`: helper que espelha 1-para-1 `web/app/domain/utils/person-display.ts > buildNickname` (primeiro nome + inicial do segundo + ponto). Usado por `AdminUserService.CreateUser` para auto-gerar quando vazio. Mudança aqui exige mudança paralela no front (e vice-versa) — drift entre camadas gera nicks diferentes.
- `admin_organizations_model.go` — DTOs admin de organizations (`OrganizationAdminView` com agregados accountCount/accountSlugs) + interface `AdminOrganizationRepository`.
- `admin_organizations_repository.go` — `PostgresAdminOrganizationRepository` (embeda `PostgresAdminRepository`): List com agregados via LATERAL join em `core.accounts`; Find; Create (com slug lowercased + uniqueness); Update; SoftDelete.
- `admin_organizations_service.go` — `AdminOrganizationService`: valida slug ≥2 chars + nome obrigatorio + lowercase.
- `admin_organizations_http.go` — `RegisterAdminOrganizationsRoutes`: 5 endpoints `/v1/admin/organizations*` (exigem platform_admin).
- `admin_service.go` — `AdminService`: regras de negocio, validacao, publicacao de eventos, invalidacao de cache.
- `admin_http.go` — `RegisterAdminRoutes`: 10 endpoints /v1/admin/accounts (todos exigem platform_admin).
- `platform_settings_model.go` — tipos da config GLOBAL do menu: `MenuLayout`, `MenuLayoutSection`, `MenuLayoutItem`, `MenuLayoutResponse` + set de placements válidos (`header/sidebar/both/hidden`) e `defaultMenuLayout()`.
- `platform_settings_repository.go` — `PostgresPlatformSettingsRepository` (mesmo `*pgxpool.Pool` dos demais repos core): `GetByKey` (linha ausente → nil/nil/nil sem erro) e `Upsert` (`insert ... on conflict (key) do update ... returning updated_at`). `updated_at`/`updated_by` scaneados como ponteiros (nullable).
- `platform_settings_service.go` — `PlatformSettingsService`: `GetMenuLayout` (default vazio quando não persistido) e `SaveMenuLayout` (valida placements → `ErrValidationFailed`, normaliza, marshal, upsert). Injeção via construtor.
- `platform_settings_http.go` — `RegisterPlatformSettingsRoutes`: `GET /v1/platform/menu-layout` (RequireAuth, todos) e `PATCH` (RequireAuth + `requirePlatformAdmin`). userID do autor via `auth.PrincipalFromContext`. Placement inválido → 400 `validation_error`.

### Origem dos agregados (C9)

| Campo | Fonte | Query |
|---|---|---|
| `userCount` | `core.account_users` | `count(distinct user_id) where is_active=true` |
| `userNicks` | `core.users` JOIN `core.account_users` | `string_agg(coalesce(nullif(nick,''), display_name), ', ')` |
| `projectCount` | `tasks.boards` | `count(*) where archived=false` |
| `projectSegments` | `tasks.boards` | `string_agg(name, ', ' order by name)` |
| `modules[]` | `core.modules` LEFT JOIN `core.account_modules` | uma row por módulo, enabled coalesce false |
| `stores[]` | `queue.stores` | todas as lojas onde `tenant_id = accountID` |

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
