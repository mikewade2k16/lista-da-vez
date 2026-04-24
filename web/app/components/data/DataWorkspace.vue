<script setup>
import { computed } from "vue";
import { formatDurationMinutes, formatPercent } from "~/domain/utils/admin-metrics";
import InsightHourlyTable from "~/components/data/InsightHourlyTable.vue";
import InsightTagList from "~/components/data/InsightTagList.vue";

const props = defineProps({
  report: {
    type: Object,
    default: null
  },
  pending: {
    type: Boolean,
    default: false
  },
  errorMessage: {
    type: String,
    default: ""
  }
});

const timeIntelligence = computed(() => props.report?.timeIntelligence || {
  quickHighPotentialCount: 0,
  longLowSaleCount: 0,
  longNoSaleCount: 0,
  quickNoSaleCount: 0,
  avgQueueWaitMs: 0,
  totalsByStatus: {
    queue: 0,
    available: 0,
    paused: 0,
    service: 0
  },
  consultantsInQueueMs: 0,
  consultantsPausedMs: 0,
  consultantsInServiceMs: 0,
  notUsingQueueRate: 0
});

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

    <article v-if="errorMessage" class="insight-card">
      <p class="settings-card__text">{{ errorMessage }}</p>
    </article>

    <article v-else-if="pending && !report" class="insight-card">
      <p class="settings-card__text">Carregando dados operacionais da loja ativa...</p>
    </article>

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

      <InsightTagList title="Produtos mais vendidos" :items="report?.soldProducts || []" />
      <InsightTagList title="Produtos mais procurados" :items="report?.requestedProducts || []" />
      <InsightTagList title="Motivos de visita" :items="report?.visitReasons || []" />
      <InsightTagList title="Origem do cliente" :items="report?.customerSources || []" />
      <InsightTagList title="Profissoes mais atendidas" :items="report?.professions || []" />
      <InsightTagList title="Desfecho dos atendimentos" :items="report?.outcomeSummary || []" />
      <InsightHourlyTable :items="report?.hourlySales || []" />
    </div>
  </section>
</template>
