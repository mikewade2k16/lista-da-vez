<script setup>
import { computed, onMounted, ref, watch } from 'vue'
import { storeToRefs } from 'pinia'
import { CalendarDays } from 'lucide-vue-next'
import AppSelectField from '~/components/ui/AppSelectField.vue'
import ConsultantDetailsDrawer from '~/components/consultant/ConsultantDetailsDrawer.vue'
import ConsultantHistoryPanel from '~/components/consultant/ConsultantHistoryPanel.vue'
import ConsultantPlayerCard from '~/components/consultant/ConsultantPlayerCard.vue'
import ConsultantPlayerGrid from '~/components/consultant/ConsultantPlayerGrid.vue'
import ConsultantRecentAttendancesTable from '~/components/consultant/ConsultantRecentAttendancesTable.vue'
import ConsultantSelector from '~/components/consultant/ConsultantSelector.vue'
import ConsultantSimulator from '~/components/consultant/ConsultantSimulator.vue'
import { useConsultantDetailsDrawer } from '~/composables/useConsultantDetailsDrawer'
import { useAuthStore } from '~/stores/auth'
import { useConsultantsStore } from '~/stores/consultants'
import { useOperationGoalsStore } from '~/stores/operation-goals'

const FILTER_ALL = 'all'

const props = defineProps({
  roster: {
    type: Array,
    default: () => [],
  },
  ranking: {
    type: Object,
    default: null,
  },
  overview: {
    type: Object,
    default: null,
  },
  history: {
    type: Array,
    default: () => [],
  },
  pending: {
    type: Boolean,
    default: false,
  },
  errorMessage: {
    type: String,
    default: '',
  },
})

const searchTerm = ref('')
const storeFilter = ref(FILTER_ALL)
const statusFilter = ref(FILTER_ALL)
const goalFilter = ref(FILTER_ALL)
const selectedConsultantId = ref('')
const simulationAdditionalSales = ref(0)
const consultantsStore = useConsultantsStore()
const { integratedDateFrom, integratedDateTo } = storeToRefs(consultantsStore)
const auth = useAuthStore()
const operationGoalsStore = useOperationGoalsStore()
const { goals: operationGoalRows } = storeToRefs(operationGoalsStore)

function currentMonthKey() {
  const now = new Date()
  return `${now.getUTCFullYear()}-${String(now.getUTCMonth() + 1).padStart(2, '0')}`
}

async function ensureOperationGoalsLoaded() {
  if (!auth.isAuthenticated || !auth.activeTenantId) return
  try {
    await operationGoalsStore.loadGoals({
      tenantId: auth.activeTenantId,
      month: currentMonthKey(),
    })
  } catch {
    // silencioso: a tela funciona com fallback do roster
  }
}

onMounted(() => {
  void ensureOperationGoalsLoaded()
})

watch(
  () => [auth.isAuthenticated, auth.activeTenantId],
  () => {
    void ensureOperationGoalsLoaded()
  },
)

const goalByConsultantId = computed(() => {
  const map = new Map()
  for (const row of operationGoalRows.value || []) {
    if (row?.scope !== 'consultant' || !row?.consultantId) continue
    map.set(String(row.consultantId).trim(), Number(row.monthlyGoal) || 0)
  }
  return map
})

const goalByStoreId = computed(() => {
  const map = new Map()
  for (const row of operationGoalRows.value || []) {
    if (row?.scope !== 'store' || !row?.storeId) continue
    map.set(String(row.storeId).trim(), Number(row.monthlyGoal) || 0)
  }
  return map
})

function resolveMonthlyGoal(consultant) {
  const consultantGoal = goalByConsultantId.value.get(String(consultant?.id || '').trim())
  if (typeof consultantGoal === 'number' && consultantGoal > 0) return consultantGoal
  const storeGoal = goalByStoreId.value.get(String(consultant?.storeId || '').trim())
  if (typeof storeGoal === 'number' && storeGoal > 0) return storeGoal
  return Math.max(0, Number(consultant?.monthlyGoal || 0) || 0)
}

function buildRowKey(storeId, consultantId) {
  return `${String(storeId || '').trim()}:${String(consultantId || '').trim()}`
}

function normalizeStatusEntry(code, label) {
  return {
    code,
    label,
  }
}

function resolveRankingRow(map, consultant) {
  return (
    map.get(buildRowKey(consultant.storeId, consultant.id)) ||
    map.get(buildRowKey('', consultant.id)) ||
    null
  )
}

const monthlyRowsMap = computed(
  () =>
    new Map(
      (props.ranking?.monthlyRows || []).map((row) => [
        buildRowKey(row.storeId, row.consultantId),
        row,
      ]),
    ),
)
const dailyRowsMap = computed(
  () =>
    new Map(
      (props.ranking?.dailyRows || []).map((row) => [
        buildRowKey(row.storeId, row.consultantId),
        row,
      ]),
    ),
)
const statusMap = computed(() => {
  const nextMap = new Map()

  ;(props.overview?.activeServices || []).forEach((item) => {
    nextMap.set(
      buildRowKey(item.storeId, item.personId),
      normalizeStatusEntry('service', 'Em atendimento'),
    )
  })
  ;(props.overview?.waitingList || []).forEach((item) => {
    nextMap.set(buildRowKey(item.storeId, item.personId), normalizeStatusEntry('queue', 'Na fila'))
  })
  ;(props.overview?.pausedEmployees || []).forEach((item) => {
    const code = String(item.pauseKind || '').trim() === 'assignment' ? 'assignment' : 'paused'
    const label = code === 'assignment' ? 'Em tarefa' : 'Pausado'
    nextMap.set(buildRowKey(item.storeId, item.personId), normalizeStatusEntry(code, label))
  })
  ;(props.overview?.availableConsultants || []).forEach((item) => {
    nextMap.set(
      buildRowKey(item.storeId, item.personId),
      normalizeStatusEntry('available', 'Disponivel'),
    )
  })

  return nextMap
})
const storeOptions = computed(() => {
  const storesById = new Map()

  ;(props.roster || []).forEach((consultant) => {
    const storeId = String(consultant.storeId || '').trim()
    const storeName = String(consultant.storeName || '').trim()

    if (!storeId || !storeName || storesById.has(storeId)) {
      return
    }

    storesById.set(storeId, {
      value: storeId,
      label: storeName,
    })
  })

  return [
    { value: FILTER_ALL, label: 'Todas as lojas' },
    ...[...storesById.values()].sort((left, right) => left.label.localeCompare(right.label)),
  ]
})
const statusOptions = [
  { value: FILTER_ALL, label: 'Todos os status' },
  { value: 'available', label: 'Disponivel' },
  { value: 'service', label: 'Em atendimento' },
  { value: 'queue', label: 'Na fila' },
  { value: 'paused', label: 'Pausado' },
  { value: 'assignment', label: 'Em tarefa' },
]
const goalOptions = [
  { value: FILTER_ALL, label: 'Todas as metas' },
  { value: 'at-goal', label: 'Batendo meta' },
  { value: 'off-goal', label: 'Abaixo da meta' },
  { value: 'no-goal', label: 'Sem meta cadastrada' },
]

const consultantRows = computed(() =>
  (props.roster || []).map((consultant) => {
    const monthly = resolveRankingRow(monthlyRowsMap.value, consultant) || {}
    const daily = resolveRankingRow(dailyRowsMap.value, consultant) || {}
    const liveStatus =
      statusMap.value.get(buildRowKey(consultant.storeId, consultant.id)) ||
      normalizeStatusEntry('available', 'Disponivel')
    const monthlyGoal = resolveMonthlyGoal(consultant)
    const soldValue = Math.max(0, Number(monthly.soldValue || 0) || 0)
    const dailySoldValue = Math.max(0, Number(daily.soldValue || 0) || 0)
    const attendances = Math.max(0, Number(monthly.attendances || 0) || 0)
    const conversions = Math.max(0, Number(monthly.conversions || 0) || 0)
    const progress = monthlyGoal > 0 ? (soldValue / monthlyGoal) * 100 : 0

    return {
      ...consultant,
      liveStatusCode: liveStatus.code,
      liveStatusLabel: liveStatus.label,
      monthlyGoal,
      soldValue,
      dailySoldValue,
      attendances,
      conversions,
      conversionRate: Math.max(0, Number(monthly.conversionRate || 0) || 0),
      ticketAverage: Math.max(0, Number(monthly.ticketAverage || 0) || 0),
      paScore: Math.max(0, Number(monthly.paScore || 0) || 0),
      erpOrders: Math.max(0, Number(monthly.erpOrders || 0) || 0),
      soldValueSource: String(monthly.soldValueSource || ''),
      ticketAverageSource: String(monthly.ticketAverageSource || ''),
      paScoreSource: String(monthly.paScoreSource || ''),
      qualityScore: Math.max(0, Number(monthly.qualityScore || 0) || 0),
      avgDurationMs: Math.max(0, Number(monthly.avgDurationMs || 0) || 0),
      queueJumpServices: Math.max(0, Number(monthly.queueJumpServices || 0) || 0),
      progress,
      hitGoal: monthlyGoal > 0 && soldValue >= monthlyGoal,
      remainingToGoal: Math.max(0, monthlyGoal - soldValue),
    }
  }),
)

const filteredRows = computed(() => {
  const normalizedSearch = String(searchTerm.value || '')
    .trim()
    .toLowerCase()

  return consultantRows.value.filter((row) => {
    if (storeFilter.value !== FILTER_ALL && row.storeId !== storeFilter.value) {
      return false
    }

    if (statusFilter.value !== FILTER_ALL && row.liveStatusCode !== statusFilter.value) {
      return false
    }

    if (goalFilter.value === 'at-goal' && !row.hitGoal) {
      return false
    }

    if (goalFilter.value === 'off-goal' && (row.hitGoal || row.monthlyGoal <= 0)) {
      return false
    }

    if (goalFilter.value === 'no-goal' && row.monthlyGoal > 0) {
      return false
    }

    if (!normalizedSearch) {
      return true
    }

    return [row.name, row.storeName, row.storeCode, row.storeCity, row.role].some((value) =>
      String(value || '')
        .toLowerCase()
        .includes(normalizedSearch),
    )
  })
})

const singleStoreMode = computed(() => storeFilter.value !== FILTER_ALL)

const singleStoreRows = computed(() => filteredRows.value)

const storeConversionAvgByStoreId = computed(() => {
  const map = {}
  const grouped = new Map()

  consultantRows.value.forEach((row) => {
    const storeId = row.storeId
    const current = grouped.get(storeId) || { attendances: 0, conversions: 0 }
    current.attendances += row.attendances
    current.conversions += row.conversions
    grouped.set(storeId, current)
  })

  grouped.forEach((value, storeId) => {
    map[storeId] = value.attendances > 0 ? (value.conversions / value.attendances) * 100 : 0
  })

  return map
})

const rankingPositionByKey = computed(() => {
  const positions = {}
  const sorted = [...consultantRows.value].sort((a, b) => b.soldValue - a.soldValue)
  sorted.forEach((row, index) => {
    positions[`${row.storeId}:${row.id}`] = index + 1
  })
  return positions
})

const groupedRows = computed(() => {
  const groups = new Map()

  filteredRows.value.forEach((row) => {
    const storeId = String(row.storeId || '').trim()
    const current = groups.get(storeId) || {
      storeId,
      storeName: String(row.storeName || 'Loja sem nome').trim() || 'Loja sem nome',
      rows: [],
    }

    current.rows.push(row)
    groups.set(storeId, current)
  })

  return [...groups.values()]
    .map((group) => ({
      ...group,
      rows: [...group.rows].sort((left, right) => {
        if (right.soldValue !== left.soldValue) {
          return right.soldValue - left.soldValue
        }

        return String(left.name || '').localeCompare(String(right.name || ''))
      }),
    }))
    .sort((left, right) => left.storeName.localeCompare(right.storeName))
})

const selectedStoreConsultant = computed(() => {
  if (!singleStoreRows.value.length) {
    return null
  }

  return (
    singleStoreRows.value.find((row) => row.id === selectedConsultantId.value) ||
    singleStoreRows.value[0]
  )
})

const selectedStoreConsultantCard = computed(() => {
  const row = selectedStoreConsultant.value
  if (!row) return null

  return {
    consultant: {
      id: row.id,
      name: row.name,
      role: row.role,
      storeName: row.storeName,
      liveStatusCode: row.liveStatusCode,
      liveStatusLabel: row.liveStatusLabel,
    },
    stats: {
      monthlyGoal: row.monthlyGoal,
      soldValue: row.soldValue,
      remainingToGoal: row.remainingToGoal,
      estimatedCommission: row.soldValue * (Number(row.commissionRate || 0) || 0),
      commissionRate: Number(row.commissionRate || 0) || 0,
      ticketAverage: row.ticketAverage,
      paScore: row.paScore,
      erpOrders: row.erpOrders,
      soldValueSource: row.soldValueSource,
      ticketAverageSource: row.ticketAverageSource,
      paScoreSource: row.paScoreSource,
      conversionRate: row.conversionRate,
      conversions: row.conversions,
      nonConversions: Math.max(0, row.attendances - row.conversions),
      averageDurationMs: row.avgDurationMs,
      nonClientConversions: 0,
      queueJumpServices: row.queueJumpServices,
      avgTicketGoal: Number(row.avgTicketGoal || 0),
      paGoal: Number(row.paGoal || 0),
      conversionGoal: Number(row.conversionGoal || 0),
    },
    storeConversionAvg: storeConversionAvgByStoreId.value[row.storeId] ?? null,
    rankingPosition: rankingPositionByKey.value[`${row.storeId}:${row.id}`] ?? null,
  }
})

watch(
  singleStoreRows,
  (rows) => {
    if (!rows.length) {
      selectedConsultantId.value = ''
      return
    }

    if (!rows.some((row) => row.id === selectedConsultantId.value)) {
      selectedConsultantId.value = rows[0].id
    }
  },
  { immediate: true },
)

const drawer = useConsultantDetailsDrawer()

const selectedDrawerConsultant = computed(() => {
  const id = drawer.currentConsultantId.value
  if (!id) return null
  return consultantRows.value.find((row) => row.id === id) || null
})

const selectedDrawerStats = computed(() => {
  const row = selectedDrawerConsultant.value
  if (!row) return null

  return {
    monthlyGoal: row.monthlyGoal,
    soldValue: row.soldValue,
    remainingToGoal: row.remainingToGoal,
    estimatedCommission: row.soldValue * (Number(row.commissionRate || 0) || 0),
    commissionRate: Number(row.commissionRate || 0) || 0,
    ticketAverage: row.ticketAverage,
    paScore: row.paScore,
    erpOrders: row.erpOrders,
    soldValueSource: row.soldValueSource,
    ticketAverageSource: row.ticketAverageSource,
    paScoreSource: row.paScoreSource,
    conversionRate: row.conversionRate,
    conversions: row.conversions,
    nonConversions: Math.max(0, row.attendances - row.conversions),
    averageDurationMs: row.avgDurationMs,
    queueJumpServices: row.queueJumpServices,
    nonClientConversions: 0,
    avgTicketGoal: Number(row.avgTicketGoal || 0),
    paGoal: Number(row.paGoal || 0),
    conversionGoal: Number(row.conversionGoal || 0),
    monthEntries: (props.history || []).filter(
      (entry) => String(entry?.personId || '').trim() === row.id,
    ),
  }
})

watch(
  () => selectedStoreConsultant.value?.id || '',
  () => {
    simulationAdditionalSales.value = 0
  },
)

function openDetails(consultantId) {
  drawer.open(consultantId)
}

function selectConsultant(consultantId) {
  selectedConsultantId.value = String(consultantId || '').trim()
}

function updateSimulationAdditionalSales(value) {
  const numeric = Number(value || 0)
  simulationAdditionalSales.value = Number.isFinite(numeric) ? numeric : 0
}

async function applyPeriodFilters() {
  await consultantsStore.applyIntegratedFilters()
}

async function resetPeriodRange() {
  consultantsStore.resetIntegratedCurrentMonth()
  await consultantsStore.applyIntegratedFilters()
}
</script>

<template>
  <section class="admin-panel" data-testid="consultant-integrated-panel">
    <header class="admin-panel__header">
      <h2 class="admin-panel__title">Consultores em todas as lojas</h2>
      <p class="admin-panel__text">
        Comparativo consolidado de meta, conversao, ticket e status operacional do tenant ativo.
      </p>
    </header>

    <article v-if="errorMessage" class="insight-card">
      <p class="settings-card__text">{{ errorMessage }}</p>
    </article>

    <article v-else-if="pending && !consultantRows.length" class="insight-card">
      <p class="settings-card__text">Carregando comparativo consolidado dos consultores...</p>
    </article>

    <template v-else>
      <article class="settings-card consultant-integrated-filters">
        <div class="consultant-integrated-filters__grid">
          <label class="settings-field consultant-integrated-filters__search">
            <span>Buscar consultor</span>
            <input v-model="searchTerm" type="text" placeholder="Nome, loja ou cargo" />
          </label>
          <label class="settings-field">
            <span>Loja</span>
            <AppSelectField
              :model-value="storeFilter"
              :options="storeOptions"
              placeholder="Filtrar loja"
              @update:model-value="storeFilter = $event"
            />
          </label>
          <label class="settings-field">
            <span>Status</span>
            <AppSelectField
              :model-value="statusFilter"
              :options="statusOptions"
              placeholder="Filtrar status"
              @update:model-value="statusFilter = $event"
            />
          </label>
          <label class="settings-field">
            <span>Meta</span>
            <AppSelectField
              :model-value="goalFilter"
              :options="goalOptions"
              placeholder="Filtrar meta"
              @update:model-value="goalFilter = $event"
            />
          </label>
          <div class="consultant-integrated-filters__period">
            <label class="settings-field consultant-integrated-filters__period-field">
              <span>Periodo</span>
              <AppDatePicker
                :model-value="integratedDateFrom"
                :end-date="integratedDateTo"
                @update:model-value="integratedDateFrom = $event"
                @update:end-date="integratedDateTo = $event"
              >
                <template #default="{ label }">
                  <button type="button" class="consultant-integrated-date-trigger">
                    <CalendarDays :size="14" />
                    <span>{{ label || 'Mes atual' }}</span>
                  </button>
                </template>
              </AppDatePicker>
            </label>
            <div class="consultant-integrated-filters__actions">
              <button
                type="button"
                class="consultant-integrated-btn consultant-integrated-btn--ghost"
                :disabled="pending"
                @click="resetPeriodRange"
              >
                Mes atual
              </button>
              <button
                type="button"
                class="consultant-integrated-btn"
                :disabled="pending"
                @click="applyPeriodFilters"
              >
                {{ pending ? 'Atualizando...' : 'Atualizar' }}
              </button>
            </div>
          </div>
        </div>
      </article>

      <template v-if="singleStoreMode">
        <ConsultantSelector
          v-if="singleStoreRows.length"
          :roster="singleStoreRows"
          :selected-consultant-id="selectedStoreConsultant?.id || ''"
          @select="selectConsultant"
        />

        <ConsultantPlayerCard
          v-if="selectedStoreConsultantCard"
          :consultant="selectedStoreConsultantCard.consultant"
          :stats="selectedStoreConsultantCard.stats"
          :store-conversion-avg="selectedStoreConsultantCard.storeConversionAvg"
          :ranking-position="selectedStoreConsultantCard.rankingPosition"
          mode="full"
          :show-details-button="false"
        />

        <div v-if="selectedStoreConsultantCard" class="consultant-integrated-insights">
          <ConsultantHistoryPanel
            :consultant-id="selectedStoreConsultantCard.consultant.id"
            :store-id="selectedStoreConsultant?.storeId"
            :entries="props.history || []"
          />

          <section class="consultant-integrated-insight-panel">
            <ConsultantSimulator
              :sold-value="selectedStoreConsultantCard.stats.soldValue"
              :monthly-goal="selectedStoreConsultantCard.stats.monthlyGoal"
              :commission-rate="selectedStoreConsultantCard.stats.commissionRate"
              :simulation-additional-sales="simulationAdditionalSales"
              @update:simulation-additional-sales="updateSimulationAdditionalSales"
            />
          </section>
        </div>

        <ConsultantRecentAttendancesTable
          v-if="selectedStoreConsultant"
          :consultant-id="selectedStoreConsultant.id"
          :consultant-name="selectedStoreConsultant.name"
          :store-id="selectedStoreConsultant.storeId"
          :store-name="selectedStoreConsultant.storeName"
          :entries="props.history || []"
        />

        <div v-else class="player-grid__empty" data-testid="player-grid-empty">
          Nenhum consultor encontrado para os filtros selecionados.
        </div>
      </template>

      <div v-else-if="groupedRows.length" class="consultant-integrated-groups">
        <section
          v-for="group in groupedRows"
          :key="group.storeId"
          class="consultant-integrated-group"
        >
          <header class="consultant-integrated-group__header">
            <div>
              <h3 class="consultant-integrated-group__title">{{ group.storeName }}</h3>
              <p class="consultant-integrated-group__text">
                {{ group.rows.length }} consultor(es) nos filtros atuais.
              </p>
            </div>
          </header>

          <ConsultantPlayerGrid
            :rows="group.rows"
            :store-conversion-avg-by-store-id="storeConversionAvgByStoreId"
            :ranking-position-by-key="rankingPositionByKey"
            @open-details="openDetails"
          />
        </section>
      </div>

      <div v-else class="player-grid__empty" data-testid="player-grid-empty">
        Nenhum consultor encontrado para os filtros selecionados.
      </div>
    </template>

    <ConsultantDetailsDrawer :consultant="selectedDrawerConsultant" :stats="selectedDrawerStats" />
  </section>
</template>

<style scoped>
.consultant-integrated-filters__grid {
  display: grid;
  grid-template-columns: minmax(0, 1.5fr) repeat(3, minmax(0, 1fr)) minmax(0, 1.4fr);
  gap: 0.85rem;
}

.consultant-integrated-filters__search {
  min-width: 0;
}

.consultant-integrated-filters__period {
  display: grid;
  gap: 0.65rem;
  min-width: 0;
}

.consultant-integrated-filters__period-field {
  min-width: 0;
}

.consultant-integrated-date-trigger {
  display: inline-flex;
  align-items: center;
  gap: 0.45rem;
  min-width: 100%;
  min-height: 42px;
  padding: 0 0.85rem;
  border-radius: 12px;
  border: 1px solid rgb(var(--border) / 0.9);
  background: rgb(var(--surface) / 0.95);
  color: rgb(var(--text));
  font-size: 0.88rem;
  font-weight: 600;
  text-align: left;
  cursor: pointer;
  white-space: nowrap;
}

.consultant-integrated-filters__actions {
  display: flex;
  gap: 0.5rem;
  flex-wrap: wrap;
}

.consultant-integrated-btn {
  min-height: 38px;
  border: none;
  border-radius: 12px;
  padding: 0.6rem 0.9rem;
  background: rgb(var(--primary));
  color: rgb(255 255 255);
  font-weight: 700;
  cursor: pointer;
  white-space: nowrap;
}

.consultant-integrated-btn:disabled {
  cursor: wait;
  opacity: 0.72;
}

.consultant-integrated-btn--ghost {
  background: rgb(var(--primary) / 0.12);
  color: rgb(var(--primary));
}

.consultant-integrated-groups {
  display: grid;
  gap: 1rem;
}

.consultant-integrated-insights {
  display: grid;
  grid-template-columns: minmax(0, 1.3fr) minmax(0, 1fr);
  gap: 0.85rem;
  align-items: start;
}

.consultant-integrated-insight-panel {
  padding: 1rem;
  border-radius: 1rem;
  border: 1px solid rgb(var(--primary) / 0.16);
  background: rgb(var(--surface) / 0.78);
  box-shadow: var(--shadow-xs);
}

.consultant-integrated-group {
  display: grid;
  gap: 0.85rem;
}

.consultant-integrated-group__header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.75rem;
  padding-bottom: 0.35rem;
  border-bottom: 1px solid rgb(var(--border) / 0.72);
}

.consultant-integrated-group__title {
  margin: 0;
  font-size: 1rem;
  color: rgb(var(--text) / 0.96);
}

.consultant-integrated-group__text {
  margin: 0.2rem 0 0;
  font-size: 0.78rem;
  color: rgb(var(--muted) / 0.9);
}

.player-grid__empty {
  padding: 2rem;
  text-align: center;
  color: rgb(var(--muted) / 0.92);
}

@media (max-width: 1100px) {
  .consultant-integrated-insights,
  .consultant-integrated-filters__grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}

@media (max-width: 720px) {
  .consultant-integrated-insights,
  .consultant-integrated-filters__grid {
    grid-template-columns: minmax(0, 1fr);
  }
}
</style>
