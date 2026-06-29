<script setup lang="ts">
import { computed } from 'vue'
import type { ApexOptions } from 'apexcharts'

import CardapioAnalyticsCard from './CardapioAnalyticsCard.vue'
import CardapioAnalyticsChart from './CardapioAnalyticsChart.vue'
import {
  formatAnalyticsInt,
  type AnalyticsSourceDimension,
  type AnalyticsSources,
} from '~/domain/cardapio/analytics'

// Bloco Origem do trafego — mapeia `sources?dimension=`. Donut (sessoes por
// origem) + lista (sessoes e pedidos por origem). value vazio => "(direto)". O
// seletor de dimensao e controlado pelo pai (v-model:dimension) para re-buscar.

const props = defineProps<{
  data: AnalyticsSources | null
  dimension: AnalyticsSourceDimension
  loading?: boolean
  error?: string
}>()

const emit = defineEmits<{
  (e: 'retry'): void
  (e: 'update:dimension', value: AnalyticsSourceDimension): void
}>()

const DIMENSION_OPTIONS: Array<{ value: AnalyticsSourceDimension; label: string }> = [
  { value: 'utm_source', label: 'Fonte (utm)' },
  { value: 'utm_medium', label: 'Midia (utm)' },
  { value: 'utm_campaign', label: 'Campanha (utm)' },
  { value: 'referrer', label: 'Site de origem' },
]

// value vazio = trafego direto (sem origem rastreada).
function originLabel(value: string): string {
  return value ? value : '(direto)'
}

const items = computed(() =>
  (props.data?.items ?? []).map((item) => ({
    label: originLabel(item.value),
    sessions: item.sessions,
    orders: item.orders,
  })),
)

const isEmpty = computed(
  () => items.value.length === 0 || items.value.every((item) => item.sessions === 0),
)

const labels = computed(() => items.value.map((item) => item.label))
const series = computed<ApexOptions['series']>(() => items.value.map((item) => item.sessions))

const chartOptions = computed<ApexOptions>(() => ({
  legend: { show: false },
  dataLabels: { enabled: false },
  stroke: { width: 0 },
  plotOptions: { pie: { donut: { size: '64%' } } },
}))

function onDimensionChange(event: Event) {
  emit('update:dimension', (event.target as HTMLSelectElement).value as AnalyticsSourceDimension)
}
</script>

<template>
  <CardapioAnalyticsCard
    title="Origem do trafego"
    subtitle="Sessoes e pedidos por origem"
    :loading="loading"
    :error="error"
    :empty="!loading && !error && isEmpty"
    @retry="emit('retry')"
  >
    <template #actions>
      <label class="cardapio-analytics-traffic__dim">
        <span class="cardapio-analytics-traffic__dim-label">Dimensao</span>
        <select
          class="cardapio-analytics-traffic__select"
          :value="dimension"
          @change="onDimensionChange"
        >
          <option v-for="option in DIMENSION_OPTIONS" :key="option.value" :value="option.value">
            {{ option.label }}
          </option>
        </select>
      </label>
    </template>

    <div class="cardapio-analytics-traffic__grid">
      <CardapioAnalyticsChart
        type="donut"
        :series="series"
        :labels="labels"
        :options="chartOptions"
        :height="220"
      />

      <ul class="cardapio-analytics-traffic__list">
        <li v-for="item in items" :key="item.label" class="cardapio-analytics-traffic__item">
          <span class="cardapio-analytics-traffic__origin">{{ item.label }}</span>
          <span class="cardapio-analytics-traffic__metrics">
            <span>{{ formatAnalyticsInt(item.sessions) }} sessoes</span>
            <span class="cardapio-analytics-traffic__orders">
              {{ formatAnalyticsInt(item.orders) }} pedidos
            </span>
          </span>
        </li>
      </ul>
    </div>
  </CardapioAnalyticsCard>
</template>

<style scoped>
.cardapio-analytics-traffic__dim {
  display: inline-flex;
  align-items: center;
  gap: 0.45rem;
}

.cardapio-analytics-traffic__dim-label {
  font-size: 0.74rem;
  font-weight: 600;
  color: var(--text-muted);
}

.cardapio-analytics-traffic__select {
  min-height: 2rem;
  padding: 0 0.5rem;
  border: 1px solid var(--line-soft);
  border-radius: var(--radius-sm);
  background: rgb(var(--surface) / 0.9);
  color: var(--text-main);
  font-size: 0.82rem;
  cursor: pointer;
}

.cardapio-analytics-traffic__grid {
  display: grid;
  grid-template-columns: minmax(0, 0.85fr) minmax(0, 1.15fr);
  gap: 1.1rem;
  align-items: center;
}

.cardapio-analytics-traffic__list {
  display: flex;
  flex-direction: column;
  gap: 0.4rem;
  list-style: none;
  margin: 0;
  padding: 0;
  min-width: 0;
}

.cardapio-analytics-traffic__item {
  display: flex;
  align-items: baseline;
  justify-content: space-between;
  gap: 0.75rem;
  font-size: 0.82rem;
}

.cardapio-analytics-traffic__origin {
  color: var(--text-main);
  font-weight: 600;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  min-width: 0;
}

.cardapio-analytics-traffic__metrics {
  display: inline-flex;
  gap: 0.65rem;
  color: var(--text-main);
  white-space: nowrap;
}

.cardapio-analytics-traffic__orders {
  color: var(--text-muted);
}

@media (max-width: 720px) {
  .cardapio-analytics-traffic__grid {
    grid-template-columns: 1fr;
  }
}
</style>
