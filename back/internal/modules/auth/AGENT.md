# AGENT

## Escopo

Estas instrucoes valem para `back/internal/modules/auth`.

## Versoes herdadas desta base

- Go: `1.24.0`
- Toolchain Go: `1.24.3`
- Nuxt integrado no frontend: `4.4.2`
- PostgreSQL alvo: `16`

## Responsabilidade do modulo

O modulo `auth` e a porta de entrada de identidade e autorizacao da plataforma.

Ele deve cuidar de:

- login
- aceite de convite e primeira senha
- recuperacao de senha por codigo
- emissao e leitura de token
- principal autenticado
- catalogo de roles
- middleware de auth/role
- autoatendimento do usuario autenticado:
  - editar nome
  - editar email
  - trocar senha
  - enviar foto de perfil

Ele nao deve cuidar de:

- regra operacional da fila
- CRUD de loja
- CRUD de tenant
- websocket de fila

## Como o auth funciona hoje

### Persistencia

- usuarios vivem no PostgreSQL
- identidade vive em `core.users` (fonte única; a view compat `public.users` foi DROPADA na 0136 — o auth lê `core.users` direto)
- papel/escopo no auth e resolvido **100% pelo core** (`resolveAuthRoleScope` → `resolveCoreAuthRoleScope`). O fallback legado e o flag `AUTH_ROLES_SOURCE` foram **REMOVIDOS** (2026-06-26) — desde o DROP da 0135 o unico caminho valido era `core`. Apagados: `resolveLegacyAuthRoleScope`, `findStoreIDs`, `resolveRole`, `legacyRoleProjection`, o tipo `authRolesSource`/`parseAuthRolesSource` e os campos legados de `userRecord`. O env var `AUTH_ROLES_SOURCE` virou no-op (remover do compose/.env no proximo deploy-cleanup).
- fonte core de papel/escopo:
  - `core.users.is_platform_admin`
  - `core.account_users`
  - `core.user_role_assignments`
  - `core.roles`
  - `core.user_module_settings(module_id='queue').config.storeIdsByAccount` para store scope
- fallback legado ainda le:
  - `users`
  - `user_invitations`
  - `user_platform_roles`
  - `user_tenant_roles`
  - `user_store_roles`
- a migration demo continua semeando usuarios para smoke local

### Fonte de roles U2

- Migration `0133_backfill_legacy_roles_to_core.sql` cria/copia roles core para compatibilidade:
  - `owner` -> `queue.owner` (template `queue.supervisor`) -> coarse `owner`
  - `director` -> `queue.director` (template `queue.supervisor`) -> coarse `director`
  - `marketing` -> `queue.marketing` (template `queue.consultant`) -> coarse `marketing`
  - `manager` -> `queue.manager` (template `queue.supervisor`) -> coarse `manager`
  - `consultant` -> `queue.consultant` (template `queue.consultant`) -> coarse `consultant`
  - `store_terminal` -> `queue.store_terminal` (template `queue.supervisor`) -> coarse `store_terminal`
- Compatibilidade para roles core antigos/genericos:
  - `core.owner` -> `owner`
  - `core.admin` ou `queue.supervisor` -> `director`
  - `core.member` -> `consultant`
- Nao remover o fallback legado nem o sync em `admin_users_repository.CreateUser` antes do U4.

### Hot-path do middleware (Fase 7A + U2)

- `AuthenticateToken` usa `LoadUserForAuth` para centralizar identidade + papel + escopo.
- `LoadUserForAuth` continua no contrato `Repository` (usado no hot-path), mas hoje so delega para `FindByID` — o fallback legado que justificava dois metodos foi removido. A resolucao de papel/tenant/store vive em `buildUser`/`resolveAuthRoleScope`.
- O resolvedor e SEMPRE `core.*` desde a remocao do fallback legado (2026-06-29): a flag `AUTH_ROLES_SOURCE` virou no-op e foi removida dos compose/`.env*.example` (ver `docs/LEGADO.md`). `resolveAuthRoleScope`/`buildUser` resolvem papel/tenant/store 100% do core.
- A resolucao de permissoes usa `access.Service.ResolveUserPermissions` que delega para `Repository.ResolveEffectivePermissions` (1 query unica consolidando `access_role_permissions` + `user_access_overrides`).
- Caminhos administrativos (update perfil, troca de senha) tambem passam pelo mesmo resolvedor ao remontar `User`.

### Token

- token assinado via HMAC
- `tenant_id`, `store_ids` e `role` ja vao no token
- `sid` (session UUID) incluido em novos tokens emitidos apos C6; tokens legados sem `sid` continuam validos
- ainda nao existe refresh token
- sessao persistida em `core.user_sessions` — `SessionRepository` LIGADO no boot (P0.2)

### Sessao e revogacao — estado real (P0.2, 2026-06-07)

- `auth.Service.SetSessionRepository(repo)` e chamado no `app.go`: `Login` cria linha em `core.user_sessions` + emite `sid`; `Authenticate` checa `IsRevoked` (revoked_at) por request; `Logout` revoga o `sid`.
- Logout invalida o token no servidor de verdade: token com `sid` revogado -> 401 no proximo request. (Antes era no-op client-side.)
- Tokens legados sem `sid` ignoram o check de sessao (validos ate expirar).
- **PrincipalCache LIGADO (AC-01, 2026-07-02).** `wirePrincipalCache` (`platform/app/principal_cache_wiring.go`) cria o cache no boot quando `AUTH_PRINCIPAL_CACHE_TTL > 0` (default `30s`; `0s` desliga e restaura o comportamento legado sem rebuild) e chama `SetPrincipalCache`. Na 2a request da mesma sessao, `AuthenticateToken` retorna o Principal cacheado sem `IsRevoked` nem `LoadUserForAuth` nem resolver permissoes.
  - **Invalidacao SINCRONA** (direta via setter injection, NAO via bus — seguranca nao pode depender de flag de feature): `Logout` -> `InvalidateSession(sid)`; `access.UpdateUserOverrides`, `users.Update/Archive`, `core RBACService.AssignRoleToUser/RemoveRoleFromUser/SetUserRoles`, `core AdminUserService.UpdateUser/DeleteUser` -> `InvalidateUser`; `access.UpdateRolePermissions` (matriz v1 por papel-coarse) e `core RBACService.DeleteRole` -> `InvalidateAll`. Logout revoga no DB ANTES de invalidar.
  - **Corrida conhecida (aceita, teto = TTL):** request A tem miss, le `IsRevoked=false`, e antes do `Set` o Logout revoga+invalida; o `Set` de A repovoa a entrada ja revogada. Janela de ms; exposicao maxima = TTL (30s). Tombstone fica para a versao Redis (AC-08).
  - **Cache local ao processo.** Invalidacao e cache nao cruzam instancias — a VPS roda 1 instancia hoje. Escalar horizontalmente exige AC-08 (Redis) antes.
  - Tokens legados sem `sid` NUNCA usam o cache (comportamento legado preservado). `sessions.Touch` continua nao sendo chamado.

### Roles atuais

- `consultant`
  - escopo de loja
- `store_terminal`
  - escopo de loja
  - operacao completa da propria unidade
- `manager`
  - escopo de loja
- `marketing`
  - escopo de tenant
- `director`
  - escopo de tenant
- `owner`
  - escopo de tenant
- `platform_admin`
  - escopo de plataforma

### Endpoints atuais

- `GET /v1/auth/roles`
- `POST /v1/auth/login`
- `POST /v1/auth/logout` (revoga o `sid` em `core.user_sessions`; idempotente p/ tokens legados sem `sid`)
- `POST /v1/auth/password-reset/request`
- `POST /v1/auth/password-reset/confirm`
- `GET /v1/auth/me`
- `PATCH /v1/auth/me/profile`
- `PATCH /v1/auth/me/password`
- `POST /v1/auth/me/avatar`
- `GET /v1/auth/invitations/{token}`
- `POST /v1/auth/invitations/accept`
- `GET /v1/auth/gateway/verify` — gate SSO consumido pelo **Caddy (forward_auth)** para liberar subdominios admin com o login do Omni. **Decisão 2026-06-18:** usado na **`waha.crowvisuals.com.br`** (API aberta exige); o **n8n NÃO usa** (fica com login próprio). Publico no roteamento; valida o cookie `omni_gw` (ou `Authorization` Bearer p/ curl) e exige `platform_admin`: **200** libera, **302** manda pro login (`WEB_APP_URL/auth/login`), **403** logado sem permissao. Sem gating de modulo nem `X-Account-Id` (o navegador nao manda). Ver `gateway.go` e docs/automation/SSO_GATEWAY_PLAN.md.

### Cookie de sessao do gate (`omni_gw`)

- `POST /v1/auth/login` e `POST /v1/auth/invitations/accept` setam, alem do JSON com `accessToken`, um cookie `omni_gw` (= o mesmo token HMAC) — `HttpOnly; Secure; SameSite=Lax; Domain=AUTH_GATEWAY_COOKIE_DOMAIN`. `POST /v1/auth/logout` expira o cookie.
- Existe **so** para o gate SSO: o navegador, ao abrir direto `n8n.`/`waha.`, nao manda o Bearer (esse vive no localStorage do SPA), mas manda o cookie do dominio pai. O SPA segue usando o Bearer do JSON — o cookie e aditivo, nao quebra nada.
- `AUTH_GATEWAY_COOKIE_DOMAIN`: vazio em dev (host-only), `.crowvisuals.com.br` em prod (vale em omni./n8n./waha.).

## Invariantes

- senha, hash, `must_change_password`, estado da conta e sessoes existentes nunca podem ser alterados
  para viabilizar smoke test sem autorizacao explicita do usuario para aquela conta; se a credencial
  nao estiver disponivel ou falhar, o teste deve parar e pedir confirmacao antes de qualquer reset
- email deve ser tratado normalizado em lowercase
- usuario inativo nao pode autenticar
- usuario sem `password_hash` deve receber `onboarding_required` no login
- **Login nao bloqueia por escopo (authn != authz, modelo two-step):** um usuario ATIVO cujo papel/escopo-coarse de fila NAO resolveu (so-agencia, ou so com papel custom nao-queue) AUTENTICA mesmo assim, com escopo VAZIO — espelha o `platform_admin`, que ja loga com `TenantID`/`AccountID` vazios. O login NUNCA devolve `403 user_no_role` por falta de papel-coarse. A autorizacao real (o que o usuario enxerga) e resolvida DEPOIS, por requisicao/account, na Etapa 2 — `GET /v2/me/accounts` (lista accounts org-aware) + `GET /v2/me/context?accountId=...` (papeis+permissoes CUSTOM por account via `RBACService.ResolveUserContext`). Ver `buildUser`/`HasEmptyScope` em `store_postgres.go`/`roles.go`.
- `ErrInvalidRoleScope` NO LOGIN sobra SO para papel STORE-scoped malformado (ex.: `consultant`/`manager`/`store_terminal` sem exatamente uma loja vinculada) — vira `403 user_store_scope` ("vinculo de loja invalido"), NUNCA `500`. Escopo-coarse vazio NAO passa por `ValidateUserScope` (nao e barrado).
- **A autoridade do que o usuario ve e a RBAC CUSTOM por account, nao o papel-coarse.** O papel-coarse de fila (`CoarseRoleFromCoreRole` + os Grants do `roleCatalog`) e LEGADO/queue e so alimenta `principal.Permissions` GLOBAL (`access.ResolveEffectivePermissions` por role coarse). O que o usuario realmente pode ver/editar por account vem de `core.role_permissions` (papeis criados no painel) + `core.user_permission_overrides`, resolvido por `accountId` (header `X-Account-Id`) — independente do coarse role e do `TenantID` de login.
- **`RequireAuthWithAccount` hidrata a RBAC efetiva (correcao 2026-08-12):** depois de validar a account, se o checker implementar `AccountPermissionResolver`, o middleware substitui `Principal.Permissions` pela matriz daquela account (`role_permissions` + override allow EXCEPT deny) e marca `PermissionsResolved=true`. Isso permite que handlers como Calendar reconhecam papeis custom/overrides (ex.: `editor`) sem depender da matriz coarse global. Lista vazia e valida e permanece fail-closed. Checkers simples usados em testes continuam suportados pela interface opcional.
- **Fallback por organizacao NAO preenche `TenantID` de login (decisao de seguranca).** Quando `account_users` nao mapeia papel-coarse, `resolveCoreAuthRoleScope` devolve escopo VAZIO (login segue). NAO derivamos um `TenantID` da conta-agencia para o principal porque varias rotas legadas tenant-scoped tratam `principal.TenantID` como PROVA de acesso sem rechecar membership (ex.: `queue/settings CanAccessTenant` curto-circuita o join em `core.account_users` quando o principal tem `TenantID`). Preencher esse `TenantID` daria leitura com autoridade vinda do LOGIN, nao da RBAC custom — o over-grant que o modelo two-step proibe. A conta-agencia e concedida ao usuario de org pelo caminho correto: o `account_checker` org-aware valida o `X-Account-Id` por requisicao (Etapa 2).
- usuario com `must_change_password = true` pode autenticar, mas deve ser conduzido ao fluxo de troca de senha no frontend
- token invalido ou expirado gera `401`
- role fora do catalogo e erro de modelagem
- `platform_admin` pode atravessar limites de tenant; os demais nao
- `consultant`, `manager` e `store_terminal` devem carregar uma unica loja no escopo efetivo
- token de convite deve ser persistido apenas em hash
- codigo de recuperacao de senha deve ser persistido apenas em hash
- foto de perfil nao deve ser salva em blob no PostgreSQL desta base
  - o banco guarda apenas `users.avatar_path`
  - o arquivo fica em disco/volume montado no backend
- convite aceito deve:
  - gravar a primeira senha
  - limpar `must_change_password`
  - revogar convites pendentes restantes do usuario
  - devolver sessao valida para entrar sem login extra
- troca de senha em `/v1/auth/me/password` deve limpar `must_change_password`
- atualizacao de perfil do usuario autenticado deve refletir o nome do consultor no roster quando houver vinculo `consultants.user_id`

## Regras para evolucao

Quando este modulo crescer, a ordem certa e:

1. adicionar refresh token e sessao por dispositivo
2. auditar login/logout e revogacao
3. expor autorizacao reutilizavel para websocket handshake
4. separar permissoes por grant para reduzir dependencia de role fixa
5. evoluir convite para entrega por email real
6. endurecer sessao de `store_terminal` por loja/dispositivo
7. auditar melhor resets de senha e primeiro login por dispositivo

## Cuidados ao integrar com o Nuxt

- o front deve usar `POST /v1/auth/login` e `GET /v1/me/context` antes de qualquer realtime
- `GET /v1/auth/me` continua util para leitura simples de identidade, mas o contexto de tenant/loja agora vem do endpoint composto
- **escopo vazio (two-step):** um usuario pode logar com `role`/`tenantId`/`storeIds` vazios (so-agencia/so-papel-custom). Nesse caso `GET /v1/me/context` (legado) devolve `tenants`/`stores` VAZIOS sem 403 (guard local em `platform/app/context_http.go`: pula a leitura de stores quando `principal.Role` e vazio). O front NAO deve tratar contexto vazio como erro de login: deve seguir para a Etapa 2 e carregar as accounts/permissoes via `GET /v2/me/accounts` + `GET /v2/me/context?accountId=...` (RBAC custom por account), que e a fonte do que o usuario pode ver.
- o dropdown de perfil de teste do frontend deve ficar oculto quando houver sessao real
- workspaces visiveis no front devem derivar do principal autenticado, nao de mock local
- a loja ativa do runtime local deve respeitar `store_ids` do principal autenticado

## Arquivos atuais

- `model.go` — structs, interfaces (incluindo `PrincipalCacheStore`, `SessionRepository`, `TokenManager`)
- `roles.go`
- `service.go` — `AuthenticateToken` consulta o PrincipalCache (quando ligado) antes do DB; `Login` cria sessao em `core.user_sessions`; `Logout` revoga no DB e chama `InvalidateSession` no cache (AC-01). `PrincipalCacheStore` inclui `InvalidateSession/InvalidateUser`.
- `service_principal_cache_test.go` — testes do fluxo de cache (hit na 2a request, logout invalida na hora, token legado ignora cache, cache desligado = comportamento legado)
- `sessions.go` — `PostgresSessionRepository`: Create/IsRevoked/Revoke/Touch em `core.user_sessions`
- `tokens.go` — JWT HMAC com claim `sid` (session UUID); `Issue(sessionID, user)` — sessionID pode ser ""
- `middleware.go`
- `http.go`
- `gateway.go` — gate SSO: `GatewayConfig`, cookie `omni_gw` (Set/Clear) e `handleGatewayVerify` (200/302/403, `platform_admin`)
- `store_postgres.go` — `buildUser` constroi o usuario autenticado; quando `HasEmptyScope` (papel-coarse vazio), retorna o usuario SEM chamar `ValidateUserScope` (login nao-bloqueante, modelo two-step). So valida corretude STORE-scoped quando ha papel.
- `core_role_resolver.go` — resolve papel/escopo 100% pelo core (`core.account_users` + `user_role_assignments` + `is_platform_admin`); mapeia core role -> role coarse. Quando `account_users` nao mapeia papel-coarse, devolve escopo VAZIO (login segue; sem `TenantID` derivado de org — ver invariantes).
- `passwords.go`
- `errors.go`
- `account_checker.go` — `PostgresAccountMemberChecker.IsMember` (portão do `RequireAuthWithAccount` que valida `X-Account-Id`). **Org-aware desde 2026-06-15 (AGENCY_TENANT plan, Etapa 3):** account acessível quando ativa E (a) user é `platform_admin`, OU (b) `agency_owner` em `core.organization_users` da org da account, OU (c) membership ativa em `core.account_users`. Espelha `core.ListAccountsForUser`. O mesmo checker implementa `ResolveAccountPermissions` com `accountPermissionsQuery`, consumido opcionalmente pelo middleware para entregar a RBAC custom efetiva no Principal account-scoped. Queries constantes testadas em `account_checker_test.go`.
- `invitations.go`
- `password_reset.go`

> `store_memory.go` foi removido na Fase 3 (2026-05-21). A persistência roda 100% no Postgres; o seed de usuários demo vive na migration `0002_seed_demo_auth.sql`. Se voltar a precisar de um store em memória pra testes, escreva como fake no próprio `_test.go` do consumidor — não exporte uma implementação pública.

## Auth como host, nao como dependencia obrigatoria do core

Quando outro projeto quiser reaproveitar o core Omni, este modulo pode ser substituido pelo auth do sistema host.

Nesse caso, o host precisa apenas conseguir entregar para os modulos do core:

- um contexto equivalente a `user_id + tenant_id + role + store_ids[]`
- um handshake autenticado para websocket quando usar realtime

Referencia:

- `../../CORE_MODULES_PORTABILITY.md`
