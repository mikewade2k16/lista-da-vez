# AGENTS

## Escopo

Estas instrucoes valem para `web/app/components/consultant`.

## Responsabilidade

Esta pasta concentra a experiencia da workspace `consultor`.

## Regras atuais

- [ConsultantWorkspace.vue](/c:/Users/Mike/Documents/Projects/fila-atendimento/web/app/components/consultant/ConsultantWorkspace.vue) decide entre 2 modos: com apenas uma loja acessivel, renderiza a leitura da loja ativa (`ConsultantPlayerCard` full + `ConsultantHistoryPanel` + `ConsultantSimulator` inline); com multiplas lojas acessiveis, delega para `ConsultantIntegratedWorkspace.vue`, que controla o filtro de loja localmente sem depender de seletor global no header.
- [ConsultantPlayerCard.vue](/c:/Users/Mike/Documents/Projects/fila-atendimento/web/app/components/consultant/ConsultantPlayerCard.vue) tem dois conjuntos de KPIs:
  - **modo full**: Vendido, Ticket, P.A., Conversao (foco em conquista da meta).
  - **modo mini**: Tempo, Conversao, Ticket, P.A. (foco em comparativo entre consultores, alinhado ao que existia antes do refactor).
- [ConsultantPlayerCard.vue](/c:/Users/Mike/Documents/Projects/fila-atendimento/web/app/components/consultant/ConsultantPlayerCard.vue) no modo full concentra a leitura primaria da loja ativa em um unico card: gauge/meta + faixa superior com `Ticket`, `P.A.`, `Conversao` e `Tempo medio` + faixa secundaria unica com os demais KPIs (`Comissao`, `Atendimentos`, `Conversoes/nao-convertidas`, `Nao-clientes convertidos`, `Fora da vez`, `Cancelamento` se disponivel) em uma linha horizontal.
- [ConsultantHistoryPanel.vue](/c:/Users/Mike/Documents/Projects/fila-atendimento/web/app/components/consultant/ConsultantHistoryPanel.vue) renderiza o historico inline da tela individual com filtros de periodo (`Hoje`, `7 dias`, `30 dias`, `Mes`), sparkline e resumo do intervalo selecionado.
- [ConsultantDetailedMetrics.vue](/c:/Users/Mike/Documents/Projects/fila-atendimento/web/app/components/consultant/ConsultantDetailedMetrics.vue) ficou legado e nao e mais montado pela workspace principal.
- [ConsultantIntegratedWorkspace.vue](/c:/Users/Mike/Documents/Projects/fila-atendimento/web/app/components/consultant/ConsultantIntegratedWorkspace.vue) e a ORQUESTRADORA: concentra todo estado, computeds (filtros, `groupedRows`, `selectedStoreConsultantCard`, `selectedStoreStaff`), watches, fiacao do quick-edit (`useGoalQuickEditContext` → `goalContext`/`storeContext`) e o `ConsultantDetailsDrawer`. Alterna 2 layouts locais: com filtro de `Loja` em uma loja especifica delega para `ConsultantSingleStoreView`; em `Todas as lojas` renderiza um `ConsultantStoreGroup` por loja. A toolbar de periodo/filtros foi extraida para `ConsultantIntegratedFilters`. Os 3 sub-componentes sao PRESENTACIONAIS (props + emits); nenhuma logica/computed vive neles. (Refactor de fatiamento para ficar < 500 linhas do `max-lines`; comportamento inalterado.)
  - [ConsultantIntegratedFilters.vue](/c:/Users/Mike/Documents/Projects/fila-atendimento/web/app/components/consultant/ConsultantIntegratedFilters.vue) — toolbar de busca/loja/status/meta + seletor de periodo (`AppDatePicker`) e botoes `Mes anterior`/`Mes atual`/`Atualizar`. Recebe valores + opcoes + `pending`; emite `update:*` por filtro e `apply`/`reset-current-month`/`set-previous-month`.
  - [ConsultantSingleStoreView.vue](/c:/Users/Mike/Documents/Projects/fila-atendimento/web/app/components/consultant/ConsultantSingleStoreView.vue) — visao single-store (1 loja): `ConsultantSelector` + banner de avisos da loja (2x `InlineFieldGuard` ticket/PA com o `goalContext` do card) + `ConsultantPlayerCard` full + insights (`ConsultantHistoryPanel` + `ConsultantSimulator`) + equipe sem fila (`ConsultantStaffPayoutCard`) + `ConsultantRecentAttendancesTable`. Recebe `rows`/`selectedConsultant`/`card`/`staff`/`history`/`simulationAdditionalSales` ja prontos; emite `select` e `update:simulation-additional-sales`.
  - [ConsultantStoreGroup.vue](/c:/Users/Mike/Documents/Projects/fila-atendimento/web/app/components/consultant/ConsultantStoreGroup.vue) — UMA loja da visao agrupada: cabecalho (nome + contagem + 2x `InlineFieldGuard` ticket/PA com o `group.storeContext`) + `ConsultantPlayerGrid`. Recebe `group` + os mapas compartilhados (`storeConversionAvgByStoreId`, `rankingPositionByKey`, `storeProgressByStoreId`, `storePayoutByStoreId`) + `staff` da loja; re-emite `open-details`.
- [ConsultantBadges.vue](/c:/Users/Mike/Documents/Projects/fila-atendimento/web/app/components/consultant/ConsultantBadges.vue) e puro: recebe `stats` + `badges` (config) e renderiza apenas os badges aplicaveis. Regras default em `useGamificationConfig()`.
- [ConsultantDetailsDrawer.vue](/c:/Users/Mike/Documents/Projects/fila-atendimento/web/app/components/consultant/ConsultantDetailsDrawer.vue) usa `USlideover` (Nuxt UI) com 3 modos: `center` (default), `fullscreen`, `side`. Controlado pelo composable `useConsultantDetailsDrawer()` (singleton). 3 tabs: Visao geral, Historico (sparkline 7d), Simulador. Continua sendo o detalhamento da visao integrada (`Todas as lojas`); a pagina individual da loja nao depende mais desse modal.
- [ConsultantSimulator.vue](/c:/Users/Mike/Documents/Projects/fila-atendimento/web/app/components/consultant/ConsultantSimulator.vue) vive inline no `ConsultantWorkspace.vue` (loja ativa) e continua sendo reutilizado dentro do drawer para a visao integrada quando necessario.
- a visao integrada usa filtro de loja local da propria tela; trocar contexto em uma aba nao deve reconfigurar outra rota.
- a visao integrada deve oferecer filtros locais por loja, nome, status e situacao de meta antes de inventar outro painel paralelo.
- com usuario multi-loja, o modo padrao da workspace `consultor` passa a ser a visao integrada com filtro local por loja.
- `cancellationRate` no `stats` e **opcional**: se nao vier no payload, o `ConsultantPlayerCard`/drawer simplesmente nao mostra. Na visao integrada (multi-loja) ja vem populado: a `consultants` store junta `queueStats.byConsultant[].queueCancellationRate` do `GET /v1/erp/crm` (ja calculado no backend, sem recalculo) por `(storeId, personId)` com fallback por nome, injeta como `cancellationRate` nas linhas do ranking, e `useConsultantIntegratedRows` propaga para a `ConsultantRow`. O `ConsultantIntegratedWorkspace` repassa para o card full e para o drawer; o grid mini repassa via `ConsultantPlayerGrid`. A visao single-store (`ConsultantWorkspace.vue` nao-integrada) NAO busca o CRM, entao ali o campo segue ausente (degrada limpo).

## Aviso acionável inline + quick-edit de metas (motor plugável)

Plano: `docs/INLINE_QUICK_EDIT_PLAN.md`. Motor genérico em `web/app/components/quick-edit`
(ver AGENT.md de lá). Onde uma meta falta e isso muda o cálculo, o card mostra um aviso
acionável; quem tem `canManageGoalTargets` clica e abre um popover ancorado que grava via
`/v1/operations/goals` (sem endpoint novo) e re-hidrata o `/v1/erp/crm`.

- **Colocação dos avisos** (escopo decide o lugar, sem repetir):
  - **Meta INDIVIDUAL do consultor** → no card (`ConsultantPlayerCard.vue`, prop `goalContext`):
    1 `<InlineFieldGuard>` (`consultantMonthlyGoal`, "Sem meta individual — R$ X da loja ÷ N") no
    bloco `player-card__goal-alerts`. Sem contexto, nada muda (degrada limpo).
  - **Meta de ticket/PA da LOJA** → no **cabeçalho do grupo da loja** (`ConsultantIntegratedWorkspace`,
    `consultant-integrated-group__alerts`), UMA vez por loja ("no início"), e no topo do single-store.
    `storeTicketGoal`/`storePaGoal`. Evita repetir o mesmo aviso de loja em cada card.
- **Montagem do contexto**: `useGoalQuickEditContext()` (auth + metas + refreshCrm; `month` vem do
  período visto `integratedDateFrom`, não do mês fixo). O grid monta um contexto por row (meta
  individual); o `ConsultantIntegratedWorkspace` monta um `storeContext` por grupo (escopo de loja,
  `consultantId` vazio) para os avisos de loja no cabeçalho.
- **Flags de gap** (contrato congelado `/v1/erp/crm`): por consultor `goalSource`,
  `missingMonthlyGoal/TicketGoal/PaGoal`; por loja `storeGoalSource`, `missingStoreGoal/...`,
  `splitConsultantCount`. Lidos em `consultant-integrated-view.ts` (`ErpMetric`/`ErpStorePayout`),
  propagados por `useConsultantIntegratedRows` (campos na `ConsultantRow`) e pela `GridRow`.
  Antes do rebuild do back vêm ausentes (default seguro = sem aviso).

## Cálculo de P.A.

P.A. (peças por atendimento) **nunca pode ser menor que 1 quando ha venda**: se a pessoa converteu, vendeu pelo menos 1 produto. Implementacao em [admin-metrics.ts:buildConsultantStats](/c:/Users/Mike/Documents/Projects/fila-atendimento/web/app/domain/utils/admin-metrics.ts):

- Contabiliza peças **apenas dos atendimentos convertidos** (`compra` ou `reserva`), nao do total de entries.
- Aplica piso de 1 peça por conversao (fallback para historico antigo sem `productsClosed`): `max(piecesFromConverted, convertedEntries.length)`.
- Divide pelo numero de **conversoes**, nao pelo total de atendimentos: `totalPieces / convertedEntries.length`.
- Sem conversao no periodo: P.A. = 0.

Mesma logica no backend em `back/internal/modules/analytics/service_ranking.go`, `back/internal/modules/erp/repository_crm.go` e `back/internal/modules/reports/service.go`.

## Fonte de dados

- roster por loja via `GET /v1/consultants?storeId=...`
- status vivo consolidado via `GET /v1/operations/overview`
- metricas comparativas integradas derivadas do historico das lojas acessiveis sem depender da workspace `ranking`

## Roadmap pendente — CRM C4 (gamificacao)

Fase `crm-c4` no `roadmap-data.ts` (grupo `crm-360`, status `pending`) prevê redesign desta workspace:

- `ConsultantMetrics.vue` sera removido. Toda a info migra para `ConsultantPlayerCard.vue` (visao primaria) + `ConsultantDetailsDrawer.vue` (detalhes).
- `ConsultantPlayerCard.vue` tem dois modos: `full` (single-store, no `ConsultantWorkspace`) e `mini` (grid all-stores, dentro de `ConsultantPlayerGrid.vue`). Mesmo componente, prop `mode`. Os tiles de KPI foram extraídos para `ConsultantPlayerCardMetrics.vue` (prop `section: 'hero' | 'detail'` + `mode`) e os tipos compartilhados (`PlayerCardStats`/`PlayerCardConsultant`) vivem em `player-card-types.ts` — para manter o card < 500 linhas.
- `ConsultantBadges.vue` e puro: recebe `stats` e devolve badges (Meta batida, Top N, Conversao > media loja, Ticket > meta, PA > meta). Sem fetch interno.
- `ConsultantDetailsDrawer.vue` tem 3 tabs: Visao geral (todos KPIs incluindo cancelamento, fora-da-vez, tempo medio, nao-clientes), Historico (sparkline 7d), Simulador (move `ConsultantSimulator.vue` atual pra dentro).
- Composable `useConsultantDetailsDrawer()` controla open/close/currentConsultantId. Mesma estrutura do composable do ranking.
- Visao all-stores: `ConsultantIntegratedWorkspace.vue` vira wrapper fino que delega para `ConsultantPlayerGrid.vue`. As tabelas (meta-por-loja, comparativo completo) somem da tela principal — viram abas dentro do drawer ou export CSV.
- Cancelamento (`cancellationRate`) ja existe no backend (`back/internal/modules/erp/repository_crm_queue.go:128/154`). Verificar se chega ate `consultants`/`analytics` stores; senao, ajustar DTO.

Plano completo: `~/.claude/plans/consultor-ranking-gamificado.md`.

## Mudancas recentes (auditoria gamificacao)

- `ConsultantWorkspace.vue`, `ConsultantPlayerGrid.vue`, `ConsultantSelector.vue`, `ConsultantSimulator.vue`: convertidos de `<script setup>` sem `lang="ts"` para `<script setup lang="ts">` com interfaces tipadas e `withDefaults(defineProps<{}>(), {...})`.
- `ConsultantHistoryPanel.vue`: adicionado `storeId?: string` na interface `HistoryEntry` (campo acessado no template mas ausente na tipagem anterior).
- `ConsultantIntegratedWorkspace.vue`: reescrito com `<script setup lang="ts">`; logica pesada extraida para composable `~/composables/useConsultantIntegratedRows.ts`. Arquivo reduziu de 792 para 350 linhas.
- `ConsultantPlayerCard.vue`: removido emoji `🎉` do texto `Meta batida`; substituido por texto simples.
- Novo composable: `~/composables/useConsultantIntegratedRows.ts` — exporta `ConsultantRosterItem`, `ConsultantRow`, `useConsultantIntegratedRows()`. Centraliza merge de roster + ranking + overview + operation-goals com tipagem completa.
- `cancellationRate` permanece opcional no `stats`, mas o gap foi fechado na visao integrada: a fonte e `GET /v1/erp/crm` (`queueStats.byConsultant[].queueCancellationRate`), mergeado na `consultants` store e propagado por `useConsultantIntegratedRows` ate o card/drawer. Sem recalculo no front nem novo endpoint/DTO no back. Single-store nao-integrada segue sem o campo (nao busca CRM).
- `CardMetricTile.vue`: cada tile de KPI do `ConsultantPlayerCardMetrics.vue` virou um componente proprio (props `icon`/`label`/`erp`/`note`/`valueClass`/`noteClass`, valor pelo slot default). Layout em 2 linhas: linha 1 = icone + rotulo (+ tag `ERP` empurrada a direita quando a metrica vem do ERP); linha 2 = valor e meta lado a lado (`.card-metrics__body` em `flex`). Para voltar a meta empilhada embaixo do valor, basta trocar o `__body` para `flex-direction: column` (um lugar so). O `ConsultantPlayerCardMetrics.vue` so orquestra os 3 grids (`hero`/`detail`/`mini`).

## CRM C9 — recebimento por meta da loja nos cards + equipe sem fila

Fase `crm-c9` no `roadmap-data.ts` (grupo `crm-360`).

> **ATUALIZADO (comissao v2 — calculo no back):** o calculo do payout saiu do front e
> virou serviço de dominio Go (`queue/commission`), embutido PRONTO no payload
> `GET /v1/erp/crm`. O front e **DISPLAY**: le o `payout` ja calculado, nunca recalcula.
> As funcoes `calculateStoreGoalPayout`/`calculateCrmGoalPayout` foram **removidas** de
> `crm-performance-policy.ts`. So sobrou `mapRoleToPayoutGroup(role)` (display) +
> `normalizeCrmGoalPayoutPolicy` (editor de Metas CRM).

Regra de negocio v2: **consultor** recebe % sobre a PRÓPRIA venda (trava ≥100% da
própria meta + penalidade P.A./Ticket, tudo no back); **gerente** recebe % sobre o total
da loja por faixa do tipo de loja (Shopping/Bairro); **caixa/auxiliar** valor da faixa de
suporte. Tudo vem calculado no back.

- **DTO do payout** (`~/domain/utils/consultant-integrated-view.ts`): `ErpPayout`
  (`amount` em R$, `ratePercent`, `base`, `group`, `ruleLabel`, `penaltyApplied`) e
  `ErpStorePayout` (`storeType`, `storeSold/Goal/Progress`, `managerPayout`,
  `supportPayout`). `normalizeErpPayout` le defensivamente (ausente/null -> null -> "—"/R$ 0,
  pagina funciona antes do rebuild do back).
- **Helpers de display** (`~/domain/utils/consultant-payout-display.ts`):
  `consultantPayoutLabel` ("% da própria venda"), `storeRolePayoutLabel` ("% da loja"),
  `storePayoutForRole(store, role)` (usa `mapRoleToPayoutGroup` p/ escolher manager vs
  support da loja). Card e drawer usam os MESMOS helpers (espelhados).
- **Consultor**: `buildErpMetricsByConsultant` captura `row.payout` do `byConsultant`;
  `useConsultantIntegratedRows` propaga em `ConsultantRow.payout`. `ConsultantPlayerCard`
  mostra "Recebe R$ X · faixa" e o drawer espelha em "Recebimento por meta". `estimatedCommission`
  (comissao por venda) mantida — recebimento e info adicional.
- **Equipe sem fila** (gerente/caixa): `ConsultantStaffPayoutCard` recebe o payout da LOJA
  conforme o papel (`storePayoutForRole`), tanto na single-store quanto no grid agrupado
  multi-loja (`ConsultantPlayerGrid` recebe `storePayoutByStoreId`). Fonte do staff:
  `GET /v1/store-staff`.
- **Payout por loja**: `buildErpStorePayoutByStore` indexa por storeId/slug/codigo;
  `useConsultantIntegratedRows` expoe `storePayoutByStoreId`. `storeProgressByStoreId`
  prefere os numeros da loja vindos do back, com fallback local.
- **Simulador**: `ConsultantSimulator` aceita `payoutRatePercent` (do `payout.ratePercent`
  do back) e mostra "Recebimento estimado" = `(venda+adicional)*ratePercent/100`, rotulado
  como ESTIMATIVA (a logica de faixa/trava/penalidade fica no back).
- **Botao "Mes anterior"**: `ConsultantIntegratedWorkspace` tem "Mes anterior" + "Mes atual";
  `resetIntegratedPreviousMonth()` na `consultants` store espelha `setRankingPreviousMonth`
  (UTC: inicio = `Date.UTC(y, getUTCMonth()-1, 1)`; fim = `Date.UTC(y, getUTCMonth(), 0)`).
- **Cor do gauge / barra da loja**: inalteradas (`goalProgressTier`, `ConsultantStoreGoalBar`).
- **CRM**: `CrmConsultantsSection.vue` le `byConsultant[].payout` do mesmo payload (% própria
  venda) — antes recalculava com `calculateCrmGoalPayout` sobre o total da loja.

> **Dependencia da Trilha A (back):** nomes EXATOS dos campos do payout em
> `GET /v1/erp/crm` (`byConsultant[].payout`, `stores[].{storeType,managerPayout,...}`) e o
> nome do campo aceito no update de loja (assumido `storeType`). O front coda defensivo: se
> vier ausente/null, mostra "—"/R$ 0 sem quebrar.
