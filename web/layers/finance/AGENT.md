# Layer `finance` (front do modulo financeiro)

Portado do `web-reference` (Fase 14 do roadmap). Planilhas mensais de entradas/saidas
com autosave, efetivacao por linha, ajustes (+/-), contas fixas com composicao,
categorias e recorrencias de clientes (grupos por loja).

## Estado atual: API Go real (sem mock)

A fonte de dados e o **back Go real** `back/internal/modules/finance/` (schema
`finance.*`, migration `0187_finance_module.sql`, rotas `/v1/finance/*`). O mock BFF
(`web/server/`) foi **removido** em AC-12 (2026-07-02); o badge "MOCK" saiu de
`/finance`.

- Os composables usam `createApiRequest(runtimeConfig, () => auth.accessToken)`; o
  `X-Account-Id` entra sozinho pelo provider global (`account-id-bridge.client.ts`) —
  nao passar manualmente. Escopo de cliente (`coreTenantId`) via `useCoreAccountStore`.
- Dados persistem no banco real (sobrevivem a restart). Contrato JSON identico ao do
  mock (camelCase 1:1). Ver `docs/finance/PLANO_MODULO_FINANCE.md`, ADR 0002,
  `docs/LEGADO.md` #6 (RESOLVIDO) e `back/internal/modules/finance/AGENT.md`.
- **Realtime**: ainda nao ha broadcast; a tela re-busca sob demanda (pendencia).

## Estrutura

- `pages/finance.vue` — **port fiel** de `web-reference/app/pages/admin/finance.vue`:
  layout (`UDashboardGroup` + `UDashboardSidebar` redimensionavel + `UDashboardPanel`),
  cabecalho, tabelas entradas/saidas e cartao de saldo reproduzidos identicos, com as
  mesmas classes/estilos `finances-page__*`. A logica vem dos composables (usa o sheet
  editor direto e `provide(FINANCE_CONFIG_KEY)` para o painel). Rota `/finance`
  (placeholder demo em `app/pages/finance.vue` removido).
  > O layout usa componentes `UDashboard*` — no Nuxt UI **v4 o "Pro" foi unificado**
  > no `@nuxt/ui`, entao existem no pacote community instalado (4.8.0). `UDashboardGroup`
  > leva `!static !inset-auto !h-auto !w-full` para ficar em fluxo dentro da pagina.
- `components/finance/`
  - `FinanceConfigPanel.vue` — slideover de configuracao (injeta `FINANCE_CONFIG_KEY`),
    port fiel do slideover de referencia.
  - `FinanceLineCard.vue`, `FinanceRecurringGroupCard.vue` — cartoes de linha/grupo (port fiel).
- `composables/`
  - `useFinancesManager.ts` / `useFinancesConfigManager.ts` — camada de dados (`createApiRequest` -> `/v1/finance/*`).
  - `useFinanceSheetEditor.ts` — draft + autosave + linhas + efetivacao + recorrencia.
  - `useFinanceConfigEditor.ts` — categorias/contas fixas/recorrencias + autosave.
    Exporta `FINANCE_CONFIG_KEY` (usado pelo painel de config).
- `utils/finance-helpers.ts` — helpers puros (formatacao, datas, snapshot, recorrencia).
- `utils/finance-ids.ts` — UUIDs e ids deterministicos de recorrencia.
- `types/finances.ts` — contrato (espelha o schema-alvo `finance.*`).

## Padroes do layer

- `~` = app (`web/app`). Arquivos do layer referenciam os proprios via caminho
  relativo (`../types/...`, `../../core/stores/account`). Componentes Omni sao
  auto-importados (nao usar import explicito com path do app).
- Nuxt UI aqui e o **community `@nuxt/ui` v4** — no v4 o "Pro" foi unificado, entao
  os `UDashboard*` existem no pacote community (port fiel do web-reference mantido).
- `OmniMoneyInput` e reaproveitado do layer `tasks` (nao duplicar).

## Nav / gating

Gate por **modulo + workspace** (AC-12). Registro:

- `web/app/utils/workspaces.ts` -> workspace `finance` (`{ id:'finance', icon:'payments', path:'/finance' }`).
- `web/app/domain/utils/permissions.ts` -> `WORKSPACE_ACCESS_DEFINITIONS` (sem
  viewPermission fixa), `ROLE_WORKSPACES.platform_admin`/`.owner` incluem `finance`, e
  `MODULE_WORKSPACE_PERMISSION_PREFIXES.finance = 'finance.'` (papeis custom com
  qualquer permissao `finance.*` enxergam o workspace — padrao `isPlatformAdmin || has(...)`).
- `web/layers/queue/nav.config.ts` -> item `finance` com `workspaceId:'finance'` +
  `moduleId:'finance'`. Some para contas sem o modulo; `platform_admin` ve via bypass.
- `web/app/middleware/module-enabled.global.ts` -> `{ prefix:'/finance', moduleId:'finance' }`
  em `MODULE_PATH_GUARDS` (rota direta tambem gated).
- `pages/finance.vue` -> `definePageMeta workspaceId: 'finance'`.

Back: gating por prefixo `/v1/finance` em `moduleGatingRules()` (`app.go`); contas sem
o modulo habilitado recebem 403 `module_disabled`; `platform_admin` tem bypass.

## Pendencias

- Realtime/WebSocket (hoje re-busca sob demanda).
- Split AC-07: `useFinanceSheetEditor.ts` (960 linhas) e `useFinancesManager.ts`
  (~485) continuam acima do teto de ~450; refatoracao e escopo do AC-07.
