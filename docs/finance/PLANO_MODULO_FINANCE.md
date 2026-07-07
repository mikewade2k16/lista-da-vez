# PLANO DO MODULO FINANCE (back Go)

Doc canonico do modulo financeiro. Fase 14 do roadmap ("Modulo Finance").
Espelha o padrao dos modulos `site` e `cardapio` (Module Registry + schema proprio +
permissoes no banco + `account_modules`).

> **Estado (implementado 2026-07-02, AC-12):** back Go real em
> `back/internal/modules/finance/` (migration `0187_finance_module.sql`, schema
> `finance.*`, rotas **`/v1/finance/*`**). O mock BFF (`web/server/`) foi
> **removido**; os composables da layer finance passaram a usar `createApiRequest`.
> DIVERGENCIAS deste doc original ja corrigidas abaixo: migration `0181`->`0187`
> e prefixo de rota `/v1/admin/finance-*`->`/v1/finance/*` (decisao AC-12, casa com
> o gating por prefixo `moduleGatingRules`). Ver `back/internal/modules/finance/AGENT.md`,
> ADR 0002 e docs/LEGADO.md #6.

---

## 1. Objetivo e escopo

Planilhas financeiras mensais por cliente (`coreTenantId`), com:
- Linhas de **entrada** e **saida** (valor + ajustes +/- com historico + efetivacao com data).
- **Categorias** e **contas fixas** (com composicao/membros) configuraveis por cliente.
- **Recorrencias de clientes** (mensalidades), inclusive por loja (`billingMode: per_store`).
- **Resumo** por planilha (esperado/efetivado de entradas, saidas e saldo).

Contrato de dados: `web/layers/finance/types/finances.ts` (fonte da forma dos payloads).

## 2. Isolamento multi-tenant (obrigatorio)

- Todas as rotas sob `RequireAuth` + `ModulesGuard.RequireModule("finance")`.
- Escopo por **account** (Principal resolvido pelo middleware, `X-Account-Id`). O
  `account_id` NUNCA vem do body.
- `coreTenantId` (cliente da planilha) e' **filtro validado dentro do escopo** da
  account, nunca confiado cru do client. Recurso fora do escopo -> **404** (nao 403).
- Defesa em profundidade: alem da validacao no service, as queries do repositorio
  filtram por `account_id` (e por `core_tenant_id` quando aplicavel).

## 3. Schema `finance.*` (derivado dos tipos do front)

Migration idempotente, SQL plano (sem `-- +goose Down`; ver regra do migrator em
docs). `id` sempre `uuid` (default `gen_random_uuid()`), timestamps `timestamptz`.

### `finance.sheets`
| coluna | tipo | notas |
| --- | --- | --- |
| id | uuid pk | |
| account_id | uuid not null | escopo dono (FK `core.accounts`) |
| core_tenant_id | text | cliente da planilha (pode ser vazio) |
| title | text not null default '' | |
| period | text not null | `YYYY-MM` (check) |
| status | text not null default 'aberta' | aberta/conferencia/fechada (livre) |
| notes | text not null default '' | |
| created_at / updated_at | timestamptz not null default now() | |

`summary` e `preview` sao **computados na leitura** (nao armazenados).
`clientName` deriva de lookup do cliente (nome da account/tenant).
Indices: `(account_id, core_tenant_id, period)`, `(account_id, updated_at desc)`.

### `finance.lines`
| coluna | tipo | notas |
| --- | --- | --- |
| id | uuid pk | |
| sheet_id | uuid not null | FK `finance.sheets` on delete cascade |
| kind | text not null | `entrada`\|`saida` (check) |
| description | text not null default '' | |
| category | text not null default '' | nome da categoria (snapshot) |
| effective | boolean not null default false | |
| effective_date | date | null quando nao efetivada |
| amount | numeric(14,2) not null default 0 | valor base (>= 0) |
| adjustment_amount | numeric(14,2) not null default 0 | soma dos ajustes (pode ser negativo) |
| fixed_account_id | uuid | vinculo opcional a `finance.fixed_accounts` ou id deterministico de recorrencia |
| details | text not null default '' | |
| position | int not null default 0 | ordem na planilha |

Indice: `(sheet_id, kind, position)`.

### `finance.line_adjustments`
| coluna | tipo | notas |
| --- | --- | --- |
| id | uuid pk | |
| line_id | uuid not null | FK `finance.lines` on delete cascade |
| amount | numeric(14,2) not null | +/- |
| note | text not null default '' | |
| date | date | |

### `finance.categories`
| coluna | tipo | notas |
| --- | --- | --- |
| id | uuid pk | |
| account_id | uuid not null | escopo |
| core_tenant_id | text not null default '' | config por cliente |
| name | text not null | |
| kind | text not null | `entrada`\|`saida`\|`ambas` (check) |
| description | text not null default '' | |

Unico: `(account_id, core_tenant_id, lower(name))`.

### `finance.fixed_accounts`
| coluna | tipo | notas |
| --- | --- | --- |
| id | uuid pk | |
| account_id | uuid not null | escopo |
| core_tenant_id | text not null default '' | |
| name | text not null | |
| kind | text not null | `entrada`\|`saida`\|`ambas` |
| category_id | uuid | FK `finance.categories` (null ok) |
| default_amount | numeric(14,2) not null default 0 | |
| notes | text not null default '' | |

### `finance.fixed_account_members`
| coluna | tipo | notas |
| --- | --- | --- |
| id | uuid pk | |
| fixed_account_id | uuid not null | FK on delete cascade |
| name | text not null default '' | |
| amount | numeric(14,2) not null default 0 | |

### `finance.recurring_entries`
| coluna | tipo | notas |
| --- | --- | --- |
| id | uuid pk | |
| account_id | uuid not null | escopo |
| core_tenant_id | text not null default '' | config do cliente dono da planilha |
| source_core_tenant_id | text not null | cliente-fonte da mensalidade |
| adjustment_amount | numeric(14,2) not null default 0 | ajuste do mes |
| notes | text not null default '' | |

Unico: `(account_id, core_tenant_id, source_core_tenant_id)`.

### Read model: recorrencias de clientes (`recurring-clients`)
NAO e' tabela nova. E' um **read model** montado por join entre `core.accounts`
(clientes que pagam mensalidade) + suas lojas + `finance.recurring_entries`
(ajuste/notas do mes). Alimenta `GET /v1/finance/config/recurring-clients`.
Regra de `billingMode`/`stores` a definir na implementacao (hoje o mock devolve vazio).

## 4. Endpoints (`/v1/finance/...`)

Todos account-scoped, `DisallowUnknownFields` no decode, erros especificos.
Mapa mock BFF -> API real:

| Metodo | Rota | Acao | Permissao |
| --- | --- | --- | --- |
| GET | `/v1/finance/sheets` | lista (`page,limit,q,coreTenantId,period`) + meta | `finance.sheets.view` |
| POST | `/v1/finance/sheets` | cria planilha | `finance.sheets.manage` |
| GET | `/v1/finance/sheets/{id}` | detalhe (entradas/saidas) | `finance.sheets.view` |
| PUT | `/v1/finance/sheets/{id}` | atualiza (full-replace das linhas) | `finance.sheets.manage` |
| DELETE | `/v1/finance/sheets/{id}` | remove | `finance.sheets.manage` |
| PATCH | `/v1/finance/sheets/{id}/lines/{lineId}` | efetiva/`effectiveDate` | `finance.sheets.manage` |
| GET | `/v1/finance/config` (`coreTenantId`) | categorias+contas fixas+recorrencias | `finance.sheets.view` |
| PUT | `/v1/finance/config` | salva config | `finance.config.manage` |
| GET | `/v1/finance/config/recurring-clients` | read model de mensalidades | `finance.sheets.view` |

Resposta padrao: `{ status: 'success', data, meta? }` (igual ao contrato do front).

O `PUT /finance-sheets/{id}` faz **full-replace** de `entradas`/`saidas`: apaga as
linhas/ajustes atuais e reinsere a partir do payload (dentro de transacao). O
`PATCH .../lines/{lineId}` altera so `effective`/`effective_date` e devolve a linha
+ o `summary` recomputado.

## 5. Permissoes e role templates

`permissions.go` (padrao do modulo). Chaves no formato `entity.verb` — concilia os
nomes do roadmap (`finance.read`/`finance.write`/`finance.recurring.manage`):

- `finance.sheets.view` (scope account) — ler planilhas + config.
- `finance.sheets.manage` (scope account) — criar/editar/excluir planilhas e linhas.
- `finance.config.manage` (scope account) — editar categorias/contas fixas.
- `finance.recurring.manage` (scope account) — ajustar recorrencias de clientes.

Role templates:
- `finance.manager` — todas as permissoes acima.
- `finance.viewer` — so `finance.sheets.view`.

## 6. Modulo Go (`back/internal/modules/finance/`)

Arquivos (padrao `site`):
- `module.go` — `Module`/`Handle`; `Metadata{ SchemaName:"finance", Label:"Finance",
  IsCore:false, SortOrder:50, OptionalModules:["contacts"] }`; `Permissions()`;
  `RoleTemplates()`; `Build(deps)` monta repos+service; `RegisterRoutes` com o guard.
- `http.go` — handlers (`handler -> service`, sem acesso a banco no handler).
- `service.go` — regras + validacao de escopo (coreTenantId dentro da account) +
  computo de `summary`/`preview`.
- `repository_sheets.go` / `repository_config.go` — SQL (filtro por account_id;
  full-replace transacional das linhas).
- `model.go` — structs de dominio + DTOs (IDs como `string`, scan de NULL com `*T`).
- `permissions.go`, `errors.go`, `AGENT.md`.

Registro no boot (`back/internal/platform/app/app.go`, junto aos demais
`registry.MustRegister(...)`):
```go
registry.MustRegister(finance.New())
```
`SyncCatalog` popula `core.modules`/`core.permissions`/`core.role_templates`.
`RegisterRoutes` deve envolver as rotas com `deps.ModulesGuard.RequireModule("finance")`.

**Integracao com `contacts` (opcional):** quando `contacts` estiver habilitado para a
account, o cliente/`coreTenantId` pode resolver via Resolver do contacts; enquanto
desligado, usa a entidade local (account/tenant). Nao bloqueia o modulo.

## 7. Migrations

SQL plano idempotente. Implementada como **`0187_finance_module.sql`** (a proxima
livre no momento do AC-12 era 0187, nao 0181): `create schema if not exists finance;`
+ as tabelas da secao 3 com `create table if not exists`, checks, FKs e indices.

Divergencias conscientes do desenho original (para espelhar o mock 1:1):
- `fixed_account_id`/`category_id` ficaram **text** (nao uuid/FK): o front usa ids
  deterministicos de recorrencia (`finance-ids.ts`) e trata como snapshot textual.
- **SEM** unique de `lower(name)` em categories e **SEM** unique em recurring_entries:
  o PUT de config e full-replace e o autosave nao valida duplicata (unique quebraria).
- `finance.config_state` guarda o `updated_at` do payload de config.

`account_modules`: **opt-in por conta** (habilitado no painel admin). NAO seedado
para todas as contas. O `SyncCatalog` garante `finance` em `core.modules`/`permissions`/
`role_templates` no boot. Sem `-- +goose Down` (o migrator roda o arquivo inteiro).

## 8. Notas de Deploy

- Migration `0187_finance_module.sql` — roda automatica no start da api. Sem backfill.
- Mexe em `back/` -> **rebuild obrigatorio**: `docker compose up -d --build api`.
- Sem novas env vars. Sem dependencia nova de infra. Portas inalteradas.
- Web: rebuild/deploy do bundle quando aprovado (composables da layer finance mudaram).
- Habilitar o modulo por conta em `core.account_modules` via painel para contas
  nao-admin; `platform_admin` ja acessa via bypass do guard.

## 9. Passo-a-passo de implementacao — FEITO (AC-12, 2026-07-02)

1. [x] `0187_finance_module.sql` + `SyncCatalog` registra `finance` no boot.
2. [x] `back/internal/modules/finance/` (model, service, service_config, store_sheets,
   store_config, http, module, errors, AGENT.md).
3. [x] `registry.MustRegister(finance.New())` + gating `{Prefix:"/v1/finance"}` em `app.go`.
4. [x] Front (`useFinancesManager`/`useFinancesConfigManager`/`useFinanceConfigEditor`)
   migrado de `$fetch` para `createApiRequest(runtimeConfig, () => auth.accessToken)`;
   X-Account-Id entra pelo provider global. Realtime: pendente (fase futura).
5. [x] Mock BFF removido: `web/server/` inteiro apagado (inclui `financeMockStore.ts`).
6. [x] `LegacyMarker` retirado de `/finance`; docs/LEGADO.md #6 marcado RESOLVIDO.
7. [x] Gating restaurado: workspace `finance` em `workspaces.ts` + `permissions.ts`
   (WORKSPACE_ACCESS_DEFINITIONS, ROLE_WORKSPACES platform_admin/owner,
   MODULE_WORKSPACE_PERMISSION_PREFIXES `finance.`); `definePageMeta workspaceId:'finance'`;
   `{ prefix:'/finance', moduleId:'finance' }` em `module-enabled.global.ts`;
   `workspaceId:'finance'` no item do `nav.config.ts`.
8. Aceite (validar no browser): criar planilha, efetivar recorrencia, ajustar valor,
   config/autosave via API Go — dados persistem (sobrevivem a restart).

## 10. Referencias

- Contrato/tipos: `web/layers/finance/types/finances.ts`.
- Front + AGENT: `web/layers/finance/AGENT.md`.
- Padrao de modulo: `back/internal/modules/site/` + `back/internal/platform/modules/module.go`.
- Roadmap: Fase 14 em `web/app/components/roadmap/roadmap-data.ts`.
- Legado/mock: `docs/LEGADO.md` #6.
