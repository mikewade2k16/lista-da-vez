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
- para modal de leitura detalhada, preferir [AppDetailDialog.vue](/c:/Users/Mike/Documents/Projects/fila-atendimento/web/app/components/ui/AppDetailDialog.vue)
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

## Diretrizes rapidas

- se a tela for um painel inteiro, procurar primeiro um `*Workspace.vue`
- se for tabela ou card especializado, procurar primeiro na pasta de dominio correspondente
- se for acao global de notificacao ou confirmacao, usar `uiStore` com os hosts de `ui`
- se for filtro simples, nao criar novo `<select>` solto; encapsular em [AppSelectField.vue](/c:/Users/Mike/Documents/Projects/fila-atendimento/web/app/components/ui/AppSelectField.vue) ou evoluir esse componente
