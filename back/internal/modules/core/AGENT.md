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
| POST | `/v1/admin/accounts` | `AccountAdminView` | cria account + clona roles + **seed dos modulos default (queue/tasks/crm, igual 0124)** — sem isto a conta nasce com `account_modules` vazio e o guard barra todas as rotas. **`adminEmail` e OPCIONAL** (2026-06-25): vazio → cliente **sem dono/usuario** (controle interno, permitido por design); pula membership/role-assignment, mantem clone de roles + seed de modulos; dono pode ser anexado depois. Quando informado, deve existir em `core.users` (senao `ErrAdminUserNotFound`→422) e e vinculado como owner. |
| GET | `/v1/admin/accounts/{id}` | `AccountAdminView` | detalhe completo com billing/contact + agregados. **NÃO** aplica o filtro `is_agency` — a conta-agência continua acessível no detalhe (só some da lista). |
| PATCH | `/v1/admin/accounts/{id}` | `AccountAdminView` | patch semantico (campos nil ignorados). Aceita `active` (toggle status) desde C9 |
| DELETE | `/v1/admin/accounts/{id}` | 204 | soft delete (is_active=false) |
| GET | `/v1/admin/accounts/{id}/modules` | `AdminModulesResponse` | lista todos os modulos com enabled/disabled |
| PUT | `/v1/admin/accounts/{id}/modules` | `AdminModulesResponse` | habilita/desabilita; invalida cache do guard; publica account.modules.changed |
| GET | `/v1/admin/accounts/{id}/stores` | `AdminStoresResponse` | lojas da account com billing_amount por loja |
| PUT | `/v1/admin/accounts/{id}/stores` | `AdminStoresResponse` | atualiza billing_amount por loja (modo per_store) |
| POST | `/v1/admin/accounts/{id}/webhook/rotate` | `AdminWebhookRotateResponse` | gera novo webhook_key (64 hex chars) |

### Delegacao multi-tenant de /v1/admin/users (AdminScopeResolver)

A partir desta rodada, `/v1/admin/users*` NAO exige mais `platform_admin` para tudo.
A autorizacao e DELEGADA a org admin e admin de cliente, resolvida **no banco,
por-request, por-account** pelo `AdminScopeResolver` (`admin_scope.go` +
`admin_scope_repository.go`). NAO confiar em `Principal.Role`/`Permissions` — sao
resolvidos UMA vez no login a partir de UMA conta "home" (`auth/core_role_resolver.go`).

Dois niveis de autoridade que NAO se confundem:

- **account-scoped** (membership/papel/override de UMA account): basta o ator ser
  (a) `platform_admin`, OU (b) `agency_owner` da org dona da account, OU (c) ter
  `core.users.manage`/`core.roles.manage` **resolvido naquela account** (assignments +
  overrides allow EXCEPT deny — espelha `rbac_repository.go` `ListPermissionsForUser`).
  (c) **nao** e "ter membership" — um `core.member` tem membership mas nao administra.
- **identity-global** (`is_platform_admin`, email, senha, nome/nick, `is_active`,
  soft-delete, MOVE destrutivo, criar usuario): **SO platform_admin**. A delegacao da
  poder sobre o VINCULO, nunca sobre a identidade global.

Regra de resposta (anti-enumeration): recurso **fora do escopo → 404** (mesma
mensagem de "nao existe"); **403** SO para ator autenticado sem **nenhum** poder de
admin (gate barato `IsAdminOfAnything` no inicio de cada handler). `actorUserID`
SEMPRE do `Principal`, nunca do body/query. Defesa em profundidade: as queries de
listagem/escopo filtram no SQL mesmo com o service ja validando.

Metodos do resolver: `CanManageAccount`, `CanManageUser`, `CanManageOrganization`,
`IsPlatformAdmin`, `IsAdminOfAnything`.

### /v1/admin/users — CRUD admin de users (C14; delegado, ver secao acima)

| Verbo | Path | Resposta | Notas |
|---|---|---|---|
| GET | `/v1/admin/users` | `AdminUserListResponse` | filtros: q (email/nome/nick), status (active/inactive), platformAdmin (true/false), **`accountId`** (uuid), page, perPage. Resposta inclui `accountCount`, `accountNames` (nomes das accounts, não slugs — renomeado em 2026-05-29) via `core.account_users` JOIN `core.accounts`, **`clientAccountId`** (string) e **`isAgencyMember`** (bool). **Campo `isAgencyMember`**: `true` quando o usuario e membro ATIVO de pelo menos uma conta-agencia ATIVA (`core.accounts.is_agency=true`); o painel usa para sinalizar na grade que esse usuario ve todos os clientes/modulos da agencia (guard-rail contra "usuario de cliente virou membro de agencia sem querer"). **Filtro `accountId`**: quando informado, retorna só usuários que são membros ATIVOS daquela conta ATIVA (`exists` em `core.account_users` + `core.accounts` ativos; vazio/ausente = sem filtro). **Campo `clientAccountId`**: id do ÚNICO cliente ativo NÃO-agência (`core.accounts.is_agency=false`) do usuário, ou `""` quando ele tem 0 ou >1 clientes não-agência (subquery `having count(*)=1`). Serve para o front preselecionar a conta e decidir se a célula "mover cliente" é editável. **Paginacao server-side** (default perPage=20, cap 100); o front (`/manage/users`) consome UMA pagina por vez com os filtros aplicados no backend — nao baixa mais todas as paginas para filtrar no cliente (Track D perf, 2026-06-15). Param opcional `includeAccounts` (default `true`): `includeAccounts=false` devolve a **projecao lean** (sem o LATERAL join de contas, `accountCount=0`/`accountNames=""`) para callers que so precisam acima-da-dobra — `clientAccountId` continua sendo computado (independe do agregado). O contrato default permanece inalterado. |
| POST | `/v1/admin/users` | `AdminUserView` | cria user — **identity-global, SO platform_admin** (403 senao; admin de org/cliente vincula um usuario JA existente via POST memberships/organizations). Senha opcional — se vazia, `must_change_password=true` (precisa convite). `accountId`+`role` matricula no cliente; `organizationId`+`orgRole` (`agency_owner`/`agency_member`, default member) matricula na agencia **e** na conta-agencia (is_agency) com papel `owner`/`director` — necessario para o user de agencia LOGAR (sem papel resolvido o login retorna 403 `user_no_role`). Enroll reaproveitado por `enrollUserInAccount`/`linkUserToOrganization`. |
| GET | `/v1/admin/users` | (acima) | **Escopado ao ator** (`ActorUserID` do Principal, nunca da query): platform_admin ve todos; agency_owner ve usuarios das accounts da org; admin de cliente ve membros das accounts onde tem `core.users.manage`. `count(*)` e linhas usam o MESMO `where` (total correto, sem enumeration). `accountId` da query vira filtro DENTRO do permitido. |
| GET | `/v1/admin/users/{id}` | `AdminUserView` | detalhe completo com agregados. **Escopo `CanManageUser`** senao 404. |
| PATCH | `/v1/admin/users/{id}` | `AdminUserView` | patch semantico. **Todos os campos sao identity-global** (`displayName/nick/email/password/isPlatformAdmin/isActive`) → **SO platform_admin**. Ator nao-platform_admin que administre o alvo mas envie qualquer campo nao-nil → **403 `forbidden_field`** (apos checar escopo do alvo: fora de escopo → 404). Safeguard do ultimo platform_admin (`ErrLastPlatformAdmin`, 409). `password` ausente/vazio = NAO toca no hash; nao-vazio (min 8) hasheado e zera `must_change_password`. Senha nunca logada. |
| DELETE | `/v1/admin/users/{id}` | 204 | soft delete (`is_active=false`) — **identity-global, SO platform_admin** (403 senao). Mesmo safeguard do PATCH. Admin de org/cliente usa DELETE membership pontual. |
| GET | `/v1/admin/users/{id}/memberships` | `AdminMembershipsResponse` | lista contas em que o user e membro: `accountId`, `slug`, `name`, `isActive`, `joinedAt`, **`role`**, **`isAgency`**. **Escopo `CanManageUser`** (404 senao) + a resposta e **FILTRADA** as accounts administraveis pelo ator (senao agency_owner veria vinculos do user em clientes de outra agencia — vazamento). platform_admin ve tudo. |
| POST | `/v1/admin/users/{id}/memberships` | `AdminMembershipsResponse` (201) | **adiciona** vinculo de cliente SEM remover os outros. Body `{accountId, role}` (role default `owner`, em {owner,director,marketing}). Escopo `CanManageAccount(accountId)` senao 404. Account destino existe+ativa (senao 404) e NAO-agencia (senao 400 `account_is_agency`). Reusa `enrollUserInAccount` (upsert reativa/nao duplica). |
| PATCH | `/v1/admin/users/{id}/memberships/{accountId}` | `AdminMembershipsResponse` | troca o papel do user naquela conta. **Escopo `CanManageAccount`** (404 senao); **se a conta for `is_agency=true`, exige autoridade de organizacao (M2)**: platform_admin OU agency_owner daquela org — `core.users.manage` account-scoped NAO basta (404 senao, nao vaza). `role` em {owner, director, marketing}; invalido → 400 `invalid_role`; nao-membro → 404. Replace via `SetUserAccountRole`. |
| DELETE | `/v1/admin/users/{id}/memberships/{accountId}` | `AdminMembershipsResponse` | **desativa** o vinculo de cliente (transacional: `account_users.is_active=false` preservando `joined_at` + delete `user_role_assignments`). **Escopo `CanManageAccount`** (404 senao); **conta-agencia exige autoridade de organizacao (M2)**, igual ao PATCH acima. Convive com o PATCH no mesmo path (method-aware). |
| POST | `/v1/admin/users/{id}/organizations/{orgId}` | `AdminUserView` | **vincula agencia** a usuario existente. Retorna **`AdminUserView`** (mesmo shape do PATCH/PUT account — front aplica na linha via `applyPatch`). Body `{orgRole, confirmAgencyWideAccess}` (orgRole em {agency_owner,agency_member}, default member). **Escopo restrito: SO platform_admin OU agency_owner da PROPRIA org** (`CanManageOrganization`) — admin de cliente → 404. Virar membro de agencia da visao de TODOS os clientes da org → exige `confirmAgencyWideAccess:true` senao **422 `confirmation_required`**. Reusa `linkUserToOrganization` (DRY com `CreateUser`): cria `organization_users` + matricula na conta-agencia para o user logar. |
| DELETE | `/v1/admin/users/{id}/organizations/{orgId}` | `AdminUserView` | **desvincula agencia**. Retorna **`AdminUserView`** (mesmo shape do PATCH). Mesmo escopo do POST. Safeguard **409 `last_agency_owner`** (nao remove o ultimo agency_owner da org). Transacional: delete `organization_users` + desativa membership na conta-agencia da org. |
| PUT | `/v1/admin/users/{id}/account` | `AdminUserView` | **MOVE** (destrutivo) o usuário para a conta-cliente destino — **identity-global, SO platform_admin** (403 senao). Body `{ "accountId": "<destino>", "role": "owner" }` (role opcional, default `owner`; validado em {owner, director, marketing} -> 400 `invalid_role`). **Transacional**: (1) valida o destino existe + ativo (senão 404 `account_not_found`, não vaza existência) e NÃO é agência (senão 400 `account_is_agency` — endpoint só para cliente); (2) remove os `user_role_assignments` das contas-CLIENTE não-agência atuais; (3) desativa as memberships `account_users` não-agência atuais (mantém a linha p/ preservar `joined_at`); (4) **auto-enroll** no destino reusando `enrollUserInAccount` (membership ativa + papel + perms). **NÃO toca vínculos de agência** (`account_users`/`role_assignments` de contas `is_agency=true`). Retorna o `AdminUserView` atualizado (mesmo shape do PATCH) para o front atualizar a linha sem refetch. **O painel admin (`/manage/users`) tambem usa este endpoint para ATRIBUIR o primeiro cliente a um usuario sem vinculo** (0 memberships → o move apenas matricula); o `<select>` da coluna "Cliente" agora aparece tambem para usuario sem cliente (2026-06-25). |

### /v1/admin/users/{id}/accounts/{accountId}/overrides — overrides allow/deny por usuario por account

Opera em `core.user_permission_overrides` (NAO reusa o modulo `access`, que e
**LEGADO** — ver abaixo). Gate: `RequireAuth` + `requireAdminActor` (403 a quem nao
administra nada); escopo fino `CanManageAccount` no service (404 fora de escopo) +
alvo precisa ser membro da account (404 senao). `actorUserID` SEMPRE do Principal;
`account_id`/`user_id` SEMPRE do path (nunca do body).

| Verbo | Path | Resposta | Notas |
|---|---|---|---|
| GET | `/v1/admin/users/{id}/accounts/{accountId}/overrides` | `UserOverridesResponse` | `{ overrides: [{permissionKey, effect, note}], available: [{key, label, moduleId, scope}] }`. `available` = permissoes de modulos HABILITADOS na account, nao deprecated, `scope != 'platform'`. 404 fora de escopo / alvo nao-membro. |
| PUT | `/v1/admin/users/{id}/accounts/{accountId}/overrides` | `UserOverridesResponse` | body `{ overrides: [{permissionKey, effect, note}] }`. Valida: `effect in {allow,deny}` (senao **422 `invalid_effect`**); key no catalogo/modulo habilitado (via `InvalidPermissionKeys`) e `scope != platform` e sem duplicata (senao **422 `invalid_permission`**). Replace transacional `ReplaceUserOverrides`: desativa ativos + insere com `created_by_user_id=actorUserID`, respeitando o indice unico parcial `(account_id,user_id,permission_key) where is_active`. |

### /v1/accounts/{accountId}/roles* — papeis por cliente (RBAC, gate apertado)

O gate antigo `requireMember` (QUALQUER membro, ate `core.member`, fazia CRUD de
papeis — gap de seguranca) foi trocado por **`requireRolesManage`** (`rbac_http.go`):
leitura (GET) exige `core.roles.view` OU `core.roles.manage`; escrita exige
`core.roles.manage`; **platform_admin e agency_owner da org da account sempre passam**;
fora de escopo → **404** (`CanAccessAccountRoles` resolve 100% no banco, espelhando a
UNION/EXCEPT de overrides). `userID` SEMPRE do Principal.

> **Mudanca de contrato a comunicar**: quem tem `core.roles.manage` ganha acesso a
> `/v1/accounts/{id}/roles*` (alem de platform_admin/agency_owner) e **membros comuns
> PERDEM** o acesso (gap fechado).

**M1 — bloqueio de permissao de plataforma na matriz de papel**: `UpdateRolePermissions`
(`PATCH .../roles/{roleId}`) agora rejeita qualquer key com `scope='platform'` (via
`RBACRepository.PlatformScopedKeys`) antes de gravar → **422 `invalid_permission`**.
Espelha o bloqueio ja aplicado aos overrides por-usuario: um `core.roles.manage` de
cliente NAO consegue conceder `core.organization.consolidated_read` (ou futuras
platform-scoped) via papel custom.

| Verbo | Path | Resposta | Notas |
|---|---|---|---|
| GET | `/v1/accounts/{accountId}/members/{userId}/roles` | `200 { roles: RoleSummary[] }` | **NOVO**: papeis core.roles atribuidos AQUELE usuario NAQUELA account (`ListRolesForUser` + `ToSummary()`). Leitura → `requireRolesManage` em modo leitura (`core.roles.view` OU manage; platform_admin/agency_owner passam; fora de escopo → 404). Complemento de leitura do PUT abaixo. |
| PUT | `/v1/accounts/{accountId}/members/{userId}/roles` | `200 { roles: RoleSummary[] }` | replace em LOTE dos papeis do user na account. Body `{ roleIds: [...] }`. Resposta = os papeis efetivos do user NAQUELA account apos o replace (mesmo shape do `GET`, via `ToSummary()`) — painel atualiza sem refetch. Exige `core.roles.manage`. Valida que o alvo e membro (404 `user_not_found` senao) e que CADA roleId pertence a account (`FindRole` → 404 `role_not_found`). Replace transacional `ReplaceUserRoleAssignments`. **Mantem** os assign/remove 1-a-1 (`POST`/`DELETE .../roles/{roleId}` — coexistencia). |

`RBACRepository` agora expoe `HasAccountPermission(accountID, userID, permKey)`,
`CanAccessAccountRoles(accountID, userID, requireManage)`,
`ReplaceUserRoleAssignments` e `PlatformScopedKeys` (em `rbac_repository_assign.go`,
para nao crescer `rbac_repository.go`).

### LEGADO: modulo `access` (`/v1/access/*`)

`access` (grava overrides por `tenantId/storeId`, `auth.Role`, catalogo hardcoded)
e **LEGADO**. A fonte canonica de overrides por usuario passa a ser
`core.user_permission_overrides` via os endpoints `.../overrides` acima. NAO
construir features novas sobre `/v1/access/*`; a remocao da pagina/endpoints
legados fica para uma rodada futura (registrar em `docs/LEGADO.md`).

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
- `admin_users_model.go` — DTOs admin de users (`AdminUserView` com `clientAccountId` e `isAgencyMember`, `AdminUserListFilter` com `AccountID`, `AdminCreateUserInput`, `AdminUpdateUserInput`, `MoveUserAccountInput`, `AccountMembershipView`) + interface `AdminUserRepository` (inclui `MoveUserAccount`).
- `admin_users_repository.go` — `PostgresAdminUserRepository`: List com `accountCount`/`accountNames` agregados via LATERAL join SO quando `filter.IncludeAccounts` (default); com `includeAccounts=false` o join e omitido (projecao lean). List tambem aceita filtro `accountId` (`exists` em account_users+accounts ativos) e ambas List/Find selecionam `clientAccountIDSelect` (const: id do unico cliente ativo nao-agencia, ou '' p/ 0/>1) e `isAgencyMemberSelect` (const bool: `exists` membro de conta-agencia ativa) como ULTIMA coluna na mesma ordem nas duas queries. `scanAdminUser` le `clientAccountId` logo apos `accountNames` e `isAgencyMember` como ultimo campo (ordem importa). Find; Create (com hash de senha); Update; SoftDelete; GetMemberships; CountActivePlatformAdmins (safeguard); **MoveUserAccount** (transacional: valida destino ativo + nao-agencia, remove vinculos de cliente nao-agencia, auto-enroll no destino). `enrollUserInAccount` virou funcao livre recebendo `pgxExec` (pool OU tx) e faz upsert da membership com `do update set is_active = true` (reativa em re-enroll). A paginacao (page/perPage, cap 100) e aplicada na query — a tela `/manage/users` busca por pagina, nao tudo de uma vez.
- `admin_users_service.go` — `AdminUserService`: agora recebe `*AdminScopeResolver` + `AdminUserLinksRepository` no construtor. Valida unicidade de email, hash de senha e safeguard do ultimo platform_admin. `CreateUser`/`DeleteUser`/`MoveUserAccount` exigem platform_admin (identity-global). `UpdateUser` checa escopo do alvo (404) e, se ator nao-platform_admin enviar campo identity-global (`hasIdentityGlobalField`), retorna `ErrForbiddenField` (403). `GetUser`/`GetMemberships` escopam via `CanManageUser`. `UpdateMembershipRole`/`RemoveMembership` usam `ensureCanMutateAccountLink` (M2): `CanManageAccount` + conta-agencia exige autoridade de organizacao.
- `admin_users_links_service.go` — metodos de vinculo do `AdminUserService` (`AddMembership`/`RemoveMembership`/`LinkOrganization`/`UnlinkOrganization`/`getMembershipsScoped`) + `ensureCanMutateAccountLink` (M2): gate de mutacao de vinculo numa account; conta-agencia exige platform_admin OU agency_owner da org (nao basta `core.users.manage`).
- `rbac_http.go` — gate `requireRolesManage` (substitui `requireMember`): GET exige view/manage, escrita exige manage, platform_admin/agency_owner passam, fora de escopo → 404. Handlers `getUserRoles` (GET roles do user) e `setUserRoles` (PUT roles em lote). `writeRBACError` mapeia `ErrInvalidPermission`→422.
- `rbac_service.go` — `SetUserRoles` (replace em lote, valida membership do alvo + cada role na account, retorna `RoleSummary[]`), `ListUserRoles` (leitura) e `HasAccountPermission`. `UpdateRolePermissions` bloqueia `scope='platform'` via `PlatformScopedKeys` (M1).
- `admin_users_http.go` — `RegisterAdminUsersRoutes`: endpoints `/v1/admin/users*`. Gate `RequireAuth` + `requireAdminActor` (403 a quem nao administra nada via `IsAdminOfAnything`); escopo fino POR-HANDLER no service. `actorID(r)` extrai o ator do Principal (nunca da query/body). `handleListUsers` injeta `ActorUserID` no filtro. Novos handlers: `handleAddMembership`, `handleRemoveMembership`, `handleLinkOrganization`, `handleUnlinkOrganization`. `writeAdminUserError` mapeia os sentinels novos (`ErrForbidden`→403, `ErrForbiddenField`→403 `forbidden_field`, `ErrOrganizationNotFound`→404, `ErrInvalidOrgRole`→400, `ErrConfirmationRequired`→422 `confirmation_required`, `ErrLastAgencyOwner`→409). **Manter** `requirePlatformAdmin` em `admin_http.go` para `/v1/admin/accounts*` (fora desta delegacao).
- `admin_scope.go` — `AdminScopeResolver` + interface `AdminScopeRepository` + `adminManagePermKeys` ({core.users.manage, core.roles.manage}). Peca-chave da delegacao: decide por-request/por-account quem administra o que. Injetado no `AdminUserService` e no `AdminOverridesService`.
- `admin_scope_repository.go` — `PostgresAdminScopeRepository`: `CanManageAccount`/`CanManageUser`/`CanManageOrganization`/`IsPlatformAdmin`/`IsAdminOfAnything`, todos resolvendo no banco. `manageablePermsExists(accountExpr)` e `manageableAccountWhere()` espelham a UNION allow EXCEPT deny de `rbac_repository.go` (caminho (c) = permissao resolvida, NAO membership). `listUsersScopeWhere(actorIdx, permsIdx)` e o predicado de escopo do GET list (mesmo `where` no count + linhas).
- `admin_users_links_http.go` — (sem handlers proprios; os handlers de vinculo vivem em `admin_users_http.go`). `admin_users_links_repository.go` — `PostgresAdminUserLinksRepository`: `FindAccountLinkInfo`, `AddMembership` (reusa `enrollUserInAccount`), `DeactivateMembership` (tx), `FindOrganizationLinkInfo`, `LinkUserToOrganization`, `CountAgencyOwners`/`IsAgencyOwner` (safeguard), `UnlinkUserFromOrganization` (tx). `linkUserToOrganization` foi EXTRAIDO de `CreateUser` (DRY) e e chamado por ambos.
- `admin_users_links_service.go` — metodos de vinculo do `AdminUserService`: `AddMembership`/`RemoveMembership` (`CanManageAccount`), `LinkOrganization`/`UnlinkOrganization` (`CanManageOrganization` + confirmacao + safeguard), `getMembershipsScoped` (filtra as memberships pelas accounts administraveis pelo ator).
- `admin_overrides_model.go`/`_repository.go`/`_service.go`/`_http.go` — overrides allow/deny por usuario por account em `core.user_permission_overrides`. Service valida via `InvalidPermissionKeys` (RBAC repo, reuso) + bloqueia `scope='platform'` + `effect in {allow,deny}` + sem duplicata. `ReplaceUserOverrides` (tx) desativa ativos + insere com `created_by_user_id`. NAO reusa o modulo `access` (LEGADO).
- `rbac_repository_assign.go` — `HasAccountPermission`, `CanAccessAccountRoles` (gate de `/v1/accounts/{id}/roles*`), `ReplaceUserRoleAssignments` (replace em lote). Separado de `rbac_repository.go` (571 l) para nao crescer.
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
