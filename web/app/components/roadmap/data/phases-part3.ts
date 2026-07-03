import type { RoadmapPhase } from "./types";

export const ROADMAP_PHASES_PART3: RoadmapPhase[] = [
  // ─── CRM 360 — Fila + ERP ─────────────────────────────────────────────────

  {
    id: "erp-sorteio",
    code: "ERP Sorteio",
    title: "Aba Compras do ERP — dados corretos + filtro de valor + ordenação",
    goal: "Base exata por compra para sorteio 'compras do dia X ao Y acima de R$ V': filtrar pela data REAL da compra (order_date), excluir cancelados em toda query, filtrar por valor mínimo e ordenar de verdade (valor numérico, data cronológica). A aba Compras hoje filtra por data de importação do lote, não ordena por valor e inclui pedidos cancelados — diverge do CRM e oscila local↔online. Risco legal: contagem de compras precisa ser exata. Plano: planos/glowing-jingling-oasis.md.",
    status: "in_progress",
    estimateWeeks: "2–3 dias",
    startedAt: "2026-06-30",
    group: "crm-360",
    tasks: [
      { id: "sort-fix", label: "repository_raw_records.go: traduzir id da coluna do front → coluna de ordenação real (total_amount_raw→total_amount_cents numérico; order_date_raw→order_date cronológico, NULLS LAST); expor order_date no CTE de dedup", done: false },
      { id: "date-toggle", label: "Filtro de período por data da compra (order_date) como padrão + toggle 'Data de importação' (source_batch_date), em GetRecordsStats e ListRawRecords (params dateField)", done: false },
      { id: "min-value", label: "Filtro 'valor mínimo' (minValueCents) por total bruto da compra, nos cards (stats) e na lista", done: false },
      { id: "exclude-canceled", label: "Anti-join erp_order_canceled_raw em TODA query de pedido ativo (Compras): cancelado nunca entra em card/lista/contagem/export", done: false },
      { id: "front-filters", label: "Front: input de valor mínimo + toggle de data na aba Compras (useErpWorkspace/erp store/ErpRecordsTab/ErpDataTable); export CSV reflete os filtros", done: false },
      { id: "go-tests", label: "Testes Go: mapeamento de ordenação + filtros (order_date, valor mínimo, exclusão de cancelados)", done: false },
      { id: "agent-md", label: "Atualizar back/internal/modules/crm/erp/AGENT.md com o novo comportamento de filtro/ordenação", done: false },
      { id: "verify", label: "Verificação: reconciliar Compras(todas as lojas) com CRM(mapeadas)+não-mapeadas no mesmo período; reportar order_date NULL", done: false }
    ],
    verifiable: "Ordenar por 'Valor total' ordena numérico e por 'Data' cronológico; período por data da compra; nenhum cancelado em card/lista/export; toggle reproduz o número antigo por lote; go test ./... passa.",
  },
  {
    id: "crm-c1",
    code: "CRM C1",
    title: "Indicadores por consultor — backend",
    goal: "Novo endpoint que funde dados de atendimento da fila (conversão, cancelamento) com dados de vendas do ERP (faturamento, PA, ticket médio, % meta).",
    status: "in_progress",
    estimateWeeks: "3–5 dias",
    startedAt: "2026-05-21",
    group: "crm-360",
    tasks: [
      { id: "model-queue-stats", label: "Adicionar QueueStats (atendimentos, conversões, taxa conversão, taxa cancelamento fila) ao CRMConsultantMetric e CRMStoreMetric", done: false },
      { id: "query-fusion", label: "Query SQL em repository_crm_aggregates.go que agrega operation_service_history por consultor/loja no período", done: false },
      { id: "service-fusion", label: "service.CRMOverview inclui dados de fila; taxa cancelamento ERP (ordercanceled/order) calculada separadamente", done: false },
      { id: "agent-md", label: "Atualizar back/internal/modules/erp/AGENT.md com novos campos e query de fusão", done: false }
    ],
    verifiable: "GET /v1/erp/crm retorna atendimentos, taxaConversao e taxaCancelamentoFila por consultor; go test ./... passa."
  },
  {
    id: "crm-c2",
    code: "CRM C2",
    title: "Produto não encontrado — modelo e modal",
    goal: "Separar campo 'produto que o cliente queria mas a loja não tinha' de 'produto visto' no modelo da fila. Adicionar distinção de motivo de perda: preço vs falta de estoque.",
    status: "in_progress",
    estimateWeeks: "2–3 dias",
    startedAt: "2026-05-21",
    group: "crm-360",
    tasks: [
      { id: "migration", label: "Migration SQL: adicionar products_not_found_json em operation_service_history", done: false },
      { id: "model-go", label: "Adicionar ProductsNotFound []ProductEntry ao FinishCommandInput e ServiceHistoryEntry em operations/model.go", done: false },
      { id: "store-postgres", label: "Persistir e ler products_not_found_json em operations/store_postgres.go", done: false },
      { id: "frontend-modal", label: "Adicionar seção 'Produto procurado / não encontrado' no OperationFinishModal.vue separada de produtos vistos", done: false },
      { id: "agent-md", label: "Atualizar back/internal/modules/operations/CONCURRENT_SERVICES.md e AGENT.md com novo campo", done: false }
    ],
    verifiable: "Finalizar atendimento com produto não encontrado persiste no banco; histórico retorna o campo; modal mostra seção distinta."
  },
  {
    id: "crm-c3",
    code: "CRM C3",
    title: "Painel CRM — gráficos e 360",
    goal: "ErpCrmWorkspace com gráficos de faturamento, % meta, conversão e cancelamento por consultor e por loja. Cards de indicadores com comparativo.",
    status: "pending",
    estimateWeeks: "4–6 dias",
    group: "crm-360",
    tasks: [
      { id: "ts-types", label: "Atualizar tipos TypeScript no CRM store com novos campos de QueueStats", done: false },
      { id: "chart-consultant", label: "Gráficos por consultor: faturamento, % meta, PA, ticket médio, taxa conversão, taxa cancelamento", done: false },
      { id: "chart-store", label: "Comparativo por loja: mesmos indicadores + ranking", done: false },
      { id: "products-not-found", label: "Seção de produto não encontrado no painel CRM com agrupamento por SKU/motivo", done: false },
      { id: "360-checklist", label: "Aba 360 com indicadores + atendimento + metas + não compra (estrutura inicial)", done: false },
      { id: "agent-md", label: "Atualizar web/app/components/erp/AGENT.md com novos componentes de gráficos", done: false }
    ],
    verifiable: "Painel CRM exibe gráficos com dados reais do novo endpoint; filtro de período funciona; sem erros de console."
  },
  {
    id: "crm-c4",
    code: "CRM C4",
    title: "Consultor gamificado — player card + drawer",
    goal: "Substituir o painel de cards planos da workspace Consultor por player card (gauge dominante + 4 KPIs core + badges) e mover métricas secundárias + simulador para drawer lateral. Visão all-stores vira grid de mini-cards.",
    status: "done",
    estimateWeeks: "4–6 dias",
    startedAt: "2026-05-22",
    group: "crm-360",
    tasks: [
      { id: "agent-md", label: "Atualizar web/app/components/consultant/AGENT.md com contrato dos novos componentes (player card, drawer, grid)", done: true, note: "2026-06-11: auditoria confirmou os componentes e atualizou consultant/AGENT.md." },
      { id: "gamification-config-composable", label: "useGamificationConfig() composable com defaults hardcoded para badges + Score 360 weights (preparado para C6 plugar fonte real)", done: true },
      { id: "player-card-component", label: "ConsultantPlayerCard.vue (modos full e mini) — gauge SVG, 4 KPIs core (Vendido, Ticket, PA, Conversão), slot de badges", done: true },
      { id: "badges-component", label: "ConsultantBadges.vue puro recebe stats + badgesConfig — defaults: Meta batida, Top N, Conversão > média loja, Ticket > meta, PA > meta", done: true },
      { id: "drawer-shell", label: "ConsultantDetailsDrawer.vue com USlideover (modos center/fullscreen/side igual TasksTaskModal) + composable useConsultantDetailsDrawer()", done: true },
      { id: "drawer-tabs", label: "Drawer com 3 tabs: Visão geral (todos KPIs incluindo cancelamento/fora-da-vez/tempo médio), Histórico (sparkline 7d), Simulador (move ConsultantSimulator atual)", done: true },
      { id: "single-store-wire", label: "ConsultantWorkspace.vue (single-store) usa ConsultantPlayerCard full + drawer", done: true },
      { id: "multi-store-grid", label: "ConsultantPlayerGrid.vue substitui tabela 'Comparativo completo' por grid de mini-cards filtráveis", done: true },
      { id: "cancellation-wire", label: "Garantir cancellationRate no DTO consumido por consultants/analytics stores (já calculado em repository_crm_queue.go)", done: true, note: "2026-06-11: fechado. O valor já vinha em GET /v1/erp/crm (queueStats.byConsultant[].queueCancellationRate, tipado em stores/erp.ts) — gap era só o merge no front. Mergeado por (storeId, personId) na store consultants → ConsultantRow → exibido no ConsultantPlayerCard (full) e no drawer. Sem mudança de back/migration. Pendente menor: modo single-store não-integrado (ConsultantWorkspace) não busca /v1/erp/crm, então lá degrada limpo (não renderiza)." },
      { id: "delete-old-metrics", label: "Remover ConsultantMetrics.vue após migração completa", done: true, note: "2026-06-11: ConsultantMetrics.vue não existe mais no repo (já removido na migração); referências restantes são ao composable useCrmConsultantMetrics (nome parecido)." }
    ],
    verifiable: "/consultor em loja única mostra player card + drawer abrindo via 'Ver detalhes'. Visão all-stores mostra grid de mini-cards; click no card abre drawer. Sem erros de console; npm test passa."
  },
  {
    id: "crm-c5",
    code: "CRM C5",
    title: "Ranking gamificado — pódio + leaderboard + drawer",
    goal: "Substituir as duas tabelas de 11 colunas por pódio dos 3 primeiros + leaderboard de cards horizontais para o resto. Tabs Lojas/Consultores/Por-loja. Score 360 vira sort default. Detalhes (breakdown 360 + alertas) em drawer lateral.",
    status: "done",
    estimateWeeks: "5–7 dias",
    startedAt: "2026-05-22",
    group: "crm-360",
    tasks: [
      { id: "agent-md", label: "Criar web/app/components/ranking/AGENT.md com contrato do novo workspace", done: true, note: "2026-06-11: criado na auditoria." },
      { id: "tabs-header", label: "RankingTabsHeader.vue (3 tabs Lojas/Consultores/Por-loja + chips de sort, Score 360 default)", done: true },
      { id: "podium-component", label: "RankingPodium.vue — top-3 visual (2º-1º-3º) com avatar, nome/loja, número grande da métrica ativa", done: true },
      { id: "leaderboard-card", label: "RankingLeaderboardCard.vue — card horizontal (4º+) com posição, métrica grande, barra meta, badge variação ↑/↓", done: true },
      { id: "variation-derivation", label: "Derivar variação vs período anterior client-side (comparar monthlyRows com snapshot mês anterior já disponível)", done: true },
      { id: "stores-tab", label: "Agregação por loja para tab Lojas: totalSoldValue, Score 360 ponderado por attendances (decisão fechada), consultantsAtGoal", done: true },
      { id: "per-store-tab", label: "Tab 'Por loja' com combobox de loja + pódio + leaderboard filtrados", done: true },
      { id: "drawer-ranking", label: "RankingDetailsDrawer.vue com USlideover (center/fullscreen/side) + tabs Visão geral / Breakdown 360 / Alertas. Mover alertas do topo do RankingWorkspace para drawer (manter contador)", done: true },
      { id: "score-breakdown", label: "Componente de barra stackeada para breakdown do Score 360 — pesos vêm de useGamificationConfig() (defaults 35/25/20/15/5 até C6 plugar fonte real)", done: true },
      { id: "legacy-table", label: "Manter RankingTable.vue acessível como 'Ver como tabela' dentro do drawer para usuários que preferem formato denso", done: true, note: "2026-06-11: auditoria corrigiu bug de pesos hardcoded no RankingTable (passou a usar computeScore360 da config)." }
    ],
    verifiable: "/ranking mostra pódio + leaderboard cards; tabs trocam agrupamento; click no card abre drawer com breakdown 360. ESC/overlay/X fecham. Alertas continuam acessíveis via drawer. npm test passa; sem regressões."
  },
  {
    id: "crm-c6",
    code: "CRM C6",
    title: "Backend de gamificação — config de badges + Score 360 weights",
    goal: "Permitir que cada tenant configure regras de badges (Meta batida, Top N, Conversão > média loja, etc.) e os pesos do Score 360 (Conversão/Valor/Qualidade/PA/Queue-jump). Plugar no composable useGamificationConfig() do front (criado em C4) substituindo defaults hardcoded.",
    status: "done",
    estimateWeeks: "3–5 dias",
    group: "crm-360",
    tasks: [
      { id: "model-go", label: "Adicionar GamificationConfig (BadgeRules []BadgeRule + ScoreWeights ScoreWeights) ao settings.Bundle e Record", done: true, note: "2026-06-11: BadgeRules em AppSettings. ScoreWeights já persistiam via settings.scoreWeight*." },
      { id: "migration", label: "Migration SQL: tabela settings_gamification (tenant_id, badge_rules_json, score_weights_json)", done: true, note: "2026-06-11: migration 0146 (public.tenant_gamification_settings, badge_rules jsonb, FK core.accounts). score_weights já persistem nos settings existentes — sem coluna nova." },
      { id: "store-postgres", label: "Persistir e ler GamificationConfig em settings store_postgres.go", done: true, note: "store_postgres_gamification.go (pgx CollectOneRow/RowToStructByName)." },
      { id: "defaults", label: "settings/defaults.go com defaults de GamificationConfig (mesmos hardcoded usados em C4/C5)", done: true },
      { id: "http-endpoint", label: "PATCH /v1/settings/gamification com perm settings.write; GET expõe junto com o bundle existente", done: true, note: "PATCH /v1/settings/gamification (RequireAuth); badges injetadas no bundle do GET." },
      { id: "frontend-settings-ui", label: "Seção 'Gamificação' na página de configurações para editar badges (CRUD lista) e weights (5 sliders que somam 100%)", done: true, note: "2026-06-11: badges CRUD + os 5 sliders de peso do Score 360 (SettingsScoreWeightsCard.vue, total com feedback de cor) na aba Gamificacao. Pesos reusam o PATCH /v1/settings/operation existente. Editor por inputs na aba Operacao mantido intacto." },
      { id: "wire-composable", label: "useGamificationConfig() passa a ler do settings store (com fallback para defaults se config não existir)", done: true, note: "resolveBadgeRules lê de runtime.state.settings.badgeRules com fallback nos defaults; API pública estável." },
      { id: "agent-md", label: "Atualizar back/internal/modules/settings/AGENT.md com novo bundle field e endpoint", done: true, note: "queue/settings/AGENT.md (módulo migrado para queue/settings)." }
    ],
    verifiable: "PATCH /v1/settings/gamification persiste; GET /v1/settings retorna gamificationConfig no bundle; UI permite editar badges e weights; após salvar, player cards e ranking refletem mudanças sem recarregar. go test ./... passa."
  },

  {
    id: "crm-c7",
    code: "CRM C7",
    title: "CRM 360 — atribuicao multi-loja e uso da lista",
    goal: "Corrigir atribuicao de vendas ERP para consultores multi-loja e substituir o uso da lista por uma metrica de cobertura que nao passa de 100%.",
    status: "done",
    estimateWeeks: "1 dia",
    startedAt: "2026-06-08",
    finishedAt: "2026-06-08",
    group: "crm-360",
    tasks: [
      { id: "store-attribution", label: "Priorizar loja explicita do ERP e historico dominante antes do cadastro atual do vendedor", done: true },
      { id: "list-usage-contract", label: "Definir cobertura da lista como consultores com atendimentos >= pedidos ERP no periodo", done: true },
      { id: "summary-cards", label: "Remover cards confusos e exibir atendimentos, conversao, uso da lista e cancelamento ERP", done: true },
      { id: "consultant-grid", label: "Trocar coluna percentual de uso por status de cobertura da lista por consultor", done: true },
      { id: "docs-tests", label: "Atualizar AGENT/docs e cobrir calculos com testes", done: true }
    ],
    verifiable: "Maio/2026 separa vendas por loja de consultores multi-loja; card Uso da lista mostra cobertura 0-100%; tabela destaca Coberto/Parcial/Sem uso; go test do pacote CRM e testes web de util passam."
  },

  {
    id: "crm-c8",
    code: "CRM C8",
    title: "CRM 360 — politica comercial de lista e recebimento",
    goal: "Evitar falsos destaques quando uso da lista esta ruim, configurar faixas de cobertura e mostrar recebimento por atingimento de meta na grade de consultores.",
    status: "done",
    estimateWeeks: "1 dia",
    startedAt: "2026-06-08",
    finishedAt: "2026-06-08",
    group: "crm-360",
    tasks: [
      { id: "list-rankings", label: "Trocar melhor loja/consultor por diagnostico quando todos estao abaixo da faixa Normal", done: true },
      { id: "config-tab", label: "Criar aba Metas CRM para faixas de uso da lista e recebimento por meta", done: true },
      { id: "settings-contract", label: "Persistir politica comercial nas settings de operacao com edicao para platform_admin e director", done: true },
      { id: "consultant-payout", label: "Adicionar recebimento na grade de consultores calculado pela meta da loja", done: true },
      { id: "docs-tests", label: "Atualizar AGENT/docs e cobrir calculos com testes", done: true }
    ],
    verifiable: "Cards nao exibem melhor loja/consultor como premio quando tudo esta ruim; Configuracoes > Metas CRM edita faixas e recebimentos; coluna Recebimento aparece na grade; testes crm-list-usage/crm-performance-policy e settings passam."
  },

  {
    id: "crm-c9",
    code: "CRM C9",
    title: "CRM 360 — recebimento por meta da loja nos cards + CRUD de faixas",
    goal: "Levar a politica de recebimento por atingimento de meta para os cards de consultor (cor do gauge pela faixa individual, barra de % da loja que muda de cor, valor a receber), mostrar gerente/caixa/auxiliar ao lado dos consultores so com o que ganham pela loja, e deixar a pagina de faixas (Metas CRM) fazer CRUD sem erro. Gate de recebimento = % da meta da loja; base do % = total vendido da loja.",
    status: "pending",
    estimateWeeks: "1-2 dias",
    group: "crm-360",
    tasks: [
      { id: "payout-domain", label: "Helper unico mapRoleToPayoutGroup + calculateStoreGoalPayout (gate = % meta da loja; base % = total vendido da loja) em crm-performance-policy.ts", done: false },
      { id: "store-progress", label: "useConsultantIntegratedRows expoe storeProgressByStoreId e storeTotalSoldByStoreId; composable de leitura da crmGoalPayoutPolicy do runtime", done: false },
      { id: "card-colors", label: "ConsultantPlayerCard: gauge muda de cor pela faixa individual + barra de % da loja colorida + linha de recebimento por meta; manter 'Sem meta cadastrada'", done: false },
      { id: "staff-cards", label: "Cards enxutos (modo payout) para gerente/caixa/auxiliar ao lado dos consultores, so com nome/papel/recebimento da loja", done: false },
      { id: "staff-endpoint", label: "Backend: endpoint lean de staff sem fila por loja (core.account_users + role_assignments), escopo validado contra o Principal, fora do escopo 404", done: false },
      { id: "crm-table-consistency", label: "CrmConsultantsSection usa o mesmo helper (base total da loja) na coluna Recebimento", done: false },
      { id: "payout-crud", label: "SettingsCrmGoalsSection + useSettingsWorkspace: CRUD sem derrubar linha ao editar, sem re-sort/troca de key no meio, save no blur, remover ate zero; layout compacto colapsavel com tokens", done: false },
      { id: "docs-tests", label: "Atualizar AGENT.md dos modulos tocados, panorama HTML e cobrir o helper de payout com teste", done: false }
    ],
    verifiable: "Em Perola Treze: gauge dos consultores muda de cor por faixa; barra de % da loja aparece e muda de cor; cada card mostra o recebimento pela meta da loja; gerente/caixa/auxiliar aparecem como cards enxutos com o valor da loja; pagina Metas CRM adiciona/edita/remove faixas sem perder foco nem derrubar linha e persiste apos refresh."
  },

  {
    id: "crm-c10",
    code: "CRM C10",
    title: "Aviso acionavel inline + quick-edit de metas via API (de qualquer tela, plugavel)",
    goal: "Quando um dado que o calculo usa (meta de ticket/PA da loja, meta por consultor, store_type) esta faltando, a tela mostra um aviso honesto onde o dado importa e, para quem tem permissao, deixa cadastrar NA HORA num popover inline que grava pela API canonica (reusa operationgoals) — sem obrigar a achar a tela de config. Mecanismo PLUGAVEL e simples: 1 descriptor + soltar <InlineFieldGuard>. Caso real: Perola Jardins sem ticket/PA (penalidade desligada) e sem meta individual (meta da loja dividida por N). Doc: docs/INLINE_QUICK_EDIT_PLAN.md.",
    status: "done",
    estimateWeeks: "2-3 dias",
    startedAt: "2026-06-17",
    finishedAt: "2026-06-17",
    group: "crm-360",
    tasks: [
      { id: "gap-flags", label: "Back: /v1/erp/crm expoe flags de gap (goalSource own|store-split|none, missingMonthlyGoal/Ticket/Pa por consultor; missingStoreGoal/Ticket/Pa + splitCount por loja) calculados no applyCRMPayouts; DTO em crm/erp/model.go; rebuild api", done: true },
      { id: "inline-field-guard", label: "Front: motor plugavel — InlineFieldGuard.vue + QuickEditPopover.vue + defineQuickEditField/registry em web/app/domain/quick-edit/ (aviso + clicavel se canEdit + salva via descriptor.save + re-hidrata via afterSave + fecha clique-fora/Esc)", done: true },
      { id: "goal-descriptors", label: "Descriptors storeTicketGoal/storePaGoal/consultantMonthlyGoal salvando via /v1/operations/goals (reusa useOperationGoalsStore; SEM endpoint novo, SEM migration)", done: true },
      { id: "consultor-plug", label: "Plugar <InlineFieldGuard> nos cards de /consultor (ConsultantPlayerCard/Grid): aviso informativo p/ todos, edicao gated por canManageGoalTargets espelhando o back; transparencia 'meta da loja R$ X / N'", done: true },
      { id: "docs-tests", label: "Sincronizar docs/INLINE_QUICK_EDIT_PLAN.md + AGENT.md (crm/erp, consultant front) + testes vitest dos descriptors do guard (16 verdes)", done: true }
    ],
    verifiable: "Na Perola Jardins, /consultor mostra 'sem TM' no cabecalho da loja (loja sem ticket) e 'sem Meta' por consultor; usuario com permissao clica, cadastra no popover (grava via /v1/operations/goals), recalcula vindo do back e persiste apos refresh; quem nao tem permissao ve so o aviso. 16 testes vitest dos descriptors verdes; vue-tsc da pasta consultant zerado."
  },

  {
    id: "crm-c11",
    code: "CRM C11",
    title: "Estender quick-edit inline a operacao/ranking/multiloja + novos descriptors",
    goal: "Reusar o MESMO motor InlineFieldGuard (entregue na C10) em /operacao, /ranking e multiloja, e escrever os descriptors de store_type (PATCH /v1/stores/{id}) e politica de comissao (PATCH /v1/settings/crm-policy). Zero codigo novo de UI por tela: 1 descriptor + soltar o componente. Doc: docs/INLINE_QUICK_EDIT_PLAN.md (Fase 2).",
    status: "pending",
    estimateWeeks: "1-2 dias",
    group: "crm-360",
    tasks: [
      { id: "operacao-plug", label: "Plugar <InlineFieldGuard> na pagina de operacao (avisos + edicao de meta no contexto consultor/loja)", done: false },
      { id: "ranking-multiloja-plug", label: "Plugar o mesmo guard em /ranking e multiloja onde meta/atingimento aparecem", done: false },
      { id: "store-type-descriptor", label: "Descriptor de store_type (PATCH /v1/stores/{id}) e de politica de comissao (PATCH /v1/settings/crm-policy)", done: false }
    ],
    verifiable: "O mesmo <InlineFieldGuard> aparece em /operacao, /ranking e multiloja sem codigo novo por tela; editar store_type/politica inline grava pela API canonica e re-hidrata."
  },

  {
    id: "qa-vue-tsc-baseline",
    code: "QA · vue-tsc",
    title: "Zerar a baseline de erros do type-check do front (vue-tsc)",
    goal: "O QUE FALTA: ~223 erros de tipo no `npx vue-tsc --noEmit` do web, pre-existentes e espalhados — site (47), crm (40), utils/runtime-remote+api-client (38), ranking (20), stores+dashboard (21), composables (13), layers/tasks (14), alerts (7), tenants (6) e o resto em admin/manager/roadmap/omni/feedback/meta-ads/bi/app.config (~17). Sao quase todos tipagem LOOSE (`unknown`/`object`/`any` implicito em respostas de API, getters de store e props), nao bugs de runtime. POR QUE NAO E' URGENTE: o vue-tsc NAO esta no pre-commit (so eslint/golangci/sql-lint sao enforcados); o app compila e roda normal (Vite/Nuxt transpila sem checagem de tipo), entao nada disso quebra em producao hoje; e' o estado ambiente de um codebase grande. POR QUE DEVEMOS RESOLVER: ENGINEERING_PRINCIPLES (TS strict: vue-tsc deve passar, pega bug em build time e nao em prod) e o objetivo de type-safety; com 223 erros de ruido NAO da pra usar o vue-tsc como gate — um erro NOVO de verdade se esconde no meio; refactor sem type-check e' arriscado; ja mordeu nesta branch (PlayerCardStats/liveStatusCode/ConsultantRow eram exatamente loose typing escondendo incompatibilidade real). Zerar permite LIGAR o gate (CI/pre-commit) e impedir regressao. COMO: tipar na FONTE (sem `any`), area por area, com subagentes Opus em paralelo (dominios disjuntos).",
    status: "pending",
    estimateWeeks: "3-5 dias",
    group: "infra-deploy",
    tasks: [
      { id: "tsc-site", label: "Zerar vue-tsc em app/components/site (47 erros) — tipar respostas/props de SiteProductsWorkspace e cia", done: false },
      { id: "tsc-crm", label: "Zerar vue-tsc em app/components/crm (40) — CrmConsultantsSection e cia (tipar metricas/payout vindos do /v1/erp/crm)", done: false },
      { id: "tsc-utils", label: "Zerar vue-tsc em app/utils (38) — runtime-remote.ts + api-client.ts: tipar payloads de fetch e o estado remoto em vez de unknown/object", done: false },
      { id: "tsc-ranking", label: "Zerar vue-tsc em app/components/ranking (20)", done: false },
      { id: "tsc-stores", label: "Zerar vue-tsc em app/stores + app/stores/dashboard (21) — multistore.ts, state.ts, meta-ads.ts, workspace.ts", done: false },
      { id: "tsc-composables-tasks", label: "Zerar vue-tsc em app/composables (13) + layers/tasks (14 — components/composables: AppDatePicker, OmniDataTable, useTasks*)", done: false },
      { id: "tsc-resto", label: "Zerar vue-tsc no resto: alerts (7), tenants (6), admin/manager/roadmap/omni/feedback/meta-ads/bi + app.config.ts (~17)", done: false },
      { id: "tsc-gate", label: "Apos zerar: ligar o gate de vue-tsc (CI e/ou pre-commit Husky) pra impedir regressao; documentar em AGENT_RULES (Qualidade)", done: false }
    ],
    verifiable: "`npx vue-tsc --noEmit` no web retorna 0 erros; gate de vue-tsc ativo (CI/pre-commit) faz PR com erro de tipo falhar antes do merge; nenhuma feature existente quebrou (regressao visual/funcional checada por area)."
  },

  {
    id: "roadmap-b1",
    code: "Roadmap B1",
    title: "Backend de Modulos & Regras editaveis",
    goal: "Persistir RoadmapModule e RoadmapRule via API Go (schema novo roadmap.*). UI passa de read-only para edicao inline com workspace de prioridade, status e descricao. Regenera AGENT_RULES.md a cada PUT para que agentes leiam sempre a versao canonica.",
    status: "done",
    estimateWeeks: "3-5 dias",
    startedAt: "2026-05-23",
    finishedAt: "2026-05-23",
    group: "multi-tenant",
    tasks: [
      { id: "migration", label: "Migration 0115_roadmap_schema.sql: schema roadmap + tabelas modules e rules (account-scoped + global)", done: true, note: "Constraints check em status/priority/category; index parcial para registros globais." },
      { id: "module-go", label: "back/internal/modules/roadmap/ (model/store_postgres/service/http/AGENT.md) seguindo padrao", done: true, note: "Tipo de dominio nomeado ModuleRecord para nao colidir com modules.Module do registry." },
      { id: "endpoints", label: "GET /v1/roadmap/modules, PUT /v1/roadmap/modules/:id, GET /v1/roadmap/rules, PUT /v1/roadmap/rules/:id, POST /v1/roadmap/rules", done: true, note: "8 endpoints CRUD + GET /v1/roadmap/rules.md." },
      { id: "seed", label: "Seed inicial a partir de ROADMAP_MODULES e ROADMAP_RULES de web/app/components/roadmap/roadmap-data.ts", done: true, note: "12 modulos + 21 regras embutidas como seed global (account_id IS NULL) na propria migration; ON CONFLICT DO NOTHING." },
      { id: "front-store", label: "Pinia store useRoadmapStore() com fetch/update; substitui ROADMAP_MODULES/ROADMAP_RULES estaticos", done: true, note: "Store em app/stores/roadmap.ts; fallback para seeds estaticos quando backend retorna 404." },
      { id: "front-edit", label: "Edicao inline em RoadmapModulesBoard.vue e RoadmapRulesBoard.vue (prioridade, status, descricao)", done: true, note: "Cards ganham botao Editar; abrem form com select status/priority + textarea descricao." },
      { id: "export-md", label: "Endpoint GET /v1/roadmap/rules.md serve AGENT_RULES.md regenerado a partir do banco", done: true, note: "service.BuildMarkdown gera mesmo formato do AGENT_RULES.md raiz." },
      { id: "agent-md", label: "Adicionar back/internal/modules/roadmap/AGENT.md", done: true }
    ],
    verifiable: "Login + PUT em uma regra reflete em GET /v1/roadmap/rules.md instantaneamente; UI permite editar prioridade do modulo Tracking de P1 para P0 e o valor persiste apos refresh."
  },

  {
    id: "tasks-t9",
    code: "Tasks T9",
    title: "Testes E2E + observabilidade",
    goal: "Cobertura > 70% no service Go (scope, DTO, tracking, version conflict); testes Vitest no front (store, realtime, useCan); smoke E2E 12 passos.",
    status: "done",
    estimateWeeks: "2–3 dias",
    startedAt: "2026-05-15",
    finishedAt: "2026-05-15",
    group: "tasks-backend",
    tasks: [
      { id: "dto-test", label: "tasks/dto_test.go: snapshot JSON agency vs client_viewer (campos ausentes, não escondidos)", done: true, note: "4 testes: agency mantem clientAccountId, client_viewer omite, uiMetadata sempre nao-nil, ISO dates." },
      { id: "cursor-test", label: "tasks/cursor_test.go: round-trip + base64url-safe + decode invalido nao panica", done: true, note: "4 testes cobrindo encode/decode opaco do listTasksCursor (paginacao T5)." },
      { id: "presence-test", label: "realtime/presence_test.go: lock exclusivo (T7.2), TTL, snapshot, leave decrementa", done: true, note: "6 testes: user_joined unico, LockField exclusivo por fieldKey, owner reclaim, UnlockField publica, Leave decrementa, TTL expira." },
      { id: "rate-limit-test", label: "httpapi/rate_limit_test.go: bucket, reset, identity resolver, X-Forwarded-For", done: true, note: "6 testes cobrindo o middleware da T8." },
      { id: "service-test", label: "tasks/service_test.go: CRUD com 3 perspectives (agency, client_viewer, outro tenant)", done: true, note: "10 testes com repository_mock_test.go (Repository mock leve com hooks): CreateTask happy/no-perm/validation, GetTask perspective, ListTasks default-limit/clamp/no-perm/perspective/nextCursor." },
      { id: "scope-test", label: "tasks/scope_test.go: fuzz 100 IDs de outros accounts → 100% 404", done: true, note: "8 testes: accountID vazio, account inexistente, cross-account = 404 (nunca 403), platform_admin bypass, client_viewer perspective, manage override, fuzz 100 IDs cross-account 100% 404, scopedQuery panica sem accountID." },
      { id: "tracking-test", label: "tasks/tracking_test.go: version conflict, 1 entry ativa por (user, task)", done: true, note: "8 testes: no-perm/task-not-found/happy-path (publica WS + audita), PauseTracking propaga ErrVersionConflict, ResumeTracking passa expectedVersion, StopTracking 404 nao publica nem audita." },
      { id: "front-tests", label: "Vitest: clampText/normalizeText + setup para composables completos (futuro)", done: true, note: "Vitest 2.1 configurado em web/. 9 testes em utils/text.test.ts cobrindo o caso T7.2 (espaco no final preservado). Composables Vue completos ficam para quando @nuxt/test-utils for adicionado." },
      { id: "smoke-e2e", label: "Smoke E2E 12 passos: migrate fresh → seed → login agência → criar task → WS → presence → tracking → share → curl 404 → inspect payload", done: true, note: "Roteiro documentado em docs/TASKS_ORCHESTRATOR_PHASE12.md (secao 'Smoke E2E 12 passos') para o usuario rodar manualmente em staging." }
    ],
    verifiable: "go test ./... passa (50 testes T9 no total); npm test passa (9 Vitest); smoke E2E 12 passos roteiro pronto para staging."
  },
];
