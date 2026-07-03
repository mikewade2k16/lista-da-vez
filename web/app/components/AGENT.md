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
`definePageMeta workspaceId: ''` para nao cair no gate de workspace) + `pages/calendario/config.vue`
(`/calendario/config`, mesmo criterio). Calendario de conteudo por cliente da agencia. Layout em
colunas: [coluna esquerda = controles + notas, UM card] [week rail S1..Sn] [calendario (scroll)]
[drawer do dia]. DUAS VISOES (Mes / Semana), toggle nos controles. Estado em [stores/calendar.ts];
helpers de data/constantes em [utils/calendar.ts]; tipos+helpers da CONFIG (contrato C2) em
[utils/calendar-config.ts](/c:/Users/Mike/Documents/Projects/fila-atendimento/web/app/utils/calendar-config.ts)
(re-exportados por utils/calendar.ts); nav em `layers/queue/nav.config.ts`.

> **CONFIG v2 (SPEC-F3)**: `CalendarConfig` (jsonb `calendar.config`) traz alem de responsaveis+feriados:
> `weekStartsOn` (sunday|monday), `clientColors` ({ [clientId]: `#rrggbb`|`none` }), `typeColors`
> ({ [tipo]: `#rrggbb` }), `whiteLabel` (logo/titulo/cor) e `ai` (provider/model/baseUrl/systemPrompt/
> temperature — chaves de API NUNCA aqui, vivem no n8n). `normalizeConfig` (calendar-api.ts) faz merge
> POR SECAO (linha antiga do banco ganha o shape completo). APLICACAO no calendario: `weekStartsOn` da
> config espelha no viewport (store observa `config.weekStartsOn`); a cor do cliente vem de
> `resolveClientColor(clientColors[id], semente)` no `clients` computed; `typeColors` desce como prop
> `typeColors` por MonthGrid/WeekView/DayCell -> EventChip (override da cor do cliente quando setado).

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
  Topo da coluna esquerda (uma linha): titulo do mes + **botao IA (`@ai`, sparkles)** +
  **engrenagem (`@config`)** + **select de cliente** (Todos/especifico, via [AppSelectField]) +
  toggle Mes/Semana + Hoje + botao "Novo". O `@config` faz `navigateTo('/calendario/config')` (nao
  abre mais modal — SPEC-F3); o `@ai` abre o [CalendarAiPlanModal.vue] (SPEC-F5).

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

- **Pagina de config** `pages/calendario/config.vue` (substitui o antigo `CalendarConfigModal.vue`,
  DELETADO): header com voltar p/ `/calendario` + [AdminPageHeader]; monta um `draft` de
  `CalendarConfig` re-hidratado de `store.config` (so preserva enquanto `touched`); botao unico
  "Salvar configuracoes" -> `store.saveConfig` (PUT `/v1/calendar/config`) + feedback ui.success/error.
  Secoes em `components/calendar/config/` (estilos `.calendar-config__*` + `.calendar-config-page__*`
  em `assets/styles/calendar/config.css`):
  - `ConfigResponsibles.vue` — checkboxes dos usuarios da conta (`store.members`; vazio = todos).
  - `ConfigHolidays.vue` — toggles BR nacional / Sergipe / Aracaju / luxo internacional.
  - `ConfigAppearance.vue` — inicio da semana (seg/dom) + cor por cliente (input color + "Sem cor" =
    `none`) + cor por tipo (checkbox "Usar" + input color) + white-label (titulo/logo/cor).
  - `ConfigAi.vue` — provider (select) + modelo + baseUrl (placeholder = default do provider) +
    systemPrompt (textarea) + temperature; AVISO fixo "as chaves de API ficam no n8n, nunca aqui".
  - `ConfigMediaLimits.vue` — tetos GLOBAIS de upload; GET sempre (via `useCalendarMedia`), edicao
    so `platform_admin` (`auth.role === 'platform_admin'`; o back tambem restringe o PUT). Salva por
    `useCalendarMedia().saveMediaLimits` (PUT `/v1/calendar/media-limits`), independente do config.
  - `ConfigClientProfiles.vue` (SPEC-F4) — PERFIL ESTRATEGICO por cliente (contrato C3), usado pelo
    assistente de IA do mes. Select de cliente (`store.clients`) com badge preenchido/vazio (via
    `filled` do index) + form dos campos estaveis (segmento, posicionamento, site, instagram,
    endereco, descricao, historia, objetivos, tom de voz) + textareas do bloco `extra` (publico-alvo,
    oferta, pilares, cadencia, restricoes, performance, assets). Salva POR CLIENTE (botao proprio,
    independente do "Salvar configuracoes" global) + feedback ui.success/error; dirty guard ao trocar
    de cliente com edicao pendente (`window.confirm`). I/O em
    [composables/useCalendarClientProfiles.ts](/c:/Users/Mike/Documents/Projects/fila-atendimento/web/app/composables/useCalendarClientProfiles.ts)
    (index + load/save; `account_id` nunca no body — o back resolve pelo Principal); tipos+defaults em
    [utils/calendar-profile.ts](/c:/Users/Mike/Documents/Projects/fila-atendimento/web/app/utils/calendar-profile.ts)
    (re-exportados por utils/calendar.ts). Endpoints (calendar-api.ts): `GET /v1/calendar/client-profile?clientId=`,
    `PUT /v1/calendar/client-profile` (upsert full-replace), `GET /v1/calendar/client-profiles` (index
    lean `{clientId,filled,updatedAt}`). Perfil inexistente = 200 com defaults (nunca 404). Estilos
    `.calendar-profile__*` + `.calendar-config__section--wide` em `assets/styles/calendar/config.css`.
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

## Diretrizes rapidas

- se a tela for um painel inteiro, procurar primeiro um `*Workspace.vue`
- se for tabela ou card especializado, procurar primeiro na pasta de dominio correspondente
- se for acao global de notificacao ou confirmacao, usar `uiStore` com os hosts de `ui`
- se for filtro simples, nao criar novo `<select>` solto; encapsular em [AppSelectField.vue](/c:/Users/Mike/Documents/Projects/fila-atendimento/web/app/components/ui/AppSelectField.vue) ou evoluir esse componente
