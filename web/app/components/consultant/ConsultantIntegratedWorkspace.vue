<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { storeToRefs } from 'pinia'
import { CalendarDays } from 'lucide-vue-next'
import { formatCurrencyBRL } from '~/domain/utils/admin-metrics'
import AppSelectField from '~/components/ui/AppSelectField.vue'
import ConsultantDetailsDrawer from '~/components/consultant/ConsultantDetailsDrawer.vue'
import ConsultantHistoryPanel from '~/components/consultant/ConsultantHistoryPanel.vue'
import ConsultantPlayerCard from '~/components/consultant/ConsultantPlayerCard.vue'
import ConsultantPlayerGrid from '~/components/consultant/ConsultantPlayerGrid.vue'
import ConsultantRecentAttendancesTable from '~/components/consultant/ConsultantRecentAttendancesTable.vue'
import ConsultantSelector from '~/components/consultant/ConsultantSelector.vue'
import ConsultantSimulator from '~/components/consultant/ConsultantSimulator.vue'
import ConsultantStaffPayoutCard from '~/components/consultant/ConsultantStaffPayoutCard.vue'
import { useConsultantDetailsDrawer } from '~/composables/useConsultantDetailsDrawer'
import {
  useConsultantIntegratedRows,
  type ConsultantRosterItem,
} from '~/composables/useConsultantIntegratedRows'
import { useCrmGoalPayoutPolicy } from '~/composables/useCrmGoalPayoutPolicy'
import {
  calculateStoreGoalPayout,
  type CrmGoalPayoutRule,
} from '~/domain/utils/crm-performance-policy'
import { useConsultantsStore } from '~/stores/consultants'

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

const rosterRef = computed(() => props.roster || [])
const rankingRef = computed(() => props.ranking || null)
const overviewRef = computed(() => props.overview || null)

const {
  consultantRows,
  storeConversionAvgByStoreId,
  rankingPositionByKey,
  storeProgressByStoreId,
} = useConsultantIntegratedRows(rosterRef, rankingRef, overviewRef)

const { policy: payoutPolicy } = useCrmGoalPayoutPolicy()

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

function payoutRuleLabel(rule: CrmGoalPayoutRule | null) {
  if (!rule) return 'Sem faixa'
  if (rule.mode === 'amount') return `${formatCurrencyBRL(rule.value)} fixo`
  return `${Number(rule.value).toLocaleString('pt-BR')}% da loja`
}

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
  const payout = calculateStoreGoalPayout({
    storeSold: store?.storeSold ?? 0,
    storeProgress: store?.progress ?? 0,
    policy: payoutPolicy.value,
    role: row.role,
  })
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
    goalPayoutAmount: payout.amount,
    goalPayoutLabel: payoutRuleLabel(payout.rule),
  }
})

const selectedStoreStaff = computed(() => {
  if (!singleStoreMode.value) return []
  const sid = String(selectedStoreConsultant.value?.storeId || '').trim()
  if (!sid) return []
  const store = storeProgressByStoreId.value[sid] ?? null
  return (staffByStoreId.value[sid] || []).map((member) => {
    const payout = calculateStoreGoalPayout({
      storeSold: store?.storeSold ?? 0,
      storeProgress: store?.progress ?? 0,
      policy: payoutPolicy.value,
      role: member.role,
    })
    return {
      member,
      storeGoalProgress: store ? store.progress : null,
      payoutAmount: payout.amount,
      payoutLabel: payoutRuleLabel(payout.rule),
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
          :store-goal-progress="selectedStoreConsultantCard.storeGoalProgress"
          :goal-payout-amount="selectedStoreConsultantCard.goalPayoutAmount"
          :goal-payout-label="selectedStoreConsultantCard.goalPayoutLabel"
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
        <section v-if="selectedStoreStaff.length" class="consultant-integrated-staff">
          <header class="consultant-integrated-staff__header">
            <h3 class="consultant-integrated-staff__title">Equipe da loja (sem fila)</h3>
            <p class="consultant-integrated-staff__text">
              Recebem pela meta da loja; nao atendem na fila.
            </p>
          </header>
          <div class="consultant-integrated-staff__grid">
            <ConsultantStaffPayoutCard
              v-for="item in selectedStoreStaff"
              :key="`staff:${item.member.storeId}:${item.member.id}`"
              :staff="item.member"
              :store-goal-progress="item.storeGoalProgress"
              :payout-amount="item.payoutAmount"
              :payout-label="item.payoutLabel"
            />
          </div>
        </section>

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
            :staff="staffByStoreId[group.storeId] || []"
            :store-conversion-avg-by-store-id="storeConversionAvgByStoreId"
            :ranking-position-by-key="rankingPositionByKey"
            :store-progress-by-store-id="storeProgressByStoreId"
            :payout-policy="payoutPolicy"
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
.consultant-integrated-staff {
  display: grid;
  gap: 0.65rem;
}
.consultant-integrated-staff__header {
  display: grid;
  gap: 0.15rem;
}
.consultant-integrated-staff__title {
  margin: 0;
  font-size: 0.95rem;
  color: rgb(var(--text) / 0.96);
}
.consultant-integrated-staff__text {
  margin: 0;
  font-size: 0.76rem;
  color: rgb(var(--muted) / 0.9);
}
.consultant-integrated-staff__grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(18rem, 1fr));
  gap: 0.85rem;
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
