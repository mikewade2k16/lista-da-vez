# AC-15 — Testes front Onda 1: stores e domain/utils puros

> Spec de implementação. Autossuficiente: o implementador NÃO precisa do diagnóstico original.
> Branch: `refactor/multitenant-complete`. Raiz do repo: `c:\Users\Mike\Documents\Projects\fila-atendimento`.
> Todas as referências `arquivo:linha` foram verificadas contra o código em 2026-07-02.

## 1. Contexto

Achado canônico **AC-15 (P1, impacto médio)**: o front tem **apenas 17 arquivos `.test.ts`**
para **313 componentes .vue e 40 arquivos de store** (o back tem 65 arquivos de teste, ~13% das
linhas). Evidências verificadas:

- `web/vitest.config.ts:22-27` — include cobre `app/**/*.test.ts` e `layers/**/*.test.ts`; ambiente `node`, sem DOM.
- `web/app/domain/utils/permissions.ts` (916 linhas; `SUPERUSER_ROLES = new Set(['platform_admin'])` em l.430; `MODULE_WORKSPACE_PERMISSION_PREFIXES` em l.437-439; ~20 funções `canX`) — é o **gating inteiro do painel** e tem só 7 casos em `permissions.test.ts`.
- `web/app/domain/utils/reports.ts` (711 linhas) — 1 único caso em `reports.test.ts`.
- `web/app/domain/utils/color.ts` (497 linhas) e `web/app/domain/utils/erp-display.ts` (616 linhas) — **zero** testes.
- `web/app/stores/multistore.ts` (531) e `web/app/stores/cardapio.ts` (722) — **zero** testes; `settings.test.ts` tem 1 caso.

Esta é a **Onda 1**: só código puro (utils) e stores Pinia com `$fetch` global mockado — a
infraestrutura já existe em `web/test/setup.ts` (mocka `$fetch`, `useCookie`, `useRuntimeConfig`,
`localStorage`/`sessionStorage`; limpa tudo em `beforeEach`). Componentes/DOM ficam para a Onda 2
(exigem `@nuxt/test-utils` + `happy-dom`, ver comentário em `web/vitest.config.ts:5-7`).

`npm run test` do web é **gate do CI** (`.github/workflows/build-images.yml` roda vitest antes do
build). Teste frágil = build bloqueado. Por isso todos os asserts abaixo são determinísticos (sem
rede, sem timers reais, sem dependência de timezone).

## 2. Objetivo e não-objetivos

**Objetivo**: criar 5 arquivos de teste novos e expandir 3 existentes (total 8), cobrindo
permissions (matriz role×workspace, bypass de platform_admin, fail-closed), reports
(agregação/formatação), color, erp-display e os fluxos principais de settings/multistore/cardapio.

**Não-objetivos (explicitamente NÃO fazer):**

- NÃO testar componentes `.vue` nem nada que precise de mount/DOM.
- **NÃO tocar em `web/app/stores/erp.ts` nem criar teste para ele** — está no backlog de refatoração do AC-07 (recorte 2). `erp-display.ts` (`domain/utils/`, 616 linhas) é OUTRO arquivo (só helpers de display) e está FORA de qualquer recorte do AC-07 — PODE ser testado.
- NÃO testar nem tocar `MultiStoreGoalsSection.vue` (decomposto pelo AC-07). O **store** `stores/multistore.ts` é arquivo distinto e não é tocado pelo AC-07 — PODE ser testado.
- NÃO alterar NENHUM arquivo de código de produção. Se um teste revelar comportamento estranho (ex.: `formatPrice('1990', null)` → `'-'`), escrever o teste como **caracterização** do comportamento atual com comentário `// caracterização: ...` — o fix é outro AC.
- NÃO mexer em `web/vitest.config.ts` nem `web/test/setup.ts` (já suportam tudo).
- NÃO adicionar dependência npm nem rodar `npm install` (lock do web é cross-platform, gerado no container).
- NÃO tocar em `web/app/utils/calendar.ts`, `web/app/components/roadmap/roadmap-data.ts` nem arquivos de calendário (working tree ativo de outra frente).

## 3. Regras de execução (obrigatórias)

1. **NENHUM comando git** (sessão multi-agente; só o usuário roda git).
2. **NÃO rodar npm/vitest/build/generate** — escrever os testes e **PARAR**. A validação fica listada na §6 para o usuário aprovar e rodar. Não há mudança em `back/`, então **não há build de api** (`docker compose up -d --build api` não se aplica a este AC).
3. Máx **450 linhas por arquivo** de teste (se um arquivo estourar, dividir — já previsto no §4.2).
4. Não remover nenhum teste existente — só adicionar (casos coexistem; não remover funcionalidade).
5. Zero mock de dado de negócio novo fora dos testes; nada de localStorage/legado novo.
6. Sem migrations, sem env vars, sem portas neste AC (se houvesse migration seria SQL plano idempotente sem goose Down, próxima = 0187+ — não é o caso).
7. Ao final, atualizar `web/AGENT.md` (seção "Lint, format e testes") — ver §4.9.
8. Descrições de `describe/it` em **inglês**, seguindo os testes existentes (`auth.test.ts`, `permissions.test.ts`).

### Padrões técnicos (valem para todos os arquivos)

- Imports relativos ao próprio diretório (`./permissions`, `./auth`), como nos testes existentes.
- Mock global de `$fetch` vem de `web/test/setup.ts`, resetado em `beforeEach`. Recuperar com:
  ```ts
  function getFetchMock() {
    return (globalThis as any).$fetch as ReturnType<typeof vi.fn>
  }
  ```
- **Dedupe global de GET**: `web/app/utils/api-client.ts` mantém um `Map` de GETs in-flight em escopo de módulo. Sempre `await` todas as promises no próprio teste e preferir `fetchMock.mockImplementation(...)` (por path) a `mockReturnValueOnce` quando a ação dispara vários GETs.
- **Moeda pt-BR**: `Intl` separa `R$` do valor com NBSP (` `). Todo assert de moeda usa:
  ```ts
  const money = (value: string) => value.replace(/ /g, ' ')
  // ex.: expect(money(formatCurrency(123456))).toBe('R$ 1.234,56')
  ```
- **Datas**: usar strings ISO **sem timezone** (`'2026-05-21T14:30:00'` = hora local) para o resultado não depender do TZ da máquina/CI. Nunca assertar hora derivada de timestamp UTC (`Z`), nem os buckets `hourly` do chartData de reports.
- **Stores autenticadas**: helper local (duplicar nos 3 arquivos de store — ~10 linhas, não criar util compartilhado):
  ```ts
  import { useAuthStore } from './auth'
  function authenticateSession(partial: Record<string, unknown> = {}) {
    const auth = useAuthStore()
    auth.accessToken = 'test-token'
    auth.user = { id: 'user-1', name: 'Teste' } as any
    auth.principal = { role: 'owner', permissions: [], permissionsResolved: true } as any
    auth.hydrated = true            // ensureSession() curto-circuita (auth.ts:405-408)
    auth.activeTenantId = 'tenant-1'
    Object.assign(auth, partial)
    return auth
  }
  ```
  Funciona porque `isAuthenticated = Boolean(accessToken && user && principal)` (`auth.ts:144-146`), `ensureSession` retorna cedo quando `hydrated` (`auth.ts:405-407`), e `runtime.ensure()` (`app-runtime.ts:39-51`) é 100% local (não faz fetch).
- Erros de API no formato que `getApiErrorMessage` (`api-client.ts`) lê:
  ```ts
  const httpError = (status: number, message: string) =>
    Object.assign(new Error(message), { statusCode: status, data: { error: { message } } })
  ```

## 4. Mudanças (passo a passo)

### 4.1 EDITAR `web/app/domain/utils/permissions.test.ts` (existente, 88 linhas — manter os 7 casos)

Adicionar os describes abaixo (funções/linhas em `permissions.ts`):

**`describe('role normalization and labels')`**
- `normalizeAppRole` (l.449): `'admin'` → `'platform_admin'` (alias `ROLE_ALIAS` l.1-3); `''`/`undefined` → `'consultant'`; `' owner '` → `'owner'`.
- `getRoleLabel` (l.17): `'manager'` → `'Gerente'`; `'admin'` → `'Admin da plataforma'`; `'store_terminal'` → `'Acesso da loja'`; `'papel_custom'` → `'papel_custom'`; **caracterização**: `getRoleLabel('')` → `'Consultor'` (normalize default consultant torna o branch `'Sem papel'` inalcançável para vazio).

**`describe('permission key helpers')`**
- `normalizePermissionKeys` (l.454): não-array → `[]`; `[' a ', '', null]` → `['a']`.
- `hasPermission` (l.460): chave vazia → `false`; lista `null` → `false`; `([' a '], 'a')` → `true`.

**`describe('workspace access state read/write')`**
- `getWorkspaceAccessOptions(getWorkspaceAccessDefinition('campanhas'))` → values `['none','view','edit']`; com `{ includeInherit: true }` inclui `'inherit'` primeiro; para `'relatorios'` (editPermission vazio) → sem `'edit'`.
- `readWorkspaceAccessState`: def `'campanhas'` + `['workspace.campanhas.view','workspace.campanhas.edit']` → `'edit'`; só view → `'view'`; `[]` → `'none'`; def `'tasks'` (sem viewPermission) → retorna o `fallbackState` passado (`'inherit'`).
- `writeWorkspaceAccessState` idempotente: aplicar `'edit'` duas vezes não duplica chaves; `'none'` remove as chaves do workspace preservando as demais.

**`describe('capability functions')`** (assinatura `(role, permissionKeys, permissionsResolved)`)
- `canManageSettings` (l.653): `('platform_admin', [], true)` → `true` (bypass `SUPERUSER_ROLES` = o lado `isPlatformAdmin` do padrão `isPlatformAdmin || has(...)`); `('owner', ['workspace.configuracoes.edit'], true)` → `true`; `('owner', ['queue.settings.manage'], true)` → `true`; `('owner', [], true)` → `false`; legacy: `('owner', [], false)` → `true`, `('manager', [], false)` → `false`.
- `canManageConsultants` (l.679): mesmo shape (`workspace.configuracoes.edit` OU owner/platform_admin + `queue.consultants.manage`).
- `canViewConsultants` (l.696): resolved `['queue.consultants.manage']` → `true`; `[]` → `false`; legacy `store_terminal` → `true`, `manager` → `false`.
- `canManageCrmCommercialPolicy` (l.670): só por papel — `'director'` e `'platform_admin'` → `true`; `'owner'` → `false` mesmo com todas as chaves.

### 4.2 CRIAR `web/app/domain/utils/permissions-workspaces.test.ts` (novo — matriz role × workspace)

Arquivo separado para não estourar 450 linhas no `permissions.test.ts`. Importa
`getAllowedWorkspaces` de `./permissions`. Casos (verificados contra `ROLE_WORKSPACES` l.307-428,
`SUPERUSER_ROLES` l.430, filtro l.624-651, aliases l.495-545):

1. **Legacy mode (permissionsResolved=false) → defaults do papel**:
   - `getAllowedWorkspaces('consultant')` → `['operacao']`
   - `getAllowedWorkspaces('store_terminal')` → contém `'operacao'`,`'consultor'`,`'relatorios'`,`'alertas'` (não assertar igualdade exata do array — usar `arrayContaining`)
   - `getAllowedWorkspaces('papel_custom_desconhecido')` → `['operacao']` (fallback consultant — fail-closed)
   - `getAllowedWorkspaces('admin')` → contém `'manage'` (alias admin→platform_admin)
2. **platform_admin bypass**: `getAllowedWorkspaces('platform_admin', [], true)` `toEqual` `getAllowedWorkspaces('platform_admin')` (lista completa mesmo com zero permissões — superuser ignora o filtro fino; contrato `isPlatformAdmin || has(...)`).
3. **Fail-closed com permissões resolvidas**:
   - `getAllowedWorkspaces('consultant', [], true)` → `[]` (nem `'operacao'`)
   - `getAllowedWorkspaces('consultant', ['workspace.operacao.view'], true)` → `['operacao']`
4. **Workspace de módulo por prefixo** (`MODULE_WORKSPACE_PERMISSION_PREFIXES = { tasks: 'tasks.' }` l.437-439):
   - `getAllowedWorkspaces('consultant', ['tasks.boards.manage'], true)` → contém `'tasks'`
5. **Aliases por papel** (owner, resolved=true, `[]`) — usar `arrayContaining`/`not.toContain`:
   - contém `'tasks'` (sem viewPermission + default do owner)
   - NÃO contém `'campanhas'`, `'usuarios'`, `'clientes'`, `'relatorios'` (viewPermission sem chave e sem alias)
6. **cardapio_web por permissão fina**: `getAllowedWorkspaces('manager', ['cardapio.view'], true)` contém `'cardapio_web'`; `getAllowedWorkspaces('manager', [], true)` não contém. (Confirmar o nome do workspace e da chave inspecionando `ROLE_WORKSPACES`/aliases antes de assertar; se o par não existir no código atual, remover este item — não inventar.)

### 4.3 EDITAR `web/app/domain/utils/reports.test.ts` (existente, 58 linhas — manter o caso atual)

Referências: `normalizeReportFilters` l.358-377, `buildReportData` l.379-641 (metrics l.623-640;
conversão l.454 `outcome === 'compra' || 'reserva'`), `buildReportRowsFromApi`. Adicionar:

- `normalizeReportFilters({})` → defaults (`dateFrom: ''`, `consultantIds: []`, `outcomes: []`, `minSaleAmount: ''`, `search: ''` etc.); legado singular `{ consultantId: 'c1', outcome: 'compra' }` → `consultantIds: ['c1']`, `outcomes: ['compra']`; array passa direto (`{ outcomes: ['reserva'] }` → `['reserva']`).
- `buildReportData({})` → `rows: []`, `metrics.totalAttendances: 0`, `metrics.conversions: 0`, `metrics.conversionRate: 0`, `metrics.soldValue: 0`.
- **Ordenação**: entries com `finishedAt: Date.parse('2026-05-21T10:00:00')` e `('2026-05-21T15:00:00')` → `rows[0]` é o mais recente (sort desc l.400).
- **Reserva conta como conversão** (l.454): `finishOutcome: 'reserva'`, `saleAmount: 100` → `metrics.conversions: 1`, `soldValue: 100`.
- **Não-compra não soma venda**: `finishOutcome: 'nao-compra'`, `saleAmount: 999` → `soldValue: 0`.
- **Fallbacks de row** (l.419-451): entry só com `{ finishedAt: Date.parse('2026-05-21T10:00:00') }` → `outcome: 'nao-compra'`, `outcomeLabel: 'Nao compra'`, `startModeLabel: 'Na vez'`, `saleAmount: 0`.
- **Label fallback pro id cru**: `visitReasons: ['vr-x']` sem option correspondente → `visitReasonsLabel: 'vr-x'`.
- **Filtro exclui**: history com `finishOutcome: 'nao-compra'` + `filters: { outcomes: ['compra'] }` → `rows: []`; `filters: { search: 'zzz' }` sem match → `rows: []`.
- `buildReportRowsFromApi`: row com `visitReasons: ['vr-1']` + options `[{ id: 'vr-1', label: 'Noiva' }]` → `visitReasonsLabel: 'Noiva'`.
- NÃO assertar `chartData.hourly` (bucket por `getHours()` — sensível a TZ) nem labels de moeda dos metrics.

### 4.4 CRIAR `web/app/domain/utils/color.test.ts` (novo)

Importar de `./color`. Casos (verificados contra o código):

- `clampPercent` (l.33): `-5` → `0`; `150` → `100`; `49.6` → `50`.
- `clampAngle` (l.37): `NaN` → `180`; `-90` → `270`; `450` → `90`; `360` → `0`.
- `normalizeHex` (l.46): `'#abc'` → `'#AABBCC'`; `'aabbccdd'` → `'#AABBCC'` (alpha descartado); `'#ab'` → `null`; `'zz'` → `null`.
- `parseHexColor` (l.67): `'#336699'` → `{ hex: '#336699', alpha: 100, hasAlpha: false }`; `'#33669980'` → `{ hex: '#336699', alpha: 50, hasAlpha: true }`; `'xyz'` → `null`.
- `parseRgbChannel` (l.95): `'50%'` → `128`; `'300'` → `255`; `'-5'` → `0`; `'abc'` → `null`; `''` → `null`.
- `parseAlphaChannel` (l.118): `undefined` → `{ alpha: 100, hasAlpha: false }`; `'0.5'` → `{ alpha: 50, hasAlpha: true }`; `'50%'` → `{ alpha: 50, hasAlpha: true }`; `'75'` → `{ alpha: 75, hasAlpha: true }`.
- `channelsToHex(255, 0, 128)` → `'#FF0080'`; `hexToRgbChannels('#FF0080')` → `[255, 0, 128]`; `hexToRgbChannels('nope')` → `null`.
- `buildCssColor` (l.458): `('#336699', false, 50)` → `'#336699'`; `('#336699', true, 50)` → `'rgba(51, 102, 153, 0.5)'`; `('invalid', true, 50)` → `'#000000'`.
- `buildGradientValue` (l.479): linear `angle: 450` → `'linear-gradient(90deg, ...)'`; `type: 'radial'` → `'radial-gradient(circle, ...)'` (ignora ângulo); `type: 'conic'`, `angle: 0` → `'conic-gradient(from 0deg, ...)'`.
- `parseGradient` (l.400): `'linear-gradient(90deg, #fff, #000)'` → `{ type: 'linear', angle: 90 }` com `start.hex '#FFFFFF'`, `end.hex '#000000'`; sem ângulo (`'linear-gradient(#fff, #000)'`) → `angle: 180`; `'#fff'` → `null`.
- `splitByTopLevelComma('rgba(0,0,0,0.5), #fff')` (l.321) → 2 tokens (vírgula dentro de parênteses não quebra).
- Round-trip: `parseGradient(buildGradientValue({ type:'linear', angle:90, start:{hex:'#FFFFFF',alphaEnabled:false,alpha:100}, end:{hex:'#000000',alphaEnabled:false,alpha:100} }))` preserva `type`/`angle`/`start.hex`/`end.hex`.

### 4.5 CRIAR `web/app/domain/utils/erp-display.test.ts` (novo)

Importar de `./erp-display`. Usar o helper `money` (NBSP). Casos:

- `formatCurrency` (l.545): `0` e `null` → `'R$ 0,00'`; `123456` → `'R$ 1.234,56'`.
- `formatNumber` (l.583): `null` → `'0'`; `1234567` → `'1.234.567'`.
- `formatDateTime` (l.555): `''`/`null` → `'-'`; `'not-a-date'` → `'not-a-date'` (retorna cru); `'2026-05-21T14:30:00'` (LOCAL, sem Z) → `toContain('2026')`, `toContain('14:30')`, `toContain(' as ')` — não assertar a string completa (formato do ICU varia entre versões de Node).
- `formatSourceFileName` (l.588): `null` → `'-'`; `'produtos.csv'` → `'produtos.csv'`; `'20260521143000_produtos.csv'` → `toContain('14:30')` (timestamp do nome vira Date local — determinístico).
- `formatPrice` (l.598): `(undefined, 1990)` → `money → 'R$ 19,90'`; `('1990', undefined)` → `'R$ 19,90'` (`Number(undefined)` é NaN → usa rawValue); **caracterização**: `('1990', null)` → `'-'` (`Number(null)` = 0, finito, vence o rawValue); `(undefined, 0)` → `'-'`.
- `productRowKey({ sku: 'A', identifier: 'B' })` → `'A-B'`; `recordsRowKey`: `{}` com index `3` → `'3'`; `{ order_id: 'o1' }` → `'o1'`; `{ id: 'i1', order_id: 'o2' }` → `'i1'` (cadeia de fallback l.614-616).
- **Consistência declarativa**: para cada key de `ERP_RECORDS_COLUMNS_BY_TAB` (l.288) existe entrada em `ERP_RECORDS_DATA_TYPE_BY_TAB` (l.356) e `ERP_RECORDS_LABEL_BY_TAB` (l.363); cada lista de colunas tem `id` únicos.

### 4.6 EDITAR `web/app/stores/settings.test.ts` (existente, 19 linhas — manter o caso atual)

Usar `authenticateSession` + `getFetchMock`. Referências em `settings.ts`: `mutateAndPersist`
l.275-307 (fail-fast tenant l.288-290, rollback l.299-306), `persistOperationPatch` l.81-102
(path `/v1/settings/operation` via `settingsPath`), `updateSetting` l.572-585.

- **Tenant fail-fast**: sessão autenticada mas `activeTenantId: ''` e `tenantContext: []` → `store.updateSetting('queueLimit', 5)` resolve `{ ok: false, message: 'Tenant ativo nao identificado para a sessao.' }` e `$fetch` NÃO é chamado.
- **Happy path**: sessão com `activeTenantId: 'tenant-1'`; `fetchMock.mockResolvedValue({})`; `await store.updateSetting('queueLimit', 5)` → `result.ok === true` (não deep-equal — `runtime.run` pode devolver payload); `$fetch` chamado 1 vez; `fetchMock.mock.calls[0][0]` `toContain('/v1/settings/operation')` e `toContain('tenantId=tenant-1')`; `calls[0][1].method === 'PATCH'`; `calls[0][1].body.settings.queueLimit === 5`.
- **Rollback quando a persistência falha** (l.299-306): `const before = JSON.parse(JSON.stringify(useAppRuntimeStore().state))`; `fetchMock.mockRejectedValue(httpError(500, 'boom'))`; resultado → `{ ok: false, message: 'boom' }` e `JSON.parse(JSON.stringify(useAppRuntimeStore().state))` `toEqual(before)` (revertido por `runtime.hydrate(previousState)`).

### 4.7 CRIAR `web/app/stores/multistore.test.ts` (novo)

`setActivePinia(createPinia())` em `beforeEach`, `authenticateSession`, `getFetchMock`.
Referências: `normalizeStore` l.24-40, `refreshManagedStores` l.227-254, `refreshOverview`
l.256-295 (degradação 403 l.283-289), `createStore` l.297-352, guards de sessão l.300/357/408/464.

- **Fail-fast sem sessão** (sem `authenticateSession`): `createStore({})`, `updateStore('s1', {})`, `archiveStore('s1')`, `deleteStore('s1')` → todos `{ ok: false, message: 'Sessao indisponivel.' }` e `$fetch` nunca chamado.
- **`refreshManagedStores` normaliza**: autenticado; `fetchMock.mockImplementation((path) => path.includes('/v1/stores') ? { stores: [{ id: 's1', code: 'ab', storeType: 'SHOPPING', monthlyGoal: -5 }, { id: 's2', storeType: 'mall' }] } : {})`; resultado: `code: 'AB'` (uppercase), `storeType: 'shopping'`, `monthlyGoal: 0` (clamp ≥0), `isActive: true` (default), `s2.storeType: 'bairro'` (desconhecido → default). Assert também que o path contém `tenantId=tenant-1` e `includeInactive=true`.
- **`refreshOverview` degrada 403 em silêncio**: `fetchMock.mockRejectedValue(httpError(403, 'forbidden'))` → resolve `null` (não lança), `overview.value === null`, `errorMessage.value === ''`, `ready.value === false`.
- **Erro não-403 sobe**: `httpError(500, 'db down')` → `await expect(store.refreshOverview()).rejects` e `errorMessage.value` contém `'db down'`.
- **`createStore` valida payload**: autenticado; mock de GETs (`{ stores: [] }` para `/v1/stores`, `{}` para o resto — `ensureLoaded` dispara refreshes antes da validação); `createStore({ name: 'Loja' })` (sem code) → `{ ok: false, message: 'Preencha nome, codigo e tenant da loja.' }` e nenhuma chamada com `method: 'POST'` a `/v1/stores`.

### 4.8 CRIAR `web/app/stores/cardapio.test.ts` (novo)

`authenticateSession` (só para o `createApiRequest` ter token) + `getFetchMock`. A store NÃO chama
`runtime.ensure`/`ensureSession` — basta o mock. Referências: `asArray` l.40-46,
`withScope`/`withScopeFor` l.59-75, `loadRestaurants` l.115-134, `createRestaurant` l.136-156,
`loadRestaurant` l.163-192, `loadOrders` l.454-487, `updateOrderStatus` l.489-502
(PATCH em `/v1/cardapio/orders/{id}`), `resetActive` l.101-111.

- **`loadRestaurants` shapes**: resposta `{ restaurants: [r1] }` → `restaurants.value` 1 item; array cru `[r1, r2]` → 2 itens (`asArray`); com `{ accountId: 'acc-1', q: 'pizza' }` o path contém `accountId=acc-1` e `q=pizza`.
- **`loadRestaurants` erro**: `mockRejectedValue(httpError(500, 'down'))` → `listError.value === 'down'`, `listPending.value === false`, não lança.
- **`loadRestaurant` com escopo**: `fetchMock.mockImplementation((path) => { if (path.includes('/categories')) return { categories: [{ id: 'c1' }] }; if (path.includes('/products')) return { products: [] }; if (path.includes('/domains')) return { domains: [{ host: 'a.com', isPrimary: true }] }; if (path.includes('/delivery-zones')) return { deliveryZones: [] }; return { id: 'r1', name: 'Rest' } })`; `await store.loadRestaurant('r1', 'acc-9')` → 5 chamadas, **todas** com `accountId=acc-9` no path; `restaurant.value.id === 'r1'`; `categories.value` 1 item; `primaryDomain.value === 'a.com'`; `detailError.value === ''`.
- **`resetActive` limpa o escopo**: após o caso acima, `resetActive()` → `restaurant.value === null`; nova `loadRestaurants()` NÃO leva `accountId` na query.
- **`loadOrders` shapes**: array cru `[o1, o2]` → `orders.value.total === 2`; `{ orders: [o1], total: 42 }` → `total === 42`, `items` 1; path contém `page=1` e `perPage=20`; com `{ status: 'pending' }` contém `status=pending`; erro → `ordersError.value` setado sem lançar.
- **`updateOrderStatus` substitui in-place**: semear via `loadOrders` (mock `{ orders: [{ id: 'o1', status: 'pending' }], total: 1 }`), depois mock do PATCH devolvendo `{ id: 'o1', status: 'done' }` → `orders.value.items[0].status === 'done'`.
- **`createRestaurant` erro** → `{ ok: false, message: 'Slug ja existe' }` com `httpError(409, 'Slug ja existe')`.

### 4.9 EDITAR `web/AGENT.md`

Na seção `## Lint, format e testes (Fase 6 do PLANO_REFATORACAO)` (linha ~213), acrescentar um
parágrafo curto:

- Onda 1 de testes (AC-15, 2026-07): cobertura de `domain/utils/permissions` (matriz role×workspace + fail-closed + bypass platform_admin), `reports`, `color`, `erp-display` e das stores `settings`/`multistore`/`cardapio` com `$fetch` global mockado (`web/test/setup.ts`). Testes determinísticos: sem rede, sem timers reais, datas sem timezone, moeda normalizando NBSP. `stores/erp.ts` ficou de fora de propósito (backlog de refatoração AC-07). Rodar com `docker compose run --rm web npm run test`.

## 5. Critérios de aceite (verificáveis)

1. Existem os 5 arquivos novos: `permissions-workspaces.test.ts`, `color.test.ts`, `erp-display.test.ts` (em `web/app/domain/utils/`), `multistore.test.ts`, `cardapio.test.ts` (em `web/app/stores/`).
2. Os 3 arquivos expandidos (`permissions.test.ts`, `reports.test.ts`, `settings.test.ts`) mantêm 100% dos casos pré-existentes.
3. Todos os casos listados no §4 estão implementados (checklist um a um).
4. Nenhum arquivo fora da lista do §8 foi alterado; nenhum arquivo de produção mudou. Em especial: `stores/erp.ts` e `MultiStoreGoalsSection.vue` intactos.
5. Nenhum teste usa rede real, `setTimeout` real, `Date.now()` sem controle, ou string ISO com `Z`/offset em assert de formatação local.
6. Cada arquivo de teste tem ≤ 450 linhas.
7. `web/AGENT.md` atualizado conforme §4.9.
8. Comportamentos estranhos preservados estão marcados com comentário `// caracterização:`.

## 6. Validação (aguardar aprovação do usuário — NÃO rodar sem "aprovei")

O implementador escreve os testes e **PARA**. Comandos para o usuário rodar:

```bash
# canônico (container — regra do lock cross-platform do web):
docker compose run --rm web npm run test

# alternativa no host (só executa vitest, não instala nada):
npm --prefix web run test

# lint dos arquivos novos:
npm --prefix web run lint
```

Se algum assert divergir do comportamento real (ex.: formato exato do ICU no Node do container),
o ajuste é **no teste** (caracterizar o real), nunca no código de produção. O usuário reporta a
saída e o ajuste é feito em novo ciclo.

## 7. Notas de Deploy

Nenhuma: sem migration, sem env var, sem rebuild de api, sem mudança de Dockerfile/deps/portas.
Atenção apenas: `npm run test` é gate do CI (`build-images.yml`) — depois do merge, teste
quebrando bloqueia o build de imagem (por isso a regra de asserts robustos do §3).

## 8. Arquivos tocados (lista completa)

| Arquivo | Ação |
| --- | --- |
| `web/app/domain/utils/permissions.test.ts` | editar (expandir) |
| `web/app/domain/utils/permissions-workspaces.test.ts` | criar |
| `web/app/domain/utils/reports.test.ts` | editar (expandir) |
| `web/app/domain/utils/color.test.ts` | criar |
| `web/app/domain/utils/erp-display.test.ts` | criar |
| `web/app/stores/settings.test.ts` | editar (expandir) |
| `web/app/stores/multistore.test.ts` | criar |
| `web/app/stores/cardapio.test.ts` | criar |
| `web/AGENT.md` | editar (nota na seção de testes) |

Fora de escopo deliberado: `web/app/stores/erp.ts` e qualquer teste dele (backlog AC-07);
`MultiStoreGoalsSection.vue` (decomposto pelo AC-07); `roadmap-data.ts` e arquivos de calendário
(working tree ativo de outras frentes).
