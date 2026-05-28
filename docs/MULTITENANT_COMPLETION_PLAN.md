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

- [ ] **Migration `0123_core_accounts_billing.sql`**: adicionar em `core.accounts` as colunas reais que hoje só existem no mock:
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
- [ ] **Migration `0124_core_account_modules_seed.sql`**: para cada `core.accounts` existente (Pérola e Duby), inserir em `core.account_modules` os módulos default do plano: `queue`, `crm` (quando habilitado). Idempotente (`on conflict do nothing`).
- [ ] **Migration `0125_core_roles_backfill.sql`**: clonar `core.role_templates` em `core.roles` para cada account; mapear `user_tenant_roles` + `user_store_roles` legados → `core.user_role_assignments`. Idempotente.

---

## C2 — Ativar fundação codificada mas inerte

- [ ] [back/internal/platform/app/app.go:313](../back/internal/platform/app/app.go#L313): **parar de descartar** `AccountModulesGuard`. Plugá-lo de fato no chain dos módulos satélites (referência: comentário "Fase 6 em diante" no [account_guard.go:18](../back/internal/platform/httpapi/account_guard.go#L18)).
- [ ] Middleware de auth: resolver `Principal.AccountID` a partir do header `X-Account-Id` em **toda** rota autenticada, validando membership em `core.account_users`. Se header ausente em rota multi-tenant, 400 `missing_account_id`.
- [ ] Reforçar regra em [docs/CONTRACT_FREEZE.md](CONTRACT_FREEZE.md): nenhum handler/repository aceita `account_id` vindo do body/query. Vem **só** do `Principal.AccountID`. PRs que violem são rejeitados.

---

## C3 — Endpoints admin de account (substituem o BFF Nitro)

Todos sob `/v1/admin/accounts*` — permissão `core.account.manage` (ou platform admin).

- [ ] `GET /v1/admin/accounts` — lista todas as accounts (filtros `q`, `status`, `organizationId`, paginação).
- [ ] `POST /v1/admin/accounts` — cria nova account: `slug`, `name`, `plan_code`, admin inicial (cria user + membership + role owner).
- [ ] `PATCH /v1/admin/accounts/:id` — edita campos (incluindo todos os billing/contact/webhook).
- [ ] `DELETE /v1/admin/accounts/:id` — soft delete (`is_active=false`).
- [ ] `GET /v1/admin/accounts/:id/modules` — módulos contratados (lê `core.account_modules`).
- [ ] `PUT /v1/admin/accounts/:id/modules` — habilita/desabilita módulos. Dispara evento `account.modules.changed` no event bus → invalida cache do `AccountModulesGuard` daquele account.
- [ ] `GET /v1/admin/accounts/:id/stores` — lojas do account com billing por loja.
- [ ] `PUT /v1/admin/accounts/:id/stores` — atualiza preços por loja (modo `per_store`).
- [ ] `POST /v1/admin/accounts/:id/webhook/rotate` — rotaciona chave do webhook.
- [ ] `GET /v1/me/accounts` (lean) — lista accounts do user logado com `id`, `name`, `organizationId`, `modules[]` (ids habilitados).
- [ ] `GET /v1/me/context?accountId=` (full) — contexto completo: `user`, `roles[]`, `permissions[]` resolvidas, `organization`, `account`.

---

## C4 — Finalizar Fase 4 (reorganização do queue)

Pendência herdada do ROADMAP Fase 4B/4C (`module-rewrite`, `subpackages`).

- [ ] Mover `back/internal/modules/operations` → `back/internal/modules/queue/operations`.
- [ ] Mover `back/internal/modules/alerts` → `back/internal/modules/queue/alerts`.
- [ ] Mover `back/internal/modules/analytics` → `back/internal/modules/queue/analytics`.
- [ ] Mover `back/internal/modules/reports` → `back/internal/modules/queue/reports`.
- [ ] Mover `back/internal/modules/feedback` → `back/internal/modules/queue/feedback`.
- [ ] Mover `back/internal/modules/consultants` → `back/internal/modules/queue/consultants`.
- [ ] Mover `back/internal/modules/settings` → `back/internal/modules/queue/settings`.
- [ ] Criar `back/internal/modules/queue/module.go` consolidando: `ID() = "queue"`, `Schema() = "queue"`, `Permissions()` união, `Dependencies()` declara `crm` opcional.
- [ ] Endpoints `/v1/operations/*`, `/v1/alerts/*`, etc. mantêm shape externo — front não muda.

---

## C5 — Finalizar Fase 8 (split CRM)

- [ ] Mover `back/internal/modules/erp` → `back/internal/modules/crm/erp`.
- [ ] Mover `back/internal/modules/catalog` → `back/internal/modules/crm/catalog`.
- [ ] Criar `back/internal/modules/crm/module.go`: `ID() = "crm"`, `Schema() = "crm"`, permissões `crm.erp.sync`, `crm.dashboard.read`, `crm.catalog.read`, etc.
- [ ] Registrar `crm.Resolver` em `Dependencies`. Quando `core.account_modules` não tem `crm` para a account, devolve `ErrNotEnabled`.
- [ ] `queue/catalog_adapter.go`: tenta `deps.CRM.SearchProducts(...)`; se `ErrNotEnabled`, fallback para `queue.products_local`.

---

## C6 — Fechar Fase 7 (performance)

- [ ] `PrincipalCache` em memória (sync.Map ou ristretto) com TTL 2-5 min. Unidade cacheada: `Principal{UserID, AccountID, Permissions[], SessionRevokedAt}` — **junto**.
- [ ] Invalidação reativa **obrigatória** por eventos:
  - `user.session.revoked` → invalida sessão exata.
  - `user.role.assignment.changed` → invalida (userID, accountID).
  - `role.permissions.changed` → invalida todas as sessões do account cujo role mudou.
  - `user.permission.override.changed` → invalida (userID, accountID).
  - `account.modules.changed` → invalida o cache do `AccountModulesGuard` E todos os Principals do account.
- [ ] JWT carrega `sessionId`. Middleware checa `core.user_sessions.revoked_at IS NULL` **no mesmo lookup** do Principal (sem query separada quando cache hit).
- [ ] Teste obrigatório: integração que (1) loga user A, (2) confirma permissão X em cache hit, (3) admin remove X via endpoint, (4) próximo request de user A já não tem X — **sem** esperar TTL.

---

## C7 — Remover o BFF Nitro inteiro

Decisão fechada 2026-05-28: `web/server/` **apaga inteiro**. Sem BFF Nitro.

- [ ] Apagar `web/server/api/admin/clients/` (5 arquivos).
- [ ] Apagar `web/server/api/admin/products/` (5 arquivos).
- [ ] Apagar `web/server/api/admin/leads/` (2 arquivos).
- [ ] Apagar `web/server/utils/clients-repository.ts`.
- [ ] Apagar `web/server/utils/products-repository.ts`.
- [ ] Apagar `web/server/utils/leads-repository.ts`.
- [ ] Apagar `web/server/utils/reference-admin-access.ts`.
- [ ] Apagar `web/app/composables/useBffFetch.ts` (composable que chamava o BFF).
- [ ] Apagar `web/server/` (diretório raiz vazio).
- [ ] Apagar `web/types/clients.ts`, `web/types/leads.ts`, `web/types/products.ts` se forem só tipos do BFF (verificar).
- [ ] Apagar `web/types/omni/` se for só do BFF (verificar).

---

## C8 — Remover fontes paralelas de cliente

- [ ] Apagar `web/app/stores/session-simulation.ts` (lista hardcoded 101-106 de tenants que não existem no banco).
- [ ] Apagar/reescrever `web/app/composables/useAdminSession.ts` se for parte do esquema mock.
- [ ] Apagar `web/app/composables/useTenantRealtime.ts` se for parte do BFF mock (verificar — pode ser legítimo).
- [ ] Grep por `x-client-id`, `x-tenant-id`, `coreTenantId` no `web/` — trocar todos por `X-Account-Id` real vindo de `useAccountStore()`.
- [ ] `useTasksPageContext`: consumir contexto real de `useAccountStore` em vez do `session-simulation`.

---

## C9 — Reescrever páginas mock contra a API Go

- [ ] `useClientsManager.ts` chamando `/v1/admin/accounts*` (em vez de `/api/admin/clients`).
- [ ] `useLeadsManager.ts` chamando API Go real (criar endpoint em `back/internal/modules/site/` se módulo Site não existir ainda — pode adiar para Fase 16, mantendo `/manage/leads-web` desativado).
- [ ] `useProductsManager.ts` chamando API Go real do `crm/catalog` (adiar para Fase 16 se necessário).
- [ ] Consolidar `manage/clientes-web.vue` + `/clientes` da fila — **uma única tela admin**. Decidir nome canônico e remover a outra.

---

## C10 — UI real do CRUD de account

Lembrar regra do usuário: **modal e board card precisam estar espelhados**. Qualquer mudança em um aplica no outro.

- [ ] Tela lista de accounts (filtros: `q`, `status`, `organizationId`; busca; paginação).
- [ ] Modal de detalhe + board card de account — **espelhados**.
- [ ] Form de criar account: nome, slug, plan, módulos contratados, billing, admin inicial (cria user owner + membership).
- [ ] Painel de habilitar/desabilitar módulos por account (com confirmação; chama `PUT /v1/admin/accounts/:id/modules`).
- [ ] Painel de billing por account: modo `single` (valor único) ou `per_store` (lista de lojas com preço cada).
- [ ] Painel de cargos por account: clonar template, editar permissões, atribuir a usuários.

---

## C11 — Menu dinâmico real

- [ ] `useNav` consome `useAccountStore().enabledModules` (vindo de `core.account_modules` via `/v1/me/accounts`). Em vez do `nav.config.ts` estático filtrado por role.
- [ ] Middleware Nuxt `module-enabled.global.ts`: bloqueia rota se módulo não está habilitado para a account ativa.
- [ ] `CoreAccountSwitcher` consome `GET /v1/me/accounts` real (em vez de qualquer mock).
- [ ] Trocar account → menu recarrega; desabilitar módulo no painel → item some sem reload (via evento WebSocket `context.changed` ou `account.modules.changed`).

---

## C12 — Smoke tests e migração de dados

- [ ] Login real com user de Pérola → `GET /v1/me/accounts` retorna Pérola (UUID `aaaaaaaa-...`) com módulos contratados corretos.
- [ ] Login real com user de Duby → idem para Duby (UUID `15209ebe-...`).
- [ ] Backfill manual das colunas novas de billing para Pérola (Duby fica com defaults — não pagamento configurado).
- [ ] Smoke: trocar account no `CoreAccountSwitcher` → menu recarrega com módulos da nova account.
- [ ] Smoke: desabilitar módulo `crm` no painel para Pérola → item `/crm` some do menu e rota direta retorna 403 `module_disabled`.
- [ ] Smoke: revogar sessão de user A no painel → próximo request de user A retorna 401 imediatamente (sem esperar TTL).

---

## C13 — Documentação

- [ ] Criar `docs/adr/0002-remove-bff-nitro-mock.md` formalizando: por que o BFF foi adotado por engano, por que está sendo removido, qual é o substituto.
- [ ] Atualizar `docs/CONTRACT_FREEZE.md`: `X-Account-Id` agora ativado; documentar a regra inegociável.
- [ ] Atualizar AGENT.md de cada módulo tocado:
  - `back/internal/platform/app/AGENT.md`
  - `back/internal/platform/httpapi/AGENT.md`
  - `back/internal/modules/core/AGENT.md`
  - `back/internal/modules/queue/AGENT.md` (novo, consolidando subpackages)
  - `back/internal/modules/crm/AGENT.md` (novo)
  - `web/AGENT.md` (remover referência a web/server/)
  - AGENT.md de cada módulo movido (auth, tenants, stores, etc.)
- [ ] **Só depois** que C1-C12 estiverem todos ✅: voltar para `roadmap-data.ts` e marcar Fases 0-5, 7, 8 como `status: "done"` de verdade — refletindo realidade do runtime, não esqueleto.

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

---

## Referência cruzada

- Plano arquitetural original → [ROADMAP.md](ROADMAP.md)
- Decisão de remoção do BFF → [adr/0002-remove-bff-nitro-mock.md](adr/0002-remove-bff-nitro-mock.md) (a criar)
- Contratos invariantes → [CONTRACT_FREEZE.md](CONTRACT_FREEZE.md)
- Schema-alvo → [SCHEMA_TARGET.md](SCHEMA_TARGET.md)
- Estado atual (snapshot 2026-05-16) → [ESTADO_ATUAL.md](ESTADO_ATUAL.md)
