<script setup>
import { computed, ref } from "vue";
import { formatCurrencyBRL, formatDurationMinutes, formatPercent } from "~/domain/utils/admin-metrics";

const props = defineProps({
  title: {
    type: String,
    required: true
  },
  rows: {
    type: Array,
    default: () => []
  },
  testid: {
    type: String,
    default: ""
  }
});

const sortBy = ref("soldValue");

const sortOptions = [
  { key: "soldValue", label: "Valor" },
  { key: "conversionRate", label: "Conversao" },
  { key: "ticketAverage", label: "Ticket" },
  { key: "paScore", label: "P.A." },
  { key: "qualityScore", label: "Qualidade" },
  { key: "score360", label: "360" },
  { key: "queueJumpServices", label: "Fora da vez" }
];

const rowsWith360 = computed(() => {
  const rows = props.rows;
  const maxSold = Math.max(...rows.map((r) => r.soldValue), 1);
  const maxPa = Math.max(...rows.map((r) => r.paScore), 0.01);

  return rows.map((row) => ({
    ...row,
    score360:
      (row.conversionRate / 100) * 35 +
      (row.soldValue / maxSold) * 25 +
      (row.qualityScore / 100) * 20 +
      (row.paScore / maxPa) * 15 +
      (1 - Math.min(1, row.queueJumpServices / Math.max(row.attendances, 1))) * 5
  }));
});

const sortedRows = computed(() => {
  const key = sortBy.value;
  return [...rowsWith360.value].sort((a, b) => {
    if (key === "queueJumpServices") return a[key] - b[key];
    return b[key] - a[key];
  });
});
</script>

<template>
  <article class="ranking-card" :data-testid="testid || undefined">
    <header class="ranking-card__header">
      <h3 class="ranking-card__title">{{ title }}</h3>
      <div class="ranking-sort">
        <button
          v-for="opt in sortOptions"
          :key="opt.key"
          class="ranking-sort__btn"
          :class="{ 'is-active': sortBy === opt.key }"
          :data-testid="testid ? `${testid}-sort-${opt.key}` : undefined"
          type="button"
          @click="sortBy = opt.key"
        >{{ opt.label }}</button>
      </div>
    </header>
    <div class="ranking-card__table-wrap">
      <table class="ranking-table">
        <thead>
          <tr>
            <th>#</th>
            <th>Consultor</th>
            <th>Vendas</th>
            <th>Conv.</th>
            <th>Taxa</th>
            <th>Ticket</th>
            <th>P.A.</th>
            <th>Qualidade</th>
            <th>Tempo</th>
            <th>Fora da vez</th>
            <th>Score 360</th>
          </tr>
        </thead>
        <tbody>
          <tr v-if="!sortedRows.length">
            <td colspan="11">Sem dados no periodo.</td>
          </tr>
          <tr v-for="(row, index) in sortedRows" :key="`${row.consultantId}-${row.storeId || 'store'}`">
            <td>{{ index + 1 }}</td>
            <td>
              <div class="ranking-table__consultant">
                <span>{{ row.consultantName }}</span>
                <small v-if="row.storeName" class="ranking-table__store">{{ row.storeName }}</small>
              </div>
            </td>
            <td>{{ formatCurrencyBRL(row.soldValue) }}</td>
            <td>{{ row.conversions }}/{{ row.attendances }}</td>
            <td>{{ formatPercent(row.conversionRate) }}</td>
            <td>{{ formatCurrencyBRL(row.ticketAverage) }}</td>
            <td>{{ row.paScore.toFixed(2) }}</td>
            <td>{{ formatPercent(row.qualityScore) }}</td>
            <td>{{ formatDurationMinutes(row.avgDurationMs) }}</td>
            <td>{{ row.queueJumpServices }}</td>
            <td><strong>{{ row.score360.toFixed(1) }}</strong></td>
          </tr>
        </tbody>
      </table>
    </div>
  </article>
</template>

<style scoped>
.ranking-table__consultant {
  display: grid;
  gap: 2px;
}

.ranking-table__store {
  color: rgba(148, 163, 184, 0.88);
  font-size: 0.68rem;
  font-weight: 600;
}
</style>
