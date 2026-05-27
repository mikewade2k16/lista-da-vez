<script setup>
import { computed } from 'vue'
import ConsultantPlayerCard from '~/components/consultant/ConsultantPlayerCard.vue'

const props = defineProps({
  rows: {
    type: Array,
    default: () => [],
  },
  storeConversionAvgByStoreId: {
    type: Object,
    default: () => ({}),
  },
  rankingPositionByKey: {
    type: Object,
    default: () => ({}),
  },
})

const emit = defineEmits(['open-details'])

const enrichedRows = computed(() =>
  props.rows.map((row) => ({
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
      ticketAverage: row.ticketAverage,
      paScore: row.paScore,
      erpOrders: row.erpOrders,
      soldValueSource: row.soldValueSource,
      ticketAverageSource: row.ticketAverageSource,
      paScoreSource: row.paScoreSource,
      conversionRate: row.conversionRate,
      averageDurationMs: row.avgDurationMs,
      avgTicketGoal: row.avgTicketGoal,
      paGoal: row.paGoal,
    },
    storeConversionAvg: props.storeConversionAvgByStoreId[row.storeId] ?? null,
    rankingPosition: props.rankingPositionByKey[`${row.storeId}:${row.id}`] ?? null,
    key: `${row.storeId}:${row.id}`,
  })),
)

function handleOpen(consultantId) {
  emit('open-details', consultantId)
}
</script>

<template>
  <div v-if="enrichedRows.length" class="player-grid" data-testid="player-grid">
    <ConsultantPlayerCard
      v-for="row in enrichedRows"
      :key="row.key"
      :consultant="row.consultant"
      :stats="row.stats"
      :store-conversion-avg="row.storeConversionAvg"
      :ranking-position="row.rankingPosition"
      mode="mini"
      :show-details-button="false"
      @open-details="handleOpen"
    />
  </div>
  <div v-else class="player-grid__empty" data-testid="player-grid-empty">
    Nenhum consultor encontrado para os filtros selecionados.
  </div>
</template>

<style scoped>
.player-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(20rem, 1fr));
  gap: 0.85rem;
}

.player-grid__empty {
  padding: 2rem;
  text-align: center;
  color: rgb(var(--muted) / 0.92);
}
</style>
