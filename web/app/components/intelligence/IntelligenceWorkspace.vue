<script setup>
import { computed } from "vue";
import {
  formatCurrencyBRL,
  formatDurationMinutes,
  formatPercent
} from "~/domain/utils/admin-metrics";
import IntelligenceDiagnosisCard from "~/components/intelligence/IntelligenceDiagnosisCard.vue";

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

const intelligence = computed(() => props.report || {
  healthScore: 0,
  severityCounts: {
    critical: 0,
    attention: 0,
    healthy: 0
  },
  totalAttendances: 0,
  diagnosis: [],
  recommendedActions: [],
  time: {
    avgQueueWaitMs: 0,
    notUsingQueueRate: 0
  },
  ticketAverage: 0,
  conversionRate: 0
});
const summaryLevelClass = computed(() => {
  if (intelligence.value.healthScore >= 80) {
    return "healthy";
  }

  if (intelligence.value.healthScore >= 60) {
    return "attention";
  }

  return "critical";
});
const contextRows = computed(() => [
  {
    label: "Tempo medio de espera na fila",
    value: formatDurationMinutes(intelligence.value.time.avgQueueWaitMs)
  },
  {
    label: "Taxa de atendimento fora da vez",
    value: formatPercent(intelligence.value.time.notUsingQueueRate)
  },
  {
    label: "Ticket medio (compra/reserva)",
    value: formatCurrencyBRL(intelligence.value.ticketAverage)
  },
  {
    label: "Conversao geral",
    value: formatPercent(intelligence.value.conversionRate)
  }
]);
const roundedScore = computed(() => Math.round(intelligence.value.healthScore));
</script>

<template>
  <section class="admin-panel" data-testid="intelligence-panel">
    <header class="admin-panel__header">
      <h2 class="admin-panel__title">Inteligencia operacional</h2>
      <p class="admin-panel__text">Leitura automatica dos dados para apoiar decisao de loja e gestao de equipe.</p>
    </header>

    <article v-if="errorMessage" class="insight-card">
      <p class="settings-card__text">{{ errorMessage }}</p>
    </article>

    <article v-else-if="pending && !report" class="insight-card">
      <p class="settings-card__text">Carregando inteligencia operacional da loja ativa...</p>
    </article>

    <article class="insight-card intel-summary" data-testid="intelligence-score">
      <div :class="`intel-summary__score intel-summary__score--${summaryLevelClass}`">
        <span class="intel-summary__label">Score operacional</span>
        <strong class="intel-summary__value">{{ roundedScore }}</strong>
      </div>
      <div class="intel-summary__meta">
        <span class="insight-tag">
          Criticos:
          <strong>{{ intelligence.severityCounts.critical }}</strong>
        </span>
        <span class="insight-tag">
          Atencao:
          <strong>{{ intelligence.severityCounts.attention }}</strong>
        </span>
        <span class="insight-tag">
          Saudaveis:
          <strong>{{ intelligence.severityCounts.healthy }}</strong>
        </span>
        <span class="insight-tag">
          Atendimentos analisados:
          <strong>{{ intelligence.totalAttendances }}</strong>
        </span>
      </div>
    </article>

    <div class="insight-grid" data-testid="intelligence-diagnosis">
      <IntelligenceDiagnosisCard
        v-for="item in intelligence.diagnosis"
        :key="item.id"
        :item="item"
      />
    </div>

    <div class="insight-grid" data-testid="intelligence-context">
      <article class="insight-card">
        <h3 class="insight-card__title">Acoes recomendadas agora</h3>
        <ul class="intel-list">
          <li v-if="!intelligence.recommendedActions.length" class="intel-list__item">
            Sem alerta relevante no momento.
          </li>
          <li
            v-for="action in intelligence.recommendedActions"
            v-else
            :key="action"
            class="intel-list__item"
          >
            {{ action }}
          </li>
        </ul>
      </article>

      <article class="insight-card">
        <h3 class="insight-card__title">Contexto rapido</h3>
        <div class="intel-context">
          <div v-for="row in contextRows" :key="row.label" class="intel-context__row">
            <span>{{ row.label }}</span>
            <strong>{{ row.value }}</strong>
          </div>
        </div>
      </article>
    </div>
  </section>
</template>
