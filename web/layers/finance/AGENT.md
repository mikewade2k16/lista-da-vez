# Layer `finance` (front do modulo financeiro)

Portado do `web-reference` (Fase 14 do roadmap). Planilhas mensais de entradas/saidas
com autosave, efetivacao por linha, ajustes (+/-), contas fixas com composicao,
categorias e recorrencias de clientes (grupos por loja).

## Estado atual: MOCK (nao pronto)

O front esta completo, mas a fonte de dados e um **mock BFF temporario** em
`web/server/api/admin/finance-*` (in-memory, some no restart, so roda em dev/SSR).
Ha um **badge "MOCK" visivel so para platform_admin** na tela `/finance`.

- NAO persiste no banco real. Registrado em `docs/LEGADO.md`.
- Back Go real desenhado em `docs/finance/PLANO_MODULO_FINANCE.md` (schema `finance.*`).
- Ao entrar o back: trocar `$fetch` por `createApiRequest` (~/utils/api-client) com
  `X-Account-Id`, reativar realtime, e apagar `web/server/api/admin/finance-*` +
  `web/server/utils/financeMockStore.ts`.

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
  - `useFinancesManager.ts` / `useFinancesConfigManager.ts` — camada de dados (`$fetch`).
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
- Nuxt UI aqui e o **community `@nuxt/ui` v4** — sem componentes Pro. O layout de
  lista+detalhe do web-reference (`UDashboard*`) foi trocado por grid simples.
- `OmniMoneyInput` e reaproveitado do layer `tasks` (nao duplicar).

## Nav / gating

`web/layers/queue/nav.config.ts` -> item `finance` com `moduleId: 'finance'`. Some
para contas sem o modulo (nenhuma ainda, pois o back nao existe); `platform_admin`
enxerga via platformView. Quando o back seedar `core.account_modules`, o item
aparece para as contas habilitadas.

**workspaceId (fase mock):** a pagina usa `definePageMeta workspaceId: ''` DE
PROPOSITO. O `auth.global.ts` redireciona qualquer `workspaceId` fora de
`auth.allowedWorkspaces` para o `homePath` (`/operacao` no admin) — e roda ate para
platform_admin (sem early-return de platformView). Como o workspace `finance` ainda
nao existe, declarar `workspaceId: 'finance'` fazia `/finance` cair em `/operacao`.
Vazio = rota nao-gated por workspace (mesmo padrao do fallback `/perfil`). Ao entrar
o back: registrar o workspace `finance` (utils/workspaces + allowedWorkspaces por
papel), adicionar `/finance` em `MODULE_PATH_GUARDS` e voltar a gatear a pagina.
