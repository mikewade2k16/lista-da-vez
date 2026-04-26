<script setup>
import { computed } from "vue";
import { buildConsultantStats } from "~/domain/utils/admin-metrics";
import ConsultantIntegratedWorkspace from "~/components/consultant/ConsultantIntegratedWorkspace.vue";
import ConsultantMetrics from "~/components/consultant/ConsultantMetrics.vue";
import ConsultantSelector from "~/components/consultant/ConsultantSelector.vue";
import ConsultantSimulator from "~/components/consultant/ConsultantSimulator.vue";
import { useConsultantsStore } from "~/stores/consultants";

const props = defineProps({
  state: {
    type: Object,
    required: true
  },
  integratedScope: {
    type: Boolean,
    default: false
  },
  integratedRoster: {
    type: Array,
    default: () => []
  },
  integratedRanking: {
    type: Object,
    default: null
  },
  integratedOverview: {
    type: Object,
    default: null
  },
  integratedPending: {
    type: Boolean,
    default: false
  },
  integratedError: {
    type: String,
    default: ""
  }
});

const consultantsStore = useConsultantsStore();

const roster = computed(() => props.state.roster || []);
const selectedConsultant = computed(() =>
  roster.value.find((consultant) => consultant.id === props.state.selectedConsultantId) || roster.value[0] || null
);
const stats = computed(() => {
  if (!selectedConsultant.value) {
    return null;
  }

  return buildConsultantStats({
    history: props.state.serviceHistory || [],
    consultantId: selectedConsultant.value.id,
    monthlyGoal: Number(selectedConsultant.value.monthlyGoal || 0),
    commissionRate: Number(selectedConsultant.value.commissionRate || 0),
    conversionGoal: Number(selectedConsultant.value.conversionGoal || 0),
    avgTicketGoal: Number(selectedConsultant.value.avgTicketGoal || 0),
    paGoal: Number(selectedConsultant.value.paGoal || 0)
  });
});
const goalPercent = computed(() => {
  if (!stats.value?.monthlyGoal) {
    return 0;
  }

  return (stats.value.soldValue / stats.value.monthlyGoal) * 100;
});

function selectConsultant(consultantId) {
  void consultantsStore.setSelectedConsultant(consultantId);
}

function updateSimulation(amount) {
  void consultantsStore.setConsultantSimulationAdditionalSales(amount);
}
</script>

<template>
  <ConsultantIntegratedWorkspace
    v-if="integratedScope"
    :roster="integratedRoster"
    :ranking="integratedRanking"
    :overview="integratedOverview"
    :pending="integratedPending"
    :error-message="integratedError"
  />

  <section v-else class="admin-panel" data-testid="consultant-panel">
    <header class="admin-panel__header">
      <h2 class="admin-panel__title">Perfil do consultor</h2>
      <p class="admin-panel__text">Meta mensal, desempenho e simulacao de venda.</p>
    </header>

    <template v-if="selectedConsultant && stats">
      <ConsultantSelector
        :roster="roster"
        :selected-consultant-id="selectedConsultant.id"
        @select="selectConsultant"
      />

      <ConsultantMetrics :stats="stats" :goal-percent="goalPercent" />

      <ConsultantSimulator
        :sold-value="stats.soldValue"
        :monthly-goal="stats.monthlyGoal"
        :commission-rate="stats.commissionRate"
        :simulation-additional-sales="Number(state.consultantSimulationAdditionalSales || 0)"
        @update:simulation-additional-sales="updateSimulation"
      />
    </template>

    <div v-else class="admin-panel__empty">
      Nenhum consultor disponivel para exibir.
    </div>
  </section>
</template>
