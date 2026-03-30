<script setup>
import { computed } from "vue";
import {
  buildInsights,
  buildTimeIntelligence,
  formatDurationMinutes,
  formatPercent
} from "@core/utils/admin-metrics";
import InsightHourlyTable from "~/components/data/InsightHourlyTable.vue";
import InsightTagList from "~/components/data/InsightTagList.vue";

const props = defineProps({
  state: {
    type: Object,
    required: true
  }
});

const insights = computed(() =>
  buildInsights({
    history: props.state.serviceHistory || [],
    visitReasonOptions: props.state.visitReasonOptions || [],
    customerSourceOptions: props.state.customerSourceOptions || []
  })
);
const timeIntelligence = computed(() =>
  buildTimeIntelligence({
    history: props.state.serviceHistory || [],
    roster: props.state.roster || [],
    waitingList: props.state.waitingList || [],
    activeServices: props.state.activeServices || [],
    pausedEmployees: props.state.pausedEmployees || [],
    consultantCurrentStatus: props.state.consultantCurrentStatus || {},
    consultantActivitySessions: props.state.consultantActivitySessions || [],
    settings: props.state.settings || {}
  })
);
const primaryTimeTags = computed(() => [
  {
    label: "Fechou muito rapido",
    value: timeIntelligence.value.quickHighPotentialCount
  },
  {
    label: "Demorou e vendeu baixo",
    value: timeIntelligence.value.longLowSaleCount
  },
  {
    label: "Demorou e nao vendeu",
    value: timeIntelligence.value.longNoSaleCount
  },
  {
    label: "Rapido e nao vendeu",
    value: timeIntelligence.value.quickNoSaleCount
  },
  {
    label: "Espera media na fila",
    value: formatDurationMinutes(timeIntelligence.value.avgQueueWaitMs)
  },
  {
    label: "Atendimento fora da vez",
    value: formatPercent(timeIntelligence.value.notUsingQueueRate)
  }
]);
const historicalTimeTags = computed(() => [
  {
    label: "Tempo historico em fila",
    value: formatDurationMinutes(timeIntelligence.value.totalsByStatus.queue)
  },
  {
    label: "Tempo historico ocioso",
    value: formatDurationMinutes(timeIntelligence.value.totalsByStatus.available)
  },
  {
    label: "Tempo historico em pausa",
    value: formatDurationMinutes(timeIntelligence.value.totalsByStatus.paused)
  },
  {
    label: "Tempo historico atendendo",
    value: formatDurationMinutes(timeIntelligence.value.totalsByStatus.service)
  }
]);
const liveTimeTags = computed(() => [
  {
    label: "Fila atual sem atender",
    value: formatDurationMinutes(timeIntelligence.value.consultantsInQueueMs)
  },
  {
    label: "Pausa atual acumulada",
    value: formatDurationMinutes(timeIntelligence.value.consultantsPausedMs)
  },
  {
    label: "Atendimento atual acumulado",
    value: formatDurationMinutes(timeIntelligence.value.consultantsInServiceMs)
  }
]);
</script>

<template>
  <section class="admin-panel" data-testid="data-panel">
    <header class="admin-panel__header">
      <h2 class="admin-panel__title">Dados operacionais</h2>
      <p class="admin-panel__text">Painel bruto de produto, motivo, origem, horario e tempo.</p>
    </header>

    <div class="insight-grid">
      <article class="insight-card insight-card--wide" data-testid="data-time-intelligence">
        <h3 class="insight-card__title">Inteligencia de tempo</h3>
        <div class="insight-time-grid">
          <span v-for="item in primaryTimeTags" :key="item.label" class="insight-tag">
            {{ item.label }}:
            <strong>{{ item.value }}</strong>
          </span>
        </div>
        <div class="insight-time-grid">
          <span v-for="item in historicalTimeTags" :key="item.label" class="insight-tag">
            {{ item.label }}:
            <strong>{{ item.value }}</strong>
          </span>
        </div>
        <div class="insight-time-grid">
          <span v-for="item in liveTimeTags" :key="item.label" class="insight-tag">
            {{ item.label }}:
            <strong>{{ item.value }}</strong>
          </span>
        </div>
      </article>

      <InsightTagList title="Produtos mais vendidos" :items="insights.soldProducts" />
      <InsightTagList title="Produtos mais procurados" :items="insights.requestedProducts" />
      <InsightTagList title="Motivos de visita" :items="insights.visitReasons" />
      <InsightTagList title="Origem do cliente" :items="insights.customerSources" />
      <InsightTagList title="Profissoes mais atendidas" :items="insights.professions" />
      <InsightTagList title="Desfecho dos atendimentos" :items="insights.outcomeSummary" />
      <InsightHourlyTable :items="insights.hourlySales" />
    </div>
  </section>
</template>
