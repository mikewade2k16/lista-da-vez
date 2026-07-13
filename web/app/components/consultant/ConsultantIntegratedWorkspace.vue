<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { storeToRefs } from 'pinia'
import ConsultantDetailsDrawer from '~/components/consultant/ConsultantDetailsDrawer.vue'
import ConsultantIntegratedFilters from '~/components/consultant/ConsultantIntegratedFilters.vue'
import ConsultantSingleStoreView from '~/components/consultant/ConsultantSingleStoreView.vue'
import ConsultantStoreGroup from '~/components/consultant/ConsultantStoreGroup.vue'
import { useConsultantDetailsDrawer } from '~/composables/useConsultantDetailsDrawer'
import {
  useConsultantIntegratedRows,
  type ConsultantRosterItem,
} from '~/composables/useConsultantIntegratedRows'
import {
  consultantPayoutLabel,
  storePayoutForRole,
  storeRolePayoutLabel,
} from '~/domain/utils/consultant-payout-display'
import { useConsultantsStore } from '~/stores/consultants'
import { useGoalQuickEditContext } from '~/composables/useGoalQuickEditContext'

interface StaffItem {
  id: string
  name: string
  role?: string
  roleLabel?: string
  storeId?: string
  storeName?: string
}

const FILTER_ALL = 'all'

const props = withDefaults(
  defineProps<{
    roster?: ConsultantRosterItem[]
    staff?: StaffItem[]
    ranking?: Record<string, unknown> | null
    overview?: Record<string, unknown> | null
    history?: Array<Record<string, unknown>>
    pending?: boolean
    errorMessage?: string
  }>(),
  {
    roster: () => [],
    staff: () => [],
    ranking: null,
    overview: null,
    history: () => [],
    pending: false,
    errorMessage: '',
  },
)

const searchTerm = ref('')
const storeFilter = ref(FILTER_ALL)
const statusFilter = ref(FILTER_ALL)
const goalFilter = ref(FILTER_ALL)
const selectedConsultantId = ref('')
const simulationAdditionalSales = ref(0)

const consultantsStore = useConsultantsStore()
const { integratedDateFrom, integratedDateTo } = storeToRefs(consultantsStore)
const { buildContext: buildGoalContext } = useGoalQuickEditContext()

const rosterRef = computed(() => props.roster || [])
const rankingRef = computed(() => props.ranking || null)
const overviewRef = computed(() => props.overview || null)

const {
  consultantRows,
  storeConversionAvgByStoreId,
  rankingPositionByKey,
  storeProgressByStoreId,
  storePayoutByStoreId,
} = useConsultantIntegratedRows(rosterRef, rankingRef, overviewRef)

const staffByStoreId = computed(() => {
  const map: Record<string, StaffItem[]> = {}
  ;(props.staff || []).forEach((member) => {
    const sid = String(member.storeId || '').trim()
    if (!sid) return
    if (!map[sid]) map[sid] = []
    map[sid].push(member)
  })
  return map
})

const storeOptions = computed(() => {
  const storesById = new Map<string, { value: string; label: string }>()
  rosterRef.value.forEach((c) => {
    const storeId = String(c.storeId || '').trim()
    const storeName = String(c.storeName || '').trim()
    if (storeId && storeName && !storesById.has(storeId))
      storesById.set(storeId, { value: storeId, label: storeName })
  })
  return [
    { value: FILTER_ALL, label: 'Todas as lojas' },
    ...[...storesById.values()].sort((a, b) => a.label.localeCompare(b.label)),
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

const filteredRows = computed(() => {
  const term = String(searchTerm.value || '')
    .trim()
    .toLowerCase()
  return consultantRows.value.filter((row) => {
    if (storeFilter.value !== FILTER_ALL && row.storeId !== storeFilter.value) return false
    if (statusFilter.value !== FILTER_ALL && row.liveStatusCode !== statusFilter.value) return false
    if (goalFilter.value === 'at-goal' && !row.hitGoal) return false
    if (goalFilter.value === 'off-goal' && (row.hitGoal || row.monthlyGoal <= 0)) return false
    if (goalFilter.value === 'no-goal' && row.monthlyGoal > 0) return false
    if (!term) return true
    return [row.name, row.storeName, row.storeCode, row.storeCity, row.role].some((v) =>
      String(v || '')
        .toLowerCase()
        .includes(term),
    )
  })
})

const singleStoreMode = computed(() => storeFilter.value !== FILTER_ALL)
const singleStoreRows = computed(() => filteredRows.value)

const groupedRows = computed(() => {
  const groups = new Map<
    string,
    { storeId: string; storeName: string; rows: typeof consultantRows.value }
  >()
  filteredRows.value.forEach((row) => {
    const sid = String(row.storeId || '').trim()
    const cur = groups.get(sid) || {
      storeId: sid,
      storeName: String(row.storeName || 'Loja sem nome').trim(),
      rows: [],
    }
    cur.rows.push(row)
    groups.set(sid, cur)
  })
  return [...groups.values()]
    .map((g) => ({
      ...g,
      // Contexto de escopo de LOJA (consultantId vazio) p/ os avisos de ticket/PA no
      // cabeçalho. Semeia o popover com a meta efetiva vinda de um consultor da loja.
      storeContext: buildGoalContext({
        storeId: g.storeId,
        store: storePayoutByStoreId.value[g.storeId] ?? null,
        currentTicketGoal: Number(g.rows[0]?.avgTicketGoal || 0),
        currentPaGoal: Number(g.rows[0]?.paGoal || 0),
      }),
      rows: [...g.rows].sort(
        (a, b) =>
          b.soldValue - a.soldValue || String(a.name || '').localeCompare(String(b.name || '')),
      ),
    }))
    .sort((a, b) => a.storeName.localeCompare(b.storeName))
})

const selectedStoreConsultant = computed(() => {
  if (!singleStoreRows.value.length) return null
  return (
    singleStoreRows.value.find((r) => r.id === selectedConsultantId.value) ||
    singleStoreRows.value[0]
  )
})

const selectedStoreConsultantCard = computed(() => {
  const row = selectedStoreConsultant.value
  if (!row) return null
  const store = storeProgressByStoreId.value[String(row.storeId || '')] ?? null
  // Consultor: payout pré-calculado no back (% da própria venda). Display só.
  const payout = row.payout ?? null
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
      estimatedCommission: row.soldValue * Number(row.commissionRate || 0),
      commissionRate: Number(row.commissionRate || 0),
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
      cancellationRate: row.cancellationRate,
    },
    storeConversionAvg: storeConversionAvgByStoreId.value[String(row.storeId || '')] ?? null,
    rankingPosition:
      rankingPositionByKey.value[`${String(row.storeId || '')}:${String(row.id || '')}`] ?? null,
    storeGoalProgress: store ? store.progress : null,
    goalPayoutAmount: payout ? payout.amount : null,
    goalPayoutLabel: payout ? consultantPayoutLabel(payout) : '',
    // Contexto do quick-edit de metas (motor plugável) para o card full single-store.
    goalContext: buildGoalContext({
      storeId: row.storeId,
      consultantId: row.id,
      store: storePayoutByStoreId.value[String(row.storeId || '')] ?? null,
      currentTicketGoal: Number(row.avgTicketGoal || 0),
      currentPaGoal: Number(row.paGoal || 0),
      consultant: {
        goalSource: row.goalSource,
        missingMonthlyGoal: row.missingMonthlyGoal,
        missingTicketGoal: row.missingTicketGoal,
        missingPaGoal: row.missingPaGoal,
        monthlyGoal: row.monthlyGoal,
      },
    }),
  }
})

const selectedStoreStaff = computed(() => {
  if (!singleStoreMode.value) return []
  const sid = String(selectedStoreConsultant.value?.storeId || '').trim()
  if (!sid) return []
  const store = storeProgressByStoreId.value[sid] ?? null
  const storePayout = storePayoutByStoreId.value[sid] ?? null
  return (staffByStoreId.value[sid] || []).map((member) => {
    // Gerente/caixa recebem pela LOJA; o papel escolhe manager vs support no back.
    const payout = storePayoutForRole(storePayout, member.role)
    return {
      member,
      storeGoalProgress: store ? store.progress : null,
      payoutAmount: payout ? payout.amount : null,
      payoutLabel: payout ? storeRolePayoutLabel(payout) : '',
    }
  })
})

watch(
  singleStoreRows,
  (rows) => {
    if (!rows.length) {
      selectedConsultantId.value = ''
      return
    }
    if (!rows.some((r) => r.id === selectedConsultantId.value))
      selectedConsultantId.value = String(rows[0].id || '')
  },
  { immediate: true },
)

watch(
  () => selectedStoreConsultant.value?.id || '',
  () => {
    simulationAdditionalSales.value = 0
  },
)

const drawer = useConsultantDetailsDrawer()

const selectedDrawerConsultant = computed(() => {
  const id = drawer.currentConsultantId.value
  if (!id) return null
  return consultantRows.value.find((r) => r.id === id) || null
})

const selectedDrawerStats = computed(() => {
  const row = selectedDrawerConsultant.value
  if (!row) return null
  return {
    monthlyGoal: row.monthlyGoal,
    soldValue: row.soldValue,
    remainingToGoal: row.remainingToGoal,
    estimatedCommission: row.soldValue * Number(row.commissionRate || 0),
    commissionRate: Number(row.commissionRate || 0),
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
    cancellationRate: row.cancellationRate,
    // Payout do consultor pré-calculado no back (espelha o card).
    goalPayoutAmount: row.payout ? row.payout.amount : null,
    goalPayoutLabel: row.payout ? consultantPayoutLabel(row.payout) : '',
    goalPayoutRatePercent: row.payout ? row.payout.ratePercent : null,
    monthEntries: (props.history || []).filter((e) => String(e?.personId || '').trim() === row.id),
  }
})

function openDetails(consultantId: string) {
  drawer.open(consultantId)
}
function selectConsultant(consultantId: string) {
  selectedConsultantId.value = String(consultantId || '').trim()
}
function updateSimulationAdditionalSales(value: number) {
  const n = Number(value || 0)
  simulationAdditionalSales.value = Number.isFinite(n) ? n : 0
}
async function applyPeriodFilters() {
  await consultantsStore.applyIntegratedFilters()
}
async function resetPeriodRange() {
  consultantsStore.resetIntegratedCurrentMonth()
  await consultantsStore.applyIntegratedFilters()
}
async function setPreviousMonthRange() {
  consultantsStore.resetIntegratedPreviousMonth()
  await consultantsStore.applyIntegratedFilters()
}
async function setWeekRange(week: number) {
  consultantsStore.setIntegratedWeek(week)
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
      <ConsultantIntegratedFilters
        :search-term="searchTerm"
        :store-filter="storeFilter"
        :status-filter="statusFilter"
        :goal-filter="goalFilter"
        :store-options="storeOptions"
        :status-options="statusOptions"
        :goal-options="goalOptions"
        :date-from="integratedDateFrom"
        :date-to="integratedDateTo"
        :pending="pending"
        @update:search-term="searchTerm = $event"
        @update:store-filter="storeFilter = $event"
        @update:status-filter="statusFilter = $event"
        @update:goal-filter="goalFilter = $event"
        @update:date-from="integratedDateFrom = $event"
        @update:date-to="integratedDateTo = $event"
        @apply="applyPeriodFilters"
        @reset-current-month="resetPeriodRange"
        @set-previous-month="setPreviousMonthRange"
        @set-week="setWeekRange"
      />

      <ConsultantSingleStoreView
        v-if="singleStoreMode"
        :rows="singleStoreRows"
        :selected-consultant="selectedStoreConsultant"
        :card="selectedStoreConsultantCard"
        :staff="selectedStoreStaff"
        :history="props.history || []"
        :simulation-additional-sales="simulationAdditionalSales"
        @select="selectConsultant"
        @update:simulation-additional-sales="updateSimulationAdditionalSales"
      />

      <div v-else-if="groupedRows.length" class="consultant-integrated-groups">
        <ConsultantStoreGroup
          v-for="group in groupedRows"
          :key="group.storeId"
          :group="group"
          :staff="staffByStoreId[group.storeId] || []"
          :store-conversion-avg-by-store-id="storeConversionAvgByStoreId"
          :ranking-position-by-key="rankingPositionByKey"
          :store-progress-by-store-id="storeProgressByStoreId"
          :store-payout-by-store-id="storePayoutByStoreId"
          @open-details="openDetails"
        />
      </div>
      <div v-else class="player-grid__empty" data-testid="player-grid-empty">
        Nenhum consultor encontrado para os filtros selecionados.
      </div>
    </template>

    <ConsultantDetailsDrawer :consultant="selectedDrawerConsultant" :stats="selectedDrawerStats" />
  </section>
</template>

<style scoped>
.consultant-integrated-groups {
  display: grid;
  gap: 1rem;
}
.player-grid__empty {
  padding: 2rem;
  text-align: center;
  color: rgb(var(--muted) / 0.92);
}
</style>
