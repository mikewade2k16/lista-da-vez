<script setup>
import { computed } from "vue";
import { formatCurrencyBRL } from "~/domain/utils/admin-metrics";
import RankingTable from "~/components/ranking/RankingTable.vue";

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
  },
  integratedScope: {
    type: Boolean,
    default: false
  }
});

const monthlyRows = computed(() => props.report?.monthlyRows || []);
const dailyRows = computed(() => props.report?.dailyRows || []);
const alerts = computed(() => props.report?.alerts || []);

const ALERT_LABELS = {
  conversion: (a) => `Conversao ${a.value.toFixed(1)}% - abaixo do minimo de ${a.threshold}%`,
  queueJump: (a) => `Fora da vez ${a.value.toFixed(1)}% - acima do maximo de ${a.threshold}%`,
  pa: (a) => `P.A. ${a.value.toFixed(2)} - abaixo do minimo de ${a.threshold}`,
  ticket: (a) => `Ticket ${formatCurrencyBRL(a.value)} - abaixo do minimo de ${formatCurrencyBRL(a.threshold)}`
};
</script>

<template>
  <section class="admin-panel" data-testid="ranking-panel">
    <header class="admin-panel__header">
      <h2 class="admin-panel__title">Ranking de vendedores</h2>
      <p class="admin-panel__text">
        {{ integratedScope ? 'Comparativo mensal e diario consolidado das lojas do tenant ativo.' : 'Comparativo mensal e diario para acompanhar consistencia e bonificacao.' }}
      </p>
    </header>

    <article v-if="errorMessage" class="insight-card">
      <p class="settings-card__text">{{ errorMessage }}</p>
    </article>

    <article v-else-if="pending && !monthlyRows.length && !dailyRows.length" class="insight-card">
      <p class="settings-card__text">{{ integratedScope ? 'Carregando ranking consolidado...' : 'Carregando ranking da loja ativa...' }}</p>
    </article>

    <div v-if="alerts.length" class="alert-list" data-testid="ranking-alerts">
      <div class="alert-list__header">
        <span class="alert-list__title">Alertas de desempenho - {{ alerts.length }} ocorrencia{{ alerts.length > 1 ? 's' : '' }}</span>
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
