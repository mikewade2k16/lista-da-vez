<script setup lang="ts">
import { computed } from 'vue'
import type { ApexOptions } from 'apexcharts'

import CardapioAnalyticsCard from './CardapioAnalyticsCard.vue'
import CardapioAnalyticsChart from './CardapioAnalyticsChart.vue'
import type { AnalyticsTimeseries } from '~/domain/cardapio/analytics'

// Bloco Horarios — mapeia `timeseries?granularity=hour_of_day|weekday_hour`.
// Toggle: barras por hora-do-dia (24 pts) OU heatmap dia-da-semana x hora (7x24).
// O modo e controlado pelo pai (v-model:granularity) para o composable re-buscar
// a granularidade certa. As cores derivam de --primary (via o base do Chart).

const props = defineProps<{
  data: AnalyticsTimeseries | null
  granularity: 'hour_of_day' | 'weekday_hour'
  loading?: boolean
  error?: string
}>()

const emit = defineEmits<{
  (e: 'retry'): void
  (e: 'update:granularity', value: 'hour_of_day' | 'weekday_hour'): void
}>()

const WEEKDAYS = ['Dom', 'Seg', 'Ter', 'Qua', 'Qui', 'Sex', 'Sab']

const points = computed(() => props.data?.points ?? [])
const isEmpty = computed(() => points.value.every((point) => point.visits === 0))

const hourLabels = computed(() => Array.from({ length: 24 }, (_, hour) => `${hour}h`))

// Barras: visitas por hora-do-dia (0..23). A serie densa ja vem 0..23 do back.
const barSeries = computed<ApexOptions['series']>(() => [
  { name: 'Visitas', data: points.value.map((point) => point.visits) },
])

// Heatmap: uma serie por dia-da-semana (0=domingo), cada uma com 24 celulas (hora).
const heatmapSeries = computed<ApexOptions['series']>(() => {
  const grid: number[][] = WEEKDAYS.map(() => Array.from({ length: 24 }, () => 0))
  for (const point of points.value) {
    const weekday = Number(point.weekday ?? -1)
    const hour = Number(point.hour ?? -1)
    if (weekday >= 0 && weekday < 7 && hour >= 0 && hour < 24) {
      grid[weekday][hour] = point.visits
    }
  }
  // Reverte para o domingo ficar no topo do heatmap (Apex desenha de baixo p/ cima).
  return WEEKDAYS.map((label, index) => ({
    name: label,
    data: grid[index].map((value, hour) => ({ x: `${hour}h`, y: value })),
  })).reverse()
})

const isHeatmap = computed(() => props.granularity === 'weekday_hour')

const chartType = computed<'bar' | 'heatmap'>(() => (isHeatmap.value ? 'heatmap' : 'bar'))

const series = computed<ApexOptions['series']>(() =>
  isHeatmap.value ? heatmapSeries.value : barSeries.value,
)

const chartOptions = computed<ApexOptions>(() => {
  if (isHeatmap.value) {
    return {
      legend: { show: false },
      plotOptions: { heatmap: { radius: 3, enableShades: true, shadeIntensity: 0.6 } },
      xaxis: { type: 'category' },
    }
  }
  return {
    categories: hourLabels.value,
    plotOptions: { bar: { borderRadius: 3, columnWidth: '70%' } },
    legend: { show: false },
  }
})
</script>

<template>
  <CardapioAnalyticsCard
    title="Horarios de pico"
    subtitle="Visitas por hora do dia"
    :loading="loading"
    :error="error"
    :empty="!loading && !error && isEmpty"
    @retry="emit('retry')"
  >
    <template #actions>
      <div class="cardapio-analytics-hours__toggle" role="group" aria-label="Visualizacao">
        <button
          type="button"
          class="cardapio-analytics-hours__toggle-btn"
          :class="{ 'cardapio-analytics-hours__toggle-btn--active': granularity === 'hour_of_day' }"
          @click="emit('update:granularity', 'hour_of_day')"
        >
          Por hora
        </button>
        <button
          type="button"
          class="cardapio-analytics-hours__toggle-btn"
          :class="{
            'cardapio-analytics-hours__toggle-btn--active': granularity === 'weekday_hour',
          }"
          @click="emit('update:granularity', 'weekday_hour')"
        >
          Dia x hora
        </button>
      </div>
    </template>

    <CardapioAnalyticsChart
      :type="chartType"
      :series="series"
      :options="chartOptions"
      :height="isHeatmap ? 320 : 280"
    />
  </CardapioAnalyticsCard>
</template>

<style scoped>
.cardapio-analytics-hours__toggle {
  display: inline-flex;
  gap: 0.25rem;
  padding: 0.2rem;
  border-radius: var(--radius-sm);
  background: rgb(var(--surface-2) / 0.6);
}

.cardapio-analytics-hours__toggle-btn {
  padding: 0.35rem 0.7rem;
  border: none;
  border-radius: var(--radius-xs);
  background: transparent;
  color: var(--text-muted);
  font-size: 0.78rem;
  font-weight: 600;
  cursor: pointer;
  transition:
    background 0.13s ease,
    color 0.13s ease;
}

.cardapio-analytics-hours__toggle-btn--active {
  background: rgb(var(--primary) / 0.18);
  color: rgb(var(--primary));
}
</style>
