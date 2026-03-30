<script setup>
import { computed } from "vue";
import { buildConsultantAlerts, buildRankingRows, formatCurrencyBRL, formatPercent } from "@core/utils/admin-metrics";
import RankingTable from "~/components/ranking/RankingTable.vue";

const props = defineProps({
  state: {
    type: Object,
    required: true
  }
});

const monthlyRows = computed(() =>
  buildRankingRows({
    history: props.state.serviceHistory || [],
    roster: props.state.roster || [],
    scope: "month"
  })
);
const dailyRows = computed(() =>
  buildRankingRows({
    history: props.state.serviceHistory || [],
    roster: props.state.roster || [],
    scope: "today"
  })
);
const alerts = computed(() =>
  buildConsultantAlerts({
    roster: props.state.roster || [],
    history: props.state.serviceHistory || [],
    settings: props.state.settings || {}
  })
);

const ALERT_LABELS = {
  conversion: (a) => `Conversão ${a.value.toFixed(1)}% — abaixo do mínimo de ${a.threshold}%`,
  queueJump: (a) => `Fora da vez ${a.value.toFixed(1)}% — acima do máximo de ${a.threshold}%`,
  pa: (a) => `P.A. ${a.value.toFixed(2)} — abaixo do mínimo de ${a.threshold}`,
  ticket: (a) => `Ticket ${formatCurrencyBRL(a.value)} — abaixo do mínimo de ${formatCurrencyBRL(a.threshold)}`
};
</script>

<template>
  <section class="admin-panel" data-testid="ranking-panel">
    <header class="admin-panel__header">
      <h2 class="admin-panel__title">Ranking de vendedores</h2>
      <p class="admin-panel__text">Comparativo mensal e diario para acompanhar consistencia e bonificacao.</p>
    </header>

    <div v-if="alerts.length" class="alert-list" data-testid="ranking-alerts">
      <div class="alert-list__header">
        <span class="alert-list__title">Alertas de desempenho — {{ alerts.length }} ocorrência{{ alerts.length > 1 ? 's' : '' }}</span>
        <span class="metric-card__text">Configure os limites em Configuracoes &gt; Alertas</span>
      </div>
      <div v-for="(alert, i) in alerts" :key="i" class="alert-item">
        <span class="alert-item__name">{{ alert.consultantName }}</span>
        <span class="alert-item__msg">{{ ALERT_LABELS[alert.type]?.(alert) || alert.type }}</span>
      </div>
    </div>

    <div class="ranking-grid">
      <RankingTable title="Ranking do mes" :rows="monthlyRows" testid="ranking-monthly" />
      <RankingTable title="Ranking de hoje" :rows="dailyRows" testid="ranking-daily" />
    </div>
  </section>
</template>
