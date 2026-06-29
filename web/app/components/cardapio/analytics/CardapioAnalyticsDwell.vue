<script setup lang="ts">
import { computed } from 'vue'

import CardapioAnalyticsCard from './CardapioAnalyticsCard.vue'
import {
  formatAnalyticsDuration,
  formatAnalyticsInt,
  type AnalyticsDwell,
  type AnalyticsDwellDimension,
  type AnalyticsDwellItem,
} from '~/domain/cardapio/analytics'

// Bloco Tempo de permanencia (dwell) — mapeia `dwell?dimension=page|product|
// section`. Lista com barra proporcional ao maior tempo + valor mm:ss + amostras.
// Seletor de dimensao controlado pelo pai (v-model:dimension) para re-buscar.

const props = defineProps<{
  data: AnalyticsDwell | null
  dimension: AnalyticsDwellDimension
  loading?: boolean
  error?: string
}>()

const emit = defineEmits<{
  (e: 'retry'): void
  (e: 'update:dimension', value: AnalyticsDwellDimension): void
}>()

const DIMENSION_OPTIONS: Array<{ value: AnalyticsDwellDimension; label: string }> = [
  { value: 'page', label: 'Pagina' },
  { value: 'product', label: 'Produto' },
  { value: 'section', label: 'Secao' },
]

const items = computed(() => props.data?.items ?? [])
const isEmpty = computed(() => items.value.length === 0)

// Maior tempo do conjunto = referencia da barra (100%); evita divisao por zero.
const maxSeconds = computed(() =>
  items.value.reduce((max, item) => Math.max(max, item.avgDwellSeconds), 0),
)

function barWidth(item: AnalyticsDwellItem): string {
  if (maxSeconds.value <= 0) {
    return '0%'
  }
  return `${Math.max((item.avgDwellSeconds / maxSeconds.value) * 100, 4)}%`
}

function onDimensionChange(event: Event) {
  emit('update:dimension', (event.target as HTMLSelectElement).value as AnalyticsDwellDimension)
}
</script>

<template>
  <CardapioAnalyticsCard
    title="Tempo de permanencia"
    subtitle="Tempo medio por pagina, produto ou secao"
    :loading="loading"
    :error="error"
    :empty="!loading && !error && isEmpty"
    @retry="emit('retry')"
  >
    <template #actions>
      <label class="cardapio-analytics-dwell__dim">
        <span class="cardapio-analytics-dwell__dim-label">Dimensao</span>
        <select
          class="cardapio-analytics-dwell__select"
          :value="dimension"
          @change="onDimensionChange"
        >
          <option v-for="option in DIMENSION_OPTIONS" :key="option.value" :value="option.value">
            {{ option.label }}
          </option>
        </select>
      </label>
    </template>

    <ul class="cardapio-analytics-dwell__list">
      <li v-for="item in items" :key="item.key" class="cardapio-analytics-dwell__item">
        <div class="cardapio-analytics-dwell__row">
          <span class="cardapio-analytics-dwell__key">{{ item.key }}</span>
          <span class="cardapio-analytics-dwell__time">
            {{ formatAnalyticsDuration(item.avgDwellSeconds) }}
            <span class="cardapio-analytics-dwell__samples">
              {{ formatAnalyticsInt(item.samples) }} amostras
            </span>
          </span>
        </div>
        <div class="cardapio-analytics-dwell__track">
          <div class="cardapio-analytics-dwell__bar" :style="{ width: barWidth(item) }"></div>
        </div>
      </li>
    </ul>
  </CardapioAnalyticsCard>
</template>

<style scoped>
.cardapio-analytics-dwell__dim {
  display: inline-flex;
  align-items: center;
  gap: 0.45rem;
}

.cardapio-analytics-dwell__dim-label {
  font-size: 0.74rem;
  font-weight: 600;
  color: var(--text-muted);
}

.cardapio-analytics-dwell__select {
  min-height: 2rem;
  padding: 0 0.5rem;
  border: 1px solid var(--line-soft);
  border-radius: var(--radius-sm);
  background: rgb(var(--surface) / 0.9);
  color: var(--text-main);
  font-size: 0.82rem;
  cursor: pointer;
}

.cardapio-analytics-dwell__list {
  display: flex;
  flex-direction: column;
  gap: 0.7rem;
  list-style: none;
  margin: 0;
  padding: 0;
}

.cardapio-analytics-dwell__row {
  display: flex;
  align-items: baseline;
  justify-content: space-between;
  gap: 0.75rem;
  margin-bottom: 0.3rem;
}

.cardapio-analytics-dwell__key {
  font-size: 0.84rem;
  font-weight: 600;
  color: var(--text-main);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  min-width: 0;
}

.cardapio-analytics-dwell__time {
  font-size: 0.84rem;
  font-weight: 700;
  color: var(--text-main);
  white-space: nowrap;
}

.cardapio-analytics-dwell__samples {
  font-weight: 500;
  color: var(--text-muted);
}

.cardapio-analytics-dwell__track {
  height: 0.5rem;
  border-radius: 999px;
  background: rgb(var(--surface-2) / 0.7);
  overflow: hidden;
}

.cardapio-analytics-dwell__bar {
  height: 100%;
  border-radius: 999px;
  background: rgb(var(--primary));
  transition: width 0.3s ease;
}
</style>
