# AGENT - `back/internal/modules/finance`

## Escopo

Modulo de planilhas financeiras mensais por cliente: entradas/saidas, efetivacao,
ajustes (adjustments), contas fixas, categorias e recorrencias. Painel em
`/finance` (front na layer `web/layers/finance`).

Decisao AC-12 (2026-07-02): substitui o mock BFF Nitro (`web/server/*`, removido)
por back Go real. Contrato JSON identico ao mock (camelCase 1:1 com
`web/layers/finance/types/finances.ts`) — o front so trocou a origem (`$fetch`
para o BFF -> `createApiRequest` para a API Go). ADR: `docs/adr/0002-remove-bff-nitro-mock.md`.
Plano de referencia: `docs/finance/PLANO_MODULO_FINANCE.md`.

## Banco

Schema proprio: `finance`. Migration: `0187_finance_module.sql` (idempotente, sem
`-- +goose Down`). `account_id uuid not null` em TODAS as tabelas raiz (multi-tenant
dia 1). `coreTenantId` e filtro/rotulo do cliente DENTRO da account (`''` = escopo
padrao).

Tabelas:

- `finance.sheets` — planilha (account_id, core_tenant_id, title, period, status, notes, timestamps).
- `finance.lines` — linhas de entrada/saida (kind, amount, adjustment_amount, effective, effective_date, position). `effective_date date` nullable (`''` <-> NULL no contrato).
- `finance.line_adjustments` — ajustes pontuais de uma linha (amount, note, date, position).
- `finance.categories` / `finance.fixed_accounts` (+ `finance.fixed_account_members`) / `finance.recurring_entries` — config por escopo.
- `finance.config_state` — guarda o `updated_at` do payload de config (PK `(account_id, core_tenant_id)`).

Decisoes de schema (espelham o mock 1:1):

- `fixed_account_id`/`category_id` sao TEXT (nao FK): o front usa ids deterministicos
  de recorrencia (`finance-ids.ts`) e trata como snapshot textual.
- SEM unique de `lower(name)` em categories e SEM unique em recurring_entries: o
  PUT de config e full-replace e o autosave nao valida duplicata.

## Rotas (`/v1/finance/*`)

Todas sob `RequireAuth`; gating de modulo por prefixo no Chain (`moduleGatingRules`
em `app.go`: `{Prefix: "/v1/finance", ModuleID: "finance"}`). `platform_admin` tem
bypass do guard. Sem account no contexto -> 403 `no_account`.

- `GET /v1/finance/sheets` -> `{status, data:[SheetListItem], meta}` (query `q`, `coreTenantId`, `period`, `page`, `limit` default 240 teto 500).
- `POST /v1/finance/sheets` -> `{status, data:SheetDetail}`.
- `GET /v1/finance/sheets/{id}` -> `SheetDetail` | 404 `sheet_not_found`.
- `PUT /v1/finance/sheets/{id}` -> `SheetDetail` (full-replace) | 404.
- `DELETE /v1/finance/sheets/{id}` -> `{status:"success"}` | 404.
- `PATCH /v1/finance/sheets/{id}/lines/{lineId}` -> `{sheetId,lineId,line,summary,preview,updatedAt}` | 404 `line_not_found`. Des-efetivar (`effective=false`) limpa `effectiveDate`.
- `GET /v1/finance/config?coreTenantId=` -> `ConfigData`.
- `PUT /v1/finance/config` -> `ConfigData` (full-replace).
- `GET /v1/finance/config/recurring-clients` -> `[RecurringClient]`.

## Permissoes (catalogo, no banco via SyncCatalog)

`finance.sheets.view|manage`, `finance.config.view|manage`, `finance.recurring.manage`
(Scope "account"). Templates: `finance.manager` (todas) e `finance.viewer`
(`sheets.view` + `config.view`).

Enforcement por request hoje = `RequireAuth` + gating de modulo (mesmo estado dos
satelites site/calendar/cardapio; nenhum aplica permissao fina por rota ainda). O
catalogo existe para o painel de papeis e para o gating de workspace no front
(prefixo `finance.`).

## Isolamento multi-tenant

- `accountID` SEMPRE do request (X-Account-Id ou TenantID do JWT), nunca do body.
- `coreTenantId` e filtro normalizado (trim+lower) DENTRO da account.
- Sheet/linha de outra account -> **404** (nao 403; nao vaza existencia).
- Defesa em profundidade: toda query filtra por `account_id`.
- `recurring-clients`: so `platform_admin` recebe a lista real (agencia
  acompanhando mensalidades); caller comum recebe `[]` (idem ao mock, zero
  vazamento cross-tenant).

## Arquivos

- `module.go` — registro, Metadata, Permissions, RoleTemplates, Build.
- `model.go` — structs + tags JSON camelCase (contrato do front).
- `service.go` — regras de sheets (normalizacao, summary, preview) portadas do mock.
- `service_config.go` — regras de config (split para respeitar ~450 linhas).
- `store_sheets.go` — pgx: List/Get/Create/Update(full-replace)/Delete/PatchLine.
- `store_config.go` — pgx: GetConfig/SaveConfig(full-replace)/ListRecurringClients.
- `http.go` — handlers `/v1/finance/*`.
- `errors.go` — sentinelas `ErrSheetNotFound`/`ErrLineNotFound`.

IDs: sempre `string` + cast `::uuid` no SQL (sem lib uuid externa). Id novo via
`coalesce(nullif($n,'')::uuid, gen_random_uuid())`; id invalido no service vira `''`.
Nullable (`effective_date`, `date`, `payment_due_day`) escaneado com ponteiro.

## Pendencias

- **Realtime/WebSocket**: nao ha broadcast; o front so re-busca sob demanda (fica
  para fase futura, como o mock).
- **Permissao fina por rota**: hoje so gating de modulo; aplicar `finance.*` por
  endpoint quando os satelites adotarem enforcement fino.
- **Integracao `contacts`**: o modulo `contacts` nao existe ainda; `clientName` sai
  de `core.accounts.name` (lookup quando `core_tenant_id` e uuid de account). Nao
  ha `OptionalModules` declarado para evitar referencia morta.
- **Split AC-07**: a layer front `useFinanceSheetEditor.ts` (960 linhas) e
  `useFinancesManager.ts` (485) continuam acima do teto; refatoracao e escopo do AC-07.
