<script setup lang="ts">
import { computed, ref } from 'vue'
import { buildConsultantStats } from '~/domain/utils/admin-metrics'
import ConsultantHistoryPanel from '~/components/consultant/ConsultantHistoryPanel.vue'
import ConsultantIntegratedWorkspace from '~/components/consultant/ConsultantIntegratedWorkspace.vue'
import ConsultantPlayerCard from '~/components/consultant/ConsultantPlayerCard.vue'
import ConsultantRecentAttendancesTable from '~/components/consultant/ConsultantRecentAttendancesTable.vue'
import ConsultantSelector from '~/components/consultant/ConsultantSelector.vue'
import ConsultantSimulator from '~/components/consultant/ConsultantSimulator.vue'
import PerformanceFeedbackModal from '~/components/performance-feedback/PerformanceFeedbackModal.vue'
import PerformanceFeedbackSettingsButton from '~/components/performance-feedback/PerformanceFeedbackSettingsButton.vue'
import { canViewPerformanceFeedback } from '~/domain/utils/permissions'
import { useAuthStore } from '~/stores/auth'
import { useConsultantsStore } from '~/stores/consultants'
import type { ConsultantRosterItem } from '~/composables/useConsultantIntegratedRows'
import type { PerformanceFeedbackTarget } from '~/types/performance-feedback'
import { performanceFeedbackMetricsFromCard } from '~/domain/utils/performance-feedback'

// Espelha a interface StaffItem de ConsultantIntegratedWorkspace.vue (não exportada lá);
// usada apenas para tipar o prop integratedStaff repassado ao componente filho.
interface IntegratedStaffItem {
  id: string
  name: string
  role?: string
  roleLabel?: string
  storeId?: string
  storeName?: string
}

// Forma mínima de um item de serviceHistory consumida neste componente.
interface ServiceHistoryEntry extends Record<string, unknown> {
  personId?: string
  finishOutcome?: string
}

interface RosterItem {
  id: string
  name: string
  storeId?: string
  storeName?: string
  monthlyGoal?: number
  commissionRate?: number
  conversionGoal?: number
  avgTicketGoal?: number
  paGoal?: number
  [key: string]: unknown
}

interface WorkspaceState {
  roster?: RosterItem[]
  selectedConsultantId?: string
  serviceHistory?: ServiceHistoryEntry[]
  consultantSimulationAdditionalSales?: number
  visitReasonOptions?: unknown[]
  customerSourceOptions?: unknown[]
  [key: string]: unknown
}

const props = withDefaults(
  defineProps<{
    state: WorkspaceState
    integratedScope?: boolean
    integratedRoster?: ConsultantRosterItem[]
    integratedStaff?: IntegratedStaffItem[]
    integratedRanking?: Record<string, unknown> | null
    integratedOverview?: Record<string, unknown> | null
    integratedHistory?: Array<Record<string, unknown>>
    integratedPending?: boolean
    integratedError?: string
  }>(),
  {
    integratedScope: false,
    integratedRoster: () => [],
    integratedStaff: () => [],
    integratedRanking: null,
    integratedOverview: null,
    integratedHistory: () => [],
    integratedPending: false,
    integratedError: '',
  },
)

const consultantsStore = useConsultantsStore()
const auth = useAuthStore()
const feedbackOpen = ref(false)
const feedbackTarget = ref<PerformanceFeedbackTarget | null>(null)

const canOpenFeedback = computed(() =>
  canViewPerformanceFeedback(
    auth.role,
    auth.effectivePermissionKeys,
    auth.effectivePermissionsResolved,
  ),
)

const roster = computed(() => props.state.roster || [])
const selectedConsultant = computed(
  () =>
    roster.value.find((consultant) => consultant.id === props.state.selectedConsultantId) ||
    roster.value[0] ||
    null,
)
const feedbackSettingsStoreId = computed(() =>
  String(selectedConsultant.value?.storeId || auth.activeStoreId || '').trim(),
)
const stats = computed(() => {
  if (!selectedConsultant.value) {
    return null
  }

  return buildConsultantStats({
    history: props.state.serviceHistory || [],
    consultantId: selectedConsultant.value.id,
    monthlyGoal: Number(selectedConsultant.value.monthlyGoal || 0),
    commissionRate: Number(selectedConsultant.value.commissionRate || 0),
    conversionGoal: Number(selectedConsultant.value.conversionGoal || 0),
    avgTicketGoal: Number(selectedConsultant.value.avgTicketGoal || 0),
    paGoal: Number(selectedConsultant.value.paGoal || 0),
  })
})

const storeConversionAvg = computed(() => {
  const history = props.state.serviceHistory || []
  const rosterIds = new Set(roster.value.map((c) => c.id))
  const inStore = history.filter((entry) => rosterIds.has(String(entry.personId || '')))
  if (!inStore.length) return null

  const converted = inStore.filter(
    (entry) => entry.finishOutcome === 'compra' || entry.finishOutcome === 'reserva',
  )
  return (converted.length / inStore.length) * 100
})

function selectConsultant(consultantId: string) {
  void consultantsStore.setSelectedConsultant(consultantId)
}

function updateSimulation(amount: number) {
  void consultantsStore.setConsultantSimulationAdditionalSales(amount)
}

function openFeedback(consultantId: string) {
  const consultant = roster.value.find((item) => item.id === consultantId)
  const storeId = String(consultant?.storeId || auth.activeStoreId || '').trim()
  if (!consultant || !storeId || !stats.value) return
  feedbackTarget.value = {
    storeId,
    storeName: String(consultant.storeName || '').trim(),
    consultantId: consultant.id,
    consultantName: consultant.name,
    metrics: performanceFeedbackMetricsFromCard({
      soldValue: stats.value.soldValue,
      attendances: stats.value.attendances,
      conversions: stats.value.conversions,
      nonConversions: stats.value.nonConversions,
      conversionRate: stats.value.conversionRate,
      ticketAverage: stats.value.ticketAverage,
      paScore: stats.value.paScore,
      qualityScore: 0,
      avgDurationMs: stats.value.averageDurationMs,
      nonClientConversions: stats.value.nonClientConversions,
      queueJumpServices: stats.value.queueJumpServices,
      salesGoal: stats.value.monthlyGoal,
      ticketGoal: stats.value.avgTicketGoal,
      conversionGoal: stats.value.conversionGoal,
      paGoal: stats.value.paGoal,
    }),
  }
  feedbackOpen.value = true
}
</script>

<template>
  <ConsultantIntegratedWorkspace
    v-if="integratedScope"
    :roster="integratedRoster"
    :staff="integratedStaff"
    :ranking="integratedRanking"
    :overview="integratedOverview"
    :history="integratedHistory"
    :pending="integratedPending"
    :error-message="integratedError"
  />

  <section v-else class="admin-panel" data-testid="consultant-panel">
    <header class="admin-panel__header consultant-workspace__header">
      <div>
        <h2 class="admin-panel__title">Perfil do consultor</h2>
        <p class="admin-panel__text">Meta mensal, desempenho e simulação de venda.</p>
      </div>
      <PerformanceFeedbackSettingsButton :store-id="feedbackSettingsStoreId" />
    </header>

    <template v-if="selectedConsultant && stats">
      <ConsultantSelector
        :roster="roster"
        :selected-consultant-id="selectedConsultant.id"
        @select="selectConsultant"
      />

      <ConsultantPlayerCard
        :consultant="selectedConsultant"
        :stats="stats"
        :store-conversion-avg="storeConversionAvg"
        mode="full"
        :show-details-button="false"
        :show-feedback-button="canOpenFeedback"
        @open-feedback="openFeedback"
      />

      <div class="consultant-workspace__insights">
        <ConsultantHistoryPanel
          :consultant-id="selectedConsultant.id"
          :store-id="selectedConsultant.storeId"
          :entries="state.serviceHistory || []"
        />

        <section class="consultant-workspace__insight-panel">
          <ConsultantSimulator
            :sold-value="stats.soldValue"
            :monthly-goal="stats.monthlyGoal"
            :commission-rate="stats.commissionRate"
            :simulation-additional-sales="Number(state.consultantSimulationAdditionalSales || 0)"
            @update:simulation-additional-sales="updateSimulation"
          />
        </section>
      </div>

      <ConsultantRecentAttendancesTable
        :consultant-id="selectedConsultant.id"
        :consultant-name="selectedConsultant.name"
        :store-id="selectedConsultant.storeId"
        :store-name="selectedConsultant.storeName"
        :entries="state.serviceHistory || []"
        :visit-reason-options="state.visitReasonOptions || []"
        :customer-source-options="state.customerSourceOptions || []"
      />
    </template>

    <div v-else class="admin-panel__empty">Nenhum consultor disponível para exibir.</div>
  </section>

  <PerformanceFeedbackModal v-model:open="feedbackOpen" :target="feedbackTarget" />
</template>

<style scoped>
.consultant-workspace__header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 1rem;
}

.consultant-workspace__insights {
  display: grid;
  grid-template-columns: minmax(0, 1.3fr) minmax(0, 1fr);
  gap: 0.85rem;
  align-items: start;
}

.consultant-workspace__insight-panel {
  padding: 1rem;
  border-radius: 1rem;
  border: 1px solid rgb(var(--primary) / 0.16);
  background: rgb(var(--surface) / 0.78);
  box-shadow: var(--shadow-xs);
}

@media (max-width: 1100px) {
  .consultant-workspace__insights {
    grid-template-columns: minmax(0, 1fr);
  }
}

@media (max-width: 720px) {
  .consultant-workspace__header {
    align-items: flex-start;
    flex-direction: column;
  }
}
</style>
