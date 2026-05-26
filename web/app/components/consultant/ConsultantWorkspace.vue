<script setup>
import { computed } from 'vue'
import { buildConsultantStats } from '~/domain/utils/admin-metrics'
import ConsultantHistoryPanel from '~/components/consultant/ConsultantHistoryPanel.vue'
import ConsultantIntegratedWorkspace from '~/components/consultant/ConsultantIntegratedWorkspace.vue'
import ConsultantPlayerCard from '~/components/consultant/ConsultantPlayerCard.vue'
import ConsultantRecentAttendancesTable from '~/components/consultant/ConsultantRecentAttendancesTable.vue'
import ConsultantSelector from '~/components/consultant/ConsultantSelector.vue'
import ConsultantSimulator from '~/components/consultant/ConsultantSimulator.vue'
import { useConsultantsStore } from '~/stores/consultants'

const props = defineProps({
  state: {
    type: Object,
    required: true,
  },
  integratedScope: {
    type: Boolean,
    default: false,
  },
  integratedRoster: {
    type: Array,
    default: () => [],
  },
  integratedRanking: {
    type: Object,
    default: null,
  },
  integratedOverview: {
    type: Object,
    default: null,
  },
  integratedHistory: {
    type: Array,
    default: () => [],
  },
  integratedPending: {
    type: Boolean,
    default: false,
  },
  integratedError: {
    type: String,
    default: '',
  },
})

const consultantsStore = useConsultantsStore()

const roster = computed(() => props.state.roster || [])
const selectedConsultant = computed(
  () =>
    roster.value.find((consultant) => consultant.id === props.state.selectedConsultantId) ||
    roster.value[0] ||
    null,
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
  const inStore = history.filter((entry) => rosterIds.has(entry.personId))
  if (!inStore.length) return null

  const converted = inStore.filter(
    (entry) => entry.finishOutcome === 'compra' || entry.finishOutcome === 'reserva',
  )
  return (converted.length / inStore.length) * 100
})

function selectConsultant(consultantId) {
  void consultantsStore.setSelectedConsultant(consultantId)
}

function updateSimulation(amount) {
  void consultantsStore.setConsultantSimulationAdditionalSales(amount)
}
</script>

<template>
  <ConsultantIntegratedWorkspace
    v-if="integratedScope"
    :roster="integratedRoster"
    :ranking="integratedRanking"
    :overview="integratedOverview"
    :history="integratedHistory"
    :pending="integratedPending"
    :error-message="integratedError"
  />

  <section v-else class="admin-panel" data-testid="consultant-panel">
    <header class="admin-panel__header">
      <h2 class="admin-panel__title">Perfil do consultor</h2>
      <p class="admin-panel__text">Meta mensal, desempenho e simulação de venda.</p>
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
</template>

<style scoped>
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
</style>
