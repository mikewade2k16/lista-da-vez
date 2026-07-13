# ERP/CRM — Escopo multi-tenant por cliente (plano)

Status: **PLANEJADO** (2026-07-10) — não implementado. Aprovação do dono para "documentar o plano" primeiro; implementação depende de ok explícito.

## Contexto / decisão do dono

Hoje o módulo ERP/CRM (telas: `/crm`, aba **ERP**, **Consultores em todas as lojas**, **Multi-loja/Metas**, **Ranking**) ignora a conta ativa do topo e resolve **sempre** a loja root `ERP_ROOT_STORE_CODE=184`, que pertence à conta **Pérola** (tenant seed `aaaaaaaa-...`). Consequência: em qualquer conta ≠ Pérola, o CRM mostra dados da Pérola (ou cruza fila da conta com venda da Pérola), e erros são mascarados por `.catch(() => null)`.

**Decisão (2026-07-10):** o ERP/CRM deve seguir o **mesmo padrão dos outros módulos** (tasks/finance):
- É um **módulo comprável**. Usuário de agência/organização vê **todos os seus clientes** via **select de cliente**, mas o select **só lista clientes com o módulo `crm` habilitado** (`core.account_modules`).
- O escopo do ERP/CRM **segue o cliente ativo** (via header `X-Account-Id`), não mais o root store 184.
- Hoje **só a Pérola** tem o módulo + a integração ERP; todo dado/config atual é dela. Trocar pra outro cliente no select → **tudo vazio** (inclusive integração ERP), porque a integração pertence só à Pérola. Outros clientes poderão comprar depois.

## Estado atual (mapa — o que reusar)

Infra já pronta (não precisa inventar):
- **Header de escopo:** `web/app/plugins/account-id-bridge.client.ts` envia `X-Account-Id = accountStore.activeAccountId` em todo request.
- **Store de conta:** `web/layers/core/stores/account.ts` (`useCoreAccountStore`): `accounts[]` (com `modules: string[]`), `activeAccountId`, `enabledModules`, `switchAccount()`. Dados de `GET /v2/me/accounts` (já traz `modules` por conta).
- **Switcher:** `web/layers/core/components/CoreAccountSwitcher.vue` (não filtra por módulo hoje).
- **Validação de membership no back:** `RequireAuthWithAccount` + `back/internal/modules/auth/account_checker.go` (platform_admin, agency_owner na org, ou membership em `core.account_users`).
- **Gating de módulo:** back `RequireModuleByPath` (`/v1/erp`→`crm`, `app.go:527`) e front `middleware/module-enabled.global.ts` (`/erp`→`crm`, `/crm`→`crm`) — **já corretos**.
- **Padrão de referência (copiar):** `web/layers/tasks/stores/tasks.ts` — `accountId` computed + `X-Account-Id` por request + `watch(accountId)` → `reloadForAccountSwitch()`.

O que prende na Pérola:
- `back/internal/modules/crm/erp/service.go:471` `resolveERPScope`: se `options.RootStoreCode != ""` chama **sempre** `ResolveRootStoreScope(...)`, ignorando o tenant. `RootStoreCode` = env `ERP_ROOT_STORE_CODE=184` (`config.go`, `docker-compose*.yml`, `.env*.example`).
- Front `web/app/stores/erp.ts`, `web/app/stores/crm.ts`, `web/app/stores/consultants.ts` usam `auth.activeTenantId`/`auth.activeStoreId` (que pro platform_admin = seed Pérola), não `accountStore.activeAccountId`.

## Plano

### Fase 1 — Back: escopo por conta ativa
1. `resolveERPScope` (`service.go:471`): parar de forçar o root store. Havendo conta ativa (tenant), escopar por ela via `ResolveDefaultERPScope`/`ResolveStoreScope` (já filtram `tenant_id` e validam `CanAccessTenant`). `ERP_ROOT_STORE_CODE` vira fallback só sem conta ativa (ou removido de vez).
2. Handlers ERP (`http.go`) derivam o tenant do header `X-Account-Id` quando o `tenantId` da query vier vazio (`X-Account-Id` == `core.accounts.id` == `queue.stores.tenant_id`). Assim TODAS as telas ERP passam a seguir a conta de uma vez.
3. Conta sem ERP → resposta **vazia** (estrutura zerada), nunca erro/forbidden mascarado.

### Fase 2 — Front: telas seguem o switcher
1. `erp.ts`, `crm.ts`, `consultants.ts`: trocar `auth.activeTenantId`/`activeStoreId` por `useCoreAccountStore().activeAccountId` + `watch(activeAccountId)` que limpa estado e refaz o fetch (molde `reloadForAccountSwitch` de `tasks.ts`).
2. Empty-state **honesto** quando a conta não tem ERP ("esta conta não tem integração ERP"), removendo o `.catch(() => null)` mudo de `consultants.ts:222`.

### Fase 3 — Select de cliente filtrado por módulo
- Reusar `CoreAccountSwitcher` + `useCoreAccountStore`, filtrando as contas por `modules.includes('crm')` (generalizar o switcher com um `moduleFilter`, ou um seletor no cabeçalho do módulo ERP/CRM). Sem endpoint novo (`modules` já vem em `/v2/me/accounts`).

### Fase 4 — Dados / seed
- Pérola mantém `crm` habilitado + os dados. Outras contas: sem `crm` (ou habilitado e vazio até importarem ERP).

## Pontos a validar na implementação (não bloqueiam o plano)
- **Escopo do CRM overview:** hoje `GetCRMOverview(store, ...)` resolve 1 store; confirmar se o agregado é tenant-wide (todas as lojas da conta) ou store-único, e ajustar para cobrir todas as lojas da conta.
- **Multi-loja dentro da conta:** pode precisar de sub-seletor de loja (já existe `integratedStoreId` na operação).

## Notas de Deploy
- **`ERP_ROOT_STORE_CODE`**: hoje fixado em `184` em `docker-compose.yml`, `docker-compose.prod.yml`, `.env.production.example`, `.env.staging.example`. Ao neutralizar/remover, atualizar os 4 arquivos + o `.env.production` da VPS, na ordem: subir a imagem nova da api ANTES de remover a env (o fallback só some quando o código novo estiver no ar). Sem migration.
- Garantir seed de `crm` em `core.account_modules` para as contas-cliente que devem ver o ERP (hoje: só Pérola).

## Verificável
1. Logado (platform_admin ou agency_owner), o select do ERP/CRM lista **só** contas com `crm` habilitado.
2. Selecionar **Pérola** → dados aparecem (venda/consultores/metas). Selecionar **outra conta** → tudo vazio com aviso "sem integração ERP", **sem** cruzar dado da Pérola.
3. Network: as chamadas `/v1/erp/*` carregam `X-Account-Id` da conta selecionada e retornam o escopo daquela conta.
4. Owner/director de uma conta sem ERP não recebe dado da Pérola (isolamento multi-tenant).
5. `golangci-lint` / `vue-tsc` / `eslint` limpos; api rebuildada.

## Relacionados
- `docs/MULTITENANT_COMPLETION_PLAN.md` (padrão multi-tenant, account_modules).
- Achados que motivaram (análise 2026-07-10): divergência de escopo ERP, `.catch` mascarando, merge de metas do front (já corrigido — fonte única no back).
