# AC-12 — Finance na API Go real: eliminar o último mock do BFF Nitro

Prioridade P1 · esforço M · achado canônico AC-12 do diagnóstico 2026-07-02.

## 1. Contexto

O ADR `docs/adr/0002-remove-bff-nitro-mock.md` (status "Aceito", 2026-05-29) mandou
remover o BFF Nitro inteiro. Restaram exatamente **10 arquivos**, todos do módulo
Finance (evidência: `web/server/**`):

- 9 handlers Nitro: `web/server/api/admin/finance-config/{index.get,index.put,recurring-clients.get}.ts`,
  `web/server/api/admin/finance-sheets/{index.get,index.post,[id].get,[id].put,[id].delete}.ts`,
  `web/server/api/admin/finance-sheets/[id]/lines/[lineId].patch.ts`
- 1 store in-memory: `web/server/utils/financeMockStore.ts` (446 linhas) — dados somem
  a cada restart, só funciona em dev/SSR.

Quem consome: `web/layers/finance/composables/useFinancesManager.ts` (linhas 56, 279,
307, 334, 367, 405, 431), `useFinancesConfigManager.ts` (linhas 21, 53, 95) e
`useFinanceConfigEditor.ts` (linha 166) — todos via `$fetch` para `/api/admin/finance-*`.

O back real **já está desenhado** em `docs/finance/PLANO_MODULO_FINANCE.md`
(2026-06-30) e registrado como legado em `docs/LEGADO.md` #6 (linha 164+). Esta spec
materializa aquele plano com as correções de contexto atual: migration **0187** (última
é `0186_calendar_ai_plans.sql`) e rotas **`/v1/finance/*`** (decisão desta spec — casa
com o padrão de gating por prefixo `moduleGatingRules()`; o plano antigo dizia
`/v1/admin/finance-*` e será atualizado).

## 2. Objetivo e não-objetivos

**Objetivo:** módulo Go `finance` completo (schema `finance.*`, permissões no catálogo
core, endpoints `/v1/finance/*` espelhando 1:1 o contrato do mock), front da layer
finance apontando pra API Go com `createApiRequest` + `X-Account-Id`, `web/server/`
apagado, gating de workspace/módulo restaurado, docs sincronizados.

**Não-objetivos (NÃO fazer):**
- NÃO seed de dados — planilhas/config começam vazias (zero mock).
- NÃO realtime/WebSocket para finance (fica para fase futura; anotar no AGENT.md).
- NÃO integração com módulo `contacts` (não existe ainda; não declarar
  `OptionalModules` no Metadata para não criar referência morta).
- NÃO refatorar `useFinanceSheetEditor.ts` (960 linhas) nem quebrar arquivos >450
  existentes da layer — isso é AC-07. Aqui as edições nos composables são cirúrgicas
  e não podem AUMENTAR linhas líquidas de `useFinancesManager.ts` (485 hoje).
- NÃO mudar layout/UX da página `/finance` (só remover o `LegacyMarker`).
- NÃO habilitar `finance` em `core.account_modules` por migration — opt-in por conta
  via painel admin (platform_admin tem bypass do guard e já enxerga).

## 3. Mudanças

### 3.1 Migration `back/internal/platform/database/migrations/0187_finance_module.sql` (criar)

SQL plano idempotente, **SEM** `-- +goose Down` (migrator roda o arquivo inteiro).
Derivado do shape real de `financeMockStore.ts` + `docs/finance/PLANO_MODULO_FINANCE.md` §3.
`account_id uuid not null` em TODAS as tabelas raiz (multi-tenant dia 1):

```sql
create schema if not exists finance;

create table if not exists finance.sheets (
    id             uuid primary key default gen_random_uuid(),
    account_id     uuid not null references core.accounts(id) on delete cascade,
    core_tenant_id text not null default '',
    title          text not null default '',
    period         text not null default '',
    status         text not null default 'aberta',
    notes          text not null default '',
    created_at     timestamptz not null default now(),
    updated_at     timestamptz not null default now()
);
create index if not exists finance_sheets_account_scope_idx
    on finance.sheets (account_id, core_tenant_id, period);
create index if not exists finance_sheets_account_updated_idx
    on finance.sheets (account_id, updated_at desc);

create table if not exists finance.lines (
    id                uuid primary key default gen_random_uuid(),
    sheet_id          uuid not null references finance.sheets(id) on delete cascade,
    kind              text not null check (kind in ('entrada','saida')),
    description       text not null default '',
    category          text not null default '',
    effective         boolean not null default false,
    effective_date    date,
    amount            numeric(14,2) not null default 0,
    adjustment_amount numeric(14,2) not null default 0,
    fixed_account_id  text not null default '',
    details           text not null default '',
    position          int not null default 0
);
create index if not exists finance_lines_sheet_idx on finance.lines (sheet_id, kind, position);

create table if not exists finance.line_adjustments (
    id       uuid primary key default gen_random_uuid(),
    line_id  uuid not null references finance.lines(id) on delete cascade,
    amount   numeric(14,2) not null default 0,
    note     text not null default '',
    date     date,
    position int not null default 0
);
create index if not exists finance_line_adjustments_line_idx
    on finance.line_adjustments (line_id, position);

create table if not exists finance.categories (
    id             uuid primary key default gen_random_uuid(),
    account_id     uuid not null references core.accounts(id) on delete cascade,
    core_tenant_id text not null default '',
    name           text not null default '',
    kind           text not null default 'ambas' check (kind in ('entrada','saida','ambas')),
    description    text not null default '',
    position       int not null default 0
);
create index if not exists finance_categories_scope_idx
    on finance.categories (account_id, core_tenant_id);

create table if not exists finance.fixed_accounts (
    id             uuid primary key default gen_random_uuid(),
    account_id     uuid not null references core.accounts(id) on delete cascade,
    core_tenant_id text not null default '',
    name           text not null default '',
    kind           text not null default 'ambas' check (kind in ('entrada','saida','ambas')),
    category_id    text not null default '',
    default_amount numeric(14,2) not null default 0,
    notes          text not null default '',
    position       int not null default 0
);
create index if not exists finance_fixed_accounts_scope_idx
    on finance.fixed_accounts (account_id, core_tenant_id);

create table if not exists finance.fixed_account_members (
    id               uuid primary key default gen_random_uuid(),
    fixed_account_id uuid not null references finance.fixed_accounts(id) on delete cascade,
    name             text not null default '',
    amount           numeric(14,2) not null default 0,
    position         int not null default 0
);
create index if not exists finance_fixed_account_members_idx
    on finance.fixed_account_members (fixed_account_id, position);

create table if not exists finance.recurring_entries (
    id                     uuid primary key default gen_random_uuid(),
    account_id             uuid not null references core.accounts(id) on delete cascade,
    core_tenant_id         text not null default '',
    source_core_tenant_id  text not null default '',
    adjustment_amount      numeric(14,2) not null default 0,
    notes                  text not null default ''
);
create index if not exists finance_recurring_entries_scope_idx
    on finance.recurring_entries (account_id, core_tenant_id);

create table if not exists finance.config_state (
    account_id     uuid not null references core.accounts(id) on delete cascade,
    core_tenant_id text not null default '',
    updated_at     timestamptz not null default now(),
    primary key (account_id, core_tenant_id)
);
```

Decisões fechadas (divergências conscientes do plano antigo, para espelhar o mock 1:1):
- `fixed_account_id`/`category_id` como **text** (não uuid/FK): o front usa ids
  determinísticos de recorrência (`finance-ids.ts`) e o mock trata como texto snapshot.
- SEM unique de `lower(name)` em categories e SEM unique em recurring_entries: o mock
  não valida duplicata e o PUT de config é full-replace — unique quebraria o autosave.
- `config_state` guarda o `updatedAt` do payload de config (o mock devolve esse campo).

### 3.2 Módulo Go `back/internal/modules/finance/` (criar — padrão do módulo `site`)

Todos os arquivos ≤450 linhas. IDs sempre `string` + cast `::uuid` no SQL (NUNCA lib
uuid externa; id novo = `gen_random_uuid()` via `coalesce(nullif($n,'')::uuid, gen_random_uuid())`,
validando formato com regexp `^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`
antes do cast — id inválido vira ''). Scan de nullable com ponteiro (`*string`/`*int`).

**`module.go`** — copiar a estrutura de `back/internal/modules/site/module.go`:
```go
func (m *Module) ID() string { return "finance" }
func (m *Module) Metadata() modules.Metadata {
    return modules.Metadata{
        SchemaName: "finance", Label: "Finance",
        Description: "Planilhas financeiras mensais por cliente: entradas/saidas, efetivacao, ajustes, contas fixas, categorias e recorrencias.",
        IsCore: false, SortOrder: 50,
    }
}
```
`Permissions()` (formato `entity.verb`, Scope "account"): `finance.sheets.view`,
`finance.sheets.manage`, `finance.config.view`, `finance.config.manage`,
`finance.recurring.manage`. `RoleTemplates()`: `finance.manager` (todas) e
`finance.viewer` (`finance.sheets.view` + `finance.config.view`). `Build(deps)` monta
`NewPostgresSheetStore(deps.Pool)`, `NewPostgresConfigStore(deps.Pool)`,
`NewService(sheets, config)` e devolve `handle{service, authMiddleware: deps.AuthMiddleware}`.
`RegisterEventHandlers` no-op; `Close` nil.

Decisão: enforcement por request = `RequireAuth` + gating de módulo no Chain (mesmo
estado dos módulos site/calendar/cardapio hoje — nenhum satélite aplica permissão fina
por rota ainda). O catálogo de permissões existe para o painel de papéis e para o
gating de workspace no front (prefixo `finance.`).

**`model.go`** — structs espelhando `web/layers/finance/types/finances.ts` (json
camelCase idêntico): `Adjustment{ID, Amount float64, Note, Date string}`,
`Line{ID, Kind, Description, Category string, Effective bool, EffectiveDate string,
Amount, AdjustmentAmount float64, Adjustments []Adjustment, FixedAccountID, Details string}`
(tags: `effectiveDate`, `adjustmentAmount`, `fixedAccountId`), `Summary` (6 campos
float64: `expectedIn/effectiveIn/expectedOut/effectiveOut/expectedBalance/effectiveBalance`),
`SheetListItem{ID, Title, Period, Status, Notes, CoreTenantID, ClientName string,
Summary Summary, Preview, CreatedAt, UpdatedAt string}` (tag `coreTenantId`),
`SheetDetail{SheetListItem; Entradas, Saidas []Line}`, inputs
`SheetInput{Title, Period, Status, Notes *string; Entradas, Saidas []Line; CoreTenantID *string}`,
`LinePatchInput{Effective *bool; EffectiveDate *string}`, e os de config:
`Category{ID, Name, Kind, Description string}`, `FixedAccountMember{ID, Name string, Amount float64}`,
`FixedAccount{ID, Name, Kind, CategoryID string, DefaultAmount float64, Notes string, Members []FixedAccountMember}`,
`RecurringEntry{SourceCoreTenantID string \`json:"sourceCoreTenantId,omitempty"\`, AdjustmentAmount float64, Notes string}`,
`ConfigData{CoreTenantID string, Categories []Category, FixedAccounts []FixedAccount,
RecurringEntries []RecurringEntry, UpdatedAt string}`,
`RecurringClientStore{ID, Name string, Amount float64}`,
`RecurringClient{ID, CoreTenantID, Name string, MonthlyPaymentAmount float64,
PaymentDueDay string, BillingMode string, Stores []RecurringClientStore}`
(tags `monthlyPaymentAmount`, `paymentDueDay`, `billingMode` — shape exato que
`useFinanceConfigEditor.ts:155-171` espera). Slices sempre não-nil na serialização.

**`service.go`** — regras portadas de `financeMockStore.ts` (fonte da verdade do
comportamento):
- `normalizeLine`: limites text (description 260, category 120, note 240, date 10,
  fixedAccountId 90, details 600), amount ≥ 0 com 2 casas, adjustmentAmount = soma dos
  adjustments quando existirem (senão o valor recebido, negativo permitido), colapso de
  whitespace (`strings.Join(strings.Fields(s), " ")`).
- `computeSummary` (linha 160-175 do mock): total da linha = amount + adjustmentAmount;
  expected soma tudo, effective soma só `effective=true`; 2 casas.
- `computePreview` (linha 177-179): `fmt.Sprintf("Entradas %s | Saidas %s | Saldo %s", f(v))`
  com `f = strconv.FormatFloat(v, 'f', -1, 64)` (mesmo output da interpolação JS).
- defaults do create: title `"Finance " + period`, period = mês corrente `YYYY-MM`,
  status `"aberta"` (linhas 298-319 do mock).
- Escopo: `accountID` SEMPRE do request (nunca do body); `coreTenantId` é filtro
  normalizado (trim+lower) dentro da account. Sheet de outra account → **404** (não 403).
- `clientName`: lookup `core.accounts.name` quando `core_tenant_id` é uuid de account;
  senão `""` (o mock devolvia "Cliente demo (mock)" — substituído por dado real).
- Datas `effective_date`/`date`: string `YYYY-MM-DD` no contrato; `""` ⇄ NULL no banco.

**`store_sheets.go`** — pgx: `List(ctx, accountID string, f ListFilter)` (WHERE
`account_id=$1::uuid` + core_tenant_id/period/q ILIKE em title, ORDER BY updated_at
DESC, LIMIT/OFFSET + `count(*)` para meta), `Get`, `Create`, `Update` (full-replace:
transação — UPDATE sheet, DELETE lines cascade, reinsere lines+adjustments com
`position` = índice do array), `Delete`, `PatchLine` (UPDATE só
`effective`/`effective_date`, `effective=false` limpa a data — mock linha 355-361 — e
toca `sheets.updated_at`), `LoadLines`. `clientName` via
`left join core.accounts a on s.core_tenant_id <> '' and a.id::text = s.core_tenant_id`.

**`store_config.go`** — `GetConfig` (3 selects por scope, ordenado por position),
`SaveConfig` (transação full-replace: DELETE por `(account_id, core_tenant_id)` e
reinsere categories/fixed_accounts+members/recurring_entries; upsert em
`config_state.updated_at = now()`), `ListRecurringClients(ctx, isPlatformAdmin bool)`:
```sql
select a.id::text, a.name, a.monthly_payment_amount, a.payment_due_day, a.billing_mode
from core.accounts a
where a.is_active = true
  and (a.monthly_payment_amount > 0 or a.billing_mode = 'per_store')
order by lower(a.name)
```
- stores por conta: `select id, name, coalesce(billing_amount,0) from queue.stores
  where tenant_id = $1::uuid and coalesce(billing_amount,0) > 0 order by lower(name)`
  (mesma fonte de `core/admin_repository_aggregates.go:135-162`).
- `payment_due_day` é `*int` no scan (nullable) → string `""` ou `"12"`.
- **Isolamento:** só `platform_admin` recebe a lista completa (é o caso de uso real —
  agência acompanhando mensalidades dos clientes); caller comum recebe `[]` (idêntico
  ao mock hoje → zero regressão, zero vazamento cross-tenant). O handler decide via
  `auth.PrincipalFromContext(r.Context())` + `principal.Role == auth.RolePlatformAdmin`.

**`http.go`** — padrão `site/http_admin.go` (wrap `middleware.RequireAuth`, helper
`accountIDFromContext` copiado de `site/http_admin.go:57-67`, `httpapi.ReadJSON` com
DisallowUnknownFields, `httpapi.WriteJSON`/`WriteError`):
```
GET    /v1/finance/sheets                    → {status:"success", data:[SheetListItem], meta:{page,limit,total,totalPages,hasMore}}
POST   /v1/finance/sheets                    → {status:"success", data:SheetDetail}
GET    /v1/finance/sheets/{id}               → {status:"success", data:SheetDetail} | 404 sheet_not_found
PUT    /v1/finance/sheets/{id}               → {status:"success", data:SheetDetail} | 404
DELETE /v1/finance/sheets/{id}               → {status:"success"} | 404
PATCH  /v1/finance/sheets/{id}/lines/{lineId}→ {status:"success", data:{sheetId,lineId,line,summary,preview,updatedAt}} | 404 line_not_found
GET    /v1/finance/config?coreTenantId=      → {status:"success", data:ConfigData}
PUT    /v1/finance/config                    → {status:"success", data:ConfigData}
GET    /v1/finance/config/recurring-clients  → {status:"success", data:[RecurringClient]}
```
Query da lista: `q`, `coreTenantId`, `period`, `page` (default 1), `limit` (default 240,
teto 500). Sem account no contexto → 403 `no_account`.

**`errors.go`** — `ErrSheetNotFound`, `ErrLineNotFound` (sentinelas p/ 404).

**`AGENT.md`** — descrever módulo, tabelas, rotas, decisões acima, pendências
(realtime, permissão fina por rota, integração contacts).

### 3.3 Registro no boot — `back/internal/platform/app/app.go` (editar)

1. Após `registry.MustRegister(cardapio.New())` (linha 358):
```go
// finance: planilhas financeiras mensais por cliente (substitui o mock BFF
// Nitro — ADR 0002). Painel em /v1/finance (gating abaixo).
// Plano: docs/finance/PLANO_MODULO_FINANCE.md.
registry.MustRegister(finance.New())
```
2. Em `moduleGatingRules()` (linha 458+), após a regra do cardapio:
```go
// finance (planilhas financeiras). platform_admin tem bypass; contas sem o
// modulo habilitado levam 403 module_disabled.
{Prefix: "/v1/finance", ModuleID: "finance"},
```
3. Import `finance` no bloco de imports dos módulos.
`SyncCatalog` (já chamado, linha 361) upserta `core.modules`/`core.permissions`/
`core.role_templates` automaticamente no boot.

### 3.4 Front — layer finance consome a API Go (editar 3 composables + página)

Padrão (igual `web/layers/tasks/composables/useTaskComments.ts:2-4,122`):
```ts
import { useAuthStore } from '~/stores/auth'
import { createApiRequest } from '~/utils/api-client'
// dentro do composable:
const auth = useAuthStore()
const runtimeConfig = useRuntimeConfig()
const apiRequest = createApiRequest(runtimeConfig, () => auth.accessToken)
```
`X-Account-Id` entra sozinho pelo provider global (`api-client.ts:248-251`, plugin
`account-id-bridge.client.ts`) — NÃO passar manualmente.

- **`useFinancesManager.ts`**: `FINANCE_SHEETS_API_BASE = '/v1/finance/sheets'`
  (linha 56); trocar cada `$fetch<T>(...)` (linhas 279, 307, 334, 367, 405, 431) por
  `(await apiRequest(path, opts)) as T` mantendo method/query/body idênticos; atualizar
  o comentário de cabeçalho (linhas 1-7, fonte agora é a API Go). Não crescer o arquivo
  (485 linhas hoje; teto 450 é assunto do AC-07).
- **`useFinancesConfigManager.ts`**: `FINANCE_CONFIG_API_BASE = '/v1/finance/config'`
  (linha 21); mesmo swap nas linhas 53 e 95; cabeçalho.
- **`useFinanceConfigEditor.ts`**: linha 155-171, trocar `$fetch(...)` por
  `apiRequest('/v1/finance/config/recurring-clients', { query: {...} })` (cast no
  mesmo tipo inline). Manter a query `limit`/`coreTenantId` como está (o back ignora
  `limit` extra sem erro — é query string, não body).
- **`pages/finance.vue`**: remover import `LegacyMarker` (linha 9) e o bloco
  `<LegacyMarker .../>` do template (linhas 131-135); `definePageMeta` →
  `workspaceId: 'finance'` (linha 25) e remover o comentário das linhas 22-24;
  atualizar comentário do topo (linhas 1-6).

### 3.5 Front — gating de workspace/módulo (editar 4 arquivos)

- **`web/app/utils/workspaces.ts`**: adicionar em `WORKSPACES` (após o item
  `meta_ads`): `{ id: 'finance', label: 'Finance', icon: 'payments', path: '/finance' }`.
- **`web/app/domain/utils/permissions.ts`**:
  1. `WORKSPACE_ACCESS_DEFINITIONS`: novo item (padrão do `tasks`, linhas 37-43):
     `{ id: 'finance', label: 'Finance', description: 'Planilhas financeiras mensais (entradas/saidas, efetivacao, recorrencias).', viewPermission: '', editPermission: '' }`.
  2. `ROLE_WORKSPACES.platform_admin` e `ROLE_WORKSPACES.owner`: adicionar `'finance'`.
  3. `MODULE_WORKSPACE_PERMISSION_PREFIXES` (linha 437-439): `finance: 'finance.'`
     (papéis custom com qualquer permissão `finance.*` enxergam o workspace —
     padrão `isPlatformAdmin || has(...)` preservado, aditivo).
- **`web/app/middleware/module-enabled.global.ts`**: em `MODULE_PATH_GUARDS`
  (linha 17+), adicionar `{ prefix: '/finance', moduleId: 'finance' }`.
- **`web/layers/queue/nav.config.ts`** (linha 258-262): adicionar
  `workspaceId: 'finance'` ao item finance e trocar o comentário para "Finance: gate
  por modulo + workspace (back real /v1/finance)".

### 3.6 Remover o mock (deletar 10 arquivos)

Apagar `web/server/` INTEIRO (só contém os 10 arquivos de finance; nada em
`web/nuxt.config.ts` referencia o diretório — verificado). Nenhum outro arquivo de
código importa `web/server` (só docs/roadmap citam como texto).

### 3.7 Docs sincronizados (editar 5)

- **`docs/LEGADO.md`** §6: manter o header, substituir o corpo por nota de resolução:
  "**RESOLVIDO 2026-07-02 (AC-12):** back Go real em `back/internal/modules/finance/`
  (migration 0187, rotas `/v1/finance/*`); `web/server/` removido por completo;
  composables migrados para `createApiRequest`; LegacyMarker retirado de `/finance`."
- **`docs/adr/0002-remove-bff-nitro-mock.md`**: Status → "Aceito — concluído
  (2026-07-02)"; adicionar item 7 ao "Histórico de remoção": "**AC-12** (2026-07-02) —
  Removido o resquício final: mock finance (`web/server/` inteiro). Módulo Go novo
  `finance`."
- **`docs/finance/PLANO_MODULO_FINANCE.md`**: bloco "Estado" → implementado
  2026-07-02; corrigir migration 0181→0187 e rotas `/v1/admin/finance-*`→`/v1/finance/*`
  (§4, §7, §8, §9); marcar passos 1-7 do §9 como feitos.
- **`web/app/components/roadmap/roadmap-data.ts`** fase-14 (linhas ~585-604):
  `backend` → `done: true` (note: módulo Go + migration 0187 + rotas /v1/finance);
  `permissions` → `done: true`; `acceptance` → `done: true` (note: persistência real
  via API Go); note do `mock-bff` ganha sufixo "REMOVIDO 2026-07-02 (web/server/
  extinto)"; atualizar `goal` e `verifiable` (mock removido; falta só
  contacts-integration). Não mexer no item da linha ~2349 (card do módulo).
- **`web/layers/finance/AGENT.md`**: reescrever seções "Estado atual" (API Go real,
  sem mock) e "Nav / gating" (workspace `finance` registrado, MODULE_PATH_GUARDS,
  workspaceId na página). Registrar pendências: realtime, split AC-07.

## 4. Critérios de aceite

1. `docker compose up -d --build api` sobe; log mostra `0187_finance_module.sql`
   aplicada; `/healthz` ok.
2. `core.modules` contém `finance`; `core.permissions` contém as 5 chaves
   `finance.*`; `core.role_templates` contém `finance.manager`/`finance.viewer`
   (efeito do SyncCatalog no boot).
3. Com token de platform_admin + `X-Account-Id`: POST cria sheet, GET lista com meta,
   GET detalhe traz `entradas`/`saidas`/`summary`/`preview`, PUT full-replace persiste,
   PATCH de linha efetiva/des-efetiva (des-efetivar limpa `effectiveDate`), DELETE
   remove; GET/PUT config round-trip; GET recurring-clients traz contas com billing.
4. Restart do container da api NÃO perde dados (prova de que saiu da memória).
5. Conta sem o módulo: qualquer `/v1/finance/*` → 403 `module_disabled`; sheet de
   outra account por id → 404.
6. `web/server/` não existe; nenhum `$fetch` para `/api/admin/finance-*` sobra
   (`grep -rn "api/admin/finance" web/app web/layers` = 0 hits em código).
7. Página `/finance` (dev já rodando) funciona igual: criar/editar/efetivar/excluir,
   config e autosave — sem badge MOCK; menu Finance visível para platform_admin.
8. Nenhum arquivo NOVO acima de 450 linhas (conferir `wc -l back/internal/modules/finance/*`).
9. AGENT.md do módulo criado; os 5 docs do §3.7 atualizados.

## 5. Validação

Back (PODE e DEVE rodar):
```
docker compose up -d --build api
docker compose logs api --tail 50        # migration 0187 + boot ok
curl -s http://localhost:9091/healthz
go vet ./... && go test ./internal/modules/finance/...   # em back/ (se testes forem escritos)
```
Smoke de API: pedir ao usuário login/senha de teste (NUNCA inventar credencial), então:
```
TOKEN=$(curl -s -X POST http://localhost:9091/v1/auth/login -d '{"email":"<pedir>","password":"<pedir>"}' | jq -r .data.accessToken)
curl -s http://localhost:9091/v1/finance/sheets -H "Authorization: Bearer $TOKEN" -H "X-Account-Id: <uuid da conta>"
```
Web (DEIXAR LISTADO para o usuário aprovar — não rodar):
```
docker compose exec web npx eslint web/layers/finance web/app/domain/utils/permissions.ts
npm run test   # vitest (17 arquivos atuais; nenhum cobre finance)
```
Validação visual: usuário abre http://localhost:3003/finance e exercita a tela.

## 6. Notas de Deploy

- **Migration nova:** `0187_finance_module.sql` — roda automática no start da api
  (CMD `migrate up && ... api`). Sem backfill, sem dado tocado.
- **Rebuild obrigatório da api** (mudou `back/`): `docker compose up -d --build api`.
- Sem env vars novas. Sem dependência de infra nova. Portas inalteradas (api 9091,
  web 3003, postgres 5432).
- Pós-deploy: habilitar `finance` por conta no painel admin (`core.account_modules`)
  para contas não-admin que forem usar; platform_admin já acessa via bypass.
- Web precisa de rebuild/deploy do bundle quando o usuário aprovar (composables
  mudaram) — na VPS as imagens sobem juntas pelo fluxo GHCR normal.

## 7. Regras de execução (obrigatórias para o implementador)

- **NENHUM comando git** (sessão multi-agente; só o usuário roda git).
- **NÃO rodar npm/build/generate** sem aprovação; a validação do back
  (`docker compose up -d --build api`) PODE e DEVE rodar. Validação web fica listada (§5).
- Máx **450 linhas** por arquivo novo/refatorado (não crescer os já estourados).
- **Não remover funcionalidade** existente; features coexistem.
- **Zero mock/legado novo**; fonte única = banco real; LEGADO.md atualizado (§3.7).
- Migrations: SQL plano idempotente, **SEM `-- +goose Down`**, número **0187**.
- Go: **sem lib uuid** (string + `::uuid`); scan nullable com ponteiro; permissões no banco.
- **NUNCA** tocar password_hash/dados de usuário; nada destrutivo sem backup.
- Atualizar **AGENT.md** dos módulos tocados (finance novo + layer finance).
- Front: gating sempre `isPlatformAdmin || has(...)` (aqui via bypass do guard +
  prefixo `finance.` — aditivo).
- Design system: sem CSS novo nesta spec; feedback de clique já existe (savingMap/
  loading dos composables — preservar).

## 8. Arquivos tocados

**Criar (back):**
- `back/internal/platform/database/migrations/0187_finance_module.sql`
- `back/internal/modules/finance/module.go`
- `back/internal/modules/finance/model.go`
- `back/internal/modules/finance/service.go`
- `back/internal/modules/finance/http.go`
- `back/internal/modules/finance/store_sheets.go`
- `back/internal/modules/finance/store_config.go`
- `back/internal/modules/finance/errors.go`
- `back/internal/modules/finance/AGENT.md`

**Editar (back):**
- `back/internal/platform/app/app.go` (MustRegister + moduleGatingRules + import)

**Editar (front):**
- `web/layers/finance/composables/useFinancesManager.ts`
- `web/layers/finance/composables/useFinancesConfigManager.ts`
- `web/layers/finance/composables/useFinanceConfigEditor.ts`
- `web/layers/finance/pages/finance.vue`
- `web/app/utils/workspaces.ts`
- `web/app/domain/utils/permissions.ts`
- `web/app/middleware/module-enabled.global.ts`
- `web/layers/queue/nav.config.ts`

**Deletar (front):** `web/server/` inteiro (10 arquivos listados no §1).

**Editar (docs):**
- `docs/LEGADO.md`
- `docs/adr/0002-remove-bff-nitro-mock.md`
- `docs/finance/PLANO_MODULO_FINANCE.md`
- `web/app/components/roadmap/roadmap-data.ts`
- `web/layers/finance/AGENT.md`
