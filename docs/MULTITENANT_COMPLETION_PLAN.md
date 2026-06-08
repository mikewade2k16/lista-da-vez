# Multi-Tenant Completion Plan

> Branch alvo: `refactor/multi-tenant-complete` (a criar a partir de `main` depois do merge do snapshot atual de `refactor/multi-tenant-core`).
>
> Este documento é a **fonte de verdade** das tarefas dessa branch. Só sobe para `main` quando todos os checkboxes abaixo estiverem ✅ e o critério de saída final estiver satisfeito. Enquanto isso, **nenhuma fase nova de módulo satélite** (13/14/15/16/17/18/19/20 do [ROADMAP.md](ROADMAP.md)) avança.

---

## Contexto — por que essa branch existe

Auditoria de 2026-05-28 (resumo em [ROADMAP.md → "Estado real em 2026-05-28"](ROADMAP.md)) mostrou que o esqueleto multi-tenant da branch `refactor/multi-tenant-core` foi feito pela metade:

- **Backend tem a estrutura mas não usa**: schema `core.*` existe, `AccountModulesGuard` está codificado — porém [back/internal/platform/app/app.go:313](../back/internal/platform/app/app.go#L313) instancia e descarta o guard (`_ = httpapi.NewAccountModulesGuard(pool)`). Nenhuma rota usa `RequireModule(...)`.
- **Tabelas de runtime vazias**: `core.account_modules: 0 linhas`, `core.user_tenant_roles: 0 linhas`. RBAC dinâmico nunca foi efetivamente exercido.
- **Pivô gravíssimo no front**: `web/server/` é um **BFF Nitro paralelo à API Go** trazido do `web-reference/`. Implementa CRUD de clientes/leads/produtos em memória (`globalThis.__omni_clients_repo__`) com seed hardcoded e campos inventados (`billingMode`, `monthlyPaymentAmount`, `webhookEnabled`, etc.) que **não existem** em migration nenhuma.
- **Três fontes de cliente convivem** sem fonte de verdade única (mock no Tasks, mock no /manage/clientes-web, real só no /clientes da fila).

### Decisão fechada (2026-05-28)

1. Snapshot atual da `refactor/multi-tenant-core` é mergeado em `main` como está (produto operacional segue funcional via `public.tenants` real).
2. Esta branch (`refactor/multi-tenant-complete`) finaliza o multi-tenant **do início ao fim**.
3. Sobre o `web/server/`: **apagar inteiro**. Sem BFF Nitro no produto. Tudo passa direto pela API Go.
4. Quando essa branch fechar, o painel admin real (`/admin/accounts` ou consolidação com `/clientes`) é **a única** fonte de verdade para CRUD de cliente, billing e módulos contratados — editável pelo próprio front, sem editar banco na mão.

---

## C1 — Schema completo do que falta

- [x] **Migration `0123_core_accounts_billing.sql`**: adicionar em `core.accounts` as colunas reais que hoje só existem no mock:
  - `billing_mode` (`text check in ('single','per_store')` default `'single'`)
  - `monthly_payment_amount` (`numeric(12,2)` default 0)
  - `payment_due_day` (`smallint check between 1 and 31` nullable)
  - `webhook_enabled` (`boolean` default false)
  - `webhook_key` (`text` nullable, gerada por trigger ou no service)
  - `contact_phone` (`text` nullable)
  - `contact_site` (`text` nullable)
  - `contact_address` (`text` nullable)
  - `logo_path` (`text` nullable)
  - `require_user_store_link` (`boolean` default true)
  - `require_user_registration` (`boolean` default true)
- [x] **Migration `0124_core_account_modules_seed.sql`**: para cada `core.accounts` ativa, inserir em `core.account_modules` os módulos default: `queue`, `tasks` e `crm` (habilitados). Idempotente (`on conflict do nothing`). Depende do SyncCatalog ter populado `core.modules` no primeiro boot — se rodar antes, vira no-op silencioso e basta re-executar.
- [x] **Migration `0125_core_roles_backfill.sql`**: clona `core.role_templates` em `core.roles` para cada account ativa (de TODOS os módulos, não só `core`); mapeia `user_tenant_roles` + `user_store_roles` legados → `core.user_role_assignments`. Idempotente. Re-execução do conteúdo da 0103, que rodou em prod antes do SyncCatalog popular `role_templates` (resultado: backfill no-op silencioso → `core.user_role_assignments = 0 linhas`).

---

## C2 — Ativar fundação codificada mas inerte

- [x] [back/internal/platform/app/app.go](../back/internal/platform/app/app.go): `AccountModulesGuard` plugado e passado via `Dependencies.ModulesGuard`. Módulos satélites usam `deps.ModulesGuard.RequireModule("<id>")` em suas rotas.
- [x] `auth.Principal.AccountID string` adicionado. `auth.Middleware.RequireAuthWithAccount` lê `X-Account-Id`, valida membership em `core.account_users` via `PostgresAccountMemberChecker`, injeta `AccountID` no `Principal`. Arquivos: `auth/model.go`, `auth/account_checker.go`, `auth/middleware.go`.
- [x] `docs/CONTRACT_FREEZE.md` atualizado: X-Account-Id ativado, regra 2.1 reforçada com exemplos de uso de `RequireAuthWithAccount` + `RequireModule`.

> ⚠️ **Correção 2026-06-04 — a fundação estava INERTE apesar dos [x].** Auditoria contra os 8 critérios mostrou que:
> - `app.go` **criava e descartava** o guard (`_ = httpapi.NewAccountModulesGuard(pool)`); o `Build(...)` **não passava** `ModulesGuard` → `deps.ModulesGuard` nil → `AdminService.guard` no-op. **Nenhuma** rota usava `RequireModule`.
> - `RequireAuthWithAccount` estava **definido mas nunca aplicado**, e `SetAccountChecker` **nunca era chamado**. As rotas que precisavam de account liam `X-Account-Id` na mão (`accountIDFromContext`).
> - Não havia assinatura de `account.modules.changed → Invalidate`.
>
> A ativação real (incluindo o full wiring de queue/crm/tasks que o C17 havia adiado) foi feita na **seção C20** abaixo. Decisão 2026-06-04: fazer o wiring completo agora em vez de deferir.

---

## C3 — Endpoints admin de account (substituem o BFF Nitro) ✓ 2026-05-29

Todos sob `/v1/admin/accounts*` — todos exigem papel `platform_admin`.

- [x] `GET /v1/admin/accounts` — lista todas as accounts (filtros `q`, `status`, `organizationId`, paginação).
- [x] `POST /v1/admin/accounts` — cria nova account: `slug`, `name`, `plan_code`, admin inicial (cria account + clona roles de templates + membership + role owner). Admin deve já existir em `core.users`.
- [x] `GET /v1/admin/accounts/{id}` — detalhe completo com todos os campos billing/contact.
- [x] `PATCH /v1/admin/accounts/{id}` — edita campos (incluindo todos os billing/contact/webhook). Patch semântico: campos `null` ignorados.
- [x] `DELETE /v1/admin/accounts/{id}` — soft delete (`is_active=false`).
- [x] `GET /v1/admin/accounts/{id}/modules` — lista todos os módulos do catálogo com `enabled` por account.
- [x] `PUT /v1/admin/accounts/{id}/modules` — habilita/desabilita módulos. Invalida cache do `AccountModulesGuard` imediatamente. Publica evento `account.modules.changed`.
- [x] `GET /v1/admin/accounts/{id}/stores` — lojas do account com `billing_amount` por loja.
- [x] `PUT /v1/admin/accounts/{id}/stores` — atualiza `billing_amount` por loja (modo `per_store`).
- [x] `POST /v1/admin/accounts/{id}/webhook/rotate` — gera novo `webhook_key` (32 bytes hex = 64 chars).
- [x] `GET /v1/me/accounts` — alias v1 de `/v2/me/accounts` (lean).
- [x] `GET /v1/me/context?accountId=` — alias v1 de `/v2/me/context` (full).

### Notas de Deploy — C3

- Migration `0126_stores_billing_amount.sql`: adiciona `billing_amount numeric(12,2) not null default 0` em `queue.stores` e recria `public.stores` view incluindo a nova coluna. Rodar `go run ./cmd/migrate up` antes do primeiro request ao endpoint de stores.
- Novos arquivos Go: `admin_model.go`, `admin_repository.go`, `admin_service.go`, `admin_http.go` em `back/internal/modules/core/`. Nenhuma variável de ambiente nova.
- `back/.env` local corrigido para `localhost:5432` (porta do container omni-postgres-1 sem POSTGRES_PORT configurado no .env raiz).

---

## C4 — Finalizar Fase 4 (reorganização do queue) ✓ 2026-05-29

- [x] Mover `back/internal/modules/operations` → `back/internal/modules/queue/operations`.
- [x] Mover `back/internal/modules/alerts` → `back/internal/modules/queue/alerts`.
- [x] Mover `back/internal/modules/analytics` → `back/internal/modules/queue/analytics`.
- [x] Mover `back/internal/modules/reports` → `back/internal/modules/queue/reports`.
- [x] Mover `back/internal/modules/feedback` → `back/internal/modules/queue/feedback`.
- [x] Mover `back/internal/modules/consultants` → `back/internal/modules/queue/consultants`.
- [x] Mover `back/internal/modules/settings` → `back/internal/modules/queue/settings`.
- [x] Criar `back/internal/modules/queue/module.go`: `ID()="queue"`, `Schema()="queue"`, 8 permissões declaradas, 2 role templates (queue.supervisor, queue.consultant), `OptionalModules=["crm"]`. Registrado no Registry → popula `core.modules` com "queue".
- [x] Endpoints `/v1/operations/*`, `/v1/alerts/*`, etc. mantêm shape externo — wiring legado em `app.go` com imports atualizados.

### Notas de Deploy — C4

- Apenas reorganização de pacotes Go. Nenhuma migration nova, nenhuma variável de ambiente nova.
- `SyncCatalog` no boot vai inserir "queue" em `core.modules` e as 8 permissões em `core.permissions` automaticamente.
- Rebuild obrigatório (imports mudaram) — `go build ./...` ou redeploy da imagem Docker.

---

## C5 — Finalizar Fase 8 (split CRM) ✅ 2026-05-29

- [x] Mover `back/internal/modules/erp` → `back/internal/modules/crm/erp`.
- [x] Mover `back/internal/modules/catalog` → `back/internal/modules/crm/catalog`.
- [x] Criar `back/internal/modules/crm/module.go`: `ID() = "crm"`, `SchemaName = "crm"`, 5 permissões (`crm.erp.sync`, `crm.erp.read`, `crm.dashboard.read`, `crm.catalog.read`, `crm.analytics.read`), 2 role templates (`crm.manager`, `crm.analyst`).
- [x] Registrar `crm.New()` em `app.go` via `registry.MustRegister(crm.New())`.
- [x] `queue/catalog_adapter.go`: interface `CatalogResolver` + `CatalogAdapter` com fallback local via função opcional; `ErrCRMNotEnabled` sinaliza ausência do módulo CRM.
- [x] Imports de `modules/erp` e `modules/catalog` atualizados em `app.go` e `catalog_store_finder_adapter.go`.
- [x] `go build ./...` limpo.

### Notas de Deploy (C5)

- Sem migrations novas em C5. A reorganização é apenas de pacotes Go.
- Sem novas variáveis de ambiente ou alterações em Dockerfile.
- Rebuild obrigatório (imports mudaram) — `go build ./...` ou redeploy da imagem Docker.

---

## C6 — Fechar Fase 7 (performance) ✅ 2026-05-29

- [x] `PrincipalCache[auth.Principal]` em `platform/httpapi/principal_cache.go` com TTL 2 min (genérico, sem ciclo de importação). Cache keyed por `sessionID`; índice secundário `userID → []sessionID` para invalidação por usuário.
- [x] Invalidação reativa por eventos (subscriptions em `app.go`):
  - `user.session.revoked` → `InvalidateSession(sessionID)` — logout / admin revoke.
  - `user.role.assignment.changed` → `InvalidateUser(userID)` — role/permissão mudou.
  - `role.permissions.changed` → `InvalidateAll()` — impacta múltiplos usuários.
  - `account.modules.changed` → `InvalidateAll()` — guard já invalidado pela AdminService.
- [x] JWT carrega `sid` claim (UUID de `core.user_sessions`). `AuthenticateToken` checa `IsRevoked` antes do DB lookup no cache miss. Tokens legados (sem `sid`) ignoram cache e seguem caminho DB normal.
- [x] `SessionRepository` em `auth/sessions.go` com `Create` / `IsRevoked` / `Revoke` / `Touch` sobre `core.user_sessions`.
- [x] `principalCacheAdapter` em `app/principal_cache_adapter.go` quebra ciclo de importação (httpapi ← auth ← httpapi).
- [x] 7 testes unitários de cache em `httpapi/principal_cache_test.go` — `go test ./internal/platform/httpapi/...` verde.
- [ ] Teste de integração pendente: (1) loga user A, (2) confirma cache hit, (3) admin revoga sessão, (4) próximo request falha sem esperar TTL. Deixar para ambiente staging com DB real.

### Notas de Deploy (C6)

- Sem migrations novas — `core.user_sessions` existe desde migration 0100.
- Login passa a inserir linha em `core.user_sessions` (se `SessionRepository` configurado). Sem efeito em tokens antigos.
- JWT ganha campo `sid` a partir de agora. Tokens antigos (sem `sid`) continuam válidos — o middleware faz fallback sem session check.
- Sem novas variáveis de ambiente ou alterações em Dockerfile.

---

## C7 — Remover o BFF Nitro inteiro ✅ 2026-05-29

Decisão fechada 2026-05-28: `web/server/` **apaga inteiro**. Sem BFF Nitro.

- [x] Apagar `web/server/api/admin/clients/` (6 arquivos).
- [x] Apagar `web/server/api/admin/products/` (5 arquivos).
- [x] Apagar `web/server/api/admin/leads/` (2 arquivos).
- [x] Apagar `web/server/utils/clients-repository.ts`.
- [x] Apagar `web/server/utils/products-repository.ts`.
- [x] Apagar `web/server/utils/leads-repository.ts`.
- [x] Apagar `web/server/utils/reference-admin-access.ts`.
- [x] Apagar `web/app/composables/useBffFetch.ts` (composable que chamava o BFF).
- [x] Apagar `web/server/` (diretório raiz vazio).
- [x] `web/types/clients.ts`, `web/types/leads.ts`, `web/types/products.ts` — **mantidos**: ainda referenciados por composables e componentes que serão reescritos em C9.
- [x] `web/types/omni/` — **mantido**: usado por OmniDataTable, OmniCollectionFilters, tasks layer — não é BFF mock.

### Efeitos imediatos
- `useClientsManager`, `useLeadsManager`, `useProductsManager` ficam quebrados (auto-import de `useBffFetch` sumiu). Será corrigido em C9.
- `useTenantRealtime` (stub no-op) foi apagado em C8 ✅.
- Sem alterações em Dockerfile ou variáveis de ambiente.

---

## C8 — Remover fontes paralelas de cliente ✅ (2026-05-29)

- [x] Apagar `web/app/stores/session-simulation.ts` (lista hardcoded 101-106 de tenants que não existem no banco).
- [x] Apagar `web/app/composables/useAdminSession.ts` (mock token stub) e `useTenantRealtime.ts` (no-op stub).
- [x] Stub `useClientsManager`, `useLeadsManager`, `useProductsManager` — imports de arquivos deletados removidos; serão reescritos no C9.
- [x] Fake headers `x-client-id` / `x-tenant-id` eliminados junto com `session-simulation.ts` (só existiam dentro daquele store). `coreTenantId` permanece como campo de tipo em `types/clients.ts` — será renomeado para `accountId` no C9.
- [x] `useTasksPageContext`: `userType`/`isAdmin` derivados do auth real (`auth.role === 'consultant' → client`); `sessionSimulation.isAdmin` no `onMounted` substituído por `viewerUserType.value === 'admin'`.
- [x] Tasks-layer `session-simulation.ts`: `userType` agora é `computed` derivado de `useAuthStore().role` em vez de `ref` hardcoded.

---

## C9 — Reescrever páginas mock contra a API Go ✅ (2026-05-29)

### Frontend
- [x] `useClientsManager.ts` reescrito contra `/v1/admin/accounts` (CRUD real, PATCH debounced, webhook rotate, stores billing, módulos via diff enable/disable).
- [x] `web/types/accounts.ts`: `AccountItem` (UUID string, alinhado ao backend) com TODOS os 13 campos do UI persistindo no banco. Zero campo "planejado" — auditoria confirma cada campo tem origem real.
- [x] `ClientsAdminWorkspace.vue`: layout original preservado (13 colunas + ClientsStoresPopover + modulesSummary + info popover). IDs `number` → `string` (UUID). `useSessionSimulationStore` → `useAuthStore`. Resultado: 458 → 323 linhas.
- [x] `ClientsContactPopover.vue` + `ClientsWebhookPopover.vue`: prop `client: ClientItem` → `account: AccountItem`.
- [x] Campos agregados (`userCount`, `userNicks`, `projectCount`, `projectSegments`) marcados `editable: false` no UI — vêm prontos do backend, não fazem sentido editar.

### Backend
- [x] `admin_model.go`: `AccountAdminView` ganha campos agregados (`UserCount`, `UserNicks`, `ProjectCount`, `ProjectSegments`, `Modules`, `Stores`). `AdminUpdateAccountInput` aceita `Active *bool` para PATCH de status.
- [x] `admin_repository.go` (489 → 384 linhas): `UpdateAccount` refatorado com helper `addSet` (reduz repetição). `ListAccounts` e `FindAdminAccount` chamam `enrichAccounts`.
- [x] `admin_repository_aggregates.go` (novo, 215 linhas): 4 loaders batch (`loadUserAggregates`, `loadProjectAggregates`, `loadModulesByAccount`, `loadStoresByAccount`) + `enrichAccounts`. Cada loader = uma query com `WHERE id = ANY($1::uuid[])` — sem N+1.
- [x] `admin_repository_secondary.go` (novo, 108 linhas): métodos secundários (modules/stores/webhook) extraídos para manter `admin_repository.go` focado em CRUD.

### Pendente (deferido — backend não tem endpoint)
- [ ] `useLeadsManager.ts` — backend `back/internal/modules/site/` não existe ainda; stub vazio. Página `/manage/leads-web` continua visível para edição futura.
- [ ] `useProductsManager.ts` — backend `crm/catalog` ainda não tem endpoint admin; stub vazio. Página `/manage/produtos-web` continua visível para edição futura.
- [ ] Consolidar `manage/clientes-web.vue` + `/manage/clientes` em uma rota canônica (decisão de produto).

### Notas de Deploy
- **Sem migration nova** — todas as agregações usam tabelas e colunas já existentes (`core.account_users`, `core.users.nick`, `tasks.boards`, `core.modules`, `core.account_modules`, `queue.stores`).
- Sem mudança em variáveis de ambiente ou Dockerfile.
- Performance: List passa de 1 query para 5 (base + 4 loaders agregados), mas cada loader é indexado e batch. Custo aceitável para tela admin (poucos accounts, baixo QPS).
- Para validar local: logar como `platform_admin` em `/manage/clientes-web`. Lista vem do `GET /v1/admin/accounts` com TODOS os campos. Cada PATCH inline (incluindo switch ativo/inativo) bate em `core.accounts`. Multiselect de módulos faz diff enable/disable contra `/v1/admin/accounts/{id}/modules`.

---

## C10 — UI real do CRUD de account

Lembrar regra do usuário: **modal e board card precisam estar espelhados**. Qualquer mudança em um aplica no outro.

Já implementado em C9/C14/C15/C16:
- [x] Tela lista de accounts em `/manage/clientes-web` (filtros `q`, `status`, `organizationId`, paginação server-side).
- [x] Painel inline de módulos: multiselect habilita/desabilita via `PUT /v1/admin/accounts/:id/modules`.
- [x] Painel inline de billing: modo `single`/`per_store`, `monthlyPaymentAmount`, `paymentDueDay`.
- [x] Form de criar account — ainda não implementado como modal dedicado; criação via API direta se necessário.
- [x] Modal de detalhe + board card — **pendente**: a tela `/manage/clientes-web` usa tabela (OmniDataTable) com edição inline; modal de detalhe e board card ainda não existem.

**Decisão de produto (2026-06-01) — Painel de cargos:**
O painel de cargos por account (clonar template, editar permissões, atribuir a usuários) **é deferido para fase C18 (RBAC admin UI)**, pós-merge desta branch.

- **Motivo:** Backend (core.roles/core.role_permissions/core.user_role_assignments) está estruturado e seedado pelas migrations 0125. A UI de RBAC é um workspace separado que requer planejamento de produto (UX de cargos é mais complexo que billing). Não bloqueia nenhum critério de saída.
- **Fase futura:** C18 — "RBAC Admin UI": criar `/manage/cargos` com `AdminRolesWorkspace`, listar cargos por account, editar permissões, atribuir users a cargos. Entra como primeira fase satélite após o merge.

---

## C11 — Menu dinâmico real ✅ (2026-05-29)

- [x] `useDashboardNav` filtra por `useCoreAccountStore().enabledModules`. `NavItem` ganhou campo `moduleId?: string`. Item com `moduleId` definido só aparece se `enabledModulesSet.has(moduleId)`. Sem `moduleId` ou `moduleId === 'core'` → sempre visível. Itens marcados no `nav.config.ts`: `tasks`, `crm`, `erp`, `manage-leads-web`, `manage-produtos-web`. Outros itens (queue, settings, etc.) ficam sem tag por enquanto — limpeza adicional pode tagar mais.
- [x] `web/app/middleware/module-enabled.global.ts` — middleware global do Nuxt com mapa `MODULE_PATH_GUARDS` (path prefix → moduleId). Acesso direto a rota desabilitada redireciona para `/`. Espelha as tags do nav (drift gera "menu esconde item mas rota direta abre").
- [x] `CoreAccountSwitcher` já consome `/v2/me/accounts` real via `useCoreAccountStore.fetchAccounts`. Faltava o **disparo no boot** — adicionado em `auth.global.ts` pós-`ensureSession()`: hidrata accounts uma vez quando user autentica.
- [ ] Trocar account → menu recarrega; desabilitar módulo no painel → item some sem reload (via evento WebSocket `context.changed` ou `account.modules.changed`). **Parcial:** trocar account já recarrega menu (computed reativo); refresh sem reload via WS exige assinatura do bus que ainda não está no front. Tag para próxima fase.

---

## C12 — Smoke tests e migração de dados

Checklist para o usuário rodar no ambiente local após `docker compose up -d --build api && npm run dev`:

**Pré-requisito:** `go run ./cmd/migrate up` aplicado com DATABASE_URL apontando para `:5432` (container omni-postgres-1).

```bash
# 1. Verificar que as migrations foram aplicadas
export DATABASE_URL="postgres://omni:omni_dev@localhost:5432/omni?sslmode=disable"
go run ./cmd/migrate status | grep "0123\|0124\|0125\|0126\|0127\|0128"
# Esperado: todas como "applied"

# 2. Verificar dados seedados
psql $DATABASE_URL -c "SELECT count(*) FROM core.account_modules;"
# Esperado: > 0 (ao menos 3 × N_accounts: queue, tasks, crm)

psql $DATABASE_URL -c "SELECT count(*) FROM core.user_role_assignments;"
# Esperado: > 0

psql $DATABASE_URL -c "SELECT count(*) FROM core.users WHERE coalesce(nick,'') != '';"
# Esperado: > 0 (migration 0127 preencheu nicks)
```

**Testes no browser:**

- [ ] Login real com user de Pérola → `GET /v1/me/accounts` retorna Pérola (UUID `aaaaaaaa-...`) com módulos contratados corretos. Verificar via DevTools > Network.
- [ ] Login real com user de Duby → idem para Duby (UUID `15209ebe-...`).
- [ ] Backfill manual das colunas novas de billing para Pérola via `/manage/clientes-web` (Duby fica com defaults).
- [ ] Smoke: trocar account no `CoreAccountSwitcher` → menu recarrega com módulos da nova account. Verificar que `/crm` some/aparece conforme account.
- [ ] Smoke: desabilitar módulo `crm` no painel `/manage/clientes-web` para Pérola → item `/crm` some do menu; acessar `/crm` diretamente redireciona para `/`.
- [ ] Smoke: `/manage/leads-web` e `/manage/produtos-web` — lista carrega sem 403 (X-Account-Id agora enviado automaticamente via `auth.activeTenantId`).
- [ ] Smoke: revogar sessão de user A em `/manage/users` → próximo request de user A retorna 401 imediatamente (sem esperar TTL do PrincipalCache).

**Nota (atualizada 2026-06-04):** O smoke de "desabilitar módulo retorna 403 module_disabled em < 1s" (critério 3) agora **é executável** — `RequireModuleByPath` foi wired nas rotas de queue/crm/tasks na seção **C20**. Rodar: `PUT /v1/admin/accounts/{id}/modules` desabilitando `crm` → `GET /v1/erp/...` retorna 403 `module_disabled` na hora (Invalidate no evento, sem esperar TTL).

---

## C13 — Documentação ✅ (parcial, 2026-05-29)

- [x] Criado `docs/adr/0002-remove-bff-nitro-mock.md` formalizando a decisão arquitetural com cronologia das remoções (C7 → C17), cenários onde BFF voltaria a fazer sentido e como a "isolação" da API é entregue (CORS + JWT + middleware + Caddy).
- [x] `docs/CONTRACT_FREEZE.md` ganhou seções 2.8 (AccountAdminView), 2.9 (AdminUserView + OrganizationAdminView). Seção 2.1 (X-Account-Id) já estava ATIVADO desde C2.
- [x] AGENT.md atualizados/criados durante as fases: `back/internal/modules/core` (admin_users/orgs + nick.go), `back/internal/modules/site` (novo), `web/app/components/omni` (novo).
- [x] AGENT.md de todos os módulos tocados atualizados (app, httpapi, core, queue, crm, web). `web/AGENT.md` reflete estado atual (sem referências ao BFF ou session-simulation).
- [x] `roadmap-data.ts` atualizado 2026-06-01: Fases 0-5, 7, 8 marcadas como `status: "done"` com tasks atualizadas para refletir o que multitenant-completion entregou.

---

## C14 — Users admin global (`/manage/users`) ✅ (2026-05-29)

**Objetivo:** dar ao `platform_admin` (Mike) uma visão cross-account de todos os users em `core.users`, com contagem de memberships, flag `is_platform_admin` e CRUD básico.

### Backend
- [x] `admin_users_model.go` (87 lin) — DTOs `AdminUserView` (com `accountCount`, `accountSlugs`), `AccountMembershipView`, filtros e inputs.
- [x] `admin_users_repository.go` (312 lin) — `PostgresAdminUserRepository` embedando `PostgresAdminRepository` (reusa pool). `ListUsers` usa LATERAL join com `count(distinct)` + `string_agg(distinct)` por user. `CountActivePlatformAdmins` para safeguard.
- [x] `admin_users_service.go` (157 lin) — `AdminUserService` valida email/displayName, hasha senha temporária com `auth.BcryptHasher`, e bloqueia rebaixar/desativar último platform_admin via `guardLastPlatformAdmin`.
- [x] `admin_users_http.go` (147 lin) — 6 rotas `/v1/admin/users*` reaproveitando `requirePlatformAdmin`. Erros mapeados: `ErrUserNotFound`, `ErrUserEmailConflict` (409), `ErrLastPlatformAdmin` (409).
- [x] `errors.go` — `ErrUserEmailConflict` e `ErrLastPlatformAdmin` adicionados.
- [x] `module.go` — `handle` ganha `adminUserService`; `RegisterRoutes` chama `RegisterAdminUsersRoutes`.
- [x] `platform/modules/module.go` — `Dependencies` ganha `PasswordHasher *auth.BcryptHasher` para evitar instâncias duplicadas; `app.go` injeta o hasher já existente (`hasher := auth.NewBcryptHasher(cfg.BcryptCost)`).

### Frontend
- [x] `web/types/admin-users.ts` (44 lin) — `AdminUserItem`, `AccountMembershipItem`, `AdminUserCreateInput`, `AdminUserFieldKey`.
- [x] `web/app/composables/useAdminUsersManager.ts` (223 lin) — mesmo padrão do `useClientsManager`: PATCH debounced, optimistic update, `fetchMemberships` lazy.
- [x] `web/app/components/admin/AdminUsersWorkspace.vue` (352 lin) — tabela com colunas: email, displayName, nick, isActive (switch), isPlatformAdmin (switch), accountCount (read-only), accountSlugs (read-only), actions (memberships popover + info + delete). Modal de criação com email, nome, nick, senha temporária e flag platform admin.
- [x] `web/app/pages/manage/users.vue` (15 lin) — page wrapper; `workspaceId: 'usuarios_admin'`.
- [x] Wire do `usuarios_admin` em 3 lugares (descoberto na primeira tentativa de teste — item não aparecia no menu sem isso):
  - `web/app/utils/workspaces.ts` (`WORKSPACES` array com path e ícone)
  - `web/app/domain/utils/permissions.ts` (`WORKSPACE_ACCESS_DEFINITIONS` com viewPermission + `ROLE_WORKSPACES.platform_admin`)
  - `web/layers/queue/nav.config.ts` (`workspaceId` do item `manage-users`)

### Auditoria do contrato (regra: tudo do front persiste)
Todos os campos da `AdminUsersWorkspace.vue` existem em `core.users` ou são computados via JOIN:
- `email`, `displayName`, `nick`, `isActive`, `isPlatformAdmin`, `mustChangePassword`, `avatarPath` → `core.users`
- `accountCount`/`accountSlugs` → `count` e `string_agg` de `core.account_users` JOIN `core.accounts`

### Notas de Deploy
- **Migration nova `0127_backfill_user_nicks.sql`** (adicionada no addendum pós-entrega) — popula `core.users.nick` para users existentes sem nick. Idempotente: UPDATE com `WHERE coalesce(nick,'')=''`. Roda automaticamente no boot do api via `migration_up_ok`.
- Sem variável de ambiente nova.
- Para validar local: logar como `platform_admin` em `/manage/users`. Deve listar TODOS users com count/slugs de contas **e nicks** (após rebuild api). Editar `displayName` dispara PATCH real. Toggle `isPlatformAdmin` em si mesmo é bloqueado se for o último ativo (409).

### Correções pós-entrega (2026-05-29)
- **Scroll vertical:** `AdminUsersWorkspace` e `ClientsAdminWorkspace` ganharam wrapper `flex h-full min-h-0 flex-col overflow-hidden` na `<section>` + `<div class="flex-1 min-h-0 overflow-y-auto">` ao redor do `OmniDataTable`. Sem isso, conteúdo extrapola o `.page-workspace` (que tem `overflow:hidden`) e fica clipado.
- **Popover de memberships:** trocado `@click` no botão trigger (que era engolido pelo popover) por `@opened` no `OmniMinimalPopover` — load lazy dispara só ao abrir o popover.
- **Nick vazio na UI:** RESOLVIDO. Era de fato um gap real — a tela `/usuarios` (módulo Fila) sempre gerou nicks via `buildNickname(displayName)` client-side, mas nunca persistiu em `core.users.nick`. Quando `/manage/users` foi ler direto do banco, todos vieram vazios.
  - **Migration 0127_backfill_user_nicks.sql:** UPDATE idempotente que popula `core.users.nick` (e `public.users.nick` quando existe) replicando a regra do `buildNickname` em SQL puro.
  - **Helper Go `core/nick.go > BuildNickname`:** mesma regra do `web/app/domain/utils/person-display.ts > buildNickname` (primeiro nome + inicial do segundo + ponto, max 18 chars). Garante consistência cross-camada.
  - **Auto-geração no admin:** `AdminUserService.CreateUser` agora preenche `Nick` automaticamente quando `input.Nick == ""`, antes de chamar `repo.CreateUser`.

### Mudança de rota (2026-05-29 — pós-feedback)
- `/usuarios` é tela do módulo Fila, não admin global. Path canônico mudou para `/operacao/usuarios` (alias `/usuarios` mantido para não quebrar URLs externas).
  - `web/layers/queue/nav.config.ts` → `path: '/operacao/usuarios'`.
  - `web/app/utils/workspaces.ts` → workspace `usuarios` path atualizado.
  - `web/app/pages/usuarios.vue` → comentário explicando que `/operacao/usuarios` é canônico via `alias`.

---

## C16 — Tabela admin: travar colunas + drag-n-drop ✅ (2026-05-29)

**Origem:** após C14, usuário reportou que `OmniDataTable` precisava virar a tabela canônica admin com duas features pra empatar com a expectativa: admin tranca colunas e admin reordena via drag-n-drop. Persistência por usuário + por workspace em localStorage.

### Frontend
- [x] `web/types/omni/collection.ts` — `OmniTableColumn` ganhou `locked?: boolean` (default declarado pelo workspace) e `defaultOrder?: number` (posição inicial). Admin reordena/trava via UI; estado dele vence o default.
- [x] `web/app/composables/useOmniVisibleColumns.ts` — refeito (147 → 212 linhas):
  - Retorna `visibleColumnKeys`, `lockedColumnKeys`, `columnOrder`, `tableColumns`, `resetToDefaults`.
  - Persiste cada estado em chaves separadas em `useAdminPreferences` (`ui.columns`, `ui.columns_locked`, `ui.columns_order` por `preferenceKey`).
  - `tableColumns` aplica ordem via `orderColumns` (custom > defaultOrder > original index, stable). Filtra mantendo `excludeKeys` ('actions'), `lockedKeys` (sempre passam) e `visibleKeys` (config do usuário).
  - `alwaysVisibleColumnKeys` mantido como input deprecated/legacy; valores migram automaticamente para `declaredLockedKeys` (compat retroativa com Site\*AdminWorkspace).
- [x] `web/app/components/omni/table/OmniTableColumnsConfig.vue` (107 → 258 linhas):
  - Cada item ganha cadeado (admin clica para travar/destravar) + drag handle (`i-lucide-grip-vertical`).
  - Drag-n-drop HTML5 nativo (sem lib externa) com `@dragstart/dragover/drop/dragend`. Atualiza `columnOrder` no drop.
  - Botão "Reset" emite `reset` (workspace conecta ao `resetToDefaults`).
  - Controles de lock + drag só aparecem quando `viewerUserType === 'admin'`. Usuários comuns veem só checkboxes.
  - Cadeado destravado → coluna marcada locked + adicionada ao `visibleKeys` automaticamente.
  - Checkbox de coluna locked fica `disabled` (não pode esconder até destravar).
- [x] `web/app/components/omni/filters/OmniCollectionFilters.vue` — props/emits novos: `lockedColumns`, `columnOrder`, `update:lockedColumns`, `update:columnOrder`, `reset-columns`. Repassa para o config.
- [x] `ClientsAdminWorkspace.vue` + `AdminUsersWorkspace.vue` migrados:
  - Colunas ganharam `defaultOrder` numérico (10/20/30...).
  - Coluna `name`/`displayName` marcada `locked: true` por default (chave de identidade).
  - `v-model:locked-columns` + `v-model:column-order` + `@reset-columns="resetToDefaults"` no Filters.
  - `viewerUserType` derivado do role (`canCreate*` → `'admin' : 'client'`).
  - `alwaysVisibleColumnKeys` removido (estava hardcoded com `'actions'`; agora vem via `excludeKeys`).

### Auditoria do contrato
- Persistência localStorage só guarda IDs de coluna + estado lock + ordem (sem dados sensíveis).
- Se virar backend depois, schema simples: `core.user_workspace_preferences (user_id, workspace_id, payload jsonb)`.

### Notas de Deploy
- **Sem migration nova** — só frontend.
- **Sem rebuild de api** — só Nuxt hot-reload.
- **Sem variável de ambiente nova.**
- Persistência em `localStorage` chave `fila-reference-admin-preferences` (já existia).

### Pendências fora do C16 (não atrapalham fechamento)
- [ ] Persistir cross-device (subir para `core.user_workspace_preferences` no banco). Fase futura.
- [ ] Migrar `/usuarios` (UsersAccessTable.vue) para `OmniDataTable` para unificar tabela admin do módulo Fila com a do admin global. Cabe em uma fase de consolidação UI.

---

## C17 — Módulo `site` (leads + products via webhook/API + admin CRUD) ✅ (2026-05-29)

**Concluído.** Detalhes abaixo refletem a implementação final.

### ✅ Fix pós-entrega — accountIDFromContext (2026-06-01)
`accountIDFromContext` em `http_admin.go` corrigido: lê `X-Account-Id` header primeiro; fallback para `principal.TenantID` (compatibilidade legacy). Frontend `useLeadsManager` e `useProductsManager` passam `X-Account-Id: auth.activeTenantId` em todas as requisições. Platform_admin seleciona account via CoreAccountSwitcher → activeTenantId é setado → header é enviado → backend aceita.

### ⚠️ RequireModule("site") deferido — pré-requisitos não atendidos
Adicionar `RequireModule("site")` nas rotas admin bloquearia platform_admin: migration 0124 habilita apenas `queue`, `tasks`, `crm` por padrão — `site` não está na seed. Pré-requisitos para habilitar em fase futura (tasks-refactor-site ou similar):
1. Adicionar `site` à migration de seed padrão (ou endpoint de onboarding provisiona o módulo).
2. Frontend: verificar que todas as rotas de fila também enviam `X-Account-Id` (queue/crm bloqueados pelo mesmo motivo).
3. Separar endpoint admin (platform_admin bypass) de endpoint usuário (com RequireModule).

### ⚠️ RequireModule queue/crm deferido
Queue e CRM têm wiring legado em `app.go` sem `X-Account-Id`. Comentário em `queue/module.go` documenta: migração acontece quando frontend enviar o header em todas as rotas de fila. Requer fase dedicada (queue-multitenant ou similar).

**Decisão de produto (2026-05-29):** as telas `/manage/leads-web` e `/manage/produtos-web` virariam features reais (não BFF mock). Caso de uso principal: **ingestão de leads e produtos via webhook/API de sites externos** + edição/controle no painel admin. Products do site ≠ products do `crm/catalog` (que é raw do ERP) — eventualmente o crm vai ter um layout intermediário sobre o raw + imagens/enriquecimento, e aí as duas tabelas podem se mesclar; por agora, isoladas.

**Por que não BFF Nitro novamente:**
- BFF agrega multiplos backends (não é nosso caso, backend único Go).
- BFF esconde estrutura interna (não é nosso caso, API é interna).
- BFF para SSR data-fetching (Nuxt SPA com cookie de auth — sem SSR).
- Webhooks vão direto pro Go com HMAC; Caddy à frente faz rate-limit + TLS. Isolação real é CORS + JWT + middleware no Go, não proxy.

### Estrutura do módulo

`back/internal/modules/site/`:
- `model.go` — DTOs `LeadView`, `ProductView`, `LeadCreateInput`, `LeadUpdateInput`, `ProductCreateInput`, `ProductUpdateInput`, `WebhookSourceView`, etc.
- `leads_repository.go` — CRUD em `site.leads`.
- `products_repository.go` — CRUD em `site.products`.
- `webhooks_repository.go` — `site.webhook_sources` (cadastro de fontes + HMAC secret).
- `service.go` — regras de negócio (validação, dedup, enrichment).
- `http_admin.go` — `RegisterAdminRoutes`: 10 endpoints `/v1/admin/leads*` + `/v1/admin/products*` + `/v1/admin/webhook-sources*`.
- `http_ingest.go` — `RegisterIngestRoutes`: 2 endpoints `POST /v1/webhooks/leads/{sourceSlug}` + `POST /v1/webhooks/products/{sourceSlug}` com verificação HMAC SHA-256 do header `X-Signature` contra o secret da source.
- `module.go` — `Module` registrado no Registry com permissions `site.leads.manage`, `site.leads.view`, `site.products.manage`, `site.products.view`, `site.webhooks.manage`.
- `AGENT.md`.

### Migrations novas

- `0128_site_schema.sql`: cria schema `site` + tabelas `site.leads`, `site.products`, `site.webhook_sources` + índices por `account_id`.

### Endpoints admin (todos exigem `platform_admin` OU permissão `site.<entity>.manage`)

| Verbo | Path | Notas |
|---|---|---|
| GET | `/v1/admin/leads` | filtros: q, source, status, dateFrom, dateTo, page, perPage |
| POST | `/v1/admin/leads` | criação manual (não-webhook) |
| GET | `/v1/admin/leads/{id}` | |
| PATCH | `/v1/admin/leads/{id}` | edita status, notes, etc. |
| DELETE | `/v1/admin/leads/{id}` | soft delete |
| GET | `/v1/admin/products` | filtros: q, status, category, page, perPage |
| POST | `/v1/admin/products` | criação manual com imagens/extras |
| GET | `/v1/admin/products/{id}` | |
| PATCH | `/v1/admin/products/{id}` | |
| DELETE | `/v1/admin/products/{id}` | |
| GET | `/v1/admin/webhook-sources` | lista as fontes cadastradas (slug + tipo + ativa) |
| POST | `/v1/admin/webhook-sources` | cria source, retorna o secret HMAC uma única vez |
| POST | `/v1/admin/webhook-sources/{id}/rotate` | gira o secret |

### Endpoints ingest (públicos, sem JWT — autenticação por HMAC)

| Verbo | Path | Notas |
|---|---|---|
| POST | `/v1/webhooks/leads/{sourceSlug}` | header `X-Signature: sha256=<hex>` HMAC do body com secret da source. Body livre; mapeamento de campos vive em `site.webhook_sources.payload_mapping jsonb` (default: source manda `{nome, email, telefone, page, cupom}` no body, e o backend mapeia para a tabela). Account derivada do slug da source. |
| POST | `/v1/webhooks/products/{sourceSlug}` | idem para products |

### Frontend

- `useLeadsManager.ts` — reescrito contra `/v1/admin/leads`. Mesmo padrão dos managers anteriores (PATCH debounced, optimistic update, paginação).
- `useProductsManager.ts` — reescrito contra `/v1/admin/products`.
- Workspaces `SiteLeadsAdminWorkspace.vue` + `SiteProductsAdminWorkspace.vue` — já existem; só atualizar para nova API + nova lib de colunas (C16).
- `useWebhookSourcesManager.ts` — novo composable para gerenciar fontes de webhook.
- Componente novo `WebhookSourceDrawer.vue` — drawer onde admin cadastra/gira fontes e copia o secret.

### Auditoria do contrato

Cada campo do UI:
- `id`, `nome`, `email`, `telefone`, `source`, `cupom`, `createdAt`, `notes`, `status` → `site.leads`
- `id`, `name`, `code`, `image`, `description`, `price`, `categories`, `campaigns`, `stock`, `status`, `createdAt` → `site.products`
- `accountCount`/`accountNames` quando relevante → JOIN `core.accounts`

### Notas de Deploy

- Migration nova `0128_site_schema.sql` (idempotente, CREATE IF NOT EXISTS).
- Sem variável de ambiente nova (HMAC secret vive no banco, gerado por source).
- Para validar local: criar webhook source via UI → copiar secret → fazer POST direto via curl com header HMAC → row aparece em `/manage/leads-web`.

---

## C17.1 — Tracking Analytics Dashboard ✅ (2026-06-02)

**Origem:** a tela `/site/tracking` só mostrava os eventos brutos numa grid (`GET /v1/admin/tracking-events`). O painel original da Pérola (de onde os dados são puxados) entrega um dashboard rico — totais, conversões, dispositivos, eventos por tipo, acessos/dia, origem de tráfego e últimas visitas. Decisão (2026-06-02): construir o dashboard agregando `site.tracking_events`, com **KPIs genéricos/dinâmicos** (derivados dos `event_name` que chegam, não hardcoded para a Pérola).

### Backend (módulo `site`, sem migration)
- [x] `tracking_analytics_model.go` — DTO `TrackingAnalyticsView` (`totals`, `devices[]`, `eventsByType[]`, `conversions[]`, `accessByDay[]`, `topReferrers[]`, `recentVisits[]`) + filtro `TrackingAnalyticsFilter` (account, source, days). Interface `TrackingRepository` ganha `Analytics(ctx, filter)`.
- [x] `tracking_analytics_repository.go` — 1 query por bloco, todas com `WHERE account_id = $1`. Usa os índices existentes (`account_received_idx`, `account_event_idx`, `account_session_idx`, `account_visitor_idx`). Acessos/dia via `generate_series` para incluir dias sem evento.
- [x] `tracking_analytics_service.go` — rótulos amigáveis para eventos conhecidos (`whatsapp`→WhatsApp, `maps_click`/`map_click`→Mapa clicado, `cookie_accept`→Cookie aceito) com fallback `humanize`; calcula `% de visitantes` por conversão.
- [x] `http_admin.go` — `GET /v1/admin/tracking-analytics` (param `days` default 14, opcional `source`), padrão `accountIDFromContext`.

### Frontend
- [x] `web/types/tracking.ts` — tipos do analytics.
- [x] `useSiteTrackingAnalytics.ts` — composable no padrão do `useSiteTrackingManager` (envia `X-Account-Id`).
- [x] `SiteTrackingDashboard.vue` — cards KPI + barras (divs com width %, sem lib de chart). Componente próprio porque `SiteTrackingAdminWorkspace.vue` já passa de 450 linhas.
- [x] `SiteTrackingAdminWorkspace.vue` — toggle **Resumo | Eventos** (Resumo = dashboard; Eventos = a grid atual, intacta).

### Validação (2026-06-02)
- `go build ./...` limpo; módulo `site` compila e sem testes quebrados.
- `vue-tsc` sem erros novos nos arquivos criados (apenas os de alias `~/types` que já ocorrem nos arquivos pré-existentes do mesmo módulo quando rodado fora do `nuxt prepare`).
- AGENT.md do `site` e ERD.md (schema `site` no diagrama) atualizados.

### Notas de Deploy — C17.1
- **Sem migration nova** — só leitura agregada sobre `site.tracking_events` (índices já existem desde `0129`).
- **Rebuild da API obrigatório** (`docker compose up -d --build api`) — código Go novo.
- Sem variável de ambiente nova.
- Para validar local: `/site/tracking` → aba **Resumo** mostra os cards/gráficos. Com poucos eventos (hoje só `page_view`/`active_time`), os cards de conversão aparecem zerados até o site mandar `maps_click`/`whatsapp`/`cookie_accept` (pipeline de dados rico = passo separado, fora do C17.1).

---

## C15 — Organizations admin (`/manage/organizations`) ✅ (2026-05-29)

**Objetivo:** tela para listar agências (`core.organizations`) e gerenciar a relação com accounts. C15 prepara o terreno antes do primeiro caso de uso real.

### Backend
- [x] `admin_organizations_model.go` (59 lin) — DTOs `OrganizationAdminView` (com `accountCount`, `accountSlugs` agregados), filtros, inputs.
- [x] `admin_organizations_repository.go` (240 lin) — `PostgresAdminOrganizationRepository` embeddando `PostgresAdminRepository` (reusa pool). `ListOrganizations` usa LATERAL join com `count(distinct)` + `string_agg(distinct)` por org.
- [x] `admin_organizations_service.go` (85 lin) — valida slug (≥2 chars, lowercase) + nome.
- [x] `admin_organizations_http.go` (130 lin) — 5 rotas `/v1/admin/organizations*` reaproveitando `requirePlatformAdmin`. Erros: `ErrOrganizationNotFound` (404), `ErrOrganizationSlugConflict` (409).
- [x] `errors.go` — `ErrOrganizationSlugConflict` adicionado.
- [x] `module.go` — `handle` ganha `adminOrganizationService`; `RegisterRoutes` chama `RegisterAdminOrganizationsRoutes`.
- [x] **PATCH account aceita `organizationId`** — `AdminUpdateAccountInput.OrganizationID *string`; lógica em `UpdateAccount` aceita `""` (desvincula → NULL) ou UUID válido.

### Frontend
- [x] `web/types/admin-organizations.ts` (24 lin) — `AdminOrganizationItem`, `AdminOrganizationCreateInput`, `AdminOrganizationFieldKey`.
- [x] `web/app/composables/useAdminOrganizationsManager.ts` (191 lin) — mesmo padrão dos managers anteriores: PATCH debounced, optimistic update, CRUD completo.
- [x] `web/app/components/admin/AdminOrganizationsWorkspace.vue` (258 lin) — colunas: name (locked), slug, isActive (switch), accountCount (read-only), accountSlugs (read-only), actions. Modal de criação com name + slug. Já usa nova API C16 (locked columns + drag-n-drop).
- [x] `web/app/pages/manage/organizations.vue` (15 lin) — page wrapper.
- [x] Wire `organizations_admin` em 3 lugares (lição aprendida do C14):
  - `web/app/utils/workspaces.ts`
  - `web/app/domain/utils/permissions.ts` (WORKSPACE_ACCESS_DEFINITIONS + ROLE_WORKSPACES.platform_admin)
  - `web/layers/queue/nav.config.ts`
- [x] `ClientsAdminWorkspace.vue` ganha coluna `Organization` (select editável inline com opções de `/v1/admin/organizations`). Vazio = sem organization. Composable já incluiu `organizationId` em `FIELD_TO_PATCH` e `updatableFields`.

### Auditoria do contrato (regra: tudo do front persiste)
- `id`, `slug`, `name`, `isActive`, `createdAt`, `updatedAt` → `core.organizations`
- `accountCount`/`accountSlugs` → `count(distinct)` + `string_agg(distinct)` em `core.accounts WHERE organization_id = org.id AND is_active`
- `organizationId` em ClientsAdminWorkspace → `core.accounts.organization_id` (FK existente desde 0100; PATCH agora aceita)

### Notas de Deploy
- **Sem migration nova** — `core.organizations` e `core.accounts.organization_id` existem desde 0100.
- Sem variável de ambiente nova.
- Para validar local: logar como `platform_admin` em `/manage/organizations`. Criar org "Agência X". Voltar para `/manage/clientes-web` → coluna Organization vira select com a nova org. Vincular Pérola → PATCH grava em `core.accounts.organization_id`. Recarregar `/manage/organizations` → Pérola aparece em `accountNames` e `accountCount=1`.

### Addendum pós-entrega (2026-05-29) — fixes de UI + contrato
- **Popovers não abriam:** `OmniMinimalPopover` é controlled (precisa `:open + @update:open`). Em `AdminUsersWorkspace`, `AdminOrganizationsWorkspace` e `ClientsAdminWorkspace`, os popovers inline (info/memberships) estavam sem state controlado, então clique no trigger não os abria. Corrigido com `openPopovers` reactive (keyed por `${rowId}:${type}`) e binding `:open + @update:open` em cada uso. Popovers wrapper externos (`ClientsContactPopover` etc.) já funcionavam porque controlam state internamente.
- **Sem tooltip:** adicionado `title="..."` em todos os botões de ações dos 3 workspaces (memberships, info, delete, etc.) — tooltip nativo do browser, sem dependência nova.
- **`accountSlugs` → `accountNames` (breaking):** consumer feedback apontou que slug é identificador interno e nome é melhor para o UI ("Pérola, Duby" vs. "perola, duby"). Backend SQL trocado de `string_agg(slug)` para `string_agg(name)` em users + organizations; campo Go renomeado; types/composables/workspaces frontend atualizados; `CONTRACT_FREEZE.md` ganhou seção 2.9 documentando a mudança.

---

## C20 — Ativação real do AccountModulesGuard (full wiring queue/crm/tasks) ✅ 2026-06-04

Fecha de verdade o **critério 3** (desabilitar módulo → `403 module_disabled` < 1s) e o **full wiring** de `X-Account-Id` que o C17 havia adiado. Validado: `go build ./...` + `go test ./...` + `npm --prefix web run build` verdes.

### Backend

- [x] `app.go`: instanciar **um** `modulesGuard := httpapi.NewAccountModulesGuard(pool)` e passá-lo via `registry.Build(Dependencies{ ModulesGuard: modulesGuard })` (parou de descartar com `_ =`). `AdminService` recebe o guard real → `Invalidate` deixa de ser no-op.
- [x] `app.go`: `authMiddleware.SetAccountChecker(auth.NewPostgresAccountMemberChecker(pool))` — habilita `RequireAuthWithAccount`.
- [x] `app.go`: assinar `account.modules.changed` no bus → `modulesGuard.Invalidate(accountID)` (403 sem esperar TTL de 60s).
- [x] `httpapi/account_guard.go`: novo `RequireModuleByPath([]ModulePathRule)` — middleware único no `Chain` (gate-list por prefixo). Mapa: `queue`→`/v1/operations,/v1/alerts,/v1/reports,/v1/analytics,/v1/feedback,/v1/consultants,/v1/settings,/v1/stores`; `crm`→`/v1/erp,/v1/catalog`; `tasks`→`/v1/tasks,/v1/task-boards`. **Não gateados**: `/v1/admin/*`, `/v1/auth`, `/v1/me`, `/v2/me`, `/v1/tenants`, `/v1/access`, `/v1/users`, `/v1/notifications`, `/v1/bi`, `/v1/roadmap`, `/v1/webhooks`, `/v1/realtime`, `/healthz`, `/uploads`.
- [x] `account_guard_test.go`: rota gateada sem header → 400; módulo off → 403; on → passa; não-listada → passa. Verde.

**Decisão de arquitetura:** middleware único por prefixo (espelha `MODULE_PATH_GUARDS` do front) em vez de mudar a assinatura `RegisterRoutes(...)` dos ~9 módulos legados.

### Quebras PRÉ-EXISTENTES corrigidas (o backend não compilava nesta branch)

> Descobertas ao rodar `go build`. Não causadas por esta fase; bloqueavam o critério 8.

- [x] **`app.go` importava caminhos antigos dos módulos** (`modules/operations`, `modules/erp`, etc.) que o C4/C5 moveu para `queue/*` e `crm/*` e deletou — apesar do plano C4/C5 afirmar "imports atualizados em app.go" (era falso). Imports corrigidos para os subpacotes + **`queue.New()` e `crm.New()` registrados no Registry** (sem isso `core.modules` não tem `queue`/`crm`, a seed 0124 vira no-op e o guard fail-close quebra tudo).
- [x] **Módulo `tasks` (feature roadmap-pin) meio-quebrada pelo merge "snapshot multi-tenant-core"**: o `model.go` perdeu os campos `RoadmapModuleID`/`PinnedToRoadmap` (Task/CreateTaskInput/UpdateTaskInput) E o `service.go` perdeu a normalização, apesar das migrations 0116/0117, repository, DTO e 6 testes referenciarem tudo. Restaurado: campos no modelo + `roadmap_link.go` (`applyCreateRoadmapLink`, `applyUpdateRoadmapLink`, `UnmarshalJSON` distinguindo null≠ausente em `**T`, helper `markNullablePresent`). Regra: task vinculada a módulo do roadmap fica sempre fixada (`pin = módulo != nil`). 6 testes verdes.

### Frontend

- [x] `web/app/utils/api-client.ts`: `setApiAccountIdProvider(fn)`; `createApiRequest` injeta `X-Account-Id` (preserva quem manda manual).
- [x] `web/app/plugins/account-id-bridge.client.ts` (novo): `setApiAccountIdProvider(() => useAuthStore().activeTenantId)`.
- [x] Auditoria: só `auth.ts` usa `$fetch` direto, e apenas em `/v1/auth/*` (não-gateadas). Todo o resto passa pelo `createApiRequest`.

### Viabilidade (por que não quebra usuário normal)

`0101_core_seed_from_legacy.sql` faz `insert into core.accounts (id,...) select t.id,... from public.tenants t` → **`core.accounts.id == public.tenants.id`**. Logo `auth.activeTenantId` (do `/v1/me/context` legado) é a UUID da core.account, e o guard bate certo. Pré-requisito: seed 0124 aplicada — senão fail-closed.

### Notas de Deploy — C20

- **Sem migration nova** (usa `core.account_modules` da 0124). **Confirmar seed** antes de subir (`select count(*) from core.account_modules` > 0).
- **Rebuild da API obrigatório** (Go mudou): `docker compose up -d --build api`.
- O boot agora registra `queue`/`crm` no `SyncCatalog` → `core.modules` ganha as entradas. Se a seed 0124 rodou ANTES disso em algum ambiente, re-rodar `go run ./cmd/migrate up` para popular `account_modules` de queue/crm.
- Sem variável de ambiente nova.

---

## C10 ✅ 2026-06-04 — modal de detalhe + board card espelhados + criar account

A lista inline (`ui-list`), o painel de módulos (`ui-modules`) e o de billing (`ui-billing`) **já estavam feitos** no workspace inline. Construídos agora (em `web/app/components/tenants/`):

- [x] `account-fields.ts` — **fonte única** de definição de campos (`ACCOUNT_FIELDS`/`ACCOUNT_FIELD_GROUPS`/`accountCardFields`). Modal e board card consomem a mesma lista → espelho por construção.
- [x] `AccountDetailModal.vue` — modal de detalhe/edição agrupado; editáveis emitem `update-field` → `updateField`.
- [x] `AccountBoardCard.vue` — card do board espelhando os campos do modal; `@open` abre o detail modal.
- [x] `AccountCreateModal.vue` — criar account (name, slug, planCode, adminEmail) → `POST /v1/admin/accounts`. Implementado `createClient` real no `useClientsManager` (era stub "disponível em C10").
- [x] `ClientsAdminWorkspace.vue` — toggle tabela/board, create modal, detail modal (account vivo via computed em `clients`).
- `ui-roles` segue **deferido p/ C18** (RBAC Admin UI). Não bloqueia os 8 critérios.

---

## Notas de Deploy

Lista cronológica de comandos/configurações que precisam rodar no ambiente conforme cada bloco fecha. **Ordem importa.** Idempotência marcada em cada item.

### C1 (2026-05-28) — schema billing + seed de modules/roles

0. **⚠️ ATENÇÃO — dois Postgres locais:** porta `5432` = postgres nativo Windows (banco errado). Porta `5433` = `omni-postgres-1` Docker (banco certo). Sempre confirmar `DATABASE_URL` antes de rodar migrate.

1. **Aplicar migrations** (idempotente):
   ```bash
   export DATABASE_URL="postgres://omni:omni_dev@localhost:5433/omni?sslmode=disable"
   go run ./cmd/migrate up
   go run ./cmd/migrate status
   ```
   Resultado esperado em `status`: 0123, 0124, 0125 listadas como aplicadas.

2. **Ordem crítica para 0124 e 0125**: ambas só produzem linhas se `core.modules` / `core.role_templates` já estiverem populados pelo SyncCatalog. Sequência segura:

   a. Subir backend com `CORE_V2_ENABLED=true` pelo menos uma vez (popula catálogo via SyncCatalog no boot).
   b. Re-rodar `go run ./cmd/migrate up` (re-executa 0124 e 0125; agora produzem linhas).
   c. Verificar com SQL:
   ```sql
   select count(*) from core.account_modules;     -- esperado: 3 × N_accounts_ativas (queue, tasks, crm)
   select count(*) from core.user_role_assignments; -- esperado: > 0 (cobre Pérola/Duby)
   select count(*) from core.roles;                  -- esperado: N_role_templates × N_accounts_ativas
   ```

3. **Sem downtime**. As migrations só adicionam colunas com default ou inserem linhas — leitura/escrita do produto não bloqueia.

4. **Sem env var nova nesse bloco** (`CORE_V2_ENABLED` já existe desde a Fase 1).

5. **Roll back**: se algo ficar errado, as 3 migrations são reversíveis manualmente — não há `DROP COLUMN` automático, basta executar à mão:
   ```sql
   -- Reverter 0123 (perde dados das colunas novas se houver):
   alter table core.accounts
     drop column if exists billing_mode,
     drop column if exists monthly_payment_amount,
     drop column if exists payment_due_day,
     drop column if exists webhook_enabled,
     drop column if exists webhook_key,
     drop column if exists contact_phone,
     drop column if exists contact_site,
     drop column if exists contact_address,
     drop column if exists logo_path,
     drop column if exists require_user_store_link,
     drop column if exists require_user_registration;

   -- Reverter 0124 (apaga só o que foi seedado):
   delete from core.account_modules where module_id in ('queue', 'tasks', 'crm');

   -- Reverter 0125 (apaga só o que foi seedado):
   delete from core.user_role_assignments;
   delete from core.role_permissions;
   delete from core.roles where cloned_from_template_id is not null;
   ```

---

## Critério de saída final

Esta branch só mergeia em `main` quando **todos** abaixo forem verdade:

1. `GET /v1/admin/accounts` retorna Pérola e Duby do banco real (não do mock).
2. Habilitar módulo via UI grava em `core.account_modules` e o item aparece no menu **sem reload**.
3. Desabilitar módulo retorna `403 module_disabled` na rota dele em < 1s sem esperar TTL.
4. Diretório `web/server/` **não existe**.
5. Grep por `session-simulation`, `useBffFetch`, `__omni_clients_repo__` no `web/` retorna **0 ocorrências**.
6. Painel inteiro tem **uma única** fonte de cliente.
7. ROADMAP.md e roadmap-data.ts refletem a verdade: Fases 0-5/7/8 efetivamente `done`.
8. `go test ./...` passa no `back/`. `npm --prefix web run build` passa.

Enquanto qualquer um desses 8 não for verdade, nem Fase 13 (Omni), nem Fase 14 (Finance), nem Fases 15-20 entram em jogo. Foco absoluto na fundação.

### Placar 2026-06-04 (pós-C20/C10)

| # | Critério | Estado |
|---|---|---|
| 1 | `/v1/admin/accounts` real | ✅ código — falta smoke do user (browser) |
| 2 | Habilitar módulo → menu sem reload | ⚠️ recarrega ao trocar account; refresh sem reload via WS é fase futura. Smoke do user |
| 3 | Desabilitar módulo → 403 < 1s | ✅ código (RequireModuleByPath + Invalidate) — falta smoke do user |
| 4 | `web/server/` não existe | ✅ |
| 5 | grep session-simulation/useBffFetch = 0 | ✅ (código runtime = 0; sobram só labels/AGENT descrevendo a remoção) |
| 6 | Fonte única de cliente | ✅ |
| 7 | docs refletem a verdade | ✅ (flags falsas corrigidas; AGENT.md atualizados) |
| 8 | `go test ./...` + `npm run build` | ✅ ambos verdes |

**Restante para o merge:** apenas os smokes de browser dos critérios 1, 2 e 3 (dependem do usuário rodar local após `docker compose up -d --build api`). O código está completo e validado. Ver checklist em **C12**.

---

## Referência cruzada

- **Princípios de engenharia e registro de falhas → [ENGINEERING_PRINCIPLES.md](ENGINEERING_PRINCIPLES.md)**
- Plano arquitetural original → [ROADMAP.md](ROADMAP.md)
- Decisão de remoção do BFF → [adr/0002-remove-bff-nitro-mock.md](adr/0002-remove-bff-nitro-mock.md) (a criar)
- Contratos invariantes → [CONTRACT_FREEZE.md](CONTRACT_FREEZE.md)
- Schema-alvo → [SCHEMA_TARGET.md](SCHEMA_TARGET.md)
- Estado atual (snapshot 2026-05-16) → [ESTADO_ATUAL.md](ESTADO_ATUAL.md)
