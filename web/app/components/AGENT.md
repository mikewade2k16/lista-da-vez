# AGENTS

## Escopo

Estas instrucoes valem para `web/app/components`.

## Objetivo

Esta pasta concentra componentes reutilizaveis de pagina, workspace e UI base do frontend.

Antes de criar componente novo:

1. verificar esta lista
2. verificar se a necessidade cabe em extensao pequena de componente existente
3. evitar duplicar variacoes visuais ou selects paralelos

## Regras de reutilizacao

- para selects simples de filtro e escolha unica, preferir [AppSelectField.vue](/c:/Users/Mike/Documents/Projects/fila-atendimento/web/app/components/ui/AppSelectField.vue)
  Ele segue a mesma linguagem visual do `.product-pick` do fechamento e deve substituir selects nativos soltos.
- para grades administrativas reutilizaveis sem `<table>`, preferir [AppEntityGrid.vue](/c:/Users/Mike/Documents/Projects/fila-atendimento/web/app/components/ui/AppEntityGrid.vue)
- para toggles booleanos compactos em linhas administrativas, preferir [AppToggleSwitch.vue](/c:/Users/Mike/Documents/Projects/fila-atendimento/web/app/components/ui/AppToggleSwitch.vue)
- para superficie de card/painel com estetica liquid glass, aplicar a classe `.omni-glass` (definida em `web/app/assets/styles/components.css`): fica solida nos temas normais e vira vidro (backdrop-filter) so no tema `.theme-liquidglass`, com fallback. Usa os tokens `--glass-*`. NAO cravar `backdrop-filter`/alpha por componente — reusar `.omni-glass` para nao duplicar e nao dar drift. Ver [docs/DESIGN_SYSTEM.md](/c:/Users/Mike/Documents/Projects/fila-atendimento/docs/DESIGN_SYSTEM.md) secao 2 e [docs/THEME_MODULE_PLAN.md](/c:/Users/Mike/Documents/Projects/fila-atendimento/docs/THEME_MODULE_PLAN.md).
- para QUALQUER modal/drawer de edicao (entidade em abas, editor, painel lateral), usar o TEMPLATE-CORE [OmniEntityDrawer.vue](/c:/Users/Mike/Documents/Projects/fila-atendimento/web/app/components/ui/OmniEntityDrawer.vue) — header fechar/expandir-toggle/popover de modo, resize no modo lado, modos lado/centro/fullscreen; so o conteudo muda (slots `default`/`#header-extra`/`#footer`). NAO criar `USlideover`/`UModal` a mao; ajuste no core vale para todos. Ver [docs/frontend/MODAL_TEMPLATE.md](/c:/Users/Mike/Documents/Projects/fila-atendimento/docs/frontend/MODAL_TEMPLATE.md).
- para modal de leitura detalhada simples, preferir [AppDetailDialog.vue](/c:/Users/Mike/Documents/Projects/fila-atendimento/web/app/components/ui/AppDetailDialog.vue)
- para dialogos e prompts globais, usar [AppDialogHost.vue](/c:/Users/Mike/Documents/Projects/fila-atendimento/web/app/components/ui/AppDialogHost.vue) via `uiStore`
- para toasts globais, usar [AppToastStack.vue](/c:/Users/Mike/Documents/Projects/fila-atendimento/web/app/components/ui/AppToastStack.vue) via `uiStore`
- para selecao pesquisavel, multi-select e detalhes por item, verificar primeiro o componente de feature [OperationProductPicker.vue](/c:/Users/Mike/Documents/Projects/fila-atendimento/web/app/components/operation/OperationProductPicker.vue) antes de inventar outro picker
- workspaces devem continuar finos: recebem estado/stores prontos e compoem a tela

## Catalogo atual

### `admin`

Area cross-account de plataforma (so `platform_admin`), backed pela API real
`/v1/admin/*`. Paginacao e filtros SERVER-SIDE.

- [AdminUsersWorkspace.vue](/c:/Users/Mike/Documents/Projects/fila-atendimento/web/app/components/admin/AdminUsersWorkspace.vue)
  Tela `/manage/users`. Lista cross-account de usuarios via `/v1/admin/users`
  (paginacao/filtros no servidor: `q`, `status`, `platformAdmin`, `accountId`).
  Filtros: Buscar, Status, Tipo e **Cliente** (select server-side; options vem de
  `useClientsManager` carregado no onMounted, traduzido `clientFilter` ->
  `filters.accountId` em `syncFiltersToBackend`).
  Coluna **Cliente** (`accountNames`, `type: 'custom'`, slot `#cell-accountNames`):
  editavel inline quando `clientAccountId != ''` e o usuario NAO e platform_admin —
  renderiza um `<select>` de clientes preselecionado e, no change, chama
  `moveUserAccount(id, accountId)` (`PUT /v1/admin/users/{id}/account` body
  `{ accountId, role: 'owner' }`) apos `window.confirm`; em sucesso aplica o user
  retornado na linha. Caso contrario (0 ou >1 clientes, ou platform_admin) mostra os
  nomes read-only. Coluna **Qtd clientes** (`accountCount`) e' read-only.
  Drawer de detalhe ([AdminUserEditDrawer.vue]) continua para dados/senha/vinculos.
  Logica de dados em [useAdminUsersManager.ts]; tipos em `web/types/admin-users.ts`
  (`AdminUserItem.clientAccountId`).

### `campaigns`

- [CampaignWorkspace.vue](/c:/Users/Mike/Documents/Projects/fila-atendimento/web/app/components/campaigns/CampaignWorkspace.vue)
  Workspace da tela de campanhas. Centraliza CRUD, regras, metas e configuracao comercial.
  Em `Todas as lojas`, deve consolidar o historico das lojas acessiveis usando filtro local da propria tela, sem depender de seletor global no header.

### `crm`

- [CrmWorkspace.vue](/c:/Users/Mike/Documents/Projects/fila-atendimento/web/app/components/crm/CrmWorkspace.vue)
  Workspace CRM comercial via ERP. Cruza vendas ERP, metas por loja e fila de atendimento.
- [CrmSummarySection.vue](/c:/Users/Mike/Documents/Projects/fila-atendimento/web/app/components/crm/CrmSummarySection.vue)
  Cards consolidados. `Uso da lista` e cobertura por consultor (`atendimentos >= pedidos ERP`), nao razao bruta de volumes. Melhor loja/consultor nao deve virar premio quando tudo esta abaixo da faixa `Normal`.
- [CrmConsultantsSection.vue](/c:/Users/Mike/Documents/Projects/fila-atendimento/web/app/components/crm/CrmConsultantsSection.vue)
  Grade ERP + fila por consultor. Em vendedor multi-loja, nao usar atendimento global de outra loja quando a linha ERP ja tem loja comercial. Coluna `Recebimento` usa meta da loja + politica `crmGoalPayoutPolicy`.

### `ranking`

- [RankingWorkspace.vue](/c:/Users/Mike/Documents/Projects/fila-atendimento/web/app/components/ranking/RankingWorkspace.vue)
  Orchestrator gamificado da workspace `ranking`: pódio do top 3 + leaderboard de cards + drawer de detalhes.
- [RankingTabsHeader.vue](/c:/Users/Mike/Documents/Projects/fila-atendimento/web/app/components/ranking/RankingTabsHeader.vue)
  Tabs `Lojas | Consultores | Por loja` + chips de sort (Score 360 default).
- [RankingPodium.vue](/c:/Users/Mike/Documents/Projects/fila-atendimento/web/app/components/ranking/RankingPodium.vue)
  Top 3 visual com pedestal (2-1-3). Click abre drawer.
- [RankingLeaderboardCard.vue](/c:/Users/Mike/Documents/Projects/fila-atendimento/web/app/components/ranking/RankingLeaderboardCard.vue)
  Card horizontal a partir do 4º com posição, métrica grande, barra de meta opcional e badge de variação ↑/↓.
- [RankingScoreBreakdown.vue](/c:/Users/Mike/Documents/Projects/fila-atendimento/web/app/components/ranking/RankingScoreBreakdown.vue)
  Barra stackeada com os 5 componentes do Score 360 (Conversão/Valor/Qualidade/P.A./Disciplina). Pesos vêm de `useGamificationConfig()`.
- [RankingDetailsDrawer.vue](/c:/Users/Mike/Documents/Projects/fila-atendimento/web/app/components/ranking/RankingDetailsDrawer.vue)
  Drawer (USlideover) com 3 tabs: Visão geral, Breakdown 360, Alertas. Modos `center`/`fullscreen`/`side` via `useRankingDetailsDrawer()`. Inclui toggle "Ver como tabela" usando o `RankingTable` legado.
- [RankingTable.vue](/c:/Users/Mike/Documents/Projects/fila-atendimento/web/app/components/ranking/RankingTable.vue)
  Tabela densa legada. Mantida exclusivamente como fallback opt-in dentro do drawer.

### `consultant`

- [ConsultantWorkspace.vue](/c:/Users/Mike/Documents/Projects/fila-atendimento/web/app/components/consultant/ConsultantWorkspace.vue)
  Workspace principal do painel do consultor.
  Com multiplas lojas acessiveis, deve alternar para a visao integrada e resolver o filtro de loja localmente dentro da propria pagina.
- [ConsultantIntegratedWorkspace.vue](/c:/Users/Mike/Documents/Projects/fila-atendimento/web/app/components/consultant/ConsultantIntegratedWorkspace.vue)
  Visao consolidada de consultores entre lojas, com filtros locais por loja, nome, status e meta; em loja especifica, reaproveita o layout detalhado com historico e simulador inline.
- [ConsultantSelector.vue](/c:/Users/Mike/Documents/Projects/fila-atendimento/web/app/components/consultant/ConsultantSelector.vue)
  Seletor visual de consultor dentro do painel administrativo/individual.
- [ConsultantPlayerCard.vue](/c:/Users/Mike/Documents/Projects/fila-atendimento/web/app/components/consultant/ConsultantPlayerCard.vue)
  Player card gamificado com gauge de meta + 4 KPIs core (Vendido, Ticket, PA, Conversão) + badges. Modos `full` (visão individual) e `mini` (grid multi-loja).
- [ConsultantBadges.vue](/c:/Users/Mike/Documents/Projects/fila-atendimento/web/app/components/consultant/ConsultantBadges.vue)
  Lista de badges puro recebendo `stats` + `badges` (config). Avalia regras de Meta batida, Top N, Conversão > média loja, Ticket > meta, PA > meta.
- [ConsultantPlayerGrid.vue](/c:/Users/Mike/Documents/Projects/fila-atendimento/web/app/components/consultant/ConsultantPlayerGrid.vue)
  Grid responsivo de player cards no modo mini para a visão integrada multi-loja.
- [ConsultantDetailsDrawer.vue](/c:/Users/Mike/Documents/Projects/fila-atendimento/web/app/components/consultant/ConsultantDetailsDrawer.vue)
  Drawer (USlideover) com 3 tabs: Visão geral (todos KPIs), Histórico (sparkline 7d), Simulador. Modos `center` (default) / `fullscreen` / `side`, controlados pelo composable `useConsultantDetailsDrawer()`.
- [ConsultantSimulator.vue](/c:/Users/Mike/Documents/Projects/fila-atendimento/web/app/components/consultant/ConsultantSimulator.vue)
  Simulador de impacto de vendas extras e metas. Renderizado inline no workspace individual e no modo de loja filtrada; segue reutilizado no drawer da visao integrada quando necessario.

### `dashboard`

- [DashboardHeader.vue](/c:/Users/Mike/Documents/Projects/fila-atendimento/web/app/components/dashboard/DashboardHeader.vue)
  Header autenticado com conta atual e acoes de sessao. Nao deve expor seletor global de loja; filtros de loja vivem dentro de cada workspace que precisa desse controle.
  Altura compacta fixa: o header usa padding vertical enxuto (`0.3rem`) e logo reduzida (`clamp(4.5rem, 8vw, 5.6rem)`) para ocupar pouca altura. A altura efetiva e ancorada pelo `min-height: 2.45rem` dos itens de nav. Sem efeito de hover/expansao.
- [DashboardWorkspaceNav.vue](/c:/Users/Mike/Documents/Projects/fila-atendimento/web/app/components/dashboard/DashboardWorkspaceNav.vue)
  Navegacao principal entre workspaces do app.
  O contexto operacional (pills de fila/atendimento/finalizados) aparece nas rotas `/operacao/*`. O seletor de loja do modo "Todas as lojas" tambem vive aqui, no `workspace-nav-context`, **antes** das pills — visivel so quando `auth.canUseAllStores` E `activeWorkspace === 'operacao'`. Atencao: este nav shell renderiza em TODO `/operacao/*` (inclui as rotas filhas `/operacao/clientes` e `/operacao/usuarios`, que sao workspaces distintos), entao o seletor e gateado pelo workspace ATIVO exato, nao pelo prefixo do path — so existe na propria pagina de operacao. NAO e um seletor global de loja: apenas escreve `integratedStoreId` no `stores/operations.ts` (lido pela pagina de operacao); nenhum outro modulo le esse filtro. Mudou so o lugar de render (do corpo da pagina para a barra do nav) para economizar altura.

### `data`

- [DataWorkspace.vue](/c:/Users/Mike/Documents/Projects/fila-atendimento/web/app/components/data/DataWorkspace.vue)
  Workspace da tela `/dados`.
- [InsightHourlyTable.vue](/c:/Users/Mike/Documents/Projects/fila-atendimento/web/app/components/data/InsightHourlyTable.vue)
  Tabela de leitura horaria/temporal.
- [InsightTagList.vue](/c:/Users/Mike/Documents/Projects/fila-atendimento/web/app/components/data/InsightTagList.vue)
  Lista compacta de tags/resumos de leitura.

### `intelligence`

- [IntelligenceWorkspace.vue](/c:/Users/Mike/Documents/Projects/fila-atendimento/web/app/components/intelligence/IntelligenceWorkspace.vue)
  Workspace da tela `/inteligencia`.
- [IntelligenceDiagnosisCard.vue](/c:/Users/Mike/Documents/Projects/fila-atendimento/web/app/components/intelligence/IntelligenceDiagnosisCard.vue)
  Card de diagnostico com severidade, contexto e recomendacoes.

### `multistore`

- [MultiStoreWorkspace.vue](/c:/Users/Mike/Documents/Projects/fila-atendimento/web/app/components/multistore/MultiStoreWorkspace.vue)
  Workspace administrativo de lojas e comparativo multiloja.
- [MultiStoreLojasSection.vue](/c:/Users/Mike/Documents/Projects/fila-atendimento/web/app/components/multistore/MultiStoreLojasSection.vue)
  Grid de cadastro/edicao inline de lojas (nome, codigo, cidade, `storeType` Shopping/Bairro, template, status). Fonte autoritativa = `useMultiStoreStore.managedStores` (de `GET /v1/stores`, que inclui `storeType` do banco `queue.stores`). O draft de cada linha e' SEMPRE re-hidratado do servidor; so se preserva enquanto `touched`/`rowBusy` (edicao pendente). Nao semear draft de fonte parcial (contexto sem `storeType`) — senao o select reverte para 'bairro' no reload mesmo com o banco em 'shopping'.
- [MultiStoreUserAccessCard.vue](/c:/Users/Mike/Documents/Projects/fila-atendimento/web/app/components/multistore/MultiStoreUserAccessCard.vue)
  Card de gerenciamento de acessos, papeis e onboarding de usuarios.

### `ranking`

- [RankingWorkspace.vue](/c:/Users/Mike/Documents/Projects/fila-atendimento/web/app/components/ranking/RankingWorkspace.vue)
  Workspace da tela `/ranking`.
- [RankingTable.vue](/c:/Users/Mike/Documents/Projects/fila-atendimento/web/app/components/ranking/RankingTable.vue)
  Tabela/ranking consolidado de consultores.

### `reports`

- [ReportsWorkspace.vue](/c:/Users/Mike/Documents/Projects/fila-atendimento/web/app/components/reports/ReportsWorkspace.vue)
  Workspace da tela `/relatorios`.
- [ReportsFilterToolbar.vue](/c:/Users/Mike/Documents/Projects/fila-atendimento/web/app/components/reports/ReportsFilterToolbar.vue)
  Barra de filtros, chips e acoes de exportacao.
- [ReportsResultsTable.vue](/c:/Users/Mike/Documents/Projects/fila-atendimento/web/app/components/reports/ReportsResultsTable.vue)
  Tabela principal de resultados/fechamentos.
- [ReportsQualityTable.vue](/c:/Users/Mike/Documents/Projects/fila-atendimento/web/app/components/reports/ReportsQualityTable.vue)
  Tabela de qualidade operacional.
- [ReportsRecentServicesTable.vue](/c:/Users/Mike/Documents/Projects/fila-atendimento/web/app/components/reports/ReportsRecentServicesTable.vue)
  Tabela de ultimos atendimentos para auditoria.

### `settings`

Importante: as configuracoes desta tela sao tenant-wide. Nao existe seletor
interno de loja nesta area, e o seletor de loja do header nao escopa nada
aqui. Toda alteracao salva vale para todas as lojas do tenant. Se for
necessario um override por loja para um item especifico no futuro, isso deve
ser implementado com seletor proprio dentro daquela secao, com aviso visual
explicito de override por loja.

- [SettingsWorkspace.vue](/c:/Users/Mike/Documents/Projects/fila-atendimento/web/app/components/settings/SettingsWorkspace.vue)
  Workspace principal de configuracoes. Mostra um banner permanente
  reforcando o escopo tenant-wide. Os payloads enviados pela
  `useSettingsStore` nao incluem `storeId` ate que um override por loja seja
  modelado.
- [SettingsTabs.vue](/c:/Users/Mike/Documents/Projects/fila-atendimento/web/app/components/settings/SettingsTabs.vue)
  Navegacao interna por abas de configuracao.
- [SettingsConsultantManager.vue](/c:/Users/Mike/Documents/Projects/fila-atendimento/web/app/components/settings/SettingsConsultantManager.vue)
  CRUD de consultores/configuracao do roster.
- [SettingsCrmGoalsSection.vue](/c:/Users/Mike/Documents/Projects/fila-atendimento/web/app/components/settings/sections/SettingsCrmGoalsSection.vue)
  Politica comercial do CRM: faixas de uso da lista, minimo de pedidos para destaque e recebimento por meta. Edicao apenas para `platform_admin` e `director`.
- [SettingsOperationTemplateManager.vue](/c:/Users/Mike/Documents/Projects/fila-atendimento/web/app/components/settings/SettingsOperationTemplateManager.vue)
  Aplicacao e gerenciamento de templates operacionais.
- [SettingsOptionManager.vue](/c:/Users/Mike/Documents/Projects/fila-atendimento/web/app/components/settings/SettingsOptionManager.vue)
  CRUD de catalogos simples como motivos, origens, pausas, fora da vez, perdas e profissao.
  Tambem concentra a ordenacao manual exibida nos selects operacionais.
- [SettingsProductManager.vue](/c:/Users/Mike/Documents/Projects/fila-atendimento/web/app/components/settings/SettingsProductManager.vue)
  CRUD do catalogo de produtos.

### `feedback`

- [FeedbackFormModal.vue](/c:/Users/Mike/Documents/Projects/fila-atendimento/web/app/components/feedback/FeedbackFormModal.vue)
  Modal flutuante para usuarios enviarem sugestoes, duvidas ou reportarem problemas.
  Acessivel via botao flutuante no layout do dashboard. Qualquer usuario autenticado pode enviar feedback.
- [FeedbackWorkspace.vue](/c:/Users/Mike/Documents/Projects/fila-atendimento/web/app/components/feedback/FeedbackWorkspace.vue)
  Workspace administrativo para visualizar, filtrar e responder feedbacks.
  Permitir filtros por tipo (sugestao, duvida, problema) e status (aberto, em analise, resolvido, fechado).
  Acessivel apenas para owner, manager e platform_admin.

### `performance`

Pagina `/performance` (workspace `performance`, so `platform_admin`). Mostra os
resultados da auditoria de navegacao do painel sem precisar re-rodar nada: a
fonte de dados e um modulo TS tipado regenerado pelo `qa-bot/perf_audit.py`
(funcao `write_perf_data_ts`, chamada junto de `write_reports`). Re-rodar a
auditoria sobrescreve `perf-data.ts` e a pagina reflete na hora.

- [perf-data.ts](/c:/Users/Mike/Documents/Projects/fila-atendimento/web/app/components/performance/perf-data.ts)
  AUTOGERADO. `PerfRow { path; mode: 'inapp'|'cold'; t1; t2; t3; capped }` + `PERF_RUN { stamp, baseUrl }` + `PERF_ROWS`. Tempos em ms, media por (rota, modo). Nao editar a mao.
- [usePerformanceData.ts](/c:/Users/Mike/Documents/Projects/fila-atendimento/web/app/components/performance/usePerformanceData.ts)
  Deriva as linhas por rota (in-app + cold lado a lado, ordenadas pela mais lenta), os summaries por modo e os destaques (rota mais lenta, total, realtime capadas).
- [PerformanceWorkspace.vue](/c:/Users/Mike/Documents/Projects/fila-atendimento/web/app/components/performance/PerformanceWorkspace.vue)
  Orquestrador: `AdminPageHeader`, cards de resumo, tabela por rota, ranking in-app/cold e o bloco de warm-up.
- [PerformanceRouteTable.vue](/c:/Users/Mike/Documents/Projects/fila-atendimento/web/app/components/performance/PerformanceRouteTable.vue)
  Tabela por rota com T1/T2/T3 nos dois modos, filtro por path e destaque de T3 lento (>=1.5s aviso, >=3s critico).
- [PerformanceRanking.vue](/c:/Users/Mike/Documents/Projects/fila-atendimento/web/app/components/performance/PerformanceRanking.vue)
  Ranking das rotas mais lentas por T3 num modo (barra proporcional + flag de realtime/cap).
- [PerformanceWarmupNote.vue](/c:/Users/Mike/Documents/Projects/fila-atendimento/web/app/components/performance/PerformanceWarmupNote.vue)
  Bloco explicativo pt-BR do warm-up de dev (custo de compilacao do Vite), dos marcos T1/T2/T3 e do cap de 15s das rotas realtime.

### `ui`

- [AppDialogHost.vue](/c:/Users/Mike/Documents/Projects/fila-atendimento/web/app/components/ui/AppDialogHost.vue)
  Host global dos dialogos e prompts do app.
- [AppDetailDialog.vue](/c:/Users/Mike/Documents/Projects/fila-atendimento/web/app/components/ui/AppDetailDialog.vue)
  Modal reutilizavel para leitura detalhada de entidades administrativas.
- [AppEntityGrid.vue](/c:/Users/Mike/Documents/Projects/fila-atendimento/web/app/components/ui/AppEntityGrid.vue)
  Grade CSS-grid reutilizavel para listagens administrativas com busca, filtros e colunas configuraveis.
- [AppToastStack.vue](/c:/Users/Mike/Documents/Projects/fila-atendimento/web/app/components/ui/AppToastStack.vue)
  Host global das notificacoes/toasts. Renderiza automaticamente via Teleport. Suporta tres tipos: `success`, `error` e `info`.

  **Animacoes:**
  - Entrada: slide suave da direita com fade in (0.4s)
  - Ícones se desenhando: checkmark (sucesso), X (erro) ou fade in (info) com efeito de escala e rotacao (0.4s + 0.5s)
  - Barra de progresso: anima diminuindo de 100% a 0% (duração: 4s para sucesso/info, 5.5s para erro)
  - Reposicionamento: quando um toast sai, os demais sobem com animacao fluida (0.4s cubic-bezier)
  - Saida: slide e fade para direita (0.3s)

  **Uso:**

  ```javascript
  const ui = useUiStore()
  ui.success('Operacao concluida!', 'Sucesso') // Desaparece em 4s
  ui.error('Algo deu errado', 'Erro') // Desaparece em 5.5s
  ui.info('Informacao importante', 'Info') // Desaparece em 4s
  ```

  O componente e automaticamente adicionado no layout e gerencia a pilha de notificacoes. Toasts são descartáveis pelo usuario (botão fechar).

- [AppSelectField.vue](/c:/Users/Mike/Documents/Projects/fila-atendimento/web/app/components/ui/AppSelectField.vue)
  Select reutilizavel para filtros e escolhas simples de uma opcao, com dropdown custom no padrao visual do `product-pick`.
- [AppToggleSwitch.vue](/c:/Users/Mike/Documents/Projects/fila-atendimento/web/app/components/ui/AppToggleSwitch.vue)
  Switch compacto para status booleanos em cards e grades administrativas.

### `users`

- [UsersAccessManager.vue](/c:/Users/Mike/Documents/Projects/fila-atendimento/web/app/components/users/UsersAccessManager.vue)
  Grade administrativa reutilizavel para usuarios, com cadastro via `+`, filtros, detalhes em modal editavel e acoes por icone.
  O inline da grade deve preferir patch local da linha e deixar o websocket reconciliar em segundo plano, sem reabrir o estado de loading da tabela a cada alteracao.
  Para `platform_admin`, a grade pode liberar manutencao inline de contas `consultant` quando isso for necessario no ambiente de desenvolvimento.
- [UsersRoleMatrixManager.vue](/c:/Users/Mike/Documents/Projects/fila-atendimento/web/app/components/users/UsersRoleMatrixManager.vue)
  Editor da matriz padrao por perfil, com visibilidade e capacidade de edicao por workspace.
- [UsersWorkspace.vue](/c:/Users/Mike/Documents/Projects/fila-atendimento/web/app/components/users/UsersWorkspace.vue)
  Workspace dedicado da area administrativa de usuarios, com abas para usuarios e matriz por perfil.

### `calendar`

Pagina `/calendario` (`pages/calendario/index.vue`; workspace global, layout `dashboard`,
`definePageMeta workspaceId: ''` para nao cair no gate de workspace). A config NAO e mais pagina:
`pages/calendario/config.vue` virou um REDIRECT (SPEC-F6) que manda pra `/calendario?config=responsaveis`
(preserva o link antigo); a config vive num drawer lateral (ver `CalendarConfigDrawer.vue` abaixo).
Calendario de conteudo por cliente da agencia. Layout em
colunas: [coluna esquerda = controles + notas, UM card] [week rail S1..Sn] [calendario (scroll)]
[drawer do dia]. DUAS VISOES (Mes / Semana), toggle nos controles. Estado em [stores/calendar.ts];
helpers de data/constantes em [utils/calendar.ts]; tipos+helpers da CONFIG (contrato C2) em
[utils/calendar-config.ts](/c:/Users/Mike/Documents/Projects/fila-atendimento/web/app/utils/calendar-config.ts)
(re-exportados por utils/calendar.ts); nav em `layers/queue/nav.config.ts`.

> **CONFIG v2/v3 (SPEC-F3/F6)**: `CalendarConfig` (jsonb `calendar.config`) traz alem de responsaveis+
> feriados: `weekStartsOn` (sunday|monday), `clientColors` ({ [clientId]: `#rrggbb`|`none` }),
> `typeColors` ({ [tipo]: `#rrggbb` }), `whiteLabel` (logo/titulo/cor), `ai` e `tasks` ({ boardId,
> defaultColumnId } — vazio = integracao com Tasks DESLIGADA; contrato C6). `normalizeConfig`
> (calendar-api.ts) faz merge POR SECAO **incluindo `tasks`/`chat`** (linha antiga do banco ganha o shape
> completo; draft sem a secao NUNCA apaga o valor persistido no full-replace do PUT). APLICACAO no
> calendario: `weekStartsOn` da config espelha no viewport (store observa `config.weekStartsOn`); a cor do
> cliente vem de `resolveClientColor(clientColors[id], semente)` no `clients` computed; `typeColors` desce
> como prop `typeColors` por MonthGrid/WeekView/DayCell -> EventChip (override da cor do cliente).

> **CONFIG v4 — IA 100% pelo painel (SPEC-F1, contratos CFG/SEC)**: o `ai` da config ganhou `enabled`
> (kill switch), `useGlobalKeys` (true = chaves GLOBAIS da plataforma; false = chaves DESTA conta),
> `transcribeProvider` (openai|gemini) e `transcribeModel`, alem do ja existente provider/model/baseUrl/
> systemPrompt/temperature. Novo bloco `chat` ({ position: center|left|right, width, height }) pro layout
> da janela de chat (aplicado na SPEC-F2 pelo [CalendarChatPanel.vue] + composable useCalendarChatWindow). O enum de provider ganhou `openai` (base `https://api.openai.com/v1`,
> label "OpenAI") nos 3 mapas de `utils/calendar-config.ts` (+ o `gemini` da wave 2). As **CHAVES de API
> NUNCA moram na config nem no n8n**: vivem em secrets server-side (contrato SEC); o front so recebe status
> MASCARADO `{set,last4}` via `GET /v1/calendar/ai-keys` (fonte ativa global|conta) e grava write-only
> (`PUT /ai-keys` conta, `PUT /ai-keys/global` so platform_admin; apiKey vazio = limpar). I/O em
> `calendar-api.ts` (`fetchAiKeys`/`putAiKey`/`fetchGlobalAiKeys`/`putGlobalAiKey`, tipo `CalendarAiKeys`).
> A aba IA ([ConfigAi.vue]) tem o kill switch, o toggle de escopo (aviso "salve pra aplicar" enquanto o
> rascunho diverge do salvo), o subcomponente [ConfigAiKeys.vue] (chaves mascaradas + input write-only +
> limpar, edicao gateada por `isPlatformAdmin` quando a fonte ativa e global), provider+modelo, transcricao
> e o prompt do sistema (a lei da IA). ConfigAiKeys le o escopo ATIVO do banco (prop `useGlobalKeys` do
> `store.config`, nao do rascunho) e re-le apos cada PUT (fonte unica = banco).

> **CONFIG WAVE 3.1 — escopo da IA por cliente (SPEC-F3, contratos CFG+/SEC+)**: o `ai` da config ganhou
> `scopeMode` (`general` | `perClient`, default general) e `disabledClientIds` (`string[]` — no modo geral,
> clientes com a IA DESLIGADA). `normalizeConfig` (calendar-api.ts) coere os dois na secao `ai` (enum +
> filtro de strings; nunca apaga no full-replace). Novo tipo `CalendarClientAiOverride` (COMPORTAMENTO por
> cliente, SEM chaves): `{ enabled: boolean|null, provider: CalendarAiProvider|'', model, baseUrl,
systemPrompt, temperature: number|null }` — cada campo null/'' = HERDA a config geral; em
> `utils/calendar-config.ts` (`defaultClientAiOverride`/`normalizeClientAiOverride`/`isEmptyClientAiOverride`,
> re-exportados por utils/calendar.ts). I/O em calendar-api.ts: `GET/PUT /v1/calendar/ai-config/client?clientId=`
> (`fetchClientAiConfig`/`putClientAiConfig`; account_id NUNCA no body/query — o back resolve pelo Principal;
> override vazio `{}` = usa a config geral). Sub-aba [ConfigAiClientScope.vue] (extraida do ConfigAi p/ nao
> passar de 450 linhas), montada como `<details>` "Escopo por cliente" na aba IA: seletor Geral × Individual
> (`ai.scopeMode`). GERAL = multi-select (checkbox) de clientes p/ DESATIVAR a IA (`ai.disabledClientIds`,
> parte do draft compartilhado — salva no footer). INDIVIDUAL = seletor de cliente (`store.clients`) + form de
> override (status tri-state Herdar/Ligada/Desligada, provider, modelo, baseUrl, temperatura, prompt) salvo
> POR CLIENTE com botao proprio via `putClientAiConfig`; badge "usa config geral" quando o override persistido
> e' vazio; dirty-guard ao trocar de cliente com edicao pendente (`ui.confirm`, padrao unico da casa). As
> CHAVES de API NAO aparecem aqui — seguem no nivel conta/global (SEC). Estilo `.calendar-config__client-scope`
> em `assets/styles/calendar/config.css`.

> **Liquid glass / aurora ambiente**: a AURORA de fundo (camada animada de gradientes que os
> cards de vidro `backdrop-filter` refratam) NAO vive mais no `/calendario`. Virou parte do TEMA
> proprio **Liquid Glass**: `.theme-liquidglass .module-workspace-full::before` /
> `.theme-liquidglass .workspace::before` em `assets/styles/omni-tokens.css` (z-index:-1, atras do
> conteudo), so aparece quando o tema Liquid Glass esta ativo (selecionavel no Theme Studio). O
> `shell.css` do calendario so mantem os cards de vidro. Tema/aparencia sao GLOBAIS da plataforma
> (modulo `theme` no back: `/v1/platform/appearance`), nao mais no queue/settings. Conversao do
> resto da UI = fase `liquid-glass-ui` (GLASS) no roadmap; plano em docs/THEME_MODULE_PLAN.md.

Interacao-chave:

- **Scroll inteligente (CSS Scroll Snap, `scroll-snap-type: y mandatory`)**: o bloco em foco
  (CARD DE MES na visao Mes, BLOCO DE SEMANA na visao Semana) e alvo do snap
  (`.calendar-snap` = `scroll-snap-align: center`), ficando SEMPRE centralizado com peek do
  anterior e do proximo. Scroll infinito nas duas direcoes (`renderedMonthKeys`/`renderedWeekKeys`).
- **Foco segue o scroll**: o bloco mais proximo do centro vira o foco (`focusMonthKey` /
  `focusWeekKey`); titulo, glow, week rail e notas acompanham. `data-block-key` alimenta a
  deteccao; `data-focus="true"` marca o bloco em foco (usado no auto-center).
- Toggle **Mes/Semana** e clicar em **S1..Sn** (rail) entram na visao Semana daquela semana.
  Clicar num dia/chip abre o drawer e move o foco.

> **DADOS 100% REAIS (sem mock/legado)**: EVENTOS, NOTAS, RESPONSAVEIS, MEMBROS, CONFIG e
> FERIADOS vem do back real (`/v1/calendar/*`, modulo Go `back/internal/modules/calendar`) via
> `createApiRequest` no `stores/calendar.ts` (janela por `from`/`to`; notas por mes com save
> debounced; CRUD com refetch). CLIENTES reais do `useTenantsStore`. RESPONSAVEIS = usuarios reais
> da conta (`/v1/calendar/responsibles`, subconjunto configuravel); `store.people` = responsaveis.
> FERIADOS computados no back (`/v1/calendar/holidays?from=&to=`, conjuntos BR nacional / Sergipe /
> Aracaju / luxo internacional, incl. moveis via Pascoa) e ligados/desligados na config. O composable
> `useCalendarData` (mock) foi REMOVIDO. **ANEXOS (Fase 3)**: imagem/video no evento
> (`event.media = CalendarMediaItem[]`) e avulsos no dia; upload via `useCalendarMedia` (XHR com
> progresso real) → `POST /v1/calendar/media`, limite de video GLOBAL da plataforma
> (`/v1/calendar/media-limits`, default 300MB). Video gera tambem um `posterUrl` (thumb): apos subir
> o video, `useCalendarMedia.uploadVideoWithPoster` captura o 1o frame via
> [utils/calendar-poster.ts](/c:/Users/Mike/Documents/Projects/fila-atendimento/web/app/utils/calendar-poster.ts)
> (`<video>`+`<canvas>`, JPEG 640px) e sobe como imagem normal; falha do poster NAO falha o upload.
> **Fundo do dia (SPEC-F2)**: os anexos avulsos por dia vem do store via
> [composables/useCalendarDayMedia.ts](/c:/Users/Mike/Documents/Projects/fila-atendimento/web/app/composables/useCalendarDayMedia.ts)
> (estado `dayMediaByDate` + `fetchDayMedia`/`saveDayMedia`, extraido do store na SPEC-F3 p/ manter
> < 450 linhas; `GET /v1/calendar/day-media?from=&to=` via `calendarApi.fetchDayMediaInRange`, buscado
> na MESMA janela debounced dos eventos e zerado na troca de conta; PUT via `calendarApi.putDayMedia`).
> [MonthGrid]/[WeekView] passam `bgUrls` por dia (helper `dayBackgroundUrls`) e [DayCell] pinta o fundo. Rota/nav ainda sem gate proprio no front (preview); o
> gate de API `/v1/calendar` ja existe (platform_admin bypassa). Fases seguintes (white-label, perfil
> do cliente + IA, aprovacao WhatsApp) em
> [docs/CALENDARIO_PLAN.md](/c:/Users/Mike/Documents/Projects/fila-atendimento/docs/CALENDARIO_PLAN.md).

- [CalendarControls.vue](/c:/Users/Mike/Documents/Projects/fila-atendimento/web/app/components/calendar/CalendarControls.vue)
  Topo da coluna esquerda (uma linha): titulo do mes + **botao chat (`@chat`, message-circle)** +
  **botao IA (`@ai`, sparkles)** + **engrenagem (`@config`)** + **select de cliente** (Todos/especifico,
  via [AppSelectField]) + toggle Mes/Semana + Hoje + botao "Novo". O `@config` ABRE O DRAWER de config
  no proprio calendario (estado local `configOpen` no index; SPEC-F6, antes navegava pra
  `/calendario/config`); o `@ai` abre o [CalendarAiPlanModal.vue] (SPEC-F5); o `@chat` reabre a janela
  do assistente (`chat.openPanel()`; SPEC-F2, substitui o antigo FAB de canto).

> **IA do mes (SPEC-F5, contrato C4/C5)**: o botao sparkles do [CalendarControls] abre o
> [CalendarAiPlanModal.vue](/c:/Users/Mike/Documents/Projects/fila-atendimento/web/app/components/calendar/CalendarAiPlanModal.vue)
> (Teleport body; fecha por X, Esc e clique no backdrop). Fluxo: seleciona 1+ clientes (multi) +
> mes (default = `focusMonthKey`) + resumo do provider/modelo da config com link "configurar" ->
> /calendario/config; "Gerar plano" -> `POST /v1/calendar/ai/plan` -> polling `GET
/v1/calendar/ai/plans/{id}` a cada 3s (max 5min, PARA ao fechar o modal); 503 `ai_not_configured`
> vira aviso acionavel (envs `CALENDAR_AI_WEBHOOK_URL`/`_SERVICE_TOKEN`/`_CALLBACK_BASE` + import do
> workflow no n8n). Resultado renderizado por
> [CalendarAiPlanResult.vue](/c:/Users/Mike/Documents/Projects/fila-atendimento/web/app/components/calendar/CalendarAiPlanResult.vue)
> (summary + pilares + por cliente os dias). Acoes: **"Aplicar nas notas"** (anexa HTML formatado a
> nota do mes-alvo — mes ativo via `store.setNotesForActiveMonth`, senao GET+append+PUT no composable
> via `calendarApi.fetch/putNotesForMonth`) e **"Criar eventos"** (loop `store.createEvent`:
> title=idea, description=copy, type mapeado por `planTypeToEventType` (fora do enum -> post), status
> planejado, priority media, clientId do bloco, date do item) -> depois `POST
/v1/calendar/ai/plans/{id}/applied`; reaplicar plano ja `applied` pede confirmacao (`ui.confirm`)
> para nao duplicar em silencio. Lista dos planos anteriores do mes (index lean, `GET
/v1/calendar/ai/plans?month=`) com abrir/excluir. I/O + polling em
> [composables/useCalendarAiPlans.ts](/c:/Users/Mike/Documents/Projects/fila-atendimento/web/app/composables/useCalendarAiPlans.ts)
> (account_id nunca no body — o back resolve pelo Principal); tipos+helpers (normalizacao,
> `planContentToNotesHtml` com escape de HTML, `planTypeToEventType`) em
> [utils/calendar-ai.ts](/c:/Users/Mike/Documents/Projects/fila-atendimento/web/app/utils/calendar-ai.ts)
> (re-exportados por utils/calendar.ts). Endpoints (calendar-api.ts): `POST /v1/calendar/ai/plan`
> (body `{month, clientIds}` -> `{id, status}`), `GET /v1/calendar/ai/plans?month=` (index lean),
> `GET /v1/calendar/ai/plans/{id}` (completo com content), `POST /v1/calendar/ai/plans/{id}/applied`,
> `DELETE /v1/calendar/ai/plans/{id}`. Estilos `.calendar-ai*` + `.calendar-ai-result__*` em
> `assets/styles/calendar/ai.css` (so tokens; o HTML de nota vem do plano via editor TipTap).

- **Drawer de config** [CalendarConfigDrawer.vue](/c:/Users/Mike/Documents/Projects/fila-atendimento/web/app/components/calendar/config/CalendarConfigDrawer.vue)
  (SPEC-F6; substitui a antiga pagina `/calendario/config`, hoje so um redirect, e o antigo
  `CalendarConfigModal.vue` DELETADO). Sobe sobre [OmniEntityDrawer](/c:/Users/Mike/Documents/Projects/fila-atendimento/web/app/components/ui/OmniEntityDrawer.vue)
  (modo `side`), montado no `pages/calendario/index.vue` via `v-model:open="configOpen"`. 7 ABAS (nav
  `.calendar-config__tabs`): responsaveis · feriados · aparencia · ia · clientes · integracoes · midia.
  Deep-link `?config=<aba>`: o index abre o drawer sempre que ha `?config` na URL (watch imediato,
  cobre mount/redirect/nav in-app como o link "configurar" do modal de IA -> `?config=ia`); o drawer
  resolve a aba pela query e escreve `router.replace` ao trocar de aba, e limpa `?config` ao fechar.
  Abas so montam na 1a visita (`visited` Set) e ficam vivas via `v-show` (preserva estado das abas de
  salvar-proprio ao trocar). MODELOS DE SALVAR: responsaveis/feriados/aparencia/ia/integracoes
  compartilham um `draft` do `CalendarConfig` (re-hidratado de `store.config` enquanto nao `touched`)
  - botao "Salvar configuracoes" no FOOTER do drawer -> `store.saveConfig` (PUT `/v1/calendar/config`);
    clientes e midia salvam com botao PROPRIO na aba (footer some nessas abas). Dirty-guard UNICO via
    `ui.confirm`: fechar com o draft compartilhado sujo pergunta antes de descartar. No open o drawer
    refaz `store.fetchConfig()` + `store.fetchMembers()`. Secoes em `components/calendar/config/`
    (estilos `.calendar-config__*` + `.calendar-config-drawer*` em `assets/styles/calendar/config.css`):
  * `ConfigResponsibles.vue` — checkboxes dos usuarios da conta (`store.members`; vazio = todos).
  * `ConfigHolidays.vue` — toggles BR nacional / Sergipe / Aracaju / luxo internacional.
  * `ConfigAppearance.vue` — inicio da semana (seg/dom) + cor por cliente (input color + "Sem cor" =
    `none`) + cor por tipo (checkbox "Usar" + input color) + white-label (titulo/logo/cor).
  * `ConfigAi.vue` — provider (select, inclui `gemini`) + modelo + baseUrl (placeholder = default do
    provider) + systemPrompt (textarea) + temperature; AVISO fixo "as chaves de API ficam no n8n, nunca aqui".
    Tem botao "Abrir chat com o assistente" que so chama `useCalendarChat().openPanel()` — a janela de
    chat (SPEC-F2) vive montada na pagina index (fora do drawer), aqui so aciona o MESMO estado singleton.
  * `ConfigTasks.vue` (SPEC-F6, aba `integracoes`) — select de board + coluna de destino ao criar task
    a partir de um evento (contrato C6, `draft.tasks`). Fonte: `useTasksStore` (import cross-layer
    `../../../../layers/tasks/stores/tasks`), boards carregados LAZY so ao abrir a aba
    (`initialize({ allowAutoCreate:false })`, sem criar board fantasma). Sem board -> aviso acionavel
    com link pra `/tasks`. Trocar de board invalida a coluna se ela nao pertence ao novo board.
  * `ConfigMediaLimits.vue` — tetos GLOBAIS de upload; GET sempre (via `useCalendarMedia`), edicao
    so `platform_admin` (`auth.role === 'platform_admin'`; o back tambem restringe o PUT). Salva por
    `useCalendarMedia().saveMediaLimits` (PUT `/v1/calendar/media-limits`), independente do config.
  * `ConfigClientProfiles.vue` (SPEC-F4) — PERFIL ESTRATEGICO por cliente (contrato C3), usado pelo
    assistente de IA do mes. Select de cliente (`store.clients`) com badge preenchido/vazio (via
    `filled` do index) + form dos campos estaveis (segmento, posicionamento, site, instagram,
    endereco, descricao, historia, objetivos, tom de voz) + textareas do bloco `extra` (publico-alvo,
    oferta, pilares, cadencia, restricoes, performance, assets). Salva POR CLIENTE (botao proprio,
    independente do "Salvar configuracoes" global) + feedback ui.success/error; dirty guard ao trocar
    de cliente com edicao pendente (`ui.confirm` — padrao unico da casa, SPEC-F6). I/O em
    [composables/useCalendarClientProfiles.ts](/c:/Users/Mike/Documents/Projects/fila-atendimento/web/app/composables/useCalendarClientProfiles.ts)
    (index + load/save; `account_id` nunca no body — o back resolve pelo Principal); tipos+defaults em
    [utils/calendar-profile.ts](/c:/Users/Mike/Documents/Projects/fila-atendimento/web/app/utils/calendar-profile.ts)
    (re-exportados por utils/calendar.ts). Endpoints (calendar-api.ts): `GET /v1/calendar/client-profile?clientId=`,
    `PUT /v1/calendar/client-profile` (upsert full-replace), `GET /v1/calendar/client-profiles` (index
    lean `{clientId,filled,updatedAt}`). Perfil inexistente = 200 com defaults (nunca 404). Estilos
    `.calendar-profile__*` + `.calendar-config__section--wide` em `assets/styles/calendar/config.css`.
- **Janela de chat + voz** [CalendarChatPanel.vue](/c:/Users/Mike/Documents/Projects/fila-atendimento/web/app/components/calendar/CalendarChatPanel.vue)
  (SPEC-F2/F10, contratos C7/C8/CHATUI/D3/D4). **SEM FAB de canto** (removido na F2): a janela abre CENTRALIZADA
  sobre a area interna do calendario (`.calendar-page`, medida em runtime) e ganhou **MINIMIZAR**
  (colapsa numa **pill** re-expansivel `.calendar-chat-pill`, sem perder a conversa) e **FECHAR**
  (some; reabre pelo botao chat dos [CalendarControls] ou pelo "Abrir chat" da aba IA). **Posicao/tamanho
  (`config.chat`)**: seletor no header (`center` = largura da area do calendario; `left` = ~painel
  esquerdo 360px; `right` = ~modal direito 560px) + **resize por arrasto** (handle no canto inferior,
  molde do OmniEntityDrawer; no modo right cresce pela borda esquerda). Toda a matematica de layout +
  persistencia vive em
  [composables/useCalendarChatWindow.ts](/c:/Users/Mike/Documents/Projects/fila-atendimento/web/app/composables/useCalendarChatWindow.ts)
  (mede `.calendar-page`, calcula `panelStyle` left/top/width/height em px, clamps MIN 320 / margem 12,
  persiste em `config.chat` via `store.saveConfig` DEBOUNCED 600ms — trocar de posicao zera width/height
  pro default; `localChat` re-hidrata de `store.config.chat` exceto com save pendente, principio 1).
  Render via `Teleport to="body"` (a `.calendar-page` tem `overflow:hidden`; precedente CalendarAiPlanModal)
  com `position:fixed` + style calculado; `.calendar-chat`/`.calendar-chat-pill` z-index 9810 em
  `assets/styles/calendar/chat.css` (so tokens). Montado 1x em `pages/calendario/index.vue`. Bolhas
  user/assistant espelham o OperationSidePanel; input textarea auto-grow (Enter envia, Shift+Enter quebra
  linha), "digitando...", header com seletor de posicao + nova conversa + minimizar + fechar (Esc fecha,
  preservando o draft). Estado SINGLETON via
  [composables/useCalendarChat.ts](/c:/Users/Mike/Documents/Projects/fila-atendimento/web/app/composables/useCalendarChat.ts)
  (`useState`: messages/draft/sending/errorMessage/**panelOpen/minimized**/conversationId + wave 4:
  **conversationTitle/conversations/loadingConversations/loadingConversation/chatScope/scopeMode/
  scopeClientId**; `openPanel` reabre cheia zerando `minimized` E dispara `ensureChatLoaded()` — busca
  a lista de conversas + o escopo do banco; `minimize`/`restore` alternam a pill): `ask()` -> POST
  `/v1/calendar/chat/ask` com `{question, conversationId, scopeMode, scopeClientId, month}` (account*id
  nunca no body — o back resolve pelo Principal), a RESPOSTA `{answer, conversationId, title}` adota o
  id/titulo que o back resolveu e atualiza a lista; AbortController cancela a pergunta anterior em voo;
  erros 503/`chat_not_configured`, 502/504 viram `errorMessage` acionavel (cita `CALENDAR_CHAT_WEBHOOK_URL`).
  **WAVE 4 — PERSISTENCIA + MEMORIA + ESCOPO (SPEC-F10, contrato D3/D4)**: conversas e mensagens agora
  PERSISTEM no banco (`calendar.chat*_`), entao o historico NAO some no reload e a IA tem MEMORIA (o back
carrega as ultimas N mensagens). `openConversation(id)`carrega as mensagens do banco (SUBSTITUI as
locais) e adota o escopo salvo;`newConversation()`limpa e zera o id (o back cria a conversa no 1o`ask`, lazy — sem conversas vazias); `removeConversation(id)` soft-delete. I/O em
[domain/calendar/calendar-chat-api.ts](/c:/Users/Mike/Documents/Projects/fila-atendimento/web/app/domain/calendar/calendar-chat-api.ts)
(`fetchConversations`/`getConversation`/`createConversation`/`deleteConversation`/`fetchChatScope`+
tipos`CalendarChatConversation`/`CalendarChatScope`; separado de calendar-api.ts p/ manter < 450
linhas). SEGURANCA: o ACESSO (quais conversas/clientes o usuario ve) e resolvido SEMPRE server-side
pela permissao (`resolveChatAccess`, nunca do body); conversa/cliente fora do visivel => 404. O escopo
(client|all) vem do `GET /chat/scope`: cliente-side (canSelect=false) trava no `lockedClientId`, agencia
escolhe. **WAVE 4 — SELECT DE ESCOPO (SPEC-F11)**: barra abaixo do header via
[CalendarChatScope.vue](/c:/Users/Mike/Documents/Projects/fila-atendimento/web/app/components/calendar/CalendarChatScope.vue)
(apresentacional: recebe `scope`/`mode`/`clientId`e emite`change(mode, clientId)`->`chat.setScope`;
`<select>`NATIVO — fecha no clique-fora/Esc sozinho, acessivel/mobile). So renderiza com`canSelect=true`
(agencia/multi-cliente): opcoes "Todos os clientes" (`scopeMode='all'`) + cada cliente visivel
(`scopeMode='client'`); cliente-side nao ve o seletor. A escolha viaja no `ask()`e fica salva na conversa;`openConversation`adota o escopo salvo dela; default do`applyScopeDefault`(cliente-side ->`lockedClientId`;
agencia -> cliente filtrado na tela se visivel, senao "Todos"). Estilos `.calendar-chat-scope_`em`assets/styles/calendar/chat-scope.css`(so tokens). Menu "Conversas" no header via
[CalendarChatConversations.vue](/c:/Users/Mike/Documents/Projects/fila-atendimento/web/app/components/calendar/CalendarChatConversations.vue)
(dropdown apresentacional: recebe a lista + emite select/new/delete; fecha no clique-fora/Esc; agencia
ve todas com autor+data, cliente-side so as suas; estilos`.calendar-chat-convos\*`em`assets/styles/calendar/chat-conversations.css`).
VOZ: [composables/useVoiceRecorder.ts](/c:/Users/Mike/Documents/Projects/fila-atendimento/web/app/composables/useVoiceRecorder.ts)
(`MediaRecorder` `audio/webm;codecs=opus`-> fallback`audio/mp4`, limite 2min para sozinho, estados
idle/recording/transcribing, permissao negada -> mensagem acionavel). Botao mic: gravar -> parar ->
POST multipart `/v1/calendar/chat/transcribe`(campo`file`; FormData nao serializado pelo api-client)
-> texto entra no INPUT (usuario revisa e envia, nao envia direto); erros C8 (503/413/400/502/504)
com mensagem acionavel. A pill (minimizada) mostra um badge quando ha `errorMessage`.
- **Realtime + presenca (SPEC-F9, contratos C11/C12)**. Dois composables NOVOS moldados sobre a
  base generica de tasks (import cross-layer `../../layers/tasks/composables/useRealtimeSocket`;
  a conta e' resolvida pela cadeia `resolveRealtimeAccountId`, NUNCA so `auth.activeTenantId`),
  montados 1x em `pages/calendario/index.vue` (desligam no unmount; troca de conta reconecta):
  - [composables/useCalendarRealtime.ts](/c:/Users/Mike/Documents/Projects/fila-atendimento/web/app/composables/useCalendarRealtime.ts)
    — canal por conta (`/v1/realtime/calendar`, scope=account, topico `calendar:account:{id}`).
    Aplica por INVALIDACAO (o WS so avisa "mudou", o front refaz o fetch, nunca patch local):
    `calendar.event_*`/`calendar.day_media_updated` -> `store.refetchWindow()` (debounce 250ms,
    coalesce de rajada); `calendar.note_updated` -> `store.reloadNoteFromRemote(monthKey)` (SO se a
    nota ja carregada e SEM save pendente — o rascunho local vence, principio 1); `calendar.config_updated`
    -> `store.fetchConfig()`; `calendar.plan_updated` -> `lastPlanEvent` repassado ao `CalendarAiPlanModal`
    (`:plan-event`) que recarrega o plano ativo e encerra o polling. Guard de conta (defesa em
    profundidade): descarta evento cujo `accountId` != conta resolvida.
  - [composables/useCalendarPresence.ts](/c:/Users/Mike/Documents/Projects/fila-atendimento/web/app/composables/useCalendarPresence.ts)
    — presenca estilo Google Docs, versao REDUZIDA (so INDICADOR, sem lock nem sync de draft na v1;
    `/v1/realtime/presence`, scope=calendar, topico `presence:calendar:{id}`). Heartbeat 15s;
    `participants` (exclui o proprio usuario); `focusField`/`blurField` com fieldKey `notes:YYYY-MM`
    (editor de notas) e `event:<id>` (form de edicao). UI: avatares no [CalendarControls]
    (`:participants`, `.calendar-controls__presence*` em shell.css) + badge "Fulano editando" no
    [MonthNotesPanel] (`:editing-label` + `@focus/@blur` do editor via focusin/focusout;
    `.calendar-notes__presence` em notes-drawer.css) e no [CalendarEventForm] (`:editing-label`,
    `.calendar-form__presence` em week-form.css; so ao EDITAR, presenca ligada por watch no index).
  - **Optimistic locking C12**: `CalendarEvent.version` guardado no store; `calendarApi.putEvent`
    envia header `If-Match: <version>`; o `updateEvent` (agora em
    [composables/useCalendarEventCrud.ts](/c:/Users/Mike/Documents/Projects/fila-atendimento/web/app/composables/useCalendarEventCrud.ts),
    EXTRAIDO do store na F9 p/ manter < 450 linhas) devolve `'ok' | 'conflict' | 'error'`. No 409
    `version_conflict` o index abre `ui.confirm` "alterado por outra pessoa" + "Recarregar"
    (`store.getEventById` re-hidrata o form com a versao do banco); sem confirmar, o rascunho do
    usuario NAO e descartado. Sem `If-Match` = comportamento antigo (compat).
- [CalendarWeekRail.vue](/c:/Users/Mike/Documents/Projects/fila-atendimento/web/app/components/calendar/CalendarWeekRail.vue)
  Rail vertical na BORDA ESQUERDA: `M` (volta pra visao Mes) + `S1..Sn` (semanas do mes em
  foco). Auto-detecta as semanas (`weeksOfFocusedMonth`); uma linha com <2 dias do mes (ex.: so
  o domingo) NAO conta. Ativo (M na visao Mes, Sn na visao Semana) fica primary-preenchido;
  marca a semana vigente.
- [MonthGrid.vue](/c:/Users/Mike/Documents/Projects/fila-atendimento/web/app/components/calendar/MonthGrid.vue)
  Card do mes (glass, `.calendar-snap`) com cabecalho + grade de 7 colunas de [DayCell.vue].
- [WeekView.vue](/c:/Users/Mike/Documents/Projects/fila-atendimento/web/app/components/calendar/WeekView.vue)
  Bloco de UMA semana (glass, `.calendar-snap`) — 7 colunas altas com todos os eventos do dia;
  a visao Semana e um scroll infinito desses blocos.
- [DayCell.vue](/c:/Users/Mike/Documents/Projects/fila-atendimento/web/app/components/calendar/DayCell.vue) +
  [EventChip.vue](/c:/Users/Mike/Documents/Projects/fila-atendimento/web/app/components/calendar/EventChip.vue)
  Celula do dia (numero, HOJE, marcadores de **feriado** em `--accent-warning`, ate N chips +
  "+N mais"); chip com a cor do cliente OU a **cor por tipo** quando setada na config (prop
  `typeColors` desce MonthGrid/WeekView -> DayCell -> `EventChip.typeColor`; override so quando
  ha `#rrggbb` para o tipo). Feriados vem de `store.holidaysByDate` (Map por data),
  passados por [MonthGrid.vue] / [WeekView.vue]. **Fundo do dia (SPEC-F2)**: prop `bgUrls?: string[]`
  (ate 4) renderiza `.calendar-cell__bg` (absolute, atras do conteudo) em grade por `data-count`
  (1 inteiro / 2 colunas / 3 = 1 alto + 2 / 4 = 2x2) + overlay `rgb(var(--surface)/alpha)` para
  legibilidade nos dois temas; conteudo com `z-index:1`. As URLs vem do helper puro
  `dayBackgroundUrls(events, dayMedia)` (utils/calendar): midias dos EVENTOS filtrados primeiro
  (imagem->`url`, video->`posterUrl`, video sem poster pulado), senao os anexos avulsos do dia.
  [WeekView.vue] reusa as mesmas classes `.calendar-cell__bg*`.
- [MonthNotesPanel.vue](/c:/Users/Mike/Documents/Projects/fila-atendimento/web/app/components/calendar/MonthNotesPanel.vue)
  Notas por mes (segue o foco); reutiliza o [OmniEditor](/c:/Users/Mike/Documents/Projects/fila-atendimento/web/app/components/omni/OmniEditor.vue)
  (TipTap). Preenche a coluna abaixo dos controles.
- [DayDrawer.vue](/c:/Users/Mike/Documents/Projects/fila-atendimento/web/app/components/calendar/DayDrawer.vue)
  Drawer do dia (`role="dialog"`): detalhe do item (pilulas cliente/tipo, Status/Prioridade/
  Responsavel/Horario) + **midia do post** (uploader readonly) + **Editar/Excluir** + lista dos itens
  - **Anexos do dia** (uploader editavel). Fonte unica no store: le `store.selectedDayMedia` (do Map
    `dayMediaByDate` buscado na janela, sem refetch por dia) e salva por `store.saveDayMedia(date, media)`
    (PUT + atualiza o Map). Espelha o modal de Tasks (DESIGN_SYSTEM §9).
  - **Task vinculada (SPEC-F8, contrato C10)**: quando `activeEvent.taskId` != '' mostra o link
    `.calendar-drawer__tasklink` -> `/tasks` (NuxtLink). Ainda SEM deep-link para a task especifica no
    board (o board `/tasks` nao le query `?task=`); leva pra pagina e o usuario acha a task. `EventChip`
    NAO ganhou badge (anti-poluicao visual). Estilo em `week-form.css`.
- [CalendarMediaUploader.vue](/c:/Users/Mike/Documents/Projects/fila-atendimento/web/app/components/calendar/CalendarMediaUploader.vue)
  Widget reutilizavel de anexos (`v-model` = `CalendarMediaItem[]`): grade de previews (img/`video`)
  - remover, tile de adicionar (input file oculto), barra de progresso por upload e validacao de
    tipo/tamanho no cliente (contra `useCalendarMedia().mediaLimits`). `readonly` = so preview (midia do
    post no drawer). Usado no [CalendarEventForm.vue] e no [DayDrawer.vue]. Clicar num item abre o
    [CalendarMediaViewer.vue] (botao remover via `@click.stop`); thumb de video usa `posterUrl` como
    `<img>` quando existe (senao `<video preload="metadata">`). Video sobe por `uploadVideoWithPoster`.
- [CalendarMediaViewer.vue](/c:/Users/Mike/Documents/Projects/fila-atendimento/web/app/components/calendar/CalendarMediaViewer.vue)
  Overlay em tela cheia (Teleport body) para imagem OU `<video controls autoplay :poster>`. Props
  `items: CalendarMediaItem[]` + `startIndex`; navegacao < > (setas do teclado tambem), nome+tamanho
  no rodape. Fecha por X, Esc E clique no backdrop (as tres coexistem). Estilos em
  `assets/styles/calendar/media.css` (`.calendar-viewer__*`, so tokens).
- [CalendarEventForm.vue](/c:/Users/Mike/Documents/Projects/fila-atendimento/web/app/components/calendar/CalendarEventForm.vue)
  Modal de **criar/editar** evento (titulo, cliente, responsavel, data, horario, tipo, status,
  prioridade, descricao). Emite `submit`/`cancel`/`remove`; a pagina chama
  `store.createEvent/updateEvent/deleteEvent` (API real).
  - **Toggle "Criar task no board" (SPEC-F8, contrato C10)**: so ao CRIAR (`v-if="!isEdit"`). Le
    `useCalendarStore().config.tasks.boardId`. Com board configurado -> checkbox sugerido pre-ligado
    para tipos `{gravacao, reuniao, evento}` (acompanha o tipo enquanto o usuario nao mexe:
    `createTaskTouched`); o `submit` inclui `createTask: true` no `CalendarEventInput`. Sem board ->
    aviso acionavel `.calendar-form__task-warn` com link `Configurar` -> `/calendario?config=integracoes`
    (abre o drawer de config na aba Integracoes via watcher de `route.query.config` no index) + `emit('cancel')`
    pra fechar o form. Estilos `.calendar-form__toggle*` / `.calendar-form__task-*` em `week-form.css`.
  - Fluxo do aviso de task no store: `createEvent` chama `calendarApi.postEvent` que agora devolve
    `{ taskId, taskWarning }`; se `taskWarning` != '' (evento salvou 201 mas a task falhou) o store dispara
    `ui.info(taskWarning, 'Task não criada')` sem derrubar o sucesso. `CalendarEvent` ganhou `taskId?`/
    `version?` (so leitura; `version` e' base do optimistic locking da SPEC-F9) e `CalendarEventInput`
    ganhou `createTask?` (omitindo `id`/`taskId`/`version`) em `utils/calendar.ts`.

## Diretrizes rapidas

- se a tela for um painel inteiro, procurar primeiro um `*Workspace.vue`
- se for tabela ou card especializado, procurar primeiro na pasta de dominio correspondente
- se for acao global de notificacao ou confirmacao, usar `uiStore` com os hosts de `ui`
- se for filtro simples, nao criar novo `<select>` solto; encapsular em [AppSelectField.vue](/c:/Users/Mike/Documents/Projects/fila-atendimento/web/app/components/ui/AppSelectField.vue) ou evoluir esse componente
