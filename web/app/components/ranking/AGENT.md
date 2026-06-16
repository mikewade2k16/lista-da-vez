# AGENTS

## Escopo

Estas instrucoes valem para `web/app/components/ranking`.

## Responsabilidade

Workspace `ranking` — comparativo gamificado de desempenho entre consultores e lojas, com podio para o top 3 e leaderboard de cards para os demais. Detalhes (incluindo o breakdown do Score 360 e alertas individuais) abrem em drawer.

## Regras atuais

- [RankingWorkspace.vue](/c:/Users/Mike/Documents/Projects/fila-atendimento/web/app/components/ranking/RankingWorkspace.vue) segue o padrao local do consultor: card de filtros com busca + loja + criterio, secoes de podio (`RankingPodium`) + leaderboard (`RankingLeaderboardCard`) e drawer (`RankingDetailsDrawer`) para linhas de consultor.
- Multi-loja nao depende mais de seletor global do header: o proprio workspace decide `loja especifica` vs `todas as lojas` pelo filtro local.
- [RankingTabsHeader.vue](/c:/Users/Mike/Documents/Projects/fila-atendimento/web/app/components/ranking/RankingTabsHeader.vue) fica legado no momento; nao e mais o controle principal da tela.
- [RankingPodium.vue](/c:/Users/Mike/Documents/Projects/fila-atendimento/web/app/components/ranking/RankingPodium.vue) mostra o top 3 com pedestal visual (2-1-3). Clique no slot abre o drawer com aquela linha selecionada.
- [RankingLeaderboardCard.vue](/c:/Users/Mike/Documents/Projects/fila-atendimento/web/app/components/ranking/RankingLeaderboardCard.vue) renderiza do 4º em diante com posicao, nome/loja, metrica grande, barra de meta opcional e badge de variacao ↑/↓.
- [RankingScoreBreakdown.vue](/c:/Users/Mike/Documents/Projects/fila-atendimento/web/app/components/ranking/RankingScoreBreakdown.vue) renderiza a barra stackeada com os 5 componentes do Score 360 (Conversao, Valor, Qualidade, P.A., Disciplina de fila). Pesos vem de `useGamificationConfig()` (defaults 35/25/20/15/5 ate `crm-c6` plugar config real).
- [RankingDetailsDrawer.vue](/c:/Users/Mike/Documents/Projects/fila-atendimento/web/app/components/ranking/RankingDetailsDrawer.vue) usa `USlideover` (Nuxt UI) com 3 modos (`center` default, `fullscreen`, `side`) controlados por `useRankingDetailsDrawer()`. 3 tabs: `Visao geral` (KPIs em grid + toggle "Ver como tabela" usando `RankingTable` legado), `Breakdown 360`, `Alertas` (filtrados pelo consultor da linha).
- [RankingTable.vue](/c:/Users/Mike/Documents/Projects/fila-atendimento/web/app/components/ranking/RankingTable.vue) permanece como **fallback legado**. So aparece dentro do drawer (botao "Ver como tabela") — nao e mais renderizada na tela principal.

## Logica do ranking

- **Score 360** e calculado client-side via `computeScore360()` em `~/composables/useGamificationConfig`. Pesos sao configuraveis (default 35/25/20/15/5).
- Quando `Loja = Todas as lojas`, a tela mostra primeiro o **ranking de lojas** e depois o ranking de consultores **agrupado por loja**.
- Quando `Loja = <loja especifica>`, a tela mostra apenas o ranking dos consultores daquela loja no mesmo layout de podio + leaderboard.
- `Score 360` da loja = **media ponderada por `attendances`** dos consultores. `soldValue` e soma direta.
- **Variacao ↑/↓** no leaderboard e derivada client-side comparando `score360` do `dailyRows` vs `monthlyRows` (ritmo do dia vs media do mes). Se algum lado estiver zerado, nao mostra badge. Wire vs snapshot do mes anterior fica para fase futura.
- **Alertas** vem de `report.alerts` e nao aparecem mais no topo da tela; o workspace mostra um hint contador e o detalhe individual aparece na aba `Alertas` do drawer.

## Fonte de dados

- Loja unica continua vindo de `useAnalyticsStore()`.
- Multi-loja usa `useConsultantsStore().integratedRanking`, reaproveitando o pipeline local que consolida roster + historico por loja sem depender do seletor global removido.
- Linhas trazem `consultantId`, `consultantName`, `storeId`, `storeName`, `soldValue`, `attendances`, `conversions`, `conversionRate`, `ticketAverage`, `paScore`, `qualityScore`, `avgDurationMs`, `queueJumpServices`.
- O drawer recebe `legacyRows` (todas as linhas ja ordenadas) para alimentar a tabela legada opcional.

## Roadmap

- Fase **CRM C5** entregue: podio + leaderboard + drawer + score breakdown.
- Fase **CRM C6** pendente: backend `GamificationConfig` (badges + score weights) que sera plugado em `useGamificationConfig()` sem refactor de componentes.

Plano historico: `~/.claude/plans/consultor-ranking-gamificado.md`.

## Mudancas recentes (auditoria gamificacao)

- `RankingTable.vue`: bug critico corrigido — pesos do Score 360 estavam hardcoded (35/25/20/15/5 fixos); agora usa `computeScore360()` com `scoreWeights.value` de `useGamificationConfig()`.
- `RankingWorkspace.vue`: reescrito com `<script setup lang="ts">`. Reducao de 853 linhas para 340. Logica de enriquecimento e agregacao de lojas extraida para `~/composables/useRankingData.ts`.
- Novo componente: `RankingFilters.vue` — form de filtros extraido de `RankingWorkspace.vue`. Props: `dateFrom/dateTo/searchTerm/storeFilter/metric/storeOptions/metricOptions/integratedScope/pending`. Emits v-model para cada campo + `applyPeriod/setCurrentMonth/setPreviousMonth`.
- Novo composable: `~/composables/useRankingData.ts` — exporta tipos `RankingRow`, `EnrichedRow`, `StoreAggRow` e funcoes `buildRowKey`, `getMetricValue`, `normalizeSearch`, `buildStoreAggregates`, `useRankingData(monthlyRows, dailyRows)`. Calcula Score 360 via `computeScore360()` com pesos da config.
