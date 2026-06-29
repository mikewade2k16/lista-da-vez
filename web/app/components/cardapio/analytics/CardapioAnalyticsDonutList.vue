<script setup lang="ts">
import { computed } from 'vue'
import type { ApexOptions } from 'apexcharts'

import CardapioAnalyticsChart from './CardapioAnalyticsChart.vue'
import { formatAnalyticsInt } from '~/domain/cardapio/analytics'

// Donut + lista reutilizavel (Traffic e Devices). Recebe pares {label,value} ja
// resolvidos (origem "(direto)" e tratada por quem chama, antes de passar). Sem
// dado => "Sem dados neste periodo." (vazio honesto). NAO faz fetch.

interface DonutSlice {
  label: string
  value: number
}

const props = withDefaults(
  defineProps<{
    title?: string
    slices: DonutSlice[]
    height?: number
  }>(),
  {
    title: '',
    height: 220,
  },
)

const isEmpty = computed(
  () => props.slices.length === 0 || props.slices.every((slice) => slice.value === 0),
)

const total = computed(() => props.slices.reduce((sum, slice) => sum + slice.value, 0))

const labels = computed(() => props.slices.map((slice) => slice.label))
const series = computed<ApexOptions['series']>(() => props.slices.map((slice) => slice.value))

// Donut tema-aware: legenda fora (lista propria abaixo), sem data labels poluindo.
const chartOptions = computed<ApexOptions>(() => ({
  legend: { show: false },
  dataLabels: { enabled: false },
  stroke: { width: 0 },
  plotOptions: { pie: { donut: { size: '64%' } } },
}))

function share(value: number): string {
  if (total.value <= 0) {
    return '0%'
  }
  return `${Math.round((value / total.value) * 100)}%`
}
</script>

<template>
  <div class="cardapio-analytics-donut">
    <h4 v-if="title" class="cardapio-analytics-donut__title">{{ title }}</h4>

    <p v-if="isEmpty" class="cardapio-analytics-donut__empty">Sem dados neste periodo.</p>

    <template v-else>
      <CardapioAnalyticsChart type="donut" :series="series" :labels="labels" :height="height" />
      <ul class="cardapio-analytics-donut__list">
        <li v-for="slice in slices" :key="slice.label" class="cardapio-analytics-donut__item">
          <span class="cardapio-analytics-donut__label">{{ slice.label }}</span>
          <span class="cardapio-analytics-donut__value">
            {{ formatAnalyticsInt(slice.value) }}
            <span class="cardapio-analytics-donut__share">{{ share(slice.value) }}</span>
          </span>
        </li>
      </ul>
    </template>
  </div>
</template>

<style scoped>
.cardapio-analytics-donut {
  display: flex;
  flex-direction: column;
  gap: 0.6rem;
  min-width: 0;
}

.cardapio-analytics-donut__title {
  font-size: 0.78rem;
  font-weight: 700;
  letter-spacing: 0.04em;
  text-transform: uppercase;
  color: var(--text-muted);
}

.cardapio-analytics-donut__empty {
  font-size: 0.82rem;
  color: var(--text-muted);
  padding: 0.85rem;
  text-align: center;
  border: 1px dashed var(--line-soft);
  border-radius: var(--radius-soft);
  background: rgb(var(--surface-2) / 0.4);
}

.cardapio-analytics-donut__list {
  display: flex;
  flex-direction: column;
  gap: 0.3rem;
  list-style: none;
  margin: 0;
  padding: 0;
}

.cardapio-analytics-donut__item {
  display: flex;
  align-items: baseline;
  justify-content: space-between;
  gap: 0.75rem;
  font-size: 0.82rem;
}

.cardapio-analytics-donut__label {
  color: var(--text-main);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  min-width: 0;
}

.cardapio-analytics-donut__value {
  font-weight: 700;
  color: var(--text-main);
  white-space: nowrap;
}

.cardapio-analytics-donut__share {
  font-weight: 500;
  color: var(--text-muted);
}
</style>
