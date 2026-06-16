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
- [ConsultantIntegratedWorkspace.vue](/c:/Users/Mike/Documents/Projects/fila-atendimento/web/app/components/consultant/ConsultantIntegratedWorkspace.vue) alterna 2 layouts locais: com filtro de `Loja` em uma loja especifica, mostra abas de consultores (`ConsultantSelector`) + `ConsultantPlayerCard` full + `ConsultantHistoryPanel` + `ConsultantSimulator`; em `Todas as lojas`, mostra `ConsultantPlayerGrid` agrupado por loja com separadores visuais. O drawer continua servindo apenas o modo em cards/all-stores.
- [ConsultantBadges.vue](/c:/Users/Mike/Documents/Projects/fila-atendimento/web/app/components/consultant/ConsultantBadges.vue) e puro: recebe `stats` + `badges` (config) e renderiza apenas os badges aplicaveis. Regras default em `useGamificationConfig()`.
- [ConsultantDetailsDrawer.vue](/c:/Users/Mike/Documents/Projects/fila-atendimento/web/app/components/consultant/ConsultantDetailsDrawer.vue) usa `USlideover` (Nuxt UI) com 3 modos: `center` (default), `fullscreen`, `side`. Controlado pelo composable `useConsultantDetailsDrawer()` (singleton). 3 tabs: Visao geral, Historico (sparkline 7d), Simulador. Continua sendo o detalhamento da visao integrada (`Todas as lojas`); a pagina individual da loja nao depende mais desse modal.
- [ConsultantSimulator.vue](/c:/Users/Mike/Documents/Projects/fila-atendimento/web/app/components/consultant/ConsultantSimulator.vue) vive inline no `ConsultantWorkspace.vue` (loja ativa) e continua sendo reutilizado dentro do drawer para a visao integrada quando necessario.
- a visao integrada usa filtro de loja local da propria tela; trocar contexto em uma aba nao deve reconfigurar outra rota.
- a visao integrada deve oferecer filtros locais por loja, nome, status e situacao de meta antes de inventar outro painel paralelo.
- com usuario multi-loja, o modo padrao da workspace `consultor` passa a ser a visao integrada com filtro local por loja.
- `cancellationRate` no `stats` e **opcional**: se nao vier no payload, o `ConsultantPlayerCard`/drawer simplesmente nao mostra. Na visao integrada (multi-loja) ja vem populado: a `consultants` store junta `queueStats.byConsultant[].queueCancellationRate` do `GET /v1/erp/crm` (ja calculado no backend, sem recalculo) por `(storeId, personId)` com fallback por nome, injeta como `cancellationRate` nas linhas do ranking, e `useConsultantIntegratedRows` propaga para a `ConsultantRow`. O `ConsultantIntegratedWorkspace` repassa para o card full e para o drawer; o grid mini repassa via `ConsultantPlayerGrid`. A visao single-store (`ConsultantWorkspace.vue` nao-integrada) NAO busca o CRM, entao ali o campo segue ausente (degrada limpo).

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
- `ConsultantPlayerCard.vue` tem dois modos: `full` (single-store, no `ConsultantWorkspace`) e `mini` (grid all-stores, dentro de `ConsultantPlayerGrid.vue`). Mesmo componente, prop `mode`.
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

## CRM C9 — recebimento por meta da loja nos cards + equipe sem fila

Fase `crm-c9` no `roadmap-data.ts` (grupo `crm-360`). Regra de negocio (decidida pelo usuario): o recebimento por atingimento de meta e destravado pelo **% da meta da LOJA** (gatilho coletivo unico) e, quando a faixa e percentual, o **% incide sobre o total vendido da loja** para todos os papeis. Caixa/auxiliar usam valor fixo (`amount`).

- **Cor do gauge por faixa**: `ConsultantPlayerCard.vue` colore o `gauge-fill` pela faixa do `%` individual via `goalProgressTier()` (`~/domain/utils/goal-progress-color.ts`): `low` danger, `mid` accent-warning, `high` primary, `hit` success, `none` muted (sem meta). Mantem o texto "Sem meta cadastrada".
- **Barra de % da loja**: componente unico [ConsultantStoreGoalBar.vue](/c:/Users/Mike/Documents/Projects/fila-atendimento/web/app/components/consultant/ConsultantStoreGoalBar.vue) (barra + label "Meta da loja", colorida pela mesma faixa). Reutilizado pelo card de consultor e pelo card de staff (sem duplicar markup/CSS).
- **Recebimento no card**: `ConsultantPlayerCard` mostra "Recebe R$ X · faixa" abaixo da barra; valor vem de `calculateStoreGoalPayout()` (`~/domain/utils/crm-performance-policy.ts`), com a politica lida por `useCrmGoalPayoutPolicy()` (settings de operacao no runtime). `estimatedCommission` (comissao por venda) foi mantido — recebimento por meta e informacao adicional, nao substituicao.
- **Equipe sem fila**: gerente/caixa/auxiliar nao atendem na fila e aparecem **ao lado dos consultores** como [ConsultantStaffPayoutCard.vue](/c:/Users/Mike/Documents/Projects/fila-atendimento/web/app/components/consultant/ConsultantStaffPayoutCard.vue) (card enxuto: nome, papel, barra da loja, recebimento). Fonte: `GET /v1/store-staff` (modulo Go `queue/consultants`), carregado pela `consultants` store em `integratedStaff` e agrupado por loja no `ConsultantIntegratedWorkspace` (grid all-stores) e na visao single-store ("Equipe da loja (sem fila)").
- **Progresso da loja**: `useConsultantIntegratedRows` expoe `storeProgressByStoreId` (`storeSold`/`storeGoal`/`progress`) e `storeTotalSoldByStoreId`. Meta da loja = `operationgoals` scope=store; fallback = soma das metas individuais da loja.
- **Mapeamento de papel -> grupo**: `mapRoleToPayoutGroup()` (gerente->`manager`; caixa/auxiliar/cashier->`support`; demais->`consultant`).
- A tabela do CRM (`CrmConsultantsSection.vue`) usa a MESMA base (total vendido da loja, via `storeSoldCentsBySlug`) para a coluna "Recebimento" — antes calculava sobre as vendas do proprio consultor.
